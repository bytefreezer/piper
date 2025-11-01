package pipeline

import (
	"fmt"
	"regexp"
	"strings"
)

// GrokPatternLibrary contains common Grok patterns for log parsing
type GrokPatternLibrary struct {
	patterns map[string]string
	compiled map[string]*regexp.Regexp
}

// NewGrokPatternLibrary creates a new pattern library with built-in patterns
func NewGrokPatternLibrary() *GrokPatternLibrary {
	lib := &GrokPatternLibrary{
		patterns: make(map[string]string),
		compiled: make(map[string]*regexp.Regexp),
	}
	lib.loadBuiltInPatterns()
	return lib
}

// loadBuiltInPatterns loads the standard Grok patterns
func (g *GrokPatternLibrary) loadBuiltInPatterns() {
	// Base patterns
	g.patterns["USERNAME"] = `[a-zA-Z0-9._-]+`
	g.patterns["USER"] = `%{USERNAME}`
	g.patterns["EMAILLOCALPART"] = `[a-zA-Z0-9_.+-]+`
	g.patterns["EMAILADDRESS"] = `%{EMAILLOCALPART}@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	g.patterns["INT"] = `(?:[+-]?(?:[0-9]+))`
	g.patterns["BASE10NUM"] = `(?:[+-]?(?:[0-9]+(?:\.[0-9]+)?))`
	g.patterns["NUMBER"] = `%{BASE10NUM}`
	g.patterns["BASE16NUM"] = `(?:0[xX]?[0-9a-fA-F]+)`
	g.patterns["WORD"] = `\b\w+\b`
	g.patterns["NOTSPACE"] = `\S+`
	g.patterns["SPACE"] = `\s*`
	g.patterns["DATA"] = `.*?`
	g.patterns["GREEDYDATA"] = `.*`
	g.patterns["QUOTEDSTRING"] = `"(?:[^"\\]|\\.)*"`
	g.patterns["UUID"] = `[A-Fa-f0-9]{8}-(?:[A-Fa-f0-9]{4}-){3}[A-Fa-f0-9]{12}`

	// Network patterns
	g.patterns["CISCOMAC"] = `(?:[A-Fa-f0-9]{4}\.){2}[A-Fa-f0-9]{4}`
	g.patterns["WINDOWSMAC"] = `(?:[A-Fa-f0-9]{2}-){5}[A-Fa-f0-9]{2}`
	g.patterns["COMMONMAC"] = `(?:[A-Fa-f0-9]{2}:){5}[A-Fa-f0-9]{2}`
	g.patterns["MAC"] = `(?:%{CISCOMAC}|%{WINDOWSMAC}|%{COMMONMAC})`
	g.patterns["IPV6"] = `(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}`
	g.patterns["IPV4"] = `(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`
	g.patterns["IP"] = `(?:%{IPV4}|%{IPV6})`
	g.patterns["HOSTNAME"] = `\b[0-9A-Za-z][0-9A-Za-z-]{0,62}(?:\.[0-9A-Za-z][0-9A-Za-z-]{0,62})*\.?`
	g.patterns["HOST"] = `%{HOSTNAME}`
	g.patterns["IPORHOST"] = `(?:%{IP}|%{HOSTNAME})`

	// Path patterns
	g.patterns["PATH"] = `(?:/[^/\s]*)+`
	g.patterns["UNIXPATH"] = `(?:/[A-Za-z0-9_.-]+)+`
	g.patterns["TTY"] = `/dev/pts/\d+`
	g.patterns["WINPATH"] = `(?:[A-Za-z]:|\\)(?:\\[^\\?*]*)+`
	g.patterns["URIPROTO"] = `[A-Za-z]+(?:\+[A-Za-z+]+)?`
	g.patterns["URIHOST"] = `%{IPORHOST}(?::%{INT})?`
	g.patterns["URIPATH"] = `(?:/[A-Za-z0-9$.+!*'(){},~:;=@#%_\-]*)*`
	g.patterns["URIPARAM"] = `\?[A-Za-z0-9$.+!*'|(){},~@#%&/=:;_?\-\[\]<>]*`
	g.patterns["URIPATHPARAM"] = `%{URIPATH}(?:%{URIPARAM})?`
	g.patterns["URI"] = `%{URIPROTO}://(?:%{USER}:[^@]*@)?(?:%{URIHOST})?(?:%{URIPATHPARAM})?`

	// Date patterns
	g.patterns["MONTH"] = `\b(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:tember)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)\b`
	g.patterns["MONTHNUM"] = `(?:0?[1-9]|1[0-2])`
	g.patterns["MONTHDAY"] = `(?:(?:0[1-9])|(?:[12][0-9])|(?:3[01])|[1-9])`
	g.patterns["DAY"] = `(?:Mon(?:day)?|Tue(?:sday)?|Wed(?:nesday)?|Thu(?:rsday)?|Fri(?:day)?|Sat(?:urday)?|Sun(?:day)?)`
	g.patterns["YEAR"] = `(?:\d\d){1,2}`
	g.patterns["HOUR"] = `(?:2[0123]|[01]?[0-9])`
	g.patterns["MINUTE"] = `(?:[0-5][0-9])`
	g.patterns["SECOND"] = `(?:(?:[0-5]?[0-9]|60)(?:[:.,][0-9]+)?)`
	g.patterns["TIME"] = `%{HOUR}:%{MINUTE}(?::%{SECOND})`
	g.patterns["ISO8601_TIMEZONE"] = `(?:Z|[+-]%{HOUR}(?::?%{MINUTE}))`
	g.patterns["ISO8601_SECOND"] = `(?:%{SECOND}|60)`
	g.patterns["TIMESTAMP_ISO8601"] = `%{YEAR}-%{MONTHNUM}-%{MONTHDAY}[T ]%{HOUR}:?%{MINUTE}(?::?%{SECOND})?%{ISO8601_TIMEZONE}?`
	g.patterns["DATE_US"] = `%{MONTHNUM}[/-]%{MONTHDAY}[/-]%{YEAR}`
	g.patterns["DATE_EU"] = `%{MONTHDAY}[./-]%{MONTHNUM}[./-]%{YEAR}`
	g.patterns["DATESTAMP"] = `%{DATE_US}|%{DATE_EU}`
	g.patterns["TZ"] = `(?:[PMCE][SD]T|UTC)`
	g.patterns["DATESTAMP_RFC822"] = `%{DAY} %{MONTH} %{MONTHDAY} %{YEAR}`
	g.patterns["DATESTAMP_RFC2822"] = `%{DAY}, %{MONTHDAY} %{MONTH} %{YEAR} %{TIME} %{ISO8601_TIMEZONE}`
	g.patterns["DATESTAMP_OTHER"] = `%{DAY} %{MONTH} %{MONTHDAY} %{TIME} %{TZ} %{YEAR}`
	g.patterns["DATESTAMP_EVENTLOG"] = `%{YEAR}%{MONTHNUM}%{MONTHDAY}%{HOUR}%{MINUTE}%{SECOND}`

	// Syslog patterns
	g.patterns["SYSLOGTIMESTAMP"] = `%{MONTH} +%{MONTHDAY} %{TIME}`
	g.patterns["PROG"] = `[\x21-\x5a\x5c\x5e-\x7e]+`
	g.patterns["SYSLOGPROG"] = `%{PROG:program}(?:\[%{INT:pid}\])?`
	g.patterns["SYSLOGHOST"] = `%{IPORHOST}`
	g.patterns["SYSLOGFACILITY"] = `<%{INT:facility}.%{INT:priority}>`
	g.patterns["HTTPDATE"] = `%{MONTHDAY}/%{MONTH}/%{YEAR}:%{TIME} %{INT}`

	// Log level patterns
	g.patterns["LOGLEVEL"] = `(?:[Aa]lert|ALERT|[Tt]race|TRACE|[Dd]ebug|DEBUG|[Nn]otice|NOTICE|[Ii]nfo|INFO|[Ww]arn?(?:ing)?|WARN?(?:ING)?|[Ee]rr?(?:or)?|ERR?(?:OR)?|[Cc]rit?(?:ical)?|CRIT?(?:ICAL)?|[Ff]atal|FATAL|[Ss]evere|SEVERE|EMERG(?:ENCY)?|[Ee]merg(?:ency)?)`

	// HTTP patterns
	g.patterns["HTTPMETHOD"] = `\b(?:GET|HEAD|POST|PUT|DELETE|CONNECT|OPTIONS|TRACE|PATCH)\b`
	g.patterns["HTTPVERSION"] = `HTTP/%{NUMBER:http_version}`

	// Common log formats
	g.patterns["COMMONAPACHELOG"] = `%{IP:client} %{USER:ident} %{USER:auth} \[%{HTTPDATE:timestamp}\] "(?:%{HTTPMETHOD:method} %{NOTSPACE:request}(?: %{HTTPVERSION})?|%{DATA:rawrequest})" %{NUMBER:status} (?:%{NUMBER:bytes}|-)`
	g.patterns["COMBINEDAPACHELOG"] = `%{COMMONAPACHELOG} %{QUOTEDSTRING:referrer} %{QUOTEDSTRING:agent}`

	// Custom ByteFreezer patterns
	g.patterns["BYTEFREEZER_TIMESTAMP"] = `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{3,9})?Z?`
	g.patterns["BYTEFREEZER_UUID"] = `[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`
	g.patterns["BYTEFREEZER_LOGLEVEL"] = `(?:DEBUG|INFO|WARN|ERROR|FATAL)`
}

