package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/storage"
	"github.com/n0needt0/bytefreezer-piper/utils"
	"github.com/n0needt0/go-goodies/log"
)

// SpoolingService manages the local staging and processing of files
// Following the receiver pattern for consistency
type SpoolingService struct {
	config         *config.Config
	s3Source       *s3.S3
	s3Dest         *s3.S3
	spoolDirectory string
	stateManager   *storage.PostgreSQLStateManager
	processorID    string // Unique identifier for this piper node
	wg             sync.WaitGroup
	shutdown       chan struct{}
	running        bool
	mu             sync.RWMutex
}

// NewSpoolingService creates a new spooling service instance
func NewSpoolingService(cfg *config.Config, stateManager *storage.PostgreSQLStateManager) (*SpoolingService, error) {
	// Create S3 sessions
	s3Source, err := createS3Session(cfg.S3Source.AccessKey, cfg.S3Source.SecretKey, cfg.S3Source.Region, cfg.S3Source.Endpoint, cfg.S3Source.SSL)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 source session: %w", err)
	}

	s3Dest, err := createS3Session(cfg.S3Dest.AccessKey, cfg.S3Dest.SecretKey, cfg.S3Dest.Region, cfg.S3Dest.Endpoint, cfg.S3Dest.SSL)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 destination session: %w", err)
	}

	// Validate and create spool directory
	spoolPath := cfg.Spooling.SpoolPath
	if spoolPath == "" {
		spoolPath = "/var/spool/bytefreezer-piper"
	}

	if err := os.MkdirAll(spoolPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create spool directory %s: %w", spoolPath, err)
	}

	// Create malformed directory
	malformedPath := utils.BuildMalformedPath(spoolPath)
	if err := os.MkdirAll(malformedPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create malformed directory %s: %w", malformedPath, err)
	}

	// Generate a unique processor ID for this node instance
	processorID := fmt.Sprintf("piper-node-%s", uuid.New().String()[:8])

	return &SpoolingService{
		config:         cfg,
		s3Source:       s3Source,
		s3Dest:         s3Dest,
		spoolDirectory: spoolPath,
		stateManager:   stateManager,
		processorID:    processorID,
		shutdown:       make(chan struct{}),
	}, nil
}

// Start starts the spooling service workers
func (s *SpoolingService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("spooling service is already running")
	}

	log.Info("Starting ByteFreezer Piper spooling service")
	s.running = true

	// Start discovery worker (checks S3 source for new files)
	s.wg.Add(1)
	go s.discoveryWorker(ctx)

	// Start queue worker (processes files from S3 to local queue)
	s.wg.Add(1)
	go s.queueWorker(ctx)

	// Start retry workers (processes files from retry directory)
	retryWorkerCount := s.config.Spooling.UploadWorkerCount
	if retryWorkerCount <= 0 {
		retryWorkerCount = 5
	}

	for i := 0; i < retryWorkerCount; i++ {
		s.wg.Add(1)
		go s.retryWorker(ctx, i)
	}

	// Start DLQ cleanup worker if enabled
	if s.config.DLQ.Enabled {
		s.wg.Add(1)
		go s.dlqCleanupWorker(ctx)
	}

	log.Info("ByteFreezer Piper spooling service started successfully")
	return nil
}

// Stop stops the spooling service gracefully
func (s *SpoolingService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	log.Info("Stopping ByteFreezer Piper spooling service")
	close(s.shutdown)
	s.wg.Wait()
	s.running = false
	log.Info("ByteFreezer Piper spooling service stopped")

	return nil
}

// discoveryWorker discovers new files in the S3 source bucket
func (s *SpoolingService) discoveryWorker(ctx context.Context) {
	defer s.wg.Done()

	log.Info("Starting S3 discovery worker")
	ticker := time.NewTicker(s.config.S3Source.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.discoverS3Files(ctx)
		}
	}
}

