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
	App          App          `koanf:"app"`
	Server       Server       `koanf:"server"`
	S3Source     S3Source     `koanf:"s3_source"`
	S3Dest       S3Dest       `koanf:"s3_destination"`
	PostgreSQL   PostgreSQL   `koanf:"postgresql"`
	Processing   Processing   `koanf:"processing"`
	Pipeline     Pipeline     `koanf:"pipeline"`
	Monitoring   Monitoring   `koanf:"monitoring"`
	Secrets      Secrets      `koanf:"secrets"`
	Housekeeping Housekeeping `koanf:"housekeeping"`
	Dev          bool         `koanf:"dev"`
}

// App represents application-level configuration
type App struct {
	Name       string `koanf:"name"`
	Version    string `koanf:"version"`
	InstanceID string `koanf:"instance_id"`
	LogLevel   string `koanf:"log_level"`
}

// Server represents server configuration following receiver pattern
type Server struct {
	ApiPort int `koanf:"apiport"`
}

// S3Source represents configuration for reading raw data from S3
type S3Source struct {
	BucketName   string        `koanf:"bucket_name"`
	Region       string        `koanf:"region"`
	Prefix       string        `koanf:"prefix"`
	PollInterval time.Duration `koanf:"poll_interval"`
	AccessKey    string        `koanf:"access_key"`
	SecretKey    string        `koanf:"secret_key"`
	SecretName   string        `koanf:"secret_name"`
	Endpoint     string        `koanf:"endpoint"`
	SSL          bool          `koanf:"ssl"`
}

// S3Dest represents configuration for writing processed data to S3
type S3Dest struct {
	BucketName string `koanf:"bucket_name"`
	Region     string `koanf:"region"`
	Prefix     string `koanf:"prefix"`
	AccessKey  string `koanf:"access_key"`
	SecretKey  string `koanf:"secret_key"`
	SecretName string `koanf:"secret_name"`
	Endpoint   string `koanf:"endpoint"`
	SSL        bool   `koanf:"ssl"`
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
}

// Processing represents processing engine configuration
type Processing struct {
	MaxConcurrentJobs int           `koanf:"max_concurrent_jobs"`
	JobTimeout        time.Duration `koanf:"job_timeout"`
	RetryAttempts     int           `koanf:"retry_attempts"`
	RetryBackoff      string        `koanf:"retry_backoff"`
	BufferSize        int           `koanf:"buffer_size"`
}

// Pipeline represents pipeline-specific configuration
type Pipeline struct {
	ControllerEndpoint    string        `koanf:"controller_endpoint"`
	ConfigRefreshInterval time.Duration `koanf:"config_refresh_interval"`
	GeoIPDatabasePath     string        `koanf:"geoip_database_path"`
	GeoIPCityDatabase     string        `koanf:"geoip_city_database"`
	GeoIPCountryDatabase  string        `koanf:"geoip_country_database"`
	EnableGeoIP           bool          `koanf:"enable_geoip"`
}

// Monitoring represents monitoring and observability configuration
type Monitoring struct {
	MetricsPort     int    `koanf:"metrics_port"`
	HealthPort      int    `koanf:"health_port"`
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
	Enabled        bool          `koanf:"enabled"`
	IntervalSeconds int           `koanf:"intervalseconds"`
	Interval       time.Duration // Calculated from IntervalSeconds
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

	return &config, nil
}

// generateInstanceID generates a dynamic instance ID based on IP address with fallbacks
func generateInstanceID() string {
	// Try to get the first non-loopback IP address
	interfaces, err := net.Interfaces()
	if err == nil {
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

				// Format IP as piper-192-168-1-100
				ipStr := ip.String()
				ipFormatted := strings.ReplaceAll(ipStr, ".", "-")
				return fmt.Sprintf("piper-%s", ipFormatted)
			}
		}
	}

	// Fallback to hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return fmt.Sprintf("piper-%s", hostname)
	}

	// Final fallback to static default
	return "piper-default"
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

		"s3_source.prefix":        "",
		"s3_source.poll_interval": "30s",
		"s3_source.ssl":           true,

		"s3_destination.prefix": "",
		"s3_destination.ssl":    true,

		"processing.max_concurrent_jobs": 10,
		"processing.job_timeout":         "30m",
		"processing.retry_attempts":      3,
		"processing.retry_backoff":       "exponential",
		"processing.buffer_size":         1000,

		"pipeline.config_refresh_interval": "5m",
		"pipeline.geoip_database_path":     "/opt/geoip",
		"pipeline.geoip_city_database":     "GeoLite2-City.mmdb",
		"pipeline.geoip_country_database":  "GeoLite2-Country.mmdb",
		"pipeline.enable_geoip":            true,

		"monitoring.metrics_port":   9090,
		"monitoring.health_port":    8080,
		"monitoring.log_level":      "info",
		"monitoring.enable_tracing": false,

		"secrets.provider": "aws",

		"housekeeping.enabled":        true,
		"housekeeping.intervalseconds": 600,
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

	if config.Monitoring.HealthPort <= 0 || config.Monitoring.HealthPort > 65535 {
		return fmt.Errorf("monitoring.health_port must be between 1 and 65535")
	}

	return nil
}
