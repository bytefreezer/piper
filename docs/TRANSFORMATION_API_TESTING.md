# Transformation API Testing Guide

## Overview

The Transformation API provides 6 endpoints for developing, testing, and deploying data transformations. This guide shows how to test each endpoint with real examples.

## Prerequisites

- Piper running on port 8090
- Data available in S3 intake bucket (format: `{tenant}/{dataset}/`)
- MinIO accessible at configured endpoint

## Testing Workflow

### 1. Get Schema and Sample Data

**Purpose**: Discover what fields are available in your data and get sample records for testing.

```bash
# Get schema with 10 sample records (default)
curl -s "http://localhost:8090/api/v1/transformations/my-tenant/web-logs/schema?count=10" | jq .

# Get schema with 50 samples
curl -s "http://localhost:8090/api/v1/transformations/my-tenant/web-logs/schema?count=50" | jq .
```

**Expected Response**:
```json
{
  "tenant_id": "my-tenant",
  "dataset_id": "web-logs",
  "schema": [
    {
      "name": "message",
      "type": "string",
      "count": 10,
      "nullable": false,
      "sample": "192.168.1.1 - - [01/Jan/2025:12:00:00 +0000] \"GET /api/v1/users HTTP/1.1\" 200 1234"
    },
    {
      "name": "client_ip",
      "type": "string",
      "count": 10,
      "nullable": false,
      "sample": "192.168.1.1"
    }
  ],
  "samples": [
    {
      "line_number": 42,
      "raw_data": "{\"message\":\"...\"}",
      "parsed_data": {
        "message": "..."
      }
    }
  ],
  "total_lines": 1000,
  "sample_count": 10
}
```

**Save samples for next step**:
```bash
curl -s "http://localhost:8090/api/v1/transformations/my-tenant/web-logs/schema?count=10" > samples.json
```

### 2. Test Transformation on Samples

**Purpose**: Iteratively develop filter configurations by testing on sample data.

```bash
# Test single grok filter
curl -s -X POST http://localhost:8090/api/v1/transformations/test \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "web-logs",
    "filters": [
      {
        "type": "grok",
        "config": {
          "pattern": "%{COMMONAPACHELOG}",
          "source_field": "message"
        },
        "enabled": true
      }
    ],
    "samples": [
      {
        "line_number": 1,
        "raw_data": "192.168.1.1 - - [01/Jan/2025:12:00:00 +0000] \"GET /api HTTP/1.1\" 200 1234",
        "parsed_data": {
          "message": "192.168.1.1 - - [01/Jan/2025:12:00:00 +0000] \"GET /api HTTP/1.1\" 200 1234"
        }
      }
    ]
  }' | jq .
```

**Expected Response**:
```json
{
  "results": [
    {
      "input": {
        "message": "192.168.1.1 - - [01/Jan/2025:12:00:00 +0000] \"GET /api HTTP/1.1\" 200 1234"
      },
      "output": {
        "message": "192.168.1.1 - - [01/Jan/2025:12:00:00 +0000] \"GET /api HTTP/1.1\" 200 1234",
        "clientip": "192.168.1.1",
        "timestamp": "01/Jan/2025:12:00:00 +0000",
        "verb": "GET",
        "request": "/api",
        "httpversion": "1.1",
        "response": "200",
        "bytes": "1234"
      },
      "applied": ["grok"],
      "skipped": false,
      "duration_ms": 2,
      "error": ""
    }
  ],
  "success_count": 1,
  "error_count": 0,
  "skipped_count": 0,
  "total_time_ms": 2
}
```

**Test multiple filters (chain)**:
```bash
curl -s -X POST http://localhost:8090/api/v1/transformations/test \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "web-logs",
    "filters": [
      {
        "type": "grok",
        "config": {
          "pattern": "%{COMMONAPACHELOG}",
          "source_field": "message"
        },
        "enabled": true
      },
      {
        "type": "geoip",
        "config": {
          "source_field": "clientip",
          "target_field": "geo"
        },
        "enabled": true
      },
      {
        "type": "mutate",
        "config": {
          "remove_field": ["message"]
        },
        "enabled": true
      }
    ],
    "samples": [...]
  }' | jq .
```

### 3. Validate on Fresh Data

**Purpose**: Test transformation on N records from the latest intake file to verify it works on new data.

```bash
# Validate on 100 fresh records (default)
curl -s -X POST http://localhost:8090/api/v1/transformations/validate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "web-logs",
    "filters": [
      {
        "type": "grok",
        "config": {
          "pattern": "%{COMMONAPACHELOG}",
          "source_field": "message"
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
    ],
    "count": 100
  }' | jq .
```

**Expected Response**:
```json
{
  "results": [...],
  "source_file": "my-tenant/web-logs/2025-01-01-12-00-00.ndjson",
  "source_lines": 5000,
  "success_count": 95,
  "error_count": 3,
  "skipped_count": 2,
  "total_time_ms": 250,
  "avg_time_per_row_ms": 2
}
```

