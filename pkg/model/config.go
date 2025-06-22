package model

// ScannerConfig holds scanner-specific configuration.
type ScannerConfig struct {
	RequestQueue          string `mapstructure:"requestQueue"`
	PriorityRequestQueue  string `mapstructure:"priorityRequestQueue"`
	ResponseQueue         string `mapstructure:"responseQueue"`
	PriorityResponseQueue string `mapstructure:"priorityResponseQueue"`
	Timeout               string `mapstructure:"timeout"`
	CacheExpiry           string `mapstructure:"cacheExpiry"`
	CacheEndpoint         string `mapstructure:"cacheEndpoint"`
	Engine                string `mapstructure:"engine"`
}

// PublisherConfig holds publisher-specific configuration.
type PublisherConfig struct {
	RequestQueue          string `mapstructure:"requestQueue"`
	PriorityRequestQueue  string `mapstructure:"priorityRequestQueue"`
	ResponseQueue         string `mapstructure:"responseQueue"`
	PriorityResponseQueue string `mapstructure:"priorityResponseQueue"`
	Timeout               string `mapstructure:"timeout"`
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
	DSN string `mapstructure:"dsn"`
}

type DatabaseDriver string

const (
	DatabaseDriverPostgres DatabaseDriver = "postgres"
)

type Database struct {
	Driver DatabaseDriver `mapstructure:"driver"` // "postgres"
	Dsn    string         `mapstructure:"dsn"`
}
