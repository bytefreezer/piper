// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package services

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytefreezer/goodies/log"
	"github.com/google/uuid"

	"github.com/bytefreezer/piper/config"
	"github.com/bytefreezer/piper/domain"
	"github.com/bytefreezer/piper/storage"
)

// SimpleDiscoveryManager discovers files for processing using PostgreSQL and control service
type SimpleDiscoveryManager struct {
	config       *config.Config
	s3Client     *storage.S3Client
	stateManager storage.StateManager
	httpClient   *http.Client

	// Lifecycle management
	isRunning  bool
	shutdownCh chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

// TenantInfo represents tenant information from control service
type TenantInfo struct {
	TenantID  string   `json:"tenant_id"`
	Datasets  []string `json:"datasets"`
	Active    bool     `json:"active"`
	CreatedAt string   `json:"created_at"`
}

// DiscoverJobs discovers files and returns processing jobs
func (sdm *SimpleDiscoveryManager) DiscoverJobs(ctx context.Context) ([]*domain.ProcessingJob, error) {
	var jobs []*domain.ProcessingJob

	// Get active tenants from control service
	tenants, err := sdm.getActiveTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active tenants: %w", err)
	}

	for _, tenant := range tenants {
		if !tenant.Active {
			continue
		}

		// Discover files for this tenant
		files, err := sdm.discoverFilesForTenant(ctx, tenant.TenantID, tenant.Datasets)
		if err != nil {
			log.Warnf("Failed to discover files for tenant %s: %v", tenant.TenantID, err)
			continue
		}

		// Create jobs for discovered files
		for _, fileKey := range files {
			// Skip if we can't determine which dataset this file belongs to
			datasetID := sdm.extractDatasetFromPath(fileKey, tenant.Datasets)
			if datasetID == "" {
				log.Warnf("Could not determine dataset for file %s", fileKey)
				continue
			}

			// Check if we can acquire a lock on this file with TTL (2x job timeout)
			processorID := sdm.config.App.InstanceID
			jobID := uuid.New().String()
			lockTTL := 2 * sdm.config.Processing.JobTimeout

			if err := sdm.stateManager.AcquireFileLockWithTTL(ctx, fileKey, "piper", processorID, jobID, lockTTL); err != nil {
				if err == domain.ErrFileLocked {
					log.Debugf("File %s is already locked", fileKey)
					continue
				}
				log.Warnf("Failed to acquire lock for file %s: %v", fileKey, err)
				continue
			}

			// Create job record
			job := &domain.ProcessingJob{
				JobID:     jobID,
				TenantID:  tenant.TenantID,
				DatasetID: datasetID,
				SourceFile: domain.S3Object{
					Key: fileKey,
				},
				Priority:    0,
				CreatedAt:   time.Now(),
				Status:      domain.JobStatusQueued,
				ProcessorID: processorID,
			}

			// Create job record in database
			jobRecord := &domain.JobRecord{
				JobID:         jobID,
				TenantID:      tenant.TenantID,
				DatasetID:     datasetID,
				ProcessorType: "piper",
				ProcessorID:   processorID,
				Status:        domain.JobStatusQueued,
				Priority:      0,
				RetryCount:    0,
				MaxRetries:    3,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
				SourceFiles:   []string{fileKey},
			}

			if err := sdm.stateManager.CreateJobRecord(ctx, jobRecord); err != nil {
				log.Warnf("Failed to create job record for %s: %v", fileKey, err)
				// Release the lock since we couldn't create the job
				if releaseErr := sdm.stateManager.ReleaseFileLock(ctx, fileKey, processorID); releaseErr != nil {
					log.Warnf("Failed to release lock for %s: %v", fileKey, releaseErr)
				}
				continue
			}

			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}

// extractDatasetFromPath attempts to extract dataset ID from file path
func (sdm *SimpleDiscoveryManager) extractDatasetFromPath(fileKey string, datasets []string) string {
	for _, dataset := range datasets {
		if strings.Contains(fileKey, dataset) {
			return dataset
		}
	}
	// If no specific dataset match, use the first dataset as default
	if len(datasets) > 0 {
		return datasets[0]
	}
	return ""
}

// NewSimpleDiscoveryManager creates a new simple discovery manager
func NewSimpleDiscoveryManager(cfg *config.Config, s3Client *storage.S3Client, stateManager storage.StateManager) *SimpleDiscoveryManager {
	return &SimpleDiscoveryManager{
		config:       cfg,
		s3Client:     s3Client,
		stateManager: stateManager,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		shutdownCh:   make(chan struct{}),
	}
}

// Start starts the discovery manager
func (sdm *SimpleDiscoveryManager) Start(ctx context.Context, jobChan chan<- *domain.JobRecord) error {
	sdm.mu.Lock()
	if sdm.isRunning {
		sdm.mu.Unlock()
		return fmt.Errorf("discovery manager is already running")
	}
	sdm.isRunning = true
	sdm.mu.Unlock()

	log.Infof("Starting simple discovery manager with poll interval: %v", sdm.config.S3Source.PollInterval)

	sdm.wg.Add(1)
	go sdm.discoveryLoop(ctx, jobChan)

	return nil
}

// Stop stops the discovery manager
func (sdm *SimpleDiscoveryManager) Stop(ctx context.Context) error {
	sdm.mu.Lock()
	if !sdm.isRunning {
		sdm.mu.Unlock()
		return nil
	}
	sdm.isRunning = false
	sdm.mu.Unlock()

	log.Infof("Stopping simple discovery manager...")

	close(sdm.shutdownCh)
	sdm.wg.Wait()

	log.Infof("Simple discovery manager stopped")
	return nil
}

// discoveryLoop runs the main discovery loop
func (sdm *SimpleDiscoveryManager) discoveryLoop(ctx context.Context, jobChan chan<- *domain.JobRecord) {
	defer sdm.wg.Done()

	ticker := time.NewTicker(sdm.config.S3Source.PollInterval)
	defer ticker.Stop()

	// Run discovery immediately on start
	sdm.runDiscovery(ctx, jobChan)

	for {
		select {
		case <-ctx.Done():
			log.Infof("Discovery loop stopped due to context cancellation")
			return
		case <-sdm.shutdownCh:
			log.Infof("Discovery loop stopped due to shutdown signal")
			return
		case <-ticker.C:
			sdm.runDiscovery(ctx, jobChan)
		}
	}
}

// runDiscovery runs a single discovery cycle
func (sdm *SimpleDiscoveryManager) runDiscovery(ctx context.Context, jobChan chan<- *domain.JobRecord) {
	log.Debugf("Running discovery cycle...")

	// Get active tenants from control service
	tenants, err := sdm.getActiveTenants(ctx)
	if err != nil {
		log.Errorf("Failed to get active tenants from control service: %v", err)
		return
	}

	if len(tenants) == 0 {
		log.Debugf("No active tenants found")
		return
	}

	log.Infof("Found %d active tenants", len(tenants))

	// Discover files for each tenant
	for _, tenant := range tenants {
		if !tenant.Active {
			continue
		}

		files, err := sdm.discoverFilesForTenant(ctx, tenant.TenantID, tenant.Datasets)
		if err != nil {
			log.Errorf("Failed to discover files for tenant %s: %v", tenant.TenantID, err)
			continue
		}

		log.Debugf("Found %d files for tenant %s", len(files), tenant.TenantID)

		// Create jobs for discovered files
		for _, file := range files {
			job, err := sdm.createJob(ctx, file, tenant.TenantID)
			if err != nil {
				log.Errorf("Failed to create job for file %s: %v", file, err)
				continue
			}

			// Try to send job to channel (non-blocking)
			select {
			case jobChan <- job:
				log.Debugf("Queued job %s for file %s", job.JobID, file)
			default:
				log.Warnf("Job channel is full, skipping job for file %s", file)
			}
		}
	}

	// Cleanup expired locks
	if err := sdm.stateManager.CleanupExpiredLocks(ctx); err != nil {
		log.Errorf("Failed to cleanup expired locks: %v", err)
	}
}

// getActiveTenants retrieves active tenants from the control service
func (sdm *SimpleDiscoveryManager) getActiveTenants(ctx context.Context) ([]TenantInfo, error) {
	// If in development mode, return fake tenant data
	if sdm.config.Dev {
		log.Debugf("Development mode enabled - returning fake tenant data")
		return sdm.getFakeTenants(), nil
	}

	// Check if Control Service is configured
	if !sdm.config.ControlService.Enabled || sdm.config.ControlService.ControlURL == "" {
		log.Warnf("Control Service not configured, no tenants will be processed")
		return []TenantInfo{}, nil
	}

	log.Infof("Fetching tenants from Control Service: %s", sdm.config.ControlService.ControlURL)

	// Determine which accounts to fetch based on deployment configuration
	var accountsURL string
	deploymentType := sdm.config.App.DeploymentType
	if deploymentType == "" {
		deploymentType = "managed" // Default to managed if not specified
	}

	if sdm.config.ControlService.AccountID != "" {
		// On-prem deployment: fetch only the specific account
		log.Infof("On-prem mode: processing only account %s (deployment_type: %s)", sdm.config.ControlService.AccountID, deploymentType)
		accountsURL = fmt.Sprintf("%s/api/v1/accounts/%s", sdm.config.ControlService.ControlURL, sdm.config.ControlService.AccountID)
	} else {
		// Managed deployment: fetch all accounts (will filter by deployment_type)
		log.Infof("Managed mode: processing accounts with deployment_type '%s'", deploymentType)
		accountsURL = fmt.Sprintf("%s/api/v1/accounts?limit=1000", sdm.config.ControlService.ControlURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", accountsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts request: %w", err)
	}

	// Add API key if configured (use Bearer token format)
	if sdm.config.ControlService.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+sdm.config.ControlService.APIKey)
	}

	resp, err := sdm.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call control service for accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control service returned status %d for accounts", resp.StatusCode)
	}

	// Parse response - handle both single account and list of accounts
	var accounts []struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Active         bool   `json:"active"`
		DeploymentType string `json:"deployment_type"`
	}

	if sdm.config.ControlService.AccountID != "" {
		// Single account response
		var singleAccount struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			Active         bool   `json:"active"`
			DeploymentType string `json:"deployment_type"`
		}
		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&singleAccount); err != nil {
			return nil, fmt.Errorf("failed to decode account response: %w", err)
		}
		accounts = append(accounts, singleAccount)
	} else {
		// Multiple accounts response
		var accountsResp struct {
			Items []struct {
				ID             string `json:"id"`
				Name           string `json:"name"`
				Active         bool   `json:"active"`
				DeploymentType string `json:"deployment_type"`
			} `json:"items"`
		}
		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&accountsResp); err != nil {
			return nil, fmt.Errorf("failed to decode accounts response: %w", err)
		}
		accounts = accountsResp.Items
	}

	log.Infof("Fetched %d accounts from Control Service", len(accounts))

	// Fetch tenants for each account
	allTenants := make([]TenantInfo, 0)
	for _, account := range accounts {
		if !account.Active {
			log.Debugf("Skipping inactive account %s", account.ID)
			continue
		}

		// Filter by deployment type in managed mode
		if sdm.config.ControlService.AccountID == "" {
			// Managed deployment: only process accounts matching configured deployment_type
			if account.DeploymentType != deploymentType {
				log.Debugf("Skipping account %s with deployment_type '%s' (service is configured for '%s')",
					account.ID, account.DeploymentType, deploymentType)
				continue
			}
		}

		log.Debugf("Processing account %s (deployment_type: %s)", account.ID, account.DeploymentType)

		tenantsURL := fmt.Sprintf("%s/api/v1/accounts/%s/tenants?limit=1000",
			sdm.config.ControlService.ControlURL, account.ID)
		req, err := http.NewRequestWithContext(ctx, "GET", tenantsURL, nil)
		if err != nil {
			log.Warnf("Failed to create tenants request for account %s: %v", account.ID, err)
			continue
		}

		if sdm.config.ControlService.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+sdm.config.ControlService.APIKey)
		}

		resp, err := sdm.httpClient.Do(req)
		if err != nil {
			log.Warnf("Failed to fetch tenants for account %s: %v", account.ID, err)
			continue
		}

		var tenantsResp struct {
			Items []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Active bool   `json:"active"`
			} `json:"items"`
		}
		if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&tenantsResp); err != nil {
			resp.Body.Close()
			log.Warnf("Failed to decode tenants for account %s: %v", account.ID, err)
			continue
		}
		resp.Body.Close()

		log.Debugf("Fetched %d tenants for account %s", len(tenantsResp.Items), account.ID)

		// Convert to TenantInfo and fetch datasets for each tenant
		for _, tenant := range tenantsResp.Items {
			if !tenant.Active {
				continue
			}

			// Fetch datasets for this tenant
			datasetsURL := fmt.Sprintf("%s/api/v1/tenants/%s/datasets?limit=1000",
				sdm.config.ControlService.ControlURL, tenant.ID)
			datasetReq, err := http.NewRequestWithContext(ctx, "GET", datasetsURL, nil)
			if err != nil {
				log.Warnf("Failed to create datasets request for tenant %s: %v", tenant.ID, err)
				continue
			}

			if sdm.config.ControlService.APIKey != "" {
				datasetReq.Header.Set("Authorization", "Bearer "+sdm.config.ControlService.APIKey)
			}

			datasetResp, err := sdm.httpClient.Do(datasetReq)
			if err != nil {
				log.Warnf("Failed to fetch datasets for tenant %s: %v", tenant.ID, err)
				continue
			}

			var datasetsResp struct {
				Items []struct {
					ID     string `json:"id"`
					Active bool   `json:"active"`
				} `json:"items"`
			}
			if err := sonic.ConfigDefault.NewDecoder(datasetResp.Body).Decode(&datasetsResp); err != nil {
				datasetResp.Body.Close()
				log.Warnf("Failed to decode datasets for tenant %s: %v", tenant.ID, err)
				continue
			}
			datasetResp.Body.Close()

			// Extract active dataset IDs
			datasets := make([]string, 0)
			for _, ds := range datasetsResp.Items {
				if ds.Active {
					datasets = append(datasets, ds.ID)
				}
			}

			log.Debugf("Fetched %d datasets for tenant %s: %v", len(datasets), tenant.ID, datasets)

			allTenants = append(allTenants, TenantInfo{
				TenantID: tenant.ID,
				Datasets: datasets,
				Active:   true,
			})
		}
	}

	log.Infof("Total tenants fetched from Control Service: %d", len(allTenants))
	return allTenants, nil
}

