package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// DiscoveryManager continuously discovers new files for processing
type DiscoveryManager struct {
	config       *config.Config
	s3Client     *storage.S3Client
	stateManager *storage.DynamoDBStateManager
	jobQueue     chan *domain.ProcessingJob
	
	// Lifecycle management
	isRunning  bool
	shutdownCh chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

// NewDiscoveryManager creates a new discovery manager
func NewDiscoveryManager(cfg *config.Config, s3Client *storage.S3Client, stateManager *storage.DynamoDBStateManager) *DiscoveryManager {
	return &DiscoveryManager{
		config:       cfg,
		s3Client:     s3Client,
		stateManager: stateManager,
		jobQueue:     make(chan *domain.ProcessingJob, cfg.Processing.BufferSize),
		shutdownCh:   make(chan struct{}),
	}
}

// Start starts the discovery manager
func (dm *DiscoveryManager) Start(ctx context.Context) error {
	dm.mu.Lock()
	if dm.isRunning {
		dm.mu.Unlock()
		return fmt.Errorf("discovery manager is already running")
	}
	dm.isRunning = true
	dm.mu.Unlock()

	log.Infof("Starting discovery manager")

	// Start file discovery loop
	dm.wg.Add(1)
	go func() {
		defer dm.wg.Done()
		dm.discoveryLoop(ctx)
	}()

	return nil
}

// Stop stops the discovery manager
func (dm *DiscoveryManager) Stop(ctx context.Context) error {
	dm.mu.Lock()
	if !dm.isRunning {
		dm.mu.Unlock()
		return nil
	}
	dm.isRunning = false
	dm.mu.Unlock()

	log.Infof("Stopping discovery manager")

	// Signal shutdown
	close(dm.shutdownCh)

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		dm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Infof("Discovery manager stopped")
	case <-ctx.Done():
		log.Warnf("Timeout stopping discovery manager")
	}

	// Close job queue
	close(dm.jobQueue)

	return nil
}

// GetJobQueue returns the job queue channel
func (dm *DiscoveryManager) GetJobQueue() <-chan *domain.ProcessingJob {
	return dm.jobQueue
}

// discoveryLoop continuously scans for new files to process
func (dm *DiscoveryManager) discoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(dm.config.S3Source.PollInterval)
	defer ticker.Stop()

	log.Infof("Starting file discovery loop (poll interval: %v)", dm.config.S3Source.PollInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-dm.shutdownCh:
			return
		case <-ticker.C:
			if err := dm.scanForNewFiles(ctx); err != nil {
				log.Errorf("Error scanning for new files: %v", err)
			}
		}
	}
}

// scanForNewFiles scans S3 for new files that need processing
func (dm *DiscoveryManager) scanForNewFiles(ctx context.Context) error {
	// List files in the source bucket
	files, err := dm.s3Client.ListRawFiles(ctx, "", 1000)
	if err != nil {
		return fmt.Errorf("failed to list raw files: %w", err)
	}

	log.Debugf("Found %d raw files to check", len(files))

	processedCount := 0
	for _, file := range files {
		// Check if file is already locked or processed
		locked, err := dm.stateManager.IsFileLocked(ctx, file.Key, "piper")
		if err != nil {
			log.Errorf("Error checking lock for file %s: %v", file.Key, err)
			continue
		}

		if locked {
			log.Debugf("File %s is already locked, skipping", file.Key)
			continue
		}

		// Parse tenant and dataset from file path
		tenantID, datasetID, err := storage.ParseS3Path(file.Key)
		if err != nil {
			log.Errorf("Error parsing S3 path %s: %v", file.Key, err)
			continue
		}

		// Check if we already processed this file by looking for output
		processedFile := dm.generateProcessedFilePath(file.Key)
		if dm.checkFileExists(ctx, processedFile) {
			log.Debugf("File %s already processed (output exists: %s)", file.Key, processedFile)
			continue
		}

		// Create processing job
		job := &domain.ProcessingJob{
			JobID:       storage.GenerateJobID("piper"),
			TenantID:    tenantID,
			DatasetID:   datasetID,
			SourceFile:  file,
			Priority:    1, // Default priority
			CreatedAt:   time.Now(),
			Status:      domain.JobStatusQueued,
			ProcessorID: dm.config.App.InstanceID,
		}

		// Try to queue the job
		select {
		case dm.jobQueue <- job:
			log.Debugf("Queued job %s for file %s (tenant: %s, dataset: %s)", 
				job.JobID, file.Key, tenantID, datasetID)
			processedCount++
		default:
			log.Warnf("Job queue is full, skipping file %s", file.Key)
		}
	}

	if processedCount > 0 {
		log.Infof("Queued %d new files for processing", processedCount)
	}

	return nil
}

// generateProcessedFilePath generates the expected processed file path
func (dm *DiscoveryManager) generateProcessedFilePath(rawFilePath string) string {
	// Convert raw path to processed path
	// raw/tenant=X/dataset=Y/... -> processed/tenant=X/dataset=Y/...
	
	processedFileName := storage.GenerateProcessedFileName(rawFilePath)
	
	// Replace 'raw/' prefix with 'processed/' and update filename
	if len(rawFilePath) > 4 && rawFilePath[:4] == "raw/" {
		// Extract directory path and replace filename
		lastSlash := -1
		for i := len(rawFilePath) - 1; i >= 0; i-- {
			if rawFilePath[i] == '/' {
				lastSlash = i
				break
			}
		}
		
		if lastSlash != -1 {
			dirPath := rawFilePath[:lastSlash+1]
			// Replace 'raw/' with 'processed/'
			processedDir := "processed/" + dirPath[4:]
			return processedDir + processedFileName
		}
	}
	
	return "processed/" + processedFileName
}

// checkFileExists checks if a file exists in the destination bucket
func (dm *DiscoveryManager) checkFileExists(ctx context.Context, filePath string) bool {
	// This is a simplified check - in a real implementation, you might
	// want to use HeadObject to check if the file exists
	// For now, we'll assume files don't exist and let the processing engine handle duplicates
	return false
}

// GetStats returns discovery manager statistics
func (dm *DiscoveryManager) GetStats() DiscoveryStats {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return DiscoveryStats{
		IsRunning:      dm.isRunning,
		QueueDepth:     len(dm.jobQueue),
		QueueCapacity:  cap(dm.jobQueue),
		PollInterval:   dm.config.S3Source.PollInterval,
	}
}

// DiscoveryStats represents discovery manager statistics
type DiscoveryStats struct {
	IsRunning      bool          `json:"is_running"`
	QueueDepth     int           `json:"queue_depth"`
	QueueCapacity  int           `json:"queue_capacity"`
	PollInterval   time.Duration `json:"poll_interval"`
	FilesDiscovered int64        `json:"files_discovered"`
	LastScanTime   time.Time     `json:"last_scan_time"`
}