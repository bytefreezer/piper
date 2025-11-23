package pipeline

import (
	"testing"
	"time"
)

func TestUppercaseKeysFilter_SingleField(t *testing.T) {
	config := map[string]interface{}{
		"source_field": "metadata",
		"recursive":    true,
	}

	filter, err := NewUppercaseKeysFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:  "test-tenant",
		DatasetID: "test-dataset",
		Timestamp: time.Now(),
	}

	input := map[string]interface{}{
		"metadata": map[string]interface{}{
			"user_name": "john",
			"user_id":   123,
		},
		"context": map[string]interface{}{
			"app_name": "myapp",
			"version":  "1.0",
		},
		"message": "test event",
	}

	result, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	if !result.Applied {
		t.Fatal("Filter was not applied")
	}

	metadata, ok := result.Record["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata field is not a map")
	}

	if _, ok := metadata["USER_NAME"]; !ok {
		t.Error("Expected USER_NAME key in metadata")
	}
	if _, ok := metadata["USER_ID"]; !ok {
		t.Error("Expected USER_ID key in metadata")
	}

	// Context should not be modified
	context, ok := result.Record["context"].(map[string]interface{})
	if !ok {
		t.Fatal("context field is not a map")
	}
	if _, ok := context["app_name"]; !ok {
		t.Error("Expected app_name key in context (should not be uppercased)")
	}
}

func TestUppercaseKeysFilter_ArrayOfFields(t *testing.T) {
	config := map[string]interface{}{
		"source_field": []interface{}{"metadata", "context"},
		"recursive":    true,
	}

	filter, err := NewUppercaseKeysFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:  "test-tenant",
		DatasetID: "test-dataset",
		Timestamp: time.Now(),
	}

	input := map[string]interface{}{
		"metadata": map[string]interface{}{
			"user_name": "john",
			"user_id":   123,
		},
		"context": map[string]interface{}{
			"app_name": "myapp",
			"version":  "1.0",
		},
		"message": "test event",
	}

	result, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	if !result.Applied {
		t.Fatal("Filter was not applied")
	}

	metadata, ok := result.Record["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata field is not a map")
	}

	if _, ok := metadata["USER_NAME"]; !ok {
		t.Error("Expected USER_NAME key in metadata")
	}
	if _, ok := metadata["USER_ID"]; !ok {
		t.Error("Expected USER_ID key in metadata")
	}

	// Context should also be modified
	context, ok := result.Record["context"].(map[string]interface{})
	if !ok {
		t.Fatal("context field is not a map")
	}
	if _, ok := context["APP_NAME"]; !ok {
		t.Error("Expected APP_NAME key in context")
	}
	if _, ok := context["VERSION"]; !ok {
		t.Error("Expected VERSION key in context")
	}
}

func TestUppercaseKeysFilter_Wildcard(t *testing.T) {
	config := map[string]interface{}{
		"source_field": "*",
		"recursive":    true,
	}

	filter, err := NewUppercaseKeysFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:  "test-tenant",
		DatasetID: "test-dataset",
		Timestamp: time.Now(),
	}

	input := map[string]interface{}{
		"metadata": map[string]interface{}{
			"user_name": "john",
		},
		"message": "test",
	}

	result, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	if !result.Applied {
		t.Fatal("Filter was not applied")
	}

	// All top-level keys should be uppercased
	if _, ok := result.Record["METADATA"]; !ok {
		t.Error("Expected METADATA key")
	}
	if _, ok := result.Record["MESSAGE"]; !ok {
		t.Error("Expected MESSAGE key")
	}
}

func TestUppercaseKeysFilter_EntireRecord(t *testing.T) {
	config := map[string]interface{}{
		"recursive": true,
	}

	filter, err := NewUppercaseKeysFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:  "test-tenant",
		DatasetID: "test-dataset",
		Timestamp: time.Now(),
	}

	input := map[string]interface{}{
		"metadata": map[string]interface{}{
			"user_name": "john",
		},
		"message": "test",
	}

	result, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	if !result.Applied {
		t.Fatal("Filter was not applied")
	}

	// All top-level keys should be uppercased
	if _, ok := result.Record["METADATA"]; !ok {
		t.Error("Expected METADATA key")
	}
	if _, ok := result.Record["MESSAGE"]; !ok {
		t.Error("Expected MESSAGE key")
	}

	// Nested keys should also be uppercased
	metadata, ok := result.Record["METADATA"].(map[string]interface{})
	if !ok {
		t.Fatal("METADATA field is not a map")
	}
	if _, ok := metadata["USER_NAME"]; !ok {
		t.Error("Expected USER_NAME key in metadata")
	}
}

