// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package pipeline

import "fmt"

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
