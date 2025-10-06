-- Cleanup script for unused database tables in piper
-- These tables were created without the piper_ prefix and are not used by the application
--
-- Created: 2025-10-04
-- Purpose: Remove legacy/unused tables that don't follow naming conventions

-- Drop unused tables (non-prefixed versions)
DROP TABLE IF EXISTS pipeline_configuration CASCADE;
DROP TABLE IF EXISTS pipeline_configurations CASCADE;
DROP TABLE IF EXISTS file_locks CASCADE;
DROP TABLE IF EXISTS job_records CASCADE;
DROP TABLE IF EXISTS tenants_cache CASCADE;

-- Drop any associated indexes
DROP INDEX IF EXISTS idx_pipeline_configuration_tenant_dataset;
DROP INDEX IF EXISTS idx_pipeline_configuration_expires_at;
DROP INDEX IF EXISTS idx_file_locks_ttl;
DROP INDEX IF EXISTS idx_job_records_status;
DROP INDEX IF EXISTS idx_job_records_tenant_dataset;
DROP INDEX IF EXISTS idx_job_records_created_at;
DROP INDEX IF EXISTS idx_tenants_cache_expires_at;
DROP INDEX IF EXISTS idx_tenants_cache_active;

-- Note: piper_ prefixed tables are the correct ones and should NOT be dropped:
-- - piper_file_locks
-- - piper_job_records
-- - piper_pipeline_configurations
-- - piper_tenants_cache
