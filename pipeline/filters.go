package pipeline

import (
	"github.com/bytedance/sonic"
	"fmt"
	"regexp"
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
	field    string
	operator string
	value    interface{}
	action   string // "keep" or "drop"
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

// JSONValidateFilter validates JSON content and fails if invalid
type JSONValidateFilter struct {
	sourceField   string
	failOnInvalid bool
}

// NewJSONValidateFilter creates a new JSON validation filter
func NewJSONValidateFilter(config map[string]interface{}) (Filter, error) {
	sourceField, ok := config["source_field"].(string)
	if !ok || sourceField == "" {
		sourceField = "message" // Default field
	}

	failOnInvalid, ok := config["fail_on_invalid"].(bool)
	if !ok {
		failOnInvalid = true // Default to failing on invalid JSON
	}

	return &JSONValidateFilter{
		sourceField:   sourceField,
		failOnInvalid: failOnInvalid,
	}, nil
}

// Type returns the filter type
func (f *JSONValidateFilter) Type() string {
	return "json_validate"
}

// Validate validates the filter configuration
func (f *JSONValidateFilter) Validate(config map[string]interface{}) error {
	return nil // No required parameters
}

// Apply applies the filter to a record
func (f *JSONValidateFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Get the source field value
	sourceValue, exists := record[f.sourceField]
	if !exists {
		if f.failOnInvalid {
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
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Convert to string if needed
	var jsonStr string
	if str, ok := sourceValue.(string); ok {
		jsonStr = str
	} else {
		jsonStr = fmt.Sprintf("%v", sourceValue)
	}

	// Validate JSON by attempting to parse it
	var parsedJSON interface{}
	if err := sonic.Unmarshal([]byte(jsonStr), &parsedJSON); err != nil {
		if f.failOnInvalid {
			// Skip this record (fail processing)
			return &FilterResult{
				Record:   record,
				Skip:     true,
				Applied:  true,
				Duration: time.Since(start),
			}, nil
		}
		// Continue processing but mark as not applied
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// JSON is valid, continue processing
	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// JSONFlattenFilter flattens nested JSON objects
type JSONFlattenFilter struct {
	sourceField string
	targetField string
	separator   string
}

// NewJSONFlattenFilter creates a new JSON flatten filter
func NewJSONFlattenFilter(config map[string]interface{}) (Filter, error) {
	sourceField, ok := config["source_field"].(string)
	if !ok || sourceField == "" {
		sourceField = "message" // Default field
	}

	targetField, ok := config["target_field"].(string)
	if !ok || targetField == "" {
		targetField = "@flatten" // Default target field
	}

	separator, ok := config["separator"].(string)
	if !ok || separator == "" {
		separator = "." // Default separator
	}

	return &JSONFlattenFilter{
		sourceField: sourceField,
		targetField: targetField,
		separator:   separator,
	}, nil
}

// Type returns the filter type
func (f *JSONFlattenFilter) Type() string {
	return "json_flatten"
}

// Validate validates the filter configuration
func (f *JSONFlattenFilter) Validate(config map[string]interface{}) error {
	return nil // No required parameters
}

// Apply applies the filter to a record
func (f *JSONFlattenFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Get the source field value
	sourceValue, exists := record[f.sourceField]
	if !exists {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Convert to JSON object if it's a string
	var jsonObj interface{}
	if str, ok := sourceValue.(string); ok {
		if err := sonic.Unmarshal([]byte(str), &jsonObj); err != nil {
			// Not valid JSON, skip flattening
			return &FilterResult{
				Record:   record,
				Skip:     false,
				Applied:  false,
				Duration: time.Since(start),
			}, nil
		}
	} else {
		jsonObj = sourceValue
	}

	// Flatten the object
	flattened := f.flattenObject(jsonObj, "")
	record[f.targetField] = flattened

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// flattenObject recursively flattens a nested object
func (f *JSONFlattenFilter) flattenObject(obj interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})

	switch v := obj.(type) {
	case map[string]interface{}:
		for key, value := range v {
			newKey := key
			if prefix != "" {
				newKey = prefix + f.separator + key
			}

			if nestedMap, ok := value.(map[string]interface{}); ok {
				// Recursively flatten nested objects
				for flatKey, flatValue := range f.flattenObject(nestedMap, newKey) {
					result[flatKey] = flatValue
				}
			} else if nestedArray, ok := value.([]interface{}); ok {
				// Handle arrays by indexing
				for i, item := range nestedArray {
					indexKey := fmt.Sprintf("%s%s%d", newKey, f.separator, i)
					if nestedMap, ok := item.(map[string]interface{}); ok {
						for flatKey, flatValue := range f.flattenObject(nestedMap, indexKey) {
							result[flatKey] = flatValue
						}
					} else {
						result[indexKey] = item
					}
				}
			} else {
				result[newKey] = value
			}
		}
	default:
		// If it's not an object, just return it as is
		if prefix != "" {
			result[prefix] = v
		} else {
			result["value"] = v
		}
	}

	return result
}

// UppercaseKeysFilter converts all keys to uppercase
type UppercaseKeysFilter struct {
	sourceField string
	recursive   bool
}

// NewUppercaseKeysFilter creates a new uppercase keys filter
func NewUppercaseKeysFilter(config map[string]interface{}) (Filter, error) {
	sourceField, _ := config["source_field"].(string)
	// If source_field is not specified or empty, operate on entire record (sourceField = "")

	recursive, ok := config["recursive"].(bool)
	if !ok {
		recursive = true // Default to recursive
	}

	return &UppercaseKeysFilter{
		sourceField: sourceField,
		recursive:   recursive,
	}, nil
}

// Type returns the filter type
func (f *UppercaseKeysFilter) Type() string {
	return "uppercase_keys"
}

// Validate validates the filter configuration
func (f *UppercaseKeysFilter) Validate(config map[string]interface{}) error {
	return nil // No required parameters
}

// Apply applies the filter to a record
func (f *UppercaseKeysFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// If no source field specified, operate on entire record
	if f.sourceField == "" {
		record = f.uppercaseKeys(record)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  true,
			Duration: time.Since(start),
		}, nil
	}

	// Get the source field value
	sourceValue, exists := record[f.sourceField]
	if !exists {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Convert keys to uppercase
	if sourceMap, ok := sourceValue.(map[string]interface{}); ok {
		updatedValue := f.uppercaseKeys(sourceMap)
		record[f.sourceField] = updatedValue
	}

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// uppercaseKeys recursively converts all keys to uppercase
func (f *UppercaseKeysFilter) uppercaseKeys(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range obj {
		upperKey := strings.ToUpper(key)

		if f.recursive {
			switch v := value.(type) {
			case map[string]interface{}:
				result[upperKey] = f.uppercaseKeys(v)
			case []interface{}:
				result[upperKey] = f.uppercaseArrayKeys(v)
			default:
				result[upperKey] = value
			}
		} else {
			result[upperKey] = value
		}
	}

	return result
}

// uppercaseArrayKeys handles arrays that might contain objects
func (f *UppercaseKeysFilter) uppercaseArrayKeys(arr []interface{}) []interface{} {
	result := make([]interface{}, len(arr))

	for i, item := range arr {
		if itemMap, ok := item.(map[string]interface{}); ok {
			result[i] = f.uppercaseKeys(itemMap)
		} else {
			result[i] = item
		}
	}

	return result
}

// RegexReplaceFilter performs regex-based find and replace on field values
type RegexReplaceFilter struct {
	sourceField string
	targetField string
	pattern     *regexp.Regexp
	replacement string
	global      bool
}

// NewRegexReplaceFilter creates a regex replace filter
func NewRegexReplaceFilter(config map[string]interface{}) (Filter, error) {
	sourceField, ok := config["source_field"].(string)
	if !ok || sourceField == "" {
		sourceField = "message" // Default field
	}

	targetField, ok := config["target_field"].(string)
	if !ok || targetField == "" {
		targetField = sourceField // Default to overwriting source field
	}

	patternStr, ok := config["pattern"].(string)
	if !ok || patternStr == "" {
		return nil, fmt.Errorf("regex_replace filter requires 'pattern' parameter")
	}

	replacement, ok := config["replacement"].(string)
	if !ok {
		replacement = "" // Default to empty string
	}

	global := true
	if globalVal, ok := config["global"].(bool); ok {
		global = globalVal
	}

	// Compile the regex pattern
	pattern, err := regexp.Compile(patternStr)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern '%s': %w", patternStr, err)
	}

	return &RegexReplaceFilter{
		sourceField: sourceField,
		targetField: targetField,
		pattern:     pattern,
		replacement: replacement,
		global:      global,
	}, nil
}

// Type returns the filter type
func (f *RegexReplaceFilter) Type() string {
	return "regex_replace"
}

// Validate validates the filter configuration
func (f *RegexReplaceFilter) Validate(config map[string]interface{}) error {
	if _, ok := config["pattern"].(string); !ok {
		return fmt.Errorf("'pattern' parameter is required and must be a string")
	}
	return nil
}

// Apply applies the filter to a record
func (f *RegexReplaceFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Get source field value
	sourceValue, exists := record[f.sourceField]
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
		sourceStr = fmt.Sprintf("%v", sourceValue)
	}

	// Perform replacement
	var result string
	if f.global {
		// Replace all occurrences
		result = f.pattern.ReplaceAllString(sourceStr, f.replacement)
	} else {
		// Replace only first occurrence
		loc := f.pattern.FindStringIndex(sourceStr)
		if loc != nil {
			result = sourceStr[:loc[0]] + f.replacement + sourceStr[loc[1]:]
		} else {
			result = sourceStr
		}
	}

	// Set the target field
	record[f.targetField] = result

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// NewJSONParseFilter creates a JSON parse filter (not implemented - use NDJSON format instead)
func NewJSONParseFilter(config map[string]interface{}) (Filter, error) {
	return nil, fmt.Errorf("json_parse filter not implemented - bytefreezer-piper only supports NDJSON format")
}

// NewDateParseFilter creates a date parse filter
func NewDateParseFilter(config map[string]interface{}) (Filter, error) {
	return NewDateParseFilterComplete(config)
}

// NewGeoIPFilter creates a GeoIP filter
func NewGeoIPFilter(config map[string]interface{}) (Filter, error) {
	return NewGeoIPFilterComplete(config)
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
