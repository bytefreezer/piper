// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// KVFilter parses key-value pairs from strings
type KVFilter struct {
	SourceField    string
	FieldSplit     string
	ValueSplit     string
	TargetField    string
	Prefix         string
	IncludeKeys    []string
	ExcludeKeys    []string
	TrimKey        string
	TrimValue      string
	AllowDuplicate bool
	DefaultValues  map[string]string
	RemoveField    bool
}

// NewKVFilter creates a new KV filter
func NewKVFilter(config map[string]interface{}) (Filter, error) {
	filter := &KVFilter{
		SourceField:    "message",
		FieldSplit:     " ",
		ValueSplit:     "=",
		IncludeKeys:    make([]string, 0),
		ExcludeKeys:    make([]string, 0),
		AllowDuplicate: true,
		DefaultValues:  make(map[string]string),
	}

	// Parse source_field
	if sourceField, ok := config["source_field"].(string); ok {
		filter.SourceField = sourceField
	}

	// Parse field_split
	if fieldSplit, ok := config["field_split"].(string); ok {
		filter.FieldSplit = fieldSplit
	}

	// Parse value_split
	if valueSplit, ok := config["value_split"].(string); ok {
		filter.ValueSplit = valueSplit
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}

	// Parse prefix
	if prefix, ok := config["prefix"].(string); ok {
		filter.Prefix = prefix
	}

	// Parse include_keys
	if includeKeys, ok := config["include_keys"].([]interface{}); ok {
		for _, key := range includeKeys {
			if keyStr, ok := key.(string); ok {
				filter.IncludeKeys = append(filter.IncludeKeys, keyStr)
			}
		}
	}

	// Parse exclude_keys
	if excludeKeys, ok := config["exclude_keys"].([]interface{}); ok {
		for _, key := range excludeKeys {
			if keyStr, ok := key.(string); ok {
				filter.ExcludeKeys = append(filter.ExcludeKeys, keyStr)
			}
		}
	}

	// Parse trim_key
	if trimKey, ok := config["trim_key"].(string); ok {
		filter.TrimKey = trimKey
	}

	// Parse trim_value
	if trimValue, ok := config["trim_value"].(string); ok {
		filter.TrimValue = trimValue
	}

	// Parse allow_duplicate
	if allowDup, ok := config["allow_duplicate"].(bool); ok {
		filter.AllowDuplicate = allowDup
	}

	// Parse default_values
	if defaults, ok := config["default_values"].(map[string]interface{}); ok {
		for k, v := range defaults {
			if vStr, ok := v.(string); ok {
				filter.DefaultValues[k] = vStr
			}
		}
	}

	// Parse remove_field
	if removeField, ok := config["remove_field"].(bool); ok {
		filter.RemoveField = removeField
	}

	return filter, nil
}

// Type returns the filter type
func (f *KVFilter) Type() string {
	return "kv"
}

// Validate validates the filter configuration
func (f *KVFilter) Validate(config map[string]interface{}) error {
	// field_split and value_split are required
	if _, ok := config["field_split"]; !ok {
		// Use default
	}
	if _, ok := config["value_split"]; !ok {
		// Use default
	}

	return nil
}

// Apply applies the KV filter to a record
func (f *KVFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Get source field value
	sourceValue, exists := record[f.SourceField]
	if !exists {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Convert to string
	sourceStr, ok := sourceValue.(string)
	if !ok {
		log.Debugf("KV filter: source field '%s' is not a string, skipping", f.SourceField)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Parse key-value pairs
	kvPairs := f.parseKeyValuePairs(sourceStr)

	if len(kvPairs) == 0 {
		log.Debugf("KV filter: no key-value pairs found in field '%s'", f.SourceField)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Apply default values for missing keys
	for key, value := range f.DefaultValues {
		if _, exists := kvPairs[key]; !exists {
			kvPairs[key] = value
		}
	}

	// Add parsed values to record
	if f.TargetField != "" {
		// Store in nested object
		record[f.TargetField] = kvPairs
	} else {
		// Add directly to record
		for key, value := range kvPairs {
			fieldName := f.Prefix + key
			record[fieldName] = value
		}
	}

	// Remove source field if configured
	if f.RemoveField {
		delete(record, f.SourceField)
	}

	log.Debugf("KV filter: extracted %d key-value pairs from '%s'", len(kvPairs), f.SourceField)

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// parseKeyValuePairs parses a string into key-value pairs
func (f *KVFilter) parseKeyValuePairs(input string) map[string]interface{} {
	result := make(map[string]interface{})

	// Split into field pairs
	pairs := strings.Split(input, f.FieldSplit)

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split into key and value
		parts := strings.SplitN(pair, f.ValueSplit, 2)
		if len(parts) != 2 {
			// Not a valid key-value pair, skip
			continue
		}

		key := parts[0]
		value := parts[1]

		// Trim key
		if f.TrimKey != "" {
			key = strings.Trim(key, f.TrimKey)
		}
		key = strings.TrimSpace(key)

		// Trim value
		if f.TrimValue != "" {
			value = strings.Trim(value, f.TrimValue)
		}
		value = strings.TrimSpace(value)

		// Skip if key is empty
		if key == "" {
			continue
		}

		// Check include/exclude lists
		if len(f.IncludeKeys) > 0 && !f.contains(f.IncludeKeys, key) {
			continue
		}

		if len(f.ExcludeKeys) > 0 && f.contains(f.ExcludeKeys, key) {
			continue
		}

		// Check for duplicates
		if !f.AllowDuplicate {
			if _, exists := result[key]; exists {
				// Key already exists, skip duplicate
				continue
			}
		}

		result[key] = value
	}

	return result
}

// contains checks if a string slice contains a value
func (f *KVFilter) contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
