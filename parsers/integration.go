package parsers

import (
	"context"
	"fmt"

	"github.com/bytefreezer/piper/pipeline"
)

// ParseFilter is a pipeline filter that can parse data using registered parsers
type ParseFilter struct {
	config    ParseConfig
	parser    Parser
	parserReg ParserRegistry
}

// NewParseFilter creates a new parse filter
func NewParseFilter(config map[string]interface{}, parserRegistry ParserRegistry) (pipeline.Filter, error) {
	parseConfig := ParseConfig{
		SourceField:    "message",
		OnParseFailure: "tag",
		PreserveRaw:    false,
		AutoSelect:     false,
	}

	// Extract configuration
	if parserName, ok := config["parser_name"].(string); ok {
		parseConfig.ParserName = parserName
	}
	if sourceField, ok := config["source_field"].(string); ok {
		parseConfig.SourceField = sourceField
	}
	if targetField, ok := config["target_field"].(string); ok {
		parseConfig.TargetField = targetField
	}
	if onFailure, ok := config["on_parse_failure"].(string); ok {
		parseConfig.OnParseFailure = onFailure
	}
	if preserveRaw, ok := config["preserve_raw"].(bool); ok {
		parseConfig.PreserveRaw = preserveRaw
	}
	if autoSelect, ok := config["auto_select"].(bool); ok {
		parseConfig.AutoSelect = autoSelect
	}
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		parseConfig.Settings = settings
	}

	filter := &ParseFilter{
		config:    parseConfig,
		parserReg: parserRegistry,
	}

	// Create parser if parser name is specified
	if parseConfig.ParserName != "" {
		parser, err := parserRegistry.CreateParser(parseConfig.ParserName, parseConfig.Settings)
		if err != nil {
			return nil, fmt.Errorf("failed to create parser %s: %w", parseConfig.ParserName, err)
		}
		filter.parser = parser
	}

	return filter, nil
}

// Type returns the filter type
func (f *ParseFilter) Type() string {
	return "parse"
}

// Validate validates the filter configuration
func (f *ParseFilter) Validate(config map[string]interface{}) error {
	if parserName, ok := config["parser_name"].(string); ok && parserName != "" {
		// Validate that the parser exists
		parsers := f.parserReg.ListParsers()
		found := false
		for _, p := range parsers {
			if p == parserName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("parser '%s' not found", parserName)
		}
	} else if autoSelect, ok := config["auto_select"].(bool); !ok || !autoSelect {
		return fmt.Errorf("either 'parser_name' or 'auto_select' must be specified")
	}

	return nil
}

// Apply applies the parse filter to a record
func (f *ParseFilter) Apply(ctx *pipeline.FilterContext, record map[string]interface{}) (*pipeline.FilterResult, error) {
	// Get source data
	sourceData, exists := record[f.config.SourceField]
	if !exists {
		switch f.config.OnParseFailure {
		case "skip":
			return &pipeline.FilterResult{Record: record, Skip: true}, nil
		case "tag":
			record["_parse_failure"] = fmt.Sprintf("source field '%s' not found", f.config.SourceField)
			return &pipeline.FilterResult{Record: record, Applied: true}, nil
		default: // pass_through
			return &pipeline.FilterResult{Record: record, Applied: false}, nil
		}
	}

	// Convert to bytes for parsing
	var data []byte
	switch v := sourceData.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		switch f.config.OnParseFailure {
		case "skip":
			return &pipeline.FilterResult{Record: record, Skip: true}, nil
		case "tag":
			record["_parse_failure"] = fmt.Sprintf("source field '%s' is not string or bytes", f.config.SourceField)
			return &pipeline.FilterResult{Record: record, Applied: true}, nil
		default: // pass_through
			return &pipeline.FilterResult{Record: record, Applied: false}, nil
		}
	}

	// Auto-select parser if needed
	parser := f.parser
	if parser == nil && f.config.AutoSelect {
		parser = f.autoSelectParser(data)
	}

	if parser == nil {
		switch f.config.OnParseFailure {
		case "skip":
			return &pipeline.FilterResult{Record: record, Skip: true}, nil
		case "tag":
			record["_parse_failure"] = "no suitable parser found"
			return &pipeline.FilterResult{Record: record, Applied: true}, nil
		default: // pass_through
			return &pipeline.FilterResult{Record: record, Applied: false}, nil
		}
	}

	// Parse the data
	parsed, err := parser.Parse(context.Background(), data)
	if err != nil {
		switch f.config.OnParseFailure {
		case "skip":
			return &pipeline.FilterResult{Record: record, Skip: true}, nil
		case "tag":
			record["_parse_failure"] = err.Error()
			return &pipeline.FilterResult{Record: record, Applied: true}, nil
		default: // pass_through
			return &pipeline.FilterResult{Record: record, Applied: false}, nil
		}
	}

	// Merge parsed data into record
	if f.config.TargetField != "" {
		record[f.config.TargetField] = parsed
	} else {
		// Merge parsed fields directly into record
		for k, v := range parsed {
			record[k] = v
		}
	}

	// Preserve raw data if requested
	if f.config.PreserveRaw {
		record["_raw"] = string(data)
	}

	return &pipeline.FilterResult{Record: record, Applied: true}, nil
}

// autoSelectParser attempts to automatically select the best parser for the data
func (f *ParseFilter) autoSelectParser(data []byte) Parser {
	// Try JSON first
	if parser, err := f.parserReg.CreateParser("json-logs", nil); err == nil {
		if _, err := parser.Parse(context.Background(), data); err == nil {
			return parser
		}
	}

	// Try NDJSON
	if parser, err := f.parserReg.CreateParser("ndjson-logs", nil); err == nil {
		if _, err := parser.Parse(context.Background(), data); err == nil {
			return parser
		}
	}

	// Try syslog
	if parser, err := f.parserReg.CreateParser("syslog-rfc3164", nil); err == nil {
		if _, err := parser.Parse(context.Background(), data); err == nil {
			return parser
		}
	}

	// Fall back to plaintext
	if parser, err := f.parserReg.CreateParser("plaintext", nil); err == nil {
		return parser
	}

	return nil
}
