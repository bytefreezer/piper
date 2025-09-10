# ByteFreezer-Piper Service Architecture

## Overview

ByteFreezer-Piper is the data transformation service in the three-service architecture. It processes raw NDJSON data from S3, applies pipeline transformations, and outputs processed NDJSON data ready for parquet conversion.

## Core Responsibilities

1. **📥 Raw Data Discovery**: Poll S3 for new raw data files
2. **🔄 Pipeline Processing**: Apply filters, transformations, enrichments
3. **📤 Processed Data Output**: Write transformed data to S3 processed directory
4. **🔐 Distributed Coordination**: Use DynamoDB for file locking and state management
5. **📊 Monitoring**: Metrics, health checks, and processing visibility

## Service Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    ByteFreezer-Piper Service                     │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐   │
│  │  S3 Discovery   │  │ Pipeline Engine │  │ Output Manager  │   │
│  │     Manager     │  │                 │  │                 │   │
│  │                 │  │ • Filter Chain  │  │ • S3 Upload     │   │
│  │ • File Polling  │  │ • Transformers  │  │ • Retry Logic   │   │
│  │ • Lock Check    │  │ • Enrichment    │  │ • Metadata      │   │
│  │ • Job Queue     │  │ • Validation    │  │                 │   │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘   │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐   │
│  │ DynamoDB State  │  │ Config Manager  │  │  Monitoring     │   │
│  │    Manager      │  │                 │  │                 │   │
│  │                 │  │ • Tenant Config │  │ • Metrics       │   │
│  │ • File Locks    │  │ • Pipeline Defs │  │ • Health Checks │   │
│  │ • Job Status    │  │ • Hot Reload    │  │ • Tracing       │   │
│  │ • Coordination  │  │                 │  │                 │   │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. S3 Discovery Manager
**Purpose**: Continuously discover new raw data files for processing

```go
type S3DiscoveryManager struct {
    s3Client        *storage.S3SourceClient
    stateManager    *DynamoDBStateManager
    jobQueue        chan *ProcessingJob
    config          DiscoveryConfig
}

type ProcessingJob struct {
    JobID       string
    TenantID    string
    DatasetID   string
    SourceFile  S3Object
    Priority    int
    CreatedAt   time.Time
}
```

**Key Functions**:
- `ScanForNewFiles()` - Poll raw/ directory for unprocessed files
- `CheckFilelock()` - Verify file isn't being processed by another instance
- `QueueJob()` - Add processing job to internal queue
- `UpdateJobStatus()` - Track job progress in DynamoDB

### 2. Pipeline Engine
**Purpose**: Core data transformation engine (migrated from receiver)

```go
type PipelineEngine struct {
    filterRegistry  *FilterRegistry
    configManager   *ConfigManager
    geoipManager    *GeoIPManager
    metrics         *MetricsCollector
}

type FilterRegistry struct {
    filters map[string]FilterFactory
}

// All filters from receiver pipeline
type FilterTypes struct {
    AddField     *AddFieldFilter
    RemoveField  *RemoveFieldFilter  
    RenameField  *RenameFieldFilter
    JSONParse    *JSONParseFilter
    RegexReplace *RegexReplaceFilter
    DateParse    *DateParseFilter
    Conditional  *ConditionalFilter
    GeoIP        *GeoIPFilter
}
```

**Processing Flow**:
1. `ProcessFile(job *ProcessingJob)` - Main processing entry point
2. `LoadPipelineConfig(tenantID, datasetID)` - Get pipeline configuration  
3. `StreamProcess(reader io.Reader)` - Line-by-line NDJSON processing
4. `ApplyFilterChain(record map[string]interface{})` - Transform data
5. `WriteOutput(processedData)` - Stream to output file

### 3. Output Manager
**Purpose**: Handle processed data upload to S3 with reliability

```go
type OutputManager struct {
    s3Client     *storage.S3DestinationClient
    retryManager *RetryManager
    metadata     *MetadataManager
}

type ProcessedFile struct {
    SourceFile      string
    ProcessedFile   string
    RecordCount     int64
    ProcessingTime  time.Duration
    PipelineVersion string
    ProcessorID     string
}
```

**Key Functions**:
- `UploadProcessedFile()` - Upload to S3 processed/ directory
- `AddProcessingMetadata()` - Inject lineage and processing metadata
- `HandleUploadFailure()` - Retry logic with exponential backoff
- `UpdateJobStatus()` - Mark job as completed in DynamoDB

### 4. DynamoDB State Manager
**Purpose**: Distributed coordination and state management

