package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/n0needt0/go-goodies/log"
	"github.com/swaggest/usecase"
	"github.com/swaggest/usecase/status"
)

// TransformationSample represents a sample record for testing
type TransformationSample struct {
	LineNumber int                    `json:"line_number"`
	RawData    string                 `json:"raw_data"`
	ParsedData map[string]interface{} `json:"parsed_data"`
}

// SchemaField represents a field in the data schema
type SchemaField struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Count    int         `json:"count"`
	Nullable bool        `json:"nullable"`
	Sample   interface{} `json:"sample,omitempty"`
}

// GetSchemaRequest represents the request to get schema and sample data
type GetSchemaRequest struct {
	TenantID  string `path:"tenantId" required:"true"`
	DatasetID string `path:"datasetId" required:"true"`
	Count     int    `query:"count" default:"10" description:"Number of sample records to return (default: 10, max: 100)"`
}

// GetSchemaResponse represents the schema and sample data response
type GetSchemaResponse struct {
	TenantID    string                 `json:"tenant_id"`
	DatasetID   string                 `json:"dataset_id"`
	Schema      []SchemaField          `json:"schema"`
	Samples     []TransformationSample `json:"samples"`
	TotalLines  int                    `json:"total_lines"`
	SampleCount int                    `json:"sample_count"`
}

// FilterConfig represents a single filter configuration
type FilterConfig struct {
	Type    string                 `json:"type" required:"true"`
	Config  map[string]interface{} `json:"config" required:"true"`
	Enabled bool                   `json:"enabled" default:"true"`
}

// TestTransformationRequest represents a request to test transformation filters
type TestTransformationRequest struct {
	TenantID  string                   `json:"tenant_id" required:"true"`
	DatasetID string                   `json:"dataset_id" required:"true"`
	Filters   []FilterConfig           `json:"filters" required:"true" maxItems:"10" description:"Up to 10 filters to apply"`
	Samples   []TransformationSample   `json:"samples" required:"true" description:"Sample data to test against"`
}

// TransformationResult represents the result of applying a transformation
type TransformationResult struct {
	Input     map[string]interface{} `json:"input"`
	Output    map[string]interface{} `json:"output"`
	Applied   []string               `json:"applied"` // List of filters that were applied
	Skipped   bool                   `json:"skipped"` // True if record was dropped
	Duration  int64                  `json:"duration_ms"`
	Error     string                 `json:"error,omitempty"`
}

// TestTransformationResponse represents the response from testing transformations
type TestTransformationResponse struct {
	Results      []TransformationResult `json:"results"`
	SuccessCount int                    `json:"success_count"`
	ErrorCount   int                    `json:"error_count"`
	SkippedCount int                    `json:"skipped_count"`
	TotalTime    int64                  `json:"total_time_ms"`
}

// ValidateFreshDataRequest represents a request to validate transformation on fresh data
type ValidateFreshDataRequest struct {
	TenantID  string         `json:"tenant_id" required:"true"`
	DatasetID string         `json:"dataset_id" required:"true"`
	Filters   []FilterConfig `json:"filters" required:"true" maxItems:"10"`
	Count     int            `json:"count" default:"100" description:"Number of fresh records to test (default: 100, max: 1000)"`
}

// ValidateFreshDataResponse represents the response from validating on fresh data
type ValidateFreshDataResponse struct {
	Results       []TransformationResult `json:"results"`
	SourceFile    string                 `json:"source_file"`
	SourceLines   int                    `json:"source_lines"`
	SuccessCount  int                    `json:"success_count"`
	ErrorCount    int                    `json:"error_count"`
	SkippedCount  int                    `json:"skipped_count"`
	TotalTime     int64                  `json:"total_time_ms"`
	AvgTimePerRow int64                  `json:"avg_time_per_row_ms"`
}

// ActivateTransformationRequest represents a request to activate a transformation
type ActivateTransformationRequest struct {
	TenantID  string         `json:"tenant_id" required:"true"`
	DatasetID string         `json:"dataset_id" required:"true"`
	Filters   []FilterConfig `json:"filters" required:"true" maxItems:"10"`
	Enabled   bool           `json:"enabled" required:"true"`
}

