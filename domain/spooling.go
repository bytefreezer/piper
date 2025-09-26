package domain

import (
	"time"
)

// RetryFile represents metadata for files in the retry system
// Following the receiver pattern for consistency
type RetryFile struct {
	TenantID         string    `json:"tenant_id"`
	DatasetID        string    `json:"dataset_id"`
	Filename         string    `json:"filename"`
	FilePath         string    `json:"file_path"`
	CompressedSize   int64     `json:"compressed_size"`
	UncompressedSize int64     `json:"uncompressed_size"`
	LineCount        int64     `json:"line_count"`
	CreatedAt        time.Time `json:"created_at"`
	Status           string    `json:"status"` // "queue", "retry", "dlq", "success"
	RetryCount       int       `json:"retry_count"`
	LastRetry        time.Time `json:"last_retry"`
	FailureReason    string    `json:"failure_reason,omitempty"`
	FilenameDebug    string    `json:"filename_debug,omitempty"`
	ProcessedAt      time.Time `json:"processed_at,omitempty"`
}

// ProcessJob represents a job for processing files
type ProcessJob struct {
	TenantID      string `json:"tenant_id"`
	DatasetID     string `json:"dataset_id"`
	SourceKey     string `json:"source_key"`     // S3 key in source bucket
	LocalFilePath string `json:"local_file_path"` // Local file path for processing
	MetaPath      string `json:"meta_path"`      // Metadata file path
	Attempts      int    `json:"attempts"`
}

// ProcessResult represents the result of a processing job
type ProcessResult struct {
	Job           ProcessJob `json:"job"`
	Success       bool       `json:"success"`
	Error         string     `json:"error,omitempty"`
	OutputPath    string     `json:"output_path,omitempty"`    // Local processed file path
	DestinationKey string    `json:"destination_key,omitempty"` // S3 key for destination bucket
	ProcessedSize int64      `json:"processed_size,omitempty"`
	LineCount     int64      `json:"line_count,omitempty"`
}

// SpoolStats represents statistics for the spooling system
type SpoolStats struct {
	TotalFiles      int                    `json:"total_files"`
	QueueFiles      int                    `json:"queue_files"`
	RetryFiles      int                    `json:"retry_files"`
	DLQFiles        int                    `json:"dlq_files"`
	ProcessingFiles int                    `json:"processing_files"`
	ByTenant        map[string]SpoolStats  `json:"by_tenant"`
	LastUpdated     time.Time              `json:"last_updated"`
}

// Stage represents the current stage of file processing
type Stage string

const (
	StageQueue Stage = "queue"
	StageRetry Stage = "retry"
	StageDLQ   Stage = "dlq"
)

// GetStageDirectory returns the directory name for the given stage
func (s Stage) GetStageDirectory() string {
	return string(s)
}