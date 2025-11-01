# ByteFreezer Piper - Filter Quick Reference

Quick reference guide for all available filters with minimal examples.

## Filter Categories

### Event Filtering
| Filter | Purpose | Example |
|--------|---------|---------|
| `include` | Keep only matching events | `{"field": "level", "equals": "error"}` |
| `exclude` | Drop matching events | `{"field": "status", "equals": "200"}` |
| `sample` | Random sampling | `{"percentage": 10.0}` |
| `drop` | Advanced conditional dropping | `{"if_field": "status", "equals": "200"}` |

### Data Parsing
| Filter | Purpose | Example |
|--------|---------|---------|
| `grok` | Pattern-based parsing | `{"pattern": "%{COMMONAPACHELOG}"}` |
| `kv` | Key-value pair parsing | `{"field_split": " ", "value_split": "="}` |
| `date_parse` | Timestamp parsing | `{"formats": ["2006-01-02T15:04:05Z"]}` |
| `split` | Split into multiple events | `{"field": "items"}` |

### Data Transformation
| Filter | Purpose | Example |
|--------|---------|---------|
| `mutate` | Field manipulation | `{"lowercase": ["email"]}` |
| `regex_replace` | Regex find/replace | `{"pattern": "[0-9]+", "replacement": "X"}` |
| `add_field` | Add field | `{"field": "env", "value": "prod"}` |
| `remove_field` | Remove fields | `{"fields": ["password", "secret"]}` |
| `rename_field` | Rename field | `{"from": "old", "to": "new"}` |

### Data Enrichment
| Filter | Purpose | Example |
|--------|---------|---------|
| `geoip` | Geographic data from IP | `{"source_field": "ip"}` |
| `useragent` | Parse user agent | `{"source_field": "user_agent"}` |
| `dns` | DNS lookups | `{"resolve": ["ip_address"]}` |
| `fingerprint` | Event hashing | `{"source_fields": ["host", "msg"]}` |

### Utility
| Filter | Purpose | Example |
|--------|---------|---------|
| `conditional` | Conditional filtering | `{"field": "x", "operator": "eq", "value": "y"}` |
| `json_validate` | Validate JSON | `{"source_field": "payload"}` |
| `json_flatten` | Flatten nested JSON | `{"separator": "."}` |
| `uppercase_keys` | Uppercase all keys | `{"recursive": true}` |

## Common Patterns

### Parse Apache Logs
```json
{
  "type": "grok",
  "config": {
    "pattern": "%{COMMONAPACHELOG}"
  }
}
```

### Exclude Bots
```json
{
  "type": "exclude",
  "config": {
    "field": "user_agent",
    "matches": "(bot|crawler|spider)"
  }
}
```

### Sample 10%
```json
{
  "type": "sample",
  "config": {
    "percentage": 10.0
  }
}
```

### Add GeoIP
```json
{
  "type": "geoip",
  "config": {
    "source_field": "ip_address",
    "target_field": "geo"
  }
}
```

### Redact Email
```json
{
  "type": "regex_replace",
  "config": {
    "source_field": "email",
    "pattern": "^(.{3}).*(@.*)$",
    "replacement": "$1***$2"
  }
}
```

### Parse Key-Value
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "field_split": " ",
    "value_split": "="
  }
}
```

### Keep Only Errors
```json
{
  "type": "include",
  "config": {
    "field": "level",
    "matches": "^(error|critical)$"
  }
}
```

### Type Conversion
```json
{
  "type": "mutate",
  "config": {
    "convert": {
      "status": "integer",
      "amount": "float"
    }
  }
}
```

## Complete Pipeline Example

```json
{
  "tenant_id": "my-tenant",
  "dataset_id": "web-logs",
  "filters": [
    {
      "type": "sample",
      "config": {
        "percentage": 50
      }
    },
    {
      "type": "exclude",
      "config": {
        "field": "path",
        "contains": "/health"
      }
    },
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{COMMONAPACHELOG}"
      }
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["02/Jan/2006:15:04:05 -0700"]
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "status": "integer",
          "bytes": "integer"
        }
      }
    },
    {
      "type": "geoip",
      "config": {
        "source_field": "client_ip",
        "target_field": "geo"
      }
    },
    {
      "type": "useragent",
      "config": {
        "source_field": "user_agent",
        "target_field": "ua"
      }
    },
    {
      "type": "remove_field",
      "config": {
        "fields": ["message"]
      }
    }
  ]
}
```

## Filter Order Best Practices

1. **Sample/Filter** - Reduce volume first
2. **Parse** - Extract structure
3. **Date** - Normalize timestamps
4. **Convert** - Fix data types
5. **Conditional** - Route/filter
6. **Enrich** - Add external data
7. **Transform** - Modify fields
8. **Clean** - Remove temp fields

## Performance Tips

- ✅ Filter early (include/exclude/sample at start)
- ✅ Avoid enriching dropped events
- ✅ Use caching for DNS/GeoIP
- ✅ Remove unused fields
- ✅ Combine multiple mutate operations
- ❌ Don't enrich before filtering
- ❌ Don't keep all fields "just in case"

## Common Use Cases

### Web Server Logs
`sample` → `exclude` → `grok` → `date_parse` → `geoip` → `useragent`

### Application Logs
`include` → `grok` → `kv` → `date_parse` → `mutate` → `fingerprint`

### Security Logs
`exclude` → `date_parse` → `conditional` → `geoip` → `dns` → `add_field`

### API Analytics
`exclude` → `grok` → `sample` → `mutate` → `fingerprint`

### Data Privacy
`grok` → `kv` → `regex_replace` (PII) → `fingerprint` → `remove_field`

## Documentation

- **Full Reference**: [FILTERS.md](FILTERS.md)
- **Chaining Examples**: [FILTER_CHAINING_EXAMPLES.md](FILTER_CHAINING_EXAMPLES.md)
- **Implementation Plan**: [../FILTER_IMPLEMENTATION_PLAN.md](../FILTER_IMPLEMENTATION_PLAN.md)

## Support

For issues or questions:
- Check documentation first
- Test with sample data
- Verify filter order
- Review logs for errors
