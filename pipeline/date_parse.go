package pipeline

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bytefreezer/goodies/log"
)

// DateParseFilter parses date strings with format specifications
type DateParseFilter struct {
	SourceField string
	TargetField string
	Formats     []string
	Timezone    string
	Locale      string
	location    *time.Location
}

// NewDateParseFilterComplete creates a complete date parse filter
func NewDateParseFilterComplete(config map[string]interface{}) (Filter, error) {
	filter := &DateParseFilter{
		SourceField: "timestamp",
		TargetField: "@timestamp",
		Formats:     make([]string, 0),
		Timezone:    "UTC",
		Locale:      "en",
	}

	// Parse source_field
	if sourceField, ok := config["source_field"].(string); ok {
		filter.SourceField = sourceField
	}

	// Parse target_field
	if targetField, ok := config["target_field"].(string); ok {
		filter.TargetField = targetField
	}

	// Parse formats
	if formats, ok := config["formats"].([]interface{}); ok {
		for _, format := range formats {
			if formatStr, ok := format.(string); ok {
				filter.Formats = append(filter.Formats, formatStr)
			}
		}
	} else if format, ok := config["format"].(string); ok {
		filter.Formats = append(filter.Formats, format)
	}

	// Parse timezone
	if timezone, ok := config["timezone"].(string); ok {
		filter.Timezone = timezone
	}

	// Parse locale
	if locale, ok := config["locale"].(string); ok {
		filter.Locale = locale
	}

	// Load timezone location
	loc, err := time.LoadLocation(filter.Timezone)
	if err != nil {
		log.Warnf("Failed to load timezone '%s', using UTC: %v", filter.Timezone, err)
		loc = time.UTC
	}
	filter.location = loc

	// Default formats if none specified
	if len(filter.Formats) == 0 {
		filter.Formats = []string{
			time.RFC3339,
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02 15:04:05",
			"02/Jan/2006:15:04:05 -0700",
		}
	}

	return filter, nil
}

// Type returns the filter type
func (f *DateParseFilter) Type() string {
	return "date_parse"
}

// Validate validates the filter configuration
func (f *DateParseFilter) Validate(config map[string]interface{}) error {
	// No strict validation needed
	return nil
}

// Apply applies the date parse filter to a record
func (f *DateParseFilter) Apply(ctx *FilterContext, record map[string]interface{}) (*FilterResult, error) {
	start := time.Now()

	// Get source field value
	sourceValue, exists := record[f.SourceField]
	if !exists {
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Try to parse the value
	parsedTime, err := f.parseValue(sourceValue)
	if err != nil {
		log.Debugf("Date parse filter: failed to parse field '%s': %v", f.SourceField, err)
		return &FilterResult{
			Record:   record,
			Skip:     false,
			Applied:  false,
			Duration: time.Since(start),
		}, nil
	}

	// Store parsed timestamp
	record[f.TargetField] = parsedTime.Format(time.RFC3339)

	log.Debugf("Date parse filter: parsed '%s' to '%s'", f.SourceField, parsedTime.Format(time.RFC3339))

	return &FilterResult{
		Record:   record,
		Skip:     false,
		Applied:  true,
		Duration: time.Since(start),
	}, nil
}

// parseValue attempts to parse a value as a timestamp
func (f *DateParseFilter) parseValue(value interface{}) (time.Time, error) {
	// Handle different input types
	switch v := value.(type) {
	case string:
		return f.parseString(v)
	case int:
		return f.parseUnixTimestamp(int64(v))
	case int64:
		return f.parseUnixTimestamp(v)
	case float64:
		return f.parseUnixTimestamp(int64(v))
	default:
		return time.Time{}, fmt.Errorf("unsupported value type: %T", value)
	}
}

// parseString attempts to parse a string using configured formats
func (f *DateParseFilter) parseString(value string) (time.Time, error) {
	value = strings.TrimSpace(value)

	// Try each format
	for _, format := range f.Formats {
		// Handle special format keywords
		switch format {
		case "unix":
			// Unix timestamp in seconds
			ts, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				return f.parseUnixTimestamp(ts)
			}

		case "unix_ms":
			// Unix timestamp in milliseconds
			ts, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				return time.Unix(0, ts*int64(time.Millisecond)).In(f.location), nil
			}

		case "unix_us":
			// Unix timestamp in microseconds
			ts, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				return time.Unix(0, ts*int64(time.Microsecond)).In(f.location), nil
			}

		case "unix_ns":
			// Unix timestamp in nanoseconds
			ts, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				return time.Unix(0, ts).In(f.location), nil
			}

		default:
			// Try parsing with the format string
			parsed, err := time.Parse(format, value)
			if err == nil {
				return parsed.In(f.location), nil
			}

			// Try parsing in specified timezone
			parsed, err = time.ParseInLocation(format, value, f.location)
			if err == nil {
				return parsed, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("no format matched value: %s", value)
}

// parseUnixTimestamp parses a Unix timestamp
func (f *DateParseFilter) parseUnixTimestamp(ts int64) (time.Time, error) {
	// Determine if timestamp is in seconds, milliseconds, or microseconds
	// based on magnitude

	if ts < 0 {
		return time.Time{}, fmt.Errorf("negative timestamp not supported")
	}

	// Timestamps > 1e12 are likely in milliseconds or higher
	if ts > 1e12 {
		// Likely milliseconds
		if ts < 1e15 {
			return time.Unix(0, ts*int64(time.Millisecond)).In(f.location), nil
		}
		// Likely microseconds
		if ts < 1e18 {
			return time.Unix(0, ts*int64(time.Microsecond)).In(f.location), nil
		}
		// Likely nanoseconds
		return time.Unix(0, ts).In(f.location), nil
	}

	// Assume seconds
	return time.Unix(ts, 0).In(f.location), nil
}
