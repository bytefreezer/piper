package domain

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrFileNotFound = errors.New("file not found")
	ErrFileLocked   = errors.New("file is locked by another processor")
)

// ProcessingJob represents a file processing job
type ProcessingJob struct {
	JobID       string    `json:"job_id"`
	TenantID    string    `json:"tenant_id"`
	DatasetID   string    `json:"dataset_id"`
	SourceFile  S3Object  `json:"source_file"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	Status      JobStatus `json:"status"`
	ProcessorID string    `json:"processor_id"`
}

// S3Object represents an S3 object with metadata
type S3Object struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
}

// JobStatus represents the status of a processing job
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusRetrying   JobStatus = "retrying"
	JobStatusCancelled  JobStatus = "cancelled"
)

// JobRecord represents a job status record in PostgreSQL
type JobRecord struct {
	JobID           string                 `json:"job_id"`
	TenantID        string                 `json:"tenant_id"`
	DatasetID       string                 `json:"dataset_id"`
	ProcessorType   string                 `json:"processor_type"`
	ProcessorID     string                 `json:"processor_id"`
	Status          JobStatus              `json:"status"`
	Priority        int                    `json:"priority"`
	RetryCount      int                    `json:"retry_count"`
	MaxRetries      int                    `json:"max_retries"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	StartedAt       *time.Time             `json:"started_at,omitempty"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	SourceFiles     []string               `json:"source_files"`
	OutputFiles     []string               `json:"output_files"`
	RecordCount     int64                  `json:"record_count"`
	ProcessingTime  int64                  `json:"processing_time_ms"`
	FileSize        int64                  `json:"file_size_bytes"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	ErrorCode       string                 `json:"error_code,omitempty"`
	ErrorDetails    map[string]string      `json:"error_details,omitempty"`
	PipelineVersion string                 `json:"pipeline_version,omitempty"`
	Configuration   map[string]interface{} `json:"configuration,omitempty"`
}

// FilterConfig represents a single filter configuration
type FilterConfig struct {
	Type      string                 `json:"type"`
	Condition string                 `json:"condition,omitempty"`
	Config    map[string]interface{} `json:"config"`
	Enabled   bool                   `json:"enabled"`
}

// PipelineConfiguration represents pipeline settings for a tenant/dataset
type PipelineConfiguration struct {
	ConfigKey string                 `json:"config_key"`
	TenantID  string                 `json:"tenant_id"`
	DatasetID string                 `json:"dataset_id"`
	Enabled   bool                   `json:"enabled"`
	Version   string                 `json:"version"`
	Filters   []FilterConfig         `json:"filters"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	UpdatedBy string                 `json:"updated_by"`
	Checksum  string                 `json:"checksum"`
	Validated bool                   `json:"validated"`
	Settings  map[string]interface{} `json:"settings,omitempty"`
}

// ProcessingStats represents statistics for a processing operation
type ProcessingStats struct {
	InputRecords    int64                  `json:"input_records"`
	OutputRecords   int64                  `json:"output_records"`
	FilteredRecords int64                  `json:"filtered_records"`
	ErrorRecords    int64                  `json:"error_records"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	InputSize       int64                  `json:"input_size_bytes"`
	OutputSize      int64                  `json:"output_size_bytes"`
	FilterStats     map[string]FilterStats `json:"filter_stats"`
}

// FilterStats represents statistics for a specific filter
type FilterStats struct {
	AppliedCount int64         `json:"applied_count"`
	SkippedCount int64         `json:"skipped_count"`
	ErrorCount   int64         `json:"error_count"`
	AvgTime      time.Duration `json:"avg_time"`
}

// ProcessingResult represents the result of processing a file
type ProcessingResult struct {
	SourceFile   string          `json:"source_file"`
	OutputFile   string          `json:"output_file"`
	Stats        ProcessingStats `json:"stats"`
	Success      bool            `json:"success"`
	ErrorMessage string          `json:"error_message,omitempty"`
	PipelineUsed string          `json:"pipeline_used"`
}

// ServiceStatus represents the current status of the service
type ServiceStatus struct {
	ServiceType    string        `json:"service_type"`
	InstanceID     string        `json:"instance_id"`
	Status         string        `json:"status"`
	Version        string        `json:"version"`
	StartedAt      time.Time     `json:"started_at"`
	LastHeartbeat  time.Time     `json:"last_heartbeat"`
	JobsInProgress int           `json:"jobs_in_progress"`
	JobsCompleted  int64         `json:"jobs_completed"`
	JobsFailed     int64         `json:"jobs_failed"`
	QueueDepth     int           `json:"queue_depth"`
	ResourceUsage  ResourceUsage `json:"resource_usage"`
}

// ResourceUsage represents current resource utilization
type ResourceUsage struct {
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryMB     int64   `json:"memory_mb"`
	DiskMB       int64   `json:"disk_mb"`
	NetworkInMB  int64   `json:"network_in_mb"`
	NetworkOutMB int64   `json:"network_out_mb"`
}

// MetricsSnapshot represents a snapshot of service metrics
type MetricsSnapshot struct {
	Timestamp        time.Time     `json:"timestamp"`
	JobsProcessed    int64         `json:"jobs_processed"`
	RecordsProcessed int64         `json:"records_processed"`
	BytesProcessed   int64         `json:"bytes_processed"`
	AverageLatency   time.Duration `json:"average_latency"`
	ErrorRate        float64       `json:"error_rate"`
	QueueDepth       int           `json:"queue_depth"`
	ActiveWorkers    int           `json:"active_workers"`
}

// S3FileMetadata represents metadata extracted from S3 file paths
type S3FileMetadata struct {
	TenantID  string `json:"tenant_id"`
	DatasetID string `json:"dataset_id"`
	Year      string `json:"year,omitempty"`
	Month     string `json:"month,omitempty"`
	Day       string `json:"day,omitempty"`
	Hour      string `json:"hour,omitempty"`
	Filename  string `json:"filename"`
}
