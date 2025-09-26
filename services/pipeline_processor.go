package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/n0needt0/go-goodies/log"

	"github.com/n0needt0/bytefreezer-piper/config"
	"github.com/n0needt0/bytefreezer-piper/domain"
	"github.com/n0needt0/bytefreezer-piper/parsers"
	"github.com/n0needt0/bytefreezer-piper/pipeline"
	"github.com/n0needt0/bytefreezer-piper/storage"
)

// PipelineProcessor processes files using configurable pipelines
type PipelineProcessor struct {
	cfg               *config.Config
	s3Client          *storage.S3Client
	stateManager      *storage.PostgreSQLStateManager
	pipelineProcessor pipeline.PipelineProcessor
	filterRegistry    pipeline.FilterRegistry
	parserRegistry    parsers.ParserRegistry
}

// NewPipelineProcessor creates a new pipeline processor
func NewPipelineProcessor(cfg *config.Config, s3Client *storage.S3Client, stateManager *storage.PostgreSQLStateManager) (*PipelineProcessor, error) {
	// Create parser registry
	parserRegistry := parsers.NewRegistry()

	// Create filter registry
	filterRegistry := pipeline.NewFilterRegistry()

	// Register parse filter to avoid import cycle
	filterRegistry.Register("parse", func(config map[string]interface{}) (pipeline.Filter, error) {
		return parsers.NewParseFilter(config, parserRegistry)
	})

	// Create pipeline processor
	pipelineProcessor := pipeline.NewBasicPipelineProcessor(filterRegistry)

	processor := &PipelineProcessor{
		cfg:               cfg,
		s3Client:          s3Client,
		stateManager:      stateManager,
		pipelineProcessor: pipelineProcessor,
		filterRegistry:    filterRegistry,
		parserRegistry:    parserRegistry,
	}

	return processor, nil
}

// ProcessFile processes a single file using the configured pipeline
func (p *PipelineProcessor) ProcessFile(ctx context.Context, job *domain.ProcessingJob) (*domain.ProcessingResult, error) {
	startTime := time.Now()

	log.Infof("Processing file %s for tenant %s, dataset %s", job.SourceFile.Key, job.TenantID, job.DatasetID)

	// Get or create pipeline for this tenant/dataset
	pipelineConfig, err := p.getPipelineConfig(ctx, job.TenantID, job.DatasetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline config: %w", err)
	}

	// Register pipeline if not already registered
	if err := p.pipelineProcessor.RegisterPipeline(job.TenantID, job.DatasetID, pipelineConfig); err != nil {
		log.Warnf("Failed to register pipeline for %s:%s: %v", job.TenantID, job.DatasetID, err)
		// Pipeline might already be registered, continue
	}

	// Download source file
	sourceData, err := p.s3Client.GetSourceObject(ctx, job.SourceFile.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to download source file: %w", err)
	}

	// Process the data
	processedData, stats, err := p.processData(ctx, job.TenantID, job.DatasetID, sourceData)
	if err != nil {
		return nil, fmt.Errorf("failed to process data: %w", err)
	}

	// Generate output file key
	outputKey := p.generateOutputKey(job.SourceFile.Key)

	// Upload processed data
	if err := p.s3Client.PutDestinationObject(ctx, outputKey, processedData); err != nil {
		return nil, fmt.Errorf("failed to upload processed file: %w", err)
	}

	// Delete source file after successful processing
	if err := p.s3Client.DeleteSourceObject(ctx, job.SourceFile.Key); err != nil {
		log.Warnf("Failed to delete source file %s: %v", job.SourceFile.Key, err)
		// Don't fail the entire job for cleanup issues
	}

	result := &domain.ProcessingResult{
		SourceFile:   job.SourceFile.Key,
		OutputFile:   outputKey,
		Stats:        *stats,
		Success:      true,
		PipelineUsed: fmt.Sprintf("%s:%s", job.TenantID, job.DatasetID),
	}

	processingTime := time.Since(startTime)
	log.Infof("Successfully processed file %s in %v, processed %d records",
		job.SourceFile.Key, processingTime, stats.OutputRecords)

	return result, nil
}

