package model

import (
	"reflect"
	"regexp"
	"strings"
	"time"

	hwmodel "github.com/metraction/handwheel/model"
)

// Config holds the application configuration.
type Config struct {
	Redis           Redis                    `mapstructure:"redis"`
	Scanner         ScannerConfig            `mapstructure:"scanner"`
	Publisher       PublisherConfig          `mapstructure:"publisher"`
	Database        Database                 `mapstructure:"database"`
	Prometheus      PrometheusReporterConfig `mapstructure:"prometheus"`
	ResultCollector ResultCollectorConfig    `mapstructure:"collector"`
	Command         string                   `mapstructure:"command"`
	BasePath        string
	EnricherCommon  EnricherCommonConfig `mapstructure:"enricherCommon" yaml:"enricherCommon" json:"enricherCommon"`
	Alerting        AlertingConfig       `mapstructure:"alerting" yaml:"alerting" json:"alerting"`
	Init            bool                 `mapstructure:"init" yaml:"init" json:"init"` // If true, used as an init container to wait for dependencies to be ready
}

type EnricherCommonConfig struct {
	EnricherPath string `yaml:"enricherPath"`
	UiUrl        string `yaml:"uiUrl"`
}

// ObfuscateSensitiveData replaces passwords and tokens in the config with "***".
func (c *Config) ObfuscateSensitiveData() *Config {
	clone := *c
	obfuscateStruct(reflect.ValueOf(&clone).Elem())
	return &clone
}

func obfuscateStruct(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := v.Type().Field(i)
		if !field.CanSet() {
			continue
		}
		switch field.Kind() {
		case reflect.String:
			name := strings.ToLower(fieldType.Name)
			if strings.Contains(name, "password") || strings.Contains(name, "token") {
				field.SetString("***")
			}
			if strings.Contains(name, "dsn") {
				// Replace anything after // and before @ with *** using regex
				dsn := field.String()
				re := regexp.MustCompile(`//(.*):(.*)?@`)
				field.SetString(re.ReplaceAllString(dsn, "//$1:***@"))
			}
		case reflect.Struct:
			obfuscateStruct(field)
		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				obfuscateStruct(field.Elem())
			}
		}
	}
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
	QueueSize             int    `mapstructure:"queueSize"`
}

type ResultCollectorConfig struct {
	QueueName    string        `mapstructure:"queueName"`
	GroupName    string        `mapstructure:"groupName"`
	ConsumerName string        `mapstructure:"consumerName"`
	BlockTimeout time.Duration `mapstructure:"blockTimeout"`
	QueueSize    int           `mapstructure:"queueSize"`
}

