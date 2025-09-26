package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/api"
	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// PiperService orchestrates the data processing pipeline
type PiperService struct {
	cfg               *config.Config
	s3Client          *storage.S3Client
	stateManager      *storage.PostgreSQLStateManager
	discoveryManager  *SimpleDiscoveryManager
	processor         *FormatProcessor
	configManager     *ConfigManager
	apiServer         *api.API
	running           bool
	mutex             sync.RWMutex
	workers           chan struct{}
	jobQueue          chan *domain.ProcessingJob
	stopChan          chan struct{}
	wg                sync.WaitGroup
}

// NewPiperService creates a new piper service with pipeline processing
func NewPiperService(cfg *config.Config) (*PiperService, error) {
	// Create S3 client
	s3Client, err := storage.NewS3Client(&cfg.S3Source, &cfg.S3Dest)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Create PostgreSQL state manager
	stateManager, err := storage.NewPostgreSQLStateManager(&cfg.PostgreSQL)
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

	// Create API server
	apiServer := api.NewAPI(configManager, cfg)

	service := &PiperService{
		cfg:              cfg,
		s3Client:         s3Client,
		stateManager:     stateManager,
		discoveryManager: discoveryManager,
		processor:        processor,
		configManager:    configManager,
		apiServer:        apiServer,
		workers:          make(chan struct{}, cfg.Processing.MaxConcurrentJobs),
		jobQueue:         make(chan *domain.ProcessingJob, cfg.Processing.BufferSize),
		stopChan:         make(chan struct{}),
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

	// Start API server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		address := fmt.Sprintf(":%d", s.cfg.Monitoring.HealthPort)
		router := s.apiServer.NewRouter()
		s.apiServer.Serve(address, router.Router)
	}()

	log.Infof("ByteFreezer Piper service started successfully")
	log.Infof("API server running on port %d", s.cfg.Monitoring.HealthPort)
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

	// Stop API server first
	if s.apiServer != nil {
		s.apiServer.Stop()
	}

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
	jobs, err := s.discoveryManager.DiscoverJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover jobs: %w", err)
	}

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

	// Update job status to completed
	if err := s.stateManager.UpdateJobStatus(jobCtx, job.JobID, domain.JobStatusCompleted); err != nil {
		log.Errorf("Failed to update job status to completed: %v", err)
	}

	// Release file lock
	if err := s.stateManager.ReleaseFileLock(jobCtx, job.SourceFile.Key, job.ProcessorID); err != nil {
		log.Errorf("Failed to release file lock: %v", err)
	}

	log.Infof("Worker %d completed job %s successfully (processed %d records)",
		workerID, job.JobID, result.Stats.OutputRecords)
}

// cleanupLoop periodically cleans up expired locks
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
				log.Errorf("Error during cleanup: %v", err)
			}
			if err := s.stateManager.CleanupExpiredCache(ctx); err != nil {
				log.Errorf("Error during cache cleanup: %v", err)
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