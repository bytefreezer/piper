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

// IncludeFilter keeps only events that match specified conditions (drops everything else)
type IncludeFilter struct {
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

// NewIncludeFilter creates a new include filter
func NewIncludeFilter(config map[string]interface{}) (Filter, error) {
	filter := &IncludeFilter{}

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
func (f *IncludeFilter) Type() string {
	return "include"
}

// Validate validates the filter configuration
func (f *IncludeFilter) Validate(config map[string]interface{}) error {
	// At least one condition should be specified
	hasCondition := false

	if _, ok := config["field"]; ok {
		hasCondition = true
	}
	if _, ok := config["any_field"]; ok {
		hasCondition = true
	}

	if !hasCondition {
		return fmt.Errorf("include filter requires at least one condition (field or any_field)")
	}

	return nil
}

// Apply applies the include filter to a record
func (f *IncludeFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	shouldKeep := false

	// Check single field conditions
	if f.Field != "" {
		fieldValue, exists := record[f.Field]

		if exists {
			// Check equals condition
			if f.Equals != nil {
				if CompareValues(fieldValue, f.Equals) {
					shouldKeep = true
				}
			}

			// Check contains condition
			if f.Contains != "" {
				if strValue, ok := fieldValue.(string); ok {
					if strings.Contains(strValue, f.Contains) {
						shouldKeep = true
					}
				}
			}

			// Check matches condition (regex)
			if f.MatchRe != nil {
				if strValue, ok := fieldValue.(string); ok {
					if f.MatchRe.MatchString(strValue) {
						shouldKeep = true
					}
				}
			}
		}
	}

	// Check any_field conditions (OR logic - if any field matches, keep)
	if len(f.AnyField) > 0 && !shouldKeep {
		for _, fieldName := range f.AnyField {
			fieldValue, exists := record[fieldName]
			if !exists {
				continue
			}

			// Check any_equals
			if f.AnyEquals != nil {
				if CompareValues(fieldValue, f.AnyEquals) {
					shouldKeep = true
					break
				}
			}

			// Check any_matches
			if f.AnyMatchRe != nil {
				if strValue, ok := fieldValue.(string); ok {
					if f.AnyMatchRe.MatchString(strValue) {
						shouldKeep = true
						break
					}
				}
			}
		}
	}

	if !shouldKeep {
		log.Debugf("Include filter: conditions not met, dropping event")
		return &FilterResult{
			Record:   record,
			Skip:     true, // Drop events that don't match
			Applied:  true,
			Duration: time.Since(start),
		}, nil
	}

	log.Debugf("Include filter: conditions met, keeping event")
	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}
