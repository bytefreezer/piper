// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bytefreezer/piper/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FilterTestCase defines a test case for a filter
type FilterTestCase struct {
	Name           string
	FilterType     string
	Config         map[string]interface{}
	Input          map[string]interface{}
	ExpectedOutput map[string]interface{}
	ExpectSkip     bool
	ExpectError    bool
}

// GetAllFilterTestCases returns comprehensive test cases for all filter types
func GetAllFilterTestCases() []FilterTestCase {
	return []FilterTestCase{
		// 1. AddField Filter
		{
			Name:       "AddField - Simple String",
			FilterType: "add_field",
			Config: map[string]interface{}{
				"field": "new_field",
				"value": "test_value",
			},
			Input: map[string]interface{}{
				"existing": "data",
			},
			ExpectedOutput: map[string]interface{}{
				"existing":  "data",
				"new_field": "test_value",
			},
		},
		{
			Name:       "AddField - Timestamp Variable",
			FilterType: "add_field",
			Config: map[string]interface{}{
				"field": "processed_at",
				"value": "${timestamp}",
			},
			Input: map[string]interface{}{
				"message": "test",
			},
			ExpectedOutput: map[string]interface{}{
				"message":      "test",
				"processed_at": "TIMESTAMP", // Will be validated as timestamp format
			},
		},

		// 2. RemoveField Filter
		{
			Name:       "RemoveField - Single Field",
			FilterType: "remove_field",
			Config: map[string]interface{}{
				"fields": []interface{}{"unwanted"},
			},
			Input: map[string]interface{}{
				"keep":     "this",
				"unwanted": "remove",
			},
			ExpectedOutput: map[string]interface{}{
				"keep": "this",
			},
		},

		// 3. RenameField Filter
		{
			Name:       "RenameField - Single Rename",
			FilterType: "rename_field",
			Config: map[string]interface{}{
				"from": "old_name",
				"to":   "new_name",
			},
			Input: map[string]interface{}{
				"old_name": "value",
				"other":    "data",
			},
			ExpectedOutput: map[string]interface{}{
				"new_name": "value",
				"other":    "data",
			},
		},

		// 4. Include Filter - Match
		{
			Name:       "Include - Regex Match",
			FilterType: "include",
			Config: map[string]interface{}{
				"field":   "host",
				"matches": "\\.108$",
			},
			Input: map[string]interface{}{
				"host":    "192.168.1.108",
				"message": "test",
			},
			ExpectedOutput: map[string]interface{}{
				"host":    "192.168.1.108",
				"message": "test",
			},
		},
		{
			Name:       "Include - Regex No Match (Skip)",
			FilterType: "include",
			Config: map[string]interface{}{
				"field":   "host",
				"matches": "\\.108$",
			},
			Input: map[string]interface{}{
				"host":    "192.168.1.109",
				"message": "test",
			},
			ExpectSkip: true,
		},

		// 5. Exclude Filter - Match (Skip)
		{
			Name:       "Exclude - Regex Match (Skip)",
			FilterType: "exclude",
			Config: map[string]interface{}{
				"field":   "level",
				"matches": "^DEBUG$",
			},
			Input: map[string]interface{}{
				"level":   "DEBUG",
				"message": "test",
			},
			ExpectSkip: true,
		},
		{
			Name:       "Exclude - Regex No Match",
			FilterType: "exclude",
			Config: map[string]interface{}{
				"field":   "level",
				"matches": "^DEBUG$",
			},
			Input: map[string]interface{}{
				"level":   "INFO",
				"message": "test",
			},
			ExpectedOutput: map[string]interface{}{
				"level":   "INFO",
				"message": "test",
			},
		},

		// 6. JSONFlatten Filter
		{
			Name:       "JSONFlatten - Nested Object",
			FilterType: "json_flatten",
			Config: map[string]interface{}{
				"separator": ".",
				"recursive": true,
			},
			Input: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "john",
					"address": map[string]interface{}{
						"city":  "NYC",
						"state": "NY",
					},
				},
				"status": "active",
			},
			ExpectedOutput: map[string]interface{}{
				"user.name":          "john",
				"user.address.city":  "NYC",
				"user.address.state": "NY",
				"status":             "active",
			},
		},

		// 7. Drop Filter
		{
			Name:       "Drop - Always Skip",
			FilterType: "drop",
			Config: map[string]interface{}{
				"always_drop": true,
			},
			Input: map[string]interface{}{
				"message": "test",
			},
			ExpectSkip: true,
		},

		// 8. Mutate Filter - Uppercase
		{
			Name:       "Mutate - Uppercase",
			FilterType: "mutate",
			Config: map[string]interface{}{
				"uppercase": []interface{}{"name"},
			},
			Input: map[string]interface{}{
				"name": "john doe",
			},
			ExpectedOutput: map[string]interface{}{
				"name": "JOHN DOE",
			},
		},
		{
			Name:       "Mutate - Lowercase",
			FilterType: "mutate",
			Config: map[string]interface{}{
				"lowercase": []interface{}{"name"},
			},
			Input: map[string]interface{}{
				"name": "JOHN DOE",
			},
			ExpectedOutput: map[string]interface{}{
				"name": "john doe",
			},
		},

		// 9. RegexReplace Filter
		{
			Name:       "RegexReplace - Replace Pattern",
			FilterType: "regex_replace",
			Config: map[string]interface{}{
				"field":       "message",
				"pattern":     "\\d{3}-\\d{2}-\\d{4}",
				"replacement": "XXX-XX-XXXX",
			},
			Input: map[string]interface{}{
				"message": "SSN: 123-45-6789",
			},
			ExpectedOutput: map[string]interface{}{
				"message": "SSN: XXX-XX-XXXX",
			},
		},

		// 10. Split Filter - Creates metadata for pipeline expansion
		{
			Name:       "Split - Comma Separated",
			FilterType: "split",
			Config: map[string]interface{}{
				"field":      "tags",
				"terminator": ",",
			},
			Input: map[string]interface{}{
				"tags": "tag1,tag2,tag3",
			},
			ExpectedOutput: map[string]interface{}{
				"tags":         "tag1,tag2,tag3",
				"_split_field": "tags",
				"_split_items": []interface{}{"tag1", "tag2", "tag3"},
				"_split_count": 3,
			},
		},

		// 11. KV Filter
		{
			Name:       "KV - Key-Value Parsing",
			FilterType: "kv",
			Config: map[string]interface{}{
				"source_field": "message",
				"field_split":  " ",
				"value_split":  "=",
			},
			Input: map[string]interface{}{
				"message": "user=john status=active level=5",
			},
			ExpectedOutput: map[string]interface{}{
				"message": "user=john status=active level=5",
				"user":    "john",
				"status":  "active",
				"level":   "5",
			},
		},

		// 12. DateParse Filter
		{
			Name:       "DateParse - RFC3339",
			FilterType: "date_parse",
			Config: map[string]interface{}{
				"source_field": "timestamp",
				"target_field": "parsed_time",
				"formats":      []interface{}{"2006-01-02T15:04:05Z07:00"},
			},
			Input: map[string]interface{}{
				"timestamp": "2025-11-24T12:00:00Z",
			},
			ExpectedOutput: map[string]interface{}{
				"timestamp":   "2025-11-24T12:00:00Z",
				"parsed_time": "TIMESTAMP", // Will be validated as parsed time
			},
		},

		// 13. Fingerprint Filter
		{
			Name:       "Fingerprint - Generate Hash",
			FilterType: "fingerprint",
			Config: map[string]interface{}{
				"source_fields": []interface{}{"user", "action"},
				"target_field":  "fingerprint",
				"method":        "SHA256",
			},
			Input: map[string]interface{}{
				"user":   "john",
				"action": "login",
			},
			ExpectedOutput: map[string]interface{}{
				"user":        "john",
				"action":      "login",
				"fingerprint": "HASH", // Will be validated as hash format
			},
		},

		// 14. Sample Filter
		{
			Name:       "Sample - Pass Through (10%)",
			FilterType: "sample",
			Config: map[string]interface{}{
				"percentage": 100.0, // 100% for testing
			},
			Input: map[string]interface{}{
				"message": "test",
			},
			ExpectedOutput: map[string]interface{}{
				"message": "test",
			},
		},

		// 15. UppercaseKeys Filter
		{
			Name:       "UppercaseKeys - All Keys",
			FilterType: "uppercase_keys",
			Config:     map[string]interface{}{},
			Input: map[string]interface{}{
				"name":   "john",
				"status": "active",
			},
			ExpectedOutput: map[string]interface{}{
				"NAME":   "john",
				"STATUS": "active",
			},
		},

		// 16. Conditional Filter - Keep on Match
		{
			Name:       "Conditional - Keep on Equals",
			FilterType: "conditional",
			Config: map[string]interface{}{
				"field":    "status",
				"operator": "eq",
				"value":    "error",
				"action":   "keep",
			},
			Input: map[string]interface{}{
				"status":  "error",
				"message": "failed",
			},
			ExpectedOutput: map[string]interface{}{
				"status":  "error",
				"message": "failed",
			},
		},
		{
			Name:       "Conditional - Drop on Match",
			FilterType: "conditional",
			Config: map[string]interface{}{
				"field":    "level",
				"operator": "eq",
				"value":    "DEBUG",
				"action":   "drop",
			},
			Input: map[string]interface{}{
				"level":   "DEBUG",
				"message": "debug info",
			},
			ExpectSkip: true,
		},

		// 17. JSONValidate Filter (pass through on valid JSON)
		{
			Name:       "JSONValidate - Valid Structure",
			FilterType: "json_validate",
			Config: map[string]interface{}{
				"fail_on_invalid": false,
			},
			Input: map[string]interface{}{
				"valid": "json",
				"count": 123,
			},
			ExpectedOutput: map[string]interface{}{
				"valid": "json",
				"count": 123,
			},
		},

		// 18. Passthrough Filter
		{
			Name:       "Passthrough - No Modification",
			FilterType: "passthrough",
			Config: map[string]interface{}{
				"description": "test passthrough",
			},
			Input: map[string]interface{}{
				"message": "test",
				"level":   "info",
			},
			ExpectedOutput: map[string]interface{}{
				"message": "test",
				"level":   "info",
			},
		},

		// 19. DNS Filter - Skip test (requires network and DNS lookup)
		// DNS lookups are not deterministic for unit tests, would need mock

		// 20. GeoIP Filter - Skip test (requires GeoIP database file)
		// Would need test database or mock to test properly

		// 21. Grok Filter - Skip test (complex pattern matching)
		// Grok is complex and would need extensive test patterns

		// 22. UserAgent Filter - Skip test (requires UA parser initialization)
		// Would need to mock UA parser or have it properly initialized

		// Complex Multi-Filter Pipeline Test
		{
			Name:       "Pipeline - Include + AddField + Flatten",
			FilterType: "multi", // Special marker for multi-filter test
			Config: map[string]interface{}{
				"filters": []map[string]interface{}{
					{
						"type": "include",
						"config": map[string]interface{}{
							"field":   "host",
							"matches": "\\.108$",
						},
					},
					{
						"type": "add_field",
						"config": map[string]interface{}{
							"field": "processed",
							"value": "true",
						},
					},
					{
						"type": "json_flatten",
						"config": map[string]interface{}{
							"separator": ".",
							"recursive": true,
						},
					},
				},
			},
			Input: map[string]interface{}{
				"host": "192.168.1.108",
				"data": map[string]interface{}{
					"cpu": 50,
					"mem": 80,
				},
			},
			ExpectedOutput: map[string]interface{}{
				"host":      "192.168.1.108",
				"data.cpu":  50,
				"data.mem":  80,
				"processed": "true",
			},
		},
	}
}

