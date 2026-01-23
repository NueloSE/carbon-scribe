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

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"carbon-scribe/project-portal/project-portal-backend/internal/financing/sales"
)

// PriceUpdateWorker handles background price update tasks
type PriceUpdateWorker struct {
	db            *sqlx.DB
	pricingEngine *sales.PricingEngine
	repository    sales.PricingRepository
	stopChan      chan struct{}
	pollInterval  time.Duration
	sources       []string
}

// PriceUpdateTask represents a price update task
type PriceUpdateTask struct {
	ID            uuid.UUID `json:"id"`
	Source        string    `json:"source"`
	Methodology   string    `json:"methodology"`
	Region        string    `json:"region"`
	VintageYear   int       `json:"vintage_year"`
	Priority      int       `json:"priority"`
	CreatedAt     time.Time `json:"created_at"`
	Retries       int       `json:"retries"`
}

// PriceUpdateWorkerConfig holds configuration for the price update worker
type PriceUpdateWorkerConfig struct {
	DatabaseURL     string        `json:"database_url"`
	PollInterval   time.Duration `json:"poll_interval"`
	MaxRetries     int           `json:"max_retries"`
	Sources        []string      `json:"sources"`
	UpdateInterval  time.Duration `json:"update_interval"`
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
	repository := sales.NewSQLPricingRepository(db)

	// Initialize pricing engine
	pricingEngine := sales.NewPricingEngine(repository, nil)

	// Create and start worker
	worker := &PriceUpdateWorker{
		db:           db,
		pricingEngine: pricingEngine,
		repository:   repository,
		stopChan:     make(chan struct{}),
		pollInterval: config.PollInterval,
		sources:      config.Sources,
	}

	log.Printf("Starting price update worker with poll interval: %v", config.PollInterval)
	log.Printf("Configured price sources: %v", config.Sources)
	
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

	log.Println("Price update worker stopped")
}

// Start starts the price update worker
func (w *PriceUpdateWorker) Start() error {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return nil
		case <-ticker.C:
			if err := w.processPriceUpdates(); err != nil {
				log.Printf("Error processing price updates: %v", err)
			}
		}
	}
}

// processPriceUpdates processes price updates from all configured sources
func (w *PriceUpdateWorker) processPriceUpdates() error {
	ctx := context.Background()

	log.Printf("Processing price updates from %d sources", len(w.sources))

	// Process each source
	for _, source := range w.sources {
		if err := w.processSourceUpdates(ctx, source); err != nil {
			log.Printf("Failed to process updates from source %s: %v", source, err)
			continue
		}
	}

	return nil
}

// processSourceUpdates processes price updates from a specific source
func (w *PriceUpdateWorker) processSourceUpdates(ctx context.Context, source string) error {
	log.Printf("Processing price updates from source: %s", source)

	// Get price data from source
	priceData, err := w.fetchPriceData(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to fetch price data from %s: %w", source, err)
	}

	// Update pricing models with new data
	for _, price := range priceData {
		if err := w.updatePricingModel(ctx, price); err != nil {
			log.Printf("Failed to update pricing model for %s/%s/%d: %v", 
				price.Methodology, price.Region, price.VintageYear, err)
			continue
		}

		log.Printf("Updated pricing model for %s/%s/%d: %.2f %s", 
			price.Methodology, price.Region, price.VintageYear, price.Price, price.Currency)
	}

	return nil
}

// fetchPriceData fetches price data from a source
func (w *PriceUpdateWorker) fetchPriceData(ctx context.Context, source string) ([]*PriceData, error) {
	switch source {
	case "cme":
		return w.fetchCMEDates(ctx)
	case "carboncredits":
		return w.fetchCarbonCreditsData(ctx)
	case "verra":
		return w.fetchVerraData(ctx)
	default:
		return nil, fmt.Errorf("unknown price source: %s", source)
	}
}

// fetchCMEDates fetches price data from CME Group
func (w *PriceUpdateWorker) fetchCMEDates(ctx context.Context) ([]*PriceData, error) {
	// Mock implementation - in real code, this would call CME API
	log.Println("Fetching price data from CME Group")

	return []*PriceData{
		{
			Source:       "cme",
			Methodology:  "VM0007",
			Region:       "US",
			VintageYear:  2024,
			Price:        18.50,
			Currency:     "USD",
			Volume:       1000.0,
			Timestamp:    time.Now(),
			QualityScore: 0.85,
		},
		{
			Source:       "cme",
			Methodology:  "VM0015",
			Region:       "US",
			VintageYear:  2024,
			Price:        17.25,
			Currency:     "USD",
			Volume:       500.0,
			Timestamp:    time.Now(),
			QualityScore: 0.80,
		},
	}, nil
}

