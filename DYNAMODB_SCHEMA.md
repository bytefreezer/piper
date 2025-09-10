# DynamoDB Schema for Distributed Pipeline State

## Overview

This document defines the DynamoDB schema for coordinating the three-service ByteFreezer architecture. The schema supports distributed file locking, job state management, and cross-service coordination.

## Table Structure

### 1. File Processing Locks Table

**Table Name**: `bytefreezer-file-locks`  
**Purpose**: Prevent duplicate processing of files across service instances

```json
{
  "TableName": "bytefreezer-file-locks",
  "KeySchema": [
    {
      "AttributeName": "file_key",
      "KeyType": "HASH"
    }
  ],
  "AttributeDefinitions": [
    {
      "AttributeName": "file_key",
      "AttributeType": "S"
    },
    {
      "AttributeName": "processor_type",
      "AttributeType": "S"
    }
  ],
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "processor-type-index",
      "KeySchema": [
        {
          "AttributeName": "processor_type",
          "KeyType": "HASH"
        },
        {
          "AttributeName": "file_key",
          "KeyType": "RANGE"
        }
      ]
    }
  ],
  "TimeToLiveSpecification": {
    "AttributeName": "ttl",
    "Enabled": true
  }
}
```

**Record Structure**:
```go
type FileLock struct {
    FileKey         string    `dynamodbav:"file_key"`         // PK: S3 file path
    ProcessorType   string    `dynamodbav:"processor_type"`   // GSI: "piper" or "packer"
    ProcessorID     string    `dynamodbav:"processor_id"`     // Instance identifier
    JobID           string    `dynamodbav:"job_id"`           // Associated job ID
    LockTimestamp   time.Time `dynamodbav:"lock_timestamp"`
    TTL             int64     `dynamodbav:"ttl"`              // Auto-expiry (1 hour)
    LockVersion     int       `dynamodbav:"lock_version"`     // Optimistic locking
}
```

**Example Records**:
```json
{
  "file_key": "raw/tenant=acme/dataset=logs/year=2024/month=09/day=01/hour=14/20240901T143055Z-a1b2c3d4.ndjson.gz",
  "processor_type": "piper",
  "processor_id": "piper-instance-1-pod-abc123",
  "job_id": "job-piper-20240901-143055-001",
  "lock_timestamp": "2024-09-01T14:35:00Z",
  "ttl": 1725202500,
  "lock_version": 1
}
```

### 2. Job Status Table

**Table Name**: `bytefreezer-job-status`  
**Purpose**: Track processing jobs across all services with full lifecycle management

```json
{
  "TableName": "bytefreezer-job-status",
  "KeySchema": [
    {
      "AttributeName": "job_id",
      "KeyType": "HASH"
    }
  ],
  "AttributeDefinitions": [
    {
      "AttributeName": "job_id",
      "AttributeType": "S"
    },
    {
      "AttributeName": "tenant_id",
      "AttributeType": "S"
    },
    {
      "AttributeName": "status",
      "AttributeType": "S"
    },
    {
      "AttributeName": "created_at",
      "AttributeType": "S"
    },
    {
      "AttributeName": "processor_type",
      "AttributeType": "S"
    }
  ],
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "tenant-status-index",
      "KeySchema": [
        {
          "AttributeName": "tenant_id",
          "KeyType": "HASH"
        },
        {
          "AttributeName": "status",
          "KeyType": "RANGE"
        }
      ]
    },
    {
      "IndexName": "status-created-index",
      "KeySchema": [
        {
          "AttributeName": "status",
          "KeyType": "HASH"
        },
        {
          "AttributeName": "created_at",
          "KeyType": "RANGE"
        }
      ]
    },
    {
      "IndexName": "processor-type-status-index",
      "KeySchema": [
        {
          "AttributeName": "processor_type",
          "KeyType": "HASH"
        },
        {
          "AttributeName": "status",
          "KeyType": "RANGE"
        }
      ]
    }
  ],
  "TimeToLiveSpecification": {
    "AttributeName": "ttl",
    "Enabled": true
  }
}
```

