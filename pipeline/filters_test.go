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
