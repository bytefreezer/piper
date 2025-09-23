package parsers

import (
	"context"
	"time"
)

// Parser represents a data parser that can process log entries
type Parser interface {
	// Parse parses a single log line/record
	Parse(ctx context.Context, data []byte) (map[string]interface{}, error)

	// Name returns the parser name
	Name() string

	// Type returns the parser type (json, syslog, grok, etc.)
	Type() string

	// Configure configures the parser with given settings
	Configure(config map[string]interface{}) error
}

// ParserFactory creates parser instances
type ParserFactory func(config map[string]interface{}) (Parser, error)

// ParserRegistry manages available parsers
type ParserRegistry interface {
	Register(parserName string, factory ParserFactory)
	CreateParser(parserName string, config map[string]interface{}) (Parser, error)
	ListParsers() []string
	GetParserTypes() []string
}

// ParseResult represents the result of a parsing operation
type ParseResult struct {
	Record    map[string]interface{} `json:"record"`
	Success   bool                   `json:"success"`
	Error     error                  `json:"error,omitempty"`
	ParseTime time.Duration          `json:"parse_time"`
	Parser    string                 `json:"parser"`
}

// ParseConfig represents parser configuration
type ParseConfig struct {
	ParserName       string                 `json:"parser_name"`
	SourceField      string                 `json:"source_field"`
	TargetField      string                 `json:"target_field,omitempty"`
	OnParseFailure   string                 `json:"on_parse_failure"` // "skip", "tag", "pass_through"
	PreserveRaw      bool                   `json:"preserve_raw"`
	Settings         map[string]interface{} `json:"settings,omitempty"`
	AutoSelect       bool                   `json:"auto_select"`
}