// discoverFilesForTenant discovers files for a specific tenant
func (sdm *SimpleDiscoveryManager) discoverFilesForTenant(ctx context.Context, tenantID string, datasets []string) ([]string, error) {
	var allFiles []string

	// Build prefix for tenant (using actual S3 path format)
	tenantPrefix := fmt.Sprintf("%s/", tenantID)

	// List objects with tenant prefix
	files, err := sdm.s3Client.ListSourceObjects(ctx, tenantPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects for tenant %s: %w", tenantID, err)
	}

	// Filter files by dataset if datasets are specified
	for _, file := range files {
		if sdm.shouldProcessFile(file, tenantID, datasets) {
			allFiles = append(allFiles, file)
		}
	}

	return allFiles, nil
}

// shouldProcessFile determines if a file should be processed
func (sdm *SimpleDiscoveryManager) shouldProcessFile(fileKey, tenantID string, datasets []string) bool {
	// Parse file metadata
	metadata, err := parseS3Key(fileKey)
	if err != nil {
		log.Debugf("Skipping file with invalid format: %s", fileKey)
		return false
	}

	// Check if file belongs to the expected tenant
	if metadata.TenantID != tenantID {
		return false
	}

	// Check if dataset is in the allowed list (if specified)
	if len(datasets) > 0 {
		found := false
		for _, dataset := range datasets {
			if metadata.DatasetID == dataset {
				found = true
				break
			}
		}
		if !found {
			log.Debugf("Skipping file for non-allowed dataset: %s", metadata.DatasetID)
			return false
		}
	}

	// Process all supported formats - they will be converted to NDJSON during processing
	// Skip already processed files (*.ndjson.gz in the piper bucket indicates processed files)
	if strings.HasSuffix(metadata.Filename, ".ndjson.gz") {
		log.Debugf("Skipping already processed NDJSON file: %s", metadata.Filename)
		return false
	}

	return true
}

