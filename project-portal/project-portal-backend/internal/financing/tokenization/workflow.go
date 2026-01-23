package tokenization

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"carbon-scribe/project-portal/project-portal-backend/internal/financing"
)

// Workflow orchestrates the token minting workflow
type Workflow struct {
	stellarClient   *StellarClient
	repository      Repository
	monitor         *TransactionMonitor
	config          *WorkflowConfig
}

// WorkflowConfig holds workflow configuration
type WorkflowConfig struct {
	MaxRetries          int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`
	ConfirmationTimeout time.Duration `json:"confirmation_timeout"`
	BatchSize           int           `json:"batch_size"`
	GasOptimization     bool          `json:"gas_optimization"`
}

// Repository defines the interface for data persistence
type Repository interface {
	GetCarbonCredits(ctx context.Context, creditIDs []uuid.UUID) ([]*financing.CarbonCredit, error)
	UpdateCreditMintingStatus(ctx context.Context, creditID uuid.UUID, status financing.CreditStatus, txHash *string, tokenIDs []string) error
	CreateMintingBatch(ctx context.Context, batch *MintingBatch) error
	UpdateMintingBatch(ctx context.Context, batchID string, status BatchStatus, txHash *string) error
	GetMintingBatch(ctx context.Context, batchID string) (*MintingBatch, error)
}

// MintingBatch represents a batch of minting operations
type MintingBatch struct {
	ID            string           `json:"id"`
	CreditIDs     []uuid.UUID      `json:"credit_ids"`
	Status        BatchStatus      `json:"status"`
	TransactionID *string          `json:"transaction_id"`
	CreatedAt     time.Time        `json:"created_at"`
	StartedAt     *time.Time       `json:"started_at"`
	CompletedAt   *time.Time       `json:"completed_at"`
	Error         *string          `json:"error,omitempty"`
	RetryCount    int              `json:"retry_count"`
}

// BatchStatus represents the status of a minting batch
type BatchStatus string

const (
	BatchStatusPending    BatchStatus = "pending"
	BatchStatusProcessing BatchStatus = "processing"
	BatchStatusCompleted  BatchStatus = "completed"
	BatchStatusFailed     BatchStatus = "failed"
	BatchStatusRetrying   BatchStatus = "retrying"
)

// WorkflowResult represents the result of a workflow execution
type WorkflowResult struct {
	BatchID      string                    `json:"batch_id"`
	Status       BatchStatus               `json:"status"`
	CreditIDs    []uuid.UUID               `json:"credit_ids"`
	TokenIDs     []string                  `json:"token_ids"`
	TransactionID *string                  `json:"transaction_id"`
	Errors       []string                  `json:"errors,omitempty"`
	Warnings     []string                  `json:"warnings,omitempty"`
	Duration     time.Duration             `json:"duration"`
	CreatedAt    time.Time                 `json:"created_at"`
	CompletedAt  *time.Time                `json:"completed_at,omitempty"`
}

// NewWorkflow creates a new token minting workflow
func NewWorkflow(stellarClient *StellarClient, repository Repository, monitor *TransactionMonitor, config *WorkflowConfig) *Workflow {
	if config == nil {
		config = &WorkflowConfig{
			MaxRetries:          3,
			RetryDelay:          30 * time.Second,
			ConfirmationTimeout: 5 * time.Minute,
			BatchSize:           10,
			GasOptimization:     true,
		}
	}
	
	return &Workflow{
		stellarClient: stellarClient,
		repository:    repository,
		monitor:       monitor,
		config:        config,
	}
}

// ExecuteMintingWorkflow executes the complete minting workflow
func (w *Workflow) ExecuteMintingWorkflow(ctx context.Context, req *financing.MintRequest) (*WorkflowResult, error) {
	startTime := time.Now()
	
	// Create workflow result
	result := &WorkflowResult{
		BatchID:   uuid.New().String(),
		CreditIDs: req.CreditIDs,
		Status:    BatchStatusPending,
		CreatedAt: startTime,
	}
	
	log.Printf("Starting minting workflow for batch %s with %d credits", result.BatchID, len(req.CreditIDs))
	
	// Step 1: Validate credits
	if err := w.validateCredits(ctx, req.CreditIDs); err != nil {
		result.Status = BatchStatusFailed
		result.Errors = append(result.Errors, err.Error())
		result.CompletedAt = &[]time.Time{time.Now()}[0]
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("credit validation failed: %w", err)
	}
	
	// Step 2: Create minting batch
	batch := &MintingBatch{
		ID:        result.BatchID,
		CreditIDs: req.CreditIDs,
		Status:    BatchStatusPending,
		CreatedAt: startTime,
	}
	
	if err := w.repository.CreateMintingBatch(ctx, batch); err != nil {
		result.Status = BatchStatusFailed
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create batch: %v", err))
		result.CompletedAt = &[]time.Time{time.Now()}[0]
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("failed to create minting batch: %w", err)
	}
	
	// Step 3: Update credit status to minting
	if err := w.updateCreditsStatus(ctx, req.CreditIDs, financing.CreditStatusMinting, nil, nil); err != nil {
		result.Status = BatchStatusFailed
		result.Errors = append(result.Errors, fmt.Sprintf("failed to update credit status: %v", err))
		result.CompletedAt = &[]time.Time{time.Now()}[0]
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("failed to update credit status: %w", err)
	}
	
	// Step 4: Execute minting with retry logic
	mintResult, err := w.executeMintingWithRetry(ctx, req, batch)
	if err != nil {
		result.Status = BatchStatusFailed
		result.Errors = append(result.Errors, err.Error())
		result.CompletedAt = &[]time.Time{time.Now()}[0]
		result.Duration = time.Since(startTime)
		
		// Update credit status to calculated on failure
		w.updateCreditsStatus(ctx, req.CreditIDs, financing.CreditStatusCalculated, nil, nil)
		
		return result, fmt.Errorf("minting execution failed: %w", err)
	}
	
	// Step 5: Update credit status to minted
	if err := w.updateCreditsStatus(ctx, req.CreditIDs, financing.CreditStatusMinted, mintResult.TransactionID, mintResult.TokenIDs); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("credits minted but status update failed: %v", err))
	}
	
	// Step 6: Start transaction monitoring
	if err := w.monitor.StartMonitoring(ctx, *mintResult.TransactionID, result.BatchID); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to start transaction monitoring: %v", err))
	}
	
	// Update result
	result.Status = BatchStatusCompleted
	result.TokenIDs = mintResult.TokenIDs
	result.TransactionID = mintResult.TransactionID
	result.CompletedAt = &[]time.Time{time.Now()}[0]
	result.Duration = time.Since(startTime)
	
	log.Printf("Minting workflow completed for batch %s in %v", result.BatchID, result.Duration)
	
	return result, nil
}

// validateCredits validates that credits are ready for minting
func (w *Workflow) validateCredits(ctx context.Context, creditIDs []uuid.UUID) error {
	credits, err := w.repository.GetCarbonCredits(ctx, creditIDs)
	if err != nil {
		return fmt.Errorf("failed to retrieve credits: %w", err)
	}
	
	if len(credits) != len(creditIDs) {
		return fmt.Errorf("missing credits: expected %d, found %d", len(creditIDs), len(credits))
	}
	
	for _, credit := range credits {
		if credit.Status != financing.CreditStatusVerified {
			return fmt.Errorf("credit %s is not verified (current status: %s)", credit.ID, credit.Status)
		}
		
		if credit.IssuedTons != nil && *credit.IssuedTons > 0 {
			return fmt.Errorf("credit %s has already been issued", credit.ID)
		}
	}
	
	return nil
}

// executeMintingWithRetry executes minting with retry logic
func (w *Workflow) executeMintingWithRetry(ctx context.Context, req *financing.MintRequest, batch *MintingBatch) (*MintResponse, error) {
	var lastErr error
	
	for attempt := 0; attempt <= w.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Update batch status to retrying
			batch.Status = BatchStatusRetrying
			batch.RetryCount = attempt
			w.repository.UpdateMintingBatch(ctx, batch.ID, batch.Status, nil)
			
			log.Printf("Retrying minting for batch %s, attempt %d/%d", batch.ID, attempt, w.config.MaxRetries)
			
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(w.config.RetryDelay):
			}
		}
		
		// Update batch status to processing
		batch.Status = BatchStatusProcessing
		now := time.Now()
		batch.StartedAt = &now
		w.repository.UpdateMintingBatch(ctx, batch.ID, batch.Status, nil)
		
		// Prepare mint request
		mintReq, err := w.prepareMintRequest(ctx, req.CreditIDs)
		if err != nil {
			lastErr = err
			continue
		}
		
		// Execute minting
		mintResult, err := w.stellarClient.MintTokens(ctx, mintReq)
		if err != nil {
			lastErr = fmt.Errorf("minting attempt %d failed: %w", attempt+1, err)
			log.Printf("Minting attempt %d failed for batch %s: %v", attempt+1, batch.ID, err)
			continue
		}
		
		// Wait for confirmation
		transaction, err := w.stellarClient.WaitForConfirmation(ctx, mintResult.TransactionID, w.config.ConfirmationTimeout)
		if err != nil {
			lastErr = fmt.Errorf("transaction confirmation failed: %w", err)
			log.Printf("Transaction confirmation failed for batch %s: %v", batch.ID, err)
			continue
		}
		
		if transaction.Status != TransactionStatusSuccess {
			lastErr = fmt.Errorf("transaction failed: %s", transaction.FailureReason)
			log.Printf("Transaction failed for batch %s: %s", batch.ID, transaction.FailureReason)
			continue
		}
		
		// Success - update batch
		batch.Status = BatchStatusCompleted
		batch.TransactionID = &mintResult.TransactionID
		completedAt := time.Now()
		batch.CompletedAt = &completedAt
		w.repository.UpdateMintingBatch(ctx, batch.ID, batch.Status, batch.TransactionID)
		
		log.Printf("Minting successful for batch %s, transaction: %s", batch.ID, mintResult.TransactionID)
		
		return mintResult, nil
	}
	
	// All retries exhausted
	batch.Status = BatchStatusFailed
	errorMsg := lastErr.Error()
	batch.Error = &errorMsg
	w.repository.UpdateMintingBatch(ctx, batch.ID, batch.Status, nil)
	
	return nil, fmt.Errorf("minting failed after %d attempts: %w", w.config.MaxRetries+1, lastErr)
}

// prepareMintRequest prepares mint request from credit IDs
func (w *Workflow) prepareMintRequest(ctx context.Context, creditIDs []uuid.UUID) (*MintRequest, error) {
	credits, err := w.repository.GetCarbonCredits(ctx, creditIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credits: %w", err)
	}
	
	if len(credits) == 0 {
		return nil, fmt.Errorf("no credits found")
	}
	
	// Use first credit for metadata (all credits should have same project and vintage)
	firstCredit := credits[0]
	
	// Calculate total amount
	var totalAmount float64
	for _, credit := range credits {
		if credit.BufferedTons > 0 {
			totalAmount += credit.BufferedTons
		} else {
			totalAmount += credit.CalculatedTons
		}
	}
	
	// Generate asset code
	assetCode := w.stellarClient.GenerateAssetCode(firstCredit.ProjectID, firstCredit.VintageYear)
	
	metadata := TokenMetadata{
		ProjectID:      firstCredit.ProjectID,
		VintageYear:    firstCredit.VintageYear,
		Methodology:    firstCredit.MethodologyCode,
		CalculatedTons: totalAmount,
		QualityScore:   0.0, // Will be calculated
		VerificationID: firstCredit.VerificationID,
		IssuedAt:       time.Now(),
	}
	
	// Calculate average quality score
	if len(credits) > 0 {
		var totalQuality float64
		qualityCount := 0
		for _, credit := range credits {
			if credit.DataQualityScore != nil {
				totalQuality += *credit.DataQualityScore
				qualityCount++
			}
		}
		if qualityCount > 0 {
			metadata.QualityScore = totalQuality / float64(qualityCount)
		}
	}
	
	return &MintRequest{
		CreditIDs:  creditIDs,
		ContractID: "carbon_asset_contract", // Default contract
		Recipient:  w.stellarClient.issuerAccount.PublicKey,
		Amount:     totalAmount,
		Metadata:   metadata,
	}, nil
}

// updateCreditsStatus updates the status of multiple credits
func (w *Workflow) updateCreditsStatus(ctx context.Context, creditIDs []uuid.UUID, status financing.CreditStatus, txHash *string, tokenIDs []string) error {
	for _, creditID := range creditIDs {
		err := w.repository.UpdateCreditMintingStatus(ctx, creditID, status, txHash, tokenIDs)
		if err != nil {
			log.Printf("Failed to update credit %s status: %v", creditID, err)
			return fmt.Errorf("failed to update credit %s: %w", creditID, err)
		}
	}
	return nil
}

// GetWorkflowStatus gets the status of a workflow
func (w *Workflow) GetWorkflowStatus(ctx context.Context, batchID string) (*WorkflowResult, error) {
	batch, err := w.repository.GetMintingBatch(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get minting batch: %w", err)
	}
	
	result := &WorkflowResult{
		BatchID:      batch.ID,
		Status:       batch.Status,
		CreditIDs:    batch.CreditIDs,
		TransactionID: batch.TransactionID,
		CreatedAt:    batch.CreatedAt,
		StartedAt:    batch.StartedAt,
		CompletedAt:  batch.CompletedAt,
	}
	
	if batch.Error != nil {
		result.Errors = append(result.Errors, *batch.Error)
	}
	
	if batch.StartedAt != nil {
		if batch.CompletedAt != nil {
			result.Duration = batch.CompletedAt.Sub(*batch.StartedAt)
		} else {
			result.Duration = time.Since(*batch.StartedAt)
		}
	}
	
	return result, nil
}

// CancelWorkflow cancels an active workflow
func (w *Workflow) CancelWorkflow(ctx context.Context, batchID string) error {
	batch, err := w.repository.GetMintingBatch(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get minting batch: %w", err)
	}
	
	if batch.Status == BatchStatusCompleted || batch.Status == BatchStatusFailed {
		return fmt.Errorf("cannot cancel completed or failed workflow")
	}
	
	// Update batch status
	batch.Status = BatchStatusFailed
	errorMsg := "Workflow cancelled by user"
	batch.Error = &errorMsg
	completedAt := time.Now()
	batch.CompletedAt = &completedAt
	
	err = w.repository.UpdateMintingBatch(ctx, batch.ID, batch.Status, nil)
	if err != nil {
		return fmt.Errorf("failed to update batch status: %w", err)
	}
	
	// Reset credit status
	err = w.updateCreditsStatus(ctx, batch.CreditIDs, financing.CreditStatusCalculated, nil, nil)
	if err != nil {
		log.Printf("Failed to reset credit status after cancellation: %v", err)
	}
	
	log.Printf("Workflow %s cancelled", batchID)
	return nil
}

// RetryWorkflow retries a failed workflow
func (w *Workflow) RetryWorkflow(ctx context.Context, batchID string) error {
	batch, err := w.repository.GetMintingBatch(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get minting batch: %w", err)
	}
	
	if batch.Status != BatchStatusFailed {
		return fmt.Errorf("can only retry failed workflows")
	}
	
	if batch.RetryCount >= w.config.MaxRetries {
		return fmt.Errorf("maximum retry attempts exceeded")
	}
	
	// Reset batch status
	batch.Status = BatchStatusPending
	batch.Error = nil
	
	err = w.repository.UpdateMintingBatch(ctx, batch.ID, batch.Status, nil)
	if err != nil {
		return fmt.Errorf("failed to reset batch status: %w", err)
	}
	
	log.Printf("Workflow %s queued for retry", batchID)
	return nil
}
