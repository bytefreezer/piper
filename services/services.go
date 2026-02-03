// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bytefreezer/goodies/log"

	"github.com/bytefreezer/piper/config"
	"github.com/bytefreezer/piper/errors"
	"github.com/bytefreezer/piper/metrics"
	"github.com/bytefreezer/piper/pipeline"
	"github.com/bytefreezer/piper/storage"
)

// Services encapsulates all service dependencies following receiver pattern
type Services struct {
	Config                   *config.Config
	PiperService             *PiperService
	PipelineDatabase         *pipeline.PipelineDatabase
	StateManager             storage.StateManager
	HealthReporter           *HealthReportingService
	DatasetMetricsClient     *metrics.DatasetMetricsClient
	SchemaSubmissionClient   *metrics.SchemaSubmissionClient
	TransformationJobService *TransformationJobService
	MetricsTracker           *TransformationMetricsTracker
	MetricsReporter          *MetricsReporter
	ErrorReporter            *errors.ErrorReporter
}

// NewServices creates and initializes all services
func NewServices(conf *config.Config) *Services {
	// Initialize error reporter if configured
	var errorReporter *errors.ErrorReporter
	if conf.ErrorTracking.Enabled && conf.ControlService.ControlURL != "" {
		errorReporter = errors.NewErrorReporter(
			conf.ControlService.ControlURL,
			conf.ControlService.APIKey,
			"piper",
			true,
		)
		log.Infof("Error reporter initialized - reporting to control service at %s", conf.ControlService.ControlURL)
	} else {
		log.Infof("Error reporting disabled (enabled: %v, control_service: %s)", conf.ErrorTracking.Enabled, conf.ControlService.ControlURL)
	}

	// Create state manager (use Control API)
	stateManager, err := storage.NewControlAPIStateManager(&conf.ControlService, conf.App.InstanceID)
	if err != nil {
		// Continue without state manager for development
		log.Warnf("Failed to create state manager: %v - continuing without state manager", err)
	}

	// Create pipeline client
	pipelineClient := pipeline.NewPipelineClient(conf)

	// Create pipeline database
	pipelineDatabase := pipeline.NewPipelineDatabase(pipelineClient, stateManager, conf.App.InstanceID)

	// Create dataset metrics client
	datasetMetricsClient := metrics.NewDatasetMetricsClient(
		conf.ControlService.ControlURL,
		conf.ControlService.APIKey,
		conf.ControlService.TimeoutSeconds,
		conf.ControlService.Enabled,
	)
	log.Infof("Dataset metrics client initialized (enabled: %v, endpoint: %s)",
		conf.ControlService.Enabled, conf.ControlService.ControlURL)

	// Create schema submission client
	schemaSubmissionClient := metrics.NewSchemaSubmissionClient(
		conf.ControlService.ControlURL,
		conf.ControlService.APIKey,
		conf.ControlService.TimeoutSeconds,
		conf.ControlService.Enabled,
	)
	log.Infof("Schema submission client initialized (enabled: %v, endpoint: %s)",
		conf.ControlService.Enabled, conf.ControlService.ControlURL)

	// Create metrics tracker
	metricsTracker := NewTransformationMetricsTracker()
	log.Info("Transformation metrics tracker initialized")

	// Create piper service
	piperService, err := NewPiperService(conf, datasetMetricsClient, schemaSubmissionClient, metricsTracker, errorReporter)
	if err != nil {
		log.Fatalf("Failed to create piper service: %v", err)
	}

	// Create services struct first
	services := &Services{
		Config:                 conf,
		PiperService:           piperService,
		PipelineDatabase:       pipelineDatabase,
		StateManager:           stateManager,
		DatasetMetricsClient:   datasetMetricsClient,
		SchemaSubmissionClient: schemaSubmissionClient,
		MetricsTracker:         metricsTracker,
		ErrorReporter:          errorReporter,
	}

	// Create transformation job service if state manager is available
	if stateManager != nil {
		pollInterval := 5 * time.Second // Poll for jobs every 5 seconds
		services.TransformationJobService = NewTransformationJobService(services, conf.App.InstanceID, pollInterval)
		log.Infof("Transformation job service initialized (instance: %s, poll interval: %v)", conf.App.InstanceID, pollInterval)
	} else {
		log.Warn("Transformation job service disabled - no state manager available")
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
		// If running in Kubernetes with NODE_NAME env var, use node.pod format
		// This helps operators identify both the node and specific pod for debugging
		hostname, err := os.Hostname()
		if err != nil {
			log.Warnf("Failed to get hostname, using 'localhost': %v", err)
			hostname = "localhost"
		}
		if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
			hostname = fmt.Sprintf("%s.%s", nodeName, hostname)
			log.Infof("Running in Kubernetes on node %s, instance ID: %s", nodeName, hostname)
		}

		// Create instance API URL without protocol
		instanceAPI := fmt.Sprintf("%s:%d", hostname, conf.Server.ApiPort)

		// Build configuration data with masked sensitive fields
		configuration := buildHealthConfiguration(conf, instanceAPI)

		// Create health reporting service (uses control_service.control_url and api_key)
		services.HealthReporter = NewHealthReportingService(
			conf.ControlService.ControlURL,
			"bytefreezer-piper",
			instanceAPI,
			conf.ControlService.APIKey, // Pass API key for Bearer token authentication
			reportInterval,
			timeout,
			configuration,
		)
		log.Infof("Health reporting service initialized (reporting to %s)", conf.ControlService.ControlURL)
	} else {
		log.Info("Health reporting disabled")
	}

	// Create and start metrics reporter if control service is enabled
	if conf.ControlService.Enabled && conf.ControlService.ControlURL != "" {
		// Use 30 second reporting interval and 10 second timeout
		metricsReportInterval := 30 * time.Second
		metricsTimeout := 10 * time.Second

		services.MetricsReporter = NewMetricsReporter(
			conf.ControlService.ControlURL,
			conf.ControlService.APIKey,
			metricsTracker,
			metricsReportInterval,
			metricsTimeout,
			true, // enabled
		)
		log.Infof("Transformation metrics reporter initialized (reporting to %s every %v)",
			conf.ControlService.ControlURL, metricsReportInterval)
	} else {
		log.Info("Transformation metrics reporting disabled (control service not configured)")
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

	configMap := map[string]interface{}{
		"service_type":    "bytefreezer-piper",
		"version":         conf.App.Version,
		"git_commit":      conf.App.GitCommit,
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
		"state_manager": map[string]interface{}{
			"type":            "control_service_api",
			"control_url":     conf.ControlService.ControlURL,
			"control_enabled": conf.ControlService.Enabled,
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
			"enabled":     conf.ControlService.Enabled,
			"control_url": conf.ControlService.ControlURL,
			"api_key":     maskSensitive(conf.ControlService.APIKey),
			"timeout":     conf.ControlService.TimeoutSeconds,
		},
		"monitoring": map[string]interface{}{
			"metrics_port":     conf.Monitoring.MetricsPort,
			"log_level":        conf.Monitoring.LogLevel,
			"enable_tracing":   conf.Monitoring.EnableTracing,
			"tracing_endpoint": conf.Monitoring.TracingEndpoint,
		},
		"housekeeping": map[string]interface{}{
			"enabled":          conf.Housekeeping.Enabled,
			"interval_seconds": conf.Housekeeping.IntervalSeconds,
		},
		"dlq": map[string]interface{}{
			"enabled":                  conf.DLQ.Enabled,
			"retry_attempts":           conf.DLQ.RetryAttempts,
			"retry_interval_seconds":   conf.DLQ.RetryIntervalSeconds,
			"cleanup_interval_seconds": conf.DLQ.CleanupIntervalSeconds,
			"max_age_days":             conf.DLQ.MaxAgeDays,
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

	// Add account_id at root level if configured (for on-prem installs)
	if conf.ControlService.AccountID != "" {
		configMap["account_id"] = conf.ControlService.AccountID
	}

	return configMap
}
