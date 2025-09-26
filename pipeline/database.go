package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// PipelineDatabase manages pipeline configurations and tenant information with local database caching
type PipelineDatabase struct {
	// In-memory cache for fast access
	pipelines map[string]*domain.PipelineConfiguration
	tenants   map[string]TenantInfo

	// Thread safety
	pipelineMutex sync.RWMutex
	tenantMutex   sync.RWMutex

	// Health tracking
	lastSync  time.Time
	isHealthy bool

	// External dependencies
	client       *PipelineClient
	stateManager *storage.PostgreSQLStateManager

	// Statistics
	cacheHits   int64
	cacheMisses int64
	statsMutex  sync.RWMutex
}

// NewPipelineDatabase creates a new pipeline database
func NewPipelineDatabase(client *PipelineClient, stateManager *storage.PostgreSQLStateManager) *PipelineDatabase {
	return &PipelineDatabase{
		pipelines:    make(map[string]*domain.PipelineConfiguration),
		tenants:      make(map[string]TenantInfo),
		client:       client,
		stateManager: stateManager,
		isHealthy:    false,
	}
}

// UpdateDatabase fetches and caches all pipeline configurations and tenant information
func (pdb *PipelineDatabase) UpdateDatabase(ctx context.Context) error {
	log.Infof("Starting pipeline database update...")

	// Fetch tenants from control service
	tenants, err := pdb.client.FetchTenants(ctx)
	if err != nil {
		pdb.markUnhealthy()
		return fmt.Errorf("failed to fetch tenants: %w", err)
	}

	log.Infof("Fetched %d tenants from control service", len(tenants))

	// Cache tenants in database and memory
	newTenants := make(map[string]TenantInfo)
	for _, tenant := range tenants {
		// Cache in PostgreSQL
		if err := pdb.stateManager.CacheTenant(ctx, tenant.TenantID, tenant.Name, tenant.Datasets, tenant.Active); err != nil {
			log.Warnf("Failed to cache tenant %s in database: %v", tenant.TenantID, err)
		}

		// Cache in memory
		newTenants[tenant.TenantID] = tenant
	}

	// Fetch pipeline configurations for each tenant/dataset combination
	newPipelines := make(map[string]*domain.PipelineConfiguration)
	totalConfigs := 0

	for _, tenant := range tenants {
		if !tenant.Active {
			continue
		}

		for _, datasetID := range tenant.Datasets {
			configKey := fmt.Sprintf("%s:%s", tenant.TenantID, datasetID)

			pipelineResp, err := pdb.client.FetchPipelineConfiguration(ctx, tenant.TenantID, datasetID)
			if err != nil {
				log.Warnf("Failed to fetch pipeline config for %s/%s: %v", tenant.TenantID, datasetID, err)
				continue
			}

			if pipelineResp.Configuration == nil {
				log.Debugf("No pipeline configuration found for %s/%s", tenant.TenantID, datasetID)
				continue
			}

			// Cache in PostgreSQL
			configJSON, err := json.Marshal(pipelineResp.Configuration)
			if err != nil {
				log.Warnf("Failed to marshal pipeline config for %s: %v", configKey, err)
				continue
			}

			filterCount := len(pipelineResp.Configuration.Filters)
			if err := pdb.stateManager.CachePipelineConfiguration(ctx, configKey, tenant.TenantID, datasetID, pipelineResp.Version, configJSON, filterCount); err != nil {
				log.Warnf("Failed to cache pipeline config %s in database: %v", configKey, err)
			}

			// Cache in memory
			newPipelines[configKey] = pipelineResp.Configuration
			totalConfigs++
		}
	}

	// Atomic update of in-memory cache
	pdb.tenantMutex.Lock()
	pdb.tenants = newTenants
	pdb.tenantMutex.Unlock()

	pdb.pipelineMutex.Lock()
	pdb.pipelines = newPipelines
	pdb.pipelineMutex.Unlock()

	// Update health status
	pdb.markHealthy()

	log.Infof("Pipeline database update completed: %d tenants, %d pipeline configurations cached", len(tenants), totalConfigs)
	return nil
}

// GetPipelineConfiguration retrieves a pipeline configuration by tenant/dataset
func (pdb *PipelineDatabase) GetPipelineConfiguration(ctx context.Context, tenantID, datasetID string) (*domain.PipelineConfiguration, error) {
	configKey := fmt.Sprintf("%s:%s", tenantID, datasetID)

	// Try in-memory cache first
	pdb.pipelineMutex.RLock()
	config, exists := pdb.pipelines[configKey]
	pdb.pipelineMutex.RUnlock()

	if exists {
		pdb.incrementCacheHits()
		log.Debugf("Pipeline configuration cache hit for %s", configKey)
		return config, nil
	}

	// Try PostgreSQL cache
	configJSON, found, err := pdb.stateManager.GetCachedPipelineConfiguration(ctx, configKey)
	if err != nil {
		log.Warnf("Failed to get cached pipeline config from database: %v", err)
	} else if found {
		var config domain.PipelineConfiguration
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Warnf("Failed to unmarshal cached pipeline config: %v", err)
		} else {
			// Cache in memory for future requests
			pdb.pipelineMutex.Lock()
			pdb.pipelines[configKey] = &config
			pdb.pipelineMutex.Unlock()

			pdb.incrementCacheHits()
			log.Debugf("Pipeline configuration database cache hit for %s", configKey)
			return &config, nil
		}
	}

	pdb.incrementCacheMisses()
	log.Debugf("Pipeline configuration cache miss for %s", configKey)
	return nil, fmt.Errorf("pipeline configuration not found for %s/%s", tenantID, datasetID)
}