// createJob creates a new job for a file
func (sdm *SimpleDiscoveryManager) createJob(ctx context.Context, fileKey, tenantID string) (*domain.JobRecord, error) {
	// Try to acquire lock first
	jobID := uuid.New().String()
	processorID := sdm.config.App.InstanceID

	err := sdm.stateManager.AcquireFileLock(ctx, fileKey, "piper", processorID, jobID)
	if err != nil {
		if err == domain.ErrFileLocked {
			log.Debugf("File %s is already locked, skipping", fileKey)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to acquire lock for file %s: %w", fileKey, err)
	}

	// Parse file metadata
	metadata, err := parseS3Key(fileKey)
	if err != nil {
		// Release lock on error
		if releaseErr := sdm.stateManager.ReleaseFileLock(ctx, fileKey, processorID); releaseErr != nil {
			log.Warnf("Failed to release lock for %s: %v", fileKey, releaseErr)
		}
		return nil, fmt.Errorf("failed to parse S3 key: %w", err)
	}

	// Create job record
	job := &domain.JobRecord{
		JobID:         jobID,
		TenantID:      metadata.TenantID,
		DatasetID:     metadata.DatasetID,
		ProcessorType: "piper",
		ProcessorID:   processorID,
		Status:        domain.JobStatusPending,
		Priority:      0,
		RetryCount:    0,
		MaxRetries:    3,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		SourceFiles:   []string{fileKey},
		OutputFiles:   []string{},
		FileSize:      0, // Will be updated during processing
	}

	// Save job to database
	if err := sdm.stateManager.CreateJobRecord(ctx, job); err != nil {
		// Release lock on error
		if releaseErr := sdm.stateManager.ReleaseFileLock(ctx, fileKey, processorID); releaseErr != nil {
			log.Warnf("Failed to release lock for %s: %v", fileKey, releaseErr)
		}
		return nil, fmt.Errorf("failed to create job record: %w", err)
	}

	log.Infof("Created job %s for file %s (tenant: %s, dataset: %s)",
		jobID, fileKey, metadata.TenantID, metadata.DatasetID)

	return job, nil
}

// getFakeTenants returns fake tenant data for development mode
func (sdm *SimpleDiscoveryManager) getFakeTenants() []TenantInfo {
	return []TenantInfo{
		{
			TenantID:  "customer-1",
			Datasets:  []string{"ebpf-data", "sflow-data"},
			Active:    true,
			CreatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339), // Created yesterday
		},
	}
}

