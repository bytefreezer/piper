package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/alerts"
	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/metrics"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// PiperService orchestrates the data processing pipeline
type PiperService struct {
	cfg                  *config.Config
	s3Client             *storage.S3Client
	stateManager         storage.StateManager
	discoveryManager     *SimpleDiscoveryManager
	processor            *FormatProcessor
	configManager        *ConfigManager
	socClient            *alerts.SOCAlertClient
	failureMonitor       *alerts.FailureMonitor
	datasetMetricsClient *metrics.DatasetMetricsClient
	running              bool
	mutex                sync.RWMutex
	workers              chan struct{}
	jobQueue             chan *domain.ProcessingJob
	stopChan             chan struct{}
	wg                   sync.WaitGroup
}

// NewPiperService creates a new piper service with pipeline processing
func NewPiperService(cfg *config.Config, datasetMetricsClient *metrics.DatasetMetricsClient) (*PiperService, error) {
	// Create S3 client
	s3Client, err := storage.NewS3Client(&cfg.S3Source, &cfg.S3Dest)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Create state manager (use Control API)
	stateManager, err := storage.NewControlAPIStateManager(&cfg.ControlService, cfg.App.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	// Create discovery manager
	discoveryManager := NewSimpleDiscoveryManager(cfg, s3Client, stateManager)

	// Create format processor
	processor, err := NewFormatProcessor(cfg, s3Client, stateManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create format processor: %w", err)
	}

	// Create config manager with database support
	configManager := NewConfigManager(cfg, stateManager)

	// Create SOC alert client
	socConfig := alerts.AlertClientConfig{
		SOC: alerts.SOCConfig{
			Enabled:  cfg.SOC.Enabled,
			Endpoint: cfg.SOC.Endpoint,
			Timeout:  cfg.SOC.Timeout,
		},
		App: alerts.AppConfig{
			Name:    cfg.App.Name,
			Version: cfg.App.Version,
		},
		Dev: cfg.App.Dev,
	}
	socClient := alerts.NewSOCAlertClient(socConfig)

	// Create failure monitor
	failureConfig := alerts.FailureThresholdConfig{
		Enabled:          cfg.FailureThreshold.Enabled,
		FailureThreshold: cfg.FailureThreshold.FailureThreshold,
		MinimumSamples:   cfg.FailureThreshold.MinimumSamples,
		WindowSize:       cfg.FailureThreshold.WindowSize,
		CheckInterval:    cfg.FailureThreshold.CheckInterval,
		CooldownPeriod:   cfg.FailureThreshold.CooldownPeriod,
	}
	failureMonitor := alerts.NewFailureMonitor(failureConfig, socClient)

	service := &PiperService{
		cfg:                  cfg,
		s3Client:             s3Client,
		stateManager:         stateManager,
		discoveryManager:     discoveryManager,
		processor:            processor,
		configManager:        configManager,
		socClient:            socClient,
		failureMonitor:       failureMonitor,
		datasetMetricsClient: datasetMetricsClient,
		workers:              make(chan struct{}, cfg.Processing.MaxConcurrentJobs),
		jobQueue:             make(chan *domain.ProcessingJob, cfg.Processing.BufferSize),
		stopChan:             make(chan struct{}),
	}

	return service, nil
}

// Start starts the piper service
func (s *PiperService) Start(ctx context.Context) error {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return fmt.Errorf("service is already running")
	}
	s.running = true
	s.mutex.Unlock()

	log.Infof("Starting ByteFreezer Piper service with format processing")
	log.Infof("Max concurrent jobs: %d", s.cfg.Processing.MaxConcurrentJobs)

	// Start failure monitoring
	if err := s.failureMonitor.Start(); err != nil {
		log.Errorf("Failed to start failure monitor: %v", err)
	}

	// Send service start alert
	if err := s.socClient.SendServiceStartAlert(); err != nil {
		log.Warnf("Failed to send service start alert: %v", err)
	}

	// Cleanup stale locks from previous instances on startup
	log.Infof("Cleaning up stale locks from previous instances")
	if err := s.stateManager.CleanupStaleLocksOnStartup(ctx, s.cfg.App.InstanceID); err != nil {
		log.Errorf("Failed to cleanup stale locks on startup: %v", err)
	}

	// Start discovery goroutine
	s.wg.Add(1)
	go s.discoveryLoop(ctx)

	// Start worker goroutines
	for i := 0; i < s.cfg.Processing.MaxConcurrentJobs; i++ {
		s.wg.Add(1)
		go s.workerLoop(ctx, i)
	}

	// Start cleanup goroutine
	s.wg.Add(1)
	go s.cleanupLoop(ctx)

	// Start housekeeping goroutine
	if s.cfg.Housekeeping.Enabled {
		s.wg.Add(1)
		go s.housekeepingLoop(ctx)
	}

	log.Infof("ByteFreezer Piper service started successfully")
	return nil
}

