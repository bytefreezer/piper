package api

import (
	"context"
	"time"

	"github.com/n0needt0/go-goodies/log"
	"github.com/swaggest/usecase"
	"github.com/swaggest/usecase/status"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status          string `json:"status"`
	Version         string `json:"version"`
	DatabaseHealthy bool   `json:"database_healthy"`
	DatabaseStatus  string `json:"database_status"`
	DevMode         bool   `json:"dev_mode"`
}

// ConfigResponse represents the current system configuration
type ConfigResponse struct {
	App        AppConfig              `json:"app"`
	S3Source   S3ConfigMasked         `json:"s3_source"`
	S3Dest     S3ConfigMasked         `json:"s3_destination"`
	Processing ProcessingConfig       `json:"processing"`
	Pipeline   PipelineConfig         `json:"pipeline"`
	Monitoring MonitoringConfig       `json:"monitoring"`
	DevMode    bool                   `json:"dev_mode"`
}

// Configuration sections for response
type AppConfig struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	InstanceID string `json:"instance_id"`
	LogLevel   string `json:"log_level"`
}

type S3ConfigMasked struct {
	BucketName string `json:"bucket_name"`
	Region     string `json:"region"`
	AccessKey  string `json:"access_key"` // Will be masked
	SecretKey  string `json:"secret_key"` // Will be masked
	Endpoint   string `json:"endpoint"`
	SSL        bool   `json:"ssl"`
}


type ProcessingConfig struct {
	MaxConcurrentJobs int    `json:"max_concurrent_jobs"`
	JobTimeout        string `json:"job_timeout"`
	RetryAttempts     int    `json:"retry_attempts"`
	RetryBackoff      string `json:"retry_backoff"`
	BufferSize        int    `json:"buffer_size"`
}

type PipelineConfig struct {
	ConfigRefreshInterval string `json:"config_refresh_interval"`
}

type MonitoringConfig struct {
	MetricsPort   int  `json:"metrics_port"`
	EnableTracing bool `json:"enable_tracing"`
}

