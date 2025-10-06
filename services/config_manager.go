package services

import (
	"context"
	"time"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/pipeline"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// ConfigManager manages pipeline configurations using local database caching
type ConfigManager struct {
	cfg              *config.Config
	pipelineClient   *pipeline.PipelineClient
	pipelineDatabase *pipeline.PipelineDatabase
	stateManager     *storage.PostgreSQLStateManager
}

// NewConfigManager creates a new configuration manager with database caching
func NewConfigManager(cfg *config.Config, stateManager *storage.PostgreSQLStateManager) *ConfigManager {
	// Initialize pipeline client for control API communication
	pipelineClient := pipeline.NewPipelineClient(cfg)

	// Initialize pipeline database for local caching
	pipelineDatabase := pipeline.NewPipelineDatabase(pipelineClient, stateManager, cfg.App.InstanceID)

	return &ConfigManager{
		cfg:              cfg,
		pipelineClient:   pipelineClient,
		pipelineDatabase: pipelineDatabase,
		stateManager:     stateManager,
	}
}

// GetPipelineConfig retrieves pipeline configuration for tenant/dataset using database cache
func (cm *ConfigManager) GetPipelineConfig(ctx context.Context, tenantID, datasetID string) (*domain.PipelineConfiguration, error) {
	return cm.pipelineDatabase.GetPipelineConfiguration(ctx, tenantID, datasetID)
}

// UpdateDatabase performs a full update of the pipeline configuration database cache
func (cm *ConfigManager) UpdateDatabase(ctx context.Context) error {
	return cm.pipelineDatabase.UpdateDatabase(ctx)
}

// GetAllTenants retrieves all cached tenants
func (cm *ConfigManager) GetAllTenants() []pipeline.TenantInfo {
	return cm.pipelineDatabase.GetAllTenants()
}

// GetPipelineConfigAsInterface returns pipeline config as interface{} for API use
func (cm *ConfigManager) GetPipelineConfigAsInterface(ctx context.Context, tenantID, datasetID string) (interface{}, error) {
	config, err := cm.GetPipelineConfig(ctx, tenantID, datasetID)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetCacheStats returns statistics about the configuration cache
func (cm *ConfigManager) GetCacheStats() map[string]interface{} {
	return cm.pipelineDatabase.GetCacheStats()
}

// IsHealthy returns the health status of the database cache
func (cm *ConfigManager) IsHealthy() bool {
	return cm.pipelineDatabase.IsHealthy()
}

// GetLastSync returns the last sync timestamp
func (cm *ConfigManager) GetLastSync() time.Time {
	return cm.pipelineDatabase.GetLastSync()
}

// GetCachedPipelineList retrieves all cached pipeline configurations with metadata
func (cm *ConfigManager) GetCachedPipelineList(ctx context.Context) ([]map[string]interface{}, error) {
	return cm.pipelineDatabase.GetCachedPipelineList(ctx)
}

// GetCachedTenantList retrieves all cached tenants with metadata
func (cm *ConfigManager) GetCachedTenantList(ctx context.Context) ([]map[string]interface{}, error) {
	return cm.pipelineDatabase.GetCachedTenantList(ctx)
}