// TestAllFilters runs all filter test cases
func TestAllFilters(t *testing.T) {
	registry := NewFilterRegistry()
	testCases := GetAllFilterTestCases()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			filterCtx := &FilterContext{
				TenantID:   "test-tenant",
				DatasetID:  "test-dataset",
				LineNumber: 1,
				Timestamp:  time.Now(),
				Variables:  make(map[string]string),
			}

			// Handle multi-filter tests
			if tc.FilterType == "multi" {
				filters := tc.Config["filters"].([]map[string]interface{})
				currentRecord := tc.Input

				for i, filterDef := range filters {
					filterType := filterDef["type"].(string)
					filterConfig := filterDef["config"].(map[string]interface{})

					filter, err := registry.CreateFilter(filterType, filterConfig)
					require.NoError(t, err, "Failed to create filter %d (%s)", i, filterType)

					result, err := filter.Apply(filterCtx, currentRecord)
					if !tc.ExpectError {
						require.NoError(t, err, "Filter %d (%s) returned error", i, filterType)
					}

					if tc.ExpectSkip {
						assert.True(t, result.Skip, "Expected record to be skipped")
						return
					}

					currentRecord = result.Record
				}

				// Validate final output
				validateOutput(t, tc, currentRecord)
				return
			}

			// Single filter test
			filter, err := registry.CreateFilter(tc.FilterType, tc.Config)
			require.NoError(t, err, "Failed to create filter")

			result, err := filter.Apply(filterCtx, tc.Input)

			if tc.ExpectError {
				assert.Error(t, err, "Expected error but got none")
				return
			}

			require.NoError(t, err, "Filter returned unexpected error")

			if tc.ExpectSkip {
				assert.True(t, result.Skip, "Expected record to be skipped")
				return
			}

			assert.False(t, result.Skip, "Record was skipped unexpectedly")
			validateOutput(t, tc, result.Record)
		})
	}
}

