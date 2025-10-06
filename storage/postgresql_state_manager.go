package storage

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
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
		CREATE TABLE IF NOT EXISTS ` + sm.buildTableName("piper_file_locks") + ` (
			file_key VARCHAR(512) PRIMARY KEY,
			processor_type VARCHAR(50) NOT NULL,
			processor_id VARCHAR(100) NOT NULL,
			job_id VARCHAR(100) NOT NULL,
			lock_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
			ttl TIMESTAMP NOT NULL,
			lock_version INTEGER NOT NULL DEFAULT 1
		)`

	if _, err := sm.db.ExecContext(ctx, lockTableSQL); err != nil {
		return fmt.Errorf("failed to create piper_file_locks table: %w", err)
	}

	// Create job records table
	jobTableSQL := `
		CREATE TABLE IF NOT EXISTS ` + sm.buildTableName("piper_job_records") + ` (
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
		return fmt.Errorf("failed to create piper_job_records table: %w", err)
	}

	// Create pipeline configurations cache table
	pipelineTableSQL := `
		CREATE TABLE IF NOT EXISTS ` + sm.buildTableName("piper_pipeline_configurations") + ` (
			config_key VARCHAR(200) PRIMARY KEY,
			tenant_id VARCHAR(100) NOT NULL,
			dataset_id VARCHAR(100) NOT NULL,
			configuration JSONB NOT NULL,
			version VARCHAR(50) NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			cached_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL,
			filter_count INTEGER NOT NULL DEFAULT 0
		)`

	if _, err := sm.db.ExecContext(ctx, pipelineTableSQL); err != nil {
		return fmt.Errorf("failed to create piper_pipeline_configurations table: %w", err)
	}

	// Create tenants cache table
	tenantsTableSQL := `
		CREATE TABLE IF NOT EXISTS ` + sm.buildTableName("piper_tenants_cache") + ` (
			tenant_id VARCHAR(100) PRIMARY KEY,
			name VARCHAR(200) NOT NULL,
			datasets TEXT[] NOT NULL DEFAULT '{}',
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			cached_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL
		)`

	if _, err := sm.db.ExecContext(ctx, tenantsTableSQL); err != nil {
		return fmt.Errorf("failed to create piper_tenants_cache table: %w", err)
	}

	// Create indexes for better performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_piper_file_locks_ttl ON " + sm.buildTableName("piper_file_locks") + " (ttl)",
		"CREATE INDEX IF NOT EXISTS idx_piper_job_records_status ON " + sm.buildTableName("piper_job_records") + " (status)",
		"CREATE INDEX IF NOT EXISTS idx_piper_job_records_tenant_dataset ON " + sm.buildTableName("piper_job_records") + " (tenant_id, dataset_id)",
		"CREATE INDEX IF NOT EXISTS idx_piper_job_records_created_at ON " + sm.buildTableName("piper_job_records") + " (created_at)",
		"CREATE INDEX IF NOT EXISTS idx_piper_pipeline_configurations_tenant_dataset ON " + sm.buildTableName("piper_pipeline_configurations") + " (tenant_id, dataset_id)",
		"CREATE INDEX IF NOT EXISTS idx_piper_pipeline_configurations_expires_at ON " + sm.buildTableName("piper_pipeline_configurations") + " (expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_piper_tenants_cache_expires_at ON " + sm.buildTableName("piper_tenants_cache") + " (expires_at)",
		"CREATE INDEX IF NOT EXISTS idx_piper_tenants_cache_active ON " + sm.buildTableName("piper_tenants_cache") + " (active)",
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
	return sm.AcquireFileLockWithTTL(ctx, fileKey, processorType, processorID, jobID, 30*time.Minute)
}

