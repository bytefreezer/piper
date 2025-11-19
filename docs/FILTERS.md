# ByteFreezer Piper - Filter Reference Guide

This guide documents all available filters in ByteFreezer Piper with detailed usage examples.

## Table of Contents

### Event Filtering Filters
- [Include Filter](#include-filter) - Keep only matching events
- [Exclude Filter](#exclude-filter) - Drop matching events
- [Sample Filter](#sample-filter) - Random sampling
- [Drop Filter](#drop-filter) - Advanced conditional dropping

### Data Parsing Filters
- [Grok Filter](#grok-filter) - Pattern-based parsing
- [KV Filter](#kv-filter) - Key-value pair parsing
- [Date Parse Filter](#date-parse-filter) - Timestamp parsing
- [Split Filter](#split-filter) - Split events into multiple records

### Data Transformation Filters
- [Mutate Filter](#mutate-filter) - Field manipulation operations
- [Regex Replace Filter](#regex-replace-filter) - Regex find and replace
- [Add Field Filter](#add-field-filter) - Add fields to records
- [Remove Field Filter](#remove-field-filter) - Remove fields from records
- [Rename Field Filter](#rename-field-filter) - Rename fields

### Data Enrichment Filters
- [GeoIP Filter](#geoip-filter) - Geographic information from IPs
- [UserAgent Filter](#useragent-filter) - Parse user agent strings
- [DNS Filter](#dns-filter) - DNS lookups
- [Fingerprint Filter](#fingerprint-filter) - Event hashing

### Utility Filters
- [Conditional Filter](#conditional-filter) - Conditional processing
- [JSON Validate Filter](#json-validate-filter) - Validate JSON
- [JSON Flatten Filter](#json-flatten-filter) - Flatten nested JSON
- [Uppercase Keys Filter](#uppercase-keys-filter) - Convert keys to uppercase

---

## Event Filtering Filters

### Include Filter

**Purpose**: Keep only events that match specified conditions (drop all others)

**Type**: `include`

**Parameters**:
- `field` (string): Field name to check
- `equals` (any): Value to match exactly
- `contains` (string): Substring to match
- `matches` (string): Regex pattern to match
- `any_field` ([]string): Array of fields to check with OR logic
- `any_equals` (any): Value to match in any field
- `any_matches` (string): Regex pattern to match in any field

**Examples**:

```json
// Keep only error and critical events
{
  "type": "include",
  "config": {
    "field": "level",
    "matches": "^(error|critical)$"
  }
}

// Keep events where status is "success"
{
  "type": "include",
  "config": {
    "field": "status",
    "equals": "success"
  }
}

// Keep events with "error" in the message
{
  "type": "include",
  "config": {
    "field": "message",
    "contains": "error"
  }
}

// Keep events where any of multiple fields equals "critical"
{
  "type": "include",
  "config": {
    "any_field": ["level", "severity", "priority"],
    "any_equals": "critical"
  }
}

// Keep events matching a pattern in any field
{
  "type": "include",
  "config": {
    "any_field": ["user_agent", "client", "source"],
    "any_matches": "Chrome"
  }
}
```

**Use Cases**:
- Filter logs to show only errors
- Keep only specific HTTP status codes
- Focus on specific user actions
- Alert on critical events only

---

### Exclude Filter

**Purpose**: Drop events that match specified conditions (keep all others)

**Type**: `exclude`

**Parameters**:
- `field` (string): Field name to check
- `equals` (any): Value to match exactly
- `contains` (string): Substring to match
- `matches` (string): Regex pattern to match
- `any_field` ([]string): Array of fields to check with OR logic
- `any_equals` (any): Value to match in any field
- `any_matches` (string): Regex pattern to match in any field

**Examples**:

```json
// Exclude debug and trace level logs
{
  "type": "exclude",
  "config": {
    "field": "level",
    "matches": "^(debug|trace)$"
  }
}

// Exclude successful requests (keep only errors)
{
  "type": "exclude",
  "config": {
    "field": "status",
    "equals": "200"
  }
}

// Exclude bot traffic
{
  "type": "exclude",
  "config": {
    "any_field": ["user_agent", "client", "source"],
    "any_matches": "(bot|crawler|spider)"
  }
}

// Exclude internal traffic
{
  "type": "exclude",
  "config": {
    "field": "ip_address",
    "matches": "^(10\\.|192\\.168\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.)"
  }
}

// Exclude health check requests
{
  "type": "exclude",
  "config": {
    "field": "path",
    "contains": "/health"
  }
}
```

**Use Cases**:
- Remove bot traffic from analytics
- Filter out debug logs in production
- Exclude health checks from metrics
- Remove internal IP addresses

---

### Sample Filter

**Purpose**: Keep a random percentage of events (statistical sampling)

**Type**: `sample`

**Parameters**:
- `percentage` (float): Percentage of events to keep (0-100)
- `rate` (float): Rate of events to keep (0.0-1.0) - alternative to percentage
- `seed` (int64): Optional seed for deterministic sampling

**Examples**:

```json
// Keep 10% of events
{
  "type": "sample",
  "config": {
    "percentage": 10.0
  }
}

// Keep 25% using rate notation
{
  "type": "sample",
  "config": {
    "rate": 0.25
  }
}

// Deterministic sampling with seed (repeatable)
{
  "type": "sample",
  "config": {
    "percentage": 50.0,
    "seed": 12345
  }
}

// Keep 1% for high-volume sampling
{
  "type": "sample",
  "config": {
    "percentage": 1.0
  }
}

// Keep half the events
{
  "type": "sample",
  "config": {
    "rate": 0.5
  }
}
```

**Use Cases**:
- Reduce storage costs for high-volume data
- Create test datasets
- Sample traffic for analysis
- Reduce processing load
- A/B testing with deterministic seed

---

### Drop Filter

**Purpose**: Advanced conditional event dropping with multiple conditions

**Type**: `drop`

**Parameters**:
- `if_field` (string): Field to check for drop condition
- `equals` (any): Drop if field equals this value
- `not_equals` (any): Drop if field doesn't equal this value
- `contains` (string): Drop if field contains this substring
- `matches` (string): Drop if field matches this regex
- `unless_field` (string): Field to check for inverse condition
- `unless_equals` (any): Don't drop if this equals
- `unless_matches` (string): Don't drop if this matches regex
- `percentage` (float): Percentage of matching events to drop (0-100)
- `always_drop` (bool): Always drop (for testing)

**Examples**:

```json
// Drop all events where status is 200
{
  "type": "drop",
  "config": {
    "if_field": "status",
    "equals": "200"
  }
}

// Drop 90% of successful requests
{
  "type": "drop",
  "config": {
    "if_field": "status",
    "equals": "200",
    "percentage": 90
  }
}

// Drop events containing "debug" unless they also contain "error"
{
  "type": "drop",
  "config": {
    "if_field": "message",
    "contains": "debug",
    "unless_field": "message",
    "unless_matches": "error"
  }
}

// Drop events matching a pattern
{
  "type": "drop",
  "config": {
    "if_field": "user_agent",
    "matches": "(bot|crawler)"
  }
}
```

**Use Cases**:
- Complex conditional dropping
- Percentage-based sampling of specific events
- Drop with exceptions (unless conditions)

---

## Data Parsing Filters

### Grok Filter

**Purpose**: Parse unstructured log data using pattern matching

**Type**: `grok`

**Parameters**:
- `source_field` (string): Field to parse (default: "message")
- `pattern` (string): Single grok pattern
- `patterns` ([]string): Multiple patterns to try
- `break_on_match` (bool): Stop after first match (default: true)
- `keep_empty_captures` (bool): Keep empty captured fields (default: false)
- `named_captures_only` (bool): Only use named captures (default: true)
- `overwrite_keys` (bool): Overwrite existing keys (default: true)
- `target_field` (string): Store extracted fields in nested object
- `custom_patterns` (map): Define custom patterns

**Built-in Patterns** (partial list):
- `IP` - IP address
- `TIMESTAMP_ISO8601` - ISO timestamp
- `NUMBER` - Integer or float
- `WORD` - Single word
- `USERNAME` - Username
- `PATH` - File path
- `COMMONAPACHELOG` - Apache log format
- `COMBINEDAPACHELOG` - Combined Apache log
- And 50+ more...

**Examples**:

```json
// Parse Apache common log format
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{COMMONAPACHELOG}"
  }
}

// Parse custom log format
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} %{GREEDYDATA:message}"
  }
}

// Try multiple patterns
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "patterns": [
      "%{COMMONAPACHELOG}",
      "%{COMBINEDAPACHELOG}",
      "%{TIMESTAMP_ISO8601:timestamp} %{GREEDYDATA:message}"
    ],
    "break_on_match": true
  }
}

// Store extracted fields in nested object
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{IP:client} %{WORD:method} %{URIPATHPARAM:request}",
    "target_field": "parsed"
  }
}

// Custom patterns
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{MYAPP_LOG}",
    "custom_patterns": {
      "MYAPP_LOG": "%{TIMESTAMP_ISO8601:time} \\[%{WORD:thread}\\] %{LOGLEVEL:level} - %{GREEDYDATA:msg}"
    }
  }
}
```

**Use Cases**:
- Parse Apache/Nginx logs
- Extract structured data from unstructured logs
- Parse custom application logs
- Extract metrics from log messages

---

### KV Filter

**Purpose**: Parse key-value pairs from strings

**Type**: `kv`

**Parameters**:
- `source_field` (string): Field containing key-value pairs (default: "message")
- `target_field` (string): Field to store parsed pairs
- `field_split` (string): Separator between pairs (default: " ")
- `value_split` (string): Separator between key and value (default: "=")
- `prefix` (string): Prefix for extracted field names
- `include_keys` ([]string): Only extract these keys
- `exclude_keys` ([]string): Don't extract these keys
- `trim_key` (string): Characters to trim from keys
- `trim_value` (string): Characters to trim from values

**Examples**:

```json
// Basic key-value parsing
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "field_split": " ",
    "value_split": "="
  }
}
// Input: "user=john action=login status=success"
// Output: {"user": "john", "action": "login", "status": "success"}

// Parse with custom separators
{
  "type": "kv",
  "config": {
    "source_field": "params",
    "field_split": "&",
    "value_split": "="
  }
}
// Input: "user=john&action=login&status=success"

// Store in nested object with prefix
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "target_field": "params",
    "prefix": "param_"
  }
}
// Output: {"params": {"param_user": "john", "param_action": "login"}}

// Only extract specific keys
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "include_keys": ["user", "status"]
  }
}

// Trim quotes from values
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "trim_value": "\""
  }
}
// Input: 'user="john" status="success"'
// Output: {"user": "john", "status": "success"}
```

**Use Cases**:
- Parse URL query parameters
- Extract key-value pairs from logs
- Parse custom formatted data
- Extract metadata from messages

---

### Date Parse Filter

**Purpose**: Parse date/time strings into timestamps

**Type**: `date_parse`

**Parameters**:
- `source_field` (string): Field containing date string
- `target_field` (string): Field to store parsed timestamp (default: "@timestamp")
- `formats` ([]string): Date formats to try
- `timezone` (string): Timezone for parsing (default: "UTC")
- `locale` (string): Locale for parsing (default: "en")

**Supported Formats**:
- Go time format strings
- `unix` - Unix timestamp (seconds)
- `unix_ms` - Unix timestamp (milliseconds)
- `unix_us` - Unix timestamp (microseconds)

**Examples**:

```json
// Parse ISO8601 timestamp
{
  "type": "date_parse",
  "config": {
    "source_field": "timestamp",
    "target_field": "@timestamp",
    "formats": ["2006-01-02T15:04:05Z07:00"]
  }
}

// Try multiple formats
{
  "type": "date_parse",
  "config": {
    "source_field": "time",
    "formats": [
      "2006-01-02T15:04:05Z07:00",
      "2006-01-02 15:04:05",
      "02/Jan/2006:15:04:05 -0700"
    ]
  }
}

// Parse Unix timestamp
{
  "type": "date_parse",
  "config": {
    "source_field": "epoch",
    "formats": ["unix"]
  }
}

// Parse with timezone
{
  "type": "date_parse",
  "config": {
    "source_field": "time",
    "formats": ["2006-01-02 15:04:05"],
    "timezone": "America/New_York"
  }
}
```

**Use Cases**:
- Normalize timestamps from different sources
- Convert string dates to proper timestamps
- Parse Unix timestamps
- Handle multiple date formats

---

### Split Filter

**Purpose**: Split a single event into multiple events

**Type**: `split`

**Parameters**:
- `field` (string): Field to split on
- `terminator` (string): Delimiter to split on (for string fields)

**Examples**:

```json
// Split array into multiple events
{
  "type": "split",
  "config": {
    "field": "items"
  }
}
// Input: {"id": 1, "items": ["a", "b", "c"]}
// Output:
//   {"id": 1, "item": "a"}
//   {"id": 1, "item": "b"}
//   {"id": 1, "item": "c"}

// Split string by delimiter
{
  "type": "split",
  "config": {
    "field": "values",
    "terminator": ","
  }
}
// Input: {"id": 1, "values": "a,b,c"}
// Output:
//   {"id": 1, "value": "a"}
//   {"id": 1, "value": "b"}
//   {"id": 1, "value": "c"}
```

**Use Cases**:
- Process each element of an array separately
- Split comma-separated values
- Create separate events from batched data
- Explode nested arrays

---

## Data Transformation Filters

### Mutate Filter

**Purpose**: Advanced field manipulation operations

**Type**: `mutate`

**Operations**:
- `split` - Split string into array
- `join` - Join array into string
- `gsub` - Regex find and replace
- `convert` - Type conversion
- `lowercase` - Convert to lowercase
- `uppercase` - Convert to uppercase
- `strip` - Remove whitespace
- `merge` - Merge fields
- `update` - Update field value
- `replace` - Replace field value
- `rename` - Rename field
- `remove` - Remove field
- `copy` - Copy field

**Examples**:

```json
// Split a string into array
{
  "type": "mutate",
  "config": {
    "split": {
      "field": "tags",
      "separator": ","
    }
  }
}
// Input: {"tags": "web,api,error"}
// Output: {"tags": ["web", "api", "error"]}

// Join array into string
{
  "type": "mutate",
  "config": {
    "join": {
      "field": "tags",
      "separator": ", "
    }
  }
}

// Regex substitution
{
  "type": "mutate",
  "config": {
    "gsub": [
      {
        "field": "message",
        "pattern": "[0-9]+",
        "replacement": "X"
      }
    ]
  }
}

// Type conversion
{
  "type": "mutate",
  "config": {
    "convert": {
      "status": "integer",
      "duration": "float",
      "active": "boolean"
    }
  }
}

// Lowercase fields
{
  "type": "mutate",
  "config": {
    "lowercase": ["username", "email"]
  }
}

// Uppercase fields
{
  "type": "mutate",
  "config": {
    "uppercase": ["country_code", "state"]
  }
}

// Strip whitespace
{
  "type": "mutate",
  "config": {
    "strip": ["username", "email"]
  }
}

// Rename field
{
  "type": "mutate",
  "config": {
    "rename": {
      "old_name": "new_name"
    }
  }
}

// Remove fields
{
  "type": "mutate",
  "config": {
    "remove": ["temp", "debug_info"]
  }
}

// Copy field
{
  "type": "mutate",
  "config": {
    "copy": {
      "source": "destination"
    }
  }
}

// Multiple operations in one filter
{
  "type": "mutate",
  "config": {
    "lowercase": ["email"],
    "strip": ["username"],
    "convert": {
      "age": "integer"
    },
    "remove": ["temp"]
  }
}
```

**Use Cases**:
- Clean and normalize data
- Type conversions
- String manipulation
- Field reorganization

---

### Regex Replace Filter

**Purpose**: Perform regex-based find and replace on field values

**Type**: `regex_replace`

**Parameters**:
- `source_field` (string): Field to perform replacement on (default: "message")
- `target_field` (string): Field to write result (default: overwrites source)
- `pattern` (string): Regex pattern to match (required)
- `replacement` (string): Replacement string (default: "")
- `global` (bool): Replace all matches or just first (default: true)

**Examples**:

```json
// Redact credit card numbers
{
  "type": "regex_replace",
  "config": {
    "source_field": "message",
    "pattern": "\\d{4}-\\d{4}-\\d{4}-\\d{4}",
    "replacement": "XXXX-XXXX-XXXX-XXXX"
  }
}

// Remove all digits
{
  "type": "regex_replace",
  "config": {
    "source_field": "text",
    "pattern": "[0-9]+",
    "replacement": ""
  }
}

// Replace only first occurrence
{
  "type": "regex_replace",
  "config": {
    "source_field": "message",
    "pattern": "error",
    "replacement": "ERROR",
    "global": false
  }
}

// Write to different field
{
  "type": "regex_replace",
  "config": {
    "source_field": "raw_message",
    "target_field": "clean_message",
    "pattern": "[^a-zA-Z0-9 ]",
    "replacement": ""
  }
}
```

**Use Cases**:
- Redact sensitive information
- Clean and normalize text
- Pattern-based substitution
- Data masking

---

### Add Field Filter

**Purpose**: Add a field to records

**Type**: `add_field`

**Parameters**:
- `field` (string): Field name to add
- `value` (string): Field value (supports template variables)

**Template Variables**:
- `${timestamp}` - Current timestamp
- `${tenant_id}` - Current tenant ID
- `${dataset_id}` - Current dataset ID
- `${line_number}` - Current line number
- Custom variables from context

**Examples**:

```json
// Add static field
{
  "type": "add_field",
  "config": {
    "field": "environment",
    "value": "production"
  }
}

// Add field with template variable
{
  "type": "add_field",
  "config": {
    "field": "processed_by",
    "value": "piper-${tenant_id}"
  }
}

// Add timestamp
{
  "type": "add_field",
  "config": {
    "field": "processed_at",
    "value": "${timestamp}"
  }
}
```

**Use Cases**:
- Add metadata to events
- Tag events with environment
- Add processing information
- Include context variables

---

### Remove Field Filter

**Purpose**: Remove fields from records

**Type**: `remove_field`

**Parameters**:
- `field` (string): Single field to remove
- `fields` ([]string): Multiple fields to remove

**Examples**:

```json
// Remove single field
{
  "type": "remove_field",
  "config": {
    "field": "password"
  }
}

// Remove multiple fields
{
  "type": "remove_field",
  "config": {
    "fields": ["password", "secret", "token"]
  }
}
```

**Use Cases**:
- Remove sensitive fields
- Clean up temporary fields
- Reduce event size
- Remove debug information

---

### Rename Field Filter

**Purpose**: Rename a field in the record

**Type**: `rename_field`

**Parameters**:
- `from` (string): Source field name
- `to` (string): Target field name

**Examples**:

```json
// Rename single field
{
  "type": "rename_field",
  "config": {
    "from": "user_name",
    "to": "username"
  }
}

// Normalize field names
{
  "type": "rename_field",
  "config": {
    "from": "client_ip",
    "to": "ip_address"
  }
}
```

**Use Cases**:
- Normalize field names across sources
- Match schema requirements
- Rename for clarity

---

## Data Enrichment Filters

### GeoIP Filter

**Purpose**: Add geographic information for IP addresses

**Type**: `geoip`

**Parameters**:
- `source_field` (string): Field containing IP address
- `target_field` (string): Field to store GeoIP data
- `database` (string): Path to GeoIP database
- `fields` ([]string): Fields to extract

**Available Fields**:
- `country_name` - Country name
- `country_code` - ISO country code
- `city_name` - City name
- `latitude` - Latitude
- `longitude` - Longitude
- `timezone` - Timezone
- `continent` - Continent name
- `postal_code` - Postal/ZIP code

**Examples**:

```json
// Basic GeoIP enrichment
{
  "type": "geoip",
  "config": {
    "source_field": "ip_address",
    "target_field": "geoip"
  }
}
// Output: {
//   "geoip": {
//     "country_name": "United States",
//     "country_code": "US",
//     "city_name": "New York",
//     "latitude": 40.7128,
//     "longitude": -74.0060
//   }
// }

// Select specific fields
{
  "type": "geoip",
  "config": {
    "source_field": "client_ip",
    "target_field": "geo",
    "fields": ["country_name", "city_name"]
  }
}
```

**Use Cases**:
- Add location data to access logs
- Geographic analytics
- Geo-blocking decisions
- Location-based filtering

---

### UserAgent Filter

**Purpose**: Parse user agent strings into structured data

**Type**: `useragent`

**Parameters**:
- `source_field` (string): Field containing user agent string
- `target_field` (string): Field to store parsed data

**Extracted Fields**:
- `name` - Browser name
- `version` - Browser version
- `os` - Operating system
- `os_version` - OS version
- `device` - Device type

**Examples**:

```json
// Parse user agent
{
  "type": "useragent",
  "config": {
    "source_field": "user_agent",
    "target_field": "ua"
  }
}
// Input: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/96.0.4664.110"
// Output: {
//   "ua": {
//     "name": "Chrome",
//     "version": "96.0.4664.110",
//     "os": "Windows",
//     "os_version": "10",
//     "device": "Desktop"
//   }
// }
```

**Use Cases**:
- Browser analytics
- Device type detection
- Platform-specific processing
- Bot detection

---

### DNS Filter

**Purpose**: Perform DNS lookups to enrich events

**Type**: `dns`

**Parameters**:
- `resolve` ([]string): Fields containing IPs to resolve
- `action` (string): "replace" or "append"
- `target_field` (string): Field to store hostname
- `nameserver` ([]string): DNS servers to use
- `timeout` (int): Lookup timeout in seconds
- `cache_size` (int): Cache size
- `cache_ttl` (int): Cache TTL in seconds

**Examples**:

```json
// Resolve IP to hostname
{
  "type": "dns",
  "config": {
    "resolve": ["ip_address"],
    "action": "replace",
    "target_field": "hostname"
  }
}

// With custom nameservers and caching
{
  "type": "dns",
  "config": {
    "resolve": ["client_ip"],
    "nameserver": ["8.8.8.8", "8.8.4.4"],
    "timeout": 2,
    "cache_size": 1000,
    "cache_ttl": 3600
  }
}
```

**Use Cases**:
- Resolve IPs to hostnames
- Reverse DNS lookups
- Network troubleshooting
- Host identification

---

### Fingerprint Filter

**Purpose**: Generate event fingerprints/hashes for deduplication

**Type**: `fingerprint`

**Parameters**:
- `source_fields` ([]string): Fields to include in fingerprint
- `target_field` (string): Field to store fingerprint
- `method` (string): Hash algorithm (MD5, SHA1, SHA256, SHA512)
- `key_separator` (string): Separator for concatenating fields
- `base64encode` (bool): Base64 encode the hash

**Examples**:

```json
// Generate SHA256 fingerprint
{
  "type": "fingerprint",
  "config": {
    "source_fields": ["host", "message"],
    "target_field": "fingerprint",
    "method": "SHA256"
  }
}

// Custom separator and base64
{
  "type": "fingerprint",
  "config": {
    "source_fields": ["user", "action", "resource"],
    "target_field": "event_id",
    "method": "SHA256",
    "key_separator": "|",
    "base64encode": true
  }
}
```

**Use Cases**:
- Event deduplication
- Create unique event IDs
- Track unique events
- Cache keys

---

## Utility Filters

### Conditional Filter

**Purpose**: Filter records based on field conditions

**Type**: `conditional`

**Parameters**:
- `field` (string): Field to check
- `operator` (string): Comparison operator
- `value` (any): Value to compare against
- `action` (string): "keep" or "drop" (default: "keep")

**Operators**:
- `eq` - Equals
- `ne` - Not equals
- `gt` - Greater than
- `lt` - Less than
- `gte` - Greater than or equal
- `lte` - Less than or equal
- `contains` - Contains substring
- `not_contains` - Doesn't contain
- `exists` - Field exists
- `not_exists` - Field doesn't exist

**Examples**:

```json
// Keep only if status equals 200
{
  "type": "conditional",
  "config": {
    "field": "status",
    "operator": "eq",
    "value": "200",
    "action": "keep"
  }
}

// Drop if field exists
{
  "type": "conditional",
  "config": {
    "field": "debug",
    "operator": "exists",
    "action": "drop"
  }
}
```

**Use Cases**:
- Conditional event processing
- Field-based filtering
- Existence checks

---

### JSON Validate Filter

**Purpose**: Validate JSON content

**Type**: `json_validate`

**Parameters**:
- `source_field` (string): Field containing JSON (default: "message")
- `fail_on_invalid` (bool): Skip record if invalid (default: true)

**Examples**:

```json
// Validate JSON, skip if invalid
{
  "type": "json_validate",
  "config": {
    "source_field": "payload",
    "fail_on_invalid": true
  }
}

// Validate but continue processing
{
  "type": "json_validate",
  "config": {
    "source_field": "data",
    "fail_on_invalid": false
  }
}
```

---

### JSON Flatten Filter

**Purpose**: Flatten nested JSON objects

**Type**: `json_flatten`

**Parameters**:
- `source_field` (string): Field containing JSON
- `target_field` (string): Field to store flattened result
- `separator` (string): Separator for nested keys (default: ".")

**Examples**:

```json
// Flatten nested JSON
{
  "type": "json_flatten",
  "config": {
    "source_field": "data",
    "target_field": "flat",
    "separator": "."
  }
}
// Input: {"data": {"user": {"name": "John", "age": 30}}}
// Output: {"flat": {"user.name": "John", "user.age": 30}}
```

---

### Uppercase Keys Filter

**Purpose**: Convert all keys to uppercase

**Type**: `uppercase_keys`

**Parameters**:
- `source_field` (array or string, optional): Array of field names to operate on, single field name, "*" for all fields, or empty/omitted for entire record
- `recursive` (bool): Recursively uppercase nested objects (default: true)

**Examples**:

```json
// Uppercase all keys in the entire record (most common use case)
{
  "type": "uppercase_keys",
  "config": {
    "recursive": true
  }
}

// Uppercase keys in a specific nested field
{
  "type": "uppercase_keys",
  "config": {
    "source_field": "metadata",
    "recursive": true
  }
}

// Uppercase keys in multiple specific fields
{
  "type": "uppercase_keys",
  "config": {
    "source_field": ["metadata", "context"],
    "recursive": true
  }
}

// Apply to all fields (same as omitting source_field)
{
  "type": "uppercase_keys",
  "config": {
    "source_field": "*",
    "recursive": true
  }
}

// Uppercase only top-level keys (non-recursive)
{
  "type": "uppercase_keys",
  "config": {
    "recursive": false
  }
}
```

---

## Filter Order and Performance

**Best Practices**:
1. Place filtering filters (include/exclude/drop/sample) early to reduce processing
2. Parse before enrichment (grok/kv before geoip/useragent)
3. Transform after parsing (mutate after grok)
4. Remove sensitive fields before storage
5. Add metadata fields last

**Example Optimal Order**:
```json
{
  "filters": [
    {"type": "sample", "config": {"percentage": 50}},
    {"type": "exclude", "config": {"field": "level", "equals": "debug"}},
    {"type": "grok", "config": {"pattern": "%{COMMONAPACHELOG}"}},
    {"type": "date_parse", "config": {"source_field": "timestamp"}},
    {"type": "geoip", "config": {"source_field": "client"}},
    {"type": "useragent", "config": {"source_field": "user_agent"}},
    {"type": "mutate", "config": {"remove": ["temp"]}},
    {"type": "add_field", "config": {"field": "processed", "value": "true"}}
  ]
}
```
