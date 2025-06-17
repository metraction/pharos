package model

// ScannerConfig holds scanner-specific configuration.
type ScannerConfig struct {
	StreamName string `mapstructure:"stream-name"`
}

// PublisherConfig holds publisher-specific configuration.
type PublisherConfig struct {
	StreamName string `mapstructure:"stream-name"`
}

// Config holds the application configuration.
type Config struct {
	Redis     Redis           `mapstructure:"redis"`
	Scanner   ScannerConfig   `mapstructure:"scanner"`
	Publisher PublisherConfig `mapstructure:"publisher"`
	Database  Database        `mapstructure:"database"`
}

// Redis holds Redis-specific configuration.
type Redis struct {
	DSN  string `mapstructure:"dsn"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DatabaseDriver string

const (
	DatabaseDriverPostgres DatabaseDriver = "postgres"
)

type Database struct {
	Driver DatabaseDriver `mapstructure:"driver"` // e.g., "sqlite" or "postgres"
	Dsn    string         `mapstructure:"dsn"`
}
