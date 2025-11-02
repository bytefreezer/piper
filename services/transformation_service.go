package services

import (
	"bufio"
	"context"
	"github.com/bytedance/sonic"
	"fmt"
	"math/rand"
	"time"

	"github.com/n0needt0/bytefreezer-piper/api"
	"github.com/n0needt0/bytefreezer-piper/pipeline"
	"github.com/n0needt0/go-goodies/log"
)

// GetSchemaAndSamples retrieves schema and sample data for a dataset
func (s *Services) GetSchemaAndSamples(ctx context.Context, tenantID, datasetID string, count int) ([]api.SchemaField, []api.TransformationSample, int, error) {
	// Get latest file from S3 for this tenant/dataset
	s3Client := s.PiperService.s3Client
	prefix := fmt.Sprintf("%s/%s/", tenantID, datasetID)

	// List objects
	keys, err := s3Client.ListSourceObjects(ctx, prefix)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to list objects: %w", err)
	}

	if len(keys) == 0 {
		return nil, nil, 0, fmt.Errorf("no data files found for %s/%s", tenantID, datasetID)
	}

	// Use the most recent file
	latestFile := keys[0]

	// Download file
	object, err := s3Client.GetSourceObjectStream(ctx, latestFile)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	// Read and parse lines
	scanner := bufio.NewScanner(object)
	var allLines []string
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, 0, fmt.Errorf("error reading file: %w", err)
	}

	totalLines := len(allLines)
	if totalLines == 0 {
		return nil, nil, 0, fmt.Errorf("no lines found in file")
	}

	// Random sampling
	samples := make([]api.TransformationSample, 0, count)
	sampleIndices := getRandomIndices(totalLines, count)

	for _, idx := range sampleIndices {
		line := allLines[idx]

		// Parse NDJSON
		var parsedData map[string]interface{}
		if err := sonic.Unmarshal([]byte(line), &parsedData); err != nil {
			// If not JSON, create a simple structure
			parsedData = map[string]interface{}{
				"message": line,
			}
		}

		samples = append(samples, api.TransformationSample{
			LineNumber: idx + 1,
			RawData:    line,
			ParsedData: parsedData,
		})
	}

	// Build schema from samples
	schema := buildSchema(samples)

	return schema, samples, totalLines, nil
}

// TestTransformation tests transformation filters on sample data
func (s *Services) TestTransformation(ctx context.Context, tenantID, datasetID string, filters []api.FilterConfig, samples []api.TransformationSample) ([]api.TransformationResult, error) {
	// Create filter registry
	registry := pipeline.NewFilterRegistry()

	// Create filter instances
	filterInstances := make([]pipeline.Filter, 0, len(filters))
	for i, filterConfig := range filters {
		if !filterConfig.Enabled {
			continue
		}

		filter, err := registry.CreateFilter(filterConfig.Type, filterConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create filter %d (%s): %w", i, filterConfig.Type, err)
		}

		filterInstances = append(filterInstances, filter)
	}

	// Apply filters to each sample
	results := make([]api.TransformationResult, 0, len(samples))

	for _, sample := range samples {
		result := s.applyFiltersToSample(sample, filterInstances, tenantID, datasetID)
		results = append(results, result)
	}

	return results, nil
}

