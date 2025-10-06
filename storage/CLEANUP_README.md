# Database Table Cleanup - ByteFreezer Piper

## Purpose

This cleanup script removes unused database tables that don't follow the `piper_` naming convention.

## Background

ByteFreezer Piper uses a `piper_` prefix for all database tables to avoid naming conflicts when multiple ByteFreezer services share the same database. However, some legacy tables were created without this prefix and are not used by the application.

## Tables Being Removed

The following tables are **unused** and will be dropped:

- `pipeline_configuration` / `pipeline_configurations` (unused, use `piper_pipeline_configurations` instead)
- `file_locks` (unused, use `piper_file_locks` instead)
- `job_records` (unused, use `piper_job_records` instead)
- `tenants_cache` (unused, use `piper_tenants_cache` instead)

## Tables That Will Remain

The following `piper_` prefixed tables are **actively used** and will NOT be dropped:

- ✅ `piper_file_locks` - File-level distributed locking for multi-instance coordination
- ✅ `piper_job_records` - Processing job tracking and status management
- ✅ `piper_pipeline_configurations` - Pipeline configuration caching
- ✅ `piper_tenants_cache` - Tenant metadata caching

## How to Run

### Option 1: Run directly with psql

```bash
psql -U postgres -d bytefreezer -f cleanup_unused_tables.sql
```

### Option 2: Run with Docker

```bash
docker exec -i bytefreezer-postgres psql -U postgres -d bytefreezer < cleanup_unused_tables.sql
```

### Option 3: Run from application

```bash
# Using piper binary with database migration
cd /home/andrew/workspace/bytefreezer/bytefreezer-piper
psql -U postgres -d bytefreezer -f storage/cleanup_unused_tables.sql
```

## Verification

After running the cleanup, verify the correct tables exist:

```sql
-- List all piper tables (should only show piper_ prefixed tables)
SELECT tablename
FROM pg_tables
WHERE schemaname = 'public'
  AND tablename LIKE '%piper%'
  OR tablename LIKE '%pipeline%'
  OR tablename LIKE '%file_locks%'
  OR tablename LIKE '%job_records%'
  OR tablename LIKE '%tenants_cache%'
ORDER BY tablename;
```

Expected output:
```
 tablename
-----------------------------------
 piper_file_locks
 piper_job_records
 piper_pipeline_configurations
 piper_tenants_cache
```

## Safety

- The cleanup script uses `CASCADE` to remove dependent objects
- The cleanup script uses `IF EXISTS` to prevent errors if tables don't exist
- All unused tables have been verified to have **zero references** in the codebase
- The script is idempotent and can be run multiple times safely

## Impact

**Zero impact** on running applications:
- No application code references the unused tables
- All active functionality uses the `piper_` prefixed tables
- Cleanup only affects unused database objects

## Date Created

2025-10-04

## Related Changes

- Phase 2: Control Service Integration
- Database naming convention standardization
- Removal of legacy unused database objects
