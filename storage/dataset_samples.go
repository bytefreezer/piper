package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/n0needt0/go-goodies/log"
)

// DatasetSample represents a data sample stored in the database
type DatasetSample struct {
	ID         int64                  `json:"id"`
	TenantID   string                 `json:"tenant_id"`
	DatasetID  string                 `json:"dataset_id"`
	SampleType string                 `json:"sample_type"` // "input" or "output"
	LineNumber int                    `json:"line_number"`
	SampleData map[string]interface{} `json:"sample_data"`
	BatchID    string                 `json:"batch_id"`
	CreatedAt  time.Time              `json:"created_at"`
}

// DatasetSampleClient handles dataset sample storage operations
type DatasetSampleClient struct {
	db *sql.DB
}

// NewDatasetSampleClient creates a new dataset sample client
func NewDatasetSampleClient(stateManager StateManager) *DatasetSampleClient {
	if stateManager == nil {
		return nil
	}
	// Only PostgreSQL state manager has direct DB access
	// For Control API state manager, this client is not available
	pgStateManager, ok := stateManager.(*PostgreSQLStateManager)
	if !ok || pgStateManager.db == nil {
		return nil
	}
	return &DatasetSampleClient{
		db: pgStateManager.db,
	}
}

// UpsertSamples stores samples and keeps only the latest N samples per type
// This replaces old samples with new ones
func (c *DatasetSampleClient) UpsertSamples(ctx context.Context, tenantID, datasetID, sampleType string, samples []DatasetSample, keepCount int) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("database not available")
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert new samples first
	insertQuery := `
		INSERT INTO dataset_samples (tenant_id, dataset_id, sample_type, line_number, sample_data, batch_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	for _, sample := range samples {
		sampleJSON, err := sonic.Marshal(sample.SampleData)
		if err != nil {
			log.Errorf("Failed to marshal sample data: %v", err)
			continue
		}

		_, err = tx.ExecContext(ctx, insertQuery,
			sample.TenantID,
			sample.DatasetID,
			sample.SampleType,
			sample.LineNumber,
			sampleJSON,
			sample.BatchID,
			sample.CreatedAt,
		)
		if err != nil {
			log.Errorf("Failed to insert sample: %v", err)
			continue
		}
	}

	// Now delete old samples, keeping only the latest keepCount total (including newly inserted)
	deleteQuery := `
		DELETE FROM dataset_samples
		WHERE tenant_id = $1
		  AND dataset_id = $2
		  AND sample_type = $3
		  AND id NOT IN (
			SELECT id FROM dataset_samples
			WHERE tenant_id = $1
			  AND dataset_id = $2
			  AND sample_type = $3
			ORDER BY created_at DESC
			LIMIT $4
		  )
	`
	_, err = tx.ExecContext(ctx, deleteQuery, tenantID, datasetID, sampleType, keepCount)
	if err != nil {
		return fmt.Errorf("failed to delete old samples: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Debugf("Upserted %d %s samples for %s/%s (keeping %d latest)",
		len(samples), sampleType, tenantID, datasetID, keepCount)

	return nil
}

// GetLatestSamples retrieves the latest N samples for a given type
func (c *DatasetSampleClient) GetLatestSamples(ctx context.Context, tenantID, datasetID, sampleType string, limit int) ([]DatasetSample, error) {
	if c == nil || c.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `
		SELECT id, tenant_id, dataset_id, sample_type, line_number, sample_data, batch_id, created_at
		FROM dataset_samples
		WHERE tenant_id = $1 AND dataset_id = $2 AND sample_type = $3
		ORDER BY created_at DESC
		LIMIT $4
	`

	rows, err := c.db.QueryContext(ctx, query, tenantID, datasetID, sampleType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query samples: %w", err)
	}
	defer rows.Close()

	samples := []DatasetSample{}
	for rows.Next() {
		var sample DatasetSample
		var sampleDataJSON []byte

		err := rows.Scan(
			&sample.ID,
			&sample.TenantID,
			&sample.DatasetID,
			&sample.SampleType,
			&sample.LineNumber,
			&sampleDataJSON,
			&sample.BatchID,
			&sample.CreatedAt,
		)
		if err != nil {
			log.Errorf("Failed to scan sample row: %v", err)
			continue
		}

		// Unmarshal JSON data
		if err := sonic.Unmarshal(sampleDataJSON, &sample.SampleData); err != nil {
			log.Errorf("Failed to unmarshal sample data: %v", err)
			continue
		}

		samples = append(samples, sample)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating samples: %w", err)
	}

	return samples, nil
}

// GetSampleCount returns the count of samples for a dataset
func (c *DatasetSampleClient) GetSampleCount(ctx context.Context, tenantID, datasetID string) (int, error) {
	if c == nil || c.db == nil {
		return 0, fmt.Errorf("database not available")
	}

	query := `
		SELECT COUNT(*)
		FROM dataset_samples
		WHERE tenant_id = $1 AND dataset_id = $2
	`

	var count int
	err := c.db.QueryRowContext(ctx, query, tenantID, datasetID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get sample count: %w", err)
	}

	return count, nil
}

// HasSamples checks if a dataset has any samples stored
func (c *DatasetSampleClient) HasSamples(ctx context.Context, tenantID, datasetID string) (bool, error) {
	count, err := c.GetSampleCount(ctx, tenantID, datasetID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpsertSchema stores or updates the computed schema for a dataset
func (c *DatasetSampleClient) UpsertSchema(ctx context.Context, tenantID, datasetID, schemaType string, schema interface{}) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("database not available")
	}

	// Marshal schema to JSON
	schemaJSON, err := sonic.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	query := `
		INSERT INTO dataset_schema (tenant_id, dataset_id, schema_type, schema_data, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, dataset_id, schema_type)
		DO UPDATE SET
			schema_data = EXCLUDED.schema_data,
			updated_at = EXCLUDED.updated_at
	`

	_, err = c.db.ExecContext(ctx, query,
		tenantID,
		datasetID,
		schemaType,
		schemaJSON,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert schema: %w", err)
	}

	log.Debugf("Upserted %s schema for %s/%s", schemaType, tenantID, datasetID)
	return nil
}

// GetSchema retrieves the cached schema for a dataset
func (c *DatasetSampleClient) GetSchema(ctx context.Context, tenantID, datasetID, schemaType string) ([]byte, error) {
	if c == nil || c.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	query := `
		SELECT schema_data
		FROM dataset_schema
		WHERE tenant_id = $1 AND dataset_id = $2 AND schema_type = $3
	`

	var schemaData []byte
	err := c.db.QueryRowContext(ctx, query, tenantID, datasetID, schemaType).Scan(&schemaData)
	if err == sql.ErrNoRows {
		return nil, nil // No schema cached
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	return schemaData, nil
}