// fetchCarbonCreditsData fetches price data from CarbonCredits.com
func (w *PriceUpdateWorker) fetchCarbonCreditsData(ctx context.Context) ([]*PriceData, error) {
	// Mock implementation - in real code, this would call CarbonCredits.com API
	log.Println("Fetching price data from CarbonCredits.com")

	return []*PriceData{
		{
			Source:       "carboncredits",
			Methodology:  "VM0007",
			Region:       "EU",
			VintageYear:  2024,
			Price:        16.20,
			Currency:     "EUR",
			Volume:       800.0,
			Timestamp:    time.Now(),
			QualityScore: 0.75,
		},
		{
			Source:       "carboncredits",
			Methodology:  "VM0033",
			Region:       "EU",
			VintageYear:  2024,
			Price:        14.80,
			Currency:     "EUR",
			Volume:       300.0,
			Timestamp:    time.Now(),
			QualityScore: 0.70,
		},
	}, nil
}

// fetchVerraData fetches price data from Verra registry
func (w *PriceUpdateWorker) fetchVerraData(ctx context.Context) ([]*PriceData, error) {
	// Mock implementation - in real code, this would call Verra API
	log.Println("Fetching price data from Verra registry")

	return []*PriceData{
		{
			Source:       "verra",
			Methodology:  "VM0007",
			Region:       "AF",
			VintageYear:  2024,
			Price:        15.75,
			Currency:     "USD",
			Volume:       600.0,
			Timestamp:    time.Now(),
			QualityScore: 0.82,
		},
		{
			Source:       "verra",
			Methodology:  "VM0015",
			Region:       "AF",
			VintageYear:  2024,
			Price:        14.50,
			Currency:     "USD",
			Volume:       400.0,
			Timestamp:    time.Now(),
			QualityScore: 0.78,
		},
	}, nil
}

// updatePricingModel updates a pricing model with new price data
func (w *PriceUpdateWorker) updatePricingModel(ctx context.Context, price *PriceData) error {
	// Check if model exists
	model, err := w.repository.GetPricingModel(ctx, price.Methodology, price.Region, price.VintageYear)
	if err != nil {
		// Create new model if it doesn't exist
		model = &financing.CreditPricingModel{
			MethodologyCode: price.Methodology,
			RegionCode:     &price.Region,
			VintageYear:    &price.VintageYear,
			BasePrice:      price.Price,
			QualityMultiplier: financing.QualityMultiplier{
				"source_quality": price.QualityScore,
			},
			MarketMultiplier: 1.0,
			ValidFrom:      price.Timestamp,
			ValidUntil:     func() *time.Time { t := price.Timestamp.AddDate(0, 1, 0); return &t }(),
			IsActive:      true,
			CreatedAt:      time.Now(),
		}

		return w.repository.CreatePricingModel(ctx, model)
	}

	// Update existing model
	model.BasePrice = price.Price
	model.QualityMultiplier = financing.QualityMultiplier{
		"source_quality": price.QualityScore,
	}
	model.ValidFrom = price.Timestamp
	model.ValidUntil = func() *time.Time { t := price.Timestamp.AddDate(0, 1, 0); return &t }()
	model.IsActive = true
	model.CreatedAt = time.Now()

	return w.repository.UpdatePricingModel(ctx, model)
}

// loadConfig loads configuration from file
func loadConfig(filename string) (*PriceUpdateWorkerConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config PriceUpdateWorkerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.PollInterval == 0 {
		config.PollInterval = 1 * time.Hour
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if len(config.Sources) == 0 {
		config.Sources = []string{"cme", "carboncredits", "verra"}
	}
	if config.UpdateInterval == 0 {
		config.UpdateInterval = 24 * time.Hour
	}

	return &config, nil
}

// PriceData represents price information from external sources
type PriceData struct {
	Source       string    `json:"source"`
	Methodology  string    `json:"methodology"`
	Region       string    `json:"region"`
	VintageYear  int       `json:"vintage_year"`
	Price        float64   `json:"price"`
	Currency     string    `json:"currency"`
	Volume       float64   `json:"volume"`
	Timestamp    time.Time `json:"timestamp"`
	QualityScore float64   `json:"quality_score"`
}
