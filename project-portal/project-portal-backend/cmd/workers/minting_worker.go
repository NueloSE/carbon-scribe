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
	"carbon-scribe/project-portal/project-portal-backend/internal/financing/tokenization"
)

// MintingWorker handles background credit minting tasks
type MintingWorker struct {
	db                *sqlx.DB
	workflow          *tokenization.Workflow
	repository        financing.Repository
	stopChan          chan struct{}
	pollInterval      time.Duration
	maxRetries        int
	batchSize         int
}

// MintingTask represents a minting task from the queue
type MintingTask struct {
	ID        uuid.UUID `json:"id"`
	CreditIDs []uuid.UUID `json:"credit_ids"`
	BatchSize int       `json:"batch_size"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	Retries   int       `json:"retries"`
}

// WorkerConfig holds configuration for the worker
type WorkerConfig struct {
	DatabaseURL        string        `json:"database_url"`
	PollInterval      time.Duration `json:"poll_interval"`
	MaxRetries        int           `json:"max_retries"`
	BatchSize         int           `json:"batch_size"`
	StellarHorizonURL string        `json:"stellar_horizon_url"`
	StellarSorobanURL string        `json:"stellar_soroban_url"`
	NetworkPassphrase  string        `json:"network_passphrase"`
	IssuerSecret      string        `json:"issuer_secret"`
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

	// Initialize repository
	repository := financing.NewSQLRepository(db)

	// Initialize Stellar client and workflow
	issuerAccount := &tokenization.StellarAccount{
		PublicKey: getPublicKey(config.IssuerSecret),
		SecretKey: config.IssuerSecret,
	}

	stellarClient := tokenization.NewStellarClient(
		config.StellarHorizonURL,
		config.StellarSorobanURL,
		config.NetworkPassphrase,
		issuerAccount,
	)

	workflowConfig := &tokenization.WorkflowConfig{
		MaxRetries:          config.MaxRetries,
		RetryDelay:          30 * time.Second,
		ConfirmationTimeout:   5 * time.Minute,
		BatchSize:           config.BatchSize,
		GasOptimization:     true,
	}

	workflow := tokenization.NewWorkflow(stellarClient, repository, nil, workflowConfig)

	// Create and start worker
	worker := &MintingWorker{
		db:           db,
		workflow:     workflow,
		repository:   repository,
		stopChan:     make(chan struct{}),
		pollInterval: config.PollInterval,
		maxRetries:   config.MaxRetries,
		batchSize:    config.BatchSize,
	}

	log.Printf("Starting minting worker with poll interval: %v", config.PollInterval)
	
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

	log.Println("Minting worker stopped")
}

// Start starts the minting worker
func (w *MintingWorker) Start() error {
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

// processPendingTasks processes all pending minting tasks
func (w *MintingWorker) processPendingTasks() error {
	ctx := context.Background()

	// Get pending tasks from database
	tasks, err := w.getPendingTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending tasks: %w", err)
	}

	if len(tasks) == 0 {
		log.Println("No pending minting tasks")
		return nil
	}

	log.Printf("Processing %d pending minting tasks", len(tasks))

	// Process each task
	for _, task := range tasks {
		if err := w.processTask(ctx, task); err != nil {
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

		log.Printf("Successfully processed minting task %s", task.ID)
	}

	return nil
}

// processTask processes a single minting task
func (w *MintingWorker) processTask(ctx context.Context, task *MintingTask) error {
	log.Printf("Processing minting task %s with %d credits", task.ID, len(task.CreditIDs))

	// Create minting request
	req := &financing.MintRequest{
		CreditIDs: task.CreditIDs,
		BatchSize: task.BatchSize,
	}

	// Execute minting workflow
	result, err := w.workflow.ExecuteMintingWorkflow(ctx, req)
	if err != nil {
		return fmt.Errorf("minting workflow failed: %w", err)
	}

	log.Printf("Minting workflow completed for task %s: batch=%s, status=%s", 
		task.ID, result.BatchID, result.Status)

	// Update credit records with minting results
	for i, creditID := range task.CreditIDs {
		var tokenIDs []string
		if i < len(result.TokenIDs) {
			tokenIDs = []string{result.TokenIDs[i]}
		}

		// Update credit status in database
		credits, err := w.repository.GetCarbonCredits(ctx, []uuid.UUID{creditID})
		if err != nil {
			return fmt.Errorf("failed to get credit %s: %w", creditID, err)
		}

		if len(credits) > 0 {
			credit := credits[0]
			credit.Status = financing.CreditStatusMinted
			credit.IssuedTons = &credit.BufferedTons
			credit.TokenIDs = tokenization.TokenIDArray(tokenIDs)
			credit.MintTransactionHash = &result.TransactionID
			mintedAt := time.Now()
			credit.MintedAt = &mintedAt
			credit.UpdatedAt = time.Now()

			if err := w.repository.UpdateCarbonCredit(ctx, credit); err != nil {
				return fmt.Errorf("failed to update credit %s: %w", creditID, err)
			}
		}
	}

	return nil
}

// getPendingTasks retrieves pending minting tasks from database
func (w *MintingWorker) getPendingTasks(ctx context.Context) ([]*MintingTask, error) {
	query := `
		SELECT id, credit_ids, batch_size, priority, created_at, retries
		FROM minting_tasks 
		WHERE status = 'pending' 
		ORDER BY priority DESC, created_at ASC
		LIMIT $1
	`

	var tasks []*MintingTask
	err := w.db.SelectContext(ctx, &tasks, query, w.batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending tasks: %w", err)
	}

	return tasks, nil
}

// markTaskCompleted marks a task as completed
func (w *MintingWorker) markTaskCompleted(ctx context.Context, taskID uuid.UUID) error {
	query := `
		UPDATE minting_tasks 
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
func (w *MintingWorker) markTaskFailed(ctx context.Context, taskID uuid.UUID, errorMsg string) error {
	query := `
		UPDATE minting_tasks 
		SET status = 'failed', error_message = $2, completed_at = NOW() 
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, taskID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark task as failed: %w", err)
	}

	return nil
}

// incrementTaskRetries increments the retry count for a task
func (w *MintingWorker) incrementTaskRetries(ctx context.Context, taskID uuid.UUID) error {
	query := `
		UPDATE minting_tasks 
		SET retries = retries + 1, next_retry_at = NOW() + INTERVAL '5 minutes' 
		WHERE id = $1
	`

	_, err := w.db.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to increment task retries: %w", err)
	}

	return nil
}

// loadConfig loads configuration from file
func loadConfig(filename string) (*WorkerConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WorkerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.PollInterval == 0 {
		config.PollInterval = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.BatchSize == 0 {
		config.BatchSize = 10
	}

	return &config, nil
}

// getPublicKey extracts public key from secret seed
func getPublicKey(secretSeed string) string {
	// In a real implementation, this would properly decode the Stellar secret seed
	// For now, return a mock public key
	return "GABCDEF1234567890ABCDEF1234567890ABCDEF12"
}

// createMintingTasksTable creates the minting tasks table if it doesn't exist
func createMintingTasksTable(db *sqlx.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS minting_tasks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			credit_ids JSONB NOT NULL,
			batch_size INTEGER NOT NULL DEFAULT 10,
			priority INTEGER NOT NULL DEFAULT 0,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			next_retry_at TIMESTAMPTZ,
			retries INTEGER NOT NULL DEFAULT 0,
			error_message TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_minting_tasks_status ON minting_tasks(status);
		CREATE INDEX IF NOT EXISTS idx_minting_tasks_priority ON minting_tasks(priority DESC, created_at ASC);
		CREATE INDEX IF NOT EXISTS idx_minting_tasks_next_retry ON minting_tasks(next_retry_at) WHERE next_retry_at IS NOT NULL;
	`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create minting_tasks table: %w", err)
	}

	return nil
}
