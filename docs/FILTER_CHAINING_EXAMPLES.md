# Filter Chaining Examples - Real-World Use Cases

This guide demonstrates how to chain multiple filters together to solve real-world data processing challenges.

## Table of Contents

1. [Web Server Log Processing](#1-web-server-log-processing)
2. [Application Log Analysis](#2-application-log-analysis)
3. [Security Event Processing](#3-security-event-processing)
4. [API Analytics](#4-api-analytics)
5. [IoT Sensor Data](#5-iot-sensor-data)
6. [E-commerce Transaction Logs](#6-e-commerce-transaction-logs)
7. [Bot Traffic Filtering](#7-bot-traffic-filtering)
8. [Multi-Source Log Normalization](#8-multi-source-log-normalization)
9. [Cost Optimization with Sampling](#9-cost-optimization-with-sampling)
10. [Data Privacy and Redaction](#10-data-privacy-and-redaction)

---

## 1. Web Server Log Processing

**Scenario**: Process Apache access logs - parse, enrich with GeoIP, exclude bots, and add metadata

**Input**:
```
192.168.1.100 - - [01/Nov/2025:10:15:30 +0000] "GET /api/users HTTP/1.1" 200 1234 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/96.0"
```

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{IP:client_ip} %{USER:ident} %{USER:auth} \\[%{HTTPDATE:timestamp}\\] \"%{WORD:method} %{URIPATHPARAM:request} HTTP/%{NUMBER:http_version}\" %{NUMBER:status} %{NUMBER:bytes} \"%{DATA:user_agent}\""
      }
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "target_field": "@timestamp",
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
      "type": "exclude",
      "config": {
        "field": "user_agent",
        "matches": "(bot|crawler|spider)"
      }
    },
    {
      "type": "geoip",
      "config": {
        "source_field": "client_ip",
        "target_field": "geo",
        "fields": ["country_name", "city_name", "latitude", "longitude"]
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
      "type": "add_field",
      "config": {
        "field": "log_type",
        "value": "apache_access"
      }
    },
    {
      "type": "remove_field",
      "config": {
        "fields": ["message", "ident", "auth"]
      }
    }
  ]
}
```

**Output**:
```json
{
  "client_ip": "192.168.1.100",
  "method": "GET",
  "request": "/api/users",
  "http_version": "1.1",
  "status": 200,
  "bytes": 1234,
  "@timestamp": "2025-11-01T10:15:30Z",
  "geo": {
    "country_name": "United States",
    "city_name": "New York",
    "latitude": 40.7128,
    "longitude": -74.0060
  },
  "ua": {
    "name": "Chrome",
    "version": "96.0",
    "os": "Windows",
    "device": "Desktop"
  },
  "log_type": "apache_access"
}
```

**Key Insights**:
- Parse first (grok) to extract fields
- Parse dates early for proper timestamps
- Type conversion for numeric fields
- Exclude bots before enrichment (saves processing)
- Enrich with geo and user agent data
- Clean up by removing raw fields

---

## 2. Application Log Analysis

**Scenario**: Parse structured application logs, filter by severity, enrich with context

**Input**:
```
2025-11-01T10:15:30Z [worker-1] ERROR user=john action=payment error="card declined" amount=99.99
```

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{TIMESTAMP_ISO8601:timestamp} \\[%{DATA:thread}\\] %{LOGLEVEL:level} %{GREEDYDATA:kv_data}"
      }
    },
    {
      "type": "kv",
      "config": {
        "source_field": "kv_data",
        "field_split": " ",
        "value_split": "=",
        "trim_value": "\""
      }
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["2006-01-02T15:04:05Z07:00"]
      }
    },
    {
      "type": "include",
      "config": {
        "field": "level",
        "matches": "^(ERROR|CRITICAL|WARN)$"
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "amount": "float"
        },
        "lowercase": ["user"]
      }
    },
    {
      "type": "fingerprint",
      "config": {
        "source_fields": ["user", "action", "error"],
        "target_field": "event_id",
        "method": "SHA256"
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "alert",
        "value": "true"
      }
    },
    {
      "type": "remove_field",
      "config": {
        "fields": ["message", "kv_data"]
      }
    }
  ]
}
```

**Output**:
```json
{
  "timestamp": "2025-11-01T10:15:30Z",
  "thread": "worker-1",
  "level": "ERROR",
  "user": "john",
  "action": "payment",
  "error": "card declined",
  "amount": 99.99,
  "event_id": "a1b2c3d4e5f6...",
  "alert": "true"
}
```

**Key Insights**:
- Use grok to extract main structure
- Use kv filter for key-value pairs
- Include filter keeps only important severities
- Fingerprint creates unique event ID for deduplication
- Add alert flag for downstream alerting

---

## 3. Security Event Processing

**Scenario**: Process security logs, detect threats, enrich with context, exclude internal IPs

**Input**:
```
{"timestamp": "2025-11-01T10:15:30Z", "event": "login_attempt", "username": "admin", "source_ip": "203.0.113.45", "status": "failed", "attempts": 5}
```

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "exclude",
      "config": {
        "field": "source_ip",
        "matches": "^(10\\.|192\\.168\\.|172\\.(1[6-9]|2[0-9]|3[01])\\.)"
      }
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["2006-01-02T15:04:05Z07:00"]
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "attempts": "integer"
        }
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "status",
        "operator": "eq",
        "value": "failed",
        "action": "keep"
      }
    },
    {
      "type": "geoip",
      "config": {
        "source_field": "source_ip",
        "target_field": "geo",
        "fields": ["country_name", "country_code"]
      }
    },
    {
      "type": "dns",
      "config": {
        "resolve": ["source_ip"],
        "target_field": "source_hostname",
        "timeout": 2,
        "cache_size": 1000
      }
    },
    {
      "type": "mutate",
      "config": {
        "add": {
          "threat_level": "high"
        }
      }
    },
    {
      "type": "fingerprint",
      "config": {
        "source_fields": ["source_ip", "username"],
        "target_field": "incident_id",
        "method": "SHA256"
      }
    }
  ]
}
```

**Output**:
```json
{
  "timestamp": "2025-11-01T10:15:30Z",
  "event": "login_attempt",
  "username": "admin",
  "source_ip": "203.0.113.45",
  "source_hostname": "suspicious.host.com",
  "status": "failed",
  "attempts": 5,
  "geo": {
    "country_name": "Russia",
    "country_code": "RU"
  },
  "threat_level": "high",
  "incident_id": "e8f7g6h5..."
}
```

**Key Insights**:
- Exclude internal IPs first (saves processing)
- Keep only failed attempts (conditional)
- Enrich with geo and DNS for context
- Create unique incident ID
- Add threat level for prioritization

---

## 4. API Analytics

**Scenario**: Analyze API usage, exclude health checks, sample high-volume endpoints

**Input**:
```
{"timestamp": "2025-11-01T10:15:30.123Z", "method": "GET", "path": "/api/v1/users/123", "status": 200, "duration_ms": 45, "user_id": "usr_123"}
```

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "exclude",
      "config": {
        "field": "path",
        "matches": "^/(health|ping|status)$"
      }
    },
    {
      "type": "grok",
      "config": {
        "source_field": "path",
        "pattern": "/api/%{WORD:api_version}/%{WORD:resource}/%{GREEDYDATA:resource_id}"
      }
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["2006-01-02T15:04:05.000Z07:00"]
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "status": "integer",
          "duration_ms": "integer"
        }
      }
    },
    {
      "type": "mutate",
      "config": {
        "add": {
          "is_error": "false"
        }
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "status",
        "operator": "gte",
        "value": 400,
        "action": "keep"
      }
    },
    {
      "type": "mutate",
      "config": {
        "update": {
          "is_error": "true"
        }
      }
    },
    {
      "type": "sample",
      "config": {
        "percentage": 10.0
      }
    },
    {
      "type": "fingerprint",
      "config": {
        "source_fields": ["method", "resource", "status"],
        "target_field": "endpoint_signature",
        "method": "MD5"
      }
    }
  ]
}
```

**Output** (10% sampled):
```json
{
  "timestamp": "2025-11-01T10:15:30.123Z",
  "method": "GET",
  "path": "/api/v1/users/123",
  "api_version": "v1",
  "resource": "users",
  "resource_id": "123",
  "status": 200,
  "duration_ms": 45,
  "user_id": "usr_123",
  "is_error": "false",
  "endpoint_signature": "a1b2c3..."
}
```

**Key Insights**:
- Exclude health checks early
- Parse API path structure with grok
- Mark errors with conditional logic
- Sample to reduce volume (10%)
- Create endpoint signature for grouping

---

## 5. IoT Sensor Data

**Scenario**: Process IoT sensor readings, validate data, convert units, sample by device type

**Input**:
```
{"device_id": "sensor_001", "type": "temperature", "value": "72.5", "unit": "F", "timestamp": 1698840930, "location": "warehouse_a"}
```

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["unix"]
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "value": "float"
        }
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "value",
        "operator": "gte",
        "value": -100,
        "action": "keep"
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "value",
        "operator": "lte",
        "value": 200,
        "action": "keep"
      }
    },
    {
      "type": "mutate",
      "config": {
        "gsub": [
          {
            "field": "unit",
            "pattern": "F",
            "replacement": "fahrenheit"
          }
        ]
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "celsius",
        "value": ""
      }
    },
    {
      "type": "split",
      "config": {
        "field": "readings",
        "terminator": ","
      }
    },
    {
      "type": "fingerprint",
      "config": {
        "source_fields": ["device_id", "type"],
        "target_field": "sensor_key",
        "method": "MD5"
      }
    },
    {
      "type": "sample",
      "config": {
        "percentage": 20.0,
        "seed": 42
      }
    }
  ]
}
```

**Use Case Notes**:
- Validate sensor readings are in reasonable range
- Convert Unix timestamp to proper timestamp
- Normalize units
- Sample 20% for analytics
- Create sensor key for time-series grouping

---

## 6. E-commerce Transaction Logs

**Scenario**: Process transaction logs, redact PII, enrich with metadata, flag high-value transactions

**Input**:
```
{"order_id": "ORD-12345", "user_email": "john.doe@example.com", "card": "4532-1234-5678-9010", "amount": "599.99", "status": "completed", "timestamp": "2025-11-01T10:15:30Z"}
```

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "regex_replace",
      "config": {
        "source_field": "card",
        "pattern": "\\d{4}-\\d{4}-\\d{4}-(\\d{4})",
        "replacement": "XXXX-XXXX-XXXX-$1"
      }
    },
    {
      "type": "regex_replace",
      "config": {
        "source_field": "user_email",
        "target_field": "user_email_masked",
        "pattern": "^(.{3}).*(@.*)$",
        "replacement": "$1***$2"
      }
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["2006-01-02T15:04:05Z07:00"]
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "amount": "float"
        }
      }
    },
    {
      "type": "include",
      "config": {
        "field": "status",
        "equals": "completed"
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "high_value",
        "value": "false"
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "amount",
        "operator": "gte",
        "value": 500,
        "action": "keep"
      }
    },
    {
      "type": "mutate",
      "config": {
        "update": {
          "high_value": "true"
        }
      }
    },
    {
      "type": "fingerprint",
      "config": {
        "source_fields": ["order_id"],
        "target_field": "transaction_hash",
        "method": "SHA256"
      }
    },
    {
      "type": "remove_field",
      "config": {
        "fields": ["user_email", "card"]
      }
    }
  ]
}
```

