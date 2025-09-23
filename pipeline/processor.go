package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/n0needt0/bytefreezer-piper/domain"
)

// BasicPipeline implements a basic filter pipeline
type BasicPipeline struct {
	config      *domain.PipelineConfiguration
	filters     []Filter
	stats       *PipelineStats
	registry    FilterRegistry
	mutex       sync.RWMutex
}

// NewBasicPipeline creates a new basic pipeline
func NewBasicPipeline(config *domain.PipelineConfiguration, filterRegistry FilterRegistry) (*BasicPipeline, error) {
	pipeline := &BasicPipeline{
		config:    config,
		filters:   make([]Filter, 0),
		registry:  filterRegistry,
		stats: &PipelineStats{
			TenantID:           config.TenantID,
			DatasetID:          config.DatasetID,
			RecordsProcessed:   0,
			RecordsFiltered:    0,
			RecordsErrored:     0,
			TotalProcessTime:   0,
			AverageProcessTime: 0,
			FilterStats:        make(map[string]domain.FilterStats),
			LastProcessed:      time.Time{},
			CreatedAt:          time.Now(),
		},
	}

	// Initialize filters from configuration
	if err := pipeline.loadFilters(); err != nil {
		return nil, fmt.Errorf("failed to load filters: %w", err)
	}

	return pipeline, nil
}

// Process processes a single record through the filter chain
func (p *BasicPipeline) Process(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	startTime := time.Now()
	result := &FilterResult{
		Record:  record,
		Skip:    false,
		Applied: false,
	}

	// Process through each filter
	for _, filter := range p.filters {
		if !p.isFilterEnabled(filter) {
			continue
		}

		filterResult, err := filter.Apply(ctx, result.Record)
		if err != nil {
			p.stats.RecordsErrored++
			return &FilterResult{
				Record: result.Record,
				Skip:   false,
				Error:  fmt.Errorf("filter %s failed: %w", filter.Type(), err),
			}, nil
		}

		if filterResult.Skip {
			p.stats.RecordsFiltered++
			return &FilterResult{
				Record:  result.Record,
				Skip:    true,
				Applied: true,
			}, nil
		}

		if filterResult.Record != nil {
			result.Record = filterResult.Record
			result.Applied = true
		}
	}

	// Update statistics
	processingTime := time.Since(startTime)
	p.stats.RecordsProcessed++
	p.stats.TotalProcessTime += processingTime
	p.stats.AverageProcessTime = p.stats.TotalProcessTime / time.Duration(p.stats.RecordsProcessed)
	p.stats.LastProcessed = time.Now()

	result.Duration = processingTime
	return result, nil
}

// GetConfig returns the pipeline configuration
func (p *BasicPipeline) GetConfig() *domain.PipelineConfiguration {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.config
}

// GetStats returns pipeline statistics
func (p *BasicPipeline) GetStats() *PipelineStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.stats
}

// Reload reloads the pipeline configuration
func (p *BasicPipeline) Reload(config *domain.PipelineConfiguration) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.config = config
	return p.loadFilters()
}

// loadFilters loads filters from configuration
func (p *BasicPipeline) loadFilters() error {
	p.filters = make([]Filter, 0, len(p.config.Filters))

	for _, filterConfig := range p.config.Filters {
		if !filterConfig.Enabled {
			continue
		}

		filter, err := p.registry.CreateFilter(filterConfig.Type, filterConfig.Config)
		if err != nil {
			return fmt.Errorf("failed to create filter %s: %w", filterConfig.Type, err)
		}

		p.filters = append(p.filters, filter)
	}

	return nil
}

// isFilterEnabled checks if a filter should be processed
func (p *BasicPipeline) isFilterEnabled(filter Filter) bool {
	// For now, all loaded filters are enabled
	// This could be extended to support runtime enable/disable
	return true
}

// BasicPipelineProcessor manages multiple pipelines
type BasicPipelineProcessor struct {
	pipelines    map[string]Pipeline
	filterReg    FilterRegistry
	mutex        sync.RWMutex
}

// NewBasicPipelineProcessor creates a new pipeline processor
func NewBasicPipelineProcessor(filterRegistry FilterRegistry) *BasicPipelineProcessor {
	return &BasicPipelineProcessor{
		pipelines: make(map[string]Pipeline),
		filterReg: filterRegistry,
	}
}

// RegisterPipeline registers a pipeline for a tenant/dataset combination
func (pp *BasicPipelineProcessor) RegisterPipeline(tenantID, datasetID string, config *domain.PipelineConfiguration) error {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	key := fmt.Sprintf("%s:%s", tenantID, datasetID)

	pipeline, err := NewBasicPipeline(config, pp.filterReg)
	if err != nil {
		return fmt.Errorf("failed to create pipeline for %s: %w", key, err)
	}

	pp.pipelines[key] = pipeline
	return nil
}

// GetPipeline retrieves a pipeline for a tenant/dataset combination
func (pp *BasicPipelineProcessor) GetPipeline(tenantID, datasetID string) (Pipeline, error) {
	pp.mutex.RLock()
	defer pp.mutex.RUnlock()

	key := fmt.Sprintf("%s:%s", tenantID, datasetID)
	pipeline, exists := pp.pipelines[key]
	if !exists {
		return nil, fmt.Errorf("pipeline not found for %s", key)
	}

	return pipeline, nil
}

// ProcessRecord processes a single record using the appropriate pipeline
func (pp *BasicPipelineProcessor) ProcessRecord(ctx context.Context, tenantID, datasetID string, record map[string]interface{}) (*FilterResult, error) {
	pipeline, err := pp.GetPipeline(tenantID, datasetID)
	if err != nil {
		return nil, err
	}

	filterCtx := &FilterContext{
		TenantID:  tenantID,
		DatasetID: datasetID,
		Timestamp: time.Now(),
		Variables: make(map[string]string),
	}

	return pipeline.Process(filterCtx, record)
}

// GetPipelineStats returns statistics for a specific pipeline
func (pp *BasicPipelineProcessor) GetPipelineStats(tenantID, datasetID string) (*PipelineStats, error) {
	pipeline, err := pp.GetPipeline(tenantID, datasetID)
	if err != nil {
		return nil, err
	}

	return pipeline.GetStats(), nil
}

// ReloadPipeline reloads configuration for a specific pipeline
func (pp *BasicPipelineProcessor) ReloadPipeline(tenantID, datasetID string) error {
	// This would typically fetch new configuration from the control service
	// For now, return not implemented
	return fmt.Errorf("pipeline reload not implemented")
}