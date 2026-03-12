# ByteFreezer Piper - Release Notes

## 2026-03-09 - Fix Infinite Housekeeping Loop

### Bug Fix

#### Remove duplicate housekeeping loop causing 100% CPU
- **Issue**: Piper had two independent housekeeping loops — one in `main.go` (with zero-interval safety guard) and one in `piper_service.go` (without guard). When `Housekeeping.Interval` was zero (koanf config issue), `time.NewTimer(0)` fired instantly, creating an infinite loop consuming 100% CPU.
- **Fix**: Removed the duplicate loop from `piper_service.go`. `main.go` now delegates to `PiperService.PerformHousekeeping()` as single source of truth. Also removed duplicate `PipelineDatabase` from `Services` struct — `ConfigManager` inside `PiperService` is the real one.
- **Files Changed**: `main.go`, `services/piper_service.go`, `services/services.go`

## 2026-03-01 - Non-fatal Job Status Updates

### Bug Fix

#### UpdateJobStatus failure no longer blocks processing
- **Issue**: `processJob()` called `UpdateJobStatus("processing")` before processing. If this control API call failed (e.g., timeout, network issue), the entire job was abandoned with `return`. This blocked ALL file processing when the control plane was slow or unreachable.
- **Fix**: Made the status update non-fatal — log a warning and continue processing. Status tracking is telemetry, not a processing gate.
- **File Changed**: `services/piper_service.go:324-330`
- **Impact**: Piper now processes files even when the control plane status endpoint is unreachable. The `failed` and `completed` status updates were already non-fatal.

## 2026-01-02 - GeoIP Nested Field Access Fix

### Bug Fix

#### GeoIP Filter: Nested Field Access Support
- **Issue**: GeoIP filter could not access nested fields using dot notation (e.g., `firewall.SRC`)
  - Filter looked for literal key `record["firewall.SRC"]` instead of traversing `record["firewall"]["SRC"]`
  - Transformation with `source_field: "firewall.SRC"` produced no geoip output
- **Fix**: Added `getNestedValue()` helper function to support dot-notation field access
  - Traverses nested maps using path parts split by `.`
  - Returns value and existence flag for proper handling
- **Additional Fix**: Added field name aliases
  - `country` now works as alias for `country_name`
  - `city` now works as alias for `city_name`
- **Files Changed**:
  - `pipeline/geoip.go:16-35` - Added `getNestedValue()` helper function
  - `pipeline/geoip.go:105` - Updated to use `getNestedValue()` for source field lookup
  - `pipeline/geoip.go:158,175` - Added field name aliases
- **Impact**: GeoIP enrichment now works with nested source fields from KV filter output

---

## 2025-11-01 - Async Transformation Job Processing with Distributed Locking

### Major Architecture Change: Async Job-Based Transformation Workflow

Refactored transformation testing API to use async job-based processing with distributed locking support for clustered piper deployments.

#### Problem Solved
Previously, transformation testing APIs were called directly from the UI to piper, causing:
- **Direct coupling** - UI couldn't work with clustered piper instances
- **No load distribution** - Single piper handled all transformation tests
- **CORS issues** - Browser security restrictions
- **No process affinity** - Concurrent requests could interfere with each other

#### New Architecture

**Request Flow:**
```
UI → Control Service → PostgreSQL (create job)
                            ↓
Piper 1 ──┐           Jobs Table
Piper 2 ──┼──→ Poll ──→ (atomic claim with SELECT FOR UPDATE SKIP LOCKED)
Piper 3 ──┘                ↓
                    Single piper claims & processes job
                            ↓
                    Update results in PostgreSQL
                            ↓
UI ← Poll Control ← Get job status & results
```

#### Changes Made

**Piper Service:**
- **domain/types.go:89-135** - Added `TransformationJob` struct and job types (test/validate/activate)
- **storage/postgresql_state_manager.go:148-181** - Added `transformation_jobs` table with indexes
- **storage/postgresql_state_manager.go:215-427** - Implemented 6 database methods:
  - `CreateTransformationJob` - Create new job
  - `ClaimTransformationJob` - **Atomic job claiming** using `SELECT FOR UPDATE SKIP LOCKED` for distributed locking
  - `UpdateTransformationJob` - Update job status/results
  - `GetTransformationJob` - Retrieve job by ID
  - `ListPendingTransformationJobs` - List pending jobs
  - `CleanupExpiredTransformationJobs` - Remove expired jobs
