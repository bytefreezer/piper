// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// IncludeFilter keeps only events that match specified conditions (drops everything else)
type IncludeFilter struct {
	FieldMatcher
}

// NewIncludeFilter creates a new include filter
func NewIncludeFilter(config map[string]interface{}) (Filter, error) {
	fm, err := ParseFieldMatcher(config)
	if err != nil {
		return nil, err
	}
	return &IncludeFilter{FieldMatcher: *fm}, nil
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