**Record Structure**:
```go
type JobStatus struct {
    JobID           string            `dynamodbav:"job_id"`           // PK
    TenantID        string            `dynamodbav:"tenant_id"`        // GSI
    DatasetID       string            `dynamodbav:"dataset_id"`
    ProcessorType   string            `dynamodbav:"processor_type"`   // "piper" or "packer"
    ProcessorID     string            `dynamodbav:"processor_id"`     // Instance ID
    
    // Job lifecycle
    Status          string            `dynamodbav:"status"`           // queued, processing, completed, failed, retrying
    Priority        int               `dynamodbav:"priority"`         // 1-10 (10 = highest)
    RetryCount      int               `dynamodbav:"retry_count"`
    MaxRetries      int               `dynamodbav:"max_retries"`
    
    // Timestamps
    CreatedAt       time.Time         `dynamodbav:"created_at"`
    UpdatedAt       time.Time         `dynamodbav:"updated_at"`
    StartedAt       *time.Time        `dynamodbav:"started_at,omitempty"`
    CompletedAt     *time.Time        `dynamodbav:"completed_at,omitempty"`
    TTL             int64             `dynamodbav:"ttl"`              // 30 days retention
    
    // File references
    SourceFiles     []string          `dynamodbav:"source_files"`     // Input files
    OutputFiles     []string          `dynamodbav:"output_files"`     // Generated files
    
    // Processing metadata
    RecordCount     int64             `dynamodbav:"record_count"`
    ProcessingTime  int64             `dynamodbav:"processing_time_ms"`
    FileSize        int64             `dynamodbav:"file_size_bytes"`
    
    // Error handling
    ErrorMessage    string            `dynamodbav:"error_message,omitempty"`
    ErrorCode       string            `dynamodbav:"error_code,omitempty"`
    ErrorDetails    map[string]string `dynamodbav:"error_details,omitempty"`
    
    // Configuration
    PipelineVersion string            `dynamodbav:"pipeline_version,omitempty"`
    Configuration   map[string]interface{} `dynamodbav:"configuration,omitempty"`
}
```

**Job Status Values**:
- `queued` - Job created, waiting for processing
- `processing` - Currently being processed
- `completed` - Successfully completed
- `failed` - Failed after all retries
- `retrying` - Failed but will retry
- `cancelled` - Manually cancelled

### 3. Pipeline Configuration Table

**Table Name**: `bytefreezer-pipeline-configs`  
**Purpose**: Store and manage pipeline configurations per tenant/dataset

```json
{
  "TableName": "bytefreezer-pipeline-configs",
  "KeySchema": [
    {
      "AttributeName": "config_key",
      "KeyType": "HASH"
    }
  ],
  "AttributeDefinitions": [
    {
      "AttributeName": "config_key",
      "AttributeType": "S"
    },
    {
      "AttributeName": "tenant_id",
      "AttributeType": "S"
    },
    {
      "AttributeName": "updated_at",
      "AttributeType": "S"
    }
  ],
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "tenant-updated-index",
      "KeySchema": [
        {
          "AttributeName": "tenant_id",
          "KeyType": "HASH"
        },
        {
          "AttributeName": "updated_at",
          "KeyType": "RANGE"
        }
      ]
    }
  ]
}
```

**Record Structure**:
```go
type PipelineConfiguration struct {
    ConfigKey    string                 `dynamodbav:"config_key"`    // PK: "tenant#{tenant_id}#dataset#{dataset_id}"
    TenantID     string                 `dynamodbav:"tenant_id"`     // GSI
    DatasetID    string                 `dynamodbav:"dataset_id"`
    
    // Configuration
    Enabled      bool                   `dynamodbav:"enabled"`
    Version      string                 `dynamodbav:"version"`
    Filters      []FilterConfig         `dynamodbav:"filters"`
    
    // Metadata
    CreatedAt    time.Time              `dynamodbav:"created_at"`
    UpdatedAt    time.Time              `dynamodbav:"updated_at"`
    UpdatedBy    string                 `dynamodbav:"updated_by"`
    
    // Validation
    Checksum     string                 `dynamodbav:"checksum"`      // Config hash for change detection
    Validated    bool                   `dynamodbav:"validated"`
    
    // Settings
    Settings     map[string]interface{} `dynamodbav:"settings,omitempty"`
}

type FilterConfig struct {
    Type      string                 `json:"type"`
    Condition string                 `json:"condition,omitempty"`
    Config    map[string]interface{} `json:"config"`
    Enabled   bool                   `json:"enabled"`
}
```

### 4. Service Coordination Table

**Table Name**: `bytefreezer-service-coordination`  
**Purpose**: Cross-service coordination, service discovery, and operational control

```json
{
  "TableName": "bytefreezer-service-coordination",
  "KeySchema": [
    {
      "AttributeName": "coordination_key",
      "KeyType": "HASH"
    }
  ],
  "AttributeDefinitions": [
    {
      "AttributeName": "coordination_key",
      "AttributeType": "S"
    },
    {
      "AttributeName": "service_type",
      "AttributeType": "S"
    }
  ],
  "GlobalSecondaryIndexes": [
    {
      "IndexName": "service-type-index",
      "KeySchema": [
        {
          "AttributeName": "service_type",
          "KeyType": "HASH"
        },
        {
          "AttributeName": "coordination_key",
          "KeyType": "RANGE"
        }
      ]
    }
  ],
  "TimeToLiveSpecification": {
    "AttributeName": "ttl",
    "Enabled": true
  }
}
```

**Record Types**:

#### Service Instance Registration
```go
type ServiceInstance struct {
    CoordinationKey string            `dynamodbav:"coordination_key"` // "instance#{service_type}#{instance_id}"
    ServiceType     string            `dynamodbav:"service_type"`     // "receiver", "piper", "packer"
    InstanceID      string            `dynamodbav:"instance_id"`
    
    // Instance info
    Status          string            `dynamodbav:"status"`           // "healthy", "unhealthy", "starting", "stopping"
    Version         string            `dynamodbav:"version"`
    StartedAt       time.Time         `dynamodbav:"started_at"`
    LastHeartbeat   time.Time         `dynamodbav:"last_heartbeat"`
    TTL             int64             `dynamodbav:"ttl"`              // 5 minutes
    
    // Configuration
    Endpoint        string            `dynamodbav:"endpoint,omitempty"`
    Capabilities    []string          `dynamodbav:"capabilities,omitempty"`
    Configuration   map[string]interface{} `dynamodbav:"configuration,omitempty"`
}
```

