package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/n0needt0/go-goodies/log"
	"go.uber.org/zap"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/metrics"
	"github.com/n0needt0/bytefreezer-piper/pipeline"
	"github.com/n0needt0/bytefreezer-piper/storage"
	"github.com/n0needt0/bytefreezer-piper/tracking"
)

// Services encapsulates all service dependencies following receiver pattern
type Services struct {
	Config               *config.Config
	PiperService         *PiperService
	PipelineDatabase     *pipeline.PipelineDatabase
	StateManager         *storage.PostgreSQLStateManager
	HealthReporter       *HealthReportingService
	DatasetMetricsClient *metrics.DatasetMetricsClient
	ErrorTracker         *tracking.ErrorTracker
}

// NewServices creates and initializes all services
func NewServices(conf *config.Config) *Services {
	// Create PostgreSQL state manager
	stateManager, err := storage.NewPostgreSQLStateManager(&conf.PostgreSQL)
	if err != nil {
		// Continue without state manager for development
		log.Warnf("Failed to create state manager: %v - continuing without database", err)
	}

	// Create pipeline client
	pipelineClient := pipeline.NewPipelineClient(conf)

	// Create pipeline database
	pipelineDatabase := pipeline.NewPipelineDatabase(pipelineClient, stateManager, conf.App.InstanceID)

	// Create dataset metrics client
	datasetMetricsClient := metrics.NewDatasetMetricsClient(
		conf.ControlService.BaseURL,
		conf.ControlService.TimeoutSeconds,
		conf.ControlService.Enabled,
	)
	log.Infof("Dataset metrics client initialized (enabled: %v, endpoint: %s)",
		conf.ControlService.Enabled, conf.ControlService.BaseURL)

	// Create error tracker if enabled
	var errorTracker *tracking.ErrorTracker
	if conf.ErrorTracking.Enabled && conf.ControlService.Enabled {
		// Create a basic zap logger for error tracker
		zapLogger, _ := zap.NewProduction()
		errorTracker = tracking.NewErrorTracker(
			conf.ControlService.BaseURL,
			conf.ControlService.APIKey,
			"bytefreezer-piper",
			zapLogger,
		)
		log.Infof("Error tracker initialized (endpoint: %s)", conf.ControlService.BaseURL)
	} else {
		log.Info("Error tracking disabled")
	}

	// Create piper service
	piperService, err := NewPiperService(conf, datasetMetricsClient, errorTracker)
	if err != nil {
		log.Fatalf("Failed to create piper service: %v", err)
	}

	// Create services struct first
	services := &Services{
		Config:               conf,
		PiperService:         piperService,
		PipelineDatabase:     pipelineDatabase,
		StateManager:         stateManager,
		DatasetMetricsClient: datasetMetricsClient,
		ErrorTracker:         errorTracker,
	}

	// Create health reporter (after services are initialized)
	if conf.HealthReporting.Enabled {
		// Parse report interval
		reportInterval := time.Duration(conf.HealthReporting.ReportInterval) * time.Second
		if reportInterval <= 0 {
			log.Warnf("Invalid health reporting interval %d, using default 30s", conf.HealthReporting.ReportInterval)
			reportInterval = 30 * time.Second
		}

		// Parse timeout
		timeout := time.Duration(conf.HealthReporting.TimeoutSeconds) * time.Second

		// Get actual hostname
		hostname, err := os.Hostname()
		if err != nil {
			log.Warnf("Failed to get hostname, using 'localhost': %v", err)
			hostname = "localhost"
		}

		// Create instance API URL without protocol
		instanceAPI := fmt.Sprintf("%s:%d", hostname, conf.Server.ApiPort)

		// Build configuration data with masked sensitive fields
		configuration := buildHealthConfiguration(conf, instanceAPI)

		// Create health reporting service (uses control_service.base_url and api_key)
		services.HealthReporter = NewHealthReportingService(
			conf.ControlService.BaseURL,
			"bytefreezer-piper",
			instanceAPI,
			conf.ControlService.APIKey, // Pass API key for Bearer token authentication
			reportInterval,
			timeout,
			configuration,
		)
		log.Infof("Health reporting service initialized (reporting to %s)", conf.ControlService.BaseURL)
	} else {
		log.Info("Health reporting disabled")
	}

	return services
}

// GetPipelineConfigAsInterface returns pipeline config as interface for API
func (s *Services) GetPipelineConfigAsInterface(ctx context.Context, tenantID, datasetID string) (interface{}, error) {
	return s.PipelineDatabase.GetPipelineConfiguration(ctx, tenantID, datasetID)
}

// GetCacheStats returns cache statistics
func (s *Services) GetCacheStats() map[string]interface{} {
	return s.PipelineDatabase.GetCacheStats()
}

// GetCachedPipelineList returns all cached pipeline configurations
func (s *Services) GetCachedPipelineList(ctx context.Context) ([]map[string]interface{}, error) {
	return s.PipelineDatabase.GetCachedPipelineList(ctx)
}

