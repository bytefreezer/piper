# ByteFreezer Piper

Data processing and pipeline orchestration service for the ByteFreezer platform. Piper reads raw data from S3, applies transformations and filters, enriches data with GeoIP and other lookups, and writes processed data to destination S3 buckets.

## Overview

ByteFreezer Piper is the **processing layer** in the ByteFreezer four-service architecture:

1. **bytefreezer-proxy**: Protocol data collection → Receiver
2. **bytefreezer-receiver**: HTTP webhook ingestion → S3 raw/
3. **bytefreezer-piper** (this service): Data processing pipeline → S3 processed/
4. **bytefreezer-packer**: Parquet optimization → S3 parquet/

### Key Features

- **Filter Pipeline** - Configurable filter chains (grok, mutate, drop, kv, etc.)
- **GeoIP Enrichment** - Automatic IP geolocation with MaxMind database updates
- **DNS Enrichment** - Reverse DNS lookups
- **User Agent Parsing** - Browser and device detection
- **Multi-Tenant Processing** - Isolated processing per tenant/dataset
- **Schema Detection** - Automatic format detection and parsing
- **Transformation API** - REST API for testing and managing transformations
- **Health Reporting** - Reports status to ByteFreezer Control
- **OpenTelemetry Integration** - Comprehensive metrics and tracing

## Installation

### Docker (Recommended)

```bash
# Pull the latest image
docker pull ghcr.io/bytefreezer/piper:latest

# Run with configuration
docker run -p 8080:8080 -v $(pwd)/config.yaml:/config.yaml ghcr.io/bytefreezer/piper:latest
```

### Build from Source

```bash
go build -o bytefreezer-piper .
./bytefreezer-piper --config config.yaml
```

## Configuration

```yaml
app:
  name: "bytefreezer-piper"
  version: "1.0.0"

logging:
  level: "info"
  encoding: "json"

server:
  api_port: 8080

s3_source:
  bucket_name: "raw-data"
  region: "us-east-1"

s3_dest:
  bucket_name: "processed-data"
  region: "us-east-1"

control_service:
  base_url: "http://control:8082"
  api_key: "your-api-key"

housekeeping:
  enabled: true
  interval_seconds: 300
```

### Environment Variables

All configuration options support environment variable overrides with the `BYTEFREEZER_PIPER_` prefix:

```bash
export BYTEFREEZER_PIPER_LOGGING_LEVEL=debug
export BYTEFREEZER_PIPER_SERVER_API_PORT=8080
```

## Filter Pipeline

Piper supports a rich set of filters for data transformation:

- **grok** - Pattern matching and field extraction
- **mutate** - Field manipulation (rename, remove, replace, convert)
- **kv** - Key-value pair parsing
- **drop** - Conditional event dropping
- **date** - Date/time parsing
- **geoip** - IP geolocation enrichment
- **dns** - DNS lookup enrichment
- **useragent** - User agent parsing
- **sample** - Random sampling
- **fingerprint** - Generate unique document fingerprints

See `docs/FILTERS.md` for detailed filter documentation.

## API Endpoints

- `GET /api/v1/health` - Service health check
- `GET /api/v1/config` - Current configuration
- `GET /api/v1/stats` - Processing statistics
- `POST /api/v1/transformations/test` - Test transformation pipeline
- `POST /api/v1/transformations/validate` - Validate transformation config
- `GET /api/v1/datasets/{tenantId}/{datasetId}/schema` - Get dataset schema

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  S3 Raw Data    │───▶│  ByteFreezer     │───▶│  S3 Processed   │
│  (.ndjson.gz)   │    │  Piper           │    │  (.ndjson.gz)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
              ┌─────▼─────┐       ┌─────▼─────┐
              │  GeoIP    │       │  Control  │
              │  Database │       │  Service  │
              └───────────┘       └───────────┘
```

## License

ByteFreezer is licensed under the [Elastic License 2.0](LICENSE.txt).

You're free to use, modify, and self-host. You cannot offer it as a managed service.
