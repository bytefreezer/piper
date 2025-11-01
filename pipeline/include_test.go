package pipeline

import (
	"testing"
	"time"
)

func TestIncludeFilter_Equals(t *testing.T) {
	config := map[string]interface{}{
		"field":  "status",
		"equals": "success",
	}

	filter, err := NewIncludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test matching record (should keep)
	record := map[string]interface{}{
		"status": "success",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result.Skip {
		t.Error("Filter should not skip matching record")
	}

	if !result.Applied {
		t.Error("Filter should be applied")
	}

	// Test non-matching record (should drop)
	record2 := map[string]interface{}{
		"status": "failure",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result2.Skip {
		t.Error("Filter should skip non-matching record")
	}
}

func TestIncludeFilter_Contains(t *testing.T) {
	config := map[string]interface{}{
		"field":    "message",
		"contains": "error",
	}

	filter, err := NewIncludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test matching record
	record := map[string]interface{}{
		"message": "This is an error message",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result.Skip {
		t.Error("Filter should not skip matching record")
	}

	// Test non-matching record
	record2 := map[string]interface{}{
		"message": "This is a success message",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result2.Skip {
		t.Error("Filter should skip non-matching record")
	}
}

func TestIncludeFilter_Matches(t *testing.T) {
	config := map[string]interface{}{
		"field":   "status",
		"matches": "^(success|ok)$",
	}

	filter, err := NewIncludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test matching record
	record := map[string]interface{}{
		"status": "success",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result.Skip {
		t.Error("Filter should not skip matching record")
	}

	// Test non-matching record
	record2 := map[string]interface{}{
		"status": "failure",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result2.Skip {
		t.Error("Filter should skip non-matching record")
	}
}

func TestIncludeFilter_AnyField(t *testing.T) {
	config := map[string]interface{}{
		"any_field":  []interface{}{"level", "severity", "priority"},
		"any_equals": "critical",
	}

	filter, err := NewIncludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test record with matching field
	record := map[string]interface{}{
		"level": "critical",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result.Skip {
		t.Error("Filter should not skip matching record")
	}

	// Test record with matching different field
	record2 := map[string]interface{}{
		"severity": "critical",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result2.Skip {
		t.Error("Filter should not skip matching record")
	}

	// Test record with no matching fields
	record3 := map[string]interface{}{
		"level": "info",
	}

	result3, err := filter.Apply(ctx, record3)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result3.Skip {
		t.Error("Filter should skip non-matching record")
	}
}

func TestIncludeFilter_MissingField(t *testing.T) {
	config := map[string]interface{}{
		"field":  "status",
		"equals": "success",
	}

	filter, err := NewIncludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test record without the field
	record := map[string]interface{}{
		"other_field": "some value",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Skip {
		t.Error("Filter should skip record without the required field")
	}
}

func TestIncludeFilter_Validation(t *testing.T) {
	// Valid config
	config := map[string]interface{}{
		"field":  "status",
		"equals": "success",
	}

	filter, err := NewIncludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	if err := filter.Validate(config); err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Invalid config (no conditions)
	invalidConfig := map[string]interface{}{}

	if err := filter.Validate(invalidConfig); err == nil {
		t.Error("Config without conditions should fail validation")
	}
}