// GetTenantByID retrieves a tenant by ID
func (pdb *PipelineDatabase) GetTenantByID(tenantID string) (*TenantInfo, bool) {
	pdb.tenantMutex.RLock()
	tenant, exists := pdb.tenants[tenantID]
	pdb.tenantMutex.RUnlock()

	if exists {
		pdb.incrementCacheHits()
		return &tenant, true
	}

	pdb.incrementCacheMisses()
	return nil, false
}

// GetAllTenants retrieves all cached tenants
func (pdb *PipelineDatabase) GetAllTenants() []TenantInfo {
	pdb.tenantMutex.RLock()
	defer pdb.tenantMutex.RUnlock()

	tenants := make([]TenantInfo, 0, len(pdb.tenants))
	for _, tenant := range pdb.tenants {
		tenants = append(tenants, tenant)
	}

	return tenants
}

// GetTenantCount returns the number of cached tenants
func (pdb *PipelineDatabase) GetTenantCount() int {
	pdb.tenantMutex.RLock()
	defer pdb.tenantMutex.RUnlock()
	return len(pdb.tenants)
}

// GetPipelineCount returns the number of cached pipeline configurations
func (pdb *PipelineDatabase) GetPipelineCount() int {
	pdb.pipelineMutex.RLock()
	defer pdb.pipelineMutex.RUnlock()
	return len(pdb.pipelines)
}

// IsHealthy returns the health status of the database
func (pdb *PipelineDatabase) IsHealthy() bool {
	pdb.tenantMutex.RLock()
	defer pdb.tenantMutex.RUnlock()
	return pdb.isHealthy
}

// GetLastSync returns the last sync timestamp
func (pdb *PipelineDatabase) GetLastSync() time.Time {
	pdb.tenantMutex.RLock()
	defer pdb.tenantMutex.RUnlock()
	return pdb.lastSync
}

// GetCacheStats returns cache statistics
func (pdb *PipelineDatabase) GetCacheStats() map[string]interface{} {
	pdb.statsMutex.RLock()
	defer pdb.statsMutex.RUnlock()

	return map[string]interface{}{
		"cache_hits":      pdb.cacheHits,
		"cache_misses":    pdb.cacheMisses,
		"hit_ratio":       pdb.calculateHitRatio(),
		"tenant_count":    pdb.GetTenantCount(),
		"pipeline_count":  pdb.GetPipelineCount(),
		"is_healthy":      pdb.IsHealthy(),
		"last_sync":       pdb.GetLastSync(),
	}
}

// GetCachedPipelineList retrieves all cached pipeline configurations with metadata
func (pdb *PipelineDatabase) GetCachedPipelineList(ctx context.Context) ([]map[string]interface{}, error) {
	return pdb.stateManager.GetCachedPipelineList(ctx)
}

// GetCachedTenantList retrieves all cached tenants with metadata
func (pdb *PipelineDatabase) GetCachedTenantList(ctx context.Context) ([]map[string]interface{}, error) {
	return pdb.stateManager.GetCachedTenants(ctx)
}

// markHealthy marks the database as healthy
func (pdb *PipelineDatabase) markHealthy() {
	pdb.tenantMutex.Lock()
	pdb.isHealthy = true
	pdb.lastSync = time.Now()
	pdb.tenantMutex.Unlock()
}

// markUnhealthy marks the database as unhealthy
func (pdb *PipelineDatabase) markUnhealthy() {
	pdb.tenantMutex.Lock()
	pdb.isHealthy = false
	pdb.tenantMutex.Unlock()
}

// incrementCacheHits atomically increments cache hit counter
func (pdb *PipelineDatabase) incrementCacheHits() {
	pdb.statsMutex.Lock()
	pdb.cacheHits++
	pdb.statsMutex.Unlock()
}

// incrementCacheMisses atomically increments cache miss counter
func (pdb *PipelineDatabase) incrementCacheMisses() {
	pdb.statsMutex.Lock()
	pdb.cacheMisses++
	pdb.statsMutex.Unlock()
}

// calculateHitRatio calculates cache hit ratio
func (pdb *PipelineDatabase) calculateHitRatio() float64 {
	total := pdb.cacheHits + pdb.cacheMisses
	if total == 0 {
		return 0.0
	}
	return float64(pdb.cacheHits) / float64(total)
}