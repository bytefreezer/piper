package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/go-goodies/log"
)

// ControlAPIStateManager manages file locks and job status using Control Service APIs
type ControlAPIStateManager struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	instanceID string
}

// NewControlAPIStateManager creates a new Control API state manager
func NewControlAPIStateManager(cfg *config.ControlService, instanceID string) (*ControlAPIStateManager, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("control service is not enabled")
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("control service base_url is required")
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &ControlAPIStateManager{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		instanceID: instanceID,
	}, nil
}

// doRequest performs an HTTP request to the control service
func (sm *ControlAPIStateManager) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := sm.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sm.apiKey)

	resp, err := sm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// AcquireFileLock acquires a file lock
func (sm *ControlAPIStateManager) AcquireFileLock(ctx context.Context, fileKey, processorType, processorID, jobID string) error {
	return sm.AcquireFileLockWithTTL(ctx, fileKey, processorType, processorID, jobID, 10*time.Minute)
}

// AcquireFileLockWithTTL acquires a file lock with a specific TTL
func (sm *ControlAPIStateManager) AcquireFileLockWithTTL(ctx context.Context, fileKey, processorType, processorID, jobID string, ttl time.Duration) error {
	// Parse tenant_id and dataset_id from fileKey (format: tenant/dataset/filename)
	parts := strings.Split(fileKey, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid file_key format: %s (expected tenant/dataset/filename)", fileKey)
	}

	tenantID := parts[0]
	datasetID := parts[1]

	body := map[string]interface{}{
		"tenant_id":             tenantID,
		"dataset_id":            datasetID,
		"file_key":              fileKey,
		"locked_by":             processorID,
		"lock_duration_seconds": int(ttl.Seconds()),
	}

	resp, err := sm.doRequest(ctx, "POST", "/api/v1/piper/locks/files", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("file lock already held by another processor")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to acquire file lock: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// ReleaseFileLock releases a file lock
func (sm *ControlAPIStateManager) ReleaseFileLock(ctx context.Context, fileKey, processorID string) error {
	// Parse tenant_id and dataset_id from fileKey (format: tenant/dataset/filename)
	parts := strings.Split(fileKey, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid file_key format: %s (expected tenant/dataset/filename)", fileKey)
	}

	tenantID := parts[0]
	datasetID := parts[1]

	body := map[string]interface{}{
		"tenant_id":  tenantID,
		"dataset_id": datasetID,
		"file_key":   fileKey,
		"locked_by":  processorID,
	}

	resp, err := sm.doRequest(ctx, "POST", "/api/v1/piper/locks/files/release", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to release file lock: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateJobRecord creates a job record
func (sm *ControlAPIStateManager) CreateJobRecord(ctx context.Context, job *domain.JobRecord) error {
	resp, err := sm.doRequest(ctx, "POST", "/api/v1/piper/jobs", job)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create job record: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// UpdateJobStatus updates a job status
func (sm *ControlAPIStateManager) UpdateJobStatus(ctx context.Context, jobID string, status domain.JobStatus) error {
	body := map[string]interface{}{
		"status": string(status),
	}

	endpoint := fmt.Sprintf("/api/v1/piper/jobs/%s/status", jobID)
	resp, err := sm.doRequest(ctx, "PUT", endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update job status: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetJobsByStatus gets jobs by status
func (sm *ControlAPIStateManager) GetJobsByStatus(ctx context.Context, status domain.JobStatus, limit int) ([]*domain.JobRecord, error) {
	endpoint := fmt.Sprintf("/api/v1/piper/jobs?status=%s&limit=%d", status, limit)
	resp, err := sm.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get jobs by status: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var jobs []*domain.JobRecord
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return jobs, nil
}

// CleanupExpiredLocks cleans up expired locks
func (sm *ControlAPIStateManager) CleanupExpiredLocks(ctx context.Context) error {
	resp, err := sm.doRequest(ctx, "DELETE", "/api/v1/piper/locks/files/cleanup/expired", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Warnf("Failed to cleanup expired locks: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CleanupStaleLocksOnStartup cleans up stale locks from previous runs
func (sm *ControlAPIStateManager) CleanupStaleLocksOnStartup(ctx context.Context, currentInstanceID string) error {
	body := map[string]interface{}{
		"instance_id": currentInstanceID,
	}

	resp, err := sm.doRequest(ctx, "DELETE", "/api/v1/piper/locks/files/cleanup/stale", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Warnf("Failed to cleanup stale locks: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CleanupInstanceLocks removes all locks held by a specific instance
func (sm *ControlAPIStateManager) CleanupInstanceLocks(ctx context.Context, instanceID string) error {
	body := map[string]interface{}{
		"instance_id": instanceID,
	}

	resp, err := sm.doRequest(ctx, "DELETE", "/api/v1/piper/locks/files/cleanup/instance", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cleanup instance locks: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Infof("Successfully cleaned up locks for instance %s", instanceID)
	return nil
}

// CachePipelineConfiguration caches a pipeline configuration
func (sm *ControlAPIStateManager) CachePipelineConfiguration(ctx context.Context, configKey, tenantID, datasetID, version string, configuration []byte, filterCount int, instanceID string) error {
	// Parse configuration bytes into map
	var configMap map[string]interface{}
	if err := json.Unmarshal(configuration, &configMap); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	body := map[string]interface{}{
		"tenant_id":     tenantID,
		"dataset_id":    datasetID,
		"configuration": configMap,
		"ttl_hours":     24,
	}

	resp, err := sm.doRequest(ctx, "POST", "/api/v1/piper/cache/pipelines", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cache pipeline configuration: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetCachedPipelineConfiguration gets a cached pipeline configuration
func (sm *ControlAPIStateManager) GetCachedPipelineConfiguration(ctx context.Context, configKey string) ([]byte, bool, error) {
	// configKey format is typically "tenantID:datasetID"
	// Parse it to get tenant and dataset IDs
	parts := splitConfigKey(configKey)
	if len(parts) != 2 {
		return nil, false, fmt.Errorf("invalid config key format: %s", configKey)
	}

	endpoint := fmt.Sprintf("/api/v1/piper/cache/pipelines/%s/%s", parts[0], parts[1])
	resp, err := sm.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("failed to get cached pipeline configuration: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Configuration map[string]interface{} `json:"configuration"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("failed to decode response: %w", err)
	}

	// Marshal the configuration map back to JSON bytes
	configBytes, err := json.Marshal(result.Configuration)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal configuration: %w", err)
	}

	return configBytes, true, nil
}

// splitConfigKey splits a config key into tenant and dataset IDs
func splitConfigKey(configKey string) []string {
	// Simple split on colon
	parts := make([]string, 0, 2)
	idx := 0
	for i := 0; i < len(configKey); i++ {
		if configKey[i] == ':' {
			parts = append(parts, configKey[idx:i])
			idx = i + 1
		}
	}
	if idx < len(configKey) {
		parts = append(parts, configKey[idx:])
	}
	return parts
}

// CacheTenant caches tenant data
func (sm *ControlAPIStateManager) CacheTenant(ctx context.Context, tenantID, name string, datasets []string, active bool, instanceID string) error {
	body := map[string]interface{}{
		"tenant_id": tenantID,
		"tenant_data": map[string]interface{}{
			"name":        name,
			"datasets":    datasets,
			"active":      active,
			"instance_id": instanceID,
		},
		"ttl_hours": 24,
	}

	resp, err := sm.doRequest(ctx, "POST", "/api/v1/piper/cache/tenants", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cache tenant: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetCachedTenants gets all cached tenants
func (sm *ControlAPIStateManager) GetCachedTenants(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := sm.doRequest(ctx, "GET", "/api/v1/piper/cache/tenants", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get cached tenants: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var tenants []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tenants); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tenants, nil
}

// GetCachedPipelineList gets a list of all cached pipelines
func (sm *ControlAPIStateManager) GetCachedPipelineList(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := sm.doRequest(ctx, "GET", "/api/v1/piper/cache/pipelines", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get cached pipeline list: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var pipelines []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pipelines); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return pipelines, nil
}

// CleanupExpiredCache cleans up expired cache entries
func (sm *ControlAPIStateManager) CleanupExpiredCache(ctx context.Context) error {
	resp, err := sm.doRequest(ctx, "DELETE", "/api/v1/piper/cache/pipelines/cleanup/expired", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Warnf("Failed to cleanup expired cache: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CleanupExpiredJobRecords cleans up expired job records
func (sm *ControlAPIStateManager) CleanupExpiredJobRecords(ctx context.Context) error {
	resp, err := sm.doRequest(ctx, "DELETE", "/api/v1/piper/jobs/cleanup/old", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Warnf("Failed to cleanup expired job records: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateTransformationJob creates a transformation job (not implemented in control API yet)
func (sm *ControlAPIStateManager) CreateTransformationJob(ctx context.Context, job *domain.TransformationJob) error {
	body := map[string]interface{}{
		"job_id":     job.JobID,
		"tenant_id":  job.TenantID,
		"dataset_id": job.DatasetID,
		"job_type":   job.JobType,
		"status":     job.Status,
		"ttl_hours":  24,
	}

	if job.ProcessorID != "" {
		body["processor_id"] = job.ProcessorID
	}
	if job.Request != nil {
		body["request"] = job.Request
	}

	resp, err := sm.doRequest(ctx, "POST", "/api/v1/piper/transformation-jobs", body)
	if err != nil {
		return fmt.Errorf("failed to create transformation job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create transformation job: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Debugf("Created transformation job %s via control API", job.JobID)
	return nil
}

// ClaimTransformationJob claims a transformation job
func (sm *ControlAPIStateManager) ClaimTransformationJob(ctx context.Context, processorID string, jobTypes []domain.TransformationJobType) (*domain.TransformationJob, error) {
	body := map[string]interface{}{
		"processor_id": processorID,
		"job_types":    jobTypes,
	}

	resp, err := sm.doRequest(ctx, "POST", "/api/v1/piper/transformation-jobs/claim", body)
	if err != nil {
		return nil, fmt.Errorf("failed to claim transformation job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to claim transformation job: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Job *domain.TransformationJob `json:"job"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Job != nil {
		log.Infof("Claimed transformation job %s via control API", result.Job.JobID)
	}
	return result.Job, nil
}

// UpdateTransformationJob updates a transformation job
func (sm *ControlAPIStateManager) UpdateTransformationJob(ctx context.Context, job *domain.TransformationJob) error {
	body := map[string]interface{}{
		"status": job.Status,
	}

	if job.ProcessorID != "" {
		body["processor_id"] = job.ProcessorID
	}
	if job.Result != nil {
		body["result"] = job.Result
	}
	if job.ErrorMsg != "" {
		body["error_message"] = job.ErrorMsg
	}

	endpoint := fmt.Sprintf("/api/v1/piper/transformation-jobs/%s", job.JobID)
	resp, err := sm.doRequest(ctx, "PUT", endpoint, body)
	if err != nil {
		return fmt.Errorf("failed to update transformation job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update transformation job: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Debugf("Updated transformation job %s via control API", job.JobID)
	return nil
}

// GetTransformationJob gets a transformation job
func (sm *ControlAPIStateManager) GetTransformationJob(ctx context.Context, jobID string) (*domain.TransformationJob, error) {
	endpoint := fmt.Sprintf("/api/v1/piper/transformation-jobs/%s", jobID)
	resp, err := sm.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get transformation job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get transformation job: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Job *domain.TransformationJob `json:"job"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Job, nil
}

// ListPendingTransformationJobs lists pending transformation jobs
func (sm *ControlAPIStateManager) ListPendingTransformationJobs(ctx context.Context, limit int) ([]*domain.TransformationJob, error) {
	endpoint := fmt.Sprintf("/api/v1/piper/transformation-jobs/pending?limit=%d", limit)
	resp, err := sm.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending transformation jobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list pending transformation jobs: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Jobs  []*domain.TransformationJob `json:"jobs"`
		Count int                          `json:"count"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Jobs, nil
}

// CleanupExpiredTransformationJobs cleans up expired transformation jobs
func (sm *ControlAPIStateManager) CleanupExpiredTransformationJobs(ctx context.Context) error {
	resp, err := sm.doRequest(ctx, "DELETE", "/api/v1/piper/transformation-jobs/cleanup/expired", nil)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired transformation jobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cleanup expired transformation jobs: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		DeletedCount int  `json:"deleted_count"`
		Success      bool `json:"success"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.DeletedCount > 0 {
		log.Infof("Cleaned up %d expired transformation jobs via control API", result.DeletedCount)
	}
	return nil
}

// Close closes any resources (no-op for HTTP client)
func (sm *ControlAPIStateManager) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}
