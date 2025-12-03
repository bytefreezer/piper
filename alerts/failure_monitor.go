// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package alerts

import (
	"fmt"
	"sync"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// FailureThresholdConfig represents failure monitoring configuration
type FailureThresholdConfig struct {
	Enabled          bool    `mapstructure:"enabled"`
	FailureThreshold float64 `mapstructure:"failure_threshold"` // Percentage threshold (e.g., 20.0 for 20%)
	MinimumSamples   int     `mapstructure:"minimum_samples"`   // Minimum number of processed files before checking threshold
	WindowSize       int     `mapstructure:"window_size"`       // Number of recent jobs to track
	CheckInterval    string  `mapstructure:"check_interval"`    // How often to check thresholds (e.g., "5m")
	CooldownPeriod   string  `mapstructure:"cooldown_period"`   // Minimum time between alerts for same tenant/dataset
}

// FailureStats tracks processing statistics for a tenant/dataset
type FailureStats struct {
	TenantID       string
	DatasetID      string
	TotalProcessed int
	TotalFailed    int
	RecentResults  []bool // true = success, false = failure (circular buffer)
	LastAlertTime  time.Time
	ResultIndex    int // Current index in circular buffer
	mu             sync.RWMutex
}

// FailureMonitor monitors processing failures and sends SOC alerts when thresholds are exceeded
type FailureMonitor struct {
	config     FailureThresholdConfig
	socClient  *SOCAlertClient
	stats      map[string]*FailureStats // key: "tenantID:datasetID"
	statsMutex sync.RWMutex
	stopChan   chan struct{}
	stopOnce   sync.Once
}

// NewFailureMonitor creates a new failure monitor
func NewFailureMonitor(config FailureThresholdConfig, socClient *SOCAlertClient) *FailureMonitor {
	if config.WindowSize <= 0 {
		config.WindowSize = 100 // Default window size
	}
	if config.MinimumSamples <= 0 {
		config.MinimumSamples = 10 // Default minimum samples
	}
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 20.0 // Default 20% failure threshold
	}

	return &FailureMonitor{
		config:    config,
		socClient: socClient,
		stats:     make(map[string]*FailureStats),
		stopChan:  make(chan struct{}),
	}
}

// Start begins the failure monitoring process
func (fm *FailureMonitor) Start() error {
	if !fm.config.Enabled {
		log.Debug("Failure monitoring disabled")
		return nil
	}

	checkInterval, err := time.ParseDuration(fm.config.CheckInterval)
	if err != nil {
		checkInterval = 5 * time.Minute // Default to 5 minutes
		log.Warnf("Invalid check_interval, using default 5m: %v", err)
	}

	log.Infof("Starting failure monitor with threshold %.2f%%, window size %d, check interval %v",
		fm.config.FailureThreshold, fm.config.WindowSize, checkInterval)

	go fm.monitorLoop(checkInterval)
	return nil
}

// Stop stops the failure monitoring process
func (fm *FailureMonitor) Stop() {
	fm.stopOnce.Do(func() {
		close(fm.stopChan)
	})
}

// RecordProcessingResult records the result of a processing operation
func (fm *FailureMonitor) RecordProcessingResult(tenantID, datasetID string, success bool) {
	if !fm.config.Enabled {
		return
	}

	key := fmt.Sprintf("%s:%s", tenantID, datasetID)

	fm.statsMutex.Lock()
	defer fm.statsMutex.Unlock()

	stats, exists := fm.stats[key]
	if !exists {
		stats = &FailureStats{
			TenantID:      tenantID,
			DatasetID:     datasetID,
			RecentResults: make([]bool, fm.config.WindowSize),
		}
		fm.stats[key] = stats
	}

	stats.mu.Lock()
	defer stats.mu.Unlock()

	// Update total counters
	stats.TotalProcessed++
	if !success {
		stats.TotalFailed++
	}

	// Update circular buffer
	stats.RecentResults[stats.ResultIndex] = success
	stats.ResultIndex = (stats.ResultIndex + 1) % fm.config.WindowSize

	log.Debugf("Recorded processing result for %s/%s: success=%t (total: %d, failed: %d)",
		tenantID, datasetID, success, stats.TotalProcessed, stats.TotalFailed)
}