**Output**:
```json
{
  "order_id": "ORD-12345",
  "user_email_masked": "joh***@example.com",
  "amount": 599.99,
  "status": "completed",
  "timestamp": "2025-11-01T10:15:30Z",
  "high_value": "true",
  "transaction_hash": "f1e2d3c4..."
}
```

**Key Insights**:
- Redact PII early (credit card, email)
- Keep only completed transactions
- Flag high-value transactions
- Create transaction hash for tracking
- Remove sensitive fields before storage

---

## 7. Bot Traffic Filtering

**Scenario**: Identify and filter bot traffic from web logs

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{COMMONAPACHELOG}"
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
      "type": "exclude",
      "config": {
        "any_field": ["user_agent", "ua.name"],
        "any_matches": "(bot|crawler|spider|scraper|headless)"
      }
    },
    {
      "type": "exclude",
      "config": {
        "field": "request",
        "matches": "/(wp-admin|wp-login|xmlrpc)"
      }
    },
    {
      "type": "exclude",
      "config": {
        "field": "status",
        "equals": "404"
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
      "type": "add_field",
      "config": {
        "field": "human_traffic",
        "value": "true"
      }
    }
  ]
}
```

**Key Insights**:
- Parse user agent first
- Exclude known bots by user agent
- Exclude suspicious paths
- Exclude 404s (often bot scans)
- Tag remaining as human traffic

---

## 8. Multi-Source Log Normalization

**Scenario**: Normalize logs from different sources into a common schema

**Pipeline for Nginx Logs**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{IP:client_ip} - - \\[%{HTTPDATE:timestamp}\\] \"%{WORD:method} %{URIPATHPARAM:path} HTTP/%{NUMBER:http_version}\" %{NUMBER:status} %{NUMBER:bytes}"
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "log_source",
        "value": "nginx"
      }
    },
    {
      "type": "rename_field",
      "config": {
        "from": "client_ip",
        "to": "source_ip"
      }
    },
    {
      "type": "rename_field",
      "config": {
        "from": "path",
        "to": "request_path"
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
      "type": "remove_field",
      "config": {
        "fields": ["message"]
      }
    }
  ]
}
```