// discoverS3Files discovers and queues new files from S3
func (s *SpoolingService) discoverS3Files(ctx context.Context) {
	log.Debug("Discovering files in S3 source bucket")

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.config.S3Source.BucketName),
		MaxKeys: aws.Int64(100), // Process in batches
	}

	err := s.s3Source.ListObjectsV2PagesWithContext(ctx, input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			if obj.Key == nil || obj.Size == nil {
				continue
			}

			key := *obj.Key
			size := *obj.Size

			// Skip directories and empty files
			if strings.HasSuffix(key, "/") || size == 0 {
				continue
			}

			// Parse tenant/dataset from key
			tenant, dataset, filename, err := utils.ParseTenantDatasetFromKey(key)
			if err != nil {
				log.Warnf("Invalid S3 key format: %s - %v", key, err)
				continue
			}

			// Check if file is malformed
			if utils.IsMalformedFilename(filename) {
				log.Warnf("Malformed filename detected: %s - quarantining", filename)
				s.quarantineMalformedFile(key, tenant, dataset, filename)
				continue
			}

			// Check if file already exists in local processing
			if s.fileExistsInSpool(tenant, dataset, filename) {
				log.Debugf("File already in spool: %s", key)
				continue
			}

			// Queue file for processing
			if err := s.queueFileForProcessing(ctx, key, tenant, dataset, filename, size); err != nil {
				log.Errorf("Failed to queue file %s: %v", key, err)
			}
		}

		return !lastPage
	})

	if err != nil {
		log.Errorf("Failed to list S3 objects: %v", err)
	}
}

// queueFileForProcessing downloads file from S3 and places it in queue directory
// Uses distributed locking to prevent multiple nodes from processing the same file
func (s *SpoolingService) queueFileForProcessing(ctx context.Context, key, tenant, dataset, filename string, size int64) error {
	log.Infof("Queueing file for processing: %s", key)

	// Calculate lock TTL as 2x job timeout
	jobTimeout := s.config.Processing.JobTimeout
	lockTTL := 2 * jobTimeout

	// Generate unique job ID for this file processing task
	jobID := uuid.New().String()

	// Attempt to acquire distributed lock for this file (if state manager available)
	if s.stateManager != nil {
		log.Debugf("Attempting to acquire lock for file: %s (TTL: %s, Node: %s)", key, lockTTL, s.processorID)
		err := s.stateManager.AcquireFileLockWithTTL(ctx, key, "piper", s.processorID, jobID, lockTTL)
		if err != nil {
			if errors.Is(err, domain.ErrFileLocked) {
				log.Debugf("File already locked by another node: %s", key)
				return nil // Skip this file, another node is processing it
			}
			return fmt.Errorf("failed to acquire file lock: %w", err)
		}
	} else {
		log.Warnf("State manager not available - proceeding without distributed locking for file: %s", key)
	}

	// Ensure lock is released when done (best effort cleanup)
	defer func() {
		if s.stateManager != nil {
			if releaseErr := s.stateManager.ReleaseFileLock(ctx, key, s.processorID); releaseErr != nil {
				log.Warnf("Failed to release file lock for %s: %v", key, releaseErr)
			}
		}
	}()

	log.Infof("Successfully acquired lock for file: %s", key)

	// Create queue directory
	queueDir := utils.BuildSpoolPath(s.spoolDirectory, tenant, dataset, domain.StageQueue.GetStageDirectory())
	if err := os.MkdirAll(queueDir, 0750); err != nil {
		return fmt.Errorf("failed to create queue directory: %w", err)
	}

	// Download file from S3
	queueFilePath := filepath.Join(queueDir, filename)
	if err := s.downloadS3File(ctx, key, queueFilePath); err != nil {
		return fmt.Errorf("failed to download file from S3: %w", err)
	}

	log.Infof("File queued for processing: %s -> %s", key, queueFilePath)
	return nil
}

// validatePath ensures the path is safe and within expected boundaries
func (s *SpoolingService) validatePath(filePath, baseDir string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(filePath)

	// Ensure the path is absolute
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure base directory is absolute
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute base directory: %w", err)
	}

	// Check if the file path is within the base directory
	relPath, err := filepath.Rel(absBaseDir, absPath)
	if err != nil {
		return fmt.Errorf("invalid path relationship: %w", err)
	}

	// Ensure the relative path doesn't start with .. (escaping base directory)
	if strings.HasPrefix(relPath, "..") || strings.Contains(relPath, string(filepath.Separator)+"..") {
		return fmt.Errorf("path traversal attempt detected: %s", filePath)
	}

	return nil
}

