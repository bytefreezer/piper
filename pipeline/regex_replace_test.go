package pipeline

import (
	"testing"
	"time"
)

func TestRegexReplaceFilter_Basic(t *testing.T) {
	// Test basic regex replacement
	config := map[string]interface{}{
		"source_field": "message",
		"pattern":      "[0-9]+",
		"replacement":  "X",
		"global":       true,
	}

	filter, err := NewRegexReplaceFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	record := map[string]interface{}{
		"message": "Error code 404 at line 123",
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Applied {
		t.Error("Filter should have been applied")
	}

	if result.Skip {
		t.Error("Filter should not skip the record")
	}

	expected := "Error code X at line X"
	if record["message"] != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, record["message"])
	}
}

func TestRegexReplaceFilter_FirstOnly(t *testing.T) {
	// Test replacing only the first match
	config := map[string]interface{}{
		"source_field": "message",
		"pattern":      "[0-9]+",
		"replacement":  "X",
		"global":       false,
	}

	filter, err := NewRegexReplaceFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	record := map[string]interface{}{
		"message": "Error code 404 at line 123",
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Applied {
		t.Error("Filter should have been applied")
	}

	expected := "Error code X at line 123"
	if record["message"] != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, record["message"])
	}
}

func TestRegexReplaceFilter_TargetField(t *testing.T) {
	// Test writing to a different target field
	config := map[string]interface{}{
		"source_field": "raw_message",
		"target_field": "clean_message",
		"pattern":      "[^a-zA-Z0-9 ]",
		"replacement":  "",
		"global":       true,
	}

	filter, err := NewRegexReplaceFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	record := map[string]interface{}{
		"raw_message": "Hello! @World#123$",
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Applied {
		t.Error("Filter should have been applied")
	}

	if result.Skip {
		t.Error("Filter should not skip the record")
	}

	// Original field should remain unchanged
	if record["raw_message"] != "Hello! @World#123$" {
		t.Error("Source field should not be modified")
	}

	// Target field should contain cleaned version
	expected := "Hello World123"
	if record["clean_message"] != expected {
		t.Errorf("Expected clean_message '%s', got '%s'", expected, record["clean_message"])
	}
}

func TestRegexReplaceFilter_MissingField(t *testing.T) {
	// Test behavior when source field is missing
	config := map[string]interface{}{
		"source_field": "message",
		"pattern":      "[0-9]+",
		"replacement":  "X",
	}

	filter, err := NewRegexReplaceFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	record := map[string]interface{}{
		"other_field": "some value",
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result.Applied {
		t.Error("Filter should not be applied when source field is missing")
	}
}

func TestRegexReplaceFilter_InvalidPattern(t *testing.T) {
	// Test that invalid regex pattern returns error
	config := map[string]interface{}{
		"source_field": "message",
		"pattern":      "[invalid(regex",
		"replacement":  "X",
	}

	_, err := NewRegexReplaceFilter(config)
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestRegexReplaceFilter_Validation(t *testing.T) {
	// Test configuration validation
	config := map[string]interface{}{
		"source_field": "message",
		"pattern":      "test",
	}

	filter, err := NewRegexReplaceFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	// Valid config
	if err := filter.Validate(config); err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Missing pattern
	invalidConfig := map[string]interface{}{
		"source_field": "message",
	}

	if err := filter.Validate(invalidConfig); err == nil {
		t.Error("Config without pattern should fail validation")
	}
}