// monitorLoop runs the periodic threshold checking
func (fm *FailureMonitor) monitorLoop(checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fm.checkThresholds()
		case <-fm.stopChan:
			log.Debug("Failure monitor stopping")
			return
		}
	}
}

// checkThresholds checks all tenant/dataset combinations for threshold violations
func (fm *FailureMonitor) checkThresholds() {
	fm.statsMutex.RLock()
	statsSnapshot := make(map[string]*FailureStats, len(fm.stats))
	for key, stats := range fm.stats {
		statsSnapshot[key] = stats
	}
	fm.statsMutex.RUnlock()

	for _, stats := range statsSnapshot {
		fm.checkStatsThreshold(stats)
	}
}

// checkStatsThreshold checks if a specific tenant/dataset exceeds failure threshold
func (fm *FailureMonitor) checkStatsThreshold(stats *FailureStats) {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	// Don't check if we don't have enough samples
	if stats.TotalProcessed < fm.config.MinimumSamples {
		return
	}

	// Check cooldown period
	cooldownPeriod, err := time.ParseDuration(fm.config.CooldownPeriod)
	if err != nil {
		cooldownPeriod = 30 * time.Minute // Default cooldown
	}

	if time.Since(stats.LastAlertTime) < cooldownPeriod {
		return // Still in cooldown period
	}

	// Calculate failure rate from recent results
	recentFailures := 0
	recentSamples := 0

	for i := 0; i < fm.config.WindowSize; i++ {
		// Check if this slot has been used (we track by checking if we've processed enough)
		if stats.TotalProcessed >= i+1 {
			recentSamples++
			if !stats.RecentResults[i] {
				recentFailures++
			}
		}
	}

	if recentSamples < fm.config.MinimumSamples {
		return // Not enough recent samples
	}

	failureRate := float64(recentFailures) / float64(recentSamples) * 100

	log.Debugf("Checking threshold for %s/%s: failure rate %.2f%% (threshold %.2f%%, samples %d)",
		stats.TenantID, stats.DatasetID, failureRate, fm.config.FailureThreshold, recentSamples)

	if failureRate >= fm.config.FailureThreshold {
		fm.sendThresholdAlert(stats, recentFailures, recentSamples, failureRate)
	}
}

// sendThresholdAlert sends a SOC alert for threshold violation
func (fm *FailureMonitor) sendThresholdAlert(stats *FailureStats, recentFailures, recentSamples int, failureRate float64) {
	// Update last alert time to prevent spam
	stats.LastAlertTime = time.Now()

	details := fmt.Sprintf("Recent window: %d failures out of %d processed files (%.2f%% failure rate). "+
		"Total processed: %d, Total failed: %d. Window size: %d files.",
		recentFailures, recentSamples, failureRate,
		stats.TotalProcessed, stats.TotalFailed, fm.config.WindowSize)

	log.Warnf("Failure threshold exceeded for %s/%s: %.2f%% >= %.2f%%",
		stats.TenantID, stats.DatasetID, failureRate, fm.config.FailureThreshold)

	if fm.socClient != nil {
		err := fm.socClient.SendProcessingFailureAlert(
			stats.TenantID,
			stats.DatasetID,
			recentFailures,
			recentSamples,
			fm.config.FailureThreshold,
			details,
		)
		if err != nil {
			log.Errorf("Failed to send processing failure alert: %v", err)
		} else {
			log.Infof("Sent SOC alert for processing failure threshold violation: %s/%s",
				stats.TenantID, stats.DatasetID)
		}
	}
}

// GetStats returns current failure statistics for all tenant/dataset combinations
func (fm *FailureMonitor) GetStats() map[string]*FailureStats {
	fm.statsMutex.RLock()
	defer fm.statsMutex.RUnlock()

	result := make(map[string]*FailureStats, len(fm.stats))
	for key, stats := range fm.stats {
		// Create a copy to avoid race conditions
		stats.mu.RLock()
		statsCopy := &FailureStats{
			TenantID:       stats.TenantID,
			DatasetID:      stats.DatasetID,
			TotalProcessed: stats.TotalProcessed,
			TotalFailed:    stats.TotalFailed,
			LastAlertTime:  stats.LastAlertTime,
		}
		stats.mu.RUnlock()

		result[key] = statsCopy
	}
	return result
}
