package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"carbon-scribe/project-portal/project-portal-backend/internal/financing"
	"carbon-scribe/project-portal/project-portal-backend/internal/financing/payments"
)

// PayoutWorker handles background revenue distribution tasks
type PayoutWorker struct {
	db                 *sqlx.DB
	distributionManager *payments.RevenueDistributionManager
	repository         financing.Repository
	stopChan           chan struct{}
	pollInterval       time.Duration
	maxRetries         int
	batchSize          int
	paymentProcessors  map[string]payments.PaymentProcessor
}

// PayoutTask represents a payout task from the queue
type PayoutTask struct {
	ID              uuid.UUID `json:"id"`
	DistributionID   uuid.UUID `json:"distribution_id"`
	BeneficiaryID   uuid.UUID `json:"beneficiary_id"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	PaymentMethod   string    `json:"payment_method"`
	PaymentProvider string    `json:"payment_provider"`
	Priority        int       `json:"priority"`
	CreatedAt       time.Time `json:"created_at"`
	Retries         int       `json:"retries"`
}

// PayoutWorkerConfig holds configuration for the payout worker
type PayoutWorkerConfig struct {
	DatabaseURL     string        `json:"database_url"`
	PollInterval   time.Duration `json:"poll_interval"`
	MaxRetries     int           `json:"max_retries"`
	BatchSize      int           `json:"batch_size"`
	PaymentConfig  payments.ProcessorConfig `json:"payment_config"`
}

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := sqlx.Connect("postgres", config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create tables if they don't exist
	if err := createPayoutTasksTable(db); err != nil {
		log.Fatalf("Failed to create payout tasks table: %v", err)
	}

	// Initialize repository
	repository := financing.NewSQLRepository(db)

	// Initialize payment processor registry
	paymentRegistry := payments.NewPaymentProcessorRegistry(&config.PaymentConfig)

	// Initialize distribution manager
	distributionConfig := &payments.DistributionConfig{
		DefaultPlatformFee:    5.0,
		MinPlatformFee:        1.0,
		MaxPlatformFee:        100.0,
		MinDistributionAmount: 0.01,
		MaxBatchSize:          100,
		PaymentTimeoutMinutes: 30,
		RetryAttempts:         3,
		RetryDelayMinutes:     5,
		AutoApproveThreshold: 1000.0,
	}

	distributionManager := payments.NewRevenueDistributionManager(repository, distributionConfig)

	// Create and start worker
	worker := &PayoutWorker{
		db:                 db,
		distributionManager: distributionManager,
		repository:         repository,
		stopChan:           make(chan struct{}),
		pollInterval:       config.PollInterval,
		maxRetries:         config.MaxRetries,
		batchSize:          config.BatchSize,
		paymentProcessors:  make(map[string]payments.PaymentProcessor),
	}

	// Register payment processors
	processors := paymentRegistry.ListProcessors()
	for _, name := range processors {
		if processor, err := paymentRegistry.GetProcessor(name); err == nil {
			worker.paymentProcessors[name] = processor
		}
	}

	log.Printf("Starting payout worker with poll interval: %v", config.PollInterval)
	log.Printf("Registered payment processors: %v", processors)
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping worker...")
		close(worker.stopChan)
	}()

	// Start worker
	if err := worker.Start(); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}

	log.Println("Payout worker stopped")
}

// Start starts the payout worker
func (w *PayoutWorker) Start() error {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return nil
		case <-ticker.C:
			if err := w.processPendingTasks(); err != nil {
				log.Printf("Error processing tasks: %v", err)
			}
		}
	}
}

// processPendingTasks processes all pending payout tasks
func (w *PayoutWorker) processPendingTasks() error {
	ctx := context.Background()

	// Get pending tasks from database
	tasks, err := w.getPendingTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending tasks: %w", err)
	}

	if len(tasks) == 0 {
		log.Println("No pending payout tasks")
		return nil
	}

	log.Printf("Processing %d pending payout tasks", len(tasks))

	// Process tasks in batches by payment provider
	tasksByProvider := w.groupTasksByProvider(tasks)
	
	for provider, providerTasks := range tasksByProvider {
		if err := w.processProviderTasks(ctx, provider, providerTasks); err != nil {
			log.Printf("Failed to process tasks for provider %s: %v", provider, err)
		}
	}

	return nil
}

// processProviderTasks processes tasks for a specific payment provider
func (w *PayoutWorker) processProviderTasks(ctx context.Context, provider string, tasks []*PayoutTask) error {
	processor, exists := w.paymentProcessors[provider]
	if !exists {
		return fmt.Errorf("payment processor %s not found", provider)
	}

	log.Printf("Processing %d tasks for provider %s", len(tasks), provider)

	// Process each task
	for _, task := range tasks {
		if err := w.processTask(ctx, task, processor); err != nil {
			log.Printf("Failed to process task %s: %v", task.ID, err)
			// Mark task as failed if max retries exceeded
			if task.Retries >= w.maxRetries {
				if err := w.markTaskFailed(ctx, task.ID, err.Error()); err != nil {
					log.Printf("Failed to mark task %s as failed: %v", task.ID, err)
				}
			} else {
				// Increment retry count and requeue
				if err := w.incrementTaskRetries(ctx, task.ID); err != nil {
					log.Printf("Failed to increment retries for task %s: %v", task.ID, err)
				}
			}
			continue
		}

		// Mark task as completed
		if err := w.markTaskCompleted(ctx, task.ID); err != nil {
			log.Printf("Failed to mark task %s as completed: %v", task.ID, err)
		}

		log.Printf("Successfully processed payout task %s", task.ID)
	}

	return nil
}

// processTask processes a single payout task
func (w *PayoutWorker) processTask(ctx context.Context, task *PayoutTask, processor payments.PaymentProcessor) error {
	log.Printf("Processing payout task %s: amount=%.2f %s via %s", 
		task.ID, task.Amount, task.Currency, task.PaymentProvider)

	// Get beneficiary details
	beneficiary, err := w.getBeneficiaryDetails(ctx, task.BeneficiaryID)
	if err != nil {
		return fmt.Errorf("failed to get beneficiary details: %w", err)
	}

	// Create payment request
	paymentReq := &payments.PaymentRequest{
		Amount:      task.Amount,
		Currency:    task.Currency,
		Recipient: payments.PaymentRecipient{
			UserID:   task.BeneficiaryID,
			Name:     beneficiary.Name,
			Email:    beneficiary.Email,
			Phone:    beneficiary.Phone,
			Country:  beneficiary.Country,
		},
		PaymentMethod: task.PaymentMethod,
		ReferenceID:  task.ID.String(),
		Description:  fmt.Sprintf("Revenue distribution from CarbonScribe - Task %s", task.ID),
		WebhookURL:    "https://api.carbon-scribe.com/webhooks/payout",
	}

	// Add payment method specific details
	switch task.PaymentMethod {
	case "bank_transfer":
		paymentReq.Recipient.BankDetails = &payments.BankDetails{
			AccountNumber: beneficiary.BankAccountNumber,
			RoutingNumber: beneficiary.RoutingNumber,
			BankName:      beneficiary.BankName,
			AccountType:   beneficiary.AccountType,
			Currency:      task.Currency,
		}
	case "stellar":
		paymentReq.Recipient.WalletDetails = &payments.WalletDetails{
			Type:     "stellar",
			Address:  beneficiary.StellarAddress,
			Network:  "public",
			Currency: task.Currency,
		}
	}

	// Process payment
	response, err := processor.ProcessPayment(ctx, paymentReq)
	if err != nil {
		return fmt.Errorf("payment processing failed: %w", err)
	}

	log.Printf("Payment initiated for task %s: transaction_id=%s, status=%s", 
		task.ID, response.TransactionID, response.Status)

	// Update task with transaction details
	if err := w.updateTaskWithTransaction(ctx, task.ID, response.TransactionID, response.Status); err != nil {
		log.Printf("Failed to update task %s with transaction details: %v", task.ID, err)
	}

	// Monitor payment status asynchronously
	go w.monitorPaymentStatus(context.Background(), task.ID, response.TransactionID, processor)

	return nil
}

// monitorPaymentStatus monitors the status of a payment
func (w *PayoutWorker) monitorPaymentStatus(ctx context.Context, taskID uuid.UUID, transactionID string, processor payments.PaymentProcessor) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	maxChecks := 30 // Check for up to 1 hour
	checkCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkCount++
			
			// Get payment status
			statusResp, err := processor.GetPaymentStatus(ctx, transactionID)
			if err != nil {
				log.Printf("Failed to get payment status for task %s: %v", taskID, err)
				continue
			}

			log.Printf("Payment status check for task %s: status=%s", taskID, statusResp.Status)

			// Update task with latest status
			if err := w.updateTaskStatus(ctx, taskID, statusResp.Status); err != nil {
				log.Printf("Failed to update task %s status: %v", taskID, err)
			}

			// Check if payment is complete
			if statusResp.Status == "completed" || statusResp.Status == "succeeded" {
				log.Printf("Payment completed for task %s", taskID)
				if err := w.markTaskCompleted(ctx, taskID); err != nil {
					log.Printf("Failed to mark task %s as completed: %v", taskID, err)
				}
				return
			}

			// Check if payment failed
			if statusResp.Status == "failed" {
				errorMsg := "Payment failed"
				if statusResp.FailureReason != nil {
					errorMsg = *statusResp.FailureReason
				}
				log.Printf("Payment failed for task %s: %s", taskID, errorMsg)
				if err := w.markTaskFailed(ctx, taskID, errorMsg); err != nil {
					log.Printf("Failed to mark task %s as failed: %v", taskID, err)
				}
				return
			}

			// Stop checking after max attempts
			if checkCount >= maxChecks {
				log.Printf("Max status checks reached for task %s, marking as timeout", taskID)
				if err := w.markTaskFailed(ctx, taskID, "Payment status monitoring timeout"); err != nil {
					log.Printf("Failed to mark task %s as failed: %v", taskID, err)
				}
				return
			}
		}
	}
}

// getPendingTasks retrieves pending payout tasks from database
func (w *PayoutWorker) getPendingTasks(ctx context.Context) ([]*PayoutTask, error) {
	query := `
		SELECT id, distribution_id, beneficiary_id, amount, currency, payment_method, payment_provider,
			   priority, created_at, retries
		FROM payout_tasks 
		WHERE status = 'pending' 
		ORDER BY priority DESC, created_at ASC
		LIMIT $1
	`

	var tasks []*PayoutTask
	err := w.db.SelectContext(ctx, &tasks, query, w.batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending tasks: %w", err)
	}

	return tasks, nil
}

// groupTasksByProvider groups tasks by payment provider
func (w *PayoutWorker) groupTasksByProvider(tasks []*PayoutTask) map[string][]*PayoutTask {
	grouped := make(map[string][]*PayoutTask)
	
	for _, task := range tasks {
		provider := task.PaymentProvider
		grouped[provider] = append(grouped[provider], task)
	}
	
	return grouped
}

// getBeneficiaryDetails retrieves beneficiary details
func (w *PayoutWorker) getBeneficiaryDetails(ctx context.Context, beneficiaryID uuid.UUID) (*BeneficiaryDetails, error) {
	// Mock implementation - in real code, this would query users/beneficiaries table
	return &BeneficiaryDetails{
		ID:                beneficiaryID,
		Name:              "John Farmer",
		Email:             "john.farmer@example.com",
		Phone:             "+254712345678",
		Country:           "KE",
		BankAccountNumber:  "1234567890",
		RoutingNumber:      "123456789",
		BankName:          "Equity Bank",
		AccountType:       "checking",
		StellarAddress:    "GABCDEF1234567890ABCDEF1234567890ABCDEF12",
	}, nil
}

// markTaskCompleted marks a task as completed
func (w *PayoutWorker) markTaskCompleted(ctx context.Context, taskID uuid.UUID) error {
	query := `
		UPDATE payout_tasks 
		SET status = 'completed', completed_at = NOW() 
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to mark task as completed: %w", err)
	}

	return nil
}

