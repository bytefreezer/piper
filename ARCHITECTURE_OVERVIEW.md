# ByteFreezer Three-Service Architecture Overview

## Executive Summary

This document presents a complete redesign of the ByteFreezer data processing architecture, evolving from a monolithic receiver service to a distributed three-service system optimized for scalability, reliability, and operational flexibility.

## Architecture Evolution

### Current State → Future State

```
BEFORE: Monolithic Processing
┌─────────────────────────────────────┐
│        bytefreezer-receiver         │
│  ┌─────────┐  ┌─────────┐  ┌──────┐ │
│  │Webhook  │→ │Pipeline │→ │ S3   │ │
│  │Ingestion│  │Filters  │  │Upload│ │
│  └─────────┘  └─────────┘  └──────┘ │
└─────────────────────────────────────┘

AFTER: Distributed Processing Pipeline
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│bytefreezer- │    │bytefreezer- │    │bytefreezer- │
│receiver     │    │piper        │    │packer       │
│             │    │             │    │             │
│ • Webhooks  │    │ • Pipeline  │    │ • Parquet   │
│ • Raw S3    │ →  │ • Filters   │ →  │ • Compress  │
│ • Fast      │    │ • Transform │    │ • Optimize  │
└─────────────┘    └─────────────┘    └─────────────┘
```

## Service Responsibilities

### 1. ByteFreezer-Receiver (Data Ingestion)
**Single Purpose**: Fast, reliable data ingestion

✅ **Core Functions**:
- Webhook endpoint handling
- Basic data validation  
- Raw NDJSON storage to S3
- High-throughput optimization

❌ **Removed Complexity**:
- Pipeline processing
- Data transformation
- Filter chains
- Complex configuration

**Benefits**:
- 🚀 **Performance**: 10x faster ingestion (no processing overhead)
- 🛡️ **Reliability**: Simpler = fewer failure points
- 📈 **Scalability**: Independent scaling for ingestion load

### 2. ByteFreezer-Piper (Data Transformation) 
**Single Purpose**: Data pipeline processing and transformation

✅ **Core Functions**:
- S3 raw data discovery and polling
- Pipeline filter chain execution
- Data enrichment (GeoIP, field transforms)
- Processed NDJSON output to S3
- Distributed coordination via DynamoDB

**Key Features**:
- 🔄 **Parallel Processing**: Multiple files concurrently
- 🔐 **File Locking**: Prevents duplicate processing
- 📊 **Pipeline Visibility**: Real-time job status tracking
- 🔧 **Hot Configuration**: Dynamic pipeline updates

### 3. ByteFreezer-Packer (Data Optimization)
**Single Purpose**: Parquet conversion and optimization

✅ **Core Functions**:
- Processed data batching and aggregation
- Parquet file generation with compression
- Intelligent file size optimization
- Columnar storage with indexing

**Enhanced Features**:
- 📦 **Smart Batching**: Size-based file merging
- ⚡ **Parallel Conversion**: Multiple parquet files concurrently
- 🎯 **Resource Management**: Memory-efficient streaming
- 📈 **Compression Optimization**: Best storage efficiency

## Data Flow Architecture

### S3 Data Layout
```
s3://bytefreezer-data/
├── raw/                    # Receiver → Raw ingested data
│   └── tenant=X/dataset=Y/year=2024/month=09/day=01/hour=14/
│       └── 20240901T143055Z-uuid.ndjson.gz
│
├── processed/              # Piper → Transformed data  
│   └── tenant=X/dataset=Y/year=2024/month=09/day=01/hour=14/
│       └── 20240901T143055Z-uuid-processed.ndjson.gz
│
└── parquet/               # Packer → Final optimized storage
    └── tenant=X/dataset=Y/year=2024/month=09/day=01/
        └── part-00001-uuid.parquet
```

### Processing Flow
```
1. Webhook Data → Receiver → S3 raw/
2. S3 raw/ → Piper → S3 processed/  
3. S3 processed/ → Packer → S3 parquet/
```

## DynamoDB Coordination

### Four Tables for Distributed State Management

