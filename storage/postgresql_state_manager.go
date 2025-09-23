package storage

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	_ "github.com/lib/pq"
	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
)

// PostgreSQLStateManager manages file locks and job status using PostgreSQL
type PostgreSQLStateManager struct {
	db     *sql.DB
	schema string
}

// NewPostgreSQLStateManager creates a new PostgreSQL state manager
func NewPostgreSQLStateManager(cfg *config.PostgreSQL) (*PostgreSQLStateManager, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, cfg.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	schema := cfg.Schema
	if schema == "" {
		schema = "public"
	}

	// Validate schema name to prevent SQL injection
	if !isValidSchemaName(schema) {
		return nil, fmt.Errorf("invalid schema name: %s", schema)
	}

	sm := &PostgreSQLStateManager{
		db:     db,
		schema: schema,
	}

	// Initialize database tables
	if err := sm.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return sm, nil
}

// isValidSchemaName validates that the schema name contains only safe characters
func isValidSchemaName(schema string) bool {
	// Allow only alphanumeric characters, underscores, and hyphens
	// This prevents SQL injection through schema names
	validSchema := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	return validSchema.MatchString(schema) && len(schema) <= 63 // PostgreSQL max identifier length
}

// buildTableName safely constructs a qualified table name
func (sm *PostgreSQLStateManager) buildTableName(tableName string) string {
	// Since schema is validated at creation time, this is safe from SQL injection
	// #nosec G202 - Schema name is validated with regex pattern
	return sm.schema + "." + tableName
}

// initTables creates the necessary tables if they don't exist
func (sm *PostgreSQLStateManager) initTables() error {
	ctx := context.Background()

	// Create file locks table
	lockTableSQL := `
		CREATE TABLE IF NOT EXISTS ` + sm.buildTableName("file_locks") + ` (
			file_key VARCHAR(512) PRIMARY KEY,
			processor_type VARCHAR(50) NOT NULL,
			processor_id VARCHAR(100) NOT NULL,
			job_id VARCHAR(100) NOT NULL,
			lock_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
			ttl TIMESTAMP NOT NULL,
			lock_version INTEGER NOT NULL DEFAULT 1
		)`

	if _, err := sm.db.ExecContext(ctx, lockTableSQL); err != nil {
		return fmt.Errorf("failed to create file_locks table: %w", err)
	}

	// Create job records table
	jobTableSQL := `
		CREATE TABLE IF NOT EXISTS ` + sm.buildTableName("job_records") + ` (
			job_id VARCHAR(100) PRIMARY KEY,
			tenant_id VARCHAR(100) NOT NULL,
			dataset_id VARCHAR(100) NOT NULL,
			processor_type VARCHAR(50) NOT NULL,
			processor_id VARCHAR(100) NOT NULL,
			status VARCHAR(20) NOT NULL,
			priority INTEGER NOT NULL DEFAULT 0,
			retry_count INTEGER NOT NULL DEFAULT 0,
			max_retries INTEGER NOT NULL DEFAULT 3,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			started_at TIMESTAMP NULL,
			completed_at TIMESTAMP NULL,
			ttl TIMESTAMP NOT NULL,
			source_files TEXT[] NOT NULL DEFAULT '{}',
			output_files TEXT[] NOT NULL DEFAULT '{}',
			record_count BIGINT NOT NULL DEFAULT 0,
			processing_time_ms BIGINT NOT NULL DEFAULT 0,
			file_size_bytes BIGINT NOT NULL DEFAULT 0,
			error_message TEXT NULL,
			error_code VARCHAR(50) NULL,
			error_details JSONB NULL,
			pipeline_version VARCHAR(50) NULL,
			configuration JSONB NULL
		)`

	if _, err := sm.db.ExecContext(ctx, jobTableSQL); err != nil {
		return fmt.Errorf("failed to create job_records table: %w", err)
	}

	// Create indexes for better performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_file_locks_ttl ON " + sm.buildTableName("file_locks") + " (ttl)",
		"CREATE INDEX IF NOT EXISTS idx_job_records_status ON " + sm.buildTableName("job_records") + " (status)",
		"CREATE INDEX IF NOT EXISTS idx_job_records_tenant_dataset ON " + sm.buildTableName("job_records") + " (tenant_id, dataset_id)",
		"CREATE INDEX IF NOT EXISTS idx_job_records_created_at ON " + sm.buildTableName("job_records") + " (created_at)",
	}

	for _, indexSQL := range indexes {
		if _, err := sm.db.ExecContext(ctx, indexSQL); err != nil {
			log.Warnf("Failed to create index: %v", err)
		}
	}

	return nil
}

