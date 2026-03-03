// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package config

import (
	"fmt"
	"os"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	yamlv3 "gopkg.in/yaml.v3"
)

// Config represents the main configuration structure for bytefreezer-piper
type Config struct {
	App              App              `koanf:"app"`
	Server           Server           `koanf:"server"`
	S3Source         S3Source         `koanf:"s3_source"`
	S3Dest           S3Dest           `koanf:"s3_destination"`
	S3GeoIP          S3GeoIP          `koanf:"s3_geoip"`
	Processing       Processing       `koanf:"processing"`
	Pipeline         Pipeline         `koanf:"pipeline"`
	ControlService   ControlService   `koanf:"control_service"`
	Monitoring       Monitoring       `koanf:"monitoring"`
	Housekeeping     Housekeeping     `koanf:"housekeeping"`
	DLQ              DLQ              `koanf:"dlq"`
	SOC              SOC              `koanf:"soc"`
	FailureThreshold FailureThreshold `koanf:"failure_threshold"`
	HealthReporting  HealthReporting  `koanf:"health_reporting"`
	ErrorTracking    ErrorTracking    `koanf:"error_tracking"`
	Dev              bool             `koanf:"dev"`
}

// App represents application-level configuration
type App struct {
	Name           string `koanf:"name"`
	Version        string `koanf:"version"`
	InstanceID     string `koanf:"instance_id"`
	LogLevel       string `koanf:"log_level"`
	Dev            bool   `koanf:"dev"`
	DeploymentType string `koanf:"deployment_type"` // "managed" or "on_prem"
}

// Server represents server configuration following receiver pattern
type Server struct {
	ApiPort int `koanf:"api_port"`
}

// S3Source represents configuration for reading raw data from S3
type S3Source struct {
	BucketName   string        `koanf:"bucket_name"`
	Region       string        `koanf:"region"`
	PollInterval time.Duration `koanf:"poll_interval"`
	AccessKey    string        `koanf:"access_key"`
	SecretKey    string        `koanf:"secret_key"`
	SecretName   string        `koanf:"secret_name"`
	Endpoint     string        `koanf:"endpoint"`
	SSL          bool          `koanf:"ssl"`
	UseIamRole   bool          `koanf:"use_iam_role"`
}

// S3Dest represents configuration for writing processed data to S3
type S3Dest struct {
	BucketName string `koanf:"bucket_name"`
	Region     string `koanf:"region"`
	AccessKey  string `koanf:"access_key"`
	SecretKey  string `koanf:"secret_key"`
	SecretName string `koanf:"secret_name"`
	Endpoint   string `koanf:"endpoint"`
	SSL        bool   `koanf:"ssl"`
	UseIamRole bool   `koanf:"use_iam_role"`
}

// S3GeoIP represents configuration for GeoIP database updates from S3
type S3GeoIP struct {
	BucketName string `koanf:"bucket_name"`
	Region     string `koanf:"region"`
	AccessKey  string `koanf:"access_key"`
	SecretKey  string `koanf:"secret_key"`
	SecretName string `koanf:"secret_name"`
	Endpoint   string `koanf:"endpoint"`
	SSL        bool   `koanf:"ssl"`
	UseIamRole bool   `koanf:"use_iam_role"`
}

// Processing represents processing engine configuration
type Processing struct {
	MaxConcurrentJobs int           `koanf:"max_concurrent_jobs"`
	JobTimeoutSeconds int           `koanf:"job_timeout_seconds"`
	JobTimeout        time.Duration // Calculated from JobTimeoutSeconds
	RetryAttempts     int           `koanf:"retry_attempts"`
	RetryBackoff      string        `koanf:"retry_backoff"`
	BufferSize        int           `koanf:"buffer_size"`
}

// Pipeline represents pipeline-specific configuration
type Pipeline struct {
	ConfigRefreshInterval time.Duration `koanf:"config_refresh_interval"`
	GeoIPDatabasePath     string        `koanf:"geoip_database_path"`
	GeoIPCityDatabase     string        `koanf:"geoip_city_database"`
	GeoIPCountryDatabase  string        `koanf:"geoip_country_database"`
	EnableGeoIP           bool          `koanf:"enable_geoip"`
}

