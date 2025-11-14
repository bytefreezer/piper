package services

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"github.com/bytedance/sonic"
	"fmt"
	"io"
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
	stateManager   storage.StateManager
	sampleClient   *storage.DatasetSampleClient
	parserRegistry parsers.ParserRegistry
	formatDetector *parsers.FormatDetector
	filterRegistry pipeline.FilterRegistry
	configManager  *ConfigManager
}

// NewFormatProcessor creates a new format processor
func NewFormatProcessor(cfg *config.Config, s3Client *storage.S3Client, stateManager storage.StateManager) (*FormatProcessor, error) {
	// Create parser registry
	parserRegistry := parsers.NewRegistry()

	// Create format detector
	formatDetector := parsers.NewFormatDetector()

	// Create filter registry
	filterRegistry := pipeline.NewFilterRegistry()

	// Create configuration manager
	configManager := NewConfigManager(cfg, stateManager)

	// Create dataset sample client for storing input/output samples
	sampleClient := storage.NewDatasetSampleClient(stateManager)

	processor := &FormatProcessor{
		cfg:            cfg,
		s3Client:       s3Client,
		stateManager:   stateManager,
		sampleClient:   sampleClient,
		parserRegistry: parserRegistry,
		formatDetector: formatDetector,
		filterRegistry: filterRegistry,
		configManager:  configManager,
	}

	return processor, nil
}

// ProcessFile processes a single file with automatic format detection
func (p *FormatProcessor) ProcessFile(ctx context.Context, job *domain.ProcessingJob) (*domain.ProcessingResult, error) {
	// Delegate to streaming implementation for memory efficiency
	log.Infof("Using streaming processing for file %s (memory-efficient mode)", job.SourceFile.Key)
	return p.ProcessFileStreaming(ctx, job)
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

	// Special handling for NDJSON - reconstruct complete JSON objects from malformed files
	if formatHint.Format == "ndjson" || formatHint.Format == "json" {
		return p.processNDJSONData(ctx, parser, data, stats, pipelineConfig, formatHint)
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
		processedJSON, err := sonic.Marshal(record)
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
		processedJSON, err := sonic.Marshal(record)
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

// processNDJSONData handles NDJSON data with simple line-by-line processing (files verified upstream)
func (p *FormatProcessor) processNDJSONData(ctx context.Context, parser parsers.Parser, data []byte, stats *domain.ProcessingStats, pipelineConfig *domain.PipelineConfiguration, formatHint *parsers.FormatHint) ([]byte, *domain.ProcessingStats, error) {
	startTime := time.Now()

	log.Infof("Processing NDJSON data (%d bytes) line-by-line", len(data))

	// Create scanner for line-by-line processing
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var processedLines []string
	lineNumber := int64(0)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}

		lineNumber++
		stats.InputRecords++

		// Since files are verified upstream, parse directly as JSON
		record, err := parser.Parse(ctx, []byte(line))
		if err != nil {
			log.Debugf("Failed to parse line %d: %v", lineNumber, err)
			stats.ErrorRecords++
			continue
		}

		// Add metadata
		record["_source_format"] = formatHint.Format
		record["_tenant_id"] = formatHint.TenantID
		record["_dataset_id"] = formatHint.DatasetID
		record["_processed_at"] = time.Now().UTC().Format(time.RFC3339)
		record["_line_number"] = lineNumber

		// Apply filters if enabled (pass-through if disabled)
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

		// Convert back to NDJSON
		processedJSON, err := sonic.Marshal(record)
		if err != nil {
			log.Warnf("Failed to marshal processed record: %v", err)
			stats.ErrorRecords++
			continue
		}

		processedLines = append(processedLines, string(processedJSON))
		stats.OutputRecords++
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading NDJSON data: %w", err)
	}

	// Join processed lines as NDJSON
	processedData := []byte(strings.Join(processedLines, "\n"))
	if len(processedLines) > 0 {
		processedData = append(processedData, '\n') // Add final newline
	}

	stats.OutputSize = int64(len(processedData))
	stats.ProcessingTime = time.Since(startTime)

	log.Infof("Processed %d/%d lines successfully in %v, dropped %d failed lines",
		stats.OutputRecords, stats.InputRecords, stats.ProcessingTime, stats.ErrorRecords)

	return processedData, stats, nil
}


