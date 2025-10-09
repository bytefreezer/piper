package services

import (
	"time"

	"github.com/n0needt0/bytefreezer-control/health"
	"github.com/n0needt0/bytefreezer-piper/config"
)

// PiperConfigProvider implements health.ConfigProvider for piper service
type PiperConfigProvider struct {
	config *config.Config
}

// NewPiperConfigProvider creates a new config provider for health reporting
func NewPiperConfigProvider(cfg *config.Config) *PiperConfigProvider {
	return &PiperConfigProvider{
		config: cfg,
	}
}

// GetSanitizedConfig returns a sanitized version of the config for health reporting
func (p *PiperConfigProvider) GetSanitizedConfig() map[string]interface{} {
	return map[string]interface{}{
		"app": map[string]interface{}{
			"name":        p.config.App.Name,
			"version":     p.config.App.Version,
			"instance_id": p.config.App.InstanceID,
			"log_level":   p.config.App.LogLevel,
		},
		"server": map[string]interface{}{
			"apiport": p.config.Server.ApiPort,
		},
		"processing": map[string]interface{}{
			"max_concurrent_jobs": p.config.Processing.MaxConcurrentJobs,
			"job_timeout":         p.config.Processing.JobTimeout.String(),
			"retry_attempts":      p.config.Processing.RetryAttempts,
			"buffer_size":         p.config.Processing.BufferSize,
		},
		"pipeline": map[string]interface{}{
			"enable_geoip":              p.config.Pipeline.EnableGeoIP,
			"config_refresh_interval":   p.config.Pipeline.ConfigRefreshInterval.String(),
		},
		"housekeeping": map[string]interface{}{
			"enabled":   p.config.Housekeeping.Enabled,
			"interval":  p.config.Housekeeping.Interval.String(),
		},
		"health_reporting": map[string]interface{}{
			"enabled":         p.config.HealthReporting.Enabled,
			"report_interval": p.config.HealthReporting.ReportInterval.String(),
		},
	}
}

// PiperMetricsProvider implements health.MetricsProvider for piper service
type PiperMetricsProvider struct {
	services *Services
}

// NewPiperMetricsProvider creates a new metrics provider for health reporting
func NewPiperMetricsProvider(services *Services) *PiperMetricsProvider {
	return &PiperMetricsProvider{
		services: services,
	}
}

// GetMetrics returns current service metrics
func (p *PiperMetricsProvider) GetMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(time.Now()).String(), // This will be set properly when service starts
	}

	// Add cache statistics if available
	if p.services.PipelineDatabase != nil {
		cacheStats := p.services.GetCacheStats()
		if cacheStats != nil {
			metrics["pipeline_cache"] = cacheStats
		}
	}

	// Add piper service metrics if available
	if p.services.PiperService != nil {
		metrics["piper_service"] = map[string]interface{}{
			"initialized": true,
		}
	}

	return metrics
}

// PiperHealthChecker implements health.HealthChecker for piper service
type PiperHealthChecker struct {
	services *Services
}

// NewPiperHealthChecker creates a new health checker for health reporting
func NewPiperHealthChecker(services *Services) *PiperHealthChecker {
	return &PiperHealthChecker{
		services: services,
	}
}

// IsHealthy returns true if the service is healthy
func (p *PiperHealthChecker) IsHealthy() bool {
	// Check if core services are initialized
	if p.services.PiperService == nil {
		return false
	}

	// Check if pipeline database is accessible (optional - not critical for health)
	// We don't fail health check if database is unavailable since it can run without it

	// Service is healthy if core components are initialized
	return true
}

// CreateHealthReporter creates a health reporter for the piper service
func CreateHealthReporter(services *Services) *health.Reporter {
	cfg := services.Config

	// Convert piper config to health config
	healthConfig := health.Config{
		Enabled:         cfg.HealthReporting.Enabled,
		ControlURL:      cfg.HealthReporting.ControlURL,
		ReportInterval:  cfg.HealthReporting.ReportInterval,
		ServiceName:     cfg.App.Name,
		ServiceID:       cfg.App.InstanceID,
		Version:         cfg.App.Version,
		TimeoutSeconds:  cfg.HealthReporting.TimeoutSeconds,
	}

	// Create providers
	configProvider := NewPiperConfigProvider(cfg)
	metricsProvider := NewPiperMetricsProvider(services)
	healthChecker := NewPiperHealthChecker(services)

	// Create and return health reporter
	return health.NewReporter(healthConfig, configProvider, metricsProvider, healthChecker)
}