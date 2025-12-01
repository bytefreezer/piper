package services

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytefreezer/goodies/log"
)

// MetricsReporter handles periodic reporting of transformation metrics to control service
type MetricsReporter struct {
	controlURL     string
	apiKey         string
	metricsTracker *TransformationMetricsTracker
	reportInterval time.Duration
	timeout        time.Duration
	httpClient     *http.Client
	stopChan       chan struct{}
	enabled        bool
}

// TransformationStatsReport represents the stats payload sent to control service
type TransformationStatsReport struct {
	TenantID       string    `json:"tenant_id"`
	DatasetID      string    `json:"dataset_id"`
	TotalProcessed int64     `json:"total_processed"`
	SuccessCount   int64     `json:"success_count"`
	ErrorCount     int64     `json:"error_count"`
	SkippedCount   int64     `json:"skipped_count"`
	AvgRowsPerSec  float64   `json:"avg_rows_per_sec"`
	LastError      string    `json:"last_error"`
	LastProcessed  time.Time `json:"last_processed"`
}

// NewMetricsReporter creates a new metrics reporter
func NewMetricsReporter(controlURL, apiKey string, metricsTracker *TransformationMetricsTracker, reportInterval, timeout time.Duration, enabled bool) *MetricsReporter {
	return &MetricsReporter{
		controlURL:     controlURL,
		apiKey:         apiKey,
		metricsTracker: metricsTracker,
		reportInterval: reportInterval,
		timeout:        timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		stopChan: make(chan struct{}),
		enabled:  enabled,
	}
}

// Start begins periodic metrics reporting
func (m *MetricsReporter) Start(ctx context.Context) {
	if !m.enabled {
		log.Info("Metrics reporting is disabled")
		return
	}

	log.Infof("Starting transformation metrics reporter - reporting to %s every %v", m.controlURL, m.reportInterval)

	go m.reportingLoop(ctx)
}

// Stop stops the metrics reporter
func (m *MetricsReporter) Stop() {
	if m.enabled {
		close(m.stopChan)
		log.Info("Metrics reporter stopped")
	}
}

// reportingLoop runs the periodic metrics reporting
func (m *MetricsReporter) reportingLoop(ctx context.Context) {
	ticker := time.NewTicker(m.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Metrics reporting stopped due to context cancellation")
			return
		case <-m.stopChan:
			log.Info("Metrics reporting stopped")
			return
		case <-ticker.C:
			if err := m.reportMetrics(ctx); err != nil {
				log.Debugf("Failed to report metrics: %v", err)
			}
		}
	}
}

// reportMetrics sends current metrics to the control service
func (m *MetricsReporter) reportMetrics(ctx context.Context) error {
	if m.metricsTracker == nil {
		return nil
	}

	// Get all current metrics
	allMetrics := m.metricsTracker.GetAllMetrics()
	if len(allMetrics) == 0 {
		log.Debugf("No metrics to report")
		return nil
	}

	// Report each dataset's metrics
	successCount := 0
	errorCount := 0

	for _, metrics := range allMetrics {
		if err := m.sendMetrics(ctx, metrics); err != nil {
			log.Debugf("Failed to send metrics for %s/%s: %v", metrics.TenantID, metrics.DatasetID, err)
			errorCount++
		} else {
			successCount++
		}
	}

	if successCount > 0 {
		log.Debugf("Successfully reported metrics for %d datasets", successCount)
	}
	if errorCount > 0 {
		log.Debugf("Failed to report metrics for %d datasets", errorCount)
	}

	return nil
}

// sendMetrics sends metrics for a single dataset to the control service
func (m *MetricsReporter) sendMetrics(ctx context.Context, metrics *DatasetMetrics) error {
	if metrics == nil {
		return nil
	}

	// Calculate average rows per second
	avgRowsPerSec := metrics.CalculateAvgRowsPerSec()

	// Create report payload
	report := TransformationStatsReport{
		TenantID:       metrics.TenantID,
		DatasetID:      metrics.DatasetID,
		TotalProcessed: metrics.TotalProcessed,
		SuccessCount:   metrics.SuccessCount,
		ErrorCount:     metrics.ErrorCount,
		SkippedCount:   metrics.SkippedCount,
		AvgRowsPerSec:  avgRowsPerSec,
		LastError:      metrics.LastError,
		LastProcessed:  metrics.LastProcessed,
	}

	// Marshal to JSON
	reqBody, err := sonic.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Create HTTP request
	endpoint := m.controlURL + "/api/v1/transformations/stats"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
	}

	// Send request
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send metrics: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics report failed with status %d", resp.StatusCode)
	}

	log.Debugf("Successfully reported metrics for %s/%s (processed: %d, success: %d, error: %d, avg: %.2f rows/sec)",
		metrics.TenantID, metrics.DatasetID, metrics.TotalProcessed, metrics.SuccessCount, metrics.ErrorCount, avgRowsPerSec)

	return nil
}