// ControlService represents Control Service configuration
type ControlService struct {
	Enabled        bool   `koanf:"enabled"`
	ControlURL     string `koanf:"control_url"`
	APIKey         string `koanf:"api_key"`
	TimeoutSeconds int    `koanf:"timeout_seconds"`
	AccountID      string `koanf:"account_id"` // Optional: for on-prem deployments, only process this account
}

// Monitoring represents monitoring and observability configuration
type Monitoring struct {
	Enabled             bool   `koanf:"enabled"`
	Mode                string `koanf:"mode"` // "prometheus" (pull), "otlp_http" (push to Prometheus), "otlp_grpc" (push to collector)
	OTLPEndpoint        string `koanf:"otlp_endpoint"`
	PushIntervalSeconds int    `koanf:"push_interval_seconds"`
	MetricsHost         string `koanf:"metrics_host"`
	MetricsPort         int    `koanf:"metrics_port"`
	ServiceName         string `koanf:"service_name"`
	LogLevel            string `koanf:"log_level"`
	EnableTracing       bool   `koanf:"enable_tracing"`
	TracingEndpoint     string `koanf:"tracing_endpoint"`
}

// Housekeeping represents housekeeping configuration
type Housekeeping struct {
	Enabled         bool          `koanf:"enabled"`
	IntervalSeconds int           `koanf:"intervalseconds"`
	Interval        time.Duration // Calculated from IntervalSeconds
}

// DLQ represents dead letter queue configuration following receiver pattern
type DLQ struct {
	Enabled                bool          `koanf:"enabled"`
	RetryAttempts          int           `koanf:"retry_attempts"`
	RetryIntervalSeconds   int           `koanf:"retry_interval_seconds"`
	CleanupIntervalSeconds int           `koanf:"cleanup_interval_seconds"`
	MaxAgeDays             int           `koanf:"max_age_days"`
	RetryInterval          time.Duration // Calculated from RetryIntervalSeconds
	CleanupInterval        time.Duration // Calculated from CleanupIntervalSeconds
}

// SOC represents SOC alerting configuration
type SOC struct {
	Enabled  bool   `koanf:"enabled"`
	Endpoint string `koanf:"endpoint"`
	Timeout  int    `koanf:"timeout"`
}

// FailureThreshold represents failure monitoring configuration
type FailureThreshold struct {
	Enabled          bool    `koanf:"enabled"`
	FailureThreshold float64 `koanf:"failure_threshold"`
	MinimumSamples   int     `koanf:"minimum_samples"`
	WindowSize       int     `koanf:"window_size"`
	CheckInterval    string  `koanf:"check_interval"`
	CooldownPeriod   string  `koanf:"cooldown_period"`
}

// HealthReporting represents health reporting configuration
// Note: Uses control_service.base_url for the control service endpoint
type HealthReporting struct {
	Enabled           bool `koanf:"enabled"`
	ReportInterval    int  `koanf:"report_interval"` // Interval in seconds
	TimeoutSeconds    int  `koanf:"timeout_seconds"`
	RegisterOnStartup bool `koanf:"register_on_startup"`
}