1. **`bytefreezer-file-locks`**
   - File-level locking across services
   - Prevents duplicate processing
   - Auto-expiring TTL locks

2. **`bytefreezer-job-status`**
   - Complete job lifecycle tracking
   - Status: queued → processing → completed/failed
   - Cross-service job visibility

3. **`bytefreezer-pipeline-configs`**
   - Dynamic pipeline configurations per tenant/dataset
   - Hot reloading without service restart
   - Version control and validation

4. **`bytefreezer-service-coordination`**
   - Service discovery and health monitoring
   - Operational controls (enable/disable processing)
   - Cross-service communication

## Key Architectural Benefits

### 1. **Operational Excellence** 🎯
- **Independent Deployments**: Update one service without affecting others
- **Service-Specific Scaling**: Scale based on actual load patterns
- **Isolated Maintenance**: Take down processing without stopping ingestion
- **Granular Monitoring**: Service-specific metrics and alerting

### 2. **Fault Tolerance & Recovery** 🛡️
- **Data Persistence**: Complete data available at each stage
- **Replay Capability**: Reprocess data from any stage
- **Graceful Degradation**: Services continue independently
- **Automatic Recovery**: Jobs auto-retry with intelligent backoff

### 3. **Performance & Scalability** 🚀
- **Parallel Processing**: Each service processes independently
- **Resource Optimization**: Right-size each service for its workload
- **Bottleneck Isolation**: Identify and scale specific bottlenecks
- **Elastic Scaling**: Auto-scale based on queue depth

### 4. **Development & Testing** 🔧
- **Team Independence**: Teams can develop services separately
- **Isolated Testing**: Test each transformation independently
- **Staged Rollouts**: Deploy changes progressively
- **Clear Ownership**: Well-defined service boundaries

## Migration Strategy

### Phase 1: Infrastructure Setup
1. ✅ Design S3 data layout
2. ✅ Design DynamoDB schema  
3. ✅ Design service architecture
4. 🔄 Implement bytefreezer-piper service
5. 📋 Update bytefreezer-receiver (remove pipeline)
6. 📋 Update bytefreezer-packer (processed data input)

### Phase 2: Service Implementation
1. Implement DynamoDB state management
2. Migrate pipeline logic to piper service
3. Add distributed coordination
4. Implement monitoring and observability

### Phase 3: Migration & Testing
1. Parallel running (old + new architecture)
2. Data validation between approaches
3. Performance comparison and optimization
4. Gradual traffic migration

### Phase 4: Production Rollout
1. Blue-green deployment strategy
2. Real-time monitoring and alerting
3. Rollback procedures if needed
4. Legacy system cleanup

## Technology Stack

### Common Dependencies
- **AWS SDK v2**: S3, DynamoDB, Secrets Manager
- **Go 1.23+**: Latest Go runtime
- **OpenTelemetry**: Metrics and distributed tracing
- **Prometheus**: Metrics collection
- **Docker**: Containerized deployment
- **Kubernetes**: Orchestration platform

### Service-Specific
- **Piper**: GeoIP2-golang, filter processing libraries
- **Packer**: Apache Arrow, Parquet libraries  
- **All**: Structured logging, configuration management

## Monitoring & Observability

### Service-Level Metrics
- **Receiver**: Ingestion rate, webhook latency, error rates
- **Piper**: Processing latency, filter performance, job queue depth
- **Packer**: Conversion efficiency, file size optimization, throughput

### Cross-Service Metrics  
- **End-to-End Latency**: Webhook to parquet completion
- **Data Quality**: Record count preservation, error rates
- **Resource Utilization**: CPU, memory, network across services

### Alerting Strategy
- **Critical**: Service health, data loss prevention
- **Warning**: Performance degradation, queue backup
- **Info**: Operational events, configuration changes

## Next Steps

1. **Implement bytefreezer-piper service** with core functionality
2. **Update existing services** for new architecture
3. **Set up DynamoDB tables** with proper indexing
4. **Implement monitoring** and observability
5. **Create migration plan** with validation steps

This architecture transforms ByteFreezer into a modern, scalable, and maintainable data processing platform that can handle enterprise-scale workloads while providing operational excellence.