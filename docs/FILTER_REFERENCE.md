# ByteFreezer Piper - Complete Filter Reference

Quick reference for all available filters with configuration parameters and examples.

---

## 1. Include Filter

**Type**: `include`
**Purpose**: Keep only events matching conditions (drop all others)

**Config Parameters**:
- `field` (string) - Field name to check
- `equals` (any) - Value to match exactly
- `contains` (string) - Substring to match
- `matches` (string) - Regex pattern to match
- `any_field` ([]string) - Array of fields to check with OR logic
- `any_equals` (any) - Value to match in any field
- `any_matches` (string) - Regex pattern to match in any field

**Examples**:
```json
{
  "type": "include",
  "config": {
    "field": "level",
    "equals": "error"
  },
  "enabled": true
}

{
  "type": "include",
  "config": {
    "field": "message",
    "contains": "failed"
  },
  "enabled": true
}

{
  "type": "include",
  "config": {
    "field": "status",
    "matches": "^(error|critical)$"
  },
  "enabled": true
}
```

---

## 2. Exclude Filter

**Type**: `exclude`
**Purpose**: Drop events matching conditions

**Config Parameters**:
- `field` (string) - Field name to check
- `equals` (any) - Value to match exactly
- `contains` (string) - Substring to match
- `matches` (string) - Regex pattern to match
- `any_field` ([]string) - Array of fields to check with OR logic
- `any_equals` (any) - Value to match in any field
- `any_matches` (string) - Regex pattern to match in any field

**Examples**:
```json
{
  "type": "exclude",
  "config": {
    "field": "level",
    "equals": "debug"
  },
  "enabled": true
}

{
  "type": "exclude",
  "config": {
    "field": "message",
    "contains": "health check"
  },
  "enabled": true
}

{
  "type": "exclude",
  "config": {
    "any_field": ["user_agent", "client"],
    "any_matches": "bot|crawler"
  },
  "enabled": true
}
```

---

## 3. Sample Filter

**Type**: `sample`
**Purpose**: Keep only a percentage of events (random sampling)

**Config Parameters**:
- `percentage` (float) - Percentage of events to keep (0-100)

**Examples**:
```json
{
  "type": "sample",
  "config": {
    "percentage": 10
  },
  "enabled": true
}

{
  "type": "sample",
  "config": {
    "percentage": 50
  },
  "enabled": true
}
```

---

## 4. Drop Filter

**Type**: `drop`
**Purpose**: Advanced conditional dropping with multiple conditions

**Config Parameters**:
- `field` (string) - Field name to check
- `equals` (any) - Value to match
- `contains` (string) - Substring to match
- `matches` (string) - Regex pattern to match
- `condition` (string) - Condition expression

**Examples**:
```json
{
  "type": "drop",
  "config": {
    "field": "status_code",
    "equals": 200
  },
  "enabled": true
}

{
  "type": "drop",
  "config": {
    "field": "path",
    "matches": "^/health"
  },
  "enabled": true
}
```

---

## 5. Add Field Filter

**Type**: `add_field`
**Purpose**: Add new fields to records

**Config Parameters**:
- `field` (string, required) - Field name to add
- `value` (any, required) - Value to set
- `overwrite` (bool) - Overwrite if field exists (default: false)

**Examples**:
```json
{
  "type": "add_field",
  "config": {
    "field": "environment",
    "value": "production"
  },
  "enabled": true
}

{
  "type": "add_field",
  "config": {
    "field": "processed_at",
    "value": "{{.Timestamp}}",
    "overwrite": false
  },
  "enabled": true
}

{
  "type": "add_field",
  "config": {
    "field": "tenant_name",
    "value": "{{.TenantID}}",
    "overwrite": true
  },
  "enabled": true
}
```

---

## 6. Remove Field Filter

**Type**: `remove_field`
**Purpose**: Remove fields from records

**Config Parameters**:
- `field` (string, required) - Field name to remove
- `fields` ([]string) - Multiple fields to remove