#### Operational Controls
```go
type OperationalControl struct {
    CoordinationKey string            `dynamodbav:"coordination_key"` // "control#{control_type}"
    ServiceType     string            `dynamodbav:"service_type"`     // "global" or specific service
    
    // Control settings
    ControlType     string            `dynamodbav:"control_type"`     // "processing_enabled", "emergency_stop"
    Value           string            `dynamodbav:"value"`            // "true", "false", etc.
    
    // Metadata
    UpdatedAt       time.Time         `dynamodbav:"updated_at"`
    UpdatedBy       string            `dynamodbav:"updated_by"`
    Reason          string            `dynamodbav:"reason,omitempty"`
}
```

## Query Patterns

### 1. File Lock Operations
```go
// Acquire lock for file processing
func AcquireFileLock(fileKey, processorType, processorID, jobID string) error

// Release lock after processing
func ReleaseFileLock(fileKey, processorType string) error

// Check if file is locked
func IsFileLocked(fileKey, processorType string) bool

// Get all locks for a processor type
func GetLocksForProcessor(processorType string) []FileLock
```

### 2. Job Management Operations
```go
// Create new job
func CreateJob(job *JobStatus) error

// Update job status
func UpdateJobStatus(jobID, status string, metadata map[string]interface{}) error

// Get jobs by status
func GetJobsByStatus(status string, limit int) []JobStatus

// Get jobs for tenant
func GetJobsForTenant(tenantID string, limit int) []JobStatus

// Get failed jobs for retry
func GetFailedJobs(processorType string, maxRetries int) []JobStatus
```

### 3. Configuration Management
```go
// Get pipeline configuration
func GetPipelineConfig(tenantID, datasetID string) (*PipelineConfiguration, error)

// Update pipeline configuration  
func UpdatePipelineConfig(config *PipelineConfiguration) error

// Get all configurations for tenant
func GetTenantConfigurations(tenantID string) []PipelineConfiguration
```

### 4. Service Coordination
```go
// Register service instance
func RegisterServiceInstance(instance *ServiceInstance) error

// Update heartbeat
func UpdateHeartbeat(serviceType, instanceID string) error

// Get healthy instances
func GetHealthyInstances(serviceType string) []ServiceInstance

// Set operational control
func SetOperationalControl(controlType, value, reason string) error

// Get operational control
func GetOperationalControl(controlType string) (*OperationalControl, error)
```

## Data Access Patterns

### High-Frequency Operations (Hot Path)
1. **File lock acquisition/release** - Per file processing
2. **Job status updates** - Throughout job lifecycle
3. **Service heartbeats** - Every 30 seconds per instance

### Medium-Frequency Operations (Warm Path)
1. **Pipeline configuration retrieval** - Per job start
2. **Job queries by status** - Monitoring dashboards
3. **Service instance queries** - Health checks

### Low-Frequency Operations (Cold Path)
1. **Pipeline configuration updates** - Administrative changes
2. **Operational control changes** - Emergency operations
3. **Historical job queries** - Reporting and analytics

## Capacity Planning

### Read/Write Patterns
- **File Locks**: High write (acquire/release), Low read (check status)
- **Job Status**: High write (updates), Medium read (monitoring)
- **Pipeline Configs**: Low write, Medium read (cached)
- **Service Coordination**: Medium write (heartbeats), Low read

### Estimated Throughput
```yaml
# For 1000 files/hour processing rate
file_locks:
  read_capacity: 50   # Check locks
  write_capacity: 100 # Acquire/release locks

job_status:
  read_capacity: 100  # Monitoring queries
  write_capacity: 200 # Status updates

pipeline_configs:
  read_capacity: 20   # Configuration retrieval
  write_capacity: 5   # Configuration updates

service_coordination:
  read_capacity: 10   # Health checks
  write_capacity: 50  # Heartbeats
```

## Error Handling and Recovery

### Lock Management
- **Orphaned Locks**: TTL-based auto-expiry (1 hour)
- **Lock Conflicts**: Conditional writes prevent race conditions
- **Instance Crashes**: Locks auto-expire, jobs marked for retry

### Job Processing
- **Failed Jobs**: Automatic retry with exponential backoff
- **Stuck Jobs**: TTL-based cleanup and retry
- **Data Consistency**: Transaction support for multi-table updates

### Configuration Management
- **Invalid Configurations**: Validation before storage
- **Configuration Conflicts**: Version-based conflict resolution
- **Rollback Capability**: Configuration history preservation

This DynamoDB schema provides robust coordination for the distributed ByteFreezer architecture while maintaining high performance and reliability.