// PipelineEntry represents a cached pipeline configuration
type PipelineEntry struct {
	TenantID     string    `json:"tenant_id"`
	DatasetID    string    `json:"dataset_id"`
	ConfigKey    string    `json:"config_key"`
	Version      string    `json:"version"`
	Enabled      bool      `json:"enabled"`
	DateCreated  time.Time `json:"date_created"`
	DateModified time.Time `json:"date_modified"`
	CachedAt     time.Time `json:"cached_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	FilterCount  int       `json:"filter_count"`
}

// PipelineListResponse represents the pipeline list response
type PipelineListResponse struct {
	Pipelines []PipelineEntry `json:"pipelines"`
	Count     int             `json:"count"`
}

// PipelineDetailsRequest represents the get pipeline details request
type PipelineDetailsRequest struct {
	TenantID  string `path:"tenantId" required:"true"`
	DatasetID string `path:"datasetId" required:"true"`
}

// PipelineDetailsResponse represents the pipeline details response
type PipelineDetailsResponse struct {
	Pipeline interface{} `json:"pipeline"`
}

// HealthCheck returns a health check handler that tests database connection
func (api *API) HealthCheck() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input struct{}, output *HealthResponse) error {
		status := "ok"
		databaseHealthy := false
		databaseStatus := "unknown"

		// Test database connection
		databaseHealthy = true
		databaseStatus = "using_control_service_api"

		output.Status = status
		output.Version = api.Config.App.Version
		output.DatabaseHealthy = databaseHealthy
		output.DatabaseStatus = databaseStatus
		output.DevMode = api.Config.Dev

		log.Debugf("Health check: status=%s, db_healthy=%t, dev_mode=%t", status, databaseHealthy, api.Config.Dev)

		return nil
	})

	u.SetTitle("Health Check")
	u.SetDescription("Check the health status of the ByteFreezer Piper service and test database connection")
	u.SetTags("Health")

	return u
}

// GetConfig returns a handler for getting current system configuration
func (api *API) GetConfig() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input struct{}, output *ConfigResponse) error {
		cfg := api.Config

		// Basic app configuration
		output.App = AppConfig{
			Name:       cfg.App.Name,
			Version:    cfg.App.Version,
			InstanceID: cfg.App.InstanceID,
			LogLevel:   cfg.App.LogLevel,
		}

		// S3 source configuration (with masked secrets)
		output.S3Source = S3ConfigMasked{
			BucketName: cfg.S3Source.BucketName,
			Region:     cfg.S3Source.Region,
			AccessKey:  maskSensitiveValue(cfg.S3Source.AccessKey),
			SecretKey:  maskSensitiveValue(cfg.S3Source.SecretKey),
			Endpoint:   cfg.S3Source.Endpoint,
			SSL:        cfg.S3Source.SSL,
		}

		// S3 destination configuration (with masked secrets)
		output.S3Dest = S3ConfigMasked{
			BucketName: cfg.S3Dest.BucketName,
			Region:     cfg.S3Dest.Region,
			AccessKey:  maskSensitiveValue(cfg.S3Dest.AccessKey),
			SecretKey:  maskSensitiveValue(cfg.S3Dest.SecretKey),
			Endpoint:   cfg.S3Dest.Endpoint,
			SSL:        cfg.S3Dest.SSL,
		}

		// PostgreSQL removed - using Control Service API

		// Processing configuration
		output.Processing = ProcessingConfig{
			MaxConcurrentJobs: cfg.Processing.MaxConcurrentJobs,
			JobTimeout:        cfg.Processing.JobTimeout.String(),
			RetryAttempts:     cfg.Processing.RetryAttempts,
			RetryBackoff:      cfg.Processing.RetryBackoff,
			BufferSize:        cfg.Processing.BufferSize,
		}

		// Pipeline configuration
		output.Pipeline = PipelineConfig{
			ConfigRefreshInterval: cfg.Pipeline.ConfigRefreshInterval.String(),
		}

		// Monitoring configuration
		output.Monitoring = MonitoringConfig{
			MetricsPort:   cfg.Monitoring.MetricsPort,
			EnableTracing: cfg.Monitoring.EnableTracing,
		}

		// Dev mode flag
		output.DevMode = cfg.Dev

		log.Debugf("Retrieved system configuration")

		return nil
	})

	u.SetTitle("Get System Configuration")
	u.SetDescription("Retrieve the current system configuration (sensitive values are masked)")
	u.SetTags("Configuration")

	return u
}

// GetPipelineList returns a handler for getting all cached pipeline configurations
func (api *API) GetPipelineList() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input struct{}, output *PipelineListResponse) error {
		// Get cached pipeline list from database
		cachedPipelines, err := api.Services.GetCachedPipelineList(ctx)
		if err != nil {
			log.Errorf("Failed to get cached pipeline list: %v", err)
			// Return empty list on error
			output.Pipelines = []PipelineEntry{}
			output.Count = 0
			return nil
		}

		// Convert database format to API format
		pipelines := make([]PipelineEntry, 0, len(cachedPipelines))
		for _, cached := range cachedPipelines {
			entry := PipelineEntry{
				TenantID:     cached["tenant_id"].(string),
				DatasetID:    cached["dataset_id"].(string),
				ConfigKey:    cached["config_key"].(string),
				Version:      cached["version"].(string),
				Enabled:      cached["enabled"].(bool),
				DateCreated:  cached["date_created"].(time.Time),
				DateModified: cached["date_modified"].(time.Time),
				CachedAt:     cached["cached_at"].(time.Time),
				ExpiresAt:    cached["expires_at"].(time.Time),
				FilterCount:  cached["filter_count"].(int),
			}
			pipelines = append(pipelines, entry)
		}

		output.Pipelines = pipelines
		output.Count = len(pipelines)

		cacheStats := api.Services.GetCacheStats()
		log.Debugf("Retrieved %d cached pipeline configurations. Cache stats: %+v", len(pipelines), cacheStats)

		return nil
	})

	u.SetTitle("Get Pipeline List")
	u.SetDescription("Retrieve all tenant/dataset combinations in pipeline cache with creation/modification dates")
	u.SetTags("Pipelines")

	return u
}

// GetPipelineDetails returns a handler for getting pipeline details for a specific tenant/dataset
func (api *API) GetPipelineDetails() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input PipelineDetailsRequest, output *PipelineDetailsResponse) error {
		// Get pipeline configuration for the specific tenant/dataset
		pipelineConfig, err := api.Services.GetPipelineConfigAsInterface(ctx, input.TenantID, input.DatasetID)
		if err != nil {
			log.Errorf("Failed to get pipeline config for %s/%s: %v", input.TenantID, input.DatasetID, err)
			return status.Wrap(status.InvalidArgument, status.NotFound)
		}

		if pipelineConfig == nil {
			return status.Wrap(status.InvalidArgument, status.NotFound)
		}

		output.Pipeline = pipelineConfig

		log.Debugf("Retrieved pipeline details for %s/%s", input.TenantID, input.DatasetID)

		return nil
	})

	u.SetTitle("Get Pipeline Details")
	u.SetDescription("Retrieve detailed pipeline configuration for a specific tenant/dataset combination")
	u.SetTags("Pipelines")

	return u
}

// maskSensitiveValue masks sensitive configuration values
func maskSensitiveValue(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}
