# S3 Data Layout for Three-Service Architecture

## Overview

This document defines the S3 data layout for the three-service ByteFreezer architecture:
- **bytefreezer-receiver** → Raw data ingestion
- **bytefreezer-piper** → Data transformation/filtering 
- **bytefreezer-packer** → Parquet conversion

## S3 Bucket Structure

```
s3://bytefreezer-data/
├── raw/                           # Raw ingested data (receiver output)
├── processed/                     # Transformed data (piper output)
├── parquet/                       # Final parquet files (packer output)
└── pipeline-state/                # Processing state and metadata
```

## Directory Structure Details

### 1. Raw Data (`raw/`)
**Source**: bytefreezer-receiver  
**Format**: NDJSON (.ndjson.gz)  
**Purpose**: Untransformed data as received from webhooks

```
raw/
└── tenant={tenant_id}/
    └── dataset={dataset_id}/
        └── year={YYYY}/month={MM}/day={DD}/hour={HH}/
            ├── {timestamp}-{uuid}.ndjson.gz
            ├── {timestamp}-{uuid}.ndjson.gz
            └── ...
```

**File Naming Convention**:
- `{timestamp}`: UTC timestamp in format `20240901T143055Z`
- `{uuid}`: Short UUID to prevent collisions
- Example: `20240901T143055Z-a1b2c3d4.ndjson.gz`

**File Content**:
```json
{"original_data": "as received", "timestamp": "2024-09-01T14:30:55Z"}
{"more": "raw data", "timestamp": "2024-09-01T14:30:56Z"}
```

### 2. Processed Data (`processed/`)
**Source**: bytefreezer-piper  
**Format**: NDJSON (.ndjson.gz)  
**Purpose**: Data after pipeline transformation but before parquet conversion

```
processed/
└── tenant={tenant_id}/
    └── dataset={dataset_id}/
        └── year={YYYY}/month={MM}/day={DD}/hour={HH}/
            ├── {raw_file_name}-processed.ndjson.gz
            ├── {raw_file_name}-processed.ndjson.gz
            └── ...
```

**File Naming Convention**:
- Preserves original raw file name with `-processed` suffix
- Example: `20240901T143055Z-a1b2c3d4-processed.ndjson.gz`

**File Content**:
```json
{"transformed_field": "value", "added_geoip": {...}, "_bytefreezer_processed_at": "2024-09-01T14:35:00Z"}
{"enriched": "data", "filtered_out_pii": true, "_bytefreezer_processed_at": "2024-09-01T14:35:01Z"}
```

### 3. Parquet Data (`parquet/`)
**Source**: bytefreezer-packer  
**Format**: Parquet (.parquet)  
**Purpose**: Final optimized columnar storage

```
parquet/
└── tenant={tenant_id}/
    └── dataset={dataset_id}/
        └── year={YYYY}/month={MM}/day={DD}/
            ├── part-00001-{uuid}.parquet
            ├── part-00002-{uuid}.parquet
            └── ...
```

**File Naming Convention**:
- `part-{sequence}-{uuid}.parquet`
- Multiple parquet files per day for optimal size management
- Example: `part-00001-a1b2c3d4.parquet`

### 4. Pipeline State (`pipeline-state/`)
**Source**: All services  
**Format**: JSON metadata files  
**Purpose**: Processing coordination and monitoring

```
pipeline-state/
├── processing-locks/              # Active processing locks
│   └── {tenant_id}-{dataset_id}-{timestamp}.lock
├── job-status/                   # Job processing status
│   └── {job_id}.json
└── pipeline-configs/             # Pipeline configurations
    └── {tenant_id}/
        └── {dataset_id}/
            └── config.json
```

## Data Flow and Processing States

### Stage 1: Raw Data Ingestion
```
Webhook → Receiver → raw/{tenant}/{dataset}/{year}/{month}/{day}/{hour}/{file}
```