// parseS3Key parses an S3 key to extract metadata (supports multiple formats)
func parseS3Key(s3Key string) (*domain.S3FileMetadata, error) {
	metadata := &domain.S3FileMetadata{}

	// Check for new format: customer-1--ebpf-data--timestamp--ndjson.gz
	if strings.Contains(s3Key, "--") {
		// Handle filename format: customer-1--ebpf-data--timestamp--ndjson.gz
		lastSlash := strings.LastIndex(s3Key, "/")
		filename := s3Key
		if lastSlash != -1 {
			filename = s3Key[lastSlash+1:]
		}

		parts := strings.Split(filename, "--")
		if len(parts) >= 2 {
			metadata.TenantID = parts[0]
			metadata.DatasetID = parts[1]
			metadata.Filename = filename
		}

		// Extract path components for tenant/dataset structure
		pathParts := strings.Split(s3Key, "/")
		if len(pathParts) >= 2 {
			metadata.TenantID = pathParts[0]
			metadata.DatasetID = pathParts[1]
		}
	} else {
		// Handle traditional format: tenant=customer1/dataset=logs/file.ndjson.gz or customer-1/ebpf-data/file.ndjson.gz
		parts := strings.Split(s3Key, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid S3 key format: %s", s3Key)
		}

		// Check for key=value format
		hasKeyValueFormat := false
		for _, part := range parts {
			if strings.HasPrefix(part, "tenant=") {
				metadata.TenantID = strings.TrimPrefix(part, "tenant=")
				hasKeyValueFormat = true
			} else if strings.HasPrefix(part, "dataset=") {
				metadata.DatasetID = strings.TrimPrefix(part, "dataset=")
				hasKeyValueFormat = true
			} else if strings.HasPrefix(part, "year=") {
				metadata.Year = strings.TrimPrefix(part, "year=")
			} else if strings.HasPrefix(part, "month=") {
				metadata.Month = strings.TrimPrefix(part, "month=")
			} else if strings.HasPrefix(part, "day=") {
				metadata.Day = strings.TrimPrefix(part, "day=")
			} else if strings.HasPrefix(part, "hour=") {
				metadata.Hour = strings.TrimPrefix(part, "hour=")
			}
		}

		// If no key=value format, assume simple path format: tenant/dataset/file
		if !hasKeyValueFormat && len(parts) >= 2 {
			metadata.TenantID = parts[0]
			metadata.DatasetID = parts[1]
		}

		// Extract filename from the last part
		if len(parts) > 0 {
			metadata.Filename = parts[len(parts)-1]
		}
	}

	// Validate required fields
	if metadata.TenantID == "" || metadata.DatasetID == "" {
		return nil, fmt.Errorf("missing required tenant_id or dataset_id in S3 key: %s", s3Key)
	}

	return metadata, nil
}