func TestUppercaseKeysFilter_EmptyArray(t *testing.T) {
	config := map[string]interface{}{
		"source_field": []interface{}{},
		"recursive":    true,
	}

	filter, err := NewUppercaseKeysFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:  "test-tenant",
		DatasetID: "test-dataset",
		Timestamp: time.Now(),
	}

	input := map[string]interface{}{
		"metadata": map[string]interface{}{
			"user_name": "john",
		},
		"message": "test",
	}

	result, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("Filter returned error: %v", err)
	}
	if !result.Applied {
		t.Fatal("Filter was not applied")
	}

	// Empty array should operate on entire record
	if _, ok := result.Record["METADATA"]; !ok {
		t.Error("Expected METADATA key")
	}
	if _, ok := result.Record["MESSAGE"]; !ok {
		t.Error("Expected MESSAGE key")
	}
}

func TestJSONFlattenFilter_FlattenEntireRecord(t *testing.T) {
	// Test flattening entire record when source_field is not specified
	config := map[string]interface{}{
		"recursive": true,
		"separator": ".",
	}

	filter, err := NewJSONFlattenFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	// Test input matching user's data structure
	input := map[string]interface{}{
		"timestamp": "2025-01-22T10:00:00Z",
		"data": map[string]interface{}{
			"user": map[string]interface{}{
				"name": "John",
				"age":  float64(30),
				"address": map[string]interface{}{
					"city": "NYC",
					"zip":  "10001",
				},
			},
			"tags": []interface{}{"test", "demo"},
		},
		"FileDevice": []interface{}{"1eh", "cgroup2", "/sys/fs/cgroup"},
		"FileEvents": map[string]interface{}{
			"ACCESS":        float64(1),
			"CLOSE_NOWRITE": float64(1),
			"OPEN":          float64(1),
		},
		"FilePermissions": []interface{}{"0644", "-rw-r--r--"},
	}

	ctx := &FilterContext{
		TenantID:  "test-tenant",
		DatasetID: "test-dataset",
		Timestamp: time.Now(),
	}
	result, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("Filter Apply failed: %v", err)
	}

	if !result.Applied {
		t.Error("Filter was not applied")
	}

	// Check that nested structures are flattened
	expectedKeys := []string{
		"timestamp",
		"data.user.name",
		"data.user.age",
		"data.user.address.city",
		"data.user.address.zip",
		"data.tags.0",
		"data.tags.1",
		"FileDevice.0",
		"FileDevice.1",
		"FileDevice.2",
		"FileEvents.ACCESS",
		"FileEvents.CLOSE_NOWRITE",
		"FileEvents.OPEN",
		"FilePermissions.0",
		"FilePermissions.1",
	}

	for _, key := range expectedKeys {
		if _, exists := result.Record[key]; !exists {
			t.Errorf("Expected key '%s' not found in flattened result", key)
		}
	}

	// Verify specific values
	if val, ok := result.Record["data.user.name"]; !ok || val != "John" {
		t.Errorf("Expected 'data.user.name' to be 'John', got %v", val)
	}
	if val, ok := result.Record["FileDevice.0"]; !ok || val != "1eh" {
		t.Errorf("Expected 'FileDevice.0' to be '1eh', got %v", val)
	}
	if val, ok := result.Record["FileEvents.ACCESS"]; !ok || val != float64(1) {
		t.Errorf("Expected 'FileEvents.ACCESS' to be 1, got %v", val)
	}
}

func TestJSONFlattenFilter_WithSourceField(t *testing.T) {
	// Test flattening specific field when source_field is specified
	config := map[string]interface{}{
		"source_field": "data",
		"target_field": "flattened",
		"separator":    ".",
	}

	filter, err := NewJSONFlattenFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	input := map[string]interface{}{
		"timestamp": "2025-01-22T10:00:00Z",
		"data": map[string]interface{}{
			"user": map[string]interface{}{
				"name": "John",
				"age":  float64(30),
			},
		},
	}

	ctx := &FilterContext{
		TenantID:  "test-tenant",
		DatasetID: "test-dataset",
		Timestamp: time.Now(),
	}
	result, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("Filter Apply failed: %v", err)
	}

	if !result.Applied {
		t.Error("Filter was not applied")
	}

	// Original timestamp should still exist
	if _, exists := result.Record["timestamp"]; !exists {
		t.Error("Original 'timestamp' field should still exist")
	}

	// Flattened data should be in 'flattened' field
	flattened, exists := result.Record["flattened"]
	if !exists {
		t.Fatal("'flattened' field not found")
	}

	flattenedMap, ok := flattened.(map[string]interface{})
	if !ok {
		t.Fatal("'flattened' field is not a map")
	}

	if _, exists := flattenedMap["user.name"]; !exists {
		t.Error("Expected 'user.name' in flattened map")
	}
}