**Examples**:
```json
{
  "type": "remove_field",
  "config": {
    "field": "password"
  },
  "enabled": true
}

{
  "type": "remove_field",
  "config": {
    "fields": ["temp", "debug_info", "internal_id"]
  },
  "enabled": true
}
```

---

## 7. Rename Field Filter

**Type**: `rename_field`
**Purpose**: Rename fields in records

**Config Parameters**:
- `from` (string, required) - Original field name
- `to` (string, required) - New field name
- `overwrite` (bool) - Overwrite if target exists (default: false)

**Examples**:
```json
{
  "type": "rename_field",
  "config": {
    "from": "msg",
    "to": "message"
  },
  "enabled": true
}

{
  "type": "rename_field",
  "config": {
    "from": "user_id",
    "to": "userId",
    "overwrite": true
  },
  "enabled": true
}
```

---

## 8. Mutate Filter

**Type**: `mutate`
**Purpose**: Multiple field operations (add, remove, rename, convert, uppercase, lowercase)

**Config Parameters**:
- `add` (map[string]any) - Fields to add
- `remove` ([]string) - Fields to remove
- `rename` (map[string]string) - Fields to rename (from -> to)
- `uppercase` ([]string) - Fields to uppercase
- `lowercase` ([]string) - Fields to lowercase
- `convert` (map[string]string) - Fields to convert types (field -> type)
- `strip` ([]string) - Fields to strip whitespace from

**Examples**:
```json
{
  "type": "mutate",
  "config": {
    "add": {
      "environment": "production",
      "version": "1.0"
    },
    "remove": ["temp", "debug"],
    "rename": {
      "msg": "message",
      "lvl": "level"
    }
  },
  "enabled": true
}

{
  "type": "mutate",
  "config": {
    "uppercase": ["hostname", "service"],
    "lowercase": ["email", "username"],
    "convert": {
      "port": "integer",
      "is_active": "boolean"
    }
  },
  "enabled": true
}

{
  "type": "mutate",
  "config": {
    "strip": ["username", "email"],
    "remove": ["password", "secret"]
  },
  "enabled": true
}
```

---

## 9. Regex Replace Filter

**Type**: `regex_replace`
**Purpose**: Find and replace using regular expressions

**Config Parameters**:
- `field` (string, required) - Field to operate on
- `pattern` (string, required) - Regex pattern to match
- `replacement` (string, required) - Replacement string
- `target` (string) - Target field for result (default: same as source)

**Examples**:
```json
{
  "type": "regex_replace",
  "config": {
    "field": "message",
    "pattern": "\\d{3}-\\d{2}-\\d{4}",
    "replacement": "XXX-XX-XXXX"
  },
  "enabled": true
}

{
  "type": "regex_replace",
  "config": {
    "field": "phone",
    "pattern": "(\\d{3})(\\d{3})(\\d{4})",
    "replacement": "($1) $2-$3"
  },
  "enabled": true
}

{
  "type": "regex_replace",
  "config": {
    "field": "url",
    "pattern": "https?://",
    "replacement": "",
    "target": "domain"
  },
  "enabled": true
}
```

---

## 10. Date Parse Filter

**Type**: `date_parse`
**Purpose**: Parse timestamp strings into structured dates

**Config Parameters**:
- `source_field` (string, required) - Field containing timestamp
- `target_field` (string) - Field to store parsed date (default: "@timestamp")
- `formats` ([]string) - Array of date formats to try
- `timezone` (string) - Timezone for parsing (default: "UTC")
- `locale` (string) - Locale for parsing

**Examples**:
```json
{
  "type": "date_parse",
  "config": {
    "source_field": "timestamp",
    "target_field": "@timestamp",
    "formats": ["2006-01-02T15:04:05Z07:00"]
  },
  "enabled": true
}

{
  "type": "date_parse",
  "config": {
    "source_field": "log_time",
    "formats": [
      "2006-01-02 15:04:05",
      "02/Jan/2006:15:04:05 -0700",
      "Jan 2 15:04:05"
    ],
    "timezone": "America/New_York"
  },
  "enabled": true
}
```

