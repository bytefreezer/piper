// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// ConditionalFieldFilter removes or replaces field values based on conditions
type ConditionalFieldFilter struct {
	Field       string      // Field to check
	Operator    string      // Comparison operator: equals, not_equals, contains, starts_with, ends_with, matches, exists, not_exists
	Value       interface{} // Value to compare against (for comparison operators)
	Action      string      // Action to take: remove, replace, set_null, drop_record, keep_record
	ReplaceWith interface{} // Replacement value (for replace action)
	regex       *regexp.Regexp
}

// NewConditionalFieldFilter creates a new conditional field filter
func NewConditionalFieldFilter(config map[string]interface{}) (Filter, error) {
	filter := &ConditionalFieldFilter{
		Operator: "equals",
		Action:   "remove",
	}

	// Parse field (required)
	if field, ok := config["field"].(string); ok && field != "" {
		filter.Field = field
	} else {
		return nil, fmt.Errorf("conditional_field: 'field' is required")
	}

	// Parse operator
	if operator, ok := config["operator"].(string); ok {
		validOperators := map[string]bool{
			"equals":      true,
			"not_equals":  true,
			"contains":    true,
			"starts_with": true,
			"ends_with":   true,
			"matches":     true,
			"exists":      true,
			"not_exists":  true,
			"gt":          true,
			"gte":         true,
			"lt":          true,
			"lte":         true,
		}
		if !validOperators[operator] {
			return nil, fmt.Errorf("conditional_field: invalid operator '%s'", operator)
		}
		filter.Operator = operator
	}

	// Parse value (required for most operators)
	if value, ok := config["value"]; ok {
		filter.Value = value
	} else if filter.Operator != "exists" && filter.Operator != "not_exists" {
		return nil, fmt.Errorf("conditional_field: 'value' is required for operator '%s'", filter.Operator)
	}

	// Compile regex if using matches operator
	if filter.Operator == "matches" {
		if pattern, ok := filter.Value.(string); ok {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("conditional_field: invalid regex pattern '%s': %w", pattern, err)
			}
			filter.regex = compiled
		} else {
			return nil, fmt.Errorf("conditional_field: 'value' must be a string for 'matches' operator")
		}
	}

	// Parse action
	if action, ok := config["action"].(string); ok {
		validActions := map[string]bool{
			"remove":      true,
			"replace":     true,
			"set_null":    true,
			"drop_record": true,
			"keep_record": true,
		}
		if !validActions[action] {
			return nil, fmt.Errorf("conditional_field: invalid action '%s'", action)
		}
		filter.Action = action
	}

	// Parse replace_with (required for replace action)
	if replaceWith, ok := config["replace_with"]; ok {
		filter.ReplaceWith = replaceWith
	} else if filter.Action == "replace" {
		return nil, fmt.Errorf("conditional_field: 'replace_with' is required for 'replace' action")
	}

	return filter, nil
}

// Type returns the filter type
func (f *ConditionalFieldFilter) Type() string {
	return "conditional_field"
}

// Validate validates the filter configuration
func (f *ConditionalFieldFilter) Validate(config map[string]interface{}) error {
	return nil
}

// Apply applies the conditional field filter to a record
func (f *ConditionalFieldFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Check if field exists
	fieldValue, exists := record[f.Field]

	// Handle exists/not_exists operators
	if f.Operator == "exists" {
		if !exists {
			return &FilterResult{
				Record:   record,
				Skip:     false,
				Applied:  false,
				Duration: time.Since(start),
			}, nil
		}
	} else if f.Operator == "not_exists" {
		if exists {
			return &FilterResult{
				Record:   record,
				Skip:     false,
				Applied:  false,
				Duration: time.Since(start),
			}, nil
		}
		// Field doesn't exist, condition met - but nothing to do
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// For other operators, field must exist
	if !exists {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Check condition
	conditionMet := f.checkCondition(fieldValue)

	// Handle keep_record action - condition NOT met means drop
	if f.Action == "keep_record" {
		if conditionMet {
			// Condition met, keep the record
			return &FilterResult{
				Record:   record,
				Skip:     false,
				Applied:  true,
				Duration: time.Since(start),
			}, nil
		}
		// Condition not met, drop the record
		log.Debugf("conditional_field: dropping record (field '%s' %s %v not met)", f.Field, f.Operator, f.Value)
		return &FilterResult{
			Record:   record,
			Skip:     true,
			Applied:  true,
			Duration: time.Since(start),
		}, nil
	}

	if !conditionMet {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Condition met, apply action
	switch f.Action {
	case "remove":
		delete(record, f.Field)
		log.Debugf("conditional_field: removed field '%s' (condition: %s %v)", f.Field, f.Operator, f.Value)

	case "replace":
		record[f.Field] = f.ReplaceWith
		log.Debugf("conditional_field: replaced field '%s' with '%v' (condition: %s %v)", f.Field, f.ReplaceWith, f.Operator, f.Value)

	case "set_null":
		record[f.Field] = nil
		log.Debugf("conditional_field: set field '%s' to null (condition: %s %v)", f.Field, f.Operator, f.Value)

	case "drop_record":
		log.Debugf("conditional_field: dropping record (field '%s' %s %v)", f.Field, f.Operator, f.Value)
		return &FilterResult{
			Record:   record,
			Skip:     true,
			Applied:  true,
			Duration: time.Since(start),
		}, nil
	}

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// checkCondition checks if the field value meets the condition
func (f *ConditionalFieldFilter) checkCondition(fieldValue interface{}) bool {
	// Convert field value to string for string comparisons
	fieldStr := fmt.Sprintf("%v", fieldValue)
	valueStr := fmt.Sprintf("%v", f.Value)

	switch f.Operator {
	case "equals":
		// Try exact type match first
		if fieldValue == f.Value {
			return true
		}
		// Fall back to string comparison
		return fieldStr == valueStr

	case "not_equals":
		if fieldValue == f.Value {
			return false
		}
		return fieldStr != valueStr

	case "contains":
		return strings.Contains(fieldStr, valueStr)

	case "starts_with":
		return strings.HasPrefix(fieldStr, valueStr)

	case "ends_with":
		return strings.HasSuffix(fieldStr, valueStr)

	case "matches":
		if f.regex != nil {
			return f.regex.MatchString(fieldStr)
		}
		return false

	case "gt", "gte", "lt", "lte":
		return f.compareNumeric(fieldValue)

	default:
		return false
	}
}

// compareNumeric compares numeric values
func (f *ConditionalFieldFilter) compareNumeric(fieldValue interface{}) bool {
	var fieldNum, valueNum float64

	// Convert field value to float64
	switch v := fieldValue.(type) {
	case int:
		fieldNum = float64(v)
	case int64:
		fieldNum = float64(v)
	case float64:
		fieldNum = v
	case float32:
		fieldNum = float64(v)
	default:
		return false
	}

	// Convert comparison value to float64
	switch v := f.Value.(type) {
	case int:
		valueNum = float64(v)
	case int64:
		valueNum = float64(v)
	case float64:
		valueNum = v
	case float32:
		valueNum = float64(v)
	default:
		return false
	}

	switch f.Operator {
	case "gt":
		return fieldNum > valueNum
	case "gte":
		return fieldNum >= valueNum
	case "lt":
		return fieldNum < valueNum
	case "lte":
		return fieldNum <= valueNum
	default:
		return false
	}
}
