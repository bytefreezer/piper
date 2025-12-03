// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package services

import (
	"sync"
	"time"
)

// TransformationMetricsTracker tracks transformation metrics for a piper instance
type TransformationMetricsTracker struct {
	mu        sync.RWMutex
	metrics   map[string]*DatasetMetrics // key: tenantID:datasetID
	startTime time.Time
}

// DatasetMetrics holds metrics for a specific dataset
type DatasetMetrics struct {
	TenantID       string
	DatasetID      string
	TotalProcessed int64
	SuccessCount   int64
	ErrorCount     int64
	SkippedCount   int64
	LastError      string
	LastProcessed  time.Time
	StartTime      time.Time
}

// NewTransformationMetricsTracker creates a new metrics tracker
func NewTransformationMetricsTracker() *TransformationMetricsTracker {
	return &TransformationMetricsTracker{
		metrics:   make(map[string]*DatasetMetrics),
		startTime: time.Now(),
	}
}

// RecordSuccess records a successful transformation
func (t *TransformationMetricsTracker) RecordSuccess(tenantID, datasetID string, count int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := tenantID + ":" + datasetID
	m := t.getOrCreateMetrics(key, tenantID, datasetID)
	m.SuccessCount += count
	m.TotalProcessed += count
	m.LastProcessed = time.Now()
}

// RecordError records a transformation error
func (t *TransformationMetricsTracker) RecordError(tenantID, datasetID string, count int64, errorMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := tenantID + ":" + datasetID
	m := t.getOrCreateMetrics(key, tenantID, datasetID)
	m.ErrorCount += count
	m.TotalProcessed += count
	m.LastError = errorMsg
	m.LastProcessed = time.Now()
}

// RecordSkip records skipped records
func (t *TransformationMetricsTracker) RecordSkip(tenantID, datasetID string, count int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := tenantID + ":" + datasetID
	m := t.getOrCreateMetrics(key, tenantID, datasetID)
	m.SkippedCount += count
	m.TotalProcessed += count
	m.LastProcessed = time.Now()
}

// GetMetrics returns current metrics for a dataset
func (t *TransformationMetricsTracker) GetMetrics(tenantID, datasetID string) *DatasetMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := tenantID + ":" + datasetID
	m, exists := t.metrics[key]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	copy := *m
	return &copy
}

// GetAllMetrics returns all dataset metrics
func (t *TransformationMetricsTracker) GetAllMetrics() []*DatasetMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	results := make([]*DatasetMetrics, 0, len(t.metrics))
	for _, m := range t.metrics {
		copy := *m
		results = append(results, &copy)
	}
	return results
}

// ResetMetrics resets metrics for a specific dataset
func (t *TransformationMetricsTracker) ResetMetrics(tenantID, datasetID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := tenantID + ":" + datasetID
	delete(t.metrics, key)
}

// getOrCreateMetrics gets or creates metrics for a dataset (must be called with lock held)
func (t *TransformationMetricsTracker) getOrCreateMetrics(key, tenantID, datasetID string) *DatasetMetrics {
	m, exists := t.metrics[key]
	if !exists {
		m = &DatasetMetrics{
			TenantID:  tenantID,
			DatasetID: datasetID,
			StartTime: time.Now(),
		}
		t.metrics[key] = m
	}
	return m
}

// CalculateAvgRowsPerSec calculates average rows per second for a dataset
func (m *DatasetMetrics) CalculateAvgRowsPerSec() float64 {
	if m == nil || m.TotalProcessed == 0 {
		return 0.0
	}

	duration := time.Since(m.StartTime).Seconds()
	if duration <= 0 {
		return 0.0
	}

	return float64(m.TotalProcessed) / duration
}