// ErrorTracking represents error tracking configuration
// Note: Uses control_service.base_url for the control service endpoint
type ErrorTracking struct {
	Enabled bool `koanf:"enabled"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	k := koanf.New(".")

	// Load default values
	defaultsYAML, err := yamlv3.Marshal(getDefaults())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal defaults: %w", err)
	}
	if err := k.Load(rawbytes.Provider(defaultsYAML), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	// Load from config file if provided
	if configPath != "" {
		if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
		}
	}

	// Load from environment variables (highest priority)
	if err := k.Load(env.Provider("PIPER_", ".", func(s string) string {
		return s[6:] // Remove PIPER_ prefix
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Unmarshal into config struct
	var config Config
	if err := k.Unmarshal("", &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Handle instance ID if not set in config
	if config.App.InstanceID == "" {
		config.App.InstanceID = generateInstanceID()
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Calculate derived fields
	config.Housekeeping.Interval = time.Duration(config.Housekeeping.IntervalSeconds) * time.Second
	config.DLQ.RetryInterval = time.Duration(config.DLQ.RetryIntervalSeconds) * time.Second
	config.DLQ.CleanupInterval = time.Duration(config.DLQ.CleanupIntervalSeconds) * time.Second
	config.Processing.JobTimeout = time.Duration(config.Processing.JobTimeoutSeconds) * time.Second

	return &config, nil
}

// isDockerContainer returns true if running inside a Docker container.
func isDockerContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// generateInstanceID creates a stable identifier for this service instance.
// Docker: host:containerID (uses HOST_HOSTNAME env var for host).
// Kubernetes: node.pod format (uses NODE_NAME env var).
// Bare metal: hostname.
func generateInstanceID() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "piper-unknown"
	}
	if isDockerContainer() {
		if hostHostname := os.Getenv("HOST_HOSTNAME"); hostHostname != "" {
			hostname = fmt.Sprintf("%s:%s", hostHostname, hostname)
		}
	}
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" && nodeName != hostname {
		hostname = fmt.Sprintf("%s.%s", nodeName, hostname)
	}
	return hostname
}

// getDefaults returns a map of default configuration values
func getDefaults() map[string]interface{} {
	// Generate dynamic instance ID based on IP address with fallbacks
	instanceID := generateInstanceID()

	return map[string]interface{}{
		"app.name":        "bytefreezer-piper",
		"app.version":     "1.0.0",
		"app.instance_id": instanceID,
		"app.log_level":   "info",

		"s3_source.poll_interval": "30s",
		"s3_source.ssl":           true,

		"s3_destination.ssl": true,

		"s3_geoip.bucket_name": "geoip",
		"s3_geoip.region":      "us-east-1",
		"s3_geoip.endpoint":    "192.168.86.125:9000",
		"s3_geoip.ssl":         false,

		"processing.max_concurrent_jobs": 10,
		"processing.job_timeout_seconds": 600, // 10 minutes
		"processing.retry_attempts":      3,
		"processing.retry_backoff":       "exponential",
		"processing.buffer_size":         1000,

		"pipeline.config_refresh_interval": "5m",
		"pipeline.geoip_database_path":     "/opt/geoip",
		"pipeline.geoip_city_database":     "GeoLite2-City.mmdb",
		"pipeline.geoip_country_database":  "GeoLite2-Country.mmdb",
		"pipeline.enable_geoip":            false,

		"control_service.enabled":         false,
		"control_service.base_url":        "",
		"control_service.api_key":         "",
		"control_service.timeout_seconds": 30,

		"monitoring.enabled":               true,
		"monitoring.mode":                  "prometheus",
		"monitoring.otlp_endpoint":         "",
		"monitoring.push_interval_seconds": 15,
		"monitoring.metrics_host":          "0.0.0.0",
		"monitoring.metrics_port":          9092,
		"monitoring.service_name":          "bytefreezer-piper",
		"monitoring.log_level":             "info",
		"monitoring.enable_tracing":        false,

		"secrets.provider": "aws",

		"housekeeping.enabled":         true,
		"housekeeping.intervalseconds": 600,

		"dlq.enabled":                  true,
		"dlq.retry_attempts":           4,
		"dlq.retry_interval_seconds":   60,
		"dlq.cleanup_interval_seconds": 3600,
		"dlq.max_age_days":             7,

		"soc.enabled":  false,
		"soc.endpoint": "http://bytefreezer-soc:8080/api/v1/alerts",
		"soc.timeout":  30,

		"failure_threshold.enabled":           false,
		"failure_threshold.failure_threshold": 20.0,
		"failure_threshold.minimum_samples":   10,
		"failure_threshold.window_size":       100,
		"failure_threshold.check_interval":    "5m",
		"failure_threshold.cooldown_period":   "30m",

		"health_reporting.enabled":             false,
		"health_reporting.report_interval":     30,
		"health_reporting.timeout_seconds":     10,
		"health_reporting.register_on_startup": true,

		"error_tracking.enabled": true,

		"app.dev": false,
	}
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.S3Source.BucketName == "" {
		return fmt.Errorf("s3_source.bucket_name is required")
	}

	if config.S3Dest.BucketName == "" {
		return fmt.Errorf("s3_destination.bucket_name is required")
	}

	if config.Processing.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("processing.max_concurrent_jobs must be greater than 0")
	}

	if config.Processing.RetryAttempts < 0 {
		return fmt.Errorf("processing.retry_attempts must be non-negative")
	}

	if config.Monitoring.MetricsPort <= 0 || config.Monitoring.MetricsPort > 65535 {
		return fmt.Errorf("monitoring.metrics_port must be between 1 and 65535")
	}

	return nil
}