// markTaskFailed marks a task as failed
func (w *PayoutWorker) markTaskFailed(ctx context.Context, taskID uuid.UUID, errorMsg string) error {
	query := `
		UPDATE payout_tasks 
		SET status = 'failed', error_message = $2, completed_at = NOW() 
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, taskID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark task as failed: %w", err)
	}

	return nil
}

// updateTaskWithTransaction updates task with transaction details
func (w *PayoutWorker) updateTaskWithTransaction(ctx context.Context, taskID uuid.UUID, transactionID, status string) error {
	query := `
		UPDATE payout_tasks 
		SET transaction_id = $2, payment_status = $3, updated_at = NOW() 
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, taskID, transactionID, status)
	if err != nil {
		return fmt.Errorf("failed to update task with transaction: %w", err)
	}

	return nil
}

// updateTaskStatus updates task payment status
func (w *PayoutWorker) updateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	query := `
		UPDATE payout_tasks 
		SET payment_status = $2, updated_at = NOW() 
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, taskID, status)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

// incrementTaskRetries increments the retry count for a task
func (w *PayoutWorker) incrementTaskRetries(ctx context.Context, taskID uuid.UUID) error {
	query := `
		UPDATE payout_tasks 
		SET retries = retries + 1, next_retry_at = NOW() + INTERVAL '10 minutes' 
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to increment task retries: %w", err)
	}

	return nil
}

