# ByteFreezer Piper Release Notes

## Version 1.0.4 - 2025-10-11

### Health Monitoring Enhancements

#### 📊 Comprehensive Configuration Reporting
- **Full Configuration Export**: Health reports now include complete service configuration with masked sensitive data
- **Sensitive Data Masking**: Passwords, API keys, and secrets are masked showing only first 2 and last 2 characters (e.g., "pa****rd")
- **Comprehensive Metrics**: Health reports include all configuration sections (s3_source, s3_destination, s3_geoip, postgresql, processing, pipeline, control_service, monitoring, housekeeping, dlq, soc, failure_threshold)
- **Service Capabilities**: Reports include service capability list for better service discovery

#### 🔧 Configuration Sections Reported
- **Service Info**: service_type, version, instance_id, instance_api, report_interval, timeout
- **API**: port configuration
- **S3 Source**: bucket_name, region, poll_interval, endpoint, ssl, access_key (masked), secret_key (masked)
- **S3 Destination**: bucket_name, region, endpoint, ssl, access_key (masked), secret_key (masked)
- **S3 GeoIP**: bucket_name, region, endpoint, ssl, access_key (masked), secret_key (masked)
- **PostgreSQL**: enabled status, host, port, database, username, password (masked), ssl_mode, schema
- **Processing**: max_concurrent_jobs, job_timeout, retry_attempts, retry_backoff, buffer_size
- **Pipeline**: controller_endpoint, config_refresh_interval, geoip_database_path, enable_geoip
- **Control Service**: enabled status, base_url, api_key (masked), timeout
- **Monitoring**: metrics_port, log_level, enable_tracing, tracing_endpoint
- **Housekeeping**: enabled status, interval_seconds
- **DLQ**: all configuration with retry and cleanup settings
- **SOC**: enabled status, endpoint, timeout
- **Failure Threshold**: all monitoring thresholds and configuration
- **Capabilities**: data_pipeline, format_parsing, data_filtering, s3_processing, geoip_enrichment, multi_format_support

### Implementation Details
- `services/health_reporting.go:25`: Added `config map[string]interface{}` field to HealthReportingService struct
- `services/health_reporting.go:59`: Updated NewHealthReportingService() signature to accept config parameter
- `services/health_reporting.go:118`: Modified RegisterService() to use full configuration data
- `services/health_reporting.go:223-238`: Modified generateMetrics() to include configuration in metrics
- `services/services.go:77`: Updated NewHealthReportingService call to pass configuration
- `services/services.go:111-227`: Added buildHealthConfiguration() function with sensitive data masking

### Benefits
- **Better Visibility**: Control service can see full configuration of all piper instances
- **Security**: Sensitive data (passwords, API keys, S3 credentials) are masked to prevent exposure
- **Troubleshooting**: Configuration information helps diagnose issues across distributed instances
- **Service Discovery**: Capabilities list enables dynamic service routing and orchestration
- **Pipeline Monitoring**: Complete visibility into data pipeline configuration and processing capabilities

---

## Version 1.0.3 - 2025-10-11

### Database Cleanup Enhancements
- **TTL-Based Job Cleanup**: Added `CleanupExpiredJobRecords()` to prevent unbounded growth of `piper_job_records` table
  - Automatically removes job records that have exceeded their TTL (Time To Live)
  - Job records are set with 24-hour TTL during creation in `CreateJobRecord()`
  - Simple cleanup: just deletes where `ttl < NOW()`
  - Implementation: `storage/postgresql_state_manager.go:659-678`
- **Consistent Pattern**: Uses same TTL pattern as other ByteFreezer components
  - TTL set at record creation time (not configurable)
  - Cleanup just deletes expired records
  - No complex retention configuration needed

### Performance Improvements
- Reduces PostgreSQL storage usage by removing expired job records
- Improves query performance on `piper_job_records` table
- Prevents long-term accumulation of historical job data
- Indexed TTL column for fast cleanup queries

### Recommended Usage
Run cleanup periodically from control service housekeeping:
```go
// Clean up all expired job records
err := stateManager.CleanupExpiredJobRecords(ctx)
```

---

## Version 1.0.2 - 2025-10-02

### Database Schema Changes
- **PostgreSQL Table Naming**: Updated all PostgreSQL tables to use `piper_` prefix for better component separation
  - `file_locks` → `piper_file_locks`
  - `job_records` → `piper_job_records`
  - `pipeline_configurations` → `piper_pipeline_configurations`
  - `tenants_cache` → `piper_tenants_cache`
  - All indexes updated with corresponding `piper_` prefixes
  - **Migration**: Existing installations will automatically create new tables with prefixed names

