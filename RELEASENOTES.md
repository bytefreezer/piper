# ByteFreezer Piper - Release Notes

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