- **services/transformation_job_service.go** - New service that polls for jobs every 5s and processes them
- **services/services.go:68-75** - Integration of transformation job service
- **main.go:170-174,220-224** - Startup/shutdown lifecycle management
- **api/api.go:50-67** - **REMOVED** CORS middleware (no longer needed)

**Control Service:**
- **storage/transformation_types.go** - Transformation job domain types
- **storage/postgresql_transformation.go** - Database operations for job management
- **storage/interface.go:260-263** - Storage interface methods
- **api/transformation_handlers.go** - 5 REST endpoints:
  - `POST /api/v1/tenants/{tenantId}/datasets/{datasetId}/transformations/test`
  - `POST /api/v1/tenants/{tenantId}/datasets/{datasetId}/transformations/validate`
  - `POST /api/v1/tenants/{tenantId}/datasets/{datasetId}/transformations/activate`
  - `GET /api/v1/transformations/jobs/{jobId}` - Get job status and results
  - `GET /api/v1/tenants/{tenantId}/datasets/{datasetId}/transformations/jobs` - List jobs
- **api/api.go:139-144** - Endpoint registration

**UI:**
- **src/types/transformations.ts:89-122** - Added job-related TypeScript types
- **src/lib/transformations-api.ts** - Complete rewrite:
  - Transformation operations now create jobs via control service
  - Automatic job polling with 2s intervals (max 5 minutes)
  - Transparent async→sync conversion for UI components
  - Read-only operations (schema, stats, preview) still call piper directly

#### Key Features

**Distributed Locking:**
- Uses PostgreSQL's `SELECT FOR UPDATE SKIP LOCKED` pattern
- Ensures only one piper instance processes each job
- No deadlocks or race conditions
- Process affinity through `processor_id` field

**Job Lifecycle:**
1. UI creates job via control service (status: `pending`)
2. Piper claims job atomically (status: `processing`, sets `processor_id`)
3. Piper executes transformation work
4. Piper updates job with results (status: `completed` or `failed`)
5. UI polls and retrieves final results
6. Jobs expire after TTL (default: 1 hour)

**Benefits:**
- ✅ **Clustered Support** - Multiple piper instances work together
- ✅ **Load Distribution** - Jobs distributed across available pipers
- ✅ **API Gateway Pattern** - UI only talks to control service
- ✅ **No CORS Issues** - Control service proxies all requests
- ✅ **Job Tracking** - Full visibility into transformation job status
- ✅ **Fault Tolerance** - Jobs can be retried if piper crashes

#### Database Schema

```sql
CREATE TABLE transformation_jobs (
    job_id VARCHAR(100) PRIMARY KEY,
    tenant_id VARCHAR(100) NOT NULL,
    dataset_id VARCHAR(100) NOT NULL,
    job_type VARCHAR(20) NOT NULL,  -- test, validate, activate
    status VARCHAR(20) NOT NULL,     -- pending, processing, completed, failed
    processor_id VARCHAR(100) NULL,  -- Which piper instance claimed this job
    request JSONB NOT NULL,
    result JSONB NULL,
    error_message TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    ttl TIMESTAMP NOT NULL
);

CREATE INDEX idx_transformation_jobs_status
    ON transformation_jobs (status, created_at);
CREATE INDEX idx_transformation_jobs_tenant_dataset
    ON transformation_jobs (tenant_id, dataset_id, created_at DESC);
```

#### Migration Notes

**Breaking Changes:**
- Transformation test/validate/activate operations are now async
- UI code updated to handle job polling automatically
- CORS middleware removed from piper (no longer accepts direct browser calls)

**Backward Compatibility:**
- Read-only operations (schema, stats, preview) unchanged
- Existing transformation testing workflow still works
- No changes to transformation filter execution logic

#### Testing

Services verified:
- Piper: Successfully starts transformation job service
- Piper: Polls for jobs every 5 seconds
- Piper: Distributed locking prevents duplicate processing
- Control: Successfully creates transformation jobs
- Control: Returns job status and results
- UI: Polls and retrieves results transparently