// AddPattern adds a custom pattern to the library
func (g *GrokPatternLibrary) AddPattern(name, pattern string) {
	g.patterns[name] = pattern
	// Clear compiled cache for this pattern
	delete(g.compiled, name)
}

// CompilePattern compiles a Grok pattern into a regex, resolving pattern references
func (g *GrokPatternLibrary) CompilePattern(pattern string) (*regexp.Regexp, map[int]string, error) {
	// Resolve pattern references recursively
	resolved, err := g.resolvePattern(pattern, make(map[string]bool))
	if err != nil {
		return nil, nil, err
	}

	// Extract named captures for field mapping
	fieldMap := make(map[int]string)
	fieldIndex := 1 // Regex group indices start at 1

	// Convert Grok syntax %{PATTERN:fieldname} to named groups (?P<fieldname>pattern)
	re := regexp.MustCompile(`%\{([A-Z0-9_]+)(?::([a-z0-9_]+))?\}`)
	finalPattern := re.ReplaceAllStringFunc(resolved, func(match string) string {
		parts := re.FindStringSubmatch(match)
		patternName := parts[1]
		fieldName := parts[2]

		// Get the actual pattern
		actualPattern, exists := g.patterns[patternName]
		if !exists {
			actualPattern = `\S+` // Default to non-whitespace if pattern not found
		}

		// If field name is specified, create named capture group
		if fieldName != "" {
			fieldMap[fieldIndex] = fieldName
			fieldIndex++
			return fmt.Sprintf("(?P<%s>%s)", fieldName, actualPattern)
		}

		// No field name, just use the pattern
		return actualPattern
	})

	// Compile final regex
	compiled, err := regexp.Compile(finalPattern)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile pattern: %w", err)
	}

	return compiled, fieldMap, nil
}