// downloadS3File downloads a file from S3 to local path
func (s *SpoolingService) downloadS3File(ctx context.Context, key, localPath string) error {
	// Get object from S3
	result, err := s.s3Source.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3Source.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get S3 object: %w", err)
	}
	defer result.Body.Close()

	// Validate the local path for security
	if err := s.validatePath(localPath, s.spoolDirectory); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	// Create local file
	// #nosec G304 - Path is validated by validatePath function above
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, result.Body)
	if err != nil {
		// Clean up on failure
		os.Remove(localPath)
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	return nil
}

// fileExistsInSpool checks if a file already exists in any stage of the spool
func (s *SpoolingService) fileExistsInSpool(tenant, dataset, filename string) bool {
	stages := []domain.Stage{domain.StageQueue, domain.StageRetry, domain.StageDLQ}

	for _, stage := range stages {
		stagePath := utils.BuildSpoolPath(s.spoolDirectory, tenant, dataset, stage.GetStageDirectory())
		filePath := filepath.Join(stagePath, filename)

		if _, err := os.Stat(filePath); err == nil {
			return true
		}
	}

	return false
}

// quarantineMalformedFile moves malformed files to quarantine
func (s *SpoolingService) quarantineMalformedFile(key, tenant, dataset, filename string) {
	log.Warnf("Quarantining malformed file: %s", key)

	malformedPath := utils.BuildMalformedPath(s.spoolDirectory)
	quarantineFile := filepath.Join(malformedPath, fmt.Sprintf("%s--%s--%s", tenant, dataset, filename))

	// Create a record of the malformed file
	record := map[string]interface{}{
		"s3_key":     key,
		"tenant":     tenant,
		"dataset":    dataset,
		"filename":   filename,
		"quarantined_at": time.Now().UTC(),
		"reason":     "malformed_filename",
	}

	data, _ := json.Marshal(record)
	os.WriteFile(quarantineFile+".json", data, 0600)
}

// queueWorker processes files from queue directory to retry directory
func (s *SpoolingService) queueWorker(ctx context.Context) {
	defer s.wg.Done()

	log.Info("Starting queue worker")
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.processQueueFiles(ctx)
		}
	}
}

// processQueueFiles processes files from queue directories
func (s *SpoolingService) processQueueFiles(ctx context.Context) {
	log.Debug("Processing queue files")

	err := filepath.Walk(s.spoolDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-queue files
		if info.IsDir() || !strings.Contains(path, "/queue/") {
			return nil
		}

		// Skip recent files (might still be downloading)
		if time.Since(info.ModTime()) < 5*time.Second {
			return nil
		}

		// Extract tenant/dataset from path
		relPath, err := filepath.Rel(s.spoolDirectory, path)
		if err != nil {
			return err
		}

		pathParts := strings.Split(relPath, string(filepath.Separator))
		if len(pathParts) < 3 {
			return nil
		}

		tenant := pathParts[0]
		dataset := pathParts[1]
		filename := info.Name()

		// Move file from queue to retry
		if err := s.moveQueueToRetry(tenant, dataset, filename); err != nil {
			log.Errorf("Failed to move queue file to retry: %v", err)
		}

		return nil
	})

	if err != nil {
		log.Errorf("Error walking queue directories: %v", err)
	}
}