**Check performance metrics**:
```bash
# Test with larger sample
curl -s -X POST http://localhost:8090/api/v1/transformations/validate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "web-logs",
    "filters": [...],
    "count": 1000
  }' | jq '.avg_time_per_row_ms'
```

### 4. Activate Transformation

**Purpose**: Enable transformation for production processing on this dataset.

```bash
# Activate transformation
curl -s -X POST http://localhost:8090/api/v1/transformations/activate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "web-logs",
    "filters": [
      {
        "type": "grok",
        "config": {
          "pattern": "%{COMMONAPACHELOG}",
          "source_field": "message"
        },
        "enabled": true
      },
      {
        "type": "geoip",
        "config": {
          "source_field": "clientip"
        },
        "enabled": true
      },
      {
        "type": "mutate",
        "config": {
          "remove_field": ["message"]
        },
        "enabled": true
      }
    ],
    "enabled": true
  }' | jq .
```

**Expected Response**:
```json
{
  "success": true,
  "message": "Transformation activated successfully",
  "version": "v1735732800",
  "updated_at": "2025-01-01T12:00:00Z"
}
```

**Deactivate transformation**:
```bash
curl -s -X POST http://localhost:8090/api/v1/transformations/activate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "web-logs",
    "filters": [],
    "enabled": false
  }' | jq .
```

### 5. Get Transformation Statistics

**Purpose**: Monitor real-time performance and error rates of active transformation.

```bash
# Get stats
curl -s "http://localhost:8090/api/v1/transformations/my-tenant/web-logs/stats" | jq .
```

**Expected Response**:
```json
{
  "stats": {
    "tenant_id": "my-tenant",
    "dataset_id": "web-logs",
    "enabled": true,
    "filter_count": 3,
    "total_processed": 50000,
    "success_count": 48500,
    "error_count": 500,
    "skipped_count": 1000,
    "avg_rows_per_sec": 150.5,
    "last_error": "geoip: invalid IP address format",
    "last_processed": "2025-01-01T12:30:00Z"
  }
}
```

**Monitor continuously**:
```bash
# Watch stats every 5 seconds
watch -n 5 'curl -s "http://localhost:8090/api/v1/transformations/my-tenant/web-logs/stats" | jq .'
```

### 6. Preview Transformation

**Purpose**: Quick view of transformation effect on random samples from live data.

```bash
# Preview with 10 samples (default)
curl -s "http://localhost:8090/api/v1/transformations/my-tenant/web-logs/preview?count=10" | jq .

# Preview with 50 samples
curl -s "http://localhost:8090/api/v1/transformations/my-tenant/web-logs/preview?count=50" | jq .
```

**Expected Response**:
```json
{
  "samples": [
    {
      "input": {
        "message": "192.168.1.1 - - [01/Jan/2025:12:00:00 +0000] \"GET /api HTTP/1.1\" 200 1234"
      },
      "output": {
        "clientip": "192.168.1.1",
        "timestamp": "01/Jan/2025:12:00:00 +0000",
        "verb": "GET",
        "request": "/api",
        "geo": {
          "city": "San Francisco",
          "country": "US"
        }
      },
      "applied": ["grok", "geoip", "mutate"],
      "skipped": false,
      "duration_ms": 3
    }
  ],
  "enabled": true,
  "filter_count": 3,
  "last_processed": "2025-01-01T12:30:00Z"
}
```

## Complete Workflow Example

```bash
#!/bin/bash

TENANT="my-tenant"
DATASET="web-logs"
BASE_URL="http://localhost:8090/api/v1"

echo "=== Step 1: Get Schema ==="
curl -s "${BASE_URL}/transformations/${TENANT}/${DATASET}/schema?count=10" | jq . > schema.json
cat schema.json

echo -e "\n=== Step 2: Test Transformation ==="
curl -s -X POST ${BASE_URL}/transformations/test \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"${TENANT}\",
    \"dataset_id\": \"${DATASET}\",
    \"filters\": [
      {
        \"type\": \"grok\",
        \"config\": {
          \"pattern\": \"%{COMMONAPACHELOG}\",
          \"source_field\": \"message\"
        },
        \"enabled\": true
      }
    ],
    \"samples\": $(cat schema.json | jq '.samples')
  }" | jq . > test_results.json
cat test_results.json

echo -e "\n=== Step 3: Validate Fresh Data ==="
curl -s -X POST ${BASE_URL}/transformations/validate \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"${TENANT}\",
    \"dataset_id\": \"${DATASET}\",
    \"filters\": [
      {
        \"type\": \"grok\",
        \"config\": {
          \"pattern\": \"%{COMMONAPACHELOG}\",
          \"source_field\": \"message\"
        },
        \"enabled\": true
      }
    ],
    \"count\": 100
  }" | jq . > validate_results.json
cat validate_results.json

echo -e "\n=== Step 4: Activate ==="
curl -s -X POST ${BASE_URL}/transformations/activate \
  -H "Content-Type: application/json" \
  -d "{
    \"tenant_id\": \"${TENANT}\",
    \"dataset_id\": \"${DATASET}\",
    \"filters\": [
      {
        \"type\": \"grok\",
        \"config\": {
          \"pattern\": \"%{COMMONAPACHELOG}\",
          \"source_field\": \"message\"
        },
        \"enabled\": true
      }
    ],
    \"enabled\": true
  }" | jq .

echo -e "\n=== Step 5: Monitor Stats ==="
curl -s "${BASE_URL}/transformations/${TENANT}/${DATASET}/stats" | jq .

echo -e "\n=== Step 6: Preview ==="
curl -s "${BASE_URL}/transformations/${TENANT}/${DATASET}/preview?count=5" | jq .
```

