// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import (
	"fmt"
	"regexp"
)

// FieldMatcher holds common field-matching conditions used by include/exclude filters
type FieldMatcher struct {
	Field    string
	Equals   interface{}
	Contains string
	Matches  string
	MatchRe  *regexp.Regexp

	AnyField   []string
	AnyEquals  interface{}
	AnyMatches string
	AnyMatchRe *regexp.Regexp
}

// ParseFieldMatcher parses common field/equals/contains/matches config into a FieldMatcher
func ParseFieldMatcher(config map[string]interface{}) (*FieldMatcher, error) {
	fm := &FieldMatcher{}

	if field, ok := config["field"].(string); ok {
		fm.Field = field
	}
	if equals, ok := config["equals"]; ok {
		fm.Equals = equals
	}
	if contains, ok := config["contains"].(string); ok {
		fm.Contains = contains
	}
	if matches, ok := config["matches"].(string); ok {
		fm.Matches = matches
		compiled, err := regexp.Compile(matches)
		if err != nil {
			return nil, fmt.Errorf("failed to compile matches pattern '%s': %w", matches, err)
		}
		fm.MatchRe = compiled
	}
	if anyFieldInterface, ok := config["any_field"].([]interface{}); ok {
		fm.AnyField = make([]string, 0, len(anyFieldInterface))
		for _, fieldInterface := range anyFieldInterface {
			if field, ok := fieldInterface.(string); ok {
				fm.AnyField = append(fm.AnyField, field)
			}
		}
	}
	if anyEquals, ok := config["any_equals"]; ok {
		fm.AnyEquals = anyEquals
	}
	if anyMatches, ok := config["any_matches"].(string); ok {
		fm.AnyMatches = anyMatches
		compiled, err := regexp.Compile(anyMatches)
		if err != nil {
			return nil, fmt.Errorf("failed to compile any_matches pattern '%s': %w", anyMatches, err)
		}
		fm.AnyMatchRe = compiled
	}

	return fm, nil
}

// CompareValues compares two values for equality
// First tries direct comparison, then falls back to string comparison
func CompareValues(a, b any) bool {
	// Try direct comparison first
	if a == b {
		return true
	}

	// Convert both to strings for comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	return aStr == bStr
}
