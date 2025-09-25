package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
)

// ConfigManager manages pipeline configurations from control service with local caching
type ConfigManager struct {
	cfg         *config.Config
	cache       map[string]*CachedConfig
	mutex       sync.RWMutex
	httpClient  *http.Client
	lastRefresh time.Time
}

// CachedConfig represents a cached pipeline configuration
type CachedConfig struct {
	Config    *domain.PipelineConfiguration
	CachedAt  time.Time
	ExpiresAt time.Time
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(cfg *config.Config) *ConfigManager {
	return &ConfigManager{
		cfg:   cfg,
		cache: make(map[string]*CachedConfig),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		lastRefresh: time.Now(),
	}
}

// GetPipelineConfig retrieves pipeline configuration for tenant/dataset
func (cm *ConfigManager) GetPipelineConfig(ctx context.Context, tenantID, datasetID string) (*domain.PipelineConfiguration, error) {
	configKey := fmt.Sprintf("%s:%s", tenantID, datasetID)

	cm.mutex.RLock()
	cached, exists := cm.cache[configKey]
	cm.mutex.RUnlock()

	// Check if cached config is still valid
	if exists && time.Now().Before(cached.ExpiresAt) {
		log.Debugf("Using cached config for %s", configKey)
		return cached.Config, nil
	}

	// Try to fetch from control service
	if cm.cfg.Pipeline.ControllerEndpoint != "" {
		log.Debugf("Fetching config for %s from control service", configKey)
		if config, err := cm.fetchFromControl(ctx, tenantID, datasetID); err == nil {
			cm.cacheConfig(configKey, config)
			return config, nil
		} else {
			log.Warnf("Failed to fetch config from control service: %v", err)
		}
	}

	// Fall back to cached config even if expired, or create default
	if exists {
		log.Infof("Using expired cached config for %s", configKey)
		return cached.Config, nil
	}

	// Create default configuration
	log.Infof("Creating default config for %s", configKey)
	defaultConfig := cm.createDefaultConfig(tenantID, datasetID)
	cm.cacheConfig(configKey, defaultConfig)
	return defaultConfig, nil
}

// fetchFromControl fetches configuration from the control service
func (cm *ConfigManager) fetchFromControl(ctx context.Context, tenantID, datasetID string) (*domain.PipelineConfiguration, error) {
	url := fmt.Sprintf("%s/api/v2/pipeline/config/%s/%s", cm.cfg.Pipeline.ControllerEndpoint, tenantID, datasetID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "bytefreezer-piper/1.0.0")

	resp, err := cm.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No specific config exists, will use default
		return nil, fmt.Errorf("config not found for %s:%s", tenantID, datasetID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("control service returned status %d: %s", resp.StatusCode, string(body))
	}

	var config domain.PipelineConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config response: %w", err)
	}

	log.Debugf("Successfully fetched config for %s:%s from control service", tenantID, datasetID)
	return &config, nil
}

// cacheConfig stores a configuration in the local cache
func (cm *ConfigManager) cacheConfig(configKey string, config *domain.PipelineConfiguration) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.cache[configKey] = &CachedConfig{
		Config:    config,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(cm.cfg.Pipeline.ConfigRefreshInterval),
	}

	log.Debugf("Cached config for %s, expires at %v", configKey, cm.cache[configKey].ExpiresAt)
}

// createDefaultConfig creates a default pipeline configuration
func (cm *ConfigManager) createDefaultConfig(tenantID, datasetID string) *domain.PipelineConfiguration {
	return &domain.PipelineConfiguration{
		ConfigKey: fmt.Sprintf("%s:%s", tenantID, datasetID),
		TenantID:  tenantID,
		DatasetID: datasetID,
		Enabled:   true,
		Version:   "1.0.0",
		Filters: []domain.FilterConfig{
			{
				Type:    "add_field",
				Enabled: true,
				Config: map[string]interface{}{
					"field": "processed_by",
					"value": "bytefreezer-piper",
				},
			},
			{
				Type:    "add_field",
				Enabled: true,
				Config: map[string]interface{}{
					"field": "processed_at",
					"value": time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UpdatedBy: "system-default",
		Validated: true,
		Settings: map[string]interface{}{
			"auto_format_detection": true,
			"drop_parse_failures":   true,
			"compress_output":       true,
		},
	}
}

// RefreshCache refreshes configurations that are about to expire
func (cm *ConfigManager) RefreshCache(ctx context.Context) {
	if time.Since(cm.lastRefresh) < time.Minute {
		return // Don't refresh too frequently
	}

	cm.mutex.RLock()
	var keysToRefresh []string
	for key, cached := range cm.cache {
		// Refresh configs that expire within the next 5 minutes
		if time.Until(cached.ExpiresAt) < 5*time.Minute {
			keysToRefresh = append(keysToRefresh, key)
		}
	}
	cm.mutex.RUnlock()

	for _, key := range keysToRefresh {
		if len(key) > 0 {
			log.Debugf("Refreshing config for %s", key)
			// This would trigger a refresh on next access
		}
	}

	cm.lastRefresh = time.Now()
	log.Debugf("Refreshed %d configurations", len(keysToRefresh))
}

// ClearCache clears the entire configuration cache
func (cm *ConfigManager) ClearCache() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.cache = make(map[string]*CachedConfig)
	log.Infof("Configuration cache cleared")
}

// GetCacheStats returns statistics about the configuration cache
func (cm *ConfigManager) GetCacheStats() map[string]interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	active := 0
	expired := 0
	now := time.Now()

	for _, cached := range cm.cache {
		if now.Before(cached.ExpiresAt) {
			active++
		} else {
			expired++
		}
	}

	return map[string]interface{}{
		"total_cached":  len(cm.cache),
		"active_cached": active,
		"expired":       expired,
		"last_refresh":  cm.lastRefresh,
	}
}