package services

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/pipeline"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// ProcessingEngine handles the actual file processing
type ProcessingEngine struct {
	config           *config.Config
	s3Client         *storage.S3Client
	stateManager     *storage.DynamoDBStateManager
	pipelineRegistry *pipeline.DefaultFilterRegistry
	
	// Job processing
	jobQueue    <-chan *domain.ProcessingJob
	workers     []*ProcessingWorker
	workerCount int
	
	// Metrics
	metrics     ProcessingMetrics
	metricsMu   sync.RWMutex
	
	// Lifecycle management
	isRunning  bool
	shutdownCh chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

// ProcessingMetrics holds processing engine metrics
type ProcessingMetrics struct {
	ActiveWorkers     int       `json:"active_workers"`
	QueueDepth        int       `json:"queue_depth"`
	JobsProcessed     int64     `json:"jobs_processed"`
	JobsSucceeded     int64     `json:"jobs_succeeded"`
	JobsFailed        int64     `json:"jobs_failed"`
	RecordsProcessed  int64     `json:"records_processed"`
	BytesProcessed    int64     `json:"bytes_processed"`
	AverageLatency    time.Duration `json:"average_latency"`
	LastJobTime       time.Time `json:"last_job_time"`
}

// ProcessingWorker represents a single processing worker
type ProcessingWorker struct {
	id           int
	engine       *ProcessingEngine
	isActive     bool
	currentJob   *domain.ProcessingJob
	mu           sync.RWMutex
}

// NewProcessingEngine creates a new processing engine
func NewProcessingEngine(cfg *config.Config, s3Client *storage.S3Client, stateManager *storage.DynamoDBStateManager) (*ProcessingEngine, error) {
	// Create pipeline registry
	pipelineRegistry := pipeline.NewFilterRegistry()

	return &ProcessingEngine{
		config:           cfg,
		s3Client:         s3Client,
		stateManager:     stateManager,
		pipelineRegistry: pipelineRegistry,
		workerCount:      cfg.Processing.MaxConcurrentJobs,
		shutdownCh:       make(chan struct{}),
	}, nil
}

// Start starts the processing engine
func (pe *ProcessingEngine) Start(ctx context.Context) error {
	pe.mu.Lock()
	if pe.isRunning {
		pe.mu.Unlock()
		return fmt.Errorf("processing engine is already running")
	}
	pe.isRunning = true
	pe.mu.Unlock()

	log.Infof("Starting processing engine with %d workers", pe.workerCount)

	// Create workers
	pe.workers = make([]*ProcessingWorker, pe.workerCount)
	for i := 0; i < pe.workerCount; i++ {
		pe.workers[i] = &ProcessingWorker{
			id:     i,
			engine: pe,
		}
	}

	// Start workers
	for i, worker := range pe.workers {
		pe.wg.Add(1)
		go func(workerID int, w *ProcessingWorker) {
			defer pe.wg.Done()
			w.run(ctx)
		}(i, worker)
	}

	return nil
}

// Stop stops the processing engine
func (pe *ProcessingEngine) Stop(ctx context.Context) error {
	pe.mu.Lock()
	if !pe.isRunning {
		pe.mu.Unlock()
		return nil
	}
	pe.isRunning = false
	pe.mu.Unlock()

	log.Infof("Stopping processing engine")

	// Signal shutdown
	close(pe.shutdownCh)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		pe.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Infof("Processing engine stopped")
	case <-ctx.Done():
		log.Warnf("Timeout stopping processing engine")
	}

	return nil
}

// SetJobQueue sets the job queue for the processing engine
func (pe *ProcessingEngine) SetJobQueue(jobQueue <-chan *domain.ProcessingJob) {
	pe.jobQueue = jobQueue
}

// GetMetrics returns current processing metrics
func (pe *ProcessingEngine) GetMetrics() ProcessingMetrics {
	pe.metricsMu.RLock()
	defer pe.metricsMu.RUnlock()

	// Count active workers
	activeWorkers := 0
	for _, worker := range pe.workers {
		if worker.isActive {
			activeWorkers++
		}
	}

	metrics := pe.metrics
	metrics.ActiveWorkers = activeWorkers
	
	if pe.jobQueue != nil {
		metrics.QueueDepth = len(pe.jobQueue)
	}

	return metrics
}

// ProcessingWorker methods

