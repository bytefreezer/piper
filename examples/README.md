# ByteFreezer Piper Pipeline Configuration Examples

This directory contains example pipeline configurations that demonstrate how to configure data processing pipelines for different use cases.

## Configuration Format

The pipeline configuration format is inspired by Logstash but adapted for ByteFreezer's architecture. Each configuration is a JSON document that defines how data should be processed for a specific tenant/dataset combination.

### Basic Structure

```json
{
  "config_key": "tenant:dataset",
  "tenant_id": "tenant-name",
  "dataset_id": "dataset-name",
  "enabled": true,
  "version": "1.0.0",
  "description": "Human readable description",
  "created_at": "2025-09-24T14:30:00Z",
  "updated_at": "2025-09-24T14:30:00Z",
  "updated_by": "user@company.com",
  "validated": true,
  "checksum": "sha256:...",
  "settings": { ... },
  "filters": [ ... ],
  "output": { ... },
  "monitoring": { ... }
}
```

## Configuration Fields

### Core Fields
- **config_key**: Unique identifier in format `tenant:dataset`
- **tenant_id**: Tenant identifier
- **dataset_id**: Dataset identifier within the tenant
- **enabled**: Boolean flag to enable/disable the pipeline
- **version**: Configuration version for tracking changes
- **description**: Human-readable description of the pipeline purpose

### Settings Section
Controls general pipeline behavior:

```json
{
  "settings": {
    "auto_format_detection": true,
    "drop_parse_failures": true,
    "preserve_raw_on_error": false,
    "compress_output": true,
    "max_line_length": 32768,
    "processing_timeout": "5m",
    "batch_size": 1000,
    "priority": "normal"
  }
}
```

### Filters Section
Array of filter configurations that define data transformations:

```json
{
  "filters": [
    {
      "type": "filter_type",
      "name": "descriptive_name",
      "enabled": true,
      "description": "What this filter does",
      "condition": "optional condition expression",
      "config": {
        "filter-specific": "parameters"
      }
    }
  ]
}
```

## Available Filter Types

### 1. add_field
Adds a field to every record.

```json
{
  "type": "add_field",
  "name": "add_environment",
  "enabled": true,
  "config": {
    "field": "environment",
    "value": "production"
  }
}
```

**Template Variables Supported:**
- `${timestamp}` - Current timestamp
- `${tenant_id}` - Current tenant ID
- `${dataset_id}` - Current dataset ID
- `${line_number}` - Line number being processed
- `${DC_REGION}` - Environment variable

### 2. remove_field
Removes specified fields from records.

```json
{
  "type": "remove_field",
  "name": "cleanup_sensitive",
  "enabled": true,
  "config": {
    "fields": ["password", "secret_key", "raw"]
  }
}
```

Can also remove a single field:
```json
{
  "config": {
    "field": "sensitive_data"
  }
}
```

### 3. rename_field
Renames a field in the record.

```json
{
  "type": "rename_field",
  "name": "standardize_names",
  "enabled": true,
  "config": {
    "from": "old_field_name",
    "to": "new_field_name"
  }
}
```

### 4. conditional
Filters records based on field values.

```json
{
  "type": "conditional",
  "name": "filter_debug_logs",
  "enabled": true,
  "config": {
    "field": "level",
    "operator": "eq",
    "value": "debug",
    "action": "drop"
  }
}
```

**Supported Operators:**
- `eq` - Equals
- `ne` - Not equals
- `gt` - Greater than
- `lt` - Less than
- `gte` - Greater than or equal
- `lte` - Less than or equal
- `contains` - String contains
- `not_contains` - String does not contain
- `exists` - Field exists
- `not_exists` - Field does not exist

**Actions:**
- `keep` - Keep records that match condition
- `drop` - Drop records that match condition

## Template Variables

Template variables can be used in field values to insert dynamic content:

- `${timestamp}` - ISO 8601 timestamp when record is processed
- `${tenant_id}` - Tenant ID from the configuration
- `${dataset_id}` - Dataset ID from the configuration
- `${line_number}` - Line number of the original input
- Environment variables: `${ENV_VAR_NAME}`

Example:
```json
{
  "type": "add_field",
  "config": {
    "field": "processed_at",
    "value": "${timestamp}"
  }
}
```

## Conditional Expressions

Filters can include optional condition expressions that determine when the filter should be applied:

```json
{
  "type": "add_field",
  "condition": "if [status] >= 400",
  "config": {
    "field": "error_flag",
    "value": "true"
  }
}
```

## Output Configuration

Defines how processed data should be output:

```json
{
  "output": {
    "format": "ndjson",
    "compression": "gzip",
    "s3": {
      "bucket": "processed-data",
      "key_pattern": "${tenant_id}/${dataset_id}/year=${year}/month=${month}/day=${day}/${filename}",
      "partitioning": "daily"
    }
  }
}
```

## Monitoring Configuration

Defines monitoring and alerting settings:

```json
{
  "monitoring": {
    "enabled": true,
    "alert_on_errors": true,
    "error_threshold": 0.05,
    "metrics": ["throughput", "error_rate", "latency"],
    "sla": {
      "max_processing_time": "30s",
      "availability": "99.9%"
    }
  }
}
```

## Example Use Cases

### 1. Web Server Logs (`pipeline-config-example.json`)
- Processes Apache/Nginx logs
- Filters out health checks and debug logs
- Adds environment and processing metadata
- Categorizes HTTP responses
- Removes sensitive fields

### 2. Security Events (`security-logs-config.json`)
- Processes firewall and IDS logs (CEF format)
- Filters by severity level
- Standardizes IP field names
- Adds threat classification
- Preserves raw data on parsing errors

### 3. Application Metrics (`metrics-processing-config.json`)
- Processes InfluxDB line protocol metrics
- Filters test environment data
- Manages high cardinality metrics
- Adds retention policies
- Optimized for high-throughput processing

### 4. Simple Processing (`simple-config.json`)
- Basic configuration with minimal transformations
- Good starting point for new pipelines
- Demonstrates core concepts

## Configuration Deployment

Configurations are deployed via the ByteFreezer Control API:

```bash
# Deploy a pipeline configuration
curl -X PUT http://control-service:8080/api/v2/pipeline/config/tenant-001/dataset-logs \
  -H "Content-Type: application/json" \
  -d @pipeline-config-example.json

# Get current configuration
curl http://control-service:8080/api/v2/pipeline/config/tenant-001/dataset-logs

# List all configurations for a tenant
curl http://control-service:8080/api/v2/pipeline/config/tenant-001
```

## Best Practices

1. **Use Descriptive Names**: Give filters meaningful names and descriptions
2. **Version Control**: Always increment version numbers when making changes
3. **Test First**: Validate configurations in staging before production
4. **Monitor Performance**: Include monitoring configuration for all pipelines
5. **Document Changes**: Use git commit messages or change logs to track modifications
6. **Security**: Remove sensitive fields early in the pipeline
7. **Efficiency**: Place most selective filters first to reduce processing overhead
8. **Validation**: Use the `validated` flag and checksums for configuration integrity

## Troubleshooting

### Common Issues

1. **Filter Not Applied**: Check that `enabled: true` and field names match exactly
2. **Performance Issues**: Reduce batch size or optimize filter order
3. **Parse Failures**: Enable `preserve_raw_on_error` for debugging
4. **Missing Fields**: Use `exists` operator to check field presence before operations

### Debug Configuration

For troubleshooting, use this minimal debug configuration:

```json
{
  "settings": {
    "drop_parse_failures": false,
    "preserve_raw_on_error": true
  },
  "filters": [
    {
      "type": "add_field",
      "enabled": true,
      "config": {
        "field": "debug_processed",
        "value": "${timestamp}"
      }
    }
  ]
}
```