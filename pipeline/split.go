// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// SplitFilter splits a single event into multiple events
type SplitFilter struct {
	Field      string
	Terminator string
	Target     string
}

// NewSplitFilter creates a new split filter
func NewSplitFilter(config map[string]interface{}) (Filter, error) {
	filter := &SplitFilter{
		Field: "message",
	}

	// Parse field
	if field, ok := config["field"].(string); ok {
		filter.Field = field
	}

	// Parse terminator (for splitting strings)
	if terminator, ok := config["terminator"].(string); ok {
		filter.Terminator = terminator
	}

	// Parse target field (optional - where to place split value)
	if target, ok := config["target"].(string); ok {
		filter.Target = target
	}

	return filter, nil
}

// Type returns the filter type
func (f *SplitFilter) Type() string {
	return "split"
}

// Validate validates the filter configuration
func (f *SplitFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["field"]; !ok {
		return fmt.Errorf("split filter requires 'field' configuration")
	}
	return nil
}

// Apply applies the split filter to a record
func (f *SplitFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Get field value
	fieldValue, exists := record[f.Field]
	if !exists {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Handle different value types
	var items []interface{}

	switch v := fieldValue.(type) {
	case []interface{}:
		// Already an array
		items = v

	case []string:
		// String array
		items = make([]interface{}, len(v))
		for i, s := range v {
			items[i] = s
		}

	case string:
		// String - split by terminator if provided
		if f.Terminator != "" {
			parts := strings.Split(v, f.Terminator)
			items = make([]interface{}, len(parts))
			for i, part := range parts {
				items[i] = strings.TrimSpace(part)
			}
		} else {
			// No terminator, treat as single item
			items = []interface{}{v}
		}

	default:
		// Other types - treat as single item
		items = []interface{}{v}
	}

	// If only one item, return original record
	if len(items) <= 1 {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Create multiple records (one per item)
	// Note: This requires pipeline changes to handle multiple records
	// For now, we'll store the split items and let the pipeline handle it
	targetField := f.Target
	if targetField == "" {
		targetField = f.Field
	}

	// Store split result as metadata for pipeline to expand
	record["_split_field"] = targetField
	record["_split_items"] = items
	record["_split_count"] = len(items)

	log.Debugf("Split filter: split field '%s' into %d items", f.Field, len(items))

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}