type PrometheusReporterConfig struct {
	URL           string                 `mapstructure:"url"`           // URL of the Prometheus server
	Interval      string                 `mapstructure:"interval"`      // Interval for scraping Prometheus metrics
	Platform      string                 `mapstructure:"platform"`      // Platform for which the metrics are collected, defaults to "linux/amd64"
	Namespace     string                 `mapstructure:"namespace"`     // Namespace for the Prometheus metrics
	PharosURL     string                 `mapstructure:"pharosUrl"`     // Root URL of the Pharos server for Prometheus metrics
	ContextLabels []string               `mapstructure:"contextLabels"` // Labels to add to the Prometheus context
	TTL           string                 `mapstructure:"ttl"`           // Time to live for the scan results
	Query         string                 `mapstructure:"query"`         // Query to use for fetching metrics
	Auth          hwmodel.PrometheusAuth `mapstructure:"auth"`          // Authentication details for Prometheus
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

/*
 */
type EnrichersConfig struct {
	Order   []string         `mapstructure:"order" yaml:"order" json:"order"`
	Sources []EnricherSource `mapstructure:"sources" yaml:"sources" json:"sources"`
}

type EnricherSource struct {
	Name string `mapstructure:"name" yaml:"name" json:"name"`
	Path string `mapstructure:"path" yaml:"path" json:"path"`
	// Git is a pointer to string to allow it to be nil
	Git *string `mapstructure:"git" yaml:"git,omitempty" json:"git,omitempty"`
	// ID is the ID of the enricher in the database, optional, can be null if not stored in database
	ID *string `mapstructure:"id" yaml:"id,omitempty" json:"id,omitempty"`
}

/*
Enrichers could be loaded from different sources: filesysem, git.
They are not part of Config structure.
*/
type EnricherConfig struct {
	BasePath string         `yaml:"basePath"`
	Configs  []MapperConfig `yaml:"configs"`
	Enricher *Enricher      `yaml:"enricher"` // Enricher configuration if loaded from database
}

type MapperConfig struct {
	Name   string `yaml:"name"`
	Config string `yaml:"config"`
	Ref    string `yaml:"ref"` // Used in NewAppendFile to set ref name in template
}

type AlertingConfig struct {
	Route     RouteConfig      `mapstructure:"route" yaml:"route" json:"route" doc:"Root Route for alerts"`
	Receivers []ReceiverConfig `mapstructure:"receivers" yaml:"receivers" json:"receivers" doc:"List of receivers for alerts"`
}

type RouteConfig struct {
	Receiver       string        `mapstructure:"receiver" yaml:"receiver" json:"receiver"`
	GroupBy        []string      `mapstructure:"group_by" yaml:"group_by" json:"group_by"`
	Continue       bool          `mapstructure:"continue" yaml:"continue,omitempty" json:"continue,omitempty"`
	Matchers       []string      `mapstructure:"matchers,omitempty" yaml:"matchers,omitempty" json:"matchers,omitempty"`
	GroupWait      string        `mapstructure:"group_wait,omitempty" yaml:"group_wait,omitempty" json:"group_wait,omitempty"`
	GroupInterval  string        `mapstructure:"group_interval,omitempty" yaml:"group_interval,omitempty" json:"group_interval,omitempty" default:"5m"`
	RepeatInterval string        `mapstructure:"repeat_interval,omitempty" yaml:"repeat_interval,omitempty" json:"repeat_interval,omitempty" default:"4h"`
	ChildRoutes    []RouteConfig `mapstructure:"child_routes,omitempty" yaml:"child_routes,omitempty" json:"child_routes,omitempty"`
}

type ReceiverConfig struct {
	Name           string          `mapstructure:"name" yaml:"name" json:"name"`
	WebhookConfigs []WebhookConfig `mapstructure:"webhook_configs" yaml:"webhook_configs" json:"webhook_configs"`
}

type WebhookConfig struct {
	// Whether to notify about resolved alerts.
	SendResolved bool `mapstructure:"send_resolved" yaml:"send_resolved" json:"send_resolved"`

	// The endpoint to send HTTP POST requests to.
	// url and url_file are mutually exclusive.
	URL     string `mapstructure:"url" yaml:"url" json:"url"`
	URLFile string `mapstructure:"url_file" yaml:"url_file" json:"url_file"`

	// The maximum number of alerts to include in a single webhook message.
	MaxAlerts int `mapstructure:"max_alerts" yaml:"max_alerts" json:"max_alerts"`

	// The maximum time to wait for a webhook request to complete.
	Timeout string `mapstructure:"timeout" yaml:"timeout" json:"timeout"`

	// The HTTP client's configuration.
	HTTPConfig *AlertingHttpConfig `mapstructure:"http_config" yaml:"http_config" json:"http_config,omitempty"`
}

type AlertingHttpConfig struct {
	BasicAuth     *AlertingBasicAuthConfig     `mapstructure:"basic_auth" yaml:"basic_auth" json:"basic_auth,omitempty"`
	Authorization *AlertingAuthorizationConfig `mapstructure:"authorization" yaml:"authorization" json:"authorization,omitempty"`
}

type AlertingBasicAuthConfig struct {
	Username     string `mapstructure:"username" yaml:"username" json:"username"`
	Password     string `mapstructure:"password" yaml:"password" json:"password"`
	PasswordFile string `mapstructure:"password_file" yaml:"password_file" json:"password_file"`
}

type AlertingAuthorizationConfig struct {
	Type        string `mapstructure:"type" yaml:"type" json:"type"` // default: Bearer
	Credentials string `mapstructure:"credentials" yaml:"credentials" json:"credentials"`
}
