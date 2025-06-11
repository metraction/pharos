package model

<<<<<<< HEAD
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
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DatabaseDriver string

const (
	DatabaseDriverSqlite   DatabaseDriver = "sqlite"
	DatabaseDriverPostgres DatabaseDriver = "postgres"
)

type Database struct {
	Driver DatabaseDriver `mapstructure:"driver"` // e.g., "sqlite" or "postgres"
	Dsn    string         `mapstructure:"dsn"`
<<<<<<< HEAD
=======
type Config struct {
	Redis Redis
}

type Redis struct {
	Port int
>>>>>>> c457fd0 (Subscriber implemented)
=======
>>>>>>> 539dff5 (add database)
}
