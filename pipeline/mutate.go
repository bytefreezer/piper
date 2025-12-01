package pipeline

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// MutateFilter performs advanced field manipulation operations
type MutateFilter struct {
	// Split operations
	SplitField     string
	SplitSeparator string
	SplitTarget    string

	// Join operations
	JoinField     string
	JoinSeparator string
	JoinTarget    string

	// Gsub operations (regex find and replace)
	GsubOperations []GsubOperation

	// Convert operations (type conversion)
	ConvertOperations map[string]string

	// Lowercase operations
	LowercaseFields []string

	// Uppercase operations
	UppercaseFields []string

	// Strip operations (trim whitespace)
	StripFields []string

	// Merge operations
	MergeOperations []MergeOperation

	// Update operations
	UpdateOperations map[string]interface{}

	// Replace operations
	ReplaceOperations map[string]interface{}

	// Rename operations
	RenameOperations map[string]string

	// Remove operations
	RemoveFields []string

	// Copy operations
	CopyOperations map[string]string
}

// GsubOperation represents a regex substitution
type GsubOperation struct {
	Field       string
	Pattern     string
	Replacement string
	Compiled    *regexp.Regexp
}

// MergeOperation represents a field merge operation
type MergeOperation struct {
	Source string
	Target string
}

// NewMutateFilter creates a new mutate filter
func NewMutateFilter(config map[string]interface{}) (Filter, error) {
	filter := &MutateFilter{
		GsubOperations:    make([]GsubOperation, 0),
		ConvertOperations: make(map[string]string),
		LowercaseFields:   make([]string, 0),
		UppercaseFields:   make([]string, 0),
		StripFields:       make([]string, 0),
		MergeOperations:   make([]MergeOperation, 0),
		UpdateOperations:  make(map[string]interface{}),
		ReplaceOperations: make(map[string]interface{}),
		RenameOperations:  make(map[string]string),
		RemoveFields:      make([]string, 0),
		CopyOperations:    make(map[string]string),
	}

	// Parse split operation
	if splitConfig, ok := config["split"].(map[string]interface{}); ok {
		if field, ok := splitConfig["field"].(string); ok {
			filter.SplitField = field
		}
		if separator, ok := splitConfig["separator"].(string); ok {
			filter.SplitSeparator = separator
		}
		if target, ok := splitConfig["target"].(string); ok {
			filter.SplitTarget = target
		}
	}

	// Parse join operation
	if joinConfig, ok := config["join"].(map[string]interface{}); ok {
		if field, ok := joinConfig["field"].(string); ok {
			filter.JoinField = field
		}
		if separator, ok := joinConfig["separator"].(string); ok {
			filter.JoinSeparator = separator
		}
		if target, ok := joinConfig["target"].(string); ok {
			filter.JoinTarget = target
		}
	}

	// Parse gsub operations
	if gsubList, ok := config["gsub"].([]interface{}); ok {
		for _, item := range gsubList {
			if gsubMap, ok := item.(map[string]interface{}); ok {
				field, _ := gsubMap["field"].(string)
				pattern, _ := gsubMap["pattern"].(string)
				replacement, _ := gsubMap["replacement"].(string)

				if field != "" && pattern != "" {
					compiled, err := regexp.Compile(pattern)
					if err != nil {
						return nil, fmt.Errorf("failed to compile gsub pattern '%s': %w", pattern, err)
					}

					filter.GsubOperations = append(filter.GsubOperations, GsubOperation{
						Field:       field,
						Pattern:     pattern,
						Replacement: replacement,
						Compiled:    compiled,
					})
				}
			}
		}
	}

	// Parse convert operations
	if convertMap, ok := config["convert"].(map[string]interface{}); ok {
		for field, typeVal := range convertMap {
			if typeStr, ok := typeVal.(string); ok {
				filter.ConvertOperations[field] = typeStr
			}
		}
	}

	// Parse lowercase fields
	if lowercaseList, ok := config["lowercase"].([]interface{}); ok {
		for _, item := range lowercaseList {
			if field, ok := item.(string); ok {
				filter.LowercaseFields = append(filter.LowercaseFields, field)
			}
		}
	}

	// Parse uppercase fields
	if uppercaseList, ok := config["uppercase"].([]interface{}); ok {
		for _, item := range uppercaseList {
			if field, ok := item.(string); ok {
				filter.UppercaseFields = append(filter.UppercaseFields, field)
			}
		}
	}

	// Parse strip fields
	if stripList, ok := config["strip"].([]interface{}); ok {
		for _, item := range stripList {
			if field, ok := item.(string); ok {
				filter.StripFields = append(filter.StripFields, field)
			}
		}
	}

	// Parse merge operations
	if mergeList, ok := config["merge"].([]interface{}); ok {
		for _, item := range mergeList {
			if mergeMap, ok := item.(map[string]interface{}); ok {
				source, _ := mergeMap["source"].(string)
				target, _ := mergeMap["target"].(string)

				if source != "" && target != "" {
					filter.MergeOperations = append(filter.MergeOperations, MergeOperation{
						Source: source,
						Target: target,
					})
				}
			}
		}
	}

	// Parse update operations
	if updateMap, ok := config["update"].(map[string]interface{}); ok {
		filter.UpdateOperations = updateMap
	}

	// Parse replace operations
	if replaceMap, ok := config["replace"].(map[string]interface{}); ok {
		filter.ReplaceOperations = replaceMap
	}

	// Parse rename operations
	if renameMap, ok := config["rename"].(map[string]interface{}); ok {
		for oldName, newNameVal := range renameMap {
			if newName, ok := newNameVal.(string); ok {
				filter.RenameOperations[oldName] = newName
			}
		}
	}

	// Parse remove fields
	if removeList, ok := config["remove"].([]interface{}); ok {
		for _, item := range removeList {
			if field, ok := item.(string); ok {
				filter.RemoveFields = append(filter.RemoveFields, field)
			}
		}
	}

	// Parse copy operations
	if copyMap, ok := config["copy"].(map[string]interface{}); ok {
		for source, targetVal := range copyMap {
			if target, ok := targetVal.(string); ok {
				filter.CopyOperations[source] = target
			}
		}
	}

	return filter, nil
}

