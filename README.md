# ByteFreezer Piper

Data processing pipeline service for the ByteFreezer platform. Piper applies transformations, parsing, filtering, and enrichment to raw data before passing it to the packer service.

## Overview

ByteFreezer Piper is the **processing layer** in the ByteFreezer architecture:

1. **bytefreezer-proxy**: Protocol data collection → Receiver
2. **bytefreezer-receiver**: HTTP webhook ingestion → S3 raw/
3. **bytefreezer-piper** (this service): Data processing pipeline → S3 processed/
4. **bytefreezer-packer**: Parquet optimization → S3 parquet/

### Key Features

- **Data Parsing** - JSON, CSV, syslog, and custom format parsing
- **Filter Pipeline** - Configurable filter chains (mutate, grok, kv, split, etc.)
- **GeoIP Enrichment** - IP address geolocation lookups
- **DNS Enrichment** - Reverse DNS lookups
- **User Agent Parsing** - Browser and device detection
- **Schema Discovery** - Automatic field type detection
- **Health Reporting** - Reports status to ByteFreezer Control
- **OpenTelemetry Integration** - Comprehensive metrics and tracing

## Installation

### Docker (Recommended)

```bash
docker pull ghcr.io/bytefreezer/piper:latest
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

s3:
  source_bucket: "raw-data"
  dest_bucket: "processed-data"
  region: "us-east-1"

control_service:
  base_url: "http://control:8082"
  api_key: "your-api-key"
```

## Filter Types

- **mutate** - Add, rename, remove, replace fields
- **grok** - Pattern-based parsing
- **kv** - Key-value pair extraction
- **split** - Split fields into arrays
- **date** - Date parsing and formatting
- **geoip** - IP geolocation
- **dns** - DNS lookups
- **useragent** - User agent parsing
- **drop** - Conditional event dropping
- **sample** - Random sampling

## API Endpoints

- `GET /api/v1/health` - Service health check
- `GET /api/v1/config` - Current configuration
- `GET /api/v1/stats` - Processing statistics
- `POST /api/v1/transformations/validate` - Validate transformation config

## License

ByteFreezer is licensed under the [Elastic License 2.0](LICENSE.txt).

You're free to use, modify, and self-host. You cannot offer it as a managed service.