// generateOutputKey generates output key with proper NDJSON extension
func (p *FormatProcessor) generateOutputKey(sourceKey string, formatHint *parsers.FormatHint) string {
	// Remove compression extension if present
	outputKey := sourceKey
	if formatHint.Extension != "" {
		outputKey = strings.TrimSuffix(outputKey, "."+formatHint.Extension)
	}

	// Always append --ndjson to show output format, preserving input format hint
	if strings.Contains(outputKey, ".") {
		parts := strings.Split(outputKey, ".")
		parts[len(parts)-2] = parts[len(parts)-2] + "--ndjson"
		outputKey = strings.Join(parts, ".")
	} else {
		outputKey = outputKey + "--ndjson"
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

// decompressGzip decompresses gzipped data
func (p *FormatProcessor) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip data: %w", err)
	}

	return decompressed, nil
}

// compressData compresses data using gzip
func (p *FormatProcessor) compressData(data []byte) ([]byte, error) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)

	_, err := writer.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to write data to gzip writer: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return compressed.Bytes(), nil
}

// ProcessFileStreaming processes a file using streaming I/O to minimize memory usage
func (p *FormatProcessor) ProcessFileStreaming(ctx context.Context, job *domain.ProcessingJob) (*domain.ProcessingResult, error) {
	log.Infof("Starting streaming processing for file %s", job.SourceFile.Key)
	startTime := time.Now()

	// Format detection
	formatHint, err := p.formatDetector.DetectFormat(job.SourceFile.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to detect format: %w", err)
	}
	log.Debugf("Detected format: %s (extension: %s, binary: %t)", formatHint.Format, formatHint.Extension, formatHint.Binary)

	// Skip binary formats for now as per requirements
	if formatHint.Binary {
		return nil, fmt.Errorf("binary format %s not supported yet", formatHint.Format)
	}

	// Get pipeline configuration for this tenant/dataset
	pipelineConfig, err := p.configManager.GetPipelineConfig(ctx, job.TenantID, job.DatasetID)
	if err != nil {
		log.Warnf("Failed to get pipeline config for %s:%s, using default: %v", job.TenantID, job.DatasetID, err)
		pipelineConfig = p.createDefaultPipelineConfig(job.TenantID, job.DatasetID)
	}

	// Open streaming connection to S3 source
	sourceReader, err := p.s3Client.GetSourceObjectStream(ctx, job.SourceFile.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to open source stream: %w", err)
	}
	defer sourceReader.Close()

	// Create decompression stream if needed
	var dataReader io.Reader = sourceReader
	if formatHint.Extension == "gz" || strings.HasSuffix(job.SourceFile.Key, ".gz") {
		log.Infof("Setting up streaming decompression for %s", job.SourceFile.Key)
		gzReader, err := gzip.NewReader(sourceReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		dataReader = gzReader
	}

	// Create output key
	outputKey := p.generateOutputKey(job.SourceFile.Key, formatHint)

	// Process streaming: read->pipeline->compress->upload
	stats, err := p.processStreamingNDJSON(ctx, dataReader, outputKey, pipelineConfig, job)
	if err != nil {
		return nil, fmt.Errorf("failed to process streaming data: %w", err)
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
		PipelineUsed: fmt.Sprintf("streaming-format-processor:%s", formatHint.Format),
	}

	finalProcessingTime := time.Since(startTime)
	log.Infof("Successfully processed file %s (streaming %s format) in %v, processed %d records",
		job.SourceFile.Key, formatHint.Format, finalProcessingTime, stats.OutputRecords)

	return result, nil
}

// processStreamingNDJSON processes NDJSON data using streaming I/O with pipeline processing
func (p *FormatProcessor) processStreamingNDJSON(ctx context.Context, dataReader io.Reader, outputKey string, pipelineConfig *domain.PipelineConfiguration, job *domain.ProcessingJob) (*domain.ProcessingStats, error) {
	log.Infof("Starting streaming NDJSON processing for output key: %s", outputKey)

	// Use buffered approach for S3 compatibility (no pipe streaming to S3)
	var processedLines []string
	var stats domain.ProcessingStats
	startTime := time.Now()

	scanner := bufio.NewScanner(dataReader)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024) // 64KB initial, 10MB max

	var lineCount int64
	var errorCount int64

	// Collect samples for schema inference (keep first 10 input and output samples)
	var inputSamples []storage.DatasetSample
	var outputSamples []storage.DatasetSample
	const maxSamples = 10
	batchID := fmt.Sprintf("batch-%d", time.Now().Unix())

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Collect input sample (before pipeline)
		if len(inputSamples) < maxSamples {
			var inputData map[string]interface{}
			if err := sonic.Unmarshal([]byte(line), &inputData); err == nil {
				// Create clean copy without internal metadata
				sampleData := make(map[string]interface{})
				for k, v := range inputData {
					if !strings.HasPrefix(k, "_") {
						sampleData[k] = v
					}
				}
				inputSamples = append(inputSamples, storage.DatasetSample{
					TenantID:   job.TenantID,
					DatasetID:  job.DatasetID,
					SampleType: "input",
					LineNumber: int(lineCount),
					SampleData: sampleData,
					BatchID:    batchID,
					CreatedAt:  time.Now(),
				})
			}
		}

		// Process line through pipeline if enabled
		processedLine := line
		if pipelineConfig != nil && pipelineConfig.Enabled {
			processed, err := p.processLineWithPipeline(ctx, line, pipelineConfig, job.TenantID, job.DatasetID)
			if err != nil {
				log.Warnf("Pipeline processing failed for line %d: %v", lineCount, err)
				errorCount++

				// Use original line on pipeline failure
				processedLine = line
			} else {
				processedLine = processed
			}
		}

		// Collect output sample (after pipeline)
		if len(outputSamples) < maxSamples {
			var outputData map[string]interface{}
			if err := sonic.Unmarshal([]byte(processedLine), &outputData); err == nil {
				// Create clean copy without internal metadata
				sampleData := make(map[string]interface{})
				for k, v := range outputData {
					if !strings.HasPrefix(k, "_") {
						sampleData[k] = v
					}
				}
				outputSamples = append(outputSamples, storage.DatasetSample{
					TenantID:   job.TenantID,
					DatasetID:  job.DatasetID,
					SampleType: "output",
					LineNumber: int(lineCount),
					SampleData: sampleData,
					BatchID:    batchID,
					CreatedAt:  time.Now(),
				})
			}
		}

		// Add processed line to buffer
		processedLines = append(processedLines, processedLine)

		// Log progress every 10000 lines
		if lineCount%10000 == 0 {
			log.Debugf("Processed %d lines for %s", lineCount, outputKey)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error at line %d: %w", lineCount, err)
	}

	// Store samples in database for schema inference
	if p.sampleClient != nil && (len(inputSamples) > 0 || len(outputSamples) > 0) {
		// Store input samples
		if len(inputSamples) > 0 {
			if err := p.sampleClient.UpsertSamples(ctx, job.TenantID, job.DatasetID, "input", inputSamples, maxSamples); err != nil {
				log.Warnf("Failed to store input samples: %v", err)
			} else {
				log.Infof("Stored %d input samples for %s/%s", len(inputSamples), job.TenantID, job.DatasetID)
			}
		}

		// Store output samples
		if len(outputSamples) > 0 {
			if err := p.sampleClient.UpsertSamples(ctx, job.TenantID, job.DatasetID, "output", outputSamples, maxSamples); err != nil {
				log.Warnf("Failed to store output samples: %v", err)
			} else {
				log.Infof("Stored %d output samples for %s/%s", len(outputSamples), job.TenantID, job.DatasetID)
			}
		}
	}

	stats = domain.ProcessingStats{
		InputRecords:   lineCount,
		OutputRecords:  lineCount - errorCount,
		ErrorRecords:   errorCount,
		ProcessingTime: time.Since(startTime),
	}

	// Convert processed lines to compressed data
	processedData := []byte(strings.Join(processedLines, "\n"))
	if len(processedLines) > 0 {
		processedData = append(processedData, '\n') // Add final newline
	}

	// Compress the data
	compressedData, err := p.compressData(processedData)
	if err != nil {
		return nil, fmt.Errorf("failed to compress processed data: %w", err)
	}

	// Set OutputSize to the COMPRESSED size that will be uploaded to S3
	// This ensures metrics reflect the actual bytes stored on S3 (matching packer's input)
	stats.OutputSize = int64(len(compressedData))

	log.Infof("Finished processing %d lines for %s (%d errors), compressed from %d to %d bytes",
		lineCount, outputKey, errorCount, len(processedData), len(compressedData))

	// Create metadata for the processed file
	sourceMetadata, err := p.s3Client.GetSourceObjectMetadata(ctx, job.SourceFile.Key)
	if err != nil {
		log.Warnf("Failed to get source metadata for %s: %v", job.SourceFile.Key, err)
		sourceMetadata = make(map[string]string)
	}

	// Create processing metadata with reduced redundancy as per todo requirements
	// Extract actual filename from key (not full path)
	filename := job.SourceFile.Key
	if lastSlash := strings.LastIndex(filename, "/"); lastSlash != -1 {
		filename = filename[lastSlash+1:]
	}

	processingMetadata := map[string]string{
		"source-original-filename": filename, // Just filename, not full S3 key path
		"tenant-id":               job.TenantID,
		"dataset-id":              job.DatasetID,
		"format":                  "ndjson",
		"processed-at":            time.Now().Format(time.RFC3339),
		"processor-type":          "bytefreezer-piper-streaming",
		"input-records":           fmt.Sprintf("%d", stats.InputRecords),
		"output-records":          fmt.Sprintf("%d", stats.OutputRecords),
	}

	// Add pipeline info if used
	if pipelineConfig != nil && pipelineConfig.Enabled {
		processingMetadata["pipeline-enabled"] = "true"
		processingMetadata["pipeline-id"] = pipelineConfig.ConfigKey
	} else {
		processingMetadata["pipeline-enabled"] = "false"
	}

	// Copy selective source metadata based on todo requirements
	// Filter metadata based on user requirements:
	// Keep: source-original-filename, tenant-id, source-content-length, source-etag, source-last-modified, source-line-count
	// Remove: source-file-key, source-tenant (redundant with tenant-id)
	for key, value := range sourceMetadata {
		// Keep essential technical metadata
		if key == "source-content-length" || key == "source-etag" || key == "source-last-modified" {
			processingMetadata[key] = value
		}
		// Keep important source identification (original filename, not file-key)
		if key == "source-original-filename" {
			processingMetadata[key] = value
		}
		// Keep tenant-id but skip redundant source-tenant
		if key == "tenant-id" {
			processingMetadata[key] = value
		}
		// Keep source line count to track discrepancies
		if key == "source-line-count" {
			processingMetadata[key] = value
		}
		// Skip redundant fields: source-file-key, source-tenant
	}

	// Upload compressed data to S3
	log.Infof("Uploading compressed data to S3: %s (%d bytes)", outputKey, len(compressedData))
	compressedReader := bytes.NewReader(compressedData)
	err = p.s3Client.UploadProcessedFile(ctx, outputKey, compressedReader, processingMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to upload processed file: %w", err)
	}

	log.Infof("Successfully completed streaming processing for %s", outputKey)
	return &stats, nil
}

// processLineWithPipeline processes a single line through the configured pipeline
func (p *FormatProcessor) processLineWithPipeline(ctx context.Context, line string, config *domain.PipelineConfiguration, tenantID, datasetID string) (string, error) {
	// For now, return the line as-is since pipeline processing is disabled in config
	// This is where pipeline transformations would be applied when enabled
	return line, nil
}
