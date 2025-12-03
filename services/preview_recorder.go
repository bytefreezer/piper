// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package services

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytefreezer/goodies/log"
)

// PreviewSample represents a transformation preview sample
type PreviewSample struct {
	TenantID        string
	DatasetID       string
	LineNumber      int
	OriginalData    map[string]interface{}
	TransformedData map[string]interface{}
}

// PreviewRecorder records transformation preview samples to database
type PreviewRecorder struct {
	db              *sql.DB
	mu              sync.Mutex
	buffer          []PreviewSample
	maxBufferSize   int
	maxSamplesPerDS int // Maximum samples to keep per dataset
}

// NewPreviewRecorder creates a new preview recorder
func NewPreviewRecorder(db *sql.DB) *PreviewRecorder {
	return &PreviewRecorder{
		db:              db,
		buffer:          make([]PreviewSample, 0),
		maxBufferSize:   5,  // Flush every 5 samples
		maxSamplesPerDS: 10, // Keep only latest 10 samples per dataset
	}
}

// RecordSample adds a sample to the buffer
func (r *PreviewRecorder) RecordSample(sample PreviewSample) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buffer = append(r.buffer, sample)

	// Flush if buffer is full
	if len(r.buffer) >= r.maxBufferSize {
		r.flushLocked()
	}
}

// Flush writes buffered samples to database
func (r *PreviewRecorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushLocked()
}

// flushLocked writes samples to database (must be called with lock held)
func (r *PreviewRecorder) flushLocked() {
	if len(r.buffer) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		log.Errorf("Failed to begin preview insert transaction: %v", err)
		r.buffer = r.buffer[:0] // Clear buffer to avoid memory buildup
		return
	}
	defer tx.Rollback()

	// Prepare insert statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO piper_transformation_preview
		(tenant_id, dataset_id, line_number, original_data, transformed_data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		log.Errorf("Failed to prepare preview insert: %v", err)
		r.buffer = r.buffer[:0]
		return
	}
	defer stmt.Close()

	// Insert all samples
	now := time.Now()
	for _, sample := range r.buffer {
		originalJSON, err := sonic.Marshal(sample.OriginalData)
		if err != nil {
			log.Warnf("Failed to marshal original data for preview: %v", err)
			continue
		}

		transformedJSON, err := sonic.Marshal(sample.TransformedData)
		if err != nil {
			log.Warnf("Failed to marshal transformed data for preview: %v", err)
			continue
		}

		_, err = stmt.ExecContext(ctx,
			sample.TenantID,
			sample.DatasetID,
			sample.LineNumber,
			originalJSON,
			transformedJSON,
			now,
		)
		if err != nil {
			log.Warnf("Failed to insert preview sample: %v", err)
		}
	}

	// Cleanup old samples - keep only latest N per dataset
	for _, sample := range r.buffer {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM piper_transformation_preview
			WHERE id IN (
				SELECT id
				FROM piper_transformation_preview
				WHERE tenant_id = $1 AND dataset_id = $2
				ORDER BY created_at DESC
				OFFSET $3
			)
		`, sample.TenantID, sample.DatasetID, r.maxSamplesPerDS)
		if err != nil {
			log.Warnf("Failed to cleanup old preview samples: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Errorf("Failed to commit preview samples: %v", err)
	}

	// Clear buffer
	r.buffer = r.buffer[:0]
}
