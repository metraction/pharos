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
}

// Redis holds Redis-specific configuration.
type Redis struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}