// ValidateFreshData validates transformation on fresh data from S3
func (s *Services) ValidateFreshData(ctx context.Context, tenantID, datasetID string, filters []api.FilterConfig, count int) ([]api.TransformationResult, string, int, error) {
	// Get latest file from S3
	s3Client := s.PiperService.s3Client
	prefix := fmt.Sprintf("%s/%s/", tenantID, datasetID)

	// List objects
	keys, err := s3Client.ListSourceObjects(ctx, prefix)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to list objects: %w", err)
	}

	if len(keys) == 0 {
		return nil, "", 0, fmt.Errorf("no data files found for %s/%s", tenantID, datasetID)
	}

	// Use the most recent file
	latestFile := keys[0]
	sourceFile := latestFile

	// Download file
	object, err := s3Client.GetSourceObjectStream(ctx, latestFile)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	// Read lines
	scanner := bufio.NewScanner(object)
	var allLines []string

	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, "", 0, fmt.Errorf("error reading file: %w", err)
	}

	totalLines := len(allLines)
	if totalLines == 0 {
		return nil, sourceFile, 0, fmt.Errorf("no lines found in file")
	}

	// Take first N lines (fresh data)
	testCount := count
	if testCount > totalLines {
		testCount = totalLines
	}

	// Create samples from fresh data
	samples := make([]api.TransformationSample, 0, testCount)
	for i := 0; i < testCount; i++ {
		line := allLines[i]

		var parsedData map[string]interface{}
		if err := sonic.Unmarshal([]byte(line), &parsedData); err != nil {
			parsedData = map[string]interface{}{
				"message": line,
			}
		}

		samples = append(samples, api.TransformationSample{
			LineNumber: i + 1,
			RawData:    line,
			ParsedData: parsedData,
		})
	}

	// Test transformation
	results, err := s.TestTransformation(ctx, tenantID, datasetID, filters, samples)
	if err != nil {
		return nil, sourceFile, totalLines, err
	}

	return results, sourceFile, totalLines, nil
}

