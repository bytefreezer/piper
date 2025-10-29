# ByteFreezer Piper - Release Notes

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