// moveQueueToRetry moves a file from queue to retry stage with metadata
func (s *SpoolingService) moveQueueToRetry(tenant, dataset, filename string) error {
	queuePath := utils.BuildSpoolPath(s.spoolDirectory, tenant, dataset, domain.StageQueue.GetStageDirectory())
	retryPath := utils.BuildSpoolPath(s.spoolDirectory, tenant, dataset, domain.StageRetry.GetStageDirectory())

	queueFilePath := filepath.Join(queuePath, filename)
	retryFilePath := filepath.Join(retryPath, filename)
	retryMetaPath := retryFilePath + ".meta"

	// Create retry directory
	if err := os.MkdirAll(retryPath, 0750); err != nil {
		return fmt.Errorf("failed to create retry directory: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(queueFilePath)
	if err != nil {
		return fmt.Errorf("failed to stat queue file: %w", err)
	}

	// Create metadata
	retryFile := domain.RetryFile{
		TenantID:       tenant,
		DatasetID:      dataset,
		Filename:       filename,
		FilePath:       retryFilePath,
		CompressedSize: fileInfo.Size(),
		CreatedAt:      time.Now().UTC(),
		Status:         "retry",
		RetryCount:     0,
		LastRetry:      time.Time{},
	}

	// Move file atomically
	if err := os.Rename(queueFilePath, retryFilePath); err != nil {
		return fmt.Errorf("failed to move file to retry: %w", err)
	}

	// Write metadata
	metaData, err := json.Marshal(retryFile)
	if err != nil {
		// Rollback file move
		os.Rename(retryFilePath, queueFilePath)
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(retryMetaPath, metaData, 0600); err != nil {
		// Rollback file move
		os.Rename(retryFilePath, queueFilePath)
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	log.Infof("Moved file to retry stage: %s/%s/%s", tenant, dataset, filename)
	return nil
}

// retryWorker processes files from retry directory
func (s *SpoolingService) retryWorker(ctx context.Context, workerID int) {
	defer s.wg.Done()

	log.Infof("Starting retry worker %d", workerID)

	// Ensure retry interval is positive (fallback to 30 seconds if misconfigured)
	retryInterval := s.config.Spooling.RetryInterval
	if retryInterval <= 0 {
		log.Warnf("Invalid retry interval %v, using default 30s", retryInterval)
		retryInterval = 30 * time.Second
	}

	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.processRetryFiles(ctx, workerID)
		}
	}
}

// processRetryFiles processes files from retry directories
func (s *SpoolingService) processRetryFiles(ctx context.Context, workerID int) {
	log.Debugf("Worker %d processing retry files", workerID)

	jobChannel := make(chan domain.ProcessJob, s.config.Processing.BufferSize)
	resultChannel := make(chan domain.ProcessResult, s.config.Processing.BufferSize)

	// Start process workers
	processWorkerCount := s.config.Spooling.ProcessWorkerCount
	if processWorkerCount <= 0 {
		processWorkerCount = 3
	}

	var wg sync.WaitGroup
	for i := 0; i < processWorkerCount; i++ {
		wg.Add(1)
		go s.processWorker(ctx, i, jobChannel, resultChannel, &wg)
	}

	// Start result processor
	wg.Add(1)
	go s.resultProcessor(ctx, resultChannel, &wg)

	// Find and queue retry files
	s.queueRetryJobs(jobChannel)

	// Close job channel and wait for completion
	close(jobChannel)
	wg.Wait()
	close(resultChannel)
}

// queueRetryJobs finds retry files and creates processing jobs
func (s *SpoolingService) queueRetryJobs(jobChannel chan<- domain.ProcessJob) {
	err := filepath.Walk(s.spoolDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for .meta files in retry directories
		if !info.IsDir() && strings.HasSuffix(path, ".meta") && strings.Contains(path, "/retry/") {
			// Extract tenant/dataset from path
			relPath, err := filepath.Rel(s.spoolDirectory, path)
			if err != nil {
				return err
			}

			pathParts := strings.Split(relPath, string(filepath.Separator))
			if len(pathParts) < 3 {
				return nil
			}

			tenant := pathParts[0]
			dataset := pathParts[1]

			// Create processing job
			dataFilePath := strings.TrimSuffix(path, ".meta")
			job := domain.ProcessJob{
				TenantID:      tenant,
				DatasetID:     dataset,
				LocalFilePath: dataFilePath,
				MetaPath:      path,
				Attempts:      0,
			}

			select {
			case jobChannel <- job:
			default:
				log.Warnf("Job channel full, skipping file: %s", dataFilePath)
			}
		}

		return nil
	})

	if err != nil {
		log.Errorf("Error walking retry directories: %v", err)
	}
}

// processWorker processes individual files
func (s *SpoolingService) processWorker(ctx context.Context, workerID int, jobChannel <-chan domain.ProcessJob, resultChannel chan<- domain.ProcessResult, wg *sync.WaitGroup) {
	defer wg.Done()

	log.Debugf("Starting process worker %d", workerID)

	for job := range jobChannel {
		result := s.processFile(ctx, job)

		select {
		case resultChannel <- result:
		case <-ctx.Done():
			return
		}
	}
}

// processFile processes a single file (pipeline processing disabled for now)
func (s *SpoolingService) processFile(ctx context.Context, job domain.ProcessJob) domain.ProcessResult {
	log.Infof("Processing file: %s", job.LocalFilePath)

	// For now, just copy the file as-is (pipeline processing disabled)
	// In the future, this is where pipeline processing would occur

	// Generate destination key
	filename := filepath.Base(job.LocalFilePath)
	destKey := fmt.Sprintf("%s/%s/%s", job.TenantID, job.DatasetID, filename)

	// Upload to destination bucket
	err := s.uploadToDestination(ctx, job.LocalFilePath, destKey)
	if err != nil {
		return domain.ProcessResult{
			Job:     job,
			Success: false,
			Error:   err.Error(),
		}
	}

	// Get file info
	fileInfo, err := os.Stat(job.LocalFilePath)
	if err != nil {
		return domain.ProcessResult{
			Job:     job,
			Success: false,
			Error:   fmt.Sprintf("failed to get file info: %v", err),
		}
	}

	return domain.ProcessResult{
		Job:            job,
		Success:        true,
		DestinationKey: destKey,
		ProcessedSize:  fileInfo.Size(),
	}
}

// uploadToDestination uploads processed file to destination S3 bucket
func (s *SpoolingService) uploadToDestination(ctx context.Context, localPath, destKey string) error {
	// Validate the local path for security
	if err := s.validatePath(localPath, s.spoolDirectory); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	// #nosec G304 - Path is validated by validatePath function above
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	_, err = s.s3Dest.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.config.S3Dest.BucketName),
		Key:    aws.String(destKey),
		Body:   file,
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	log.Infof("Uploaded file to destination: %s", destKey)
	return nil
}

