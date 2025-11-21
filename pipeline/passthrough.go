package pipeline

import (
	"time"
)

// PassthroughFilter passes records through without modification
type PassthroughFilter struct {
	description string
}

// NewPassthroughFilter creates a new passthrough filter
func NewPassthroughFilter(config map[string]interface{}) (Filter, error) {
	description := ""
	if desc, ok := config["description"].(string); ok {
		description = desc
	}

	return &PassthroughFilter{
		description: description,
	}, nil
}

// Type returns the filter type
func (f *PassthroughFilter) Type() string {
	return "passthrough"
}

// Validate validates the filter configuration
func (f *PassthroughFilter) Validate(config map[string]interface{}) error {
	return nil // No required parameters
}

// Apply applies the filter to a record (passes through without modification)
func (f *PassthroughFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}