**Pipeline for Application Logs**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} %{GREEDYDATA:log_message}"
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "log_source",
        "value": "application"
      }
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["2006-01-02T15:04:05Z07:00"]
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

**Key Insights**:
- Use consistent field names across sources
- Add log_source field to identify origin
- Normalize timestamps to common format
- Remove raw message field after parsing

---

## 9. Cost Optimization with Sampling

**Scenario**: Reduce storage costs while maintaining statistical accuracy

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{COMMONAPACHELOG}"
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "status": "integer"
        }
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "sampled",
        "value": "false"
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "status",
        "operator": "gte",
        "value": 400,
        "action": "keep"
      }
    },
    {
      "type": "sample",
      "config": {
        "percentage": 5.0
      }
    },
    {
      "type": "mutate",
      "config": {
        "update": {
          "sampled": "true",
          "sample_rate": "0.05"
        }
      }
    }
  ]
}
```

**Alternative - Tiered Sampling**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{COMMONAPACHELOG}"
      }
    },
    {
      "type": "mutate",
      "config": {
        "convert": {
          "status": "integer"
        }
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "status",
        "operator": "gte",
        "value": 500,
        "action": "drop"
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "sample_tier",
        "value": "errors_100_percent"
      }
    },
    {
      "type": "conditional",
      "config": {
        "field": "status",
        "operator": "eq",
        "value": 200,
        "action": "keep"
      }
    },
    {
      "type": "sample",
      "config": {
        "percentage": 1.0
      }
    },
    {
      "type": "mutate",
      "config": {
        "update": {
          "sample_tier": "success_1_percent"
        }
      }
    }
  ]
}
```