```go
type DynamoDBStateManager struct {
    dynamoClient *dynamodb.Client
    lockTable    string
    jobTable     string
    configTable  string
}

// File processing lock
type FileLock struct {
    FileKey       string    `dynamodbav:"file_key"`        // PK
    LockedBy      string    `dynamodbav:"locked_by"`       // Service instance ID
    LockTimestamp time.Time `dynamodbav:"lock_timestamp"`
    TTL           int64     `dynamodbav:"ttl"`            // Auto-expiry
    JobID         string    `dynamodbav:"job_id"`
}

// Job processing status
type JobStatus struct {
    JobID         string    `dynamodbav:"job_id"`         // PK  
    TenantID      string    `dynamodbav:"tenant_id"`      // GSI
    DatasetID     string    `dynamodbav:"dataset_id"`
    Status        string    `dynamodbav:"status"`         // queued, processing, completed, failed
    SourceFile    string    `dynamodbav:"source_file"`
    ProcessedFile string    `dynamodbav:"processed_file,omitempty"`
    ProcessorID   string    `dynamodbav:"processor_id"`
    CreatedAt     time.Time `dynamodbav:"created_at"`
    UpdatedAt     time.Time `dynamodbav:"updated_at"`
    CompletedAt   time.Time `dynamodbav:"completed_at,omitempty"`
    ErrorMessage  string    `dynamodbav:"error_message,omitempty"`
    TTL           int64     `dynamodbav:"ttl"`
}
```

### 5. Config Manager  
**Purpose**: Dynamic pipeline configuration management

```go
type ConfigManager struct {
    controllerClient *ControllerClient
    cache           *sync.Map
    refreshInterval time.Duration
}

type PipelineConfig struct {
    TenantID    string         `json:"tenant_id"`
    DatasetID   string         `json:"dataset_id"`
    Enabled     bool           `json:"enabled"`
    Version     string         `json:"version"`
    Filters     []FilterConfig `json:"filters"`
    UpdatedAt   time.Time      `json:"updated_at"`
}
```

## Service Interfaces

### Main Service Interface
```go
type PiperService interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    GetStatus() ServiceStatus
    GetJobStatus(jobID string) (*JobStatus, error)
    GetMetrics() *MetricsSnapshot
}
```

### Processing Pipeline Interface
```go
type ProcessingPipeline interface {
    ProcessFile(ctx context.Context, job *ProcessingJob) error
    ValidateConfig(config *PipelineConfig) error
    GetPipelineMetrics(tenantID, datasetID string) *PipelineMetrics
}
```

## Configuration Structure

```yaml
# bytefreezer-piper configuration
app:
  name: "bytefreezer-piper"
  version: "1.0.0"
  instance_id: "piper-${HOSTNAME}"

# S3 source (raw data input)
s3_source:
  bucket_name: "bytefreezer-raw"
  region: "us-east-1"
  prefix: "raw/"
  poll_interval: "30s"

# S3 destination (processed data output)  
s3_destination:
  bucket_name: "bytefreezer-processed"
  region: "us-east-1"
  prefix: "processed/"

# DynamoDB coordination
dynamodb:
  lock_table: "piper-file-locks"
  job_table: "piper-job-status" 
  config_table: "piper-pipeline-configs"
  region: "us-east-1"

# Processing configuration
processing:
  max_concurrent_jobs: 10
  job_timeout: "30m"
  retry_attempts: 3
  retry_backoff: "exponential"

# Pipeline configuration
pipeline:
  controller_endpoint: "http://controller-api:8080"
  config_refresh_interval: "5m"
  geoip_database_path: "/opt/geoip"

# Monitoring
monitoring:
  metrics_port: 9090
  health_port: 8080
  log_level: "info"
```

## API Endpoints

### Health and Status
- `GET /health` - Service health check
- `GET /status` - Detailed service status
- `GET /metrics` - Prometheus metrics

### Job Management
- `GET /jobs` - List recent jobs
- `GET /jobs/{job_id}` - Get job details
- `POST /jobs/{job_id}/retry` - Retry failed job
- `DELETE /jobs/{job_id}` - Cancel queued job

### Pipeline Management
- `GET /pipelines` - List pipeline configurations
- `GET /pipelines/{tenant_id}/{dataset_id}` - Get pipeline config
- `POST /pipelines/{tenant_id}/{dataset_id}/reload` - Reload configuration
- `GET /pipelines/{tenant_id}/{dataset_id}/metrics` - Pipeline metrics

## Processing Flow

### 1. File Discovery Loop
```
1. Poll S3 raw/ directory for new files
2. Check DynamoDB for existing locks/jobs
3. Create job for unprocessed files
4. Queue job for processing
```

### 2. Job Processing
```
1. Acquire file lock in DynamoDB
2. Download raw file from S3
3. Load pipeline configuration for tenant/dataset
4. Stream process through filter chain
5. Upload processed file to S3
6. Update job status to completed
7. Release file lock
```

### 3. Error Handling
```
1. Failed jobs marked in DynamoDB
2. Retry logic with exponential backoff
3. Dead letter queue for persistent failures
4. Alert/notification for critical errors
```

## Monitoring and Observability

### Key Metrics
- **Processing Rate**: Files/minute, records/second
- **Latency**: End-to-end processing time per file
- **Error Rate**: Failed jobs percentage
- **Queue Depth**: Pending jobs count
- **Resource Usage**: CPU, memory, network

### Alerting
- Processing failures exceeding threshold
- Queue backup beyond limits
- DynamoDB errors or throttling
- S3 access failures

### Distributed Tracing
- Track job lifecycle across all components
- Pipeline filter execution tracing
- Cross-service correlation (receiver → piper → packer)

This architecture provides a robust, scalable, and maintainable data transformation service that integrates seamlessly with the three-service ByteFreezer architecture.