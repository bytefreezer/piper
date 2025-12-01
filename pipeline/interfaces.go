package pipeline

import (
	"context"
	"time"

	"github.com/bytefreezer/piper/domain"
)

// Filter represents a single pipeline filter
type Filter interface {
	// Apply applies the filter to a record
	Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error)

	// Type returns the filter type name
	Type() string

	// Validate validates the filter configuration
	Validate(config map[string]interface{}) error
}

// FilterFactory creates filter instances
type FilterFactory func(config map[string]interface{}) (Filter, error)

// FilterRegistry manages available filters
type FilterRegistry interface {
	Register(filterType string, factory FilterFactory)
	CreateFilter(filterType string, config map[string]interface{}) (Filter, error)
	ListTypes() []string
}

// FilterConfig represents a single filter configuration
type FilterConfig struct {
	Type      string                 `json:"type"`
	Condition string                 `json:"condition,omitempty"`
	Config    map[string]interface{} `json:"config"`
	Enabled   bool                   `json:"enabled"`
}

// FilterContext provides context for filter execution
type FilterContext struct {
	TenantID     string
	DatasetID    string
	LineNumber   int64
	Timestamp    time.Time
	Variables    map[string]string
	GeoIPManager GeoIPManager
	StateManager interface{} // Database state manager for enrichers and other data
}

// FilterResult represents the result of filter application
type FilterResult struct {
	Record   map[string]interface{} `json:"record"`
	Skip     bool                   `json:"skip"`     // Skip this record (filtered out)
	Error    error                  `json:"error"`    // Error during processing
	Applied  bool                   `json:"applied"`  // Whether filter was actually applied
	Duration time.Duration          `json:"duration"` // Time taken to apply filter
}

// Pipeline represents a configured pipeline for a tenant/dataset
type Pipeline interface {
	// Process processes a single record through the filter chain
	Process(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error)

	// GetConfig returns the pipeline configuration
	GetConfig() *domain.PipelineConfiguration

	// GetStats returns pipeline statistics
	GetStats() *PipelineStats

	// Reload reloads the pipeline configuration
	Reload(config *domain.PipelineConfiguration) error
}

// PipelineProcessor manages pipelines for multiple tenants/datasets
type PipelineProcessor interface {
	// RegisterPipeline registers a pipeline for a tenant/dataset combination
	RegisterPipeline(tenantID, datasetID string, config *domain.PipelineConfiguration) error

	// GetPipeline retrieves a pipeline for a tenant/dataset combination
	GetPipeline(tenantID, datasetID string) (Pipeline, error)

	// ProcessRecord processes a single record using the appropriate pipeline
	ProcessRecord(ctx context.Context, tenantID, datasetID string, record map[string]interface{}) (*FilterResult, error)

	// GetPipelineStats returns statistics for a specific pipeline
	GetPipelineStats(tenantID, datasetID string) (*PipelineStats, error)

	// ReloadPipeline reloads configuration for a specific pipeline
	ReloadPipeline(tenantID, datasetID string) error
}

// GeoIPManager provides GeoIP lookup functionality
type GeoIPManager interface {
	LookupCity(ip string) (*GeoIPCityResult, error)
	LookupCountry(ip string) (*GeoIPCountryResult, error)
	IsEnabled() bool
}

// GeoIPCityResult represents GeoIP city lookup result
type GeoIPCityResult struct {
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region"`
	RegionCode  string  `json:"region_code"`
	City        string  `json:"city"`
	PostalCode  string  `json:"postal_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	TimeZone    string  `json:"timezone"`
}

// GeoIPCountryResult represents GeoIP country lookup result
type GeoIPCountryResult struct {
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
}

// PipelineStats represents statistics for a pipeline
type PipelineStats struct {
	TenantID           string                        `json:"tenant_id"`
	DatasetID          string                        `json:"dataset_id"`
	RecordsProcessed   int64                         `json:"records_processed"`
	RecordsFiltered    int64                         `json:"records_filtered"`
	RecordsErrored     int64                         `json:"records_errored"`
	TotalProcessTime   time.Duration                 `json:"total_process_time"`
	AverageProcessTime time.Duration                 `json:"average_process_time"`
	FilterStats        map[string]domain.FilterStats `json:"filter_stats"`
	LastProcessed      time.Time                     `json:"last_processed"`
	CreatedAt          time.Time                     `json:"created_at"`
}

// ProcessingMetrics represents metrics for the entire processing operation
type ProcessingMetrics struct {
	JobID           string                        `json:"job_id"`
	TenantID        string                        `json:"tenant_id"`
	DatasetID       string                        `json:"dataset_id"`
	SourceFile      string                        `json:"source_file"`
	OutputFile      string                        `json:"output_file"`
	StartTime       time.Time                     `json:"start_time"`
	EndTime         time.Time                     `json:"end_time"`
	Duration        time.Duration                 `json:"duration"`
	InputRecords    int64                         `json:"input_records"`
	OutputRecords   int64                         `json:"output_records"`
	FilteredRecords int64                         `json:"filtered_records"`
	ErrorRecords    int64                         `json:"error_records"`
	InputSizeBytes  int64                         `json:"input_size_bytes"`
	OutputSizeBytes int64                         `json:"output_size_bytes"`
	PipelineVersion string                        `json:"pipeline_version"`
	ProcessorID     string                        `json:"processor_id"`
	FilterMetrics   map[string]domain.FilterStats `json:"filter_metrics"`
}
