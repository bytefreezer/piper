// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package services

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"time"

	"github.com/bytefreezer/goodies/log"
	"github.com/bytefreezer/piper/api"
	"github.com/bytefreezer/piper/domain"
)

// TransformationJobService handles async transformation job processing
type TransformationJobService struct {
	services     *Services
	instanceID   string
	pollInterval time.Duration
	stopChan     chan struct{}
	stopped      chan struct{}
}

// NewTransformationJobService creates a new transformation job processor
func NewTransformationJobService(services *Services, instanceID string, pollInterval time.Duration) *TransformationJobService {
	return &TransformationJobService{
		services:     services,
		instanceID:   instanceID,
		pollInterval: pollInterval,
		stopChan:     make(chan struct{}),
		stopped:      make(chan struct{}),
	}
}

// Start begins polling for and processing transformation jobs
func (s *TransformationJobService) Start(ctx context.Context) {
	log.Infof("Starting transformation job service (instance: %s, poll interval: %v)", s.instanceID, s.pollInterval)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	defer close(s.stopped)

	// Process jobs immediately on startup
	s.processJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("Transformation job service stopped (context cancelled)")
			return
		case <-s.stopChan:
			log.Info("Transformation job service stopped")
			return
		case <-ticker.C:
			s.processJobs(ctx)
		}
	}
}

// Stop gracefully stops the job processor
func (s *TransformationJobService) Stop() {
	close(s.stopChan)
	<-s.stopped
	log.Info("Transformation job service shutdown complete")
}

// processJobs claims and processes pending transformation jobs
func (s *TransformationJobService) processJobs(ctx context.Context) {
	log.Infof("TRACE: processJobs called")
	if s.services.StateManager == nil {
		log.Warnf("Transformation job service: StateManager is nil, cannot process jobs")
		return // No state manager available
	}
	log.Infof("TRACE: StateManager is not nil, proceeding to claim job")

	// Claim a pending job (supports all job types)
	jobTypes := []domain.TransformationJobType{
		domain.TransformationJobTypeTest,
		domain.TransformationJobTypeValidate,
		domain.TransformationJobTypeActivate,
	}

	job, err := s.services.StateManager.ClaimTransformationJob(ctx, s.instanceID, jobTypes)
	if err != nil {
		log.Infof("ClaimTransformationJob returned error: %v", err)
		return
	}

	if job == nil {
		return // No pending jobs
	}

	log.Infof("Claimed transformation job %s (type: %s, tenant: %s, dataset: %s)",
		job.JobID, job.JobType, job.TenantID, job.DatasetID)

	// Process the job based on type
	s.processJob(ctx, job)
}

// processJob executes a transformation job and updates its status
func (s *TransformationJobService) processJob(ctx context.Context, job *domain.TransformationJob) {
	now := time.Now()

	// Update job to processing status
	job.Status = domain.JobStatusProcessing
	job.StartedAt = &now
	job.UpdatedAt = now

	if err := s.services.StateManager.UpdateTransformationJob(ctx, job); err != nil {
		log.Errorf("Failed to update job %s to processing: %v", job.JobID, err)
		return
	}

	// Execute job based on type
	var result interface{}
	var jobErr error

	switch job.JobType {
	case domain.TransformationJobTypeTest:
		result, jobErr = s.executeTestJob(ctx, job)
	case domain.TransformationJobTypeValidate:
		result, jobErr = s.executeValidateJob(ctx, job)
	case domain.TransformationJobTypeActivate:
		result, jobErr = s.executeActivateJob(ctx, job)
	default:
		jobErr = fmt.Errorf("unknown job type: %s", job.JobType)
	}

	// Update job with result or error
	completedAt := time.Now()
	job.CompletedAt = &completedAt
	job.UpdatedAt = completedAt

	if jobErr != nil {
		job.Status = domain.JobStatusFailed
		job.ErrorMsg = jobErr.Error()
		log.Errorf("Transformation job %s failed: %v", job.JobID, jobErr)
	} else {
		job.Status = domain.JobStatusCompleted
		job.Result = result
		log.Infof("Transformation job %s completed successfully", job.JobID)
	}

	if err := s.services.StateManager.UpdateTransformationJob(ctx, job); err != nil {
		log.Errorf("Failed to update job %s completion status: %v", job.JobID, err)
	}
}

// executeTestJob processes a test transformation job
func (s *TransformationJobService) executeTestJob(ctx context.Context, job *domain.TransformationJob) (interface{}, error) {
	// Parse request
	var request struct {
		TenantID  string                     `json:"tenant_id"`
		DatasetID string                     `json:"dataset_id"`
		Filters   []api.FilterConfig         `json:"filters"`
		Samples   []api.TransformationSample `json:"samples"`
	}

	requestJSON, err := sonic.Marshal(job.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := sonic.Unmarshal(requestJSON, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Execute transformation test
	results, err := s.services.TestTransformation(ctx, request.TenantID, request.DatasetID, request.Filters, request.Samples)
	if err != nil {
		return nil, fmt.Errorf("test transformation failed: %w", err)
	}

	return map[string]interface{}{
		"results": results,
	}, nil
}

// executeValidateJob processes a validate transformation job
func (s *TransformationJobService) executeValidateJob(ctx context.Context, job *domain.TransformationJob) (interface{}, error) {
	// Parse request
	var request struct {
		TenantID  string             `json:"tenant_id"`
		DatasetID string             `json:"dataset_id"`
		Filters   []api.FilterConfig `json:"filters"`
		Count     int                `json:"count"`
	}

	requestJSON, err := sonic.Marshal(job.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := sonic.Unmarshal(requestJSON, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Execute validation
	results, sourceFile, totalLines, err := s.services.ValidateFreshData(ctx, request.TenantID, request.DatasetID, request.Filters, request.Count)
	if err != nil {
		return nil, fmt.Errorf("validate transformation failed: %w", err)
	}

	return map[string]interface{}{
		"results":     results,
		"source_file": sourceFile,
		"total_lines": totalLines,
	}, nil
}

// executeActivateJob processes an activate transformation job
func (s *TransformationJobService) executeActivateJob(ctx context.Context, job *domain.TransformationJob) (interface{}, error) {
	// Parse request
	var request struct {
		TenantID  string             `json:"tenant_id"`
		DatasetID string             `json:"dataset_id"`
		Filters   []api.FilterConfig `json:"filters"`
		Enabled   bool               `json:"enabled"`
	}

	requestJSON, err := sonic.Marshal(job.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := sonic.Unmarshal(requestJSON, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Execute activation
	version, err := s.services.ActivateTransformation(ctx, request.TenantID, request.DatasetID, request.Filters, request.Enabled)
	if err != nil {
		return nil, fmt.Errorf("activate transformation failed: %w", err)
	}

	return map[string]interface{}{
		"version":    version,
		"enabled":    request.Enabled,
		"updated_at": time.Now().Format(time.RFC3339),
	}, nil
}
