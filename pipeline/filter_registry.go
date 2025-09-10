package pipeline

import (
	"fmt"
	"sort"
	"sync"
)

// DefaultFilterRegistry implements the FilterRegistry interface
type DefaultFilterRegistry struct {
	factories map[string]FilterFactory
	mu        sync.RWMutex
}

// NewFilterRegistry creates a new filter registry with default filters
func NewFilterRegistry() *DefaultFilterRegistry {
	registry := &DefaultFilterRegistry{
		factories: make(map[string]FilterFactory),
	}

	// Register all built-in filters
	registry.registerBuiltInFilters()

	return registry
}

// Register registers a filter factory with the given type name
func (r *DefaultFilterRegistry) Register(filterType string, factory FilterFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[filterType] = factory
}

// CreateFilter creates a filter instance of the specified type
func (r *DefaultFilterRegistry) CreateFilter(filterType string, config map[string]interface{}) (Filter, error) {
	r.mu.RLock()
	factory, exists := r.factories[filterType]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown filter type: %s", filterType)
	}

	return factory(config)
}

// ListTypes returns a sorted list of available filter types
func (r *DefaultFilterRegistry) ListTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.factories))
	for filterType := range r.factories {
		types = append(types, filterType)
	}

	sort.Strings(types)
	return types
}

// registerBuiltInFilters registers all built-in filter types
func (r *DefaultFilterRegistry) registerBuiltInFilters() {
	// Add field filter
	r.Register("add_field", func(config map[string]interface{}) (Filter, error) {
		return NewAddFieldFilter(config)
	})

	// Remove field filter
	r.Register("remove_field", func(config map[string]interface{}) (Filter, error) {
		return NewRemoveFieldFilter(config)
	})

	// Rename field filter
	r.Register("rename_field", func(config map[string]interface{}) (Filter, error) {
		return NewRenameFieldFilter(config)
	})

	// JSON parse filter
	r.Register("json_parse", func(config map[string]interface{}) (Filter, error) {
		return NewJSONParseFilter(config)
	})

	// Regex replace filter
	r.Register("regex_replace", func(config map[string]interface{}) (Filter, error) {
		return NewRegexReplaceFilter(config)
	})

	// Date parse filter
	r.Register("date_parse", func(config map[string]interface{}) (Filter, error) {
		return NewDateParseFilter(config)
	})

	// Conditional filter
	r.Register("conditional", func(config map[string]interface{}) (Filter, error) {
		return NewConditionalFilter(config)
	})

	// GeoIP filter
	r.Register("geoip", func(config map[string]interface{}) (Filter, error) {
		return NewGeoIPFilter(config)
	})
}

// ValidateFilterConfig validates a filter configuration
func ValidateFilterConfig(registry FilterRegistry, filterConfig FilterConfig) error {
	// Check if filter type exists
	availableTypes := registry.ListTypes()
	typeExists := false
	for _, availableType := range availableTypes {
		if availableType == filterConfig.Type {
			typeExists = true
			break
		}
	}

	if !typeExists {
		return fmt.Errorf("unknown filter type: %s (available: %v)", filterConfig.Type, availableTypes)
	}

	// Create a temporary filter instance to validate configuration
	filter, err := registry.CreateFilter(filterConfig.Type, filterConfig.Config)
	if err != nil {
		return fmt.Errorf("failed to create filter %s: %w", filterConfig.Type, err)
	}

	// Validate the configuration
	if err := filter.Validate(filterConfig.Config); err != nil {
		return fmt.Errorf("invalid configuration for filter %s: %w", filterConfig.Type, err)
	}

	return nil
}

// ValidatePipelineConfig validates an entire pipeline configuration
func ValidatePipelineConfig(registry FilterRegistry, config *PipelineConfiguration) error {
	if config == nil {
		return fmt.Errorf("pipeline configuration is nil")
	}

	if config.TenantID == "" {
		return fmt.Errorf("tenant ID is required")
	}

	if config.DatasetID == "" {
		return fmt.Errorf("dataset ID is required")
	}

	// Validate each filter
	for i, filterConfig := range config.Filters {
		if err := ValidateFilterConfig(registry, filterConfig); err != nil {
			return fmt.Errorf("filter %d validation failed: %w", i, err)
		}
	}

	return nil
}