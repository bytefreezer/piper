package parsers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// JSONParser parses JSON log entries
type JSONParser struct {
	name string
}

func NewJSONParser(config map[string]interface{}) (Parser, error) {
	return &JSONParser{
		name: "json-logs",
	}, nil
}

func (p *JSONParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}

func (p *JSONParser) Name() string {
	return p.name
}

func (p *JSONParser) Type() string {
	return "json"
}

func (p *JSONParser) Configure(config map[string]interface{}) error {
	if name, ok := config["name"].(string); ok {
		p.name = name
	}
	return nil
}

// NDJSONParser parses NDJSON (newline-delimited JSON) entries
type NDJSONParser struct {
	name string
}

func NewNDJSONParser(config map[string]interface{}) (Parser, error) {
	return &NDJSONParser{
		name: "ndjson-logs",
	}, nil
}

func (p *NDJSONParser) Parse(ctx context.Context, data []byte) (map[string]interface{}, error) {
	// Remove trailing newline if present
	data = []byte(strings.TrimSuffix(string(data), "\n"))

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse NDJSON: %w", err)
	}
	return result, nil
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
