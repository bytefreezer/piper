package parsers

import (
	"fmt"
	"sync"
)

// DefaultRegistry is the default parser registry
type DefaultRegistry struct {
	factories map[string]ParserFactory
	mutex     sync.RWMutex
}

// NewRegistry creates a new parser registry
func NewRegistry() ParserRegistry {
	registry := &DefaultRegistry{
		factories: make(map[string]ParserFactory),
	}

	// Register built-in parsers
	// registry.Register("json-logs", NewJSONParser) // Removed - only NDJSON supported
	registry.Register("ndjson-logs", NewNDJSONParser)
	registry.Register("syslog-rfc3164", NewSyslogParser)
	registry.Register("plaintext", NewPlaintextParser)

	// Register format parsers for data pipeline
	registry.Register("ndjson", NewNDJSONParser)
	registry.Register("csv", NewCSVParser)
	registry.Register("tsv", NewTSVParser)
	registry.Register("apache", NewApacheLogParser)
	registry.Register("nginx", NewNginxLogParser)
	registry.Register("influx", NewInfluxLineProtocolParser)
	registry.Register("cef", NewCEFParser)
	registry.Register("sflow", NewSflowParser)
	registry.Register("raw", NewRawTextParser)

	return registry
}

// Register registers a parser factory
func (r *DefaultRegistry) Register(parserName string, factory ParserFactory) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.factories[parserName] = factory
}

// CreateParser creates a parser instance
func (r *DefaultRegistry) CreateParser(parserName string, config map[string]interface{}) (Parser, error) {
	r.mutex.RLock()
	factory, exists := r.factories[parserName]
	r.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("parser '%s' not found", parserName)
	}

	return factory(config)
}

// ListParsers returns a list of registered parser names
func (r *DefaultRegistry) ListParsers() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	parsers := make([]string, 0, len(r.factories))
	for name := range r.factories {
		parsers = append(parsers, name)
	}
	return parsers
}

// GetParserTypes returns a list of parser types
func (r *DefaultRegistry) GetParserTypes() []string {
	return []string{"ndjson", "syslog", "plaintext", "grok"}
}