### Lock Management Improvements
- **Enhanced Instance ID Generation**: Instance IDs now include PID and timestamp for true uniqueness
  - Format: `piper-{ip/hostname}-{pid}-{timestamp}`
  - Example: `piper-192-168-1-100-12345-1759438349`
  - Prevents lock conflicts between service restarts
- **Startup Lock Cleanup**: Added automatic cleanup of stale locks from previous instances on startup
  - Immediately cleans locks from previous processes with the same base instance ID
  - Reduces lock wait time from 30 minutes to near-instant on restart
  - Maintains TTL-based cleanup as safety fallback
- **Improved Restart Behavior**: File processing can resume immediately after service restart instead of waiting for TTL expiration

### Configuration Updates
- **Added Control Service Configuration**: Added native support for `control_service` configuration block
  - Follows same pattern as other ByteFreezer components for consistency
  - Supports `enabled`, `base_url`, `api_key`, `timeout_seconds`, `account_id`, `tenant_id` settings
  - Replaces legacy `pipeline.controller_endpoint` for proper control service integration
  - Default configuration has `control_service.enabled: false` to maintain backward compatibility

### Bug Fixes
- **Fixed PostgreSQL Table References**: Corrected SQL query that referenced old `file_locks` table name instead of new `piper_file_locks` table
  - Resolves error: "missing FROM-clause entry for table 'file_locks'"
  - Ensures proper lock acquisition and file processing functionality

## Version 1.0.1 - 2025-09-27

### Breaking Changes
- **Removed JSON Parser Support**: Removed `json-logs` parser - only NDJSON format is now supported
  - Rationale: Single JSON documents are a corner case of NDJSON, and JSON arrays are not processed by this system
  - Migration: Convert JSON files to NDJSON format (one JSON object per line)
  - `json_parse` filter also removed from pipeline filters

### New Features
- **Source Metadata Preservation**: Now preserves all source file metadata from S3 and copies it to processed files
  - All source metadata preserved with `source-` prefix to avoid conflicts
  - Includes source file size, content type, ETag, last modified timestamp, and custom metadata
  - Source file path preserved as `source-file-key` metadata
- **Processing Time Tracking**: Added detailed processing time metrics to processed file metadata
  - `processing-time-ms`: Total processing time in milliseconds
  - `processing-started-at`: ISO timestamp when processing began
  - Processing statistics: input/output/filtered/error record counts
  - Processor identification: instance ID and type for debugging

### Bug Fixes
- **Fixed File Discovery**: Fixed S3 path format mismatch that prevented files from being discovered for processing
- **Fixed Database Schema**: Fixed PostgreSQL string slice conversion error in job record creation
- **Improved Path Parsing**: Enhanced S3 file path parsing to handle multiple file naming formats
- **Fixed Discovery Loop**: Fixed missing initial discovery run on startup - discovery now starts immediately
- **Fixed File Format Filter**: Fixed discovery manager to process all file formats instead of only .ndjson.gz files
- **Enhanced Discovery Logging**: Added detailed logging to discovery process for better debugging

## Version 1.0.0 - 2025-09-24

### Major Features Added

#### Automatic Data Format Detection
- **Format Detection System**: Implemented automatic format detection based on filename hints
  - Supports pattern: `tenant--dataset--format.extension` (e.g., `tenant-001--dataset-001--apache.gz`)
  - Falls back to extension-based detection for non-structured filenames
  - Detects 20+ data formats including text and binary formats

#### Comprehensive Data Parsers
- **Text Format Parsers**: Implemented parsers for major text-based data formats
  - **NDJSON**: Newline-delimited JSON parsing
  - **CSV/TSV**: Comma and tab-separated values with header detection
  - **Apache Logs**: Common Log Format parsing with field extraction
  - **Nginx Logs**: Nginx access log parsing with extended fields
  - **InfluxDB Line Protocol**: Time-series data parsing with tags and fields
  - **CEF (Common Event Format)**: Security event parsing
  - **Raw Text**: Fallback parser for unstructured text data

- **Binary Format Support**: Framework for binary format parsing (sFlow, NetFlow, IPFIX)
  - Binary formats currently detected but not processed (as per requirements)
  - Extensible architecture for future binary format support

#### Error Handling and Data Quality
- **Fail-Safe Parsing**: Lines that fail to parse are dropped automatically
- **Error Logging**: Detailed error reporting with line numbers and format context
- **Statistics Tracking**: Comprehensive metrics on processed, filtered, and error records

#### Configuration Management
- **ByteFreezer Control Integration**: Retrieves pipeline configurations from control service
- **Local Caching**: Caches configurations with configurable TTL to reduce control service load
- **Fallback Support**: Uses default configurations when control service is unavailable
- **Environment Variable Support**: All settings configurable via environment variables

