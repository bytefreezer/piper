// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/bytedance/sonic"
	"github.com/bytefreezer/goodies/log"
	"github.com/bytefreezer/piper/api"
	"github.com/bytefreezer/piper/pipeline"
)

// GetSchemaAndSamples retrieves schema and sample data for a dataset from database
func (s *Services) GetSchemaAndSamples(ctx context.Context, tenantID, datasetID string, count int) ([]api.SchemaField, []api.TransformationSample, int, error) {
	if s.DatasetSampleClient == nil {
		return nil, nil, 0, fmt.Errorf("dataset sample client not available")
	}

	// Get input samples from database (these are the raw samples before transformation)
	dbSamples, err := s.DatasetSampleClient.GetLatestSamples(ctx, tenantID, datasetID, "input", count)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get samples from database: %w", err)
	}

	if len(dbSamples) == 0 {
		return nil, nil, 0, fmt.Errorf("no samples found for %s/%s - please process some data first", tenantID, datasetID)
	}

	// Convert database samples to API format
	samples := make([]api.TransformationSample, 0, len(dbSamples))
	for _, dbSample := range dbSamples {
		// Marshal sample data back to JSON string for RawData
		rawData, err := sonic.Marshal(dbSample.SampleData)
		if err != nil {
			log.Warnf("Failed to marshal sample data: %v", err)
			continue
		}

		samples = append(samples, api.TransformationSample{
			LineNumber: dbSample.LineNumber,
			RawData:    string(rawData),
			ParsedData: dbSample.SampleData,
		})
	}

	if len(samples) == 0 {
		return nil, nil, 0, fmt.Errorf("no valid samples found")
	}

	// Build schema from samples
	schema := buildSchema(samples)

	// Get total sample count (including both input and output)
	totalCount, err := s.DatasetSampleClient.GetSampleCount(ctx, tenantID, datasetID)
	if err != nil {
		log.Warnf("Failed to get total sample count: %v", err)
		totalCount = len(samples)
	}

	return schema, samples, totalCount, nil
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

// ValidateFreshData validates transformation on fresh data from database
func (s *Services) ValidateFreshData(ctx context.Context, tenantID, datasetID string, filters []api.FilterConfig, count int) ([]api.TransformationResult, string, int, error) {
	if s.DatasetSampleClient == nil {
		return nil, "", 0, fmt.Errorf("dataset sample client not available")
	}

	// Get input samples from database (raw data before transformation)
	dbSamples, err := s.DatasetSampleClient.GetLatestSamples(ctx, tenantID, datasetID, "input", count)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get samples from database: %w", err)
	}

	if len(dbSamples) == 0 {
		return nil, "", 0, fmt.Errorf("no samples found for %s/%s - please process some data first", tenantID, datasetID)
	}

	// Convert database samples to API format
	samples := make([]api.TransformationSample, 0, len(dbSamples))
	for _, dbSample := range dbSamples {
		// Marshal sample data back to JSON string for RawData
		rawData, err := sonic.Marshal(dbSample.SampleData)
		if err != nil {
			log.Warnf("Failed to marshal sample data: %v", err)
			continue
		}

		samples = append(samples, api.TransformationSample{
			LineNumber: dbSample.LineNumber,
			RawData:    string(rawData),
			ParsedData: dbSample.SampleData,
		})
	}

	if len(samples) == 0 {
		return nil, "", 0, fmt.Errorf("no valid samples found")
	}

	// Get total sample count
	totalCount, err := s.DatasetSampleClient.GetSampleCount(ctx, tenantID, datasetID)
	if err != nil {
		log.Warnf("Failed to get total sample count: %v", err)
		totalCount = len(samples)
	}

	// Test transformation on samples
	results, err := s.TestTransformation(ctx, tenantID, datasetID, filters, samples)
	if err != nil {
		return nil, "database", totalCount, err
	}

	sourceInfo := fmt.Sprintf("database (%d samples)", len(samples))
	return results, sourceInfo, totalCount, nil
}

// ActivateTransformation activates or deactivates transformation for a dataset
func (s *Services) ActivateTransformation(ctx context.Context, tenantID, datasetID string, filters []api.FilterConfig, enabled bool) (string, error) {
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
		log.Infof("Would store transformation config: %s", string(configJSON))
	}

	// Compute and cache schema from stored samples
	if s.DatasetSampleClient != nil {
		schema, _, _, err := s.GetSchemaAndSamples(ctx, tenantID, datasetID, 10)
		if err != nil {
			log.Warnf("Failed to compute schema for %s/%s: %v", tenantID, datasetID, err)
		} else {
			// Cache the input schema
			if err := s.DatasetSampleClient.UpsertSchema(ctx, tenantID, datasetID, "input", schema); err != nil {
				log.Errorf("Failed to cache input schema for %s/%s: %v", tenantID, datasetID, err)
			} else {
				log.Infof("Cached input schema for %s/%s (%d fields)", tenantID, datasetID, len(schema))
			}

			// Also compute and cache output schema if transformation is enabled
			if enabled && len(filters) > 0 {
				// Get output samples and compute output schema
				dbSamples, err := s.DatasetSampleClient.GetLatestSamples(ctx, tenantID, datasetID, "output", 10)
				if err == nil && len(dbSamples) > 0 {
					// Convert to TransformationSample format
					outputSamples := make([]api.TransformationSample, 0, len(dbSamples))
					for _, dbSample := range dbSamples {
						outputSamples = append(outputSamples, api.TransformationSample{
							LineNumber: dbSample.LineNumber,
							ParsedData: dbSample.SampleData,
						})
					}

					// Build output schema
					outputSchema := buildSchema(outputSamples)
					if err := s.DatasetSampleClient.UpsertSchema(ctx, tenantID, datasetID, "output", outputSchema); err != nil {
						log.Errorf("Failed to cache output schema for %s/%s: %v", tenantID, datasetID, err)
					} else {
						log.Infof("Cached output schema for %s/%s (%d fields)", tenantID, datasetID, len(outputSchema))
					}
				}
			}
		}
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
		TenantID:     tenantID,
		DatasetID:    datasetID,
		Timestamp:    time.Now(),
		LineNumber:   int64(sample.LineNumber),
		Variables:    make(map[string]string),
		StateManager: s.StateManager,
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
