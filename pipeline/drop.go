package pipeline

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/n0needt0/go-goodies/log"
)

// DropFilter conditionally drops events from the pipeline
type DropFilter struct {
	// Field-based conditions
	IfField    string
	Equals     interface{}
	NotEquals  interface{}
	Contains   string
	Matches    string
	MatchesRe  *regexp.Regexp

	// Percentage-based dropping
	Percentage float64

	// Always drop (for testing)
	AlwaysDrop bool

	// Inverse logic
	UnlessField   string
	UnlessEquals  interface{}
	UnlessMatches string
	UnlessMatchRe *regexp.Regexp

	// Random seed for percentage-based dropping
	random *rand.Rand
}

// NewDropFilter creates a new drop filter
func NewDropFilter(config map[string]interface{}) (Filter, error) {
	filter := &DropFilter{
		random: rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404 - weak random is acceptable for data sampling, not cryptographic use
	}

	// Parse if_field condition
	if ifField, ok := config["if_field"].(string); ok {
		filter.IfField = ifField
	}

	// Parse equals condition
	if equals, ok := config["equals"]; ok {
		filter.Equals = equals
	}

	// Parse not_equals condition
	if notEquals, ok := config["not_equals"]; ok {
		filter.NotEquals = notEquals
	}

	// Parse contains condition
	if contains, ok := config["contains"].(string); ok {
		filter.Contains = contains
	}

	// Parse matches condition (regex)
	if matches, ok := config["matches"].(string); ok {
		filter.Matches = matches
		compiled, err := regexp.Compile(matches)
		if err != nil {
			return nil, fmt.Errorf("failed to compile matches pattern '%s': %w", matches, err)
		}
		filter.MatchesRe = compiled
	}

	// Parse unless_field condition
	if unlessField, ok := config["unless_field"].(string); ok {
		filter.UnlessField = unlessField
	}

	// Parse unless_equals condition
	if unlessEquals, ok := config["unless_equals"]; ok {
		filter.UnlessEquals = unlessEquals
	}

	// Parse unless_matches condition
	if unlessMatches, ok := config["unless_matches"].(string); ok {
		filter.UnlessMatches = unlessMatches
		compiled, err := regexp.Compile(unlessMatches)
		if err != nil {
			return nil, fmt.Errorf("failed to compile unless_matches pattern '%s': %w", unlessMatches, err)
		}
		filter.UnlessMatchRe = compiled
	}

	// Parse percentage
	if percentage, ok := config["percentage"].(float64); ok {
		filter.Percentage = percentage
	} else if percentageInt, ok := config["percentage"].(int); ok {
		filter.Percentage = float64(percentageInt)
	}

	// Parse always_drop
	if alwaysDrop, ok := config["always_drop"].(bool); ok {
		filter.AlwaysDrop = alwaysDrop
	}

	return filter, nil
}

// Type returns the filter type
func (f *DropFilter) Type() string {
	return "drop"
}

// Validate validates the filter configuration
func (f *DropFilter) Validate(config map[string]interface{}) error {
	// At least one condition should be specified
	hasCondition := false

	if _, ok := config["if_field"]; ok {
		hasCondition = true
	}
	if _, ok := config["unless_field"]; ok {
		hasCondition = true
	}
	if _, ok := config["percentage"]; ok {
		hasCondition = true
	}
	if _, ok := config["always_drop"]; ok {
		hasCondition = true
	}

	if !hasCondition {
		return fmt.Errorf("drop filter requires at least one condition (if_field, unless_field, percentage, or always_drop)")
	}

	// Validate percentage range
	if percentage, ok := config["percentage"].(float64); ok {
		if percentage < 0 || percentage > 100 {
			return fmt.Errorf("percentage must be between 0 and 100, got: %f", percentage)
		}
	}

	return nil
}

// Apply applies the drop filter to a record
func (f *DropFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Check always_drop first
	if f.AlwaysDrop {
		log.Debugf("Drop filter: always_drop enabled, dropping event")
		return &FilterResult{
			Record:   record,
			Skip:     true, // Mark for dropping
			Applied:  true,
			Duration: time.Since(start),
		}, nil
	}

	// Check unless conditions (inverse logic - if condition matches, DON'T drop)
	if f.UnlessField != "" {
		fieldValue, exists := record[f.UnlessField]

		if exists {
			// Check unless_equals
			if f.UnlessEquals != nil && f.compareValues(fieldValue, f.UnlessEquals) {
				log.Debugf("Drop filter: unless_equals condition matched, NOT dropping")
				return &FilterResult{
					Record:   record,
					Skip:     false,
					Applied:  false,
					Duration: time.Since(start),
				}, nil
			}

			// Check unless_matches
			if f.UnlessMatchRe != nil {
				if strValue, ok := fieldValue.(string); ok {
					if f.UnlessMatchRe.MatchString(strValue) {
						log.Debugf("Drop filter: unless_matches condition matched, NOT dropping")
						return &FilterResult{
							Record:   record,
							Skip:     false,
							Applied:  false,
							Duration: time.Since(start),
						}, nil
					}
				}
			}
		}
	}

	// Check if conditions (positive logic - if condition matches, DO drop)
	shouldDrop := false

	if f.IfField != "" {
		fieldValue, exists := record[f.IfField]

		if !exists {
			// Field doesn't exist, don't drop
			return &FilterResult{
				Record:   record,
				Skip:     false,
				Applied:  false,
				Duration: time.Since(start),
			}, nil
		}

		// Check equals condition
		if f.Equals != nil {
			if f.compareValues(fieldValue, f.Equals) {
				shouldDrop = true
			}
		}

		// Check not_equals condition
		if f.NotEquals != nil {
			if !f.compareValues(fieldValue, f.NotEquals) {
				shouldDrop = true
			}
		}

		// Check contains condition
		if f.Contains != "" {
			if strValue, ok := fieldValue.(string); ok {
				if strings.Contains(strValue, f.Contains) {
					shouldDrop = true
				}
			}
		}

		// Check matches condition (regex)
		if f.MatchesRe != nil {
			if strValue, ok := fieldValue.(string); ok {
				if f.MatchesRe.MatchString(strValue) {
					shouldDrop = true
				}
			}
		}
	}

	// Apply percentage-based dropping
	if f.Percentage > 0 && shouldDrop {
		randomValue := f.random.Float64() * 100
		if randomValue > f.Percentage {
			// Random value is higher than percentage, don't drop
			shouldDrop = false
			log.Debugf("Drop filter: percentage check failed (%.2f > %.2f), NOT dropping", randomValue, f.Percentage)
		} else {
			log.Debugf("Drop filter: percentage check passed (%.2f <= %.2f), dropping", randomValue, f.Percentage)
		}
	} else if f.Percentage > 0 && !shouldDrop && f.IfField == "" {
		// Percentage-only mode (no if_field condition)
		randomValue := f.random.Float64() * 100
		if randomValue <= f.Percentage {
			shouldDrop = true
			log.Debugf("Drop filter: percentage-only check passed (%.2f <= %.2f), dropping", randomValue, f.Percentage)
		}
	}

	if shouldDrop {
		log.Debugf("Drop filter: conditions met, dropping event")
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

// compareValues compares two values for equality
func (f *DropFilter) compareValues(a, b interface{}) bool {
	// Try direct comparison first
	if a == b {
		return true
	}

	// Convert both to strings for comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	return aStr == bStr
}