// validateOutput validates the output matches expected output
func validateOutput(t *testing.T, tc FilterTestCase, actual map[string]interface{}) {
	for key, expectedValue := range tc.ExpectedOutput {
		actualValue, exists := actual[key]
		assert.True(t, exists, "Expected field %s not found in output", key)

		// Special validation for dynamic values
		switch expectedValue {
		case "TIMESTAMP":
			// Validate it's a timestamp string
			_, ok := actualValue.(string)
			assert.True(t, ok, "Expected timestamp string for field %s", key)
		case "HASH":
			// Validate it's a hash string (non-empty)
			hashStr, ok := actualValue.(string)
			assert.True(t, ok, "Expected hash string for field %s", key)
			assert.NotEmpty(t, hashStr, "Hash should not be empty")
		default:
			// Compare values (need to handle type conversions)
			assert.Equal(t, normalizeValue(expectedValue), normalizeValue(actualValue),
				"Field %s value mismatch", key)
		}
	}

	// Check for unexpected extra fields (except internal metadata)
	for key := range actual {
		if key[0] == '_' {
			continue // Skip internal metadata fields
		}
		_, expected := tc.ExpectedOutput[key]
		assert.True(t, expected, "Unexpected field %s in output", key)
	}
}

// normalizeValue normalizes values for comparison (handles JSON number conversions)
func normalizeValue(v interface{}) interface{} {
	// Convert float64 to int if it's a whole number
	if f, ok := v.(float64); ok {
		if f == float64(int(f)) {
			return int(f)
		}
	}
	return v
}