// Stop stops the piper service
func (s *PiperService) Stop(ctx context.Context) error {
	s.mutex.Lock()
	if !s.running {
		s.mutex.Unlock()
		return fmt.Errorf("service is not running")
	}
	s.running = false
	s.mutex.Unlock()

	log.Infof("Stopping ByteFreezer Piper service...")

	// Stop failure monitoring
	s.failureMonitor.Stop()

	// Stop API server first

	// Signal stop to all goroutines
	close(s.stopChan)

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Infof("All workers stopped gracefully")
	case <-ctx.Done():
		log.Warnf("Shutdown timeout reached, some workers may not have stopped gracefully")
	}

	// Close resources
	if err := s.stateManager.Close(); err != nil {
		log.Errorf("Error closing state manager: %v", err)
	}

	log.Infof("ByteFreezer Piper service stopped")
	return nil
}

// discoveryLoop continuously discovers new files to process
func (s *PiperService) discoveryLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.S3Source.PollInterval)
	defer ticker.Stop()

	log.Infof("Starting discovery loop with interval: %v", s.cfg.S3Source.PollInterval)

	// Run initial discovery immediately
	log.Debugf("Running initial discovery on startup...")
	if err := s.discoverAndQueueJobs(ctx); err != nil {
		log.Errorf("Error during initial discovery: %v", err)
	}

	for {
		select {
		case <-s.stopChan:
			log.Infof("Discovery loop stopping")
			return
		case <-ctx.Done():
			log.Infof("Discovery loop stopped due to context cancellation")
			return
		case <-ticker.C:
			if err := s.discoverAndQueueJobs(ctx); err != nil {
				log.Errorf("Error during discovery: %v", err)
			}
		}
	}
}

// discoverAndQueueJobs discovers new files and queues them for processing
func (s *PiperService) discoverAndQueueJobs(ctx context.Context) error {
	log.Debugf("Starting job discovery...")
	jobs, err := s.discoveryManager.DiscoverJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover jobs: %w", err)
	}

	log.Infof("Discovery found %d jobs to process", len(jobs))

	for _, job := range jobs {
		select {
		case s.jobQueue <- job:
			log.Debugf("Queued job %s for file %s", job.JobID, job.SourceFile.Key)
		case <-s.stopChan:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			log.Warnf("Job queue is full, dropping job %s", job.JobID)
		}
	}

	if len(jobs) > 0 {
		log.Infof("Discovered and queued %d new jobs", len(jobs))
	}

	return nil
}

// workerLoop processes jobs from the queue
func (s *PiperService) workerLoop(ctx context.Context, workerID int) {
	defer s.wg.Done()

	log.Infof("Starting worker %d", workerID)

	for {
		select {
		case <-s.stopChan:
			log.Infof("Worker %d stopping", workerID)
			return
		case <-ctx.Done():
			log.Infof("Worker %d stopped due to context cancellation", workerID)
			return
		case job := <-s.jobQueue:
			s.processJob(ctx, job, workerID)
		}
	}
}

