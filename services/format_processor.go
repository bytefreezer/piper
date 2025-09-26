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

// FormatProcessor processes files with automatic format detection
type FormatProcessor struct {
	cfg            *config.Config
	s3Client       *storage.S3Client
	stateManager   *storage.PostgreSQLStateManager
	parserRegistry parsers.ParserRegistry
	formatDetector *parsers.FormatDetector
	filterRegistry pipeline.FilterRegistry
	configManager  *ConfigManager
}

// NewFormatProcessor creates a new format processor
func NewFormatProcessor(cfg *config.Config, s3Client *storage.S3Client, stateManager *storage.PostgreSQLStateManager) (*FormatProcessor, error) {
	// Create parser registry
	parserRegistry := parsers.NewRegistry()

	// Create format detector
	formatDetector := parsers.NewFormatDetector()

	// Create filter registry
	filterRegistry := pipeline.NewFilterRegistry()

	// Create configuration manager
	configManager := NewConfigManager(cfg, stateManager)

	processor := &FormatProcessor{
		cfg:            cfg,
		s3Client:       s3Client,
		stateManager:   stateManager,
		parserRegistry: parserRegistry,
		formatDetector: formatDetector,
		filterRegistry: filterRegistry,
		configManager:  configManager,
	}

	return processor, nil
}

// ProcessFile processes a single file with automatic format detection
func (p *FormatProcessor) ProcessFile(ctx context.Context, job *domain.ProcessingJob) (*domain.ProcessingResult, error) {
	startTime := time.Now()

	log.Infof("Processing file %s with format detection for tenant %s, dataset %s",
		job.SourceFile.Key, job.TenantID, job.DatasetID)

	// Detect format from filename
	formatHint, err := p.formatDetector.DetectFormat(job.SourceFile.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to detect format from filename: %w", err)
	}

	log.Infof("Detected format: %s for file %s", formatHint.Format, job.SourceFile.Key)

	// Skip binary formats for now as per requirements
	if formatHint.Binary {
		return nil, fmt.Errorf("binary format %s not supported yet", formatHint.Format)
	}

	// Download source file
	sourceData, err := p.s3Client.GetSourceObject(ctx, job.SourceFile.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to download source file: %w", err)
	}

	// Get pipeline configuration for this tenant/dataset
	pipelineConfig, err := p.configManager.GetPipelineConfig(ctx, job.TenantID, job.DatasetID)
	if err != nil {
		log.Warnf("Failed to get pipeline config for %s:%s, using default: %v", job.TenantID, job.DatasetID, err)
		pipelineConfig = p.createDefaultPipelineConfig(job.TenantID, job.DatasetID)
	}

	// Process the data with format-specific parser and filters
	processedData, stats, err := p.processDataWithFormat(ctx, formatHint, sourceData, pipelineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to process data: %w", err)
	}

	// Generate output file key (change extension to indicate NDJSON)
	outputKey := p.generateOutputKey(job.SourceFile.Key, formatHint)

	// Upload processed data
	if err := p.s3Client.PutDestinationObject(ctx, outputKey, processedData); err != nil {
		return nil, fmt.Errorf("failed to upload processed file: %w", err)
	}

	// Delete source file after successful processing
	if err := p.s3Client.DeleteSourceObject(ctx, job.SourceFile.Key); err != nil {
		log.Warnf("Failed to delete source file %s: %v", job.SourceFile.Key, err)
	}

	result := &domain.ProcessingResult{
		SourceFile:   job.SourceFile.Key,
		OutputFile:   outputKey,
		Stats:        *stats,
		Success:      true,
		PipelineUsed: fmt.Sprintf("format-processor:%s", formatHint.Format),
	}

	processingTime := time.Since(startTime)
	log.Infof("Successfully processed file %s (%s format) in %v, processed %d records",
		job.SourceFile.Key, formatHint.Format, processingTime, stats.OutputRecords)

	return result, nil
}

