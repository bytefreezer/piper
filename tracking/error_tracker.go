package tracking

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ErrorTracker handles centralized error tracking with adaptive sampling
type ErrorTracker struct {
	controlURL string
	apiKey     string
	component  string
	client     *http.Client
	logger     *zap.Logger

	// Local error counters for client-side sampling
	errorCounts sync.Map
	mu          sync.RWMutex
}

// ErrorRecord represents an error to be tracked
type ErrorRecord struct {
	ErrorType    string                 `json:"error_type"`
	ErrorMessage string                 `json:"error_message"`
	TenantID     string                 `json:"tenant_id,omitempty"`
	DatasetID    string                 `json:"dataset_id,omitempty"`
	Severity     string                 `json:"severity"`     // debug, info, warning, error, critical
	ErrorSample  map[string]interface{} `json:"error_sample"` // Context, stack trace, etc.
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// errorCounter tracks local error occurrences for client-side sampling
type errorCounter struct {
	count      int64
	firstSeen  time.Time
	lastSeen   time.Time
	sampleRate float64
}

// NewErrorTracker creates a new error tracker client
func NewErrorTracker(controlURL, apiKey, component string, logger *zap.Logger) *ErrorTracker {
	return &ErrorTracker{
		controlURL: strings.TrimSuffix(controlURL, "/"),
		apiKey:     apiKey,
		component:  component,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// TrackError records an error with adaptive sampling
func (t *ErrorTracker) TrackError(ctx context.Context, record ErrorRecord) error {
	// Generate error hash for deduplication
	errorHash := t.generateErrorHash(record.ErrorType, record.ErrorMessage)

	// Check local sampling
	shouldSample := t.shouldSampleLocally(errorHash)
	if !shouldSample {
		t.logger.Debug("Error dropped due to local sampling",
			zap.String("error_hash", errorHash),
			zap.String("error_type", record.ErrorType))
		return nil
	}

	// Prepare request
	payload := map[string]interface{}{
		"error_hash":    errorHash,
		"error_type":    record.ErrorType,
		"component":     t.component,
		"tenant_id":     record.TenantID,
		"dataset_id":    record.DatasetID,
		"error_message": record.ErrorMessage,
		"error_sample":  record.ErrorSample,
		"severity":      record.Severity,
		"metadata":      record.Metadata,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal error record: %w", err)
	}

	// Send to control service (async via goroutine to avoid blocking)
	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := t.sendError(asyncCtx, jsonData); err != nil {
			t.logger.Warn("Failed to send error to control service",
				zap.Error(err),
				zap.String("error_type", record.ErrorType))
		}
	}()

	return nil
}

// TrackPipelineError is a convenience method for pipeline processing errors
func (t *ErrorTracker) TrackPipelineError(ctx context.Context, tenantID, datasetID string, lineNumber int64, err error) error {
	return t.TrackError(ctx, ErrorRecord{
		ErrorType:    "pipeline_processing",
		ErrorMessage: err.Error(),
		TenantID:     tenantID,
		DatasetID:    datasetID,
		Severity:     "error",
		ErrorSample: map[string]interface{}{
			"line_number": lineNumber,
			"timestamp":   time.Now().Unix(),
		},
	})
}

// TrackS3Error is a convenience method for S3-related errors
func (t *ErrorTracker) TrackS3Error(ctx context.Context, tenantID, datasetID, operation, key string, err error) error {
	return t.TrackError(ctx, ErrorRecord{
		ErrorType:    "s3_operation",
		ErrorMessage: err.Error(),
		TenantID:     tenantID,
		DatasetID:    datasetID,
		Severity:     "error",
		ErrorSample: map[string]interface{}{
			"operation": operation,
			"s3_key":    key,
			"timestamp": time.Now().Unix(),
		},
	})
}

// TrackValidationError is a convenience method for validation errors
func (t *ErrorTracker) TrackValidationError(ctx context.Context, tenantID, datasetID, field string, err error) error {
	return t.TrackError(ctx, ErrorRecord{
		ErrorType:    "validation",
		ErrorMessage: err.Error(),
		TenantID:     tenantID,
		DatasetID:    datasetID,
		Severity:     "warning",
		ErrorSample: map[string]interface{}{
			"field":     field,
			"timestamp": time.Now().Unix(),
		},
	})
}

// generateErrorHash creates a consistent hash for error deduplication
func (t *ErrorTracker) generateErrorHash(errorType, errorMessage string) string {
	// Normalize error message by removing variable parts (line numbers, timestamps, etc.)
	normalized := t.normalizeErrorMessage(errorMessage)

	// Create hash from component + error_type + normalized_message
	hashInput := fmt.Sprintf("%s:%s:%s", t.component, errorType, normalized)
	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}

// normalizeErrorMessage removes variable parts from error messages for better deduplication
func (t *ErrorTracker) normalizeErrorMessage(message string) string {
	// Remove common variable patterns
	message = strings.TrimSpace(message)

	// TODO: Add more normalization patterns as needed:
	// - Remove file paths: /path/to/file.go:123
	// - Remove timestamps: 2025-10-27T12:34:56Z
	// - Remove IDs: id=123456
	// - Remove IP addresses: 192.168.1.1

	// For now, just truncate very long messages to first 500 chars
	if len(message) > 500 {
		message = message[:500]
	}

	return message
}

// shouldSampleLocally determines if an error should be sampled based on local counts
func (t *ErrorTracker) shouldSampleLocally(errorHash string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// Load existing counter
	value, exists := t.errorCounts.Load(errorHash)
	var counter *errorCounter
	if !exists {
		counter = &errorCounter{
			count:      1,
			firstSeen:  now,
			lastSeen:   now,
			sampleRate: 1.0, // 100% initially
		}
		t.errorCounts.Store(errorHash, counter)
		return true // Always sample first occurrence
	}

	counter = value.(*errorCounter)
	counter.count++
	counter.lastSeen = now

	// Adaptive sampling based on count
	// After 100 occurrences, sample 10%
	// After 1000 occurrences, sample 1%
	// After 10000 occurrences, sample 0.1%
	if counter.count >= 10000 {
		counter.sampleRate = 0.001
	} else if counter.count >= 1000 {
		counter.sampleRate = 0.01
	} else if counter.count >= 100 {
		counter.sampleRate = 0.1
	} else {
		counter.sampleRate = 1.0
	}

	// Use simple random sampling
	// In production, consider using reservoir sampling for more sophisticated approaches
	sample := (float64(time.Now().UnixNano()%10000) / 10000.0) <= counter.sampleRate

	return sample
}

// sendError sends error data to the control service
func (t *ErrorTracker) sendError(ctx context.Context, jsonData []byte) error {
	url := fmt.Sprintf("%s/api/v1/errors/track", t.controlURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("X-API-Key", t.apiKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("control service returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close performs cleanup (e.g., flushing pending errors)
func (t *ErrorTracker) Close() error {
	// Future: Flush any pending errors
	return nil
}