// processData processes the raw data through the pipeline
func (p *PipelineProcessor) processData(ctx context.Context, tenantID, datasetID string, data []byte) ([]byte, *domain.ProcessingStats, error) {
	stats := &domain.ProcessingStats{
		InputRecords:    0,
		OutputRecords:   0,
		FilteredRecords: 0,
		ErrorRecords:    0,
		InputSize:       int64(len(data)),
		FilterStats:     make(map[string]domain.FilterStats),
	}

	startTime := time.Now()

	// Read input data line by line
	reader := bufio.NewScanner(bytes.NewReader(data))
	var processedLines []string

	for reader.Scan() {
		line := reader.Text()
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}

		stats.InputRecords++

		// Parse line as JSON (assuming NDJSON format)
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			log.Warnf("Failed to parse line as JSON: %v", err)
			stats.ErrorRecords++

			// Create a basic record with the raw line
			record = map[string]interface{}{
				"message":      line,
				"timestamp":    time.Now().UTC().Format(time.RFC3339),
				"_parse_error": err.Error(),
			}
		}

		// Process record through pipeline
		result, err := p.pipelineProcessor.ProcessRecord(ctx, tenantID, datasetID, record)
		if err != nil {
			log.Warnf("Pipeline processing failed for record: %v", err)
			stats.ErrorRecords++
			continue
		}

		if result.Skip {
			stats.FilteredRecords++
			continue
		}

		// Convert processed record back to JSON
		processedJSON, err := json.Marshal(result.Record)
		if err != nil {
			log.Warnf("Failed to marshal processed record: %v", err)
			stats.ErrorRecords++
			continue
		}

		processedLines = append(processedLines, string(processedJSON))
		stats.OutputRecords++
	}

	if err := reader.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading input data: %w", err)
	}

	// Join processed lines
	processedData := []byte(strings.Join(processedLines, "\n"))
	if len(processedLines) > 0 {
		processedData = append(processedData, '\n') // Add final newline
	}

	stats.OutputSize = int64(len(processedData))
	stats.ProcessingTime = time.Since(startTime)

	return processedData, stats, nil
}

// getPipelineConfig gets or creates a default pipeline configuration
func (p *PipelineProcessor) getPipelineConfig(ctx context.Context, tenantID, datasetID string) (*domain.PipelineConfiguration, error) {
	// For now, return a default configuration
	// In a real implementation, this would fetch from the control service

	defaultConfig := &domain.PipelineConfiguration{
		ConfigKey: fmt.Sprintf("%s:%s", tenantID, datasetID),
		TenantID:  tenantID,
		DatasetID: datasetID,
		Enabled:   true,
		Version:   "1.0",
		Filters: []domain.FilterConfig{
			{
				Type:    "parse",
				Enabled: true,
				Config: map[string]interface{}{
					"auto_select":      true,
					"source_field":     "message",
					"on_parse_failure": "pass_through",
					"preserve_raw":     false,
				},
			},
			{
				Type:    "add_field",
				Enabled: true,
				Config: map[string]interface{}{
					"field": "processed_by",
					"value": "bytefreezer-piper",
				},
			},
			{
				Type:    "add_field",
				Enabled: true,
				Config: map[string]interface{}{
					"field": "processed_at",
					"value": time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UpdatedBy: "system",
		Validated: true,
		Settings:  make(map[string]interface{}),
	}

	return defaultConfig, nil
}

// generateOutputKey generates the output S3 key for a processed file
func (p *PipelineProcessor) generateOutputKey(sourceKey string) string {
	// With separate buckets and no prefixes, we can use the same key structure
	return sourceKey
}