// ActivateTransformation activates or deactivates transformation for a dataset
func (s *Services) ActivateTransformation(ctx context.Context, tenantID, datasetID string, filters []api.FilterConfig, enabled bool) (string, error) {
	// TODO: Store transformation config in control service or database
	// For now, we'll use the pipeline database cache

	// Create pipeline configuration
	pipelineConfig := map[string]interface{}{
		"tenant_id":  tenantID,
		"dataset_id": datasetID,
		"enabled":    enabled,
		"filters":    filters,
		"updated_at": time.Now().Format(time.RFC3339),
		"version":    fmt.Sprintf("v%d", time.Now().Unix()),
	}

	// Convert to JSON
	configJSON, err := sonic.Marshal(pipelineConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	// Store in state manager if available
	if s.StateManager != nil {
		// For now, log that we would store it
		log.Infof("Would store transformation config: %s", string(configJSON))
	}

	version := pipelineConfig["version"].(string)
	return version, nil
}

// GetTransformationStats retrieves statistics for a running transformation
func (s *Services) GetTransformationStats(ctx context.Context, tenantID, datasetID string) (api.TransformationStats, error) {
	// TODO: Retrieve real-time stats from metrics or database
	// For now, return mock data

	stats := api.TransformationStats{
		TenantID:       tenantID,
		DatasetID:      datasetID,
		Enabled:        true,
		FilterCount:    3,
		TotalProcessed: 1000,
		SuccessCount:   950,
		ErrorCount:     25,
		SkippedCount:   25,
		AvgRowsPerSec:  150.5,
		LastProcessed:  time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
	}

	return stats, nil
}

// PreviewTransformation generates preview with random samples
func (s *Services) PreviewTransformation(ctx context.Context, tenantID, datasetID string, count int) ([]api.TransformationResult, bool, int, string, error) {
	// Get pipeline configuration
	pipelineConfigInterface, err := s.GetPipelineConfigAsInterface(ctx, tenantID, datasetID)
	if err != nil {
		return nil, false, 0, "", fmt.Errorf("failed to get pipeline config: %w", err)
	}

	if pipelineConfigInterface == nil {
		return nil, false, 0, "", fmt.Errorf("no pipeline configured for %s/%s", tenantID, datasetID)
	}

	// Convert to domain.PipelineConfiguration
	configJSON, err := sonic.Marshal(pipelineConfigInterface)
	if err != nil {
		return nil, false, 0, "", fmt.Errorf("failed to marshal config: %w", err)
	}

	var pipelineConfig struct {
		Filters []struct {
			Type    string                 `json:"type"`
			Config  map[string]interface{} `json:"config"`
			Enabled bool                   `json:"enabled"`
		} `json:"filters"`
	}
	if err := sonic.Unmarshal(configJSON, &pipelineConfig); err != nil {
		return nil, false, 0, "", fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Extract filters
	var filters []api.FilterConfig
	if pipelineConfig.Filters != nil {
		for _, f := range pipelineConfig.Filters {
			filters = append(filters, api.FilterConfig{
				Type:    f.Type,
				Config:  f.Config,
				Enabled: f.Enabled,
			})
		}
	}

	// Get random samples
	_, samples, _, err := s.GetSchemaAndSamples(ctx, tenantID, datasetID, count)
	if err != nil {
		return nil, false, 0, "", err
	}

	// Apply transformation
	results, err := s.TestTransformation(ctx, tenantID, datasetID, filters, samples)
	if err != nil {
		return nil, false, 0, "", err
	}

	enabled := true // TODO: Get from actual config
	filterCount := len(filters)
	lastProcessed := time.Now().Format(time.RFC3339)

	return results, enabled, filterCount, lastProcessed, nil
}

// Helper functions

func getRandomIndices(total, count int) []int {
	if count >= total {
		indices := make([]int, total)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	// Use map to avoid duplicates
	selected := make(map[int]bool)
	indices := make([]int, 0, count)

	rand.Seed(time.Now().UnixNano())

	for len(indices) < count {
		idx := rand.Intn(total) // #nosec G404 - weak random is acceptable for data sampling, not cryptographic use
		if !selected[idx] {
			selected[idx] = true
			indices = append(indices, idx)
		}
	}

	return indices
}

func buildSchema(samples []api.TransformationSample) []api.SchemaField {
	// Collect all fields from all samples
	fieldTypes := make(map[string]map[string]int) // field -> type -> count
	fieldSamples := make(map[string]interface{})
	fieldNullable := make(map[string]bool)
	fieldCount := make(map[string]int)

	for _, sample := range samples {
		seenFields := make(map[string]bool)

		for key, value := range sample.ParsedData {
			seenFields[key] = true
			fieldCount[key]++

			if fieldTypes[key] == nil {
				fieldTypes[key] = make(map[string]int)
			}

			valueType := detectType(value)
			fieldTypes[key][valueType]++

			// Store sample value
			if _, exists := fieldSamples[key]; !exists {
				fieldSamples[key] = value
			}
		}

		// Mark fields not present in this sample as nullable
		for field := range fieldCount {
			if !seenFields[field] {
				fieldNullable[field] = true
			}
		}
	}

	// Build schema
	schema := make([]api.SchemaField, 0, len(fieldTypes))
	for field, types := range fieldTypes {
		// Find most common type
		maxCount := 0
		mostCommonType := "string"
		for t, c := range types {
			if c > maxCount {
				maxCount = c
				mostCommonType = t
			}
		}

		schema = append(schema, api.SchemaField{
			Name:     field,
			Type:     mostCommonType,
			Count:    fieldCount[field],
			Nullable: fieldNullable[field],
			Sample:   fieldSamples[field],
		})
	}

	return schema
}

func detectType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case bool:
		return "boolean"
	case float64, float32, int, int64, int32:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

func (s *Services) applyFiltersToSample(sample api.TransformationSample, filters []pipeline.Filter, tenantID, datasetID string) api.TransformationResult {
	start := time.Now()

	// Make a copy of the parsed data
	record := make(map[string]interface{})
	for k, v := range sample.ParsedData {
		record[k] = v
	}

	inputCopy := copyMap(record)

	appliedFilters := make([]string, 0)
	skipped := false
	errorMsg := ""

	// Create filter context
	ctx := &pipeline.FilterContext{
		TenantID:   tenantID,
		DatasetID:  datasetID,
		Timestamp:  time.Now(),
		LineNumber: int64(sample.LineNumber),
		Variables:  make(map[string]string),
	}

	// Apply each filter
	for _, filter := range filters {
		result, err := filter.Apply(ctx, record)
		if err != nil {
			errorMsg = fmt.Sprintf("Filter %s error: %v", filter.Type(), err)
			break
		}

		if result.Applied {
			appliedFilters = append(appliedFilters, filter.Type())
		}

		if result.Skip {
			skipped = true
			break
		}

		// Update record with filter result
		record = result.Record
	}

	duration := time.Since(start)

	return api.TransformationResult{
		Input:    inputCopy,
		Output:   record,
		Applied:  appliedFilters,
		Skipped:  skipped,
		Duration: duration.Milliseconds(),
		Error:    errorMsg,
	}
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{})
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
