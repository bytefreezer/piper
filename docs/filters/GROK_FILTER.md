# Grok Filter

The Grok filter performs pattern-based parsing of unstructured log data into structured fields. It's compatible with Logstash Grok patterns and provides a powerful way to extract meaningful data from raw log messages.

## Overview

Grok works by combining text patterns into a more complex pattern that matches your logs. The syntax for a Grok pattern is: `%{PATTERN_NAME:field_name}`

- `PATTERN_NAME`: The name of a predefined pattern (e.g., `IP`, `TIMESTAMP_ISO8601`, `NUMBER`)
- `field_name`: The name of the field to extract the matched value into (optional)

## Configuration

```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{IP:client_ip} - %{USER:user} \\[%{HTTPDATE:timestamp}\\] \"%{WORD:method} %{URIPATHPARAM:request} HTTP/%{NUMBER:http_version}\" %{NUMBER:status} %{NUMBER:bytes}",
    "break_on_match": true,
    "keep_empty_captures": false,
    "named_captures_only": true,
    "overwrite_keys": true
  }
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `source_field` | string | `"message"` | The field to parse |
| `pattern` | string | required | Single Grok pattern to match |
| `patterns` | array | optional | Array of patterns to try (alternative to `pattern`) |
| `break_on_match` | boolean | `true` | Stop trying patterns after first match |
| `keep_empty_captures` | boolean | `false` | Keep fields that captured empty strings |
| `named_captures_only` | boolean | `true` | Only extract named capture groups |
| `overwrite_keys` | boolean | `true` | Overwrite existing fields with same name |
| `target_field` | string | optional | Store all extracted fields in nested object |
| `custom_patterns` | object | optional | Add custom pattern definitions |

## Built-in Patterns

The Grok filter includes a comprehensive library of built-in patterns:

### Base Patterns
- `USERNAME`: `[a-zA-Z0-9._-]+`
- `USER`: Alias for `USERNAME`
- `INT`: Integer numbers
- `NUMBER`: Decimal numbers
- `WORD`: Word characters
- `NOTSPACE`: Non-whitespace
- `DATA`: Minimal match (.*?)
- `GREEDYDATA`: Greedy match (.*)
- `QUOTEDSTRING`: Quoted strings
- `UUID`: UUID format

### Network Patterns
- `IP`: IPv4 or IPv6 address
- `IPV4`: IPv4 address
- `IPV6`: IPv6 address
- `MAC`: MAC address (multiple formats)
- `HOSTNAME`: DNS hostname
- `IPORHOST`: IP or hostname

### Date/Time Patterns
- `TIMESTAMP_ISO8601`: ISO 8601 timestamp
- `HTTPDATE`: Apache/Nginx date format
- `SYSLOGTIMESTAMP`: Syslog timestamp
- `DATE_US`: US date format (MM/DD/YYYY)
- `DATE_EU`: European date format (DD/MM/YYYY)
- `MONTH`: Month name
- `MONTHNUM`: Month number (01-12)
- `MONTHDAY`: Day of month (01-31)
- `YEAR`: Year
- `TIME`: Time (HH:MM:SS)

### HTTP Patterns
- `HTTPMETHOD`: HTTP methods (GET, POST, etc.)
- `HTTPVERSION`: HTTP version
- `COMMONAPACHELOG`: Common Apache log format
- `COMBINEDAPACHELOG`: Combined Apache log format

### Path Patterns
- `PATH`: Unix file path
- `UNIXPATH`: Unix absolute path
- `WINPATH`: Windows file path
- `URI`: Full URI with protocol

### Log Level Patterns
- `LOGLEVEL`: Log severity levels (DEBUG, INFO, WARN, ERROR, etc.)

## Examples

### Example 1: Apache Common Log Format

**Input:**
```json
{
  "message": "192.168.1.100 - admin [10/Oct/2024:13:55:36 -0700] \"GET /api/users HTTP/1.1\" 200 1234"
}
```

**Filter Configuration:**
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{COMMONAPACHELOG}"
  }
}
```

**Output:**
```json
{
  "message": "192.168.1.100 - admin [10/Oct/2024:13:55:36 -0700] \"GET /api/users HTTP/1.1\" 200 1234",
  "client": "192.168.1.100",
  "ident": "-",
  "auth": "admin",
  "timestamp": "10/Oct/2024:13:55:36 -0700",
  "method": "GET",
  "request": "/api/users",
  "http_version": "1.1",
  "status": "200",
  "bytes": "1234"
}
```

### Example 2: Custom Application Log

**Input:**
```json
{
  "message": "2024-10-30T14:23:45Z [ERROR] user john@example.com failed login from 10.0.0.50"
}
```

**Filter Configuration:**
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{TIMESTAMP_ISO8601:timestamp} \\[%{LOGLEVEL:level}\\] user %{EMAILADDRESS:email} %{GREEDYDATA:event_description}"
  }
}
```

**Output:**
```json
{
  "message": "2024-10-30T14:23:45Z [ERROR] user john@example.com failed login from 10.0.0.50",
  "timestamp": "2024-10-30T14:23:45Z",
  "level": "ERROR",
  "email": "john@example.com",
  "event_description": "failed login from 10.0.0.50"
}
```

### Example 3: Multiple Patterns

Try multiple patterns until one matches:

**Filter Configuration:**
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "patterns": [
      "%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} %{GREEDYDATA:message}",
      "%{SYSLOGTIMESTAMP:timestamp} %{HOSTNAME:host} %{GREEDYDATA:message}",
      "%{HTTPDATE:timestamp} %{GREEDYDATA:message}"
    ],
    "break_on_match": true
  }
}
```

