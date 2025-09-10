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

// PiperService is the main service that orchestrates the data processing pipeline
type PiperService struct {
	config           *config.Config
	stateManager     *storage.DynamoDBStateManager
	s3Client         *storage.S3Client
	discoveryManager *DiscoveryManager
	processingEngine *ProcessingEngine
	
	// Service lifecycle
	startTime        time.Time
	isRunning        bool
	shutdownCh       chan struct{}
	wg               sync.WaitGroup
	mu               sync.RWMutex
}

// NewPiperService creates a new piper service instance
func NewPiperService(cfg *config.Config) (*PiperService, error) {
	// Create DynamoDB state manager
	stateManager, err := storage.NewDynamoDBStateManager(&cfg.DynamoDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	// Create S3 client
	s3Client, err := storage.NewS3Client(&cfg.S3Source, &cfg.S3Dest)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Create discovery manager
	discoveryManager := NewDiscoveryManager(cfg, s3Client, stateManager)

	// Create processing engine
	processingEngine, err := NewProcessingEngine(cfg, s3Client, stateManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create processing engine: %w", err)
	}

	return &PiperService{
		config:           cfg,
		stateManager:     stateManager,
		s3Client:         s3Client,
		discoveryManager: discoveryManager,
		processingEngine: processingEngine,
		shutdownCh:       make(chan struct{}),
	}, nil
}

// Start starts the piper service
func (ps *PiperService) Start(ctx context.Context) error {
	ps.mu.Lock()
	if ps.isRunning {
		ps.mu.Unlock()
		return fmt.Errorf("service is already running")
	}
	ps.isRunning = true
	ps.startTime = time.Now()
	ps.mu.Unlock()

	log.Infof("Starting bytefreezer-piper service")

	// Test connections
	if err := ps.testConnections(ctx); err != nil {
		return fmt.Errorf("connection tests failed: %w", err)
	}

	// Register this service instance
	if err := ps.registerServiceInstance(ctx); err != nil {
		return fmt.Errorf("failed to register service instance: %w", err)
	}

	// Start discovery manager
	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()
		if err := ps.discoveryManager.Start(ctx); err != nil {
			log.Errorf("Discovery manager error: %v", err)
		}
	}()

	// Connect discovery manager to processing engine
	ps.processingEngine.SetJobQueue(ps.discoveryManager.GetJobQueue())

	// Start processing engine
	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()
		if err := ps.processingEngine.Start(ctx); err != nil {
			log.Errorf("Processing engine error: %v", err)
		}
	}()

	// Start heartbeat goroutine
	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()
		ps.heartbeatLoop(ctx)
	}()

	log.Infof("Piper service started successfully")
	return nil
}

// Stop stops the piper service gracefully
func (ps *PiperService) Stop(ctx context.Context) error {
	ps.mu.Lock()
	if !ps.isRunning {
		ps.mu.Unlock()
		return fmt.Errorf("service is not running")
	}
	ps.isRunning = false
	ps.mu.Unlock()

	log.Infof("Stopping piper service...")

	// Signal shutdown
	close(ps.shutdownCh)

	// Stop components with timeout
	stopCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Stop discovery manager
	if err := ps.discoveryManager.Stop(stopCtx); err != nil {
		log.Errorf("Error stopping discovery manager: %v", err)
	}

	// Stop processing engine
	if err := ps.processingEngine.Stop(stopCtx); err != nil {
		log.Errorf("Error stopping processing engine: %v", err)
	}

	// Wait for all goroutines with timeout
	done := make(chan struct{})
	go func() {
		ps.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Infof("All goroutines stopped")
	case <-stopCtx.Done():
		log.Warnf("Timeout waiting for goroutines to stop")
	}

	return nil
}

// GetStatus returns the current service status
func (ps *PiperService) GetStatus() domain.ServiceStatus {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	status := domain.ServiceStatus{
		ServiceType:   "piper",
		InstanceID:    ps.config.App.InstanceID,
		Version:       ps.config.App.Version,
		StartedAt:     ps.startTime,
		LastHeartbeat: time.Now(),
	}

	if ps.isRunning {
		status.Status = "healthy"
	} else {
		status.Status = "stopped"
	}

	// Get metrics from components
	if ps.processingEngine != nil {
		metrics := ps.processingEngine.GetMetrics()
		status.JobsInProgress = metrics.ActiveWorkers
		status.QueueDepth = metrics.QueueDepth
		// Add more metrics as needed
	}

	return status
}

// GetJobStatus returns the status of a specific job
func (ps *PiperService) GetJobStatus(jobID string) (*domain.JobRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return ps.stateManager.GetJobStatus(ctx, jobID)
}

// GetMetrics returns current service metrics
func (ps *PiperService) GetMetrics() *domain.MetricsSnapshot {
	metrics := &domain.MetricsSnapshot{
		Timestamp: time.Now(),
	}

	if ps.processingEngine != nil {
		engineMetrics := ps.processingEngine.GetMetrics()
		metrics.QueueDepth = engineMetrics.QueueDepth
		metrics.ActiveWorkers = engineMetrics.ActiveWorkers
		// Add more metrics as needed
	}

	return metrics
}

// testConnections tests all external connections
func (ps *PiperService) testConnections(ctx context.Context) error {
	log.Infof("Testing external connections...")

	// Test S3 source connection
	if err := ps.s3Client.TestSourceConnection(ctx); err != nil {
		return fmt.Errorf("S3 source connection failed: %w", err)
	}

	// Test S3 destination connection
	if err := ps.s3Client.TestDestinationConnection(ctx); err != nil {
		return fmt.Errorf("S3 destination connection failed: %w", err)
	}

	log.Infof("All connection tests passed")
	return nil
}

// registerServiceInstance registers this service instance in DynamoDB
func (ps *PiperService) registerServiceInstance(ctx context.Context) error {
	instance := &domain.ServiceInstance{
		ServiceType:   "piper",
		InstanceID:    ps.config.App.InstanceID,
		Status:        "healthy",
		Version:       ps.config.App.Version,
		StartedAt:     ps.startTime,
		LastHeartbeat: time.Now(),
		Capabilities:  []string{"pipeline_processing", "file_transformation"},
		Configuration: map[string]interface{}{
			"max_concurrent_jobs": ps.config.Processing.MaxConcurrentJobs,
			"source_bucket":       ps.config.S3Source.BucketName,
			"dest_bucket":         ps.config.S3Dest.BucketName,
		},
	}

	if err := ps.stateManager.RegisterServiceInstance(ctx, instance); err != nil {
		return fmt.Errorf("failed to register service instance: %w", err)
	}

	log.Infof("Service instance registered successfully")
	return nil
}

// heartbeatLoop maintains the service heartbeat
func (ps *PiperService) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ps.shutdownCh:
			return
		case <-ticker.C:
			if err := ps.updateHeartbeat(ctx); err != nil {
				log.Errorf("Failed to update heartbeat: %v", err)
			}
		}
	}
}

// updateHeartbeat updates the service heartbeat in DynamoDB
func (ps *PiperService) updateHeartbeat(ctx context.Context) error {
	heartbeatCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return ps.stateManager.UpdateHeartbeat(heartbeatCtx, "piper", ps.config.App.InstanceID)
}