### Stage 2: Pipeline Processing
```
raw/{file} → Piper → processed/{file}-processed
```

### Stage 3: Parquet Conversion
```
processed/{files} → Packer → parquet/{tenant}/{dataset}/{year}/{month}/{day}/part-*.parquet
```

## File Size and Batching Strategy

### Raw Files (Receiver)
- **Target Size**: 10-50MB compressed
- **Batching**: Time-based (5-15 minutes) or size-based
- **Compression**: gzip compression (.gz)

### Processed Files (Piper)
- **Size**: Similar to input raw files (may vary due to transformations)
- **1:1 Mapping**: Each raw file produces one processed file
- **Preservation**: Maintains temporal ordering

### Parquet Files (Packer)
- **Target Size**: 100-500MB per parquet file
- **Batching**: Multiple processed files combined
- **Optimization**: Columnar storage, compression, indexing

## Metadata and Lineage

### File Metadata
Each processing stage adds metadata:

**Raw Files**:
```json
{
  "_bytefreezer_ingested_at": "2024-09-01T14:30:55Z",
  "_bytefreezer_source": "webhook",
  "_bytefreezer_receiver_id": "receiver-instance-1"
}
```

**Processed Files**:
```json
{
  "_bytefreezer_processed_at": "2024-09-01T14:35:00Z",
  "_bytefreezer_piper_id": "piper-instance-2",
  "_bytefreezer_pipeline_version": "1.2.3",
  "_bytefreezer_source_file": "raw/tenant=123/dataset=logs/..."
}
```

**Parquet Files**:
```json
{
  "_bytefreezer_packed_at": "2024-09-01T14:40:00Z",
  "_bytefreezer_packer_id": "packer-instance-1",
  "_bytefreezer_source_files": ["processed/file1", "processed/file2"],
  "_bytefreezer_record_count": 50000,
  "_bytefreezer_compression_ratio": 0.25
}
```

## Data Retention Policy

### Raw Data
- **Retention**: 90 days (configurable)
- **Purpose**: Disaster recovery, reprocessing
- **Storage Class**: Standard → IA → Glacier

### Processed Data  
- **Retention**: 30 days (configurable)
- **Purpose**: Re-packing, debugging
- **Storage Class**: Standard → IA

### Parquet Data
- **Retention**: Indefinite (business data)
- **Purpose**: Analytics, queries
- **Storage Class**: Standard for recent, IA for older

## Cross-Service Coordination

### Processing State Files
Services coordinate through state files in `pipeline-state/`:

1. **Lock Files**: Prevent duplicate processing
2. **Job Status**: Track processing progress
3. **Configuration**: Pipeline settings per tenant/dataset

### Event-Driven Processing
- Services poll S3 for new files in their input directories
- Use S3 event notifications for real-time processing
- DynamoDB for distributed coordination

## Monitoring and Observability

### Metrics by Stage
- **Raw**: Ingestion rate, file sizes, error rates
- **Processed**: Transformation latency, filter statistics  
- **Parquet**: Conversion efficiency, compression ratios

### Data Quality Checks
- Schema validation at each stage
- Record count preservation/tracking
- Duplicate detection and handling

## Example Complete Flow

```
1. Webhook data received
   → raw/tenant=acme/dataset=logs/year=2024/month=09/day=01/hour=14/20240901T143055Z-a1b2c3d4.ndjson.gz

2. Piper processes raw file
   → processed/tenant=acme/dataset=logs/year=2024/month=09/day=01/hour=14/20240901T143055Z-a1b2c3d4-processed.ndjson.gz

3. Packer combines multiple processed files
   → parquet/tenant=acme/dataset=logs/year=2024/month=09/day=01/part-00001-x9y8z7w6.parquet
```

This layout provides:
- ✅ Complete data lineage tracking
- ✅ Independent service scaling
- ✅ Fault tolerance and recovery
- ✅ Optimal storage efficiency
- ✅ Easy monitoring and debugging