// resultProcessor handles processing results
func (s *SpoolingService) resultProcessor(ctx context.Context, resultChannel <-chan domain.ProcessResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for result := range resultChannel {
		if result.Success {
			s.handleSuccessfulProcessing(result)
		} else {
			s.handleFailedProcessing(result)
		}
	}
}

// handleSuccessfulProcessing handles successful file processing
func (s *SpoolingService) handleSuccessfulProcessing(result domain.ProcessResult) {
	log.Infof("Successfully processed file: %s", result.Job.LocalFilePath)

	// Build the S3 key to match what was used for locking
	sourceKey := fmt.Sprintf("%s/%s/%s", result.Job.TenantID, result.Job.DatasetID, filepath.Base(result.Job.LocalFilePath))

	// Release the distributed lock for this file (if it exists)
	if s.stateManager != nil {
		if err := s.stateManager.ReleaseFileLock(context.Background(), sourceKey, s.processorID); err != nil {
			log.Warnf("Failed to release file lock after successful processing for %s: %v", sourceKey, err)
		}
	}

	// Delete source file from S3
	s.deleteSourceFile(sourceKey)

	// Clean up local files
	os.Remove(result.Job.LocalFilePath)
	os.Remove(result.Job.MetaPath)
}

// handleFailedProcessing handles failed file processing
func (s *SpoolingService) handleFailedProcessing(result domain.ProcessResult) {
	log.Errorf("Failed to process file %s: %s", result.Job.LocalFilePath, result.Error)

	// Read metadata
	var retryFile domain.RetryFile
	metaData, err := os.ReadFile(result.Job.MetaPath)
	if err != nil {
		log.Errorf("Failed to read metadata: %v", err)
		return
	}

	if err := json.Unmarshal(metaData, &retryFile); err != nil {
		log.Errorf("Failed to unmarshal metadata: %v", err)
		return
	}

	// Increment retry count
	retryFile.RetryCount++
	retryFile.LastRetry = time.Now().UTC()
	retryFile.FailureReason = result.Error

	// Check if max retries exceeded
	if retryFile.RetryCount >= s.config.DLQ.RetryAttempts {
		s.moveToDLQ(result.Job, retryFile, "max_retries_exceeded")
	} else {
		// Update metadata for next retry
		updatedMeta, _ := json.Marshal(retryFile)
		os.WriteFile(result.Job.MetaPath, updatedMeta, 0600)
		log.Infof("File queued for retry %d/%d: %s", retryFile.RetryCount, s.config.DLQ.RetryAttempts, result.Job.LocalFilePath)
	}
}

