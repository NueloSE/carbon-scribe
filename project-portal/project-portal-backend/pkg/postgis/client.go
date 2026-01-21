package postgis

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Client wraps a GORM database connection with PostGIS support
type Client struct {
	DB *gorm.DB
}

// Config holds PostGIS client configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	LogLevel        logger.LogLevel
}

// NewClient creates a new PostGIS client
func NewClient(config *Config) (*Client, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(config.LogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)

	// Verify PostGIS is installed
	if err := verifyPostGIS(db); err != nil {
		return nil, fmt.Errorf("PostGIS verification failed: %w", err)
	}

	return &Client{DB: db}, nil
}

// verifyPostGIS checks if PostGIS extension is installed
func verifyPostGIS(db *gorm.DB) error {
	var version string
	result := db.Raw("SELECT PostGIS_Version()").Scan(&version)
	if result.Error != nil {
		return fmt.Errorf("PostGIS not installed or not accessible: %w", result.Error)
	}
	return nil
}

// Close closes the database connection
func (c *Client) Close() error {
	sqlDB, err := c.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Health checks the database connection health
func (c *Client) Health() error {
	sqlDB, err := c.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