---

## 11. Grok Filter

**Type**: `grok`
**Purpose**: Parse unstructured text using patterns

**Config Parameters**:
- `pattern` (string, required) - Grok pattern
- `source_field` (string) - Field to parse (default: "message")
- `target_field` (string) - Field for parsed data
- `remove_field` (bool) - Remove source field after parsing
- `patterns_dir` (string) - Directory with custom patterns

**Examples**:
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{COMMONAPACHELOG}"
  },
  "enabled": true
}

{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} %{GREEDYDATA:message}"
  },
  "enabled": true
}

{
  "type": "grok",
  "config": {
    "source_field": "raw_log",
    "pattern": "%{IP:client_ip} - - \\[%{HTTPDATE:timestamp}\\] \"%{WORD:method} %{URIPATHPARAM:request} HTTP/%{NUMBER:http_version}\" %{NUMBER:status_code} %{NUMBER:bytes}",
    "remove_field": true
  },
  "enabled": true
}
```

---

## 12. KV Filter

**Type**: `kv`
**Purpose**: Parse key-value pairs from strings

**Config Parameters**:
- `source_field` (string) - Field to parse (default: "message")
- `target_field` (string) - Field for parsed data
- `field_split` (string) - Separator between pairs (default: " ")
- `value_split` (string) - Separator between key and value (default: "=")
- `trim_key` (string) - Characters to trim from keys
- `trim_value` (string) - Characters to trim from values
- `remove_field` (bool) - Remove source field after parsing

**Examples**:
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "field_split": " ",
    "value_split": "="
  },
  "enabled": true
}

{
  "type": "kv",
  "config": {
    "source_field": "params",
    "field_split": "&",
    "value_split": "=",
    "target_field": "query_params"
  },
  "enabled": true
}

{
  "type": "kv",
  "config": {
    "source_field": "headers",
    "field_split": ",",
    "value_split": ":",
    "trim_key": " ",
    "trim_value": " \""
  },
  "enabled": true
}
```

---

## 13. Split Filter

**Type**: `split`
**Purpose**: Split one event into multiple events

**Config Parameters**:
- `field` (string, required) - Field containing array to split
- `separator` (string) - String separator if field is string
- `target` (string) - Field name for split value in new events

**Examples**:
```json
{
  "type": "split",
  "config": {
    "field": "tags"
  },
  "enabled": true
}

{
  "type": "split",
  "config": {
    "field": "items",
    "separator": ",",
    "target": "item"
  },
  "enabled": true
}
```

---

## 14. GeoIP Filter

**Type**: `geoip`
**Purpose**: Add geographic information from IP addresses

**Config Parameters**:
- `source_field` (string, required) - Field containing IP address
- `target_field` (string) - Field for geo data (default: "geoip")
- `database_path` (string) - Path to MaxMind database
- `fields` ([]string) - Specific fields to extract

**Examples**:
```json
{
  "type": "geoip",
  "config": {
    "source_field": "client_ip",
    "target_field": "geo"
  },
  "enabled": true
}

{
  "type": "geoip",
  "config": {
    "source_field": "remote_addr",
    "target_field": "location",
    "fields": ["country_name", "city_name", "latitude", "longitude"]
  },
  "enabled": true
}

{
  "type": "geoip",
  "config": {
    "source_field": "ip",
    "database_path": "/var/lib/geoip/GeoLite2-City.mmdb"
  },
  "enabled": true
}
```

---

## 15. UserAgent Filter

**Type**: `useragent`
**Purpose**: Parse user agent strings

**Config Parameters**:
- `source_field` (string, required) - Field containing user agent
- `target_field` (string) - Field for parsed data (default: "user_agent")
- `regex_file` (string) - Custom regex file path

