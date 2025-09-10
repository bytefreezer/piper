# ByteFreezer-Piper

ByteFreezer-Piper is the data transformation service in the three-service ByteFreezer architecture. It processes raw NDJSON data from S3, applies pipeline transformations, and outputs processed NDJSON data ready for parquet conversion.

## Architecture Overview

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

## Core Functionality

### 📥 Raw Data Discovery
- Continuously polls S3 for new raw data files
- Intelligent file discovery with tenant/dataset parsing
- Avoids duplicate processing through DynamoDB coordination

### 🔄 Pipeline Processing  
- Applies configurable filter chains to transform data
- Supports 8 filter types: add_field, remove_field, rename_field, json_parse, regex_replace, date_parse, conditional, geoip
- Template variable interpolation (${timestamp}, ${tenant_id}, etc.)
- Error handling and statistics tracking

### 📤 Processed Data Output
- Streams transformed data to S3 processed directory
- Preserves data lineage with processing metadata
- Handles file compression and efficient uploads

### 🔐 Distributed Coordination
- File-level locking prevents duplicate processing
- Job status tracking across service instances
- Service discovery and health monitoring via DynamoDB

## Configuration

### Basic Configuration (`config.yaml`)

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

dynamodb:
  region: "us-east-1"
  lock_table: "bytefreezer-file-locks"
  job_table: "bytefreezer-job-status"
  config_table: "bytefreezer-pipeline-configs"
  coordination_table: "bytefreezer-service-coordination"

processing:
  max_concurrent_jobs: 10
  job_timeout: "30m"
  retry_attempts: 3

pipeline:
  controller_endpoint: "http://controller:8080"
  config_refresh_interval: "5m"
  enable_geoip: false

monitoring:
  metrics_port: 9090
  health_port: 8080
```

### Environment Variables
All configuration can be overridden with environment variables using the `PIPER_` prefix:

```bash
PIPER_S3_SOURCE_BUCKET_NAME=my-raw-bucket
PIPER_PROCESSING_MAX_CONCURRENT_JOBS=20
PIPER_APP_LOG_LEVEL=debug
```

## Running the Service

### Local Development
```bash
# Using default config.yaml
./bytefreezer-piper

# Using custom config
./bytefreezer-piper --config /path/to/config.yaml
```

### Docker
```bash
docker run -v $(pwd)/config.yaml:/config.yaml \
  bytefreezer/piper:latest \
  --config /config.yaml
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bytefreezer-piper
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: piper
        image: bytefreezer/piper:latest
        args: ["--config", "/config/config.yaml"]
        volumeMounts:
        - name: config
          mountPath: /config
        env:
        - name: PIPER_APP_INSTANCE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
```

## Data Processing Flow

### 1. File Discovery
```
1. Poll S3 raw/ directory every 30s (configurable)
2. Parse tenant/dataset from file paths
3. Check DynamoDB for existing locks
4. Queue unprocessed files for processing
```

### 2. Job Processing
```
1. Acquire file lock in DynamoDB
2. Download raw .ndjson.gz file from S3
3. Load pipeline configuration for tenant/dataset
4. Stream process through filter chain
5. Upload processed .ndjson.gz to S3
6. Update job status and release lock
```

### 3. Pipeline Filters
Available filter types:
- **add_field**: Add static or dynamic fields
- **remove_field**: Remove specified fields
- **rename_field**: Rename fields
- **conditional**: Filter records based on conditions
- **json_parse**: Parse JSON strings to objects
- **regex_replace**: Regex-based transformations
- **date_parse**: Standardize date formats
- **geoip**: IP geolocation enrichment

## Monitoring and Observability

### Health Endpoints
- `GET /health` - Service health check
- `GET /status` - Detailed service status  
- `GET /metrics` - Prometheus metrics

### Key Metrics
- **Processing Rate**: Files/minute, records/second
- **Latency**: End-to-end processing time
- **Error Rate**: Failed jobs percentage
- **Queue Depth**: Pending jobs count
- **Resource Usage**: CPU, memory, network

### Distributed Tracing
- Complete job lifecycle tracking
- Filter execution tracing
- Cross-service correlation

## Architecture Documents

For detailed architecture information, see:
- [`S3_DATA_LAYOUT.md`](./S3_DATA_LAYOUT.md) - S3 bucket structure and data flow
- [`SERVICE_ARCHITECTURE.md`](./SERVICE_ARCHITECTURE.md) - Detailed service design
- [`DYNAMODB_SCHEMA.md`](./DYNAMODB_SCHEMA.md) - Database schema and coordination
- [`ARCHITECTURE_OVERVIEW.md`](./ARCHITECTURE_OVERVIEW.md) - Complete system overview

## Development

### Building
```bash
go build -o bytefreezer-piper .
```

### Testing
```bash
go test ./...
```

### Linting
```bash
golangci-lint run
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
    },
    {
      "type": "geoip",
      "enabled": true,
      "config": {
        "source_field": "client_ip",
        "target_field": "geo"
      }
    }
  ]
}
```

## Deployment Considerations

### Scaling
- Scale horizontally by adding more piper instances
- Each instance processes jobs independently
- DynamoDB coordinates to prevent duplicate processing

### High Availability
- Deploy across multiple availability zones
- Use ELB for health checking and traffic distribution
- Configure appropriate resource limits and monitoring

### Security
- Use IAM roles for AWS service access
- Encrypt data in transit and at rest
- Network isolation with VPC and security groups

## Troubleshooting

### Common Issues

**Files not being processed**
- Check S3 source bucket permissions
- Verify DynamoDB table access
- Check file path naming conventions

**High error rates**
- Review pipeline configurations
- Check filter syntax and conditions
- Monitor DynamoDB throttling

**Performance issues**
- Increase `max_concurrent_jobs`
- Scale out with more instances
- Check S3 request patterns

### Logs and Debugging
```bash
# Enable debug logging
PIPER_APP_LOG_LEVEL=debug ./bytefreezer-piper

# Check job status
curl http://localhost:8080/jobs/{job-id}

# View metrics
curl http://localhost:9090/metrics
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run linting and tests
6. Submit a pull request

## License

ByteFreezer-Piper is licensed under the MIT License. See [LICENSE](../LICENSE) for details.