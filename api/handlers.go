package api

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/n0needt0/go-goodies/log"
	"github.com/swaggest/usecase"
	"github.com/swaggest/usecase/status"
	_ "github.com/lib/pq" // PostgreSQL driver
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
	App        AppConfig     `json:"app"`
	S3Source   S3ConfigMasked `json:"s3_source"`
	S3Dest     S3ConfigMasked `json:"s3_destination"`
	PostgreSQL PostgreSQLConfigMasked `json:"postgresql"`
	Processing ProcessingConfig `json:"processing"`
	Pipeline   PipelineConfig   `json:"pipeline"`
	Monitoring MonitoringConfig `json:"monitoring"`
	DevMode    bool            `json:"dev_mode"`
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
	Prefix     string `json:"prefix"`
	AccessKey  string `json:"access_key"` // Will be masked
	SecretKey  string `json:"secret_key"` // Will be masked
	Endpoint   string `json:"endpoint"`
	SSL        bool   `json:"ssl"`
}

type PostgreSQLConfigMasked struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"` // Will be masked
	SSLMode  string `json:"ssl_mode"`
	Schema   string `json:"schema"`
}

type ProcessingConfig struct {
	MaxConcurrentJobs int    `json:"max_concurrent_jobs"`
	JobTimeout        string `json:"job_timeout"`
	RetryAttempts     int    `json:"retry_attempts"`
	RetryBackoff      string `json:"retry_backoff"`
	BufferSize        int    `json:"buffer_size"`
}

type PipelineConfig struct {
	ControllerEndpoint    string `json:"controller_endpoint"`
	ConfigRefreshInterval string `json:"config_refresh_interval"`
	EnableGeoIP           bool   `json:"enable_geoip"`
}

type MonitoringConfig struct {
	MetricsPort   int  `json:"metrics_port"`
	HealthPort    int  `json:"health_port"`
	EnableTracing bool `json:"enable_tracing"`
}

// PipelineEntry represents a cached pipeline configuration
type PipelineEntry struct {
	TenantID      string    `json:"tenant_id"`
	DatasetID     string    `json:"dataset_id"`
	ConfigKey     string    `json:"config_key"`
	Version       string    `json:"version"`
	Enabled       bool      `json:"enabled"`
	DateCreated   time.Time `json:"date_created"`
	DateModified  time.Time `json:"date_modified"`
	CachedAt      time.Time `json:"cached_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	FilterCount   int       `json:"filter_count"`
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
		if api.Config.PostgreSQL.Host != "" {
			connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
				api.Config.PostgreSQL.Host,
				api.Config.PostgreSQL.Port,
				api.Config.PostgreSQL.Username,
				api.Config.PostgreSQL.Password,
				api.Config.PostgreSQL.Database,
				api.Config.PostgreSQL.SSLMode)

			db, err := sql.Open("postgres", connStr)
			if err != nil {
				databaseStatus = fmt.Sprintf("connection_failed: %v", err)
				status = "degraded"
			} else {
				defer db.Close()
				if err := db.Ping(); err != nil {
					databaseStatus = fmt.Sprintf("ping_failed: %v", err)
					status = "degraded"
				} else {
					databaseHealthy = true
					databaseStatus = "connected"
				}
			}
		} else {
			databaseStatus = "not_configured"
			status = "degraded"
		}

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
			Prefix:     cfg.S3Source.Prefix,
			AccessKey:  maskSensitiveValue(cfg.S3Source.AccessKey),
			SecretKey:  maskSensitiveValue(cfg.S3Source.SecretKey),
			Endpoint:   cfg.S3Source.Endpoint,
			SSL:        cfg.S3Source.SSL,
		}

		// S3 destination configuration (with masked secrets)
		output.S3Dest = S3ConfigMasked{
			BucketName: cfg.S3Dest.BucketName,
			Region:     cfg.S3Dest.Region,
			Prefix:     cfg.S3Dest.Prefix,
			AccessKey:  maskSensitiveValue(cfg.S3Dest.AccessKey),
			SecretKey:  maskSensitiveValue(cfg.S3Dest.SecretKey),
			Endpoint:   cfg.S3Dest.Endpoint,
			SSL:        cfg.S3Dest.SSL,
		}

		// PostgreSQL configuration (with masked password)
		output.PostgreSQL = PostgreSQLConfigMasked{
			Host:     cfg.PostgreSQL.Host,
			Port:     cfg.PostgreSQL.Port,
			Database: cfg.PostgreSQL.Database,
			Username: cfg.PostgreSQL.Username,
			Password: maskSensitiveValue(cfg.PostgreSQL.Password),
			SSLMode:  cfg.PostgreSQL.SSLMode,
			Schema:   cfg.PostgreSQL.Schema,
		}

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
			ControllerEndpoint:    cfg.Pipeline.ControllerEndpoint,
			ConfigRefreshInterval: cfg.Pipeline.ConfigRefreshInterval.String(),
			EnableGeoIP:           cfg.Pipeline.EnableGeoIP,
		}

		// Monitoring configuration
		output.Monitoring = MonitoringConfig{
			MetricsPort:   cfg.Monitoring.MetricsPort,
			HealthPort:    cfg.Monitoring.HealthPort,
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
		// Get cache statistics from config manager
		cacheStats := api.ConfigManager.GetCacheStats()
		pipelines := []PipelineEntry{}

		// For now, we'll return minimal info since we don't have direct cache access
		// In dev mode, show the fake pipeline
		if api.Config.Dev {
			pipelines = append(pipelines, PipelineEntry{
				TenantID:      "customer-1",
				DatasetID:     "ebpf-data",
				ConfigKey:     "customer-1:ebpf-data",
				Version:       "dev-1.0.0",
				Enabled:       true,
				DateCreated:   time.Now().Add(-24 * time.Hour), // Fake created yesterday
				DateModified:  time.Now().Add(-1 * time.Hour),  // Fake modified 1 hour ago
				CachedAt:      time.Now(),
				ExpiresAt:     time.Now().Add(5 * time.Minute),
				FilterCount:   5, // json_validate, json_flatten, uppercase_keys, 2x add_field
			})
		}

		output.Pipelines = pipelines
		output.Count = len(pipelines)

		log.Debugf("Retrieved %d pipeline configurations. Cache stats: %+v", len(pipelines), cacheStats)

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
		pipelineConfig, err := api.ConfigManager.GetPipelineConfigAsInterface(ctx, input.TenantID, input.DatasetID)
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