// Type returns the filter type
func (f *MutateFilter) Type() string {
	return "mutate"
}

// Validate validates the filter configuration
func (f *MutateFilter) Validate(config map[string]interface{}) error {
	// Mutate filter can have any combination of operations, so no strict validation needed
	return nil
}

// Apply applies the mutate filter to a record
func (f *MutateFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()
	applied := false

	// 1. Copy operations
	for source, target := range f.CopyOperations {
		if value, exists := record[source]; exists {
			record[target] = value
			applied = true
		}
	}

	// 2. Rename operations
	for oldName, newName := range f.RenameOperations {
		if value, exists := record[oldName]; exists {
			record[newName] = value
			delete(record, oldName)
			applied = true
		}
	}

	// 3. Convert operations (type conversion)
	for field, targetType := range f.ConvertOperations {
		if value, exists := record[field]; exists {
			converted, err := f.convertValue(value, targetType)
			if err == nil {
				record[field] = converted
				applied = true
			} else {
				log.Debugf("Mutate filter: failed to convert field '%s' to %s: %v", field, targetType, err)
			}
		}
	}

	// 4. Split operations
	if f.SplitField != "" && f.SplitSeparator != "" {
		if value, exists := record[f.SplitField]; exists {
			if strValue, ok := value.(string); ok {
				parts := strings.Split(strValue, f.SplitSeparator)
				targetField := f.SplitTarget
				if targetField == "" {
					targetField = f.SplitField
				}
				record[targetField] = parts
				applied = true
			}
		}
	}

	// 5. Join operations
	if f.JoinField != "" && f.JoinSeparator != "" {
		if value, exists := record[f.JoinField]; exists {
			if arrayValue, ok := value.([]interface{}); ok {
				strParts := make([]string, 0, len(arrayValue))
				for _, part := range arrayValue {
					strParts = append(strParts, fmt.Sprintf("%v", part))
				}
				joined := strings.Join(strParts, f.JoinSeparator)
				targetField := f.JoinTarget
				if targetField == "" {
					targetField = f.JoinField
				}
				record[targetField] = joined
				applied = true
			} else if arrayStr, ok := value.([]string); ok {
				joined := strings.Join(arrayStr, f.JoinSeparator)
				targetField := f.JoinTarget
				if targetField == "" {
					targetField = f.JoinField
				}
				record[targetField] = joined
				applied = true
			}
		}
	}

	// 6. Gsub operations (regex replace)
	for _, gsub := range f.GsubOperations {
		if value, exists := record[gsub.Field]; exists {
			if strValue, ok := value.(string); ok {
				replaced := gsub.Compiled.ReplaceAllString(strValue, gsub.Replacement)
				if replaced != strValue {
					record[gsub.Field] = replaced
					applied = true
				}
			}
		}
	}

	// 7. Lowercase operations
	for _, field := range f.LowercaseFields {
		if value, exists := record[field]; exists {
			if strValue, ok := value.(string); ok {
				record[field] = strings.ToLower(strValue)
				applied = true
			}
		}
	}

	// 8. Uppercase operations
	for _, field := range f.UppercaseFields {
		if value, exists := record[field]; exists {
			if strValue, ok := value.(string); ok {
				record[field] = strings.ToUpper(strValue)
				applied = true
			}
		}
	}

	// 9. Strip operations
	for _, field := range f.StripFields {
		if value, exists := record[field]; exists {
			if strValue, ok := value.(string); ok {
				record[field] = strings.TrimSpace(strValue)
				applied = true
			}
		}
	}

	// 10. Merge operations
	for _, merge := range f.MergeOperations {
		sourceValue, sourceExists := record[merge.Source]
		targetValue, targetExists := record[merge.Target]

		if sourceExists {
			if !targetExists {
				record[merge.Target] = sourceValue
				applied = true
			} else {
				// Merge based on type
				merged := f.mergeValues(targetValue, sourceValue)
				if merged != nil {
					record[merge.Target] = merged
					applied = true
				}
			}
		}
	}

	// 11. Update operations (only if field exists)
	for field, value := range f.UpdateOperations {
		if _, exists := record[field]; exists {
			record[field] = value
			applied = true
		}
	}

	// 12. Replace operations (always set, even if field doesn't exist)
	for field, value := range f.ReplaceOperations {
		record[field] = value
		applied = true
	}

	// 13. Remove operations (last, so other operations can work on fields before removal)
	for _, field := range f.RemoveFields {
		if _, exists := record[field]; exists {
			delete(record, field)
			applied = true
		}
	}

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  applied,
		Duration: time.Since(start),
	}, nil
}

