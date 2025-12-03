// Licensed under Elastic License 2.0
// See LICENSE.txt for details

package parsers

// ParserSettings represents parser configuration settings
type ParserSettings struct {
	MaxLineLength   int    `koanf:"max_line_length"`
	BufferSize      int    `koanf:"buffer_size"`
	ParseTimeout    string `koanf:"parse_timeout"`
	MaxParseErrors  int    `koanf:"max_parse_errors"`
	GrokPatternsDir string `koanf:"grok_patterns_dir"`
}

// ParserConfig represents the overall parser configuration
type ParserConfig struct {
	Enabled           bool                     `koanf:"enabled"`
	LoadDefaults      bool                     `koanf:"load_defaults"`
	Settings          ParserSettings           `koanf:"settings"`
	AutoTenantParsers bool                     `koanf:"auto_tenant_parsers"`
	FormatDefaults    map[string][]ParseConfig `koanf:"format_defaults"`
}

// GetDefaultParserConfig returns default parser configuration
func GetDefaultParserConfig() ParserConfig {
	return ParserConfig{
		Enabled:      true,
		LoadDefaults: true,
		Settings: ParserSettings{
			MaxLineLength:   65536,
			BufferSize:      4096,
			ParseTimeout:    "30s",
			MaxParseErrors:  100,
			GrokPatternsDir: "/opt/grok-patterns",
		},
		AutoTenantParsers: true,
		FormatDefaults: map[string][]ParseConfig{
			"plaintext": {
				{
					ParserName:     "plaintext",
					SourceField:    "message",
					OnParseFailure: "tag",
					PreserveRaw:    false,
					AutoSelect:     true,
				},
			},
			"syslog": {
				{
					ParserName:     "syslog-rfc3164",
					SourceField:    "message",
					OnParseFailure: "tag",
					PreserveRaw:    false,
				},
			},
			"ndjson": {
				{
					ParserName:     "ndjson-logs",
					SourceField:    "message",
					OnParseFailure: "pass_through",
					PreserveRaw:    false,
				},
			},
			"json": {
				{
					ParserName:     "json-logs",
					SourceField:    "message",
					OnParseFailure: "pass_through",
					PreserveRaw:    false,
				},
			},
		},
	}
}
