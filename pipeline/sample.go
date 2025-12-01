package pipeline

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// SampleFilter keeps a random percentage of events (drops the rest)
type SampleFilter struct {
	// Percentage of events to keep (0-100)
	Percentage float64

	// Rate as fraction (0.0-1.0) - alternative to percentage
	Rate float64

	// Random seed for deterministic sampling (optional)
	Seed int64

	// Random number generator
	random *rand.Rand
}

// NewSampleFilter creates a new sample filter
func NewSampleFilter(config map[string]interface{}) (Filter, error) {
	filter := &SampleFilter{}

	// Track if percentage or rate was explicitly set
	hasPercentage := false
	hasRate := false

	// Parse percentage (0-100)
	if percentage, ok := config["percentage"].(float64); ok {
		if percentage < 0 || percentage > 100 {
			return nil, fmt.Errorf("percentage must be between 0 and 100, got: %f", percentage)
		}
		filter.Percentage = percentage
		filter.Rate = percentage / 100.0
		hasPercentage = true
	} else if percentageInt, ok := config["percentage"].(int); ok {
		if percentageInt < 0 || percentageInt > 100 {
			return nil, fmt.Errorf("percentage must be between 0 and 100, got: %d", percentageInt)
		}
		filter.Percentage = float64(percentageInt)
		filter.Rate = float64(percentageInt) / 100.0
		hasPercentage = true
	}

	// Parse rate (0.0-1.0) - alternative to percentage
	if rate, ok := config["rate"].(float64); ok {
		if rate < 0 || rate > 1 {
			return nil, fmt.Errorf("rate must be between 0.0 and 1.0, got: %f", rate)
		}
		filter.Rate = rate
		filter.Percentage = rate * 100.0
		hasRate = true
	}

	// Validate we have either percentage or rate
	if !hasPercentage && !hasRate {
		return nil, fmt.Errorf("sample filter requires 'percentage' (0-100) or 'rate' (0.0-1.0) parameter")
	}

	// Parse optional seed for deterministic sampling
	if seed, ok := config["seed"].(int64); ok {
		filter.Seed = seed
		filter.random = rand.New(rand.NewSource(seed)) // #nosec G404 - weak random is acceptable for data sampling, not cryptographic use
	} else if seedInt, ok := config["seed"].(int); ok {
		filter.Seed = int64(seedInt)
		filter.random = rand.New(rand.NewSource(int64(seedInt))) // #nosec G404 - weak random is acceptable for data sampling, not cryptographic use
	} else {
		// Use current time for non-deterministic sampling
		filter.Seed = time.Now().UnixNano()
		filter.random = rand.New(rand.NewSource(filter.Seed)) // #nosec G404 - weak random is acceptable for data sampling, not cryptographic use
	}

	return filter, nil
}

// Type returns the filter type
func (f *SampleFilter) Type() string {
	return "sample"
}

// Validate validates the filter configuration
func (f *SampleFilter) Validate(config map[string]interface{}) error {
	// Check for percentage or rate
	hasPercentage := false
	hasRate := false

	if percentage, ok := config["percentage"].(float64); ok {
		hasPercentage = true
		if percentage < 0 || percentage > 100 {
			return fmt.Errorf("percentage must be between 0 and 100")
		}
	} else if percentage, ok := config["percentage"].(int); ok {
		hasPercentage = true
		if percentage < 0 || percentage > 100 {
			return fmt.Errorf("percentage must be between 0 and 100")
		}
	}

	if rate, ok := config["rate"].(float64); ok {
		hasRate = true
		if rate < 0 || rate > 1 {
			return fmt.Errorf("rate must be between 0.0 and 1.0")
		}
	}

	if !hasPercentage && !hasRate {
		return fmt.Errorf("sample filter requires 'percentage' or 'rate' parameter")
	}

	return nil
}

// Apply applies the sample filter to a record
func (f *SampleFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Generate random value between 0.0 and 1.0
	randomValue := f.random.Float64()

	// Keep event if random value is less than rate
	if randomValue <= f.Rate {
		log.Debugf("Sample filter: keeping event (%.4f <= %.4f)", randomValue, f.Rate)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  true,
			Duration: time.Since(start),
		}, nil
	}

	log.Debugf("Sample filter: dropping event (%.4f > %.4f)", randomValue, f.Rate)
	return &FilterResult{
		Record:   record,
		Skip:     true,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}
