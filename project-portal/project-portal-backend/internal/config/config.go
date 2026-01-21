package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Maps     MapsConfig
	Logging  LoggingConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Host            string
	Port            int
	Mode            string // "development", "production"
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// MapsConfig holds mapping API configuration
type MapsConfig struct {
	MapboxAccessToken     string
	MapboxStyleURL        string
	GoogleMapsAPIKey      string
	DefaultMapProvider    string // "mapbox" or "google"
	TileCacheTTL          time.Duration
	MaxTileCacheSize      int64 // in bytes
	StaticMapWidth        int
	StaticMapHeight       int
	DefaultZoomLevel      int
	MaxConcurrentRequests int
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string // "debug", "info", "warn", "error"
	Format     string // "json", "console"
	OutputPath string
}

// Load loads configuration from environment variables and config file
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found is ok, we'll use env vars and defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config

	// Server configuration
	config.Server = ServerConfig{
		Host:            viper.GetString("server.host"),
		Port:            viper.GetInt("server.port"),
		Mode:            viper.GetString("server.mode"),
		ReadTimeout:     viper.GetDuration("server.read_timeout"),
		WriteTimeout:    viper.GetDuration("server.write_timeout"),
		ShutdownTimeout: viper.GetDuration("server.shutdown_timeout"),
	}

	// Database configuration
	config.Database = DatabaseConfig{
		Host:            viper.GetString("database.host"),
		Port:            viper.GetInt("database.port"),
		User:            viper.GetString("database.user"),
		Password:        viper.GetString("database.password"),
		DBName:          viper.GetString("database.dbname"),
		SSLMode:         viper.GetString("database.sslmode"),
		MaxOpenConns:    viper.GetInt("database.max_open_conns"),
		MaxIdleConns:    viper.GetInt("database.max_idle_conns"),
		ConnMaxLifetime: viper.GetDuration("database.conn_max_lifetime"),
	}

	// Maps configuration
	config.Maps = MapsConfig{
		MapboxAccessToken:     viper.GetString("maps.mapbox_access_token"),
		MapboxStyleURL:        viper.GetString("maps.mapbox_style_url"),
		GoogleMapsAPIKey:      viper.GetString("maps.google_maps_api_key"),
		DefaultMapProvider:    viper.GetString("maps.default_provider"),
		TileCacheTTL:          viper.GetDuration("maps.tile_cache_ttl"),
		MaxTileCacheSize:      viper.GetInt64("maps.max_tile_cache_size"),
		StaticMapWidth:        viper.GetInt("maps.static_map_width"),
		StaticMapHeight:       viper.GetInt("maps.static_map_height"),
		DefaultZoomLevel:      viper.GetInt("maps.default_zoom_level"),
		MaxConcurrentRequests: viper.GetInt("maps.max_concurrent_requests"),
	}

	// Logging configuration
	config.Logging = LoggingConfig{
		Level:      viper.GetString("logging.level"),
		Format:     viper.GetString("logging.format"),
		OutputPath: viper.GetString("logging.output_path"),
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "development")
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.write_timeout", 30*time.Second)
	viper.SetDefault("server.shutdown_timeout", 10*time.Second)

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.dbname", "carbonscribe_portal")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 5*time.Minute)

	// Maps defaults
	viper.SetDefault("maps.default_provider", "mapbox")
	viper.SetDefault("maps.tile_cache_ttl", 24*time.Hour)
	viper.SetDefault("maps.max_tile_cache_size", 1073741824) // 1GB
	viper.SetDefault("maps.static_map_width", 800)
	viper.SetDefault("maps.static_map_height", 600)
	viper.SetDefault("maps.default_zoom_level", 10)
	viper.SetDefault("maps.max_concurrent_requests", 10)
	viper.SetDefault("maps.mapbox_style_url", "mapbox://styles/mapbox/satellite-v9")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output_path", "stdout")
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