---

## 2025-11-01 - CORS Support for Web UI Integration (DEPRECATED - See Async Architecture above)

### API Enhancement

Added CORS (Cross-Origin Resource Sharing) middleware to enable web UI access to transformation APIs.

**Changes**:
- Added `corsMiddleware` function to handle cross-origin requests
- Configured CORS headers:
  - `Access-Control-Allow-Origin: *` (allow all origins in development)
  - `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
  - `Access-Control-Allow-Headers: Content-Type, Authorization, X-Requested-With`
  - `Access-Control-Max-Age: 86400` (24 hours)
- Handles OPTIONS preflight requests correctly

**Impact**: The transformation testing API can now be accessed directly from web browsers, enabling the ByteFreezer UI to provide an interactive transformation workflow.

**Note**: In production, consider restricting `Access-Control-Allow-Origin` to specific trusted origins.

## 2025-10-31 - Transformation Testing API

### New API Endpoints

Added 6 new REST API endpoints for transformation testing and management workflow, enabling users to develop, test, and activate data transformations interactively.

#### 1. Get Schema and Sample Data
**Endpoint**: `GET /api/v1/transformations/{tenantId}/{datasetId}/schema?count=10`

Retrieves data schema and random sample records for transformation testing.

**Response**:
- Schema with field names, types, and sample values
- Random sample records (configurable count, max 100)
- Total line count in dataset

**Use Case**: Discover available fields and select field names for filter configuration

#### 2. Test Transformation
**Endpoint**: `POST /api/v1/transformations/test`

Tests transformation filters on sample data and shows input/output diff.

**Request**:
```json
{
  "tenant_id": "my-tenant",
  "dataset_id": "web-logs",
  "filters": [
    {"type": "grok", "config": {"pattern": "%{COMMONAPACHELOG}"}, "enabled": true},
    {"type": "geoip", "config": {"source_field": "client_ip"}, "enabled": true}
  ],
  "samples": [...] // Samples from schema endpoint
}
```

**Response**:
- Input/output for each sample
- Applied filters list
- Skipped/dropped records
- Processing duration per record
- Success/error counts

**Use Case**: Iteratively develop and test filter configurations

#### 3. Validate on Fresh Data
**Endpoint**: `POST /api/v1/transformations/validate`

Tests transformation on N fresh records from latest intake.

**Request**:
```json
{
  "tenant_id": "my-tenant",
  "dataset_id": "web-logs",
  "filters": [...],
  "count": 100 // max 1000
}
```

**Response**:
- Transformation results for fresh data
- Source file name
- Performance metrics (avg time per row)
- Success/error/skipped counts

**Use Case**: Validate transformation works on new data before activation

#### 4. Activate Transformation
**Endpoint**: `POST /api/v1/transformations/activate`

Activates or deactivates transformation on a dataset.

**Request**:
```json
{
  "tenant_id": "my-tenant",
  "dataset_id": "web-logs",
  "filters": [...], // Up to 10 filters
  "enabled": true
}
```

**Response**:
- Success status
- Configuration version
- Updated timestamp

**Use Case**: Enable transformation for production processing

#### 5. Get Transformation Statistics
**Endpoint**: `GET /api/v1/transformations/{tenantId}/{datasetId}/stats`

Retrieves real-time statistics about transformation processing.

**Response**:
- Total records processed
- Success/error/skipped counts
- Average rows per second
- Last error message
- Last processed timestamp

**Use Case**: Monitor transformation performance and errors

#### 6. Preview Transformation
**Endpoint**: `GET /api/v1/transformations/{tenantId}/{datasetId}/preview?count=10`

Shows random samples with input/output side-by-side for active transformation.

**Response**:
- Random transformation samples
- Enabled status
- Filter count
- Last processed timestamp

**Use Case**: Quick view of transformation effect on live data

### Implementation Details

**Files Created**:
- `api/transformation_handlers.go` - 6 API endpoints with validation
- `services/transformation_service.go` - Service layer implementation with S3 integration

**Files Modified**:
- `api/api.go:18-31` - Added Services interface methods
- `api/api.go:75-81` - Registered transformation endpoints

**Features**:
- Schema discovery with automatic type detection
- Random sampling for representative testing
- Filter chain validation (max 10 filters)
- Input/output diff for each record
- Performance metrics (duration per record)
- Error handling and reporting

### API Features

- **Filter Validation**: Validates up to 10 filters per transformation
- **Sample Limits**: Schema samples (max 100), fresh data validation (max 1000)
- **Error Reporting**: Detailed error messages for filter failures
- **Performance Tracking**: Duration metrics for optimization
- **OpenAPI Documentation**: Full Swagger docs at `/docs`

### Workflow Example

```bash
# 1. Get schema and samples
curl "http://localhost:8080/api/v1/transformations/my-tenant/web-logs/schema?count=10"