// moveToDLQ moves a file to dead letter queue
func (s *SpoolingService) moveToDLQ(job domain.ProcessJob, retryFile domain.RetryFile, reason string) {
	log.Warnf("Moving file to DLQ: %s (reason: %s)", job.LocalFilePath, reason)

	dlqPath := utils.BuildSpoolPath(s.spoolDirectory, job.TenantID, job.DatasetID, domain.StageDLQ.GetStageDirectory())
	if err := os.MkdirAll(dlqPath, 0750); err != nil {
		log.Errorf("Failed to create DLQ directory: %v", err)
		return
	}

	filename := filepath.Base(job.LocalFilePath)
	dlqFilePath := filepath.Join(dlqPath, filename)
	dlqMetaPath := dlqFilePath + ".meta"

	// Update metadata
	retryFile.Status = "dlq"
	retryFile.FailureReason = reason
	retryFile.FilePath = dlqFilePath

	// Move file to DLQ
	if err := os.Rename(job.LocalFilePath, dlqFilePath); err != nil {
		log.Errorf("Failed to move file to DLQ: %v", err)
		return
	}

	// Update metadata
	metaData, _ := json.Marshal(retryFile)
	if err := os.WriteFile(dlqMetaPath, metaData, 0600); err != nil {
		log.Errorf("Failed to write DLQ metadata: %v", err)
	}

	// Remove old metadata
	os.Remove(job.MetaPath)

	log.Infof("File moved to DLQ: %s", dlqFilePath)
}

// deleteSourceFile deletes the source file from S3
func (s *SpoolingService) deleteSourceFile(sourceKey string) {
	log.Infof("Deleting source file from S3: %s", sourceKey)

	_, err := s.s3Source.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s.config.S3Source.BucketName),
		Key:    aws.String(sourceKey),
	})

	if err != nil {
		log.Errorf("Failed to delete source file from S3: %v", err)
	} else {
		log.Infof("Source file deleted from S3: %s", sourceKey)
	}
}

// dlqCleanupWorker cleans up old files from DLQ
func (s *SpoolingService) dlqCleanupWorker(ctx context.Context) {
	defer s.wg.Done()

	log.Info("Starting DLQ cleanup worker")
	ticker := time.NewTicker(s.config.DLQ.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.cleanupDLQ()
		}
	}
}

// cleanupDLQ removes old files from DLQ based on age
func (s *SpoolingService) cleanupDLQ() {
	log.Debug("Cleaning up DLQ files")

	maxAge := time.Duration(s.config.DLQ.MaxAgeDays) * 24 * time.Hour
	cutoffTime := time.Now().Add(-maxAge)

	err := filepath.Walk(s.spoolDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for files in DLQ directories
		if !info.IsDir() && strings.Contains(path, "/dlq/") && info.ModTime().Before(cutoffTime) {
			log.Infof("Removing old DLQ file: %s", path)
			os.Remove(path)
		}

		return nil
	})

	if err != nil {
		log.Errorf("Error during DLQ cleanup: %v", err)
	}
}

// createS3Session creates an S3 session with the given credentials
func createS3Session(accessKey, secretKey, region, endpoint string, ssl bool) (*s3.S3, error) {
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")

	sess, err := session.NewSession(&aws.Config{
		Credentials:      creds,
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		DisableSSL:       aws.Bool(!ssl),
		S3ForcePathStyle: aws.Bool(true),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return s3.New(sess), nil
}