#### Customer Data Transformation
- **Filter Pipeline**: Configurable filter chain for data transformation
  - **add_field**: Add custom fields to records
  - **remove_field**: Remove unwanted fields
  - **rename_field**: Rename fields
  - **conditional**: Filter records based on field values
  - Support for additional filters (JSON parse, regex replace, date parse, GeoIP)
- **Template Variables**: Support for dynamic values using template variables
- **Per-Tenant Configuration**: Different filter pipelines per tenant/dataset combination

#### S3 Integration
- **Automatic Output**: Processed data automatically written to S3 destination bucket
- **Format Conversion**: All input formats converted to compressed NDJSON (.gz)
- **Smart Key Generation**: Output keys indicate format transformation (e.g., `--ndjson.gz`)
- **Source Cleanup**: Source files automatically deleted after successful processing

### Technical Implementation

#### Architecture
- **Service-Oriented**: Clean separation of concerns with dedicated services
- **Format Processor**: New processor replacing basic pipeline processor
- **Registry Pattern**: Extensible parser and filter registries
- **Concurrent Processing**: Multi-worker processing with configurable concurrency

#### Data Flow
1. **File Discovery**: Continuous S3 polling for new files
2. **Format Detection**: Automatic format detection from filename
3. **Parsing**: Format-specific parsing to structured JSON
4. **Filtering**: Optional customer-defined transformations
5. **Output**: Compressed NDJSON written to destination S3 bucket
6. **Cleanup**: Source file removal and state tracking

#### Configuration
- **YAML Configuration**: Centralized configuration with sensible defaults
- **Environment Overrides**: All settings can be overridden via environment variables
- **PostgreSQL State**: Persistent state management and job tracking
- **Health Monitoring**: Built-in health checks and metrics

### Supported Data Formats

#### Text Formats (Auto-processed to NDJSON)
- NDJSON - Newline-delimited JSON
- CSV - Comma-separated values
- TSV - Tab-separated values
- Apache Logs - Apache access/error logs
- Nginx Logs - Nginx access/error logs
- InfluxDB Line Protocol - Time-series data
- CEF - Common Event Format (ArcSight)
- Raw Text - Unstructured text data

#### Binary Formats (Detected but not yet processed)
- sFlow - Sampled network packet data
- NetFlow v5/v9 - Cisco flow data
- IPFIX - IP Flow Information Export

#### Additional Text Formats (Framework ready)
- IIS Logs, Squid Logs, Prometheus, StatsD, Graphite
- Syslog RFC5424, GELF, LEEF, FIX Protocol, HL7 v2

### Performance Features
- **Concurrent Processing**: Configurable worker pools for parallel processing
- **Memory Efficient**: Streaming line-by-line processing
- **Compression**: All output data compressed by default
- **Batch Operations**: Efficient S3 operations with proper error handling

### Monitoring and Observability
- **Processing Statistics**: Detailed metrics on throughput and error rates
- **Job Tracking**: PostgreSQL-based job state management
- **Health Checks**: Service health monitoring endpoints
- **Structured Logging**: JSON-formatted logs with context

### Configuration Examples

#### Basic Configuration
```yaml
s3_source:
  bucket_name: "intake"
  poll_interval: "30s"

s3_destination:
  bucket_name: "processed"

processing:
  max_concurrent_jobs: 10
  job_timeout: "30m"

pipeline:
  controller_endpoint: "http://bytefreezer-control:8080"
  config_refresh_interval: "5m"
```

#### Filter Configuration (via Control Service)
```json
{
  "tenant_id": "tenant-001",
  "dataset_id": "dataset-001",
  "enabled": true,
  "filters": [
    {
      "type": "add_field",
      "enabled": true,
      "config": {
        "field": "environment",
        "value": "production"
      }
    },
    {
      "type": "conditional",
      "enabled": true,
      "config": {
        "field": "level",
        "operator": "eq",
        "value": "debug",
        "action": "drop"
      }
    }
  ]
}
```

### Breaking Changes
- **Replaced Pipeline Processor**: Old pipeline processor replaced with format processor
- **New Configuration Schema**: Updated configuration format for new features
- **Output Format Change**: All output now in NDJSON format instead of preserving input format

### Migration Guide
1. Update configuration files to new schema
2. Ensure ByteFreezer Control service is accessible for pipeline configurations
3. Update any downstream consumers to expect NDJSON format
4. Review filter configurations and migrate to new filter system

### Known Issues
- Binary formats (sFlow, NetFlow, IPFIX) detected but not yet parsed
- Large CSV files processed entirely in memory (will be optimized in future releases)
- Configuration cache refresh could be more intelligent (currently time-based only)

### Future Enhancements Planned
- Binary format parser implementations
- Streaming CSV processing for large files
- Advanced filter conditions and transformations
- Real-time configuration updates via WebSocket
- Enhanced GeoIP integration
- Custom parser plugin system