// AcquireFileLockWithTTL attempts to acquire a lock on a file with custom TTL
func (sm *PostgreSQLStateManager) AcquireFileLockWithTTL(ctx context.Context, fileKey, processorType, processorID, jobID string, ttl time.Duration) error {
	tx, err := sm.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if lock already exists and is not expired
	var existingProcessorID string
	var existingTTL time.Time
	checkSQL := `
		SELECT processor_id, ttl
		FROM ` + sm.buildTableName("piper_file_locks") + `
		WHERE file_key = $1`

	err = tx.QueryRowContext(ctx, checkSQL, fileKey).Scan(&existingProcessorID, &existingTTL)
	if err == nil {
		// Lock exists
		if time.Now().Before(existingTTL) {
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
	lockTTL := time.Now().Add(ttl)
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	upsertSQL := `
		INSERT INTO ` + sm.buildTableName("piper_file_locks") + ` (file_key, processor_type, processor_id, job_id, lock_timestamp, ttl, instance_id)
		VALUES ($1, $2, $3, $4, NOW(), $5, $6)
		ON CONFLICT (file_key) DO UPDATE SET
			processor_type = EXCLUDED.processor_type,
			processor_id = EXCLUDED.processor_id,
			job_id = EXCLUDED.job_id,
			lock_timestamp = NOW(),
			ttl = EXCLUDED.ttl,
			instance_id = EXCLUDED.instance_id,
			lock_version = ` + sm.buildTableName("piper_file_locks") + `.lock_version + 1`

	_, err = tx.ExecContext(ctx, upsertSQL, fileKey, processorType, processorID, jobID, lockTTL, processorID)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	return tx.Commit()
}

// ReleaseFileLock releases a file lock
func (sm *PostgreSQLStateManager) ReleaseFileLock(ctx context.Context, fileKey, processorID string) error {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	deleteSQL := `
		DELETE FROM ` + sm.buildTableName("piper_file_locks") + `
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
		INSERT INTO ` + sm.buildTableName("piper_job_records") + ` (
			job_id, tenant_id, dataset_id, processor_type, processor_id, status,
			priority, retry_count, max_retries, created_at, updated_at, ttl,
			source_files, file_size_bytes, instance_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	_, err := sm.db.ExecContext(ctx, insertSQL,
		job.JobID, job.TenantID, job.DatasetID, job.ProcessorType, job.ProcessorID, string(job.Status),
		job.Priority, job.RetryCount, job.MaxRetries, job.CreatedAt, job.UpdatedAt, jobTTL,
		pq.Array(job.SourceFiles), job.FileSize, job.ProcessorID)

	if err != nil {
		return fmt.Errorf("failed to create job record: %w", err)
	}

	return nil
}

// UpdateJobStatus updates the status of a job
func (sm *PostgreSQLStateManager) UpdateJobStatus(ctx context.Context, jobID string, status domain.JobStatus) error {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	updateSQL := `
		UPDATE ` + sm.buildTableName("piper_job_records") + `
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
		FROM ` + sm.buildTableName("piper_job_records") + `
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
		DELETE FROM ` + sm.buildTableName("piper_file_locks") + `
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

// CleanupStaleLocksOnStartup removes locks from previous instances of the same base processor
func (sm *PostgreSQLStateManager) CleanupStaleLocksOnStartup(ctx context.Context, currentInstanceID string) error {
	// Extract base instance ID (everything before the first hyphen after "piper-")
	// Example: "piper-192-168-1-100-12345-1234567890" -> "piper-192-168-1-100"
	baseID := extractBaseInstanceID(currentInstanceID)
	if baseID == "" {
		log.Warnf("Could not extract base ID from instance ID: %s", currentInstanceID)
		return nil
	}

	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	deleteSQL := `
		DELETE FROM ` + sm.buildTableName("piper_file_locks") + `
		WHERE processor_id LIKE $1 AND processor_id != $2`

	result, err := sm.db.ExecContext(ctx, deleteSQL, baseID+"-%", currentInstanceID)
	if err != nil {
		return fmt.Errorf("failed to cleanup stale locks: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected > 0 {
		log.Infof("Cleaned up %d stale locks from previous instances (base: %s)", rowsAffected, baseID)
	}

	return nil
}

// extractBaseInstanceID extracts the base instance ID without PID and timestamp
// Example: "piper-192-168-1-100-12345-1234567890" -> "piper-192-168-1-100"
// Example: "piper-hostname-12345-1234567890" -> "piper-hostname"
func extractBaseInstanceID(instanceID string) string {
	parts := strings.Split(instanceID, "-")
	if len(parts) < 3 {
		return ""
	}

	// Check if we have the expected format: piper-{base}-{pid}-{timestamp}
	// The base part can contain hyphens (like IP addresses), so we need to find
	// the last two numeric parts which should be PID and timestamp
	if len(parts) >= 3 {
		// Try to identify numeric suffix (PID-timestamp)
		lastIdx := len(parts) - 1
		secondLastIdx := len(parts) - 2

		// Check if last two parts are numeric (PID and timestamp)
		if isNumeric(parts[lastIdx]) && isNumeric(parts[secondLastIdx]) {
			// Join everything except the last two numeric parts
			baseparts := parts[:secondLastIdx]
			return strings.Join(baseparts, "-")
		}
	}

	// Fallback: assume only one hyphen separates base from suffix
	// This handles cases like "piper-hostname-12345-1234567890"
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-2], "-")
	}

	return ""
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CachePipelineConfiguration caches a pipeline configuration
func (sm *PostgreSQLStateManager) CachePipelineConfiguration(ctx context.Context, configKey, tenantID, datasetID, version string, configuration []byte, filterCount int, instanceID string) error {
	expiresAt := time.Now().Add(5 * time.Minute) // 5 minute cache

	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	upsertSQL := `
		INSERT INTO ` + sm.buildTableName("piper_pipeline_configurations") + ` (
			config_key, tenant_id, dataset_id, configuration, version,
			enabled, created_at, updated_at, cached_at, expires_at, filter_count, instance_id
		) VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW(), NOW(), $6, $7, $8)
		ON CONFLICT (config_key) DO UPDATE SET
			configuration = EXCLUDED.configuration,
			version = EXCLUDED.version,
			updated_at = NOW(),
			cached_at = NOW(),
			expires_at = EXCLUDED.expires_at,
			filter_count = EXCLUDED.filter_count,
			instance_id = EXCLUDED.instance_id`

	_, err := sm.db.ExecContext(ctx, upsertSQL, configKey, tenantID, datasetID, configuration, version, expiresAt, filterCount, instanceID)
	if err != nil {
		return fmt.Errorf("failed to cache pipeline configuration: %w", err)
	}

	return nil
}

// GetCachedPipelineConfiguration retrieves a cached pipeline configuration
func (sm *PostgreSQLStateManager) GetCachedPipelineConfiguration(ctx context.Context, configKey string) ([]byte, bool, error) {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	querySQL := `
		SELECT configuration, expires_at
		FROM ` + sm.buildTableName("piper_pipeline_configurations") + `
		WHERE config_key = $1 AND enabled = true`

	var configuration []byte
	var expiresAt time.Time

	err := sm.db.QueryRowContext(ctx, querySQL, configKey).Scan(&configuration, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get cached pipeline configuration: %w", err)
	}

	// Check if expired
	if time.Now().After(expiresAt) {
		return nil, false, nil
	}

	return configuration, true, nil
}

// CacheTenant caches tenant information
func (sm *PostgreSQLStateManager) CacheTenant(ctx context.Context, tenantID, name string, datasets []string, active bool, instanceID string) error {
	expiresAt := time.Now().Add(10 * time.Minute) // 10 minute cache for tenants

	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	upsertSQL := `
		INSERT INTO ` + sm.buildTableName("piper_tenants_cache") + ` (
			tenant_id, name, datasets, active, created_at, updated_at, cached_at, expires_at, instance_id
		) VALUES ($1, $2, $3, $4, NOW(), NOW(), NOW(), $5, $6)
		ON CONFLICT (tenant_id) DO UPDATE SET
			name = EXCLUDED.name,
			datasets = EXCLUDED.datasets,
			active = EXCLUDED.active,
			updated_at = NOW(),
			cached_at = NOW(),
			expires_at = EXCLUDED.expires_at,
			instance_id = EXCLUDED.instance_id`

	_, err := sm.db.ExecContext(ctx, upsertSQL, tenantID, name, pq.Array(datasets), active, expiresAt, instanceID)
	if err != nil {
		return fmt.Errorf("failed to cache tenant: %w", err)
	}

	return nil
}

// GetCachedTenants retrieves all cached active tenants
func (sm *PostgreSQLStateManager) GetCachedTenants(ctx context.Context) ([]map[string]interface{}, error) {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	querySQL := `
		SELECT tenant_id, name, datasets, active, created_at, updated_at, cached_at, expires_at
		FROM ` + sm.buildTableName("piper_tenants_cache") + `
		WHERE active = true AND expires_at > NOW()
		ORDER BY tenant_id`

	rows, err := sm.db.QueryContext(ctx, querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached tenants: %w", err)
	}
	defer rows.Close()

	var tenants []map[string]interface{}
	for rows.Next() {
		var tenantID, name string
		var datasets pq.StringArray
		var active bool
		var createdAt, updatedAt, cachedAt, expiresAt time.Time

		err := rows.Scan(&tenantID, &name, &datasets, &active, &createdAt, &updatedAt, &cachedAt, &expiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tenant: %w", err)
		}

		tenant := map[string]interface{}{
			"tenant_id":  tenantID,
			"name":       name,
			"datasets":   []string(datasets),
			"active":     active,
			"created_at": createdAt.Format(time.RFC3339),
		}
		tenants = append(tenants, tenant)
	}

	return tenants, nil
}

// GetCachedPipelineList retrieves all cached pipeline configurations
func (sm *PostgreSQLStateManager) GetCachedPipelineList(ctx context.Context) ([]map[string]interface{}, error) {
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	querySQL := `
		SELECT config_key, tenant_id, dataset_id, version, enabled,
			   created_at, updated_at, cached_at, expires_at, filter_count
		FROM ` + sm.buildTableName("piper_pipeline_configurations") + `
		WHERE enabled = true
		ORDER BY tenant_id, dataset_id`

	rows, err := sm.db.QueryContext(ctx, querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached pipeline list: %w", err)
	}
	defer rows.Close()

	var pipelines []map[string]interface{}
	for rows.Next() {
		var configKey, tenantID, datasetID, version string
		var enabled bool
		var createdAt, updatedAt, cachedAt, expiresAt time.Time
		var filterCount int

		err := rows.Scan(&configKey, &tenantID, &datasetID, &version, &enabled, &createdAt, &updatedAt, &cachedAt, &expiresAt, &filterCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pipeline: %w", err)
		}

		pipeline := map[string]interface{}{
			"config_key":    configKey,
			"tenant_id":     tenantID,
			"dataset_id":    datasetID,
			"version":       version,
			"enabled":       enabled,
			"date_created":  createdAt,
			"date_modified": updatedAt,
			"cached_at":     cachedAt,
			"expires_at":    expiresAt,
			"filter_count":  filterCount,
		}
		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// CleanupExpiredCache removes expired pipeline configurations and tenants
func (sm *PostgreSQLStateManager) CleanupExpiredCache(ctx context.Context) error {
	// Clean up expired pipeline configurations
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	deletePipelineSQL := `
		DELETE FROM ` + sm.buildTableName("piper_pipeline_configurations") + `
		WHERE expires_at < NOW()`

	result, err := sm.db.ExecContext(ctx, deletePipelineSQL)
	if err != nil {
		log.Warnf("Failed to cleanup expired pipeline configurations: %v", err)
	} else {
		if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
			log.Infof("Cleaned up %d expired pipeline configurations", rowsAffected)
		}
	}

	// Clean up expired tenants
	// #nosec G202 - Schema name is validated with regex pattern in NewPostgreSQLStateManager
	deleteTenantsSQL := `
		DELETE FROM ` + sm.buildTableName("piper_tenants_cache") + `
		WHERE expires_at < NOW()`

	result, err = sm.db.ExecContext(ctx, deleteTenantsSQL)
	if err != nil {
		log.Warnf("Failed to cleanup expired tenants: %v", err)
	} else {
		if rowsAffected, err := result.RowsAffected(); err == nil && rowsAffected > 0 {
			log.Infof("Cleaned up %d expired tenants", rowsAffected)
		}
	}

	return nil
}

// Close closes the database connection
func (sm *PostgreSQLStateManager) Close() error {
	return sm.db.Close()
}