# 2. Test transformation
curl -X POST http://localhost:8080/api/v1/transformations/test \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"my-tenant","dataset_id":"web-logs","filters":[...],"samples":[...]}'

# 3. Validate on fresh data
curl -X POST http://localhost:8080/api/v1/transformations/validate \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"my-tenant","dataset_id":"web-logs","filters":[...],"count":100}'

# 4. Activate transformation
curl -X POST http://localhost:8080/api/v1/transformations/activate \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"my-tenant","dataset_id":"web-logs","filters":[...],"enabled":true}'

# 5. Monitor stats
curl "http://localhost:8080/api/v1/transformations/my-tenant/web-logs/stats"

# 6. Preview transformation
curl "http://localhost:8080/api/v1/transformations/my-tenant/web-logs/preview?count=10"
```

### Testing Resources

**Documentation**:
- `docs/TRANSFORMATION_API_TESTING.md` - Comprehensive testing guide with examples, use cases, and troubleshooting
- `docs/FILTERS.md` - Complete reference for all 24 available filters
- `docs/FILTER_CHAINING_EXAMPLES.md` - 10 real-world pipeline examples

**Test Script**:
- `test_transformation_api.sh` - Automated test suite for all 6 transformation endpoints
  - Tests each endpoint with sample data
  - Validates error handling (invalid filters, too many filters)
  - Provides colored output with pass/fail summary
  - Usage: `./test_transformation_api.sh`
  - Environment variables: `PIPER_URL`, `TEST_TENANT`, `TEST_DATASET`

---

## 2025-10-31 - Event Filtering Filters + Comprehensive Documentation

### New Filters

Added three essential event filtering filters for controlling which events flow through the pipeline.

#### 1. Include Filter
- **Purpose**: Keep only events that match specified conditions (drop everything else)
- **Features**:
  - Field-based matching (equals, contains, regex)
  - Multi-field matching with OR logic (any_field)
  - Inverse of exclude filter
- **Configuration**:
  - `field`: Field name to check (required if no any_field)
  - `equals`: Value to match exactly
  - `contains`: Substring to match
  - `matches`: Regex pattern to match
  - `any_field`: Array of fields to check (OR logic)
  - `any_equals`: Value to match in any field
  - `any_matches`: Regex pattern to match in any field
- **Example**:
  ```json
  {
    "type": "include",
    "config": {
      "field": "level",
      "matches": "^(error|critical)$"
    }
  }
  ```
- **Use Case**: Keep only error and critical level events

#### 2. Exclude Filter
- **Purpose**: Drop events that match specified conditions (keep everything else)
- **Features**:
  - Field-based matching (equals, contains, regex)
  - Multi-field matching with OR logic (any_field)
  - Simpler alternative to drop filter for basic exclusions
- **Configuration**:
  - `field`: Field name to check (required if no any_field)
  - `equals`: Value to match exactly
  - `contains`: Substring to match
  - `matches`: Regex pattern to match
  - `any_field`: Array of fields to check (OR logic)
  - `any_equals`: Value to match in any field
  - `any_matches`: Regex pattern to match in any field
- **Example**:
  ```json
  {
    "type": "exclude",
    "config": {
      "any_field": ["user_agent", "client"],
      "any_matches": "bot"
    }
  }
  ```
- **Use Case**: Exclude bot traffic from multiple fields

#### 3. Sample Filter
- **Purpose**: Keep a random percentage of events (drop the rest) for sampling
- **Features**:
  - Percentage-based sampling (0-100)
  - Rate-based sampling (0.0-1.0)
  - Deterministic sampling with optional seed
  - Statistical sampling for high-volume datasets
- **Configuration**:
  - `percentage`: Percentage of events to keep (0-100)
  - `rate`: Rate of events to keep (0.0-1.0) - alternative to percentage
  - `seed`: Optional seed for deterministic sampling
- **Example**:
  ```json
  {
    "type": "sample",
    "config": {
      "percentage": 10.0,
      "seed": 12345
    }
  }
  ```
- **Use Case**: Sample 10% of events for testing or cost reduction

### Implementation Details

**Files Created:**
- `pipeline/include.go` - Include filter implementation
- `pipeline/exclude.go` - Exclude filter implementation
- `pipeline/sample.go` - Sample filter implementation
- `pipeline/include_test.go` - 6 test cases for include filter
- `pipeline/exclude_test.go` - 6 test cases for exclude filter
- `pipeline/sample_test.go` - 8 test cases for sample filter

**Files Modified:**
- `pipeline/filter_registry.go:160-173` - Registered all three filters

**Tests**: All tests passing (20 test cases total)

**Updated Filter List:** `grok`, `mutate`, `drop`, `kv`, `date_parse`, `split`, `useragent`, `dns`, `geoip`, `fingerprint`, `regex_replace`, `include`, `exclude`, `sample`

### Comprehensive Documentation

Added complete documentation for all 24 filters with real-world examples and best practices.

**Documentation Files Created:**
- `docs/README.md` - Documentation index and quick start guide
- `docs/FILTERS.md` - Complete reference for all 24 filters with detailed examples
- `docs/FILTER_CHAINING_EXAMPLES.md` - 10 real-world use cases with complete pipeline configurations:
  1. Web Server Log Processing (Apache/Nginx)
  2. Application Log Analysis
  3. Security Event Processing
  4. API Analytics
  5. IoT Sensor Data
  6. E-commerce Transaction Logs
  7. Bot Traffic Filtering
  8. Multi-Source Log Normalization
  9. Cost Optimization with Sampling
  10. Data Privacy and Redaction
- `docs/QUICK_REFERENCE.md` - Quick lookup table for all filters with common patterns

**Documentation Highlights:**
- 24 filters fully documented with parameters and examples
- 10 complete real-world pipeline examples
- Best practices for filter ordering and performance
- Security and compliance patterns (PII redaction, data masking)
- Troubleshooting guide
- Quick reference tables for fast lookup

**Total Documentation**: 2000+ lines covering all aspects of filter usage and pipeline configuration

---

## 2025-10-31 - Additional Filter: Regex Replace

### New Filters

#### Regex Replace Filter
- **Purpose**: Perform regex-based find and replace operations on field values
- **Features**:
  - Configurable source and target fields
  - Global replacement (all matches) or first match only
  - Full regex pattern support with capture groups
  - Handles missing fields gracefully
- **Configuration**:
  - `source_field`: Field to perform replacement on (default: "message")
  - `target_field`: Field to write result to (default: overwrites source)
  - `pattern`: Regex pattern to match (required)
  - `replacement`: Replacement string (default: empty string)
  - `global`: Replace all matches or just first (default: true)
- **Example**:
  ```json
  {
    "type": "regex_replace",
    "config": {
      "source_field": "message",
      "pattern": "[0-9]+",
      "replacement": "X",
      "global": true
    }
  }
  ```
- **Files Changed**:
  - `pipeline/filters.go:647-756` - Complete implementation
  - `pipeline/regex_replace_test.go` - Comprehensive test suite (6 test cases)
- **Tests**: All tests passing

**Updated Filter List:** `grok`, `mutate`, `drop`, `kv`, `date_parse`, `split`, `useragent`, `dns`, `geoip`, `fingerprint`, `regex_replace`

---

## 2025-10-30 - Major Feature: 10 New Transformation Filters

### New Filters - Logstash Compatibility

Added 10 powerful transformation filters to match Logstash functionality, enabling comprehensive data parsing and enrichment in the pipeline.

#### Phase 1: Critical Filters (5)

1. **Grok Filter** - Pattern-based parsing with 50+ built-in patterns
2. **Mutate Filter** - 13 field manipulation operations
3. **Drop Filter** - Conditional event dropping with sampling
4. **KV Filter** - Key-value pair parsing
5. **Date Parse Filter** - Timestamp parsing with multiple formats

#### Phase 2: Important Filters (5)

6. **Split Filter** - Split events into multiple records
7. **UserAgent Filter** - Parse user agent strings
8. **DNS Filter** - DNS enrichment with caching
9. **GeoIP Filter** - Geographic information from IPs
10. **Fingerprint Filter** - Event hashing for deduplication

See `FILTER_IMPLEMENTATION_PLAN.md` and `docs/filters/` for detailed documentation.

**Dependencies Added:**
- `github.com/ua-parser/uap-go` v0.0.0-20250917011043-9c86a9b0f8f0
- `github.com/oschwald/geoip2-golang` v1.13.0

**Filters Available:** `grok`, `mutate`, `drop`, `kv`, `date_parse`, `split`, `useragent`, `dns`, `geoip`, `fingerprint`

---

## 2025-10-29 - Authentication Fixes & AWS SDK Warning Suppression

### Bug Fixes

#### 🔇 AWS SDK Checksum Warning Suppression
- **Issue**: AWS SDK v2 logging checksum validation warnings when using MinIO
  - Warning: "WARN Response has no supported checksum. Not validating response payload."
  - Occurs on every S3 operation, clutters logs
- **Fix**: Added `DisableLogOutputChecksumValidationSkipped` flag to S3 client options
  - Applied to both MinIO endpoint and standard AWS S3 configurations
  - Completely suppresses checksum validation warnings
- **Impact**: Clean logs without AWS SDK warnings
- **Files Changed**:
  - `storage/s3_client.go:112,117` (added flag to both client configurations)

#### 🔧 Dataset Metrics Recording Authentication
- **Issue**: Dataset metrics recording to control service was failing with 401 Unauthorized errors
  - `DatasetMetricsClient` was not sending Authorization header with API key
  - Error logged: "Dataset metrics recording failed with status 401 for {tenant}/{dataset}"
  - Control service requires `Authorization: Bearer <api_key>` for dataset metrics endpoint
- **Fix**: Added API key authentication to dataset metrics client
  - Added `apiKey` field to `DatasetMetricsClient` struct
  - Updated `NewDatasetMetricsClient()` to accept `apiKey` parameter
  - Modified HTTP request creation to include Authorization header
  - Now uses `control_service.api_key` from configuration
- **Impact**: Dataset metrics successfully recorded to control service
- **Files Changed**:
  - `metrics/dataset_metrics.go:17,40,94-96` (added apiKey field and Authorization header)
  - `services/services.go:45` (pass API key to client constructor)

## 2025-10-29 - Control Service Authentication Fix

### Bug Fixes

#### 🔧 Control Service API Authentication
- **Issue**: Control service API calls were failing with 401 Unauthorized errors
  - Piper was using `X-API-Key` header for authentication
  - Control service requires `Authorization: Bearer <api_key>` format
  - Error logged: "failed to get active tenants: control service returned status 401 for accounts"
  - Affected all Control Service API calls (accounts, tenants, datasets)
- **Fix**: Updated all Control Service HTTP requests to use Bearer token authentication
  - Changed from `X-API-Key: <key>` to `Authorization: Bearer <key>`
  - Updated discovery manager (accounts, tenants, datasets endpoints)
  - Updated pipeline client (tenants and datasets endpoints)
  - Now uses standard Bearer token format expected by Control Service JWT middleware
- **Impact**: Piper can now successfully discover jobs from Control Service
- **Files Changed**:
  - `services/simple_discovery_manager.go:300,342,383` (changed to Authorization header)
  - `pipeline/client.go:153,197` (changed to Authorization header)

### Authentication Format

**Before** (Incorrect):
```go
req.Header.Set("X-API-Key", apiKey)
```

**After** (Correct):
```go
req.Header.Set("Authorization", "Bearer "+apiKey)
```

### Configuration
The piper uses the existing `control_service.api_key` configuration:
```yaml
control_service:
  api_key: "bytefreezer-service-api-key-..."  # System-wide service API key
```

This system API key is validated by the Control Service JWT middleware and grants system_admin privileges for accessing all accounts/tenants/datasets.