// loadConfig loads configuration from file
func loadConfig(filename string) (*PayoutWorkerConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config PayoutWorkerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.PollInterval == 0 {
		config.PollInterval = 60 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.BatchSize == 0 {
		config.BatchSize = 50
	}

	return &config, nil
}

// BeneficiaryDetails represents beneficiary payment information
type BeneficiaryDetails struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	Country           string    `json:"country"`
	BankAccountNumber string    `json:"bank_account_number"`
	RoutingNumber     string    `json:"routing_number"`
	BankName         string    `json:"bank_name"`
	AccountType      string    `json:"account_type"`
	StellarAddress   string    `json:"stellar_address"`
}

// createPayoutTasksTable creates the payout tasks table if it doesn't exist
func createPayoutTasksTable(db *sqlx.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS payout_tasks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			distribution_id UUID NOT NULL REFERENCES revenue_distributions(id),
			beneficiary_id UUID NOT NULL,
			amount DECIMAL(14, 4) NOT NULL,
			currency VARCHAR(3) NOT NULL,
			payment_method VARCHAR(50) NOT NULL,
			payment_provider VARCHAR(50) NOT NULL,
			transaction_id VARCHAR(100),
			payment_status VARCHAR(50),
			priority INTEGER NOT NULL DEFAULT 0,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			next_retry_at TIMESTAMPTZ,
			retries INTEGER NOT NULL DEFAULT 0,
			error_message TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_payout_tasks_status ON payout_tasks(status);
		CREATE INDEX IF NOT EXISTS idx_payout_tasks_priority ON payout_tasks(priority DESC, created_at ASC);
		CREATE INDEX IF NOT EXISTS idx_payout_tasks_next_retry ON payout_tasks(next_retry_at) WHERE next_retry_at IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_payout_tasks_distribution ON payout_tasks(distribution_id);
	`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create payout_tasks table: %w", err)
	}

	return nil
}
