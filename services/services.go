package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/pipeline"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// Services encapsulates all service dependencies following receiver pattern
type Services struct {
	Config           *config.Config
	PiperService     *PiperService
	PipelineDatabase *pipeline.PipelineDatabase
	StateManager     *storage.PostgreSQLStateManager
	HealthReporter   *HealthReportingService
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

	// Create piper service
	piperService, err := NewPiperService(conf)
	if err != nil {
		log.Fatalf("Failed to create piper service: %v", err)
	}

	// Create services struct first
	services := &Services{
		Config:           conf,
		PiperService:     piperService,
		PipelineDatabase: pipelineDatabase,
		StateManager:     stateManager,
	}

	// Create health reporter (after services are initialized)
	if conf.HealthReporting.Enabled {
		// Parse report interval
		reportInterval, err := time.ParseDuration(conf.HealthReporting.ReportInterval)
		if err != nil {
			log.Warnf("Failed to parse health reporting interval '%s', using default 30s: %v", conf.HealthReporting.ReportInterval, err)
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

		// Create health reporting service
		services.HealthReporter = NewHealthReportingService(
			conf.HealthReporting.ControlURL,
			"bytefreezer-piper",
			instanceAPI,
			reportInterval,
			timeout,
		)
		log.Info("Health reporting service initialized")
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