// processDataWithFormat processes data using the detected format parser
func (p *FormatProcessor) processDataWithFormat(ctx context.Context, formatHint *parsers.FormatHint, data []byte, pipelineConfig *domain.PipelineConfiguration) ([]byte, *domain.ProcessingStats, error) {
	stats := &domain.ProcessingStats{
		InputRecords:    0,
		OutputRecords:   0,
		FilteredRecords: 0,
		ErrorRecords:    0,
		InputSize:       int64(len(data)),
		FilterStats:     make(map[string]domain.FilterStats),
	}

	startTime := time.Now()

	// Create parser for the detected format
	parser, err := p.parserRegistry.CreateParser(formatHint.Format, map[string]interface{}{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create parser for format %s: %w", formatHint.Format, err)
	}

	log.Debugf("Created parser %s for format %s", parser.Name(), formatHint.Format)

	// Read input data line by line
	reader := bufio.NewScanner(bytes.NewReader(data))
	var processedLines []string

	// Special handling for CSV/TSV - extract headers first
	if formatHint.Format == "csv" || formatHint.Format == "tsv" {
		return p.processCSVData(ctx, parser, data, stats, pipelineConfig, formatHint)
	}

	// Process line by line for other formats
	lineNumber := int64(0)
	for reader.Scan() {
		line := reader.Text()
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}

		lineNumber++
		stats.InputRecords++

		// Parse line using format-specific parser
		record, err := parser.Parse(ctx, []byte(line))
		if err != nil {
			log.Debugf("Failed to parse line %d with %s parser: %v", lineNumber, formatHint.Format, err)
			stats.ErrorRecords++
			// Drop the line as per requirements
			continue
		}

		// Add metadata
		record["_source_format"] = formatHint.Format
		record["_tenant_id"] = formatHint.TenantID
		record["_dataset_id"] = formatHint.DatasetID
		record["_processed_at"] = time.Now().UTC().Format(time.RFC3339)
		record["_line_number"] = lineNumber

		// Apply filters if enabled
		if pipelineConfig.Enabled {
			filteredRecord, skip, err := p.applyFilters(ctx, record, formatHint, lineNumber, pipelineConfig)
			if err != nil {
				log.Warnf("Filter processing failed for line %d: %v", lineNumber, err)
				stats.ErrorRecords++
				continue
			}

			if skip {
				stats.FilteredRecords++
				continue
			}

			record = filteredRecord
		}

		// Convert to NDJSON
		processedJSON, err := json.Marshal(record)
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

	// Join processed lines as NDJSON
	processedData := []byte(strings.Join(processedLines, "\n"))
	if len(processedLines) > 0 {
		processedData = append(processedData, '\n') // Add final newline
	}

	stats.OutputSize = int64(len(processedData))
	stats.ProcessingTime = time.Since(startTime)

	log.Infof("Processed %d/%d records successfully, dropped %d failed records",
		stats.OutputRecords, stats.InputRecords, stats.ErrorRecords)

	return processedData, stats, nil
}

// processCSVData handles CSV/TSV data which needs special processing
func (p *FormatProcessor) processCSVData(ctx context.Context, parser parsers.Parser, data []byte, stats *domain.ProcessingStats, pipelineConfig *domain.PipelineConfiguration, formatHint *parsers.FormatHint) ([]byte, *domain.ProcessingStats, error) {
	// For CSV, we need to process the entire data at once to handle headers properly
	// This is a simplified approach - in production, you'd want to handle large files differently

	reader := bufio.NewScanner(bytes.NewReader(data))
	var lines []string

	// Read all lines first
	for reader.Scan() {
		line := reader.Text()
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return []byte{}, stats, nil
	}

	stats.InputRecords = int64(len(lines) - 1) // Subtract header row

	var processedLines []string

	// Process each data line (skip header)
	for i := 1; i < len(lines); i++ {
		// Create combined data with header + current line for CSV parser
		csvData := lines[0] + "\n" + lines[i]

		record, err := parser.Parse(ctx, []byte(csvData))
		if err != nil {
			log.Debugf("Failed to parse CSV line %d: %v", i+1, err)
			stats.ErrorRecords++
			continue
		}

		// Add metadata
		record["_source_format"] = formatHint.Format
		record["_tenant_id"] = formatHint.TenantID
		record["_dataset_id"] = formatHint.DatasetID
		record["_processed_at"] = time.Now().UTC().Format(time.RFC3339)
		record["_line_number"] = i + 1

		// Apply filters if enabled
		if pipelineConfig.Enabled {
			filteredRecord, skip, err := p.applyFilters(ctx, record, formatHint, int64(i+1), pipelineConfig)
			if err != nil {
				log.Warnf("Filter processing failed for CSV line %d: %v", i+1, err)
				stats.ErrorRecords++
				continue
			}

			if skip {
				stats.FilteredRecords++
				continue
			}

			record = filteredRecord
		}

		// Convert to NDJSON
		processedJSON, err := json.Marshal(record)
		if err != nil {
			log.Warnf("Failed to marshal CSV record: %v", err)
			stats.ErrorRecords++
			continue
		}

		processedLines = append(processedLines, string(processedJSON))
		stats.OutputRecords++
	}

	// Join processed lines as NDJSON
	processedData := []byte(strings.Join(processedLines, "\n"))
	if len(processedLines) > 0 {
		processedData = append(processedData, '\n')
	}

	stats.OutputSize = int64(len(processedData))

	return processedData, stats, nil
}

// generateOutputKey generates output key with proper NDJSON extension
func (p *FormatProcessor) generateOutputKey(sourceKey string, formatHint *parsers.FormatHint) string {
	// Remove compression extension if present
	outputKey := sourceKey
	if formatHint.Extension != "" {
		outputKey = strings.TrimSuffix(outputKey, "."+formatHint.Extension)
	}

	// Replace source format hint with ndjson indicator
	if strings.Contains(outputKey, "--"+formatHint.Format) {
		outputKey = strings.Replace(outputKey, "--"+formatHint.Format, "--ndjson", 1)
	} else {
		// Add ndjson suffix if no hint was present
		if strings.Contains(outputKey, ".") {
			parts := strings.Split(outputKey, ".")
			parts[len(parts)-2] = parts[len(parts)-2] + "--ndjson"
			outputKey = strings.Join(parts, ".")
		} else {
			outputKey = outputKey + "--ndjson"
		}
	}

	// Ensure .gz extension for output (compressed NDJSON)
	if !strings.HasSuffix(outputKey, ".gz") {
		outputKey += ".gz"
	}

	return outputKey
}

// applyFilters applies the configured filters to a record
func (p *FormatProcessor) applyFilters(ctx context.Context, record map[string]interface{}, formatHint *parsers.FormatHint, lineNumber int64, pipelineConfig *domain.PipelineConfiguration) (map[string]interface{}, bool, error) {
	// Create filter context
	filterCtx := &pipeline.FilterContext{
		TenantID:   formatHint.TenantID,
		DatasetID:  formatHint.DatasetID,
		LineNumber: lineNumber,
		Timestamp:  time.Now(),
		Variables:  make(map[string]string),
	}

	currentRecord := record

	// Apply each enabled filter
	for _, filterConfig := range pipelineConfig.Filters {
		if !filterConfig.Enabled {
			continue
		}

		// Create filter instance
		filter, err := p.filterRegistry.CreateFilter(filterConfig.Type, filterConfig.Config)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create filter %s: %w", filterConfig.Type, err)
		}

		// Apply filter
		result, err := filter.Apply(filterCtx, currentRecord)
		if err != nil {
			return nil, false, fmt.Errorf("failed to apply filter %s: %w", filterConfig.Type, err)
		}

		if result.Skip {
			return nil, true, nil // Record should be skipped
		}

		currentRecord = result.Record
	}

	return currentRecord, false, nil
}

// createDefaultPipelineConfig creates a default pipeline configuration when control service is unavailable
func (p *FormatProcessor) createDefaultPipelineConfig(tenantID, datasetID string) *domain.PipelineConfiguration {
	return &domain.PipelineConfiguration{
		ConfigKey: fmt.Sprintf("%s:%s", tenantID, datasetID),
		TenantID:  tenantID,
		DatasetID: datasetID,
		Enabled:   true,
		Version:   "1.0.0",
		Filters: []domain.FilterConfig{
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
					"field": "pipeline_version",
					"value": "default-1.0",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UpdatedBy: "system-default",
		Validated: true,
		Settings: map[string]interface{}{
			"auto_format_detection": true,
			"drop_parse_failures":   true,
			"compress_output":       true,
		},
	}
}
