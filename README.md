# ByteFreezer Piper

Data pipeline processing service for the ByteFreezer ecosystem. Transforms raw NDJSON data from S3 using configurable filter chains.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ bytefreezer-    │    │ bytefreezer-    │    │ bytefreezer-    │
│ receiver        │    │ piper           │    │ packer          │
│                 │    │                 │    │                 │
│ • Webhooks      │    │ • Pipeline      │    │ • Parquet       │
│ • Raw S3 Store  │ →  │ • Filters       │ →  │ • Optimization  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
     Raw JSON              Processed JSON          Parquet Files
```

## Features

- **File Discovery**: Continuously polls S3 for new raw data files
- **Pipeline Processing**: Transforms data using configurable filter chains
- **PostgreSQL State Management**: Uses PostgreSQL for distributed locking and job tracking
- **Control Service Integration**: Gets active tenant list from control service
- **File Cleanup**: Deletes source files after successful processing
- **Monitoring**: Health checks and metrics endpoints

### Current Mode: Pipeline Processing
The piper processes data through configurable pipelines that:
- Parse NDJSON/JSON data from `intake` bucket
- Apply configurable filters (parse, add_field, etc.)
- Output processed data to `piper` bucket
- Uses PostgreSQL for distributed coordination
- Gets tenant information from control service
- Deletes source files after successful copy

*Note: Advanced pipeline filtering is available but currently disabled for simplicity*

## Configuration

Basic configuration in `config.yaml`:

```yaml
app:
  name: "bytefreezer-piper"
  instance_id: "piper-prod-001"
  log_level: "info"

s3_source:
  bucket_name: "bytefreezer-raw"
  region: "us-east-1"
  prefix: "raw/"
  poll_interval: "30s"

s3_destination:
  bucket_name: "bytefreezer-processed"
  region: "us-east-1"
  prefix: "processed/"

postgresql:
  host: "localhost"
  port: 5432
  database: "bytefreezer"
  username: "bytefreezer"
  password: "bytefreezer"
  ssl_mode: "disable"
  schema: "public"

pipeline:
  controller_endpoint: "http://bytefreezer-control:8080"

processing:
  max_concurrent_jobs: 10
  job_timeout: "30m"
  retry_attempts: 3

monitoring:
  metrics_port: 9090
  health_port: 8080
```

Environment variables override config using `PIPER_` prefix:
```bash
PIPER_S3_SOURCE_BUCKET_NAME=my-raw-bucket
PIPER_PROCESSING_MAX_CONCURRENT_JOBS=20
PIPER_APP_LOG_LEVEL=debug
```

## Running

### Local Development
```bash
# Default config
./bytefreezer-piper

# Custom config
./bytefreezer-piper --config /path/to/config.yaml
```

### Docker
```bash
docker run -v $(pwd)/config.yaml:/config.yaml \
  bytefreezer/piper:latest --config /config.yaml
```

## Data Flow

1. **Discovery**: Poll S3 for new raw files
2. **Locking**: Acquire file lock in PostgreSQL
3. **Processing**: Transform data through configurable filter pipelines
4. **Output**: Upload processed files to S3
5. **Cleanup**: Update job status and release lock

## Monitoring

### Health Endpoints
- `GET /health` - Service health check
- `GET /status` - Detailed service status
- `GET /metrics` - Prometheus metrics

### Key Metrics
- Processing rate (files/minute, records/second)
- Processing latency (end-to-end timing)
- Error rate (failed jobs percentage)
- Queue depth (pending jobs count)

## Development

### Building
```bash
go build -o bytefreezer-piper .
```

### Testing
```bash
go test ./...
```

### Running Locally
```bash
./bytefreezer-piper --config config.yaml
```

## Pipeline Configuration Example

```json
{
  "tenant_id": "acme",
  "dataset_id": "logs",
  "enabled": true,
  "version": "1.0.0",
  "filters": [
    {
      "type": "add_field",
      "enabled": true,
      "config": {
        "field": "processed_at",
        "value": "${timestamp}"
      }
    },
    {
      "type": "conditional",
      "enabled": true,
      "config": {
        "field": "level",
        "operator": "eq",
        "value": "ERROR",
        "action": "keep"
      }
    }
  ]
}
```

## License

ByteFreezer Piper is licensed under the MIT License.