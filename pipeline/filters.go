package pipeline

import (
	"fmt"
	"strings"
	"time"
)

// AddFieldFilter adds a field to the record
type AddFieldFilter struct {
	fieldName string
	value     string
}

// NewAddFieldFilter creates a new add field filter
func NewAddFieldFilter(config map[string]interface{}) (Filter, error) {
	fieldName, ok := config["field"].(string)
	if !ok || fieldName == "" {
		return nil, fmt.Errorf("add_field filter requires 'field' parameter")
	}

	value, ok := config["value"].(string)
	if !ok {
		return nil, fmt.Errorf("add_field filter requires 'value' parameter")
	}

	return &AddFieldFilter{
		fieldName: fieldName,
		value:     value,
	}, nil
}

// Type returns the filter type
func (f *AddFieldFilter) Type() string {
	return "add_field"
}

// Validate validates the filter configuration
func (f *AddFieldFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["field"].(string); !ok {
		return fmt.Errorf("'field' parameter is required and must be a string")
	}
	if _, ok := config["value"].(string); !ok {
		return fmt.Errorf("'value' parameter is required and must be a string")
	}
	return nil
}

// Apply applies the filter to a record
func (f *AddFieldFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Interpolate template variables in the value
	interpolatedValue := interpolateVariables(f.value, ctx)
	
	// Add the field to the record
	record[f.fieldName] = interpolatedValue

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// RemoveFieldFilter removes specified fields from the record
type RemoveFieldFilter struct {
	fields []string
}

// NewRemoveFieldFilter creates a new remove field filter
func NewRemoveFieldFilter(config map[string]interface{}) (Filter, error) {
	var fields []string

	// Handle single field
	if field, ok := config["field"].(string); ok {
		fields = []string{field}
	}

	// Handle multiple fields
	if fieldsInterface, ok := config["fields"].([]interface{}); ok {
		for _, fieldInterface := range fieldsInterface {
			if field, ok := fieldInterface.(string); ok {
				fields = append(fields, field)
			}
		}
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("remove_field filter requires 'field' or 'fields' parameter")
	}

	return &RemoveFieldFilter{
		fields: fields,
	}, nil
}

// Type returns the filter type
func (f *RemoveFieldFilter) Type() string {
	return "remove_field"
}

// Validate validates the filter configuration
func (f *RemoveFieldFilter) Validate(config map[string]interface{}) error {
	hasField := false
	if _, ok := config["field"].(string); ok {
		hasField = true
	}
	if fields, ok := config["fields"].([]interface{}); ok && len(fields) > 0 {
		hasField = true
	}
	if !hasField {
		return fmt.Errorf("'field' or 'fields' parameter is required")
	}
	return nil
}

// Apply applies the filter to a record
func (f *RemoveFieldFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Remove specified fields
	for _, field := range f.fields {
		delete(record, field)
	}

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// RenameFieldFilter renames a field in the record
type RenameFieldFilter struct {
	fromField string
	toField   string
}

// NewRenameFieldFilter creates a new rename field filter
func NewRenameFieldFilter(config map[string]interface{}) (Filter, error) {
	fromField, ok := config["from"].(string)
	if !ok || fromField == "" {
		return nil, fmt.Errorf("rename_field filter requires 'from' parameter")
	}

	toField, ok := config["to"].(string)
	if !ok || toField == "" {
		return nil, fmt.Errorf("rename_field filter requires 'to' parameter")
	}

	return &RenameFieldFilter{
		fromField: fromField,
		toField:   toField,
	}, nil
}

// Type returns the filter type
func (f *RenameFieldFilter) Type() string {
	return "rename_field"
}

// Validate validates the filter configuration
func (f *RenameFieldFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["from"].(string); !ok {
		return fmt.Errorf("'from' parameter is required and must be a string")
	}
	if _, ok := config["to"].(string); !ok {
		return fmt.Errorf("'to' parameter is required and must be a string")
	}
	return nil
}

// Apply applies the filter to a record
func (f *RenameFieldFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Check if source field exists
	if value, exists := record[f.fromField]; exists {
		// Add new field with the value
		record[f.toField] = value
		// Remove old field
		delete(record, f.fromField)
	}

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// ConditionalFilter filters records based on field values
type ConditionalFilter struct {
	field     string
	operator  string
	value     interface{}
	action    string // "keep" or "drop"
}

// NewConditionalFilter creates a new conditional filter
func NewConditionalFilter(config map[string]interface{}) (Filter, error) {
	field, ok := config["field"].(string)
	if !ok || field == "" {
		return nil, fmt.Errorf("conditional filter requires 'field' parameter")
	}

	operator, ok := config["operator"].(string)
	if !ok || operator == "" {
		return nil, fmt.Errorf("conditional filter requires 'operator' parameter")
	}

	value, exists := config["value"]
	if !exists {
		return nil, fmt.Errorf("conditional filter requires 'value' parameter")
	}

	action, ok := config["action"].(string)
	if !ok {
		action = "keep" // Default action
	}

	// Validate operator
	validOperators := []string{"eq", "ne", "gt", "lt", "gte", "lte", "contains", "not_contains", "exists", "not_exists"}
	isValidOperator := false
	for _, validOp := range validOperators {
		if operator == validOp {
			isValidOperator = true
			break
		}
	}
	if !isValidOperator {
		return nil, fmt.Errorf("invalid operator '%s', valid operators: %v", operator, validOperators)
	}

	return &ConditionalFilter{
		field:    field,
		operator: operator,
		value:    value,
		action:   action,
	}, nil
}

// Type returns the filter type
func (f *ConditionalFilter) Type() string {
	return "conditional"
}

// Validate validates the filter configuration
func (f *ConditionalFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["field"].(string); !ok {
		return fmt.Errorf("'field' parameter is required and must be a string")
	}
	if _, ok := config["operator"].(string); !ok {
		return fmt.Errorf("'operator' parameter is required and must be a string")
	}
	if _, exists := config["value"]; !exists {
		return fmt.Errorf("'value' parameter is required")
	}
	return nil
}

// Apply applies the filter to a record
func (f *ConditionalFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	fieldValue, exists := record[f.field]
	conditionMet := f.evaluateCondition(fieldValue, exists)

	skip := false
	if f.action == "keep" && !conditionMet {
		skip = true
	} else if f.action == "drop" && conditionMet {
		skip = true
	}

	return &FilterResult{
		Record:   record,
		Skip:     skip,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// evaluateCondition evaluates the condition based on operator
func (f *ConditionalFilter) evaluateCondition(fieldValue interface{}, exists bool) bool {
	switch f.operator {
	case "exists":
		return exists
	case "not_exists":
		return !exists
	case "eq":
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", f.value)
	case "ne":
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", f.value)
	case "contains":
		fieldStr := fmt.Sprintf("%v", fieldValue)
		valueStr := fmt.Sprintf("%v", f.value)
		return strings.Contains(fieldStr, valueStr)
	case "not_contains":
		fieldStr := fmt.Sprintf("%v", fieldValue)
		valueStr := fmt.Sprintf("%v", f.value)
		return !strings.Contains(fieldStr, valueStr)
	// Add more operators as needed
	default:
		return false
	}
}

// Placeholder implementations for other filters
// These would be implemented similar to the above patterns

// NewJSONParseFilter creates a JSON parse filter
func NewJSONParseFilter(config map[string]interface{}) (Filter, error) {
	// TODO: Implement JSON parsing filter
	return nil, fmt.Errorf("json_parse filter not yet implemented")
}

// NewRegexReplaceFilter creates a regex replace filter
func NewRegexReplaceFilter(config map[string]interface{}) (Filter, error) {
	// TODO: Implement regex replace filter
	return nil, fmt.Errorf("regex_replace filter not yet implemented")
}

// NewDateParseFilter creates a date parse filter
func NewDateParseFilter(config map[string]interface{}) (Filter, error) {
	// TODO: Implement date parsing filter
	return nil, fmt.Errorf("date_parse filter not yet implemented")
}

// NewGeoIPFilter creates a GeoIP filter
func NewGeoIPFilter(config map[string]interface{}) (Filter, error) {
	// TODO: Implement GeoIP filter
	return nil, fmt.Errorf("geoip filter not yet implemented")
}

// interpolateVariables replaces template variables in a string
func interpolateVariables(template string, ctx *FilterContext) string {
	result := template

	// Replace built-in variables
	if ctx != nil {
		if ctx.Variables != nil {
			for key, value := range ctx.Variables {
				placeholder := "${" + key + "}"
				result = strings.ReplaceAll(result, placeholder, value)
			}
		}

		// Replace standard variables
		result = strings.ReplaceAll(result, "${timestamp}", ctx.Timestamp.Format(time.RFC3339))
		result = strings.ReplaceAll(result, "${tenant_id}", ctx.TenantID)
		result = strings.ReplaceAll(result, "${dataset_id}", ctx.DatasetID)
		result = strings.ReplaceAll(result, "${line_number}", fmt.Sprintf("%d", ctx.LineNumber))
	}

	return result
}