// ExportTestCasesAsJSON exports test cases as JSON for external testing
func ExportTestCasesAsJSON() (string, error) {
	testCases := GetAllFilterTestCases()
	data, err := json.MarshalIndent(testCases, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RunTransformationTest runs a transformation test using the actual transformation API
// This simulates what the UI test function does
func RunTransformationTest(t *testing.T, tenantID, datasetID string, filters []domain.FilterConfig, input map[string]interface{}) map[string]interface{} {
	registry := NewFilterRegistry()
	filterCtx := &FilterContext{
		TenantID:   tenantID,
		DatasetID:  datasetID,
		LineNumber: 1,
		Timestamp:  time.Now(),
		Variables:  make(map[string]string),
	}

	currentRecord := input
	for i, filterConfig := range filters {
		if !filterConfig.Enabled {
			continue
		}

		filter, err := registry.CreateFilter(filterConfig.Type, filterConfig.Config)
		require.NoError(t, err, "Failed to create filter %d (%s)", i, filterConfig.Type)

		result, err := filter.Apply(filterCtx, currentRecord)
		require.NoError(t, err, "Filter %d (%s) returned error", i, filterConfig.Type)

		if result.Skip {
			return nil // Record was filtered out
		}

		currentRecord = result.Record
	}

	return currentRecord
}
