// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// ExcludeFilter drops events that match specified conditions (keeps everything else)
type ExcludeFilter struct {
	FieldMatcher
}

// NewExcludeFilter creates a new exclude filter
func NewExcludeFilter(config map[string]interface{}) (Filter, error) {
	fm, err := ParseFieldMatcher(config)
	if err != nil {
		return nil, err
	}
	return &ExcludeFilter{FieldMatcher: *fm}, nil
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
				if CompareValues(fieldValue, f.Equals) {
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
				if CompareValues(fieldValue, f.AnyEquals) {
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