// convertValue converts a value to the target type
func (f *MutateFilter) convertValue(value interface{}, targetType string) (interface{}, error) {
	switch targetType {
	case "string":
		return fmt.Sprintf("%v", value), nil

	case "integer", "int":
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			return strconv.Atoi(v)
		default:
			return 0, fmt.Errorf("cannot convert %T to integer", value)
		}

	case "float":
		switch v := value.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case string:
			return strconv.ParseFloat(v, 64)
		default:
			return 0.0, fmt.Errorf("cannot convert %T to float", value)
		}

	case "boolean", "bool":
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return strconv.ParseBool(v)
		case int:
			return v != 0, nil
		case float64:
			return v != 0, nil
		default:
			return false, fmt.Errorf("cannot convert %T to boolean", value)
		}

	default:
		return nil, fmt.Errorf("unknown target type: %s", targetType)
	}
}

// mergeValues merges two values intelligently based on their types
func (f *MutateFilter) mergeValues(target, source interface{}) interface{} {
	// If both are arrays, concatenate
	if targetArray, ok := target.([]interface{}); ok {
		if sourceArray, ok := source.([]interface{}); ok {
			return append(targetArray, sourceArray...)
		}
		// Source is not array, append as single element
		return append(targetArray, source)
	}

	// If both are maps, merge keys
	if targetMap, ok := target.(map[string]interface{}); ok {
		if sourceMap, ok := source.(map[string]interface{}); ok {
			merged := make(map[string]interface{})
			for k, v := range targetMap {
				merged[k] = v
			}
			for k, v := range sourceMap {
				merged[k] = v
			}
			return merged
		}
	}

	// If both are strings, concatenate
	if targetStr, ok := target.(string); ok {
		if sourceStr, ok := source.(string); ok {
			return targetStr + sourceStr
		}
	}

	// Default: return source (replace)
	return source
}
