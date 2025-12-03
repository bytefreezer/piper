// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"testing"
	"time"
)

func TestSampleFilter_Percentage(t *testing.T) {
	config := map[string]interface{}{
		"percentage": 50.0,
		"seed":       int64(12345), // Deterministic seed for testing
	}

	filter, err := NewSampleFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Run multiple times and count kept vs dropped
	kept := 0
	dropped := 0
	totalRuns := 1000

	for i := 0; i < totalRuns; i++ {
		record := map[string]interface{}{
			"message": "test message",
		}

		result, err := filter.Apply(ctx, record)
		if err != nil {
			t.Fatalf("Filter apply failed: %v", err)
		}

		if !result.Applied {
			t.Error("Filter should always be applied")
		}

		if result.Skip {
			dropped++
		} else {
			kept++
		}
	}

	// Check that approximately 50% were kept
	keptPercentage := float64(kept) / float64(totalRuns) * 100
	if keptPercentage < 40 || keptPercentage > 60 {
		t.Errorf("Expected approximately 50%% kept, got %.2f%% (%d/%d)", keptPercentage, kept, totalRuns)
	}
}

func TestSampleFilter_Rate(t *testing.T) {
	config := map[string]interface{}{
		"rate": 0.25,
		"seed": int64(54321), // Deterministic seed for testing
	}

	filter, err := NewSampleFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Run multiple times and count kept vs dropped
	kept := 0
	dropped := 0
	totalRuns := 1000

	for i := 0; i < totalRuns; i++ {
		record := map[string]interface{}{
			"message": "test message",
		}

		result, err := filter.Apply(ctx, record)
		if err != nil {
			t.Fatalf("Filter apply failed: %v", err)
		}

		if result.Skip {
			dropped++
		} else {
			kept++
		}
	}

	// Check that approximately 25% were kept
	keptPercentage := float64(kept) / float64(totalRuns) * 100
	if keptPercentage < 15 || keptPercentage > 35 {
		t.Errorf("Expected approximately 25%% kept, got %.2f%% (%d/%d)", keptPercentage, kept, totalRuns)
	}
}

func TestSampleFilter_FullSample(t *testing.T) {
	// 100% sampling should keep all events
	config := map[string]interface{}{
		"percentage": 100.0,
	}

	filter, err := NewSampleFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test multiple times - all should be kept
	for i := 0; i < 100; i++ {
		record := map[string]interface{}{
			"message": "test message",
		}

		result, err := filter.Apply(ctx, record)
		if err != nil {
			t.Fatalf("Filter apply failed: %v", err)
		}

		if result.Skip {
			t.Error("100% sampling should never skip events")
		}
	}
}

func TestSampleFilter_NoSample(t *testing.T) {
	// 0% sampling should drop all events
	config := map[string]interface{}{
		"percentage": 0.0,
	}

	filter, err := NewSampleFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Test multiple times - all should be dropped
	for i := 0; i < 100; i++ {
		record := map[string]interface{}{
			"message": "test message",
		}

		result, err := filter.Apply(ctx, record)
		if err != nil {
			t.Fatalf("Filter apply failed: %v", err)
		}

		if !result.Skip {
			t.Error("0% sampling should always skip events")
		}
	}
}

func TestSampleFilter_DeterministicSeed(t *testing.T) {
	// Same seed should produce same results
	config1 := map[string]interface{}{
		"percentage": 50.0,
		"seed":       int64(99999),
	}

	filter1, err := NewSampleFilter(config1)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	config2 := map[string]interface{}{
		"percentage": 50.0,
		"seed":       int64(99999),
	}

	filter2, err := NewSampleFilter(config2)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	ctx := &FilterContext{
		TenantID:   "test-tenant",
		DatasetID:  "test-dataset",
		Timestamp:  time.Now(),
		LineNumber: 1,
	}

	// Both filters with same seed should produce identical results
	for i := 0; i < 100; i++ {
		record1 := map[string]interface{}{
			"message": "test message",
		}

		record2 := map[string]interface{}{
			"message": "test message",
		}

		result1, _ := filter1.Apply(ctx, record1)
		result2, _ := filter2.Apply(ctx, record2)

		if result1.Skip != result2.Skip {
			t.Errorf("Same seed should produce same results at iteration %d", i)
		}
	}
}

func TestSampleFilter_InvalidPercentage(t *testing.T) {
	// Test percentage > 100
	config := map[string]interface{}{
		"percentage": 150.0,
	}

	_, err := NewSampleFilter(config)
	if err == nil {
		t.Error("Should fail with percentage > 100")
	}

	// Test percentage < 0
	config2 := map[string]interface{}{
		"percentage": -10.0,
	}

	_, err2 := NewSampleFilter(config2)
	if err2 == nil {
		t.Error("Should fail with percentage < 0")
	}
}

func TestSampleFilter_InvalidRate(t *testing.T) {
	// Test rate > 1.0
	config := map[string]interface{}{
		"rate": 1.5,
	}

	_, err := NewSampleFilter(config)
	if err == nil {
		t.Error("Should fail with rate > 1.0")
	}

	// Test rate < 0.0
	config2 := map[string]interface{}{
		"rate": -0.5,
	}

	_, err2 := NewSampleFilter(config2)
	if err2 == nil {
		t.Error("Should fail with rate < 0.0")
	}
}

func TestSampleFilter_Validation(t *testing.T) {
	// Valid config with percentage
	config := map[string]interface{}{
		"percentage": 50.0,
	}

	filter, err := NewSampleFilter(config)
	if err != nil {
		t.Fatalf("Failed to create filter: %v", err)
	}

	if err := filter.Validate(config); err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Valid config with rate
	config2 := map[string]interface{}{
		"rate": 0.5,
	}

	if err := filter.Validate(config2); err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Invalid config (no percentage or rate)
	invalidConfig := map[string]interface{}{}

	if err := filter.Validate(invalidConfig); err == nil {
		t.Error("Config without percentage or rate should fail validation")
	}
}