**Examples**:
```json
{
  "type": "useragent",
  "config": {
    "source_field": "user_agent"
  },
  "enabled": true
}

{
  "type": "useragent",
  "config": {
    "source_field": "http_user_agent",
    "target_field": "ua"
  },
  "enabled": true
}
```

---

## 16. DNS Filter

**Type**: `dns`
**Purpose**: Perform DNS lookups

**Config Parameters**:
- `source_field` (string, required) - Field containing IP or hostname
- `target_field` (string) - Field for result
- `action` (string) - "reverse" or "resolve" (default: "reverse")
- `nameserver` (string) - DNS server to use
- `timeout` (int) - Timeout in seconds

**Examples**:
```json
{
  "type": "dns",
  "config": {
    "source_field": "client_ip",
    "target_field": "hostname",
    "action": "reverse"
  },
  "enabled": true
}

{
  "type": "dns",
  "config": {
    "source_field": "domain",
    "target_field": "ip_address",
    "action": "resolve",
    "timeout": 5
  },
  "enabled": true
}
```

---

## 17. Fingerprint Filter

**Type**: `fingerprint`
**Purpose**: Generate hash fingerprints for events

**Config Parameters**:
- `source` ([]string) - Fields to include in hash
- `target_field` (string) - Field for fingerprint (default: "fingerprint")
- `method` (string) - Hash method: "MD5", "SHA1", "SHA256" (default: "SHA256")
- `concatenate_sources` (bool) - Concatenate source fields (default: true)

**Examples**:
```json
{
  "type": "fingerprint",
  "config": {
    "source": ["message", "timestamp", "host"],
    "method": "SHA256"
  },
  "enabled": true
}

{
  "type": "fingerprint",
  "config": {
    "source": ["user_id", "session_id"],
    "target_field": "event_id",
    "method": "MD5"
  },
  "enabled": true
}
```

---

## 18. Conditional Filter

**Type**: `conditional`
**Purpose**: Apply filters based on conditions

**Config Parameters**:
- `condition` (string, required) - Condition expression
- `if_true` ([]FilterConfig) - Filters to apply if condition is true
- `if_false` ([]FilterConfig) - Filters to apply if condition is false

**Examples**:
```json
{
  "type": "conditional",
  "config": {
    "condition": "{{.level}} == 'error'",
    "if_true": [
      {
        "type": "add_field",
        "config": {
          "field": "alert",
          "value": "true"
        }
      }
    ]
  },
  "enabled": true
}

{
  "type": "conditional",
  "config": {
    "condition": "{{.status_code}} >= 400",
    "if_true": [
      {
        "type": "add_field",
        "config": {
          "field": "error_flag",
          "value": "true"
        }
      }
    ],
    "if_false": [
      {
        "type": "remove_field",
        "config": {
          "field": "error_details"
        }
      }
    ]
  },
  "enabled": true
}
```

---

## 19. JSON Validate Filter

**Type**: `json_validate`
**Purpose**: Validate JSON structure and schema

**Config Parameters**:
- `source_field` (string) - Field to validate (default: entire record)
- `schema` (object) - JSON schema for validation
- `drop_invalid` (bool) - Drop invalid events (default: false)
- `tag_on_failure` (string) - Tag to add on validation failure

**Examples**:
```json
{
  "type": "json_validate",
  "config": {
    "source_field": "payload",
    "drop_invalid": true
  },
  "enabled": true
}

{
  "type": "json_validate",
  "config": {
    "schema": {
      "type": "object",
      "required": ["user_id", "timestamp"],
      "properties": {
        "user_id": {"type": "string"},
        "timestamp": {"type": "string"}
      }
    },
    "tag_on_failure": "invalid_schema"
  },
  "enabled": true
}
```

---

## 20. JSON Flatten Filter

**Type**: `json_flatten`
**Purpose**: Flatten nested JSON objects