// buildHealthConfiguration builds comprehensive configuration for health reporting
// with sensitive data masked
func buildHealthConfiguration(conf *config.Config, instanceAPI string) map[string]interface{} {
	maskSensitive := func(value string) string {
		if value == "" {
			return ""
		}
		if len(value) <= 4 {
			return "****"
		}
		return value[:2] + "****" + value[len(value)-2:]
	}

	return map[string]interface{}{
		"service_type":    "bytefreezer-piper",
		"version":         conf.App.Version,
		"instance_id":     conf.App.InstanceID,
		"instance_api":    instanceAPI,
		"report_interval": conf.HealthReporting.ReportInterval,
		"timeout":         fmt.Sprintf("%ds", conf.HealthReporting.TimeoutSeconds),
		"api": map[string]interface{}{
			"port": conf.Server.ApiPort,
		},
		"s3_source": map[string]interface{}{
			"bucket_name":   conf.S3Source.BucketName,
			"region":        conf.S3Source.Region,
			"poll_interval": conf.S3Source.PollInterval.String(),
			"endpoint":      conf.S3Source.Endpoint,
			"ssl":           conf.S3Source.SSL,
			"access_key":    maskSensitive(conf.S3Source.AccessKey),
			"secret_key":    maskSensitive(conf.S3Source.SecretKey),
		},
		"s3_destination": map[string]interface{}{
			"bucket_name": conf.S3Dest.BucketName,
			"region":      conf.S3Dest.Region,
			"endpoint":    conf.S3Dest.Endpoint,
			"ssl":         conf.S3Dest.SSL,
			"access_key":  maskSensitive(conf.S3Dest.AccessKey),
			"secret_key":  maskSensitive(conf.S3Dest.SecretKey),
		},
		"s3_geoip": map[string]interface{}{
			"bucket_name": conf.S3GeoIP.BucketName,
			"region":      conf.S3GeoIP.Region,
			"endpoint":    conf.S3GeoIP.Endpoint,
			"ssl":         conf.S3GeoIP.SSL,
			"access_key":  maskSensitive(conf.S3GeoIP.AccessKey),
			"secret_key":  maskSensitive(conf.S3GeoIP.SecretKey),
		},
		"postgresql": map[string]interface{}{
			"enabled":  conf.PostgreSQL.Host != "",
			"host":     conf.PostgreSQL.Host,
			"port":     conf.PostgreSQL.Port,
			"database": conf.PostgreSQL.Database,
			"username": conf.PostgreSQL.Username,
			"password": maskSensitive(conf.PostgreSQL.Password),
			"ssl_mode": conf.PostgreSQL.SSLMode,
			"schema":   conf.PostgreSQL.Schema,
		},
		"processing": map[string]interface{}{
			"max_concurrent_jobs": conf.Processing.MaxConcurrentJobs,
			"job_timeout":         conf.Processing.JobTimeout.String(),
			"retry_attempts":      conf.Processing.RetryAttempts,
			"retry_backoff":       conf.Processing.RetryBackoff,
			"buffer_size":         conf.Processing.BufferSize,
		},
		"pipeline": map[string]interface{}{
			"config_refresh_interval": conf.Pipeline.ConfigRefreshInterval.String(),
			"geoip_database_path":     conf.Pipeline.GeoIPDatabasePath,
			"enable_geoip":            conf.Pipeline.EnableGeoIP,
		},
		"control_service": map[string]interface{}{
			"enabled":  conf.ControlService.Enabled,
			"base_url": conf.ControlService.BaseURL,
			"api_key":  maskSensitive(conf.ControlService.APIKey),
			"timeout":  conf.ControlService.TimeoutSeconds,
		},
		"monitoring": map[string]interface{}{
			"metrics_port":    conf.Monitoring.MetricsPort,
			"log_level":       conf.Monitoring.LogLevel,
			"enable_tracing":  conf.Monitoring.EnableTracing,
			"tracing_endpoint": conf.Monitoring.TracingEndpoint,
		},
		"housekeeping": map[string]interface{}{
			"enabled":          conf.Housekeeping.Enabled,
			"interval_seconds": conf.Housekeeping.IntervalSeconds,
		},
		"dlq": map[string]interface{}{
			"enabled":                   conf.DLQ.Enabled,
			"retry_attempts":            conf.DLQ.RetryAttempts,
			"retry_interval_seconds":    conf.DLQ.RetryIntervalSeconds,
			"cleanup_interval_seconds":  conf.DLQ.CleanupIntervalSeconds,
			"max_age_days":              conf.DLQ.MaxAgeDays,
		},
		"soc": map[string]interface{}{
			"enabled":  conf.SOC.Enabled,
			"endpoint": conf.SOC.Endpoint,
			"timeout":  conf.SOC.Timeout,
		},
		"failure_threshold": map[string]interface{}{
			"enabled":           conf.FailureThreshold.Enabled,
			"failure_threshold": conf.FailureThreshold.FailureThreshold,
			"minimum_samples":   conf.FailureThreshold.MinimumSamples,
			"window_size":       conf.FailureThreshold.WindowSize,
			"check_interval":    conf.FailureThreshold.CheckInterval,
			"cooldown_period":   conf.FailureThreshold.CooldownPeriod,
		},
		"capabilities": []string{
			"data_pipeline",
			"format_parsing",
			"data_filtering",
			"s3_processing",
			"geoip_enrichment",
			"multi_format_support",
		},
	}
}