// run is the main worker loop
func (pw *ProcessingWorker) run(ctx context.Context) {
	log.Debugf("Worker %d started", pw.id)
	defer log.Debugf("Worker %d stopped", pw.id)

	for {
		select {
		case <-ctx.Done():
			return
		case <-pw.engine.shutdownCh:
			return
		case job, ok := <-pw.engine.jobQueue:
			if !ok {
				log.Debugf("Worker %d: job queue closed", pw.id)
				return
			}

			if job == nil {
				continue
			}

			// Process the job
			pw.setActive(true, job)
			err := pw.processJob(ctx, job)
			pw.setActive(false, nil)

			// Update metrics
			pw.engine.updateJobMetrics(job, err)

			if err != nil {
				log.Errorf("Worker %d: failed to process job %s: %v", pw.id, job.JobID, err)
			} else {
				log.Infof("Worker %d: successfully processed job %s", pw.id, job.JobID)
			}
		}
	}
}

// setActive sets the worker's active status
func (pw *ProcessingWorker) setActive(active bool, job *domain.ProcessingJob) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	pw.isActive = active
	pw.currentJob = job
}

// processJob processes a single job
func (pw *ProcessingWorker) processJob(ctx context.Context, job *domain.ProcessingJob) error {
	startTime := time.Now()

	log.Infof("Processing job %s: file %s (tenant: %s, dataset: %s)", 
		job.JobID, job.SourceFile.Key, job.TenantID, job.DatasetID)

	// Create job record in DynamoDB
	if err := pw.engine.stateManager.CreateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to create job record: %w", err)
	}

	// Acquire file lock
	if err := pw.engine.stateManager.AcquireFileLock(ctx, job.SourceFile.Key, "piper", job.ProcessorID, job.JobID); err != nil {
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}

	// Ensure lock is released
	defer func() {
		if err := pw.engine.stateManager.ReleaseFileLock(ctx, job.SourceFile.Key, "piper"); err != nil {
			log.Errorf("Failed to release file lock for %s: %v", job.SourceFile.Key, err)
		}
	}()

	// Update job status to processing
	if err := pw.engine.stateManager.UpdateJobStatus(ctx, job.JobID, domain.JobStatusProcessing, map[string]interface{}{
		"started_at": startTime,
	}); err != nil {
		log.Errorf("Failed to update job status to processing: %v", err)
	}

	// Process the file
	result, err := pw.processFile(ctx, job)
	
	endTime := time.Now()
	processingTime := endTime.Sub(startTime)

	if err != nil {
		// Update job status to failed
		updateErr := pw.engine.stateManager.UpdateJobStatus(ctx, job.JobID, domain.JobStatusFailed, map[string]interface{}{
			"completed_at":       endTime,
			"processing_time_ms": processingTime.Milliseconds(),
			"error_message":      err.Error(),
		})
		if updateErr != nil {
			log.Errorf("Failed to update job status to failed: %v", updateErr)
		}
		return err
	}

	// Update job status to completed
	if err := pw.engine.stateManager.UpdateJobStatus(ctx, job.JobID, domain.JobStatusCompleted, map[string]interface{}{
		"completed_at":       endTime,
		"processing_time_ms": processingTime.Milliseconds(),
		"output_file":        result.OutputFile,
		"record_count":       result.Stats.OutputRecords,
	}); err != nil {
		log.Errorf("Failed to update job status to completed: %v", err)
	}

	log.Infof("Job %s completed successfully: %d records processed in %v", 
		job.JobID, result.Stats.OutputRecords, processingTime)

	return nil
}