**Config Parameters**:
- `source_field` (string) - Field to flatten (default: entire record)
- `target_field` (string) - Field for flattened result
- `separator` (string) - Separator for nested keys (default: ".")
- `depth` (int) - Maximum depth to flatten (0 = unlimited)

**Examples**:
```json
{
  "type": "json_flatten",
  "config": {
    "separator": "."
  },
  "enabled": true
}

{
  "type": "json_flatten",
  "config": {
    "source_field": "metadata",
    "target_field": "flat_metadata",
    "separator": "_",
    "depth": 2
  },
  "enabled": true
}
```

---

## 21. Uppercase Keys Filter

**Type**: `uppercase_keys`
**Purpose**: Convert all object keys to uppercase

**Config Parameters**:
- `source_field` (string, optional) - Specific field to operate on (if not specified, operates on entire record)
- `recursive` (bool) - Recursively uppercase nested objects (default: true)

**Examples**:
```json
{
  "type": "uppercase_keys",
  "config": {
    "recursive": true
  },
  "enabled": true
}

{
  "type": "uppercase_keys",
  "config": {
    "source_field": "metadata",
    "recursive": true
  },
  "enabled": true
}

{
  "type": "uppercase_keys",
  "config": {
    "recursive": false
  },
  "enabled": true
}
```

---

## Common Configuration Patterns

### Pattern 1: Log Parsing Pipeline
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "pattern": "%{COMMONAPACHELOG}"
      },
      "enabled": true
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["02/Jan/2006:15:04:05 -0700"]
      },
      "enabled": true
    },
    {
      "type": "geoip",
      "config": {
        "source_field": "clientip"
      },
      "enabled": true
    }
  ]
}
```

### Pattern 2: Data Cleanup Pipeline
```json
{
  "filters": [
    {
      "type": "exclude",
      "config": {
        "field": "level",
        "equals": "debug"
      },
      "enabled": true
    },
    {
      "type": "mutate",
      "config": {
        "remove": ["password", "secret", "token"],
        "strip": ["username", "email"]
      },
      "enabled": true
    },
    {
      "type": "sample",
      "config": {
        "percentage": 10
      },
      "enabled": true
    }
  ]
}
```

### Pattern 3: Enrichment Pipeline
```json
{
  "filters": [
    {
      "type": "add_field",
      "config": {
        "field": "environment",
        "value": "production"
      },
      "enabled": true
    },
    {
      "type": "geoip",
      "config": {
        "source_field": "client_ip"
      },
      "enabled": true
    },
    {
      "type": "useragent",
      "config": {
        "source_field": "user_agent"
      },
      "enabled": true
    },
    {
      "type": "fingerprint",
      "config": {
        "source": ["message", "timestamp"],
        "method": "SHA256"
      },
      "enabled": true
    }
  ]
}
```

### Pattern 4: Transformation Pipeline
```json
{
  "filters": [
    {
      "type": "json_flatten",
      "config": {
        "separator": "_"
      },
      "enabled": true
    },
    {
      "type": "uppercase_keys",
      "config": {
        "recursive": true
      },
      "enabled": true
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "port": "integer",
          "bytes": "integer",
          "duration": "float"
        }
      },
      "enabled": true
    }
  ]
}
```

---

## Filter Execution Order Best Practices

1. **Sample/Filter First**: Apply sample, include, exclude early to reduce processing
2. **Parse Before Transform**: Use grok, kv, date_parse before other operations
3. **Enrich After Parse**: Apply geoip, useragent after basic parsing
4. **Transform Last**: Use mutate, rename, uppercase_keys at the end
5. **Remove Sensitive Data**: Use remove_field or mutate to drop secrets before storage

**Optimal Order Example**:
```
1. sample (reduce volume)
2. exclude (drop unwanted events)
3. grok/kv (parse structure)
4. date_parse (normalize timestamps)
5. geoip/useragent (enrich)
6. mutate/rename (transform)
7. remove_field (cleanup)
8. add_field (add metadata)
```
