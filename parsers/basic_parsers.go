// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package parsers

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NDJSONParser parses NDJSON (newline-delimited JSON) entries with robust handling of malformed files
type NDJSONParser struct {
	name string
}

func NewNDJSONParser(config map[string]interface{}) (Parser, error) {
	return &NDJSONParser{
		name: "ndjson-logs",
	}, nil
}

func (p *NDJSONParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	// For single-line parsing (called from format processor), try to reconstruct complete JSON
	jsonStr := string(data)

	// First attempt: try parsing as-is (for well-formed NDJSON)
	var result map[string]interface{}
	if err := sonic.Unmarshal([]byte(jsonStr), &result); err == nil {
		return result, nil
	}

	// Second attempt: try to fix malformed JSON by reconstructing complete objects
	// Since proxy is fixing format upstream, be permissive with malformed documents
	fixedJSON, err := p.reconstructJSONObject(jsonStr)
	if err != nil {
		// Return raw data as valid object - no strict validation
		return map[string]interface{}{
			"message":   jsonStr,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"_source":   "malformed_ndjson",
		}, nil
	}

	if err := sonic.Unmarshal([]byte(fixedJSON), &result); err != nil {
		// Return raw data as valid object - no strict validation
		return map[string]interface{}{
			"message":   jsonStr,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"_source":   "malformed_ndjson",
		}, nil
	}

	return result, nil
}

// reconstructJSONObject attempts to reconstruct a complete JSON object from potentially malformed input
func (p *NDJSONParser) reconstructJSONObject(input string) (string, error) {
	input = strings.TrimSpace(input)

	// If it already looks like complete JSON, return as-is
	if p.isCompleteJSON(input) {
		return input, nil
	}

	// Try to fix common issues with embedded newlines
	fixed := input

	// Remove internal newlines that break JSON structure
	// This handles cases where newlines are embedded within string values
	fixed = strings.ReplaceAll(fixed, "\n", "\\n")
	fixed = strings.ReplaceAll(fixed, "\r", "\\r")
	fixed = strings.ReplaceAll(fixed, "\t", "\\t")

	// Try to balance braces if missing closing brace
	if !p.isCompleteJSON(fixed) {
		openBraces := strings.Count(fixed, "{") - strings.Count(fixed, "}")
		for i := 0; i < openBraces; i++ {
			fixed += "}"
		}
	}

	// Validate the reconstructed JSON
	if p.isCompleteJSON(fixed) {
		return fixed, nil
	}

	return "", fmt.Errorf("unable to reconstruct valid JSON from: %s", input[:min(50, len(input))])
}

// isCompleteJSON checks if a string contains valid, complete JSON
func (p *NDJSONParser) isCompleteJSON(s string) bool {
	var temp map[string]interface{}
	return sonic.Unmarshal([]byte(s), &temp) == nil
}

func (p *NDJSONParser) Name() string {
	return p.name
}

func (p *NDJSONParser) Type() string {
	return "ndjson"
}

func (p *NDJSONParser) Configure(config map[string]interface{}) error {
	if name, ok := config["name"].(string); ok {
		p.name = name
	}
	return nil
}

// SyslogParser parses RFC3164 syslog entries
type SyslogParser struct {
	name  string
	regex *regexp.Regexp
}

func NewSyslogParser(config map[string]interface{}) (Parser, error) {
	// RFC3164 syslog pattern
	pattern := `^<(\d+)>(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(.+)$`
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile syslog regex: %w", err)
	}

	return &SyslogParser{
		name:  "syslog-rfc3164",
		regex: regex,
	}, nil
}

func (p *SyslogParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	line := string(data)
	matches := p.regex.FindStringSubmatch(line)
	if len(matches) != 5 {
		return nil, fmt.Errorf("line does not match syslog format")
	}

	priority, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid priority: %w", err)
	}

	facility := priority / 8
	severity := priority % 8

	result := map[string]interface{}{
		"priority":  priority,
		"facility":  facility,
		"severity":  severity,
		"timestamp": matches[2],
		"hostname":  matches[3],
		"message":   matches[4],
		"raw":       line,
	}

	return result, nil
}

func (p *SyslogParser) Name() string {
	return p.name
}

func (p *SyslogParser) Type() string {
	return "syslog"
}

func (p *SyslogParser) Configure(config map[string]interface{}) error {
	if name, ok := config["name"].(string); ok {
		p.name = name
	}
	return nil
}

// PlaintextParser parses plain text entries
type PlaintextParser struct {
	name string
}

func NewPlaintextParser(config map[string]interface{}) (Parser, error) {
	return &PlaintextParser{
		name: "plaintext",
	}, nil
}

func (p *PlaintextParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"message":   string(data),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return result, nil
}

func (p *PlaintextParser) Name() string {
	return p.name
}

func (p *PlaintextParser) Type() string {
	return "plaintext"
}

func (p *PlaintextParser) Configure(config map[string]interface{}) error {
	if name, ok := config["name"].(string); ok {
		p.name = name
	}
	return nil
}
