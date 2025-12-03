// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"testing"
	"time"
)

func TestExcludeFilter_Equals(t *testing.T) {
	config := map[string]interface{}{
		"field":  "status",
		"equals": "failure",
	}

	filter, err := NewExcludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test matching record (should drop)
	record := map[string]interface{}{
		"status": "failure",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Skip {
		t.Error("Filter should skip matching record")
	}

	if !result.Applied {
		t.Error("Filter should be applied")
	}

	// Test non-matching record (should keep)
	record2 := map[string]interface{}{
		"status": "success",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result2.Skip {
		t.Error("Filter should not skip non-matching record")
	}
}

func TestExcludeFilter_Contains(t *testing.T) {
	config := map[string]interface{}{
		"field":    "message",
		"contains": "debug",
	}

	filter, err := NewExcludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test matching record (should drop)
	record := map[string]interface{}{
		"message": "This is a debug message",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Skip {
		t.Error("Filter should skip matching record")
	}

	// Test non-matching record (should keep)
	record2 := map[string]interface{}{
		"message": "This is an error message",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result2.Skip {
		t.Error("Filter should not skip non-matching record")
	}
}

func TestExcludeFilter_Matches(t *testing.T) {
	config := map[string]interface{}{
		"field":   "level",
		"matches": "^(debug|trace)$",
	}

	filter, err := NewExcludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test matching record (should drop)
	record := map[string]interface{}{
		"level": "debug",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Skip {
		t.Error("Filter should skip matching record")
	}

	// Test non-matching record (should keep)
	record2 := map[string]interface{}{
		"level": "error",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result2.Skip {
		t.Error("Filter should not skip non-matching record")
	}
}

func TestExcludeFilter_AnyField(t *testing.T) {
	config := map[string]interface{}{
		"any_field":   []interface{}{"user_agent", "client", "source"},
		"any_matches": "bot",
	}

	filter, err := NewExcludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test record with matching field (should drop)
	record := map[string]interface{}{
		"user_agent": "Mozilla/5.0 (compatible; Googlebot/2.1)",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if !result.Skip {
		t.Error("Filter should skip matching record")
	}

	// Test record with no matching fields (should keep)
	record2 := map[string]interface{}{
		"user_agent": "Mozilla/5.0 (Windows NT 10.0)",
	}

	result2, err := filter.Apply(ctx, record2)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result2.Skip {
		t.Error("Filter should not skip non-matching record")
	}
}

func TestExcludeFilter_MissingField(t *testing.T) {
	config := map[string]interface{}{
		"field":  "status",
		"equals": "failure",
	}

	filter, err := NewExcludeFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test record without the field (should keep)
	record := map[string]interface{}{
		"other_field": "some value",
	}

	result, err := filter.Apply(ctx, record)
	if err != nil {
		t.Fatalf("Filter apply failed: %v", err)
	}

	if result.Skip {
		t.Error("Filter should not skip record without the field")
	}
}

func TestExcludeFilter_Validation(t *testing.T) {
	// Valid config
	config := map[string]interface{}{
		"field":  "status",
		"equals": "failure",
	}

	filter, err := NewExcludeFilter(config)
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
