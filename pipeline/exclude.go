// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// ExcludeFilter drops events that match specified conditions (keeps everything else)
type ExcludeFilter struct {
	// Field-based conditions
	Field    string
	Equals   interface{}
	Contains string
	Matches  string
	MatchRe  *regexp.Regexp

	// Multiple field matching (OR logic)
	AnyField   []string
	AnyEquals  interface{}
	AnyMatches string
	AnyMatchRe *regexp.Regexp
}

// NewExcludeFilter creates a new exclude filter
func NewExcludeFilter(config map[string]interface{}) (Filter, error) {
	filter := &ExcludeFilter{}

	// Parse field condition
	if field, ok := config["field"].(string); ok {
		filter.Field = field
	}

	// Parse equals condition
	if equals, ok := config["equals"]; ok {
		filter.Equals = equals
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
		filter.MatchRe = compiled
	}

	// Parse any_field condition (array of fields)
	if anyFieldInterface, ok := config["any_field"].([]interface{}); ok {
		filter.AnyField = make([]string, 0, len(anyFieldInterface))
		for _, fieldInterface := range anyFieldInterface {
			if field, ok := fieldInterface.(string); ok {
				filter.AnyField = append(filter.AnyField, field)
			}
		}
	}

	// Parse any_equals condition
	if anyEquals, ok := config["any_equals"]; ok {
		filter.AnyEquals = anyEquals
	}

	// Parse any_matches condition
	if anyMatches, ok := config["any_matches"].(string); ok {
		filter.AnyMatches = anyMatches
		compiled, err := regexp.Compile(anyMatches)
		if err != nil {
			return nil, fmt.Errorf("failed to compile any_matches pattern '%s': %w", anyMatches, err)
		}
		filter.AnyMatchRe = compiled
	}

	return filter, nil
}

// Type returns the filter type
func (f *ExcludeFilter) Type() string {
	return "exclude"
}

// Validate validates the filter configuration
func (f *ExcludeFilter) Validate(config map[string]interface{}) error {
	// At least one condition should be specified
	hasCondition := false

	if _, ok := config["field"]; ok {
		hasCondition = true
	}
	if _, ok := config["any_field"]; ok {
		hasCondition = true
	}

	if !hasCondition {
		return fmt.Errorf("exclude filter requires at least one condition (field or any_field)")
	}

	return nil
}

// Apply applies the exclude filter to a record
func (f *ExcludeFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	shouldDrop := false

	// Check single field conditions
	if f.Field != "" {
		fieldValue, exists := record[f.Field]

		if exists {
			// Check equals condition
			if f.Equals != nil {
				if f.compareValues(fieldValue, f.Equals) {
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
			if f.MatchRe != nil {
				if strValue, ok := fieldValue.(string); ok {
					if f.MatchRe.MatchString(strValue) {
						shouldDrop = true
					}
				}
			}
		}
	}

	// Check any_field conditions (OR logic - if any field matches, drop)
	if len(f.AnyField) > 0 && !shouldDrop {
		for _, fieldName := range f.AnyField {
			fieldValue, exists := record[fieldName]
			if !exists {
				continue
			}

			// Check any_equals
			if f.AnyEquals != nil {
				if f.compareValues(fieldValue, f.AnyEquals) {
					shouldDrop = true
					break
				}
			}

			// Check any_matches
			if f.AnyMatchRe != nil {
				if strValue, ok := fieldValue.(string); ok {
					if f.AnyMatchRe.MatchString(strValue) {
						shouldDrop = true
						break
					}
				}
			}
		}
	}

	if shouldDrop {
		log.Debugf("Exclude filter: conditions met, dropping event")
		return &FilterResult{
			Record:   record,
			Skip:     true,
			Applied:  true,
			Duration: time.Since(start),
		}, nil
	}

	log.Debugf("Exclude filter: conditions not met, keeping event")
	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// compareValues compares two values for equality
func (f *ExcludeFilter) compareValues(a, b interface{}) bool {
	// Try direct comparison first
	if a == b {
		return true
	}

	// Convert both to strings for comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	return aStr == bStr
}
