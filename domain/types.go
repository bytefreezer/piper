package domain

import (
	"time"
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
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusRetrying   JobStatus = "retrying"
	JobStatusCancelled  JobStatus = "cancelled"
)

// FileLock represents a file processing lock in DynamoDB
type FileLock struct {
	FileKey         string    `dynamodbav:"file_key"`
	ProcessorType   string    `dynamodbav:"processor_type"`
	ProcessorID     string    `dynamodbav:"processor_id"`
	JobID           string    `dynamodbav:"job_id"`
	LockTimestamp   time.Time `dynamodbav:"lock_timestamp"`
	TTL             int64     `dynamodbav:"ttl"`
	LockVersion     int       `dynamodbav:"lock_version"`
}

// JobRecord represents a job status record in DynamoDB
type JobRecord struct {
	JobID           string            `dynamodbav:"job_id"`
	TenantID        string            `dynamodbav:"tenant_id"`
	DatasetID       string            `dynamodbav:"dataset_id"`
	ProcessorType   string            `dynamodbav:"processor_type"`
	ProcessorID     string            `dynamodbav:"processor_id"`
	Status          string            `dynamodbav:"status"`
	Priority        int               `dynamodbav:"priority"`
	RetryCount      int               `dynamodbav:"retry_count"`
	MaxRetries      int               `dynamodbav:"max_retries"`
	CreatedAt       time.Time         `dynamodbav:"created_at"`
	UpdatedAt       time.Time         `dynamodbav:"updated_at"`
	StartedAt       *time.Time        `dynamodbav:"started_at,omitempty"`
	CompletedAt     *time.Time        `dynamodbav:"completed_at,omitempty"`
	TTL             int64             `dynamodbav:"ttl"`
	SourceFiles     []string          `dynamodbav:"source_files"`
	OutputFiles     []string          `dynamodbav:"output_files"`
	RecordCount     int64             `dynamodbav:"record_count"`
	ProcessingTime  int64             `dynamodbav:"processing_time_ms"`
	FileSize        int64             `dynamodbav:"file_size_bytes"`
	ErrorMessage    string            `dynamodbav:"error_message,omitempty"`
	ErrorCode       string            `dynamodbav:"error_code,omitempty"`
	ErrorDetails    map[string]string `dynamodbav:"error_details,omitempty"`
	PipelineVersion string            `dynamodbav:"pipeline_version,omitempty"`
	Configuration   map[string]interface{} `dynamodbav:"configuration,omitempty"`
}

// PipelineConfiguration represents pipeline settings for a tenant/dataset
type PipelineConfiguration struct {
	ConfigKey    string                 `dynamodbav:"config_key"`
	TenantID     string                 `dynamodbav:"tenant_id"`
	DatasetID    string                 `dynamodbav:"dataset_id"`
	Enabled      bool                   `dynamodbav:"enabled"`
	Version      string                 `dynamodbav:"version"`
	Filters      []FilterConfig         `dynamodbav:"filters"`
	CreatedAt    time.Time              `dynamodbav:"created_at"`
	UpdatedAt    time.Time              `dynamodbav:"updated_at"`
	UpdatedBy    string                 `dynamodbav:"updated_by"`
	Checksum     string                 `dynamodbav:"checksum"`
	Validated    bool                   `dynamodbav:"validated"`
	Settings     map[string]interface{} `dynamodbav:"settings,omitempty"`
}

// FilterConfig represents a single filter configuration
type FilterConfig struct {
	Type      string                 `json:"type"`
	Condition string                 `json:"condition,omitempty"`
	Config    map[string]interface{} `json:"config"`
	Enabled   bool                   `json:"enabled"`
}

// ServiceInstance represents a service instance registration
type ServiceInstance struct {
	CoordinationKey string                 `dynamodbav:"coordination_key"`
	ServiceType     string                 `dynamodbav:"service_type"`
	InstanceID      string                 `dynamodbav:"instance_id"`
	Status          string                 `dynamodbav:"status"`
	Version         string                 `dynamodbav:"version"`
	StartedAt       time.Time              `dynamodbav:"started_at"`
	LastHeartbeat   time.Time              `dynamodbav:"last_heartbeat"`
	TTL             int64                  `dynamodbav:"ttl"`
	Endpoint        string                 `dynamodbav:"endpoint,omitempty"`
	Capabilities    []string               `dynamodbav:"capabilities,omitempty"`
	Configuration   map[string]interface{} `dynamodbav:"configuration,omitempty"`
}

// OperationalControl represents operational control settings
type OperationalControl struct {
	CoordinationKey string    `dynamodbav:"coordination_key"`
	ServiceType     string    `dynamodbav:"service_type"`
	ControlType     string    `dynamodbav:"control_type"`
	Value           string    `dynamodbav:"value"`
	UpdatedAt       time.Time `dynamodbav:"updated_at"`
	UpdatedBy       string    `dynamodbav:"updated_by"`
	Reason          string    `dynamodbav:"reason,omitempty"`
}

// ProcessingStats represents statistics for a processing operation
type ProcessingStats struct {
	InputRecords    int64         `json:"input_records"`
	OutputRecords   int64         `json:"output_records"`
	FilteredRecords int64         `json:"filtered_records"`
	ErrorRecords    int64         `json:"error_records"`
	ProcessingTime  time.Duration `json:"processing_time"`
	InputSize       int64         `json:"input_size_bytes"`
	OutputSize      int64         `json:"output_size_bytes"`
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
	SourceFile    string           `json:"source_file"`
	OutputFile    string           `json:"output_file"`
	Stats         ProcessingStats  `json:"stats"`
	Success       bool             `json:"success"`
	ErrorMessage  string           `json:"error_message,omitempty"`
	PipelineUsed  string           `json:"pipeline_used"`
}

// ServiceStatus represents the current status of the service
type ServiceStatus struct {
	ServiceType     string    `json:"service_type"`
	InstanceID      string    `json:"instance_id"`
	Status          string    `json:"status"`
	Version         string    `json:"version"`
	StartedAt       time.Time `json:"started_at"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`
	JobsInProgress  int       `json:"jobs_in_progress"`
	JobsCompleted   int64     `json:"jobs_completed"`
	JobsFailed      int64     `json:"jobs_failed"`
	QueueDepth      int       `json:"queue_depth"`
	ResourceUsage   ResourceUsage `json:"resource_usage"`
}

// ResourceUsage represents current resource utilization
type ResourceUsage struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryMB      int64   `json:"memory_mb"`
	DiskMB        int64   `json:"disk_mb"`
	NetworkInMB   int64   `json:"network_in_mb"`
	NetworkOutMB  int64   `json:"network_out_mb"`
}

// MetricsSnapshot represents a snapshot of service metrics
type MetricsSnapshot struct {
	Timestamp       time.Time `json:"timestamp"`
	JobsProcessed   int64     `json:"jobs_processed"`
	RecordsProcessed int64    `json:"records_processed"`
	BytesProcessed  int64     `json:"bytes_processed"`
	AverageLatency  time.Duration `json:"average_latency"`
	ErrorRate       float64   `json:"error_rate"`
	QueueDepth      int       `json:"queue_depth"`
	ActiveWorkers   int       `json:"active_workers"`
}