// AcquireFileLock attempts to acquire a lock on a file
func (sm *PostgreSQLStateManager) AcquireFileLock(ctx context.Context, fileKey, processorType, processorID, jobID string) error {
	tx, err := sm.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if lock already exists and is not expired
	var existingProcessorID string
	var ttl time.Time
	checkSQL := `
		SELECT processor_id, ttl
		FROM ` + sm.buildTableName("file_locks") + `
		WHERE file_key = $1`

	err = tx.QueryRowContext(ctx, checkSQL, fileKey).Scan(&existingProcessorID, &ttl)
	if err == nil {
		// Lock exists
		if time.Now().Before(ttl) {
			// Lock is still valid
			if existingProcessorID != processorID {
				return domain.ErrFileLocked
			}
			// We already own this lock
			return nil
		}
		// Lock is expired, we can take it
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing lock: %w", err)
	}

	// Acquire or update the lock
	lockTTL := time.Now().Add(30 * time.Minute) // 30 minute TTL
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	upsertSQL := `
		INSERT INTO ` + sm.buildTableName("file_locks") + ` (file_key, processor_type, processor_id, job_id, lock_timestamp, ttl)
		VALUES ($1, $2, $3, $4, NOW(), $5)
		ON CONFLICT (file_key) DO UPDATE SET
			processor_type = EXCLUDED.processor_type,
			processor_id = EXCLUDED.processor_id,
			job_id = EXCLUDED.job_id,
			lock_timestamp = NOW(),
			ttl = EXCLUDED.ttl,
			lock_version = file_locks.lock_version + 1`

	_, err = tx.ExecContext(ctx, upsertSQL, fileKey, processorType, processorID, jobID, lockTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	return tx.Commit()
}

// ReleaseFileLock releases a file lock
func (sm *PostgreSQLStateManager) ReleaseFileLock(ctx context.Context, fileKey, processorID string) error {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	deleteSQL := `
		DELETE FROM ` + sm.buildTableName("file_locks") + `
		WHERE file_key = $1 AND processor_id = $2`

	result, err := sm.db.ExecContext(ctx, deleteSQL, fileKey, processorID)
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		log.Warnf("No lock found to release for file %s and processor %s", fileKey, processorID)
	}

	return nil
}

// CreateJobRecord creates a new job record
func (sm *PostgreSQLStateManager) CreateJobRecord(ctx context.Context, job *domain.JobRecord) error {
	jobTTL := time.Now().Add(24 * time.Hour) // 24 hour TTL for job records

	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	insertSQL := `
		INSERT INTO ` + sm.buildTableName("job_records") + ` (
			job_id, tenant_id, dataset_id, processor_type, processor_id, status,
			priority, retry_count, max_retries, created_at, updated_at, ttl,
			source_files, file_size_bytes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := sm.db.ExecContext(ctx, insertSQL,
		job.JobID, job.TenantID, job.DatasetID, job.ProcessorType, job.ProcessorID, string(job.Status),
		job.Priority, job.RetryCount, job.MaxRetries, job.CreatedAt, job.UpdatedAt, jobTTL,
		job.SourceFiles, job.FileSize)

	if err != nil {
		return fmt.Errorf("failed to create job record: %w", err)
	}

	return nil
}

// UpdateJobStatus updates the status of a job
func (sm *PostgreSQLStateManager) UpdateJobStatus(ctx context.Context, jobID string, status domain.JobStatus) error {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	updateSQL := `
		UPDATE ` + sm.buildTableName("job_records") + `
		SET status = $1, updated_at = NOW()
		WHERE job_id = $2`

	_, err := sm.db.ExecContext(ctx, updateSQL, string(status), jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

// GetJobsByStatus retrieves jobs by status
func (sm *PostgreSQLStateManager) GetJobsByStatus(ctx context.Context, status domain.JobStatus, limit int) ([]*domain.JobRecord, error) {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	querySQL := `
		SELECT job_id, tenant_id, dataset_id, processor_type, processor_id, status,
			   priority, retry_count, max_retries, created_at, updated_at,
			   COALESCE(started_at, '1970-01-01'::timestamp),
			   COALESCE(completed_at, '1970-01-01'::timestamp),
			   source_files, output_files, record_count, processing_time_ms,
			   file_size_bytes, COALESCE(error_message, ''), COALESCE(error_code, ''),
			   COALESCE(pipeline_version, '')
		FROM ` + sm.buildTableName("job_records") + `
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := sm.db.QueryContext(ctx, querySQL, string(status), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs by status: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.JobRecord
	for rows.Next() {
		job := &domain.JobRecord{}
		var startedAt, completedAt time.Time

		err := rows.Scan(
			&job.JobID, &job.TenantID, &job.DatasetID, &job.ProcessorType, &job.ProcessorID, &job.Status,
			&job.Priority, &job.RetryCount, &job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
			&startedAt, &completedAt,
			&job.SourceFiles, &job.OutputFiles, &job.RecordCount, &job.ProcessingTime,
			&job.FileSize, &job.ErrorMessage, &job.ErrorCode, &job.PipelineVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job record: %w", err)
		}

		// Handle nullable timestamps
		if !startedAt.IsZero() && startedAt.Year() > 1970 {
			job.StartedAt = &startedAt
		}
		if !completedAt.IsZero() && completedAt.Year() > 1970 {
			job.CompletedAt = &completedAt
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// CleanupExpiredLocks removes expired file locks
func (sm *PostgreSQLStateManager) CleanupExpiredLocks(ctx context.Context) error {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	deleteSQL := `
		DELETE FROM ` + sm.buildTableName("file_locks") + `
		WHERE ttl < NOW()`

	result, err := sm.db.ExecContext(ctx, deleteSQL)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired locks: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected > 0 {
		log.Infof("Cleaned up %d expired file locks", rowsAffected)
	}

	return nil
}

// Close closes the database connection
func (sm *PostgreSQLStateManager) Close() error {
	return sm.db.Close()
}