// resolvePattern recursively resolves pattern references
func (g *GrokPatternLibrary) resolvePattern(pattern string, visited map[string]bool) (string, error) {
	// Detect circular references
	if visited[pattern] {
		return "", fmt.Errorf("circular pattern reference detected")
	}
	visited[pattern] = true

	result := pattern
	re := regexp.MustCompile(`%\{([A-Z0-9_]+)(?::[a-z0-9_]+)?\}`)

	// Keep resolving until no more pattern references
	maxIterations := 100 // Prevent infinite loops
	for i := 0; i < maxIterations; i++ {
		matches := re.FindAllStringSubmatch(result, -1)
		if len(matches) == 0 {
			break
		}

		changed := false
		for _, match := range matches {
			patternName := match[1]
			if patternDef, exists := g.patterns[patternName]; exists {
				// Replace pattern reference with actual pattern
				old := fmt.Sprintf("%%{%s}", patternName)
				result = strings.ReplaceAll(result, old, patternDef)
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	return result, nil
}

// GetPattern retrieves a pattern by name
func (g *GrokPatternLibrary) GetPattern(name string) (string, bool) {
	pattern, exists := g.patterns[name]
	return pattern, exists
}

// ListPatterns returns all available pattern names
func (g *GrokPatternLibrary) ListPatterns() []string {
	names := make([]string, 0, len(g.patterns))
	for name := range g.patterns {
		names = append(names, name)
	}
	return names
}