// processJob processes a single job
func (s *PiperService) processJob(ctx context.Context, job *domain.ProcessingJob, workerID int) {
	// Acquire worker slot
	s.workers <- struct{}{}
	defer func() { <-s.workers }()

	log.Infof("Worker %d processing job %s (file: %s)", workerID, job.JobID, job.SourceFile.Key)

	// Create a timeout context for this job
	jobCtx, cancel := context.WithTimeout(ctx, s.cfg.Processing.JobTimeout)
	defer cancel()

	// Update job status to processing
	if err := s.stateManager.UpdateJobStatus(jobCtx, job.JobID, domain.JobStatusProcessing); err != nil {
		log.Errorf("Failed to update job status to processing: %v", err)
		return
	}

	// Process the file using pipeline
	result, err := s.processor.ProcessFile(jobCtx, job)
	if err != nil {
		log.Errorf("Worker %d failed to process job %s: %v", workerID, job.JobID, err)

		// Record failure in failure monitor
		s.failureMonitor.RecordProcessingResult(job.TenantID, job.DatasetID, false)

		// Update job status to failed
		if updateErr := s.stateManager.UpdateJobStatus(jobCtx, job.JobID, domain.JobStatusFailed); updateErr != nil {
			log.Errorf("Failed to update job status to failed: %v", updateErr)
		}

		// Release file lock
		if releaseErr := s.stateManager.ReleaseFileLock(jobCtx, job.SourceFile.Key, job.ProcessorID); releaseErr != nil {
			log.Errorf("Failed to release file lock: %v", releaseErr)
		}

		return
	}

	// Record success in failure monitor
	s.failureMonitor.RecordProcessingResult(job.TenantID, job.DatasetID, true)

	// Record dataset metrics asynchronously (non-blocking)
	if s.datasetMetricsClient != nil {
		go func(tenantID, datasetID string, stats domain.ProcessingStats) {
			metricsCtx, metricsCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer metricsCancel()

			// Record metrics with relevant statistics
			err := s.datasetMetricsClient.RecordMetric(
				metricsCtx,
				tenantID,
				datasetID,
				stats.InputSize,                                 // inputBytes
				stats.OutputSize,                                // outputBytes
				stats.OutputRecords,                             // linesProcessed
				stats.ErrorRecords,                              // errorCount
				map[string]interface{}{
					"filtered_records": stats.FilteredRecords,
					"processing_time_ms": stats.ProcessingTime.Milliseconds(),
				},
			)
			if err != nil {
				log.Debugf("Failed to record dataset metrics for %s/%s: %v", tenantID, datasetID, err)
			}
		}(job.TenantID, job.DatasetID, result.Stats)
	}

	// Update job status to completed
	if err := s.stateManager.UpdateJobStatus(jobCtx, job.JobID, domain.JobStatusCompleted); err != nil {
		log.Errorf("Failed to update job status to completed: %v", err)
	}

	// Delete source file from S3 after successful processing
	if err := s.s3Client.DeleteSourceObject(jobCtx, job.SourceFile.Key); err != nil {
		log.Errorf("Failed to delete source file %s after successful processing: %v", job.SourceFile.Key, err)
		// Don't fail the job if source deletion fails - processing was successful
	} else {
		log.Infof("Successfully deleted source file %s after processing", job.SourceFile.Key)
	}

	// Release file lock
	if err := s.stateManager.ReleaseFileLock(jobCtx, job.SourceFile.Key, job.ProcessorID); err != nil {
		log.Errorf("Failed to release file lock: %v", err)
	}

	log.Infof("Worker %d completed job %s successfully (processed %d records)",
		workerID, job.JobID, result.Stats.OutputRecords)
}

// cleanupLoop periodically cleans up expired locks, cache, and job records
func (s *PiperService) cleanupLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	log.Infof("Starting cleanup loop")

	for {
		select {
		case <-s.stopChan:
			log.Infof("Cleanup loop stopping")
			return
		case <-ctx.Done():
			log.Infof("Cleanup loop stopped due to context cancellation")
			return
		case <-ticker.C:
			if err := s.stateManager.CleanupExpiredLocks(ctx); err != nil {
				log.Errorf("Error during lock cleanup: %v", err)
			}
			if err := s.stateManager.CleanupExpiredCache(ctx); err != nil {
				log.Errorf("Error during cache cleanup: %v", err)
			}
			if err := s.stateManager.CleanupExpiredJobRecords(ctx); err != nil {
				log.Errorf("Error during job records cleanup: %v", err)
			}
		}
	}
}

// housekeepingLoop periodically updates pipeline configurations and tenant information
func (s *PiperService) housekeepingLoop(ctx context.Context) {
	defer s.wg.Done()

	// Initial delay to allow system to start up
	initialDelay := 30 * time.Second
	time.Sleep(initialDelay)

	// Perform initial update
	s.performHousekeeping(ctx)

	// Create ticker with randomized interval (like receiver)
	baseInterval := s.cfg.Housekeeping.Interval
	log.Infof("Starting housekeeping loop with base interval: %v", baseInterval)

	failureCount := 0
	for {
		// Randomized interval between 1x and 2x base interval for load balancing
		randomMultiplier := 1.0 + (0.5 * float64(time.Now().UnixNano()%1000) / 1000.0)
		nextInterval := time.Duration(float64(baseInterval) * randomMultiplier)

		timer := time.NewTimer(nextInterval)

		select {
		case <-s.stopChan:
			timer.Stop()
			log.Infof("Housekeeping loop stopping")
			return
		case <-ctx.Done():
			timer.Stop()
			log.Infof("Housekeeping loop stopped due to context cancellation")
			return
		case <-timer.C:
			if err := s.performHousekeeping(ctx); err != nil {
				failureCount++
				log.Errorf("Housekeeping failed (attempt %d): %v", failureCount, err)
			} else {
				if failureCount > 0 {
					log.Infof("Housekeeping recovered after %d failures", failureCount)
					failureCount = 0
				}
			}
		}
	}
}

// performHousekeeping performs a single housekeeping cycle
func (s *PiperService) performHousekeeping(ctx context.Context) error {
	log.Debugf("Starting housekeeping cycle...")

	// Update pipeline configuration database
	if err := s.configManager.UpdateDatabase(ctx); err != nil {
		return fmt.Errorf("failed to update pipeline configuration database: %w", err)
	}

	cacheStats := s.configManager.GetCacheStats()
	log.Infof("Housekeeping completed successfully. Cache stats: %+v", cacheStats)

	return nil
}