### Example 4: Custom Patterns

Define your own patterns:

**Filter Configuration:**
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{CUSTOM_APP_LOG}",
    "custom_patterns": {
      "CUSTOM_APP_LOG": "\\[%{TIMESTAMP_ISO8601:timestamp}\\] %{WORD:service}/%{WORD:component}: %{GREEDYDATA:msg}"
    }
  }
}
```

**Input:**
```json
{
  "message": "[2024-10-30T14:23:45Z] auth/login: User authentication successful"
}
```

**Output:**
```json
{
  "message": "[2024-10-30T14:23:45Z] auth/login: User authentication successful",
  "timestamp": "2024-10-30T14:23:45Z",
  "service": "auth",
  "component": "login",
  "msg": "User authentication successful"
}
```

### Example 5: Target Field (Nested Output)

Store all extracted fields in a nested object:

**Filter Configuration:**
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{IP:ip} %{USER:user} %{NUMBER:status}",
    "target_field": "parsed"
  }
}
```

**Input:**
```json
{
  "message": "192.168.1.1 admin 200"
}
```

**Output:**
```json
{
  "message": "192.168.1.1 admin 200",
  "parsed": {
    "ip": "192.168.1.1",
    "user": "admin",
    "status": "200"
  }
}
```

### Example 6: Syslog Parsing

**Input:**
```json
{
  "message": "Oct 30 14:23:45 server1 sshd[1234]: Failed password for invalid user admin from 10.0.0.50 port 22 ssh2"
}
```

**Filter Configuration:**
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{SYSLOGTIMESTAMP:timestamp} %{HOSTNAME:hostname} %{SYSLOGPROG}: %{GREEDYDATA:event}"
  }
}
```

**Output:**
```json
{
  "message": "Oct 30 14:23:45 server1 sshd[1234]: Failed password for invalid user admin from 10.0.0.50 port 22 ssh2",
  "timestamp": "Oct 30 14:23:45",
  "hostname": "server1",
  "program": "sshd",
  "pid": "1234",
  "event": "Failed password for invalid user admin from 10.0.0.50 port 22 ssh2"
}
```

## Pipeline Configuration Example

Complete pipeline configuration with grok filter:

```json
{
  "tenant_id": "tenant-001",
  "dataset_id": "dataset-001",
  "filters": [
    {
      "type": "grok",
      "enabled": true,
      "config": {
        "source_field": "message",
        "pattern": "%{COMMONAPACHELOG}"
      }
    },
    {
      "type": "date_parse",
      "enabled": true,
      "config": {
        "source_field": "timestamp",
        "target_field": "@timestamp",
        "formats": ["02/Jan/2006:15:04:05 -0700"]
      }
    },
    {
      "type": "remove_field",
      "enabled": true,
      "config": {
        "fields": ["ident", "auth"]
      }
    }
  ]
}
```

## Performance Considerations

1. **Pattern Complexity**: Complex patterns with many alternatives take longer to match. Use specific patterns when possible.

2. **Break on Match**: Set `break_on_match: true` when using multiple patterns to stop after the first successful match.

3. **Named Captures**: Use `named_captures_only: true` to only extract explicitly named fields, reducing overhead.

4. **Pattern Order**: When using multiple patterns, put the most common patterns first for better performance.

## Common Patterns Reference

### Application Logs
```
%{TIMESTAMP_ISO8601:timestamp} \\[%{LOGLEVEL:level}\\] %{GREEDYDATA:message}
```

### Nginx Access Log
```
%{IP:client} - %{USER:user} \\[%{HTTPDATE:timestamp}\\] "%{WORD:method} %{URIPATHPARAM:request} HTTP/%{NUMBER:httpversion}" %{NUMBER:status} %{NUMBER:bytes} "%{DATA:referrer}" "%{DATA:agent}"
```

### JSON Log with Prefix
```
%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} %{GREEDYDATA:json_payload}
```

### Firewall Logs
```
%{SYSLOGTIMESTAMP:timestamp} %{HOSTNAME:firewall} %{WORD:action} %{WORD:protocol} %{IP:src_ip}:%{NUMBER:src_port} -> %{IP:dst_ip}:%{NUMBER:dst_port}
```

## Troubleshooting

### Pattern Not Matching

If your pattern doesn't match:

1. Test with simpler patterns first
2. Check for special characters that need escaping
3. Use `break_on_match: false` to try all patterns
4. Enable debug logging to see pattern matching attempts

### Empty Field Captures

If fields are captured but empty:

- Set `keep_empty_captures: true` to preserve them
- Check if your pattern allows optional matches that result in empty strings

### Performance Issues

If Grok is slow:

- Simplify your patterns
- Use more specific patterns instead of `DATA` or `GREEDYDATA`
- Consider splitting complex parsing into multiple simpler Grok filters
- Use `break_on_match: true` with multiple patterns

## See Also

- [KV Filter](KV_FILTER.md) - For parsing key-value pairs
- [Date Parse Filter](DATE_PARSE_FILTER.md) - For parsing extracted timestamps
- [Mutate Filter](MUTATE_FILTER.md) - For transforming extracted fields
