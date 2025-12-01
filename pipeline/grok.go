package pipeline

import (
	"fmt"
	"regexp"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// GrokFilter performs pattern-based parsing of unstructured log data
type GrokFilter struct {
	SourceField       string
	Pattern           string
	Patterns          []string
	BreakOnMatch      bool
	KeepEmptyCaptures bool
	NamedCapturesOnly bool
	OverwriteKeys     bool
	TargetField       string
	PatternLibrary    *GrokPatternLibrary
	CompiledPatterns  []*regexp.Regexp
	FieldMaps         []map[int]string
}

// NewGrokFilter creates a new Grok filter
func NewGrokFilter(config map[string]interface{}) (Filter, error) {
	filter := &GrokFilter{
		SourceField:       "message",
		BreakOnMatch:      true,
		KeepEmptyCaptures: false,
		NamedCapturesOnly: true,
		OverwriteKeys:     true,
		PatternLibrary:    NewGrokPatternLibrary(),
	}

	// Parse source_field
	if sourceField, ok := config["source_field"].(string); ok {
		filter.SourceField = sourceField
	}

	// Parse pattern (single pattern)
	if pattern, ok := config["pattern"].(string); ok {
		filter.Pattern = pattern
		filter.Patterns = []string{pattern}
	}

	// Parse patterns (multiple patterns)
	if patterns, ok := config["patterns"].([]interface{}); ok {
		filter.Patterns = make([]string, 0, len(patterns))
		for _, p := range patterns {
			if pattern, ok := p.(string); ok {
				filter.Patterns = append(filter.Patterns, pattern)
			}
		}
	}

	// Parse break_on_match
	if breakOnMatch, ok := config["break_on_match"].(bool); ok {
		filter.BreakOnMatch = breakOnMatch
	}

	// Parse keep_empty_captures
	if keepEmpty, ok := config["keep_empty_captures"].(bool); ok {
		filter.KeepEmptyCaptures = keepEmpty
	}

	// Parse named_captures_only
	if namedOnly, ok := config["named_captures_only"].(bool); ok {
		filter.NamedCapturesOnly = namedOnly
	}

	// Parse overwrite_keys
	if overwrite, ok := config["overwrite_keys"].(bool); ok {
		filter.OverwriteKeys = overwrite
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}

	// Parse custom patterns
	if customPatterns, ok := config["custom_patterns"].(map[string]interface{}); ok {
		for name, pattern := range customPatterns {
			if patternStr, ok := pattern.(string); ok {
				filter.PatternLibrary.AddPattern(name, patternStr)
			}
		}
	}

	// Validate we have at least one pattern
	if len(filter.Patterns) == 0 {
		return nil, fmt.Errorf("grok filter requires at least one pattern")
	}

	// Compile all patterns
	filter.CompiledPatterns = make([]*regexp.Regexp, 0, len(filter.Patterns))
	filter.FieldMaps = make([]map[int]string, 0, len(filter.Patterns))

	for i, pattern := range filter.Patterns {
		compiled, fieldMap, err := filter.PatternLibrary.CompilePattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern %d: %w", i, err)
		}
		filter.CompiledPatterns = append(filter.CompiledPatterns, compiled)
		filter.FieldMaps = append(filter.FieldMaps, fieldMap)
	}

	return filter, nil
}

// Type returns the filter type
func (f *GrokFilter) Type() string {
	return "grok"
}

// Validate validates the filter configuration
func (f *GrokFilter) Validate(config map[string]interface{}) error {
	// Check for required fields
	_, hasPattern := config["pattern"]
	_, hasPatterns := config["patterns"]

	if !hasPattern && !hasPatterns {
		return fmt.Errorf("grok filter requires 'pattern' or 'patterns' field")
	}

	return nil
}

// Apply applies the grok filter to a record
func (f *GrokFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
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
		log.Debugf("Grok filter: source field '%s' is not a string, skipping", f.SourceField)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Try each pattern until we find a match
	matched := false
	extractedFields := make(map[string]interface{})

	for i, compiledPattern := range f.CompiledPatterns {
		if matches := compiledPattern.FindStringSubmatch(sourceStr); matches != nil {
			matched = true
			fieldMap := f.FieldMaps[i]

			// Extract named captures
			for idx, fieldName := range fieldMap {
				if idx < len(matches) {
					value := matches[idx]

					// Skip empty captures if configured
					if !f.KeepEmptyCaptures && value == "" {
						continue
					}

					extractedFields[fieldName] = value
				}
			}

			// Also extract regex named groups
			for i, name := range compiledPattern.SubexpNames() {
				if i > 0 && name != "" && i < len(matches) {
					value := matches[i]

					// Skip empty captures if configured
					if !f.KeepEmptyCaptures && value == "" {
						continue
					}

					extractedFields[name] = value
				}
			}

			// Break if configured to stop on first match
			if f.BreakOnMatch {
				break
			}
		}
	}

	// If no match found, return original record
	if !matched {
		log.Debugf("Grok filter: no pattern matched for field '%s'", f.SourceField)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Apply extracted fields to record
	if f.TargetField != "" {
		// Store all extracted fields in a nested object
		if f.OverwriteKeys {
			record[f.TargetField] = extractedFields
		} else {
			// Merge with existing target field if it exists
			if existing, exists := record[f.TargetField]; exists {
				if existingMap, ok := existing.(map[string]interface{}); ok {
					for k, v := range extractedFields {
						if _, alreadyExists := existingMap[k]; !alreadyExists {
							existingMap[k] = v
						}
					}
					record[f.TargetField] = existingMap
				} else {
					record[f.TargetField] = extractedFields
				}
			} else {
				record[f.TargetField] = extractedFields
			}
		}
	} else {
		// Add fields directly to record
		for fieldName, value := range extractedFields {
			if f.OverwriteKeys {
				record[fieldName] = value
			} else {
				// Only add if field doesn't already exist
				if _, exists := record[fieldName]; !exists {
					record[fieldName] = value
				}
			}
		}
	}

	log.Debugf("Grok filter: extracted %d fields from '%s'", len(extractedFields), f.SourceField)

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}