**Key Insights**:
- Keep 100% of errors (status >= 400)
- Sample 1-10% of successful requests
- Tag with sample rate for accurate analytics
- Tiered sampling by importance

---

## 10. Data Privacy and Redaction

**Scenario**: Redact PII and sensitive data before storage

**Pipeline Configuration**:
```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "source_field": "message",
        "pattern": "%{TIMESTAMP_ISO8601:timestamp} %{GREEDYDATA:log_data}"
      }
    },
    {
      "type": "kv",
      "config": {
        "source_field": "log_data"
      }
    },
    {
      "type": "regex_replace",
      "config": {
        "source_field": "email",
        "pattern": "^(.{3}).*(@.*)$",
        "replacement": "$1***$2"
      }
    },
    {
      "type": "regex_replace",
      "config": {
        "source_field": "phone",
        "pattern": "\\d{3}-\\d{3}-(\\d{4})",
        "replacement": "XXX-XXX-$1"
      }
    },
    {
      "type": "regex_replace",
      "config": {
        "source_field": "ssn",
        "pattern": "\\d{3}-\\d{2}-(\\d{4})",
        "replacement": "XXX-XX-$1"
      }
    },
    {
      "type": "regex_replace",
      "config": {
        "source_field": "credit_card",
        "pattern": "\\d{4}-\\d{4}-\\d{4}-(\\d{4})",
        "replacement": "XXXX-XXXX-XXXX-$1"
      }
    },
    {
      "type": "regex_replace",
      "config": {
        "source_field": "ip_address",
        "pattern": "(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})\\.(\\d{1,3})",
        "replacement": "$1.XXX"
      }
    },
    {
      "type": "fingerprint",
      "config": {
        "source_fields": ["email", "phone"],
        "target_field": "user_hash",
        "method": "SHA256"
      }
    },
    {
      "type": "remove_field",
      "config": {
        "fields": ["password", "token", "api_key", "secret"]
      }
    },
    {
      "type": "add_field",
      "config": {
        "field": "redacted",
        "value": "true"
      }
    }
  ]
}
```

