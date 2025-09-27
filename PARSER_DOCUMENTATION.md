# ByteFreezer Piper Parser and Pipeline Configuration Documentation

## Table of Contents
1. [Overview](#overview)
2. [Parser Types](#parser-types)
3. [Filter Types](#filter-types)
4. [Pipeline Configuration Structure](#pipeline-configuration-structure)
5. [JSON Configuration Examples](#json-configuration-examples)
6. [Built-in Parsers Usage](#built-in-parsers-usage)
7. [Format-Specific Processing](#format-specific-processing)
8. [Configuration Deployment](#configuration-deployment)
9. [Best Practices](#best-practices)
10. [Troubleshooting](#troubleshooting)

## Overview

ByteFreezer Piper is a data processing pipeline service that transforms raw data from various sources into structured, enriched formats. The system uses configurable parsers and filters to process data files automatically based on format detection and tenant-specific pipeline configurations.

### Key Features
- **Auto Format Detection** - Automatically detects data formats from filenames
- **Extensible Parser System** - Support for multiple data formats with pluggable parsers
- **Configurable Filter Chains** - Apply transformations, validations, and enrichments
- **Template Variables** - Dynamic field population using runtime context
- **Per-Tenant Configuration** - Isolated processing pipelines per tenant/dataset

## Parser Types

### Built-in Parser Types

The system includes the following built-in parsers:

#### Text-Based Parsers
| Parser | Format | Description |
|--------|--------|-------------|
| `ndjson-logs`, `ndjson` | NDJSON | Newline-delimited JSON |
| `syslog-rfc3164` | Syslog | Standard syslog format (RFC 3164) |
| `plaintext` | Text | Raw text logs |
| `csv` | CSV | Comma-separated values |
| `tsv` | TSV | Tab-separated values |
| `apache` | Apache | Apache Common Log Format |
| `nginx` | Nginx | Nginx access logs |
| `influx` | InfluxDB | InfluxDB Line Protocol |
| `cef` | CEF | Common Event Format (security logs) |
| `raw` | Raw | Fallback for unrecognized formats |

#### Binary Parsers
| Parser | Format | Description |
|--------|--------|-------------|
| `sflow` | sFlow | Network flow data using Cistern sFlow library |

### Parser Interface

All parsers implement the following interface:

```go
type Parser interface {
    Parse(ctx context.Context, data []byte) (map[string]interface{}, error)
    Name() string
    Type() string
    Configure(config map[string]interface{}) error
}
```

### Parser Configuration Examples

#### NDJSON Parser
```json
{
  "type": "ndjson",
  "config": {}
}
```

#### CSV Parser
```json
{
  "type": "csv",
  "config": {
    "delimiter": ",",
    "headers": ["timestamp", "level", "message", "source"],
    "skip_header": true
  }
}
```

#### Apache Log Parser
```json
{
  "type": "apache",
  "config": {
    "log_format": "common"
  }
}
```

## Filter Types

### Core Filter Types

#### Field Manipulation Filters

**add_field** - Adds fields to records
```json
{
  "type": "add_field",
  "config": {
    "field": "environment",
    "value": "production"
  }
}
```

**remove_field** - Removes specified fields
```json
{
  "type": "remove_field",
  "config": {
    "fields": ["password", "secret", "_parse_error"]
  }
}
```

**rename_field** - Renames fields
```json
{
  "type": "rename_field",
  "config": {
    "from": "msg",
    "to": "message"
  }
}
```

#### Conditional Processing

**conditional** - Filters records based on field values
```json
{
  "type": "conditional",
  "config": {
    "field": "level",
    "operator": "eq",
    "value": "debug",
    "action": "drop"
  }
}
```

Supported operators:
- `eq`, `ne` - Equal/Not equal
- `gt`, `lt`, `gte`, `lte` - Greater/Less than comparisons
- `contains`, `not_contains` - String containment
- `exists`, `not_exists` - Field presence
- `regex` - Regular expression matching

Actions:
- `keep` - Keep matching records
- `drop` - Drop matching records

#### JSON Processing

**json_validate** - Validates JSON content
```json
{
  "type": "json_validate",
  "config": {
    "field": "payload",
    "strict": true
  }
}
```

**json_flatten** - Flattens nested JSON objects
```json
{
  "type": "json_flatten",
  "config": {
    "field": "nested_data",
    "separator": "_",
    "max_depth": 3
  }
}
```

#### Data Transformation

**uppercase_keys** - Converts all keys to uppercase
```json
{
  "type": "uppercase_keys",
  "config": {}
}
```

**regex_replace** - Regular expression replacements
```json
{
  "type": "regex_replace",
  "config": {
    "field": "message",
    "pattern": "\\d{4}-\\d{2}-\\d{2}",
    "replacement": "[DATE]"
  }
}
```

### Template Variables

The system supports template variables in filter values:

| Variable | Description |
|----------|-------------|
| `${timestamp}` | Current processing timestamp (ISO 8601) |
| `${tenant_id}` | Tenant identifier |
| `${dataset_id}` | Dataset identifier |
| `${line_number}` | Line number being processed |
| `${ENV_VAR_NAME}` | Environment variables |

Example usage:
```json
{
  "type": "add_field",
  "config": {
    "field": "processed_at",
    "value": "${timestamp}"
  }
}
```

## Pipeline Configuration Structure

### Core Configuration

```json
{
  "config_key": "tenant-id:dataset-id",
  "tenant_id": "tenant-001",
  "dataset_id": "application-logs",
  "enabled": true,
  "version": "1.0.0",
  "settings": {
    "auto_format_detection": true,
    "drop_parse_failures": false,
    "compress_output": true,
    "max_line_length": 32768,
    "processing_timeout": "5m",
    "batch_size": 1000
  },
  "filters": [
    // Filter configurations here
  ],
  "monitoring": {
    "enabled": true,
    "alert_on_errors": true,
    "error_threshold": 0.05
  }
}
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `config_key` | string | Yes | Unique identifier (format: `tenant:dataset`) |
| `tenant_id` | string | Yes | Tenant identifier |
| `dataset_id` | string | Yes | Dataset identifier |
| `enabled` | boolean | Yes | Whether configuration is active |
| `version` | string | Yes | Configuration version |
| `settings` | object | No | Processing settings |
| `filters` | array | No | Filter chain configuration |
| `monitoring` | object | No | Monitoring configuration |

### Settings Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `auto_format_detection` | boolean | `true` | Enable automatic format detection |
| `drop_parse_failures` | boolean | `false` | Drop records that fail parsing |
| `compress_output` | boolean | `true` | Compress processed output |
| `max_line_length` | integer | `65536` | Maximum line length in bytes |
| `processing_timeout` | string | `10m` | Processing timeout per file |
| `batch_size` | integer | `1000` | Batch size for processing |

## JSON Configuration Examples

### Simple Log Processing

```json
{
  "config_key": "tenant-002:simple-logs",
  "tenant_id": "tenant-002",
  "dataset_id": "simple-logs",
  "enabled": true,
  "version": "1.0.0",
  "settings": {
    "auto_format_detection": true,
    "drop_parse_failures": true,
    "compress_output": true
  },
  "filters": [
    {
      "type": "add_field",
      "name": "add_environment",
      "enabled": true,
      "config": {
        "field": "env",
        "value": "staging"
      }
    },
    {
      "type": "conditional",
      "name": "drop_empty_messages",
      "enabled": true,
      "config": {
        "field": "message",
        "operator": "exists",
        "value": true,
        "action": "keep"
      }
    }
  ]
}
```

### Complex Production Configuration

```json
{
  "config_key": "tenant-001:application-logs",
  "tenant_id": "tenant-001",
  "dataset_id": "application-logs",
  "enabled": true,
  "version": "2.1.0",
  "settings": {
    "auto_format_detection": true,
    "drop_parse_failures": false,
    "compress_output": true,
    "max_line_length": 32768,
    "processing_timeout": "5m",
    "batch_size": 1000
  },
  "filters": [
    {
      "type": "add_field",
      "name": "environment_tagging",
      "enabled": true,
      "config": {
        "field": "environment",
        "value": "production"
      }
    },
    {
      "type": "add_field",
      "name": "processing_metadata",
      "enabled": true,
      "config": {
        "field": "processed_at",
        "value": "${timestamp}"
      }
    },
    {
      "type": "conditional",
      "name": "filter_debug_logs",
      "enabled": true,
      "config": {
        "field": "level",
        "operator": "ne",
        "value": "debug",
        "action": "keep"
      }
    },
    {
      "type": "conditional",
      "name": "keep_error_logs",
      "enabled": true,
      "config": {
        "field": "level",
        "operator": "eq",
        "value": "error",
        "action": "keep"
      }
    },
    {
      "type": "json_validate",
      "name": "validate_payload",
      "enabled": true,
      "config": {
        "field": "payload",
        "strict": false
      }
    },
    {
      "type": "remove_field",
      "name": "cleanup_sensitive_fields",
      "enabled": true,
      "config": {
        "fields": ["raw", "_parse_error", "internal_id", "password"]
      }
    }
  ],
  "monitoring": {
    "enabled": true,
    "alert_on_errors": true,
    "error_threshold": 0.05
  }
}
```

### Security Event Processing

```json
{
  "config_key": "security-team:threat-logs",
  "tenant_id": "security-team",
  "dataset_id": "threat-logs",
  "enabled": true,
  "version": "1.2.0",
  "settings": {
    "auto_format_detection": true,
    "drop_parse_failures": false,
    "compress_output": true
  },
  "filters": [
    {
      "type": "add_field",
      "name": "security_classification",
      "enabled": true,
      "config": {
        "field": "classification",
        "value": "security_event"
      }
    },
    {
      "type": "conditional",
      "name": "high_severity_alerts",
      "enabled": true,
      "config": {
        "field": "severity",
        "operator": "gte",
        "value": 7,
        "action": "keep"
      }
    },
    {
      "type": "regex_replace",
      "name": "mask_ip_addresses",
      "enabled": true,
      "config": {
        "field": "message",
        "pattern": "\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b",
        "replacement": "[IP_MASKED]"
      }
    }
  ]
}
```

## Built-in Parsers Usage

### Format Detection

The system automatically detects formats based on filename patterns:

**Expected filename format:** `tenant--dataset--format_hint.gz`

Examples:
- `company--logs--nginx.gz` → nginx parser
- `security--events--cef.gz` → CEF parser
- `metrics--data--influx.gz` → InfluxDB parser
- `app--data--ndjson.gz` → NDJSON parser

### Parser-Specific Examples

#### Processing Apache Logs
Filename: `webserver--access--apache.gz`
```
192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326
```

Output:
```json
{
  "remote_addr": "192.168.1.1",
  "remote_user": "-",
  "time_local": "10/Oct/2024:13:55:36 -0700",
  "request": "GET /index.html HTTP/1.0",
  "status": 200,
  "body_bytes_sent": 2326
}
```

#### Processing NDJSON Logs
Filename: `application--logs--ndjson.gz`
```
{"timestamp":"2024-10-10T13:55:36Z","level":"info","message":"User login successful","user_id":"12345"}
{"timestamp":"2024-10-10T13:55:37Z","level":"error","message":"Database connection failed","error":"timeout"}
```

Output (each line processed separately):
```json
{
  "timestamp": "2024-10-10T13:55:36Z",
  "level": "info",
  "message": "User login successful",
  "user_id": "12345"
}
```

#### Processing CSV Data
Filename: `metrics--data--csv.gz`
```
timestamp,cpu_usage,memory_usage,disk_usage
2024-10-10T13:55:36Z,45.2,78.1,23.4
2024-10-10T13:55:37Z,47.8,79.3,23.5
```

Configuration:
```json
{
  "type": "csv",
  "config": {
    "headers": ["timestamp", "cpu_usage", "memory_usage", "disk_usage"],
    "skip_header": true
  }
}
```

Output:
```json
{
  "timestamp": "2024-10-10T13:55:36Z",
  "cpu_usage": "45.2",
  "memory_usage": "78.1",
  "disk_usage": "23.4"
}
```

## Format-Specific Processing

### Processing Flow

1. **File Discovery** - Piper discovers files in the source S3 bucket
2. **Format Detection** - Analyze filename for format hints
3. **Parser Selection** - Create appropriate parser instance
4. **Data Processing** - Parse line-by-line or bulk (for CSV)
5. **Filter Application** - Apply configured filter chain
6. **Output Generation** - Convert to NDJSON format
7. **Metadata Enrichment** - Add processing metadata
8. **Storage** - Write processed data to destination bucket

### Special Format Handling

#### CSV/TSV Processing
- Header extraction and validation
- Per-row processing with field mapping
- Type inference for numeric values
- Support for custom delimiters

#### Binary Formats
- Currently limited to sFlow support
- Binary detection prevents text parsing attempts
- Future expansion planned for additional binary formats

#### Multiline Logs
- Stack trace detection and grouping
- Multi-line message reconstruction
- Java exception parsing

## Configuration Deployment

### API Endpoints

Pipeline configurations are managed via the ByteFreezer Control service REST API:

#### Deploy Configuration
```bash
curl -X PUT http://control-service:8080/api/v2/pipeline/config/tenant-001/dataset-logs \
  -H "Content-Type: application/json" \
  -d @pipeline-config.json
```

#### Get Configuration
```bash
curl http://control-service:8080/api/v2/pipeline/config/tenant-001/dataset-logs
```

#### List Tenant Configurations
```bash
curl http://control-service:8080/api/v2/pipeline/config/tenant-001
```

#### Delete Configuration
```bash
curl -X DELETE http://control-service:8080/api/v2/pipeline/config/tenant-001/dataset-logs
```

### Configuration Validation

The system validates configurations for:
- Filter type existence
- Required parameters
- Configuration syntax
- Value validation
- Circular dependencies

### Deployment Process

1. **Validation** - Configuration is validated against schema
2. **Storage** - Configuration is stored in PostgreSQL database
3. **Cache Update** - Piper instances refresh their configuration cache
4. **Processing** - New files are processed with updated configuration

## Best Practices

### Configuration Design

1. **Start Simple** - Begin with basic field additions and conditional filters
2. **Test Incrementally** - Add filters one at a time and validate output
3. **Use Template Variables** - Leverage dynamic field population
4. **Security First** - Remove sensitive fields early in the pipeline
5. **Performance Optimization** - Place selective filters first to reduce processing

### Filter Chain Optimization

```json
{
  "filters": [
    // 1. Early filtering to reduce data volume
    {
      "type": "conditional",
      "config": {
        "field": "level",
        "operator": "ne",
        "value": "debug",
        "action": "keep"
      }
    },
    // 2. Add metadata fields
    {
      "type": "add_field",
      "config": {
        "field": "processed_at",
        "value": "${timestamp}"
      }
    },
    // 3. Data transformation
    {
      "type": "json_validate",
      "config": {
        "field": "payload"
      }
    },
    // 4. Security cleanup (last)
    {
      "type": "remove_field",
      "config": {
        "fields": ["password", "secret"]
      }
    }
  ]
}
```

### Performance Considerations

- **Filter Order** - Place filters that drop records early in the chain
- **Batch Size** - Adjust batch_size based on record size and processing complexity
- **Timeout Settings** - Set appropriate processing timeouts for large files
- **Memory Usage** - Monitor filter memory consumption in complex pipelines

### Security Guidelines

- **Sensitive Data** - Always remove passwords, tokens, and PII
- **Field Validation** - Validate data integrity before processing
- **Access Control** - Implement proper tenant isolation
- **Audit Logging** - Track configuration changes and processing errors

## Troubleshooting

### Common Issues

#### Configuration Not Applied
- Check configuration validation errors
- Verify cache refresh interval (default: 5 minutes)
- Review Piper service logs for configuration loading errors

#### Parsing Failures
- Verify filename format includes correct format hint
- Check parser configuration parameters
- Review sample data for format compliance
- Enable debug logging for detailed error information

#### Filter Errors
- Validate filter configuration syntax
- Check field existence before applying filters
- Verify operator compatibility with data types
- Test filters individually to isolate issues

#### Performance Issues
- Review filter chain complexity
- Adjust batch size and timeout settings
- Monitor memory usage during processing
- Check S3 connection latency and throughput

### Debug Logging

Enable debug logging in the Piper configuration:

```yaml
app:
  log_level: "debug"
```

### Monitoring and Alerting

Configure monitoring settings in pipeline configuration:

```json
{
  "monitoring": {
    "enabled": true,
    "alert_on_errors": true,
    "error_threshold": 0.05,
    "metrics": {
      "processing_time": true,
      "record_count": true,
      "error_count": true
    }
  }
}
```

### Error Handling

The system provides several error handling options:

- **drop_parse_failures** - Drop records that fail parsing
- **on_parse_failure** - Action to take on parsing errors (`tag`, `drop`, `pass_through`)
- **max_parse_errors** - Maximum parse errors before stopping processing
- **retry_attempts** - Number of retry attempts for transient errors

This documentation provides comprehensive guidance for configuring and using the ByteFreezer Piper parser and pipeline system. For additional support, refer to the system logs and API documentation.