// ActivateTransformationResponse represents the response from activating a transformation
type ActivateTransformationResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Version   string `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

// TransformationStats represents statistics about a running transformation
type TransformationStats struct {
	TenantID       string  `json:"tenant_id"`
	DatasetID      string  `json:"dataset_id"`
	Enabled        bool    `json:"enabled"`
	FilterCount    int     `json:"filter_count"`
	TotalProcessed int64   `json:"total_processed"`
	SuccessCount   int64   `json:"success_count"`
	ErrorCount     int64   `json:"error_count"`
	SkippedCount   int64   `json:"skipped_count"`
	AvgRowsPerSec  float64 `json:"avg_rows_per_sec"`
	LastError      string  `json:"last_error,omitempty"`
	LastProcessed  string  `json:"last_processed"`
}

// GetTransformationStatsRequest represents a request to get transformation statistics
type GetTransformationStatsRequest struct {
	TenantID  string `path:"tenantId" required:"true"`
	DatasetID string `path:"datasetId" required:"true"`
}

// GetTransformationStatsResponse represents transformation statistics response
type GetTransformationStatsResponse struct {
	Stats TransformationStats `json:"stats"`
}

// PreviewTransformationRequest represents a request to preview transformation on random samples
type PreviewTransformationRequest struct {
	TenantID  string `path:"tenantId" required:"true"`
	DatasetID string `path:"datasetId" required:"true"`
	Count     int    `query:"count" default:"10" description:"Number of random samples to show (default: 10, max: 100)"`
}

// PreviewTransformationResponse represents a preview of input/output for transformation
type PreviewTransformationResponse struct {
	Samples       []TransformationResult `json:"samples"`
	Enabled       bool                   `json:"enabled"`
	FilterCount   int                    `json:"filter_count"`
	LastProcessed string                 `json:"last_processed"`
}

// GetSchema returns a handler for getting schema and sample data
func (api *API) GetSchema() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input GetSchemaRequest, output *GetSchemaResponse) error {
		// Validate count
		if input.Count <= 0 {
			input.Count = 10
		}
		if input.Count > 100 {
			input.Count = 100
		}

		// Get schema and samples from service
		schema, samples, totalLines, err := api.Services.GetSchemaAndSamples(ctx, input.TenantID, input.DatasetID, input.Count)
		if err != nil {
			log.Errorf("Failed to get schema for %s/%s: %v", input.TenantID, input.DatasetID, err)
			return status.Wrap(err, status.Internal)
		}

		output.TenantID = input.TenantID
		output.DatasetID = input.DatasetID
		output.Schema = schema
		output.Samples = samples
		output.TotalLines = totalLines
		output.SampleCount = len(samples)

		log.Infof("Retrieved schema with %d fields and %d samples for %s/%s", len(schema), len(samples), input.TenantID, input.DatasetID)

		return nil
	})

	u.SetTitle("Get Schema and Sample Data")
	u.SetDescription("Retrieve data schema and random sample records for transformation testing")
	u.SetTags("Transformations")

	return u
}

// TestTransformation returns a handler for testing transformation filters on sample data
func (api *API) TestTransformation() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input TestTransformationRequest, output *TestTransformationResponse) error {
		// Validate filters
		if len(input.Filters) == 0 {
			return fmt.Errorf("at least one filter is required")
		}
		if len(input.Filters) > 10 {
			return fmt.Errorf("maximum 10 filters allowed")
		}

		// Test transformation on samples
		results, err := api.Services.TestTransformation(ctx, input.TenantID, input.DatasetID, input.Filters, input.Samples)
		if err != nil {
			log.Errorf("Failed to test transformation for %s/%s: %v", input.TenantID, input.DatasetID, err)
			return status.Wrap(err, status.Internal)
		}

		// Calculate statistics
		successCount := 0
		errorCount := 0
		skippedCount := 0
		totalTime := int64(0)

		for _, result := range results {
			totalTime += result.Duration
			if result.Error != "" {
				errorCount++
			} else if result.Skipped {
				skippedCount++
			} else {
				successCount++
			}
		}

		output.Results = results
		output.SuccessCount = successCount
		output.ErrorCount = errorCount
		output.SkippedCount = skippedCount
		output.TotalTime = totalTime

		log.Infof("Tested transformation for %s/%s: %d success, %d error, %d skipped, %dms total",
			input.TenantID, input.DatasetID, successCount, errorCount, skippedCount, totalTime)

		return nil
	})

	u.SetTitle("Test Transformation")
	u.SetDescription("Test transformation filters on sample data and see input/output diff")
	u.SetTags("Transformations")

	return u
}

// ValidateFreshData returns a handler for validating transformation on fresh data
func (api *API) ValidateFreshData() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input ValidateFreshDataRequest, output *ValidateFreshDataResponse) error {
		// Validate count
		if input.Count <= 0 {
			input.Count = 100
		}
		if input.Count > 1000 {
			input.Count = 1000
		}

		// Validate filters
		if len(input.Filters) == 0 {
			return fmt.Errorf("at least one filter is required")
		}
		if len(input.Filters) > 10 {
			return fmt.Errorf("maximum 10 filters allowed")
		}

		// Get fresh data and test transformation
		results, sourceFile, sourceLines, err := api.Services.ValidateFreshData(ctx, input.TenantID, input.DatasetID, input.Filters, input.Count)
		if err != nil {
			log.Errorf("Failed to validate fresh data for %s/%s: %v", input.TenantID, input.DatasetID, err)
			return status.Wrap(err, status.Internal)
		}

		// Calculate statistics
		successCount := 0
		errorCount := 0
		skippedCount := 0
		totalTime := int64(0)

		for _, result := range results {
			totalTime += result.Duration
			if result.Error != "" {
				errorCount++
			} else if result.Skipped {
				skippedCount++
			} else {
				successCount++
			}
		}

		avgTime := int64(0)
		if len(results) > 0 {
			avgTime = totalTime / int64(len(results))
		}

		output.Results = results
		output.SourceFile = sourceFile
		output.SourceLines = sourceLines
		output.SuccessCount = successCount
		output.ErrorCount = errorCount
		output.SkippedCount = skippedCount
		output.TotalTime = totalTime
		output.AvgTimePerRow = avgTime

		log.Infof("Validated fresh data for %s/%s: %d/%d records processed, %d success, %d error, %d skipped, avg %dms/row",
			input.TenantID, input.DatasetID, len(results), sourceLines, successCount, errorCount, skippedCount, avgTime)

		return nil
	})

	u.SetTitle("Validate Fresh Data")
	u.SetDescription("Test transformation on fresh records from latest intake and see performance metrics")
	u.SetTags("Transformations")

	return u
}

// ActivateTransformation returns a handler for activating/deactivating transformation
func (api *API) ActivateTransformation() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input ActivateTransformationRequest, output *ActivateTransformationResponse) error {
		// Validate filters if enabling
		if input.Enabled {
			if len(input.Filters) == 0 {
				return fmt.Errorf("at least one filter is required when enabling")
			}
			if len(input.Filters) > 10 {
				return fmt.Errorf("maximum 10 filters allowed")
			}
		}

		// Activate or deactivate transformation
		version, err := api.Services.ActivateTransformation(ctx, input.TenantID, input.DatasetID, input.Filters, input.Enabled)
		if err != nil {
			log.Errorf("Failed to activate transformation for %s/%s: %v", input.TenantID, input.DatasetID, err)
			return status.Wrap(err, status.Internal)
		}

		action := "activated"
		if !input.Enabled {
			action = "deactivated"
		}

		output.Success = true
		output.Message = "Transformation " + action + " successfully"
		output.Version = version
		output.UpdatedAt = time.Now().Format(time.RFC3339)

		log.Infof("Transformation %s for %s/%s with %d filters, version: %s",
			action, input.TenantID, input.DatasetID, len(input.Filters), version)

		return nil
	})

	u.SetTitle("Activate Transformation")
	u.SetDescription("Activate or deactivate transformation on a dataset")
	u.SetTags("Transformations")

	return u
}

// GetTransformationStats returns a handler for getting transformation statistics
func (api *API) GetTransformationStats() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input GetTransformationStatsRequest, output *GetTransformationStatsResponse) error {
		stats, err := api.Services.GetTransformationStats(ctx, input.TenantID, input.DatasetID)
		if err != nil {
			log.Errorf("Failed to get transformation stats for %s/%s: %v", input.TenantID, input.DatasetID, err)
			return status.Wrap(err, status.Internal)
		}

		output.Stats = stats

		log.Debugf("Retrieved transformation stats for %s/%s: processed=%d, success=%d, error=%d, skipped=%d, avg_rows/sec=%.2f",
			input.TenantID, input.DatasetID, stats.TotalProcessed, stats.SuccessCount, stats.ErrorCount, stats.SkippedCount, stats.AvgRowsPerSec)

		return nil
	})

	u.SetTitle("Get Transformation Statistics")
	u.SetDescription("Get real-time statistics about transformation processing")
	u.SetTags("Transformations")

	return u
}

// PreviewTransformation returns a handler for previewing transformation with random samples
func (api *API) PreviewTransformation() usecase.Interactor {
	u := usecase.NewInteractor(func(ctx context.Context, input PreviewTransformationRequest, output *PreviewTransformationResponse) error {
		// Validate count
		if input.Count <= 0 {
			input.Count = 10
		}
		if input.Count > 100 {
			input.Count = 100
		}

		samples, enabled, filterCount, lastProcessed, err := api.Services.PreviewTransformation(ctx, input.TenantID, input.DatasetID, input.Count)
		if err != nil {
			log.Errorf("Failed to preview transformation for %s/%s: %v", input.TenantID, input.DatasetID, err)
			return status.Wrap(err, status.Internal)
		}

		output.Samples = samples
		output.Enabled = enabled
		output.FilterCount = filterCount
		output.LastProcessed = lastProcessed

		log.Infof("Generated %d transformation preview samples for %s/%s (enabled=%t, filters=%d)",
			len(samples), input.TenantID, input.DatasetID, enabled, filterCount)

		return nil
	})

	u.SetTitle("Preview Transformation")
	u.SetDescription("Preview transformation with random samples showing input/output side-by-side")
	u.SetTags("Transformations")

	return u
}

// Helper function to convert filter configs to JSON for logging
func filtersToJSON(filters []FilterConfig) string {
	data, _ := json.Marshal(filters)
	return string(data)
}