**Redaction Patterns**:
- Email: `joh***@example.com`
- Phone: `XXX-XXX-1234`
- SSN: `XXX-XX-1234`
- Credit Card: `XXXX-XXXX-XXXX-1234`
- IP: `192.168.1.XXX`

**Key Insights**:
- Redact PII with regex_replace
- Keep last 4 digits for support
- Create hashed identifiers for tracking
- Remove secrets entirely
- Tag as redacted for compliance

---

## Best Practices for Filter Chaining

### 1. Order Matters

**Optimal Order**:
1. **Sampling/Filtering** - Reduce volume early
2. **Parsing** - Extract structure (grok, kv)
3. **Date Parsing** - Normalize timestamps
4. **Type Conversion** - Convert data types
5. **Conditional Logic** - Filter/route
6. **Enrichment** - Add external data (geoip, dns, useragent)
7. **Transformation** - Modify fields (mutate, regex_replace)
8. **Redaction** - Remove/mask sensitive data
9. **Metadata** - Add processing metadata
10. **Cleanup** - Remove temporary fields

### 2. Performance Tips

- **Filter early**: Use include/exclude/sample at the beginning
- **Avoid unnecessary enrichment**: Only enrich records you'll keep
- **Cache DNS lookups**: Use DNS filter with caching
- **Batch operations**: Combine multiple mutate operations
- **Remove fields**: Clean up to reduce storage

### 3. Error Handling

- Use `json_validate` before parsing JSON
- Use conditional filters to validate data ranges
- Use multiple grok patterns with `break_on_match: true`
- Always have a fallback pattern

### 4. Testing

- Test with sample data first
- Use deterministic sampling (with seed) for testing
- Validate output schema
- Check performance metrics

### 5. Monitoring

- Track filter application rates
- Monitor dropped event counts
- Measure processing duration per filter
- Alert on parsing failures

---

## Summary

These examples demonstrate:
- Real-world scenarios across different domains
- Proper filter ordering for performance
- Combining multiple filters effectively
- Data privacy and compliance
- Cost optimization strategies
- Statistical sampling techniques

Each pipeline can be customized based on your specific needs and data formats.
