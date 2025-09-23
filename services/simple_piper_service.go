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

// SimplePiperService coordinates the simplified file copying pipeline
type SimplePiperService struct {
	config           *config.Config
	s3Client         *storage.S3Client
	stateManager     *storage.PostgreSQLStateManager
	discoveryManager *SimpleDiscoveryManager
	copyProcessor    *SimpleCopyProcessor

	// Job management
	jobChan chan *domain.JobRecord
	workers []worker

	// Lifecycle management
	isRunning  bool
	shutdownCh chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

// worker represents a file processing worker
type worker struct {
	id        int
	processor *SimpleCopyProcessor
	stopCh    chan struct{}
}

// NewSimplePiperService creates a new simplified piper service
func NewSimplePiperService(cfg *config.Config) (*SimplePiperService, error) {
	// Create S3 client
	s3Client, err := storage.NewS3Client(&cfg.S3Source, &cfg.S3Dest)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Create PostgreSQL state manager
	stateManager, err := storage.NewPostgreSQLStateManager(&cfg.PostgreSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL state manager: %w", err)
	}

	// Create discovery manager
	discoveryManager := NewSimpleDiscoveryManager(cfg, s3Client, stateManager)

	// Create copy processor
	copyProcessor := NewSimpleCopyProcessor(cfg, s3Client, stateManager)

	// Create job channel
	jobChan := make(chan *domain.JobRecord, cfg.Processing.BufferSize)

	return &SimplePiperService{
		config:           cfg,
		s3Client:         s3Client,
		stateManager:     stateManager,
		discoveryManager: discoveryManager,
		copyProcessor:    copyProcessor,
		jobChan:          jobChan,
		shutdownCh:       make(chan struct{}),
	}, nil
}

// Start starts the piper service
func (sps *SimplePiperService) Start(ctx context.Context) error {
	sps.mu.Lock()
	if sps.isRunning {
		sps.mu.Unlock()
		return fmt.Errorf("piper service is already running")
	}
	sps.isRunning = true
	sps.mu.Unlock()

	log.Infof("Starting SimplePiperService with %d workers", sps.config.Processing.MaxConcurrentJobs)

	// Test connections
	if err := sps.testConnections(ctx); err != nil {
		return fmt.Errorf("connection tests failed: %w", err)
	}

	// Start workers
	sps.startWorkers(ctx)

	// Start discovery manager
	if err := sps.discoveryManager.Start(ctx, sps.jobChan); err != nil {
		return fmt.Errorf("failed to start discovery manager: %w", err)
	}

	log.Infof("SimplePiperService started successfully")
	return nil
}

// Stop stops the piper service
func (sps *SimplePiperService) Stop(ctx context.Context) error {
	sps.mu.Lock()
	if !sps.isRunning {
		sps.mu.Unlock()
		return nil
	}
	sps.isRunning = false
	sps.mu.Unlock()

	log.Infof("Stopping SimplePiperService...")

	// Stop discovery manager
	if err := sps.discoveryManager.Stop(ctx); err != nil {
		log.Errorf("Error stopping discovery manager: %v", err)
	}

	// Stop workers
	sps.stopWorkers()

	// Close job channel
	close(sps.jobChan)

	// Wait for all goroutines to finish
	sps.wg.Wait()

	// Close state manager
	if err := sps.stateManager.Close(); err != nil {
		log.Errorf("Error closing state manager: %v", err)
	}

	log.Infof("SimplePiperService stopped")
	return nil
}

// testConnections tests all external connections
func (sps *SimplePiperService) testConnections(ctx context.Context) error {
	log.Infof("Testing connections...")

	// Test S3 connections
	if err := sps.s3Client.TestSourceConnection(ctx); err != nil {
		return fmt.Errorf("source S3 connection failed: %w", err)
	}

	if err := sps.s3Client.TestDestinationConnection(ctx); err != nil {
		return fmt.Errorf("destination S3 connection failed: %w", err)
	}

	log.Infof("All connections tested successfully")
	return nil
}

// startWorkers starts the worker goroutines
func (sps *SimplePiperService) startWorkers(ctx context.Context) {
	maxWorkers := sps.config.Processing.MaxConcurrentJobs
	sps.workers = make([]worker, maxWorkers)

	for i := 0; i < maxWorkers; i++ {
		worker := worker{
			id:        i + 1,
			processor: sps.copyProcessor,
			stopCh:    make(chan struct{}),
		}
		sps.workers[i] = worker

		sps.wg.Add(1)
		go sps.runWorker(ctx, worker)
	}

	log.Infof("Started %d workers", maxWorkers)
}

// stopWorkers stops all worker goroutines
func (sps *SimplePiperService) stopWorkers() {
	log.Infof("Stopping %d workers...", len(sps.workers))

	for i := range sps.workers {
		close(sps.workers[i].stopCh)
	}

	// Workers will stop when they detect the closed channel
}

// runWorker runs a single worker goroutine
func (sps *SimplePiperService) runWorker(ctx context.Context, w worker) {
	defer sps.wg.Done()

	log.Infof("Worker %d started", w.id)

	for {
		select {
		case <-ctx.Done():
			log.Infof("Worker %d stopped due to context cancellation", w.id)
			return
		case <-w.stopCh:
			log.Infof("Worker %d stopped due to shutdown signal", w.id)
			return
		case job, ok := <-sps.jobChan:
			if !ok {
				log.Infof("Worker %d stopped due to closed job channel", w.id)
				return
			}

			sps.processJob(ctx, w, job)
		}
	}
}

// processJob processes a single job
func (sps *SimplePiperService) processJob(ctx context.Context, w worker, job *domain.JobRecord) {
	log.Infof("Worker %d processing job %s for file %s", w.id, job.JobID, job.SourceFiles[0])

	startTime := time.Now()

	// Create timeout context for job
	jobCtx, cancel := context.WithTimeout(ctx, sps.config.Processing.JobTimeout)
	defer cancel()

	// Process the file
	err := w.processor.ProcessFile(jobCtx, job)

	processingTime := time.Since(startTime)

	if err != nil {
		log.Errorf("Worker %d failed to process job %s: %v", w.id, job.JobID, err)

		// Update job status to failed
		if updateErr := sps.stateManager.UpdateJobStatus(ctx, job.JobID, domain.JobStatusFailed); updateErr != nil {
			log.Errorf("Failed to update job status to failed: %v", updateErr)
		}

		// Release the file lock
		if len(job.SourceFiles) > 0 {
			if releaseErr := sps.stateManager.ReleaseFileLock(ctx, job.SourceFiles[0], job.ProcessorID); releaseErr != nil {
				log.Errorf("Failed to release file lock: %v", releaseErr)
			}
		}
	} else {
		log.Infof("Worker %d successfully processed job %s in %v", w.id, job.JobID, processingTime)

		// Release the file lock
		if len(job.SourceFiles) > 0 {
			if releaseErr := sps.stateManager.ReleaseFileLock(ctx, job.SourceFiles[0], job.ProcessorID); releaseErr != nil {
				log.Errorf("Failed to release file lock: %v", releaseErr)
			}
		}
	}
}

// GetStatus returns the current status of the service
func (sps *SimplePiperService) GetStatus() map[string]interface{} {
	sps.mu.RLock()
	defer sps.mu.RUnlock()

	status := map[string]interface{}{
		"service":        "bytefreezer-piper-simple",
		"version":        "1.0.0",
		"instance_id":    sps.config.App.InstanceID,
		"running":        sps.isRunning,
		"workers":        len(sps.workers),
		"max_workers":    sps.config.Processing.MaxConcurrentJobs,
		"queue_depth":    len(sps.jobChan),
		"queue_capacity": cap(sps.jobChan),
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	if sps.s3Client != nil {
		sourceBucket, sourcePrefix := sps.s3Client.GetSourceBucketInfo()
		destBucket, destPrefix := sps.s3Client.GetDestinationBucketInfo()

		status["s3_source"] = map[string]string{
			"bucket": sourceBucket,
			"prefix": sourcePrefix,
		}
		status["s3_destination"] = map[string]string{
			"bucket": destBucket,
			"prefix": destPrefix,
		}
	}

	return status
}
