package config

import (
	"fmt"
	"net"
	"os"
	"strings"
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
	Secrets          Secrets          `koanf:"secrets"`
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
	Name       string `koanf:"name"`
	Version    string `koanf:"version"`
	InstanceID string `koanf:"instance_id"`
	LogLevel   string `koanf:"log_level"`
	Dev        bool   `koanf:"dev"`
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

// PostgreSQL represents PostgreSQL configuration for state management
type PostgreSQL struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Database string `koanf:"database"`
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	SSLMode  string `koanf:"ssl_mode"`
	Schema   string `koanf:"schema"`
	// Connection pool settings
	MaxOpenConns    int           `koanf:"max_open_conns"`     // Maximum number of open connections (0 = unlimited)
	MaxIdleConns    int           `koanf:"max_idle_conns"`     // Maximum number of idle connections
	ConnMaxLifetime time.Duration `koanf:"conn_max_lifetime"`  // Maximum connection lifetime
	ConnMaxIdleTime time.Duration `koanf:"conn_max_idle_time"` // Maximum connection idle time
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
	BaseURL        string `koanf:"base_url"`
	APIKey         string `koanf:"api_key"`
	TimeoutSeconds int    `koanf:"timeout_seconds"`
	AccountID      string `koanf:"account_id"` // Optional: for on-prem deployments, only process this account
}

// Monitoring represents monitoring and observability configuration
type Monitoring struct {
	MetricsPort     int    `koanf:"metrics_port"`
	LogLevel        string `koanf:"log_level"`
	EnableTracing   bool   `koanf:"enable_tracing"`
	TracingEndpoint string `koanf:"tracing_endpoint"`
}

// Secrets represents secrets management configuration
type Secrets struct {
	Provider string `koanf:"provider"` // "aws", "env", "file"
	Region   string `koanf:"region"`
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
	Enabled           bool   `koanf:"enabled"`
	ReportInterval    int    `koanf:"report_interval"` // Interval in seconds
	TimeoutSeconds    int    `koanf:"timeout_seconds"`
	RegisterOnStartup bool   `koanf:"register_on_startup"`
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

// generateInstanceID generates a unique instance ID based on IP address, PID, and timestamp
func generateInstanceID() string {
	pid := os.Getpid()
	timestamp := time.Now().Unix()

	// Try to get the first non-loopback IP address
	if ip := getFirstNonLoopbackIP(); ip != "" {
		ipFormatted := strings.ReplaceAll(ip, ".", "-")
		return fmt.Sprintf("piper-%s-%d-%d", ipFormatted, pid, timestamp)
	}

	// Fallback to hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return fmt.Sprintf("piper-%s-%d-%d", hostname, pid, timestamp)
	}

	// Final fallback to static default with unique suffix
	return fmt.Sprintf("piper-default-%d-%d", pid, timestamp)
}

// getFirstNonLoopbackIP extracts IP address logic for reuse
func getFirstNonLoopbackIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			return ip.String()
		}
	}
	return ""
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

		// PostgreSQL connection pool defaults
		"postgresql.max_open_conns":     25,      // Enough for 10 workers + overhead
		"postgresql.max_idle_conns":     10,      // Match concurrent job count
		"postgresql.conn_max_lifetime":  "5m",    // Rotate connections every 5 minutes
		"postgresql.conn_max_idle_time": "5m",    // Close idle connections after 5 minutes

		"processing.max_concurrent_jobs":  10,
		"processing.job_timeout_seconds":  600,  // 10 minutes
		"processing.retry_attempts":       3,
		"processing.retry_backoff":        "exponential",
		"processing.buffer_size":          1000,

		"pipeline.config_refresh_interval": "5m",
		"pipeline.geoip_database_path":     "/opt/geoip",
		"pipeline.geoip_city_database":     "GeoLite2-City.mmdb",
		"pipeline.geoip_country_database":  "GeoLite2-Country.mmdb",
		"pipeline.enable_geoip":            false,

		"control_service.enabled":         false,
		"control_service.base_url":        "",
		"control_service.api_key":         "",
		"control_service.timeout_seconds": 30,

		"monitoring.metrics_port":   9090,
		"monitoring.log_level":      "info",
		"monitoring.enable_tracing": false,

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

		"health_reporting.enabled":              false,
		"health_reporting.report_interval":      30,
		"health_reporting.timeout_seconds":      10,
		"health_reporting.register_on_startup":  true,

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