// processFile processes a single file through the pipeline
func (pw *ProcessingWorker) processFile(ctx context.Context, job *domain.ProcessingJob) (*domain.ProcessingResult, error) {
	// Get pipeline configuration for tenant/dataset
	pipelineConfig, err := pw.engine.stateManager.GetPipelineConfig(ctx, job.TenantID, job.DatasetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline config: %w", err)
	}

	// Download file from S3
	reader, err := pw.engine.s3Client.GetRawFile(ctx, job.SourceFile.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer reader.Close()

	// Generate output file path
	outputFile := pw.generateOutputFilePath(job.SourceFile.Key)

	// Process file content
	result, err := pw.processFileContent(ctx, reader, job, pipelineConfig, outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to process file content: %w", err)
	}

	return result, nil
}

// processFileContent processes the file content through the pipeline
func (pw *ProcessingWorker) processFileContent(ctx context.Context, reader io.Reader, job *domain.ProcessingJob, 
	pipelineConfig *domain.PipelineConfiguration, outputFile string) (*domain.ProcessingResult, error) {

	// Create gzip reader for compressed input
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create processing context
	filterCtx := &pipeline.FilterContext{
		TenantID:  job.TenantID,
		DatasetID: job.DatasetID,
		Timestamp: time.Now(),
		Variables: map[string]string{
			"processor_id": job.ProcessorID,
			"job_id":      job.JobID,
		},
	}

	// Create output buffer
	var outputBuffer strings.Builder
	gzipWriter := gzip.NewWriter(&outputBuffer)
	defer gzipWriter.Close()

	// Process line by line
	scanner := bufio.NewScanner(gzipReader)
	stats := domain.ProcessingStats{
		FilterStats: make(map[string]domain.FilterStats),
	}

	lineNumber := int64(0)
	for scanner.Scan() {
		lineNumber++
		filterCtx.LineNumber = lineNumber

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		stats.InputRecords++

		// Parse JSON
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			log.Warnf("Failed to parse JSON line %d: %v", lineNumber, err)
			stats.ErrorRecords++
			continue
		}

		// Apply pipeline filters if enabled
		if pipelineConfig.Enabled && len(pipelineConfig.Filters) > 0 {
			processed, err := pw.applyFilters(filterCtx, record, pipelineConfig.Filters)
			if err != nil {
				log.Warnf("Filter error on line %d: %v", lineNumber, err)
				stats.ErrorRecords++
				continue
			}

			if processed.Skip {
				stats.FilteredRecords++
				continue
			}

			record = processed.Record
		}

		// Add processing metadata
		record["_bytefreezer_processed_at"] = time.Now().Format(time.RFC3339)
		record["_bytefreezer_processor_id"] = job.ProcessorID
		record["_bytefreezer_job_id"] = job.JobID
		record["_bytefreezer_source_file"] = job.SourceFile.Key

		// Write processed record
		outputLine, err := json.Marshal(record)
		if err != nil {
			log.Warnf("Failed to marshal processed record: %v", err)
			stats.ErrorRecords++
			continue
		}

		if _, err := gzipWriter.Write(append(outputLine, '\n')); err != nil {
			return nil, fmt.Errorf("failed to write output: %w", err)
		}

		stats.OutputRecords++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input file: %w", err)
	}

	// Close gzip writer to flush
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Upload processed file to S3
	outputReader := strings.NewReader(outputBuffer.String())
	metadata := map[string]string{
		"source-file":     job.SourceFile.Key,
		"tenant-id":       job.TenantID,
		"dataset-id":      job.DatasetID,
		"job-id":         job.JobID,
		"record-count":   fmt.Sprintf("%d", stats.OutputRecords),
	}

	if err := pw.engine.s3Client.UploadProcessedFile(ctx, outputFile, outputReader, metadata); err != nil {
		return nil, fmt.Errorf("failed to upload processed file: %w", err)
	}

	stats.InputSize = job.SourceFile.Size
	stats.OutputSize = int64(outputBuffer.Len())

	return &domain.ProcessingResult{
		SourceFile:   job.SourceFile.Key,
		OutputFile:   outputFile,
		Stats:        stats,
		Success:      true,
		PipelineUsed: pipelineConfig.Version,
	}, nil
}

// applyFilters applies the pipeline filters to a record
func (pw *ProcessingWorker) applyFilters(ctx *pipeline.FilterContext, record map[string]interface{}, 
	filters []domain.FilterConfig) (*pipeline.FilterResult, error) {

	result := &pipeline.FilterResult{
		Record:  record,
		Skip:    false,
		Applied: false,
	}

	for _, filterConfig := range filters {
		if !filterConfig.Enabled {
			continue
		}

		// Create filter instance
		filter, err := pw.engine.pipelineRegistry.CreateFilter(filterConfig.Type, filterConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create filter %s: %w", filterConfig.Type, err)
		}

		// Apply filter
		filterResult, err := filter.Apply(ctx, result.Record)
		if err != nil {
			return nil, fmt.Errorf("filter %s failed: %w", filterConfig.Type, err)
		}

		// Update result
		result.Record = filterResult.Record
		result.Applied = true

		if filterResult.Skip {
			result.Skip = true
			break
		}
	}

	return result, nil
}

// generateOutputFilePath generates the output file path
func (pw *ProcessingWorker) generateOutputFilePath(inputPath string) string {
	return storage.GenerateProcessedFileName(inputPath)
}

// updateJobMetrics updates processing metrics
func (pe *ProcessingEngine) updateJobMetrics(job *domain.ProcessingJob, err error) {
	pe.metricsMu.Lock()
	defer pe.metricsMu.Unlock()

	pe.metrics.JobsProcessed++
	pe.metrics.LastJobTime = time.Now()

	if err != nil {
		pe.metrics.JobsFailed++
	} else {
		pe.metrics.JobsSucceeded++
	}
}