## Common Use Cases

### Case 1: Apache Log Parsing

```json
{
  "filters": [
    {
      "type": "grok",
      "config": {
        "pattern": "%{COMMONAPACHELOG}",
        "source_field": "message"
      },
      "enabled": true
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "target_field": "@timestamp",
        "formats": ["dd/MMM/yyyy:HH:mm:ss Z"]
      },
      "enabled": true
    },
    {
      "type": "geoip",
      "config": {
        "source_field": "clientip",
        "target_field": "geo"
      },
      "enabled": true
    }
  ]
}
```

### Case 2: JSON Log Enrichment

```json
{
  "filters": [
    {
      "type": "mutate",
      "config": {
        "add_field": {
          "environment": "production",
          "pipeline_version": "1.0"
        }
      },
      "enabled": true
    },
    {
      "type": "geoip",
      "config": {
        "source_field": "client_ip",
        "target_field": "geo"
      },
      "enabled": true
    }
  ]
}
```

### Case 3: PII Redaction

```json
{
  "filters": [
    {
      "type": "regex_replace",
      "config": {
        "source_field": "message",
        "pattern": "\\b\\d{3}-\\d{2}-\\d{4}\\b",
        "replacement": "XXX-XX-XXXX",
        "global": true
      },
      "enabled": true
    },
    {
      "type": "regex_replace",
      "config": {
        "source_field": "message",
        "pattern": "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b",
        "replacement": "[EMAIL_REDACTED]",
        "global": true
      },
      "enabled": true
    }
  ]
}
```

### Case 4: Bot Filtering

```json
{
  "filters": [
    {
      "type": "exclude",
      "config": {
        "any_field": ["user_agent", "useragent"],
        "any_matches": "(bot|crawler|spider|scraper)"
      },
      "enabled": true
    }
  ]
}
```

### Case 5: Sampling for Cost Reduction

```json
{
  "filters": [
    {
      "type": "sample",
      "config": {
        "percentage": 10.0
      },
      "enabled": true
    }
  ]
}
```

## Error Handling

### Invalid Filter Type
```bash
# Response
{
  "error": "failed to create filter 0 (invalid_type): unknown filter type: invalid_type"
}
```

### Missing Required Field
```bash
# Response
{
  "error": "at least one filter is required"
}
```

### Too Many Filters
```bash
# Response
{
  "error": "maximum 10 filters allowed"
}
```

### No Data Available
```bash
# Response
{
  "error": "no data files found for my-tenant/web-logs"
}
```

## Performance Tips

1. **Test on samples first**: Use small sample counts (10-20) for iterative development
2. **Validate on larger sets**: Test with 100-1000 records before activation
3. **Monitor avg_time_per_row_ms**: Should be < 5ms for most transformations
4. **Use filter chaining wisely**: Each filter adds processing time
5. **Check error_count**: High error rates indicate filter configuration issues

## Troubleshooting

### Schema endpoint returns empty samples
- Check S3 bucket has data in `{tenant}/{dataset}/` prefix
- Verify MinIO credentials and endpoint in config.yaml
- Check piper logs for S3 connection errors

### Test transformation returns errors
- Validate filter configuration syntax
- Check source_field names match schema
- Review filter-specific requirements in docs/FILTERS.md

### Validation shows high error_count
- Review error messages in results array
- Test problematic filters individually
- Check data format matches filter expectations

### Stats show 0 processed records
- Verify transformation is activated (enabled: true)
- Check pipeline is processing jobs
- Review piper logs for processing errors

## API Response Codes

- **200 OK**: Successful operation
- **400 Bad Request**: Invalid parameters or configuration
- **404 Not Found**: Tenant/dataset not found
- **500 Internal Server Error**: Server-side processing error

## Limits and Constraints

- **Maximum filters per transformation**: 10
- **Maximum sample count (schema)**: 100
- **Maximum validation count**: 1000
- **Filter configuration size**: Reasonable (validated by filter type)
- **API timeout**: 30 seconds
