package model

import "time"

// Config holds the application configuration.
type Config struct {
	Redis           Redis                    `mapstructure:"redis"`
	Scanner         ScannerConfig            `mapstructure:"scanner"`
	Publisher       PublisherConfig          `mapstructure:"publisher"`
	Database        Database                 `mapstructure:"database"`
	Prometheus      PrometheusReporterConfig `mapstructure:"prometheus"`
	ResultCollector ResultCollectorConfig    `mapstructure:"collector"`
}

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

type ResultCollectorConfig struct {
	QueueName    string        `mapstructure:"queueName"`
	GroupName    string        `mapstructure:"groupName"`
	ConsumerName string        `mapstructure:"consumerName"`
	BlockTimeout time.Duration `mapstructure:"blockTimeout"`
	MessageCount string        `mapstructure:"messageCount"`
}

type PrometheusReporterConfig struct {
	URL           string   `mapstructure:"url"`           // URL of the Prometheus server
	Interval      string   `mapstructure:"interval"`      // Interval for scraping Prometheus metrics
	Platform      string   `mapstructure:"platform"`      // Platform for which the metrics are collected, defaults to "linux/amd64"
	Namespace     string   `mapstructure:"namespace"`     // Namespace for the Prometheus metrics
	PharosURL     string   `mapstructure:"pharosUrl"`     // Root URL of the Pharos server for Prometheus metrics
	ContextLabels []string `mapstructure:"contextLabels"` // Labels to add to the Prometheus context
	TTL           string   `mapstructure:"ttl"`           // Time to live for the scan results
	Query         string   `mapstructure:"query"`         // Query to use for fetching metrics
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
