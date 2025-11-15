package storage

import (
	"context"
	"time"

	"github.com/n0needt0/bytefreezer-piper/domain"
)

// StateManager defines the interface for state management operations
type StateManager interface {
	// File locking
	AcquireFileLock(ctx context.Context, fileKey, processorType, processorID, jobID string) error
	AcquireFileLockWithTTL(ctx context.Context, fileKey, processorType, processorID, jobID string, ttl time.Duration) error
	ReleaseFileLock(ctx context.Context, fileKey, processorID string) error
	CleanupExpiredLocks(ctx context.Context) error
	CleanupStaleLocksOnStartup(ctx context.Context, currentInstanceID string) error
	CleanupInstanceLocks(ctx context.Context, instanceID string) error

	// Job management
	CreateJobRecord(ctx context.Context, job *domain.JobRecord) error
	UpdateJobStatus(ctx context.Context, jobID string, status domain.JobStatus) error
	GetJobsByStatus(ctx context.Context, status domain.JobStatus, limit int) ([]*domain.JobRecord, error)
	CleanupExpiredJobRecords(ctx context.Context) error

	// Pipeline configuration caching
	CachePipelineConfiguration(ctx context.Context, configKey, tenantID, datasetID, version string, configuration []byte, filterCount int, instanceID string) error
	GetCachedPipelineConfiguration(ctx context.Context, configKey string) ([]byte, bool, error)
	GetCachedPipelineList(ctx context.Context) ([]map[string]interface{}, error)

	// Tenant caching
	CacheTenant(ctx context.Context, tenantID, name string, datasets []string, active bool, instanceID string) error
	GetCachedTenants(ctx context.Context) ([]map[string]interface{}, error)
	CleanupExpiredCache(ctx context.Context) error

	// Transformation jobs (optional - may not be implemented by all backends)
	CreateTransformationJob(ctx context.Context, job *domain.TransformationJob) error
	ClaimTransformationJob(ctx context.Context, processorID string, jobTypes []domain.TransformationJobType) (*domain.TransformationJob, error)
	UpdateTransformationJob(ctx context.Context, job *domain.TransformationJob) error
	GetTransformationJob(ctx context.Context, jobID string) (*domain.TransformationJob, error)
	ListPendingTransformationJobs(ctx context.Context, limit int) ([]*domain.TransformationJob, error)
	CleanupExpiredTransformationJobs(ctx context.Context) error

	// Lifecycle
	Close() error
}
