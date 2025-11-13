# ByteFreezer AI Agent - Technical Implementation Plan

**Version:** 1.0  
**Date:** November 6, 2025  
**Purpose:** Complete implementation guide for AI-powered configuration management  
**Target:** Claude Code CLI implementation

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [System Architecture](#system-architecture)
3. [Data Models](#data-models)
4. [API Specification](#api-specification)
5. [Project Structure](#project-structure)
6. [Implementation Phases](#implementation-phases)
7. [Key Implementation Details](#key-implementation-details)
8. [Testing Strategy](#testing-strategy)
9. [Configuration](#configuration)
10. [Deployment](#deployment)
11. [Security](#security)
12. [Monitoring](#monitoring)

---

## Executive Summary

### Problem Statement
ByteFreezer pipeline configuration is complex, especially for Piper. Manual YAML/JSON configuration is error-prone and time-consuming, creating a barrier to adoption.

### Solution
AI agent that generates, validates, and optimizes ByteFreezer configurations based on natural language intent. Agent observes actual data flow to validate configurations work correctly.

### Core Principles
1. **BYOA (Bring Your Own AI)** - Follows ByteFreezer's BYOB philosophy
2. **Batch-as-Transaction** - Configs applied at batch boundaries, not mid-stream
3. **Multi-tenant** - Per-tenant configs stored in Control DB
4. **Lifecycle Management** - Dev → Staging → Production promotion pipeline
5. **Free Tier** - Limited AI usage to demonstrate value, then upgrade

### Business Model

| Tier | Price | AI Config Gen | Validations | Optimizations | API Cost |
|------|-------|---------------|-------------|---------------|----------|
| **Free** | $0 | 5/month | 10/month | 3/month | Platform pays |
| **Pro** | $50/mo | Unlimited | Unlimited | Unlimited | Platform pays |
| **BYOA** | $0 | Unlimited | Unlimited | Unlimited | Customer pays Claude |
| **Enterprise** | Custom | Unlimited | Unlimited | Unlimited | Negotiable |

**Cost Analysis:**
- Free tier: ~$0.10-0.15 per user per month in API costs
- Expected conversion: 10-15% to Pro or BYOA
- ROI: 50x (revenue vs API costs)

---

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  ByteFreezer Users                       │
│              (Web UI / CLI / API)                        │
└───────────────────┬─────────────────────────────────────┘
                    │ HTTPS/REST
                    v
┌─────────────────────────────────────────────────────────┐
│              AI Agent Service (Go)                       │
│  ┌──────────────────────────────────────────────────┐  │
│  │  HTTP API Layer                                   │  │
│  │  - Authentication & Authorization                 │  │
│  │  - Rate Limiting                                  │  │
│  │  - Request Validation                             │  │
│  └──────────────────────────────────────────────────┘  │
│                                                          │
│  ┌──────────────┬──────────────┬────────────────────┐  │
│  │ Config Gen   │  Validator   │  Monitor/Optimize  │  │
│  │              │              │                    │  │
│  │ - Prompts    │ - Schema val │ - Metric analysis  │  │
│  │ - Caching    │ - Test runs  │ - Issue detection  │  │
│  │ - Parsing    │ - Output     │ - Recommendations  │  │
│  │              │   analysis   │                    │  │
│  └──────┬───────┴──────┬───────┴────────┬───────────┘  │
│         │               │                 │              │
│         v               v                 v              │
│  ┌─────────────────────────────────────────────┐       │
│  │      Claude API Client (Multi-tenant)        │       │
│  │  - Per-tenant API keys (platform or BYOA)   │       │
│  │  - Model selection (Sonnet/Haiku)           │       │
│  │  - Retry logic & error handling              │       │
│  │  - Cost tracking & quota enforcement         │       │
│  └─────────────────────────────────────────────┘       │
└──────────┬──────────────────────────┬───────────────────┘
           │                          │
           v                          v
    ┌─────────────┐          ┌──────────────────┐
    │ Control DB  │          │ Anthropic API    │
    │             │          │ (Claude)         │
    │ - Configs   │          │                  │
    │ - Quotas    │          │ OR               │
    │ - Usage     │          │                  │
    │ - API keys  │          │ Customer's       │
    └─────────────┘          │ Claude Account   │
           ↑                 └──────────────────┘
           │
           │ Read active config per batch
           │
    ┌──────┴─────────────┐
    │  ByteFreezer       │
    │  Receivers         │
    │                    │
    │  For each batch:   │
    │  1. Get tenant ID  │
    │  2. Read config    │
    │  3. Process batch  │
    │  4. Emit metrics   │
    └────────────────────┘
           ↓
    ┌────────────────────┐
    │  S3/MinIO Output   │
    │  (Parquet files)   │
    └────────────────────┘
```

### Data Flow

#### Config Generation Flow
```
User Intent
    ↓
Generate Config Request → Check Quota
    ↓ (if allowed)
Check Cache
    ↓ (cache miss)
Build Prompt → Call Claude API → Parse Response
    ↓
Validate Schema → Cache Result → Return to User
    ↓
Store in Control DB (dev environment)
```

#### Batch Processing Flow (Existing ByteFreezer)
```
Batch Arrives (tenant X)
    ↓
Read Active Config Version (tenant X)
    ↓
Read Config from Control DB
    ↓
Configure Pipeline (Piper + ByteFreezer)
    ↓
Process Batch (all-or-nothing transaction)
    ↓
Write Parquet to S3/MinIO
    ↓
Emit Metrics to Agent (for monitoring)
```

#### Monitoring & Optimization Flow
```
Periodic Check (every 5 min)
    ↓
Collect Metrics (last N batches)
    ↓
Detect Issues? (errors, data loss, slow queries)
    ↓ (if yes)
Sample Recent Output → Analyze with Claude
    ↓
Generate Improved Config → Store in Dev
    ↓
Notify User (Slack/Email)
```

---

## Data Models

### Control DB Schema

```yaml
# Tenant Configuration Document
tenant_configs:
  {tenant_id}:
    # Billing & Tier
    tier: "free" | "pro" | "byoa" | "enterprise"
    billing_cycle_start: "2025-11-01T00:00:00Z"
    created_at: "2025-10-15T08:30:00Z"
    
    # AI Provider Configuration
    ai_config:
      provider: "platform" | "byoa"
      # For BYOA only:
      api_key: "<AES-256 encrypted>"
      model: "claude-sonnet-4-5-20250929"
      endpoint: "https://api.anthropic.com/v1/messages"
    
    # Current Usage (resets monthly)
    usage:
      current_period:
        config_generations: 2
        validations: 5
        optimizations: 0
        total_tokens: 12450
        total_cost: 0.18
      last_reset: "2025-11-01T00:00:00Z"
      history:
        - period: "2025-10"
          config_generations: 5
          validations: 10
          total_tokens: 48230
          total_cost: 0.42
        - period: "2025-09"
          config_generations: 4
          validations: 8
          total_tokens: 39120
          total_cost: 0.35
    
    # Active Production Config
    active_version: 7
    
    # Environment-based Configs
    environments:
      dev:
        version: 8
        status: "testing" | "validated" | "failed"
        created_by: "ai_agent" | "user@example.com"
        created_at: "2025-11-06T10:30:00Z"
        config:
          # Full ByteFreezer + Piper Config
          listeners:
            - protocol: udp
              port: 6343
              buffer_size: 65536
              workers: 4
          
          storage:
            type: s3
            bucket: "acme-netflow-data"
            region: "us-west-2"
            path_template: "tenant={tenant}/year={year}/month={month}/day={day}/hour={hour}"
            compression: snappy
            row_group_size: 100000
            partitioning_keys:
              - tenant
              - year
              - month
              - day
              - hour
          
          piper:
            parsers:
              - name: sflow_parser
                type: sflow_v5
                output_fields:
                  - src_ip
                  - dst_ip
                  - src_port
                  - dst_port
                  - protocol
                  - bytes
                  - packets
            
            transformers:
              - name: ip_to_subnet
                type: cidr_mask
                input: src_ip
                output: src_net24
                mask: 24
              
              - name: ip_to_subnet_dst
                type: cidr_mask
                input: dst_ip
                output: dst_net24
                mask: 24
            
            aggregations:
              - name: flow_stats
                window: 5m
                group_by:
                  - src_net24
                  - dst_net24
                  - protocol
                aggregates:
                  - field: bytes
                    function: sum
                  - field: packets
                    function: sum
                  - field: src_ip
                    function: count_distinct
        
        # Validation Results
        validation_results:
          schema_valid: true
          no_data_loss: true
          data_loss_percent: 2.5
          partitioning_sane: true
          partition_count: 256
          query_latency_ms: 120
          memory_usage_mb: 1847
          issues: []
          sample_output:
            schema:
              - name: src_net24
                type: string
              - name: dst_net24
                type: string
              - name: protocol
                type: int32
              - name: bytes_sum
                type: int64
              - name: packets_sum
                type: int64
            rows:
              - src_net24: "10.0.1.0/24"
                dst_net24: "192.168.1.0/24"
                protocol: 6
                bytes_sum: 1048576
                packets_sum: 1024
          validated_at: "2025-11-06T10:35:00Z"
      
      staging:
        version: 7
        status: "validated"
        created_by: "user@acme.com"
        created_at: "2025-11-05T14:20:00Z"
        promoted_from: "dev"
        promoted_at: "2025-11-05T18:00:00Z"
        config: {...}
        validation_results: {...}
      
      production:
        version: 7
        status: "active"
        created_by: "user@acme.com"
        created_at: "2025-11-04T09:15:00Z"
        promoted_from: "staging"
        promoted_at: "2025-11-04T12:00:00Z"
        config: {...}
        production_metrics:
          batches_processed: 14523
          average_latency_ms: 185
          error_rate: 0.002
          data_loss_rate: 0.015
          last_updated: "2025-11-06T11:00:00Z"
    
    # Version History
    history:
      - version: 6
        config: {...}
        retired_at: "2025-11-03T14:20:00Z"
        retired_by: "ai_agent"
        reason: "cardinality_explosion_on_user_agent"
        rollback: false
      
      - version: 5
        config: {...}
        retired_at: "2025-10-28T09:15:00Z"
        retired_by: "user@acme.com"
        reason: "promoted_v6"
        rollback: false
      
      - version: 4
        config: {...}
        retired_at: "2025-10-25T16:45:00Z"
        retired_by: "system"
        reason: "emergency_rollback_from_v5"
        rollback: true
```

### Go Data Structures

```go
// Tenant represents a ByteFreezer tenant
type Tenant struct {
    ID                 string                        `json:"id"`
    Tier               Tier                          `json:"tier"`
    BillingCycleStart  time.Time                     `json:"billing_cycle_start"`
    CreatedAt          time.Time                     `json:"created_at"`
    AIConfig           AIConfig                      `json:"ai_config"`
    Usage              Usage                         `json:"usage"`
    ActiveVersion      int                           `json:"active_version"`
    Environments       map[string]Environment        `json:"environments"`
    History            []HistoryEntry                `json:"history"`
}

// Tier represents subscription tier
type Tier string

const (
    TierFree       Tier = "free"
    TierPro        Tier = "pro"
    TierBYOA       Tier = "byoa"
    TierEnterprise Tier = "enterprise"
)

// AIConfig represents AI provider configuration
type AIConfig struct {
    Provider string `json:"provider"` // "platform" or "byoa"
    APIKey   string `json:"api_key,omitempty"`   // Encrypted, only for BYOA
    Model    string `json:"model"`
    Endpoint string `json:"endpoint,omitempty"`
}

// Usage tracks tenant's AI usage
type Usage struct {
    CurrentPeriod UsagePeriod   `json:"current_period"`
    LastReset     time.Time     `json:"last_reset"`
    History       []UsagePeriod `json:"history"`
}

type UsagePeriod struct {
    Period            string  `json:"period,omitempty"` // "2025-10"
    ConfigGenerations int     `json:"config_generations"`
    Validations       int     `json:"validations"`
    Optimizations     int     `json:"optimizations"`
    TotalTokens       int     `json:"total_tokens"`
    TotalCost         float64 `json:"total_cost"`
}

// Environment represents dev/staging/production
type Environment struct {
    Version           int                    `json:"version"`
    Status            string                 `json:"status"` // testing, validated, failed, active
    CreatedBy         string                 `json:"created_by"`
    CreatedAt         time.Time              `json:"created_at"`
    PromotedFrom      string                 `json:"promoted_from,omitempty"`
    PromotedAt        *time.Time             `json:"promoted_at,omitempty"`
    Config            map[string]interface{} `json:"config"`
    ValidationResults *ValidationResults     `json:"validation_results,omitempty"`
    ProductionMetrics *ProductionMetrics     `json:"production_metrics,omitempty"`
}

// ValidationResults from test runs
type ValidationResults struct {
    SchemaValid       bool                     `json:"schema_valid"`
    NoDataLoss        bool                     `json:"no_data_loss"`
    DataLossPercent   float64                  `json:"data_loss_percent"`
    PartitioningSane  bool                     `json:"partitioning_sane"`
    PartitionCount    int                      `json:"partition_count"`
    QueryLatencyMs    int                      `json:"query_latency_ms"`
    MemoryUsageMb     int                      `json:"memory_usage_mb"`
    Issues            []string                 `json:"issues"`
    SampleOutput      map[string]interface{}   `json:"sample_output"`
    ValidatedAt       time.Time                `json:"validated_at"`
}

// ProductionMetrics from live traffic
type ProductionMetrics struct {
    BatchesProcessed   int       `json:"batches_processed"`
    AverageLatencyMs   int       `json:"average_latency_ms"`
    ErrorRate          float64   `json:"error_rate"`
    DataLossRate       float64   `json:"data_loss_rate"`
    LastUpdated        time.Time `json:"last_updated"`
}

// HistoryEntry tracks config changes
type HistoryEntry struct {
    Version    int                    `json:"version"`
    Config     map[string]interface{} `json:"config"`
    RetiredAt  time.Time              `json:"retired_at"`
    RetiredBy  string                 `json:"retired_by"`
    Reason     string                 `json:"reason"`
    Rollback   bool                   `json:"rollback"`
}

// Quota defines tier limits
type Quota struct {
    ConfigGenerations int     // -1 = unlimited
    Validations       int
    Optimizations     int
    TotalTokens       int
    MonthlyPrice      float64
}

var TierQuotas = map[Tier]Quota{
    TierFree: {
        ConfigGenerations: 5,
        Validations:       10,
        Optimizations:     3,
        TotalTokens:       50000,
        MonthlyPrice:      0,
    },
    TierPro: {
        ConfigGenerations: -1,
        Validations:       -1,
        Optimizations:     -1,
        TotalTokens:       -1,
        MonthlyPrice:      50,
    },
    TierBYOA: {
        ConfigGenerations: -1,
        Validations:       -1,
        Optimizations:     -1,
        TotalTokens:       -1,
        MonthlyPrice:      0,
    },
}
```

### Claude API Models

```go
// Claude API Request
type ClaudeRequest struct {
    Model       string    `json:"model"`
    MaxTokens   int       `json:"max_tokens"`
    Messages    []Message `json:"messages"`
    Temperature float64   `json:"temperature,omitempty"`
}

type Message struct {
    Role    string `json:"role"`    // "user" or "assistant"
    Content string `json:"content"`
}

// Claude API Response
type ClaudeResponse struct {
    ID      string `json:"id"`
    Type    string `json:"type"`
    Role    string `json:"role"`
    Content []struct {
        Type string `json:"type"`
        Text string `json:"text"`
    } `json:"content"`
    Model        string `json:"model"`
    StopReason   string `json:"stop_reason"`
    StopSequence string `json:"stop_sequence,omitempty"`
    Usage        Usage  `json:"usage"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}

// Model pricing (per million tokens)
var ModelPricing = map[string]struct {
    Input  float64
    Output float64
}{
    "claude-sonnet-4-5-20250929": {
        Input:  3.0,
        Output: 15.0,
    },
    "claude-haiku-4-20250514": {
        Input:  0.25,
        Output: 1.25,
    },
}
```

---

## API Specification

### Base URL
```
https://api.bytefreezer.io/v1
```

### Authentication
All requests require Bearer token authentication:
```
Authorization: Bearer <tenant_api_token>
```

### Common Response Codes
- `200 OK` - Success
- `400 Bad Request` - Invalid input
- `401 Unauthorized` - Missing or invalid auth token
- `403 Forbidden` - Quota exceeded or insufficient permissions
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Claude API unavailable

### Endpoints

#### 1. Generate Configuration

**Request:**
```http
POST /tenants/{tenant_id}/configs/generate
Content-Type: application/json
Authorization: Bearer <token>

{
  "intent": "Ingest sFlow v5 data, aggregate flows by source /24 subnet, 5-minute time windows, store in Parquet format",
  "sample_data": "<hex dump or base64 encoded sample>",
  "constraints": {
    "max_memory_mb": 2048,
    "retention_days": 90,
    "expected_throughput_mbps": 100,
    "query_patterns": ["top talkers", "traffic by subnet"]
  }
}
```

**Response (Success):**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "version": 8,
  "environment": "dev",
  "config": {
    "listeners": [
      {
        "protocol": "udp",
        "port": 6343,
        "buffer_size": 65536,
        "workers": 4
      }
    ],
    "storage": {
      "type": "s3",
      "bucket": "acme-netflow-data",
      "compression": "snappy",
      "partitioning_keys": ["tenant", "year", "month", "day"]
    },
    "piper": {
      "parsers": [...],
      "transformers": [...],
      "aggregations": [...]
    }
  },
  "confidence": 0.85,
  "generated_at": "2025-11-06T10:30:00Z",
  "from_cache": false,
  "estimated_cost": 0.018,
  "quota": {
    "remaining_generations": 3,
    "remaining_validations": 8,
    "remaining_optimizations": 3,
    "resets_at": "2025-12-01T00:00:00Z"
  }
}
```

**Response (Quota Exceeded):**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "quota_exceeded",
  "message": "Free tier limit reached: 5/5 config generations used this month",
  "quota_exceeded": true,
  "current_usage": {
    "config_generations": 5,
    "validations": 7,
    "optimizations": 2
  },
  "resets_at": "2025-12-01T00:00:00Z",
  "upgrade_options": {
    "pro": {
      "name": "Pro Plan",
      "price": "$50/month",
      "features": [
        "Unlimited AI config generations",
        "Unlimited validations",
        "Unlimited optimizations",
        "Email support"
      ],
      "url": "https://bytefreezer.io/upgrade?tier=pro"
    },
    "byoa": {
      "name": "Bring Your Own API",
      "price": "$0/month (usage-based via your Claude account)",
      "features": [
        "Unlimited usage",
        "Use your own Claude API key",
        "Full control over AI costs",
        "Same features as Pro"
      ],
      "url": "https://bytefreezer.io/settings/ai-config"
    }
  }
}
```

#### 2. Validate Configuration

**Request:**
```http
POST /tenants/{tenant_id}/configs/{version}/validate
Content-Type: application/json
Authorization: Bearer <token>

{
  "environment": "dev",
  "test_data_path": "s3://test-bucket/sample-sflow-1000-packets.pcap",
  "test_duration_seconds": 30
}
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "version": 8,
  "environment": "dev",
  "status": "passed",
  "results": {
    "schema_valid": true,
    "no_data_loss": true,
    "data_loss_percent": 2.5,
    "partitioning_sane": true,
    "partition_count": 256,
    "query_latency_ms": 120,
    "memory_usage_mb": 1847,
    "throughput_mbps": 95
  },
  "issues": [],
  "warnings": [
    "Partition count is approaching recommended limit (256 vs 1000 max)"
  ],
  "sample_output": {
    "schema": [
      {"name": "src_net24", "type": "string"},
      {"name": "dst_net24", "type": "string"},
      {"name": "protocol", "type": "int32"},
      {"name": "bytes_sum", "type": "int64"},
      {"name": "packets_sum", "type": "int64"}
    ],
    "row_count": 256,
    "sample_rows": [
      {
        "src_net24": "10.0.1.0/24",
        "dst_net24": "192.168.1.0/24",
        "protocol": 6,
        "bytes_sum": 1048576,
        "packets_sum": 1024
      }
    ]
  },
  "validated_at": "2025-11-06T10:35:00Z",
  "validation_duration_ms": 4523
}
```

#### 3. Promote Configuration

**Request:**
```http
POST /tenants/{tenant_id}/configs/{version}/promote
Content-Type: application/json
Authorization: Bearer <token>

{
  "from_environment": "dev",
  "to_environment": "staging"
}
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "version": 8,
  "promoted_from": "dev",
  "promoted_to": "staging",
  "promoted_at": "2025-11-06T10:40:00Z",
  "promoted_by": "user@acme.com",
  "previous_staging_version": 7
}
```

#### 4. Rollback Configuration

**Request:**
```http
POST /tenants/{tenant_id}/configs/rollback
Content-Type: application/json
Authorization: Bearer <token>

{
  "to_version": 7,
  "reason": "high_error_rate_in_production",
  "force": false
}
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "rolled_back_from": 8,
  "rolled_back_to": 7,
  "active_version": 7,
  "reason": "high_error_rate_in_production",
  "rolled_back_at": "2025-11-06T11:00:00Z",
  "rolled_back_by": "user@acme.com"
}
```

#### 5. Set AI Configuration (BYOA)

**Request:**
```http
PUT /tenants/{tenant_id}/ai-config
Content-Type: application/json
Authorization: Bearer <token>

{
  "provider": "anthropic",
  "api_key": "sk-ant-api03-...",
  "model": "claude-sonnet-4-5-20250929"
}
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "provider": "anthropic",
  "model": "claude-sonnet-4-5-20250929",
  "validated": true,
  "tier_updated_to": "byoa",
  "unlimited_usage": true,
  "message": "Your Claude API key has been validated and saved. You now have unlimited AI features."
}
```

#### 6. Get Usage Statistics

**Request:**
```http
GET /tenants/{tenant_id}/usage
Authorization: Bearer <token>
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "tier": "free",
  "current_period": {
    "starts_at": "2025-11-01T00:00:00Z",
    "ends_at": "2025-12-01T00:00:00Z",
    "config_generations": {
      "used": 5,
      "limit": 5,
      "remaining": 0
    },
    "validations": {
      "used": 7,
      "limit": 10,
      "remaining": 3
    },
    "optimizations": {
      "used": 1,
      "limit": 3,
      "remaining": 2
    },
    "total_tokens": 42000,
    "estimated_cost": 0.38
  },
  "history": [
    {
      "period": "2025-10",
      "config_generations": 5,
      "validations": 10,
      "total_tokens": 48230,
      "estimated_cost": 0.42
    }
  ]
}
```

#### 7. Get Active Configuration

**Request:**
```http
GET /tenants/{tenant_id}/configs/active
Authorization: Bearer <token>
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "version": 7,
  "environment": "production",
  "config": {
    "listeners": [...],
    "storage": {...},
    "piper": {...}
  },
  "activated_at": "2025-11-04T12:00:00Z",
  "metrics": {
    "batches_processed": 14523,
    "average_latency_ms": 185,
    "error_rate": 0.002,
    "last_updated": "2025-11-06T11:00:00Z"
  }
}
```

#### 8. List Configuration History

**Request:**
```http
GET /tenants/{tenant_id}/configs/history?limit=10
Authorization: Bearer <token>
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "tenant_id": "acme",
  "history": [
    {
      "version": 8,
      "environment": "dev",
      "status": "testing",
      "created_at": "2025-11-06T10:30:00Z",
      "created_by": "ai_agent"
    },
    {
      "version": 7,
      "environment": "production",
      "status": "active",
      "created_at": "2025-11-04T09:15:00Z",
      "promoted_from": "staging",
      "promoted_at": "2025-11-04T12:00:00Z"
    },
    {
      "version": 6,
      "environment": "retired",
      "status": "retired",
      "retired_at": "2025-11-03T14:20:00Z",
      "reason": "cardinality_explosion_on_user_agent"
    }
  ]
}
```

#### 9. Admin: Get Cost Statistics

**Request:**
```http
GET /admin/costs?period=month&group_by=tenant
Authorization: Bearer <admin_token>
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "period": "2025-11",
  "total_cost": 245.67,
  "total_requests": 1847,
  "average_cost_per_request": 0.133,
  "by_tier": {
    "free": {
      "cost": 89.23,
      "requests": 892,
      "tenants": 594
    },
    "pro": {
      "cost": 156.44,
      "requests": 955,
      "tenants": 31
    },
    "byoa": {
      "cost": 0.00,
      "requests": 423,
      "tenants": 18
    }
  },
  "by_tenant": {
    "acme": 12.45,
    "widgets_inc": 8.90,
    "techcorp": 15.67
  },
  "by_model": {
    "claude-sonnet-4-5-20250929": 198.34,
    "claude-haiku-4-20250514": 47.33
  }
}
```

---

## Project Structure

```
bytefreezer-agent/
├── go.mod
├── go.sum
├── main.go                      # Entry point
├── config/
│   ├── config.go                # Application configuration
│   └── config.yaml              # Config file (optional)
├── agent/
│   ├── client.go                # Claude API client
│   ├── config_gen.go            # Config generation logic
│   ├── validator.go             # Config validation
│   ├── monitor.go               # Continuous monitoring & optimization
│   ├── prompts.go               # Prompt templates
│   └── cache.go                 # Caching layer
├── cost/
│   ├── tracker.go               # Usage & cost tracking
│   ├── quota.go                 # Quota enforcement
│   └── stats.go                 # Statistics aggregation
├── controldb/
│   ├── interface.go             # DB interface
│   ├── postgres.go              # PostgreSQL implementation
│   ├── memory.go                # In-memory implementation (testing)
│   └── models.go                # Data models
├── crypto/
│   └── encryption.go            # API key encryption/decryption
├── server/
│   ├── server.go                # HTTP server setup
│   ├── api.go                   # API handlers
│   ├── middleware.go            # Auth, rate limiting, logging
│   └── routes.go                # Route definitions
├── pkg/
│   ├── errors/
│   │   └── errors.go            # Custom error types
│   ├── logger/
│   │   └── logger.go            # Structured logging
│   └── metrics/
│       └── metrics.go           # Prometheus metrics
└── testing/
    ├── fixtures/
    │   ├── sample_configs.yaml
    │   └── sample_data.pcap
    ├── integration_test.go
    └── load_test.go
```

### Key Files Detail

#### `main.go`
```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/yourusername/bytefreezer-agent/agent"
    "github.com/yourusername/bytefreezer-agent/config"
    "github.com/yourusername/bytefreezer-agent/controldb"
    "github.com/yourusername/bytefreezer-agent/cost"
    "github.com/yourusername/bytefreezer-agent/server"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Initialize components
    db := controldb.NewPostgresClient(cfg.DatabaseURL)
    defer db.Close()

    tracker := cost.NewTracker(db)
    quotaManager := cost.NewQuotaManager(db, tracker)

    claudeClient := agent.NewClaudeClient(cfg.AnthropicAPIKey, tracker)
    generator := agent.NewConfigGenerator(claudeClient, quotaManager, db)
    validator := agent.NewValidator(claudeClient, quotaManager, db)
    monitor := agent.NewMonitor(claudeClient, db, tracker)

    // Start background services
    go monitor.Start(context.Background())
    go quotaManager.StartResetScheduler(context.Background())

    // Start HTTP server
    srv := server.New(cfg, generator, validator, db, tracker)
    httpServer := &http.Server{
        Addr:         cfg.Server.Address,
        Handler:      srv.Handler(),
        ReadTimeout:  cfg.Server.ReadTimeout,
        WriteTimeout: cfg.Server.WriteTimeout,
        IdleTimeout:  cfg.Server.IdleTimeout,
    }

    // Graceful shutdown
    go func() {
        log.Printf("Starting ByteFreezer Agent API on %s", cfg.Server.Address)
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := httpServer.Shutdown(ctx); err != nil {
        log.Fatal("Server forced to shutdown:", err)
    }

    log.Println("Server exited")
}
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1)
**Goal:** Basic config generation with platform API key

**Tasks:**
- [ ] Project setup
  - [ ] Initialize Go module
  - [ ] Set up directory structure
  - [ ] Add dependencies (gorilla/mux, lib/pq, etc.)
- [ ] Claude API client
  - [ ] HTTP client with timeout and retries
  - [ ] Request/response models
  - [ ] Error handling and logging
  - [ ] Basic cost tracking
- [ ] Control DB interface
  - [ ] Define interface
  - [ ] In-memory implementation (for testing)
  - [ ] Tenant CRUD operations
  - [ ] Config CRUD operations
- [ ] Config generator
  - [ ] Prompt template system
  - [ ] YAML parsing and validation
  - [ ] Basic response caching
  - [ ] Generate endpoint implementation
- [ ] HTTP API server
  - [ ] Server setup with middleware
  - [ ] Authentication middleware
  - [ ] `/configs/generate` endpoint
  - [ ] Basic error responses
- [ ] Testing
  - [ ] Unit tests for core components
  - [ ] Integration test for end-to-end flow

**Deliverable:** API that can generate ByteFreezer configs using platform Claude API key

**Success Criteria:**
- Generate valid YAML config from natural language intent
- Response time < 5 seconds
- Basic cost tracking working

---

### Phase 2: Quota System (Week 2)
**Goal:** Free tier with limits, upgrade paths

**Tasks:**
- [ ] Quota manager
  - [ ] Tier definitions and quota limits
  - [ ] Usage tracking per tenant/operation
  - [ ] Monthly reset logic
  - [ ] Quota check before API calls
  - [ ] Increment usage after successful calls
- [ ] Update API responses
  - [ ] Include quota info in success responses
  - [ ] Quota exceeded error with upgrade options
  - [ ] Remaining quota display
- [ ] Usage tracking
  - [ ] Token counting and cost calculation
  - [ ] Usage history storage
  - [ ] `/usage` endpoint
- [ ] Testing
  - [ ] Test quota enforcement
  - [ ] Test monthly reset
  - [ ] Test different tier behaviors

**Deliverable:** Free tier with 5 config generations, 10 validations, 3 optimizations per month

**Success Criteria:**
- Quota correctly enforced for free tier
- Clear upgrade messaging when quota exceeded
- Monthly reset working correctly

---

### Phase 3: BYOA Support (Week 2-3)
**Goal:** Customers can use their own Claude API keys

**Tasks:**
- [ ] API key encryption
  - [ ] AES-256 encryption implementation
  - [ ] Master key management
  - [ ] Secure key derivation
- [ ] Multi-tenant API client
  - [ ] Per-tenant key lookup from DB
  - [ ] Dynamic API key selection
  - [ ] Error handling for invalid keys
- [ ] AI config management
  - [ ] `PUT /ai-config` endpoint
  - [ ] Validate API key before storing
  - [ ] Test key with Claude API
  - [ ] Encrypt and store in DB
  - [ ] Update tier to BYOA
- [ ] BYOA tier logic
  - [ ] Unlimited quota for BYOA users
  - [ ] Track usage (for visibility, not billing)
  - [ ] Cost attribution to customer
- [ ] Testing
  - [ ] Test encryption/decryption
  - [ ] Test BYOA flow end-to-end
  - [ ] Test invalid key handling

**Deliverable:** Customers can provide their own Claude API key for unlimited usage

**Success Criteria:**
- API keys encrypted at rest
- BYOA users have unlimited quota
- Usage tracked but not billed to platform

---

### Phase 4: Validation & Promotion (Week 3)
**Goal:** Config lifecycle management (dev → staging → production)

**Tasks:**
- [ ] Validator service
  - [ ] Schema validation
  - [ ] ByteFreezer dry-run integration
  - [ ] Test data processing
  - [ ] Output analysis (data loss, partitioning, performance)
  - [ ] Issue detection with Claude
- [ ] Environment management
  - [ ] Support dev/staging/production environments
  - [ ] Config versioning system
  - [ ] Promotion workflow
  - [ ] Rollback functionality
- [ ] Validation API
  - [ ] `POST /configs/{version}/validate` endpoint
  - [ ] `POST /configs/{version}/promote` endpoint
  - [ ] `POST /configs/rollback` endpoint
  - [ ] `GET /configs/history` endpoint
- [ ] Control DB updates
  - [ ] Store validation results
  - [ ] Track promotion history
  - [ ] Version history
- [ ] Testing
  - [ ] Test validation logic
  - [ ] Test promotion workflow
  - [ ] Test rollback

**Deliverable:** Full config lifecycle with validation and promotion

**Success Criteria:**
- Configs validated before promotion
- Clean dev → staging → production flow
- Rollback works correctly
- Version history tracked

---

### Phase 5: Continuous Monitoring (Week 4)
**Goal:** AI agent monitors production and suggests optimizations

**Tasks:**
- [ ] Monitoring service
  - [ ] Collect metrics from ByteFreezer receivers
  - [ ] Detect anomalies (errors, data loss, performance)
  - [ ] Sample output data from S3
  - [ ] Periodic health checks (every 5 min)
- [ ] Optimization engine
  - [ ] Analyze issues with Claude
  - [ ] Generate improved configs
  - [ ] Write candidates to dev environment
  - [ ] Notify users of recommendations
- [ ] Background workers
  - [ ] Monitoring loop
  - [ ] Quota reset scheduler
  - [ ] Cost reporting aggregation
- [ ] Notifications
  - [ ] Email alerts for issues
  - [ ] Slack integration (optional)
  - [ ] In-app notifications
- [ ] Testing
  - [ ] Test anomaly detection
  - [ ] Test optimization suggestions
  - [ ] Test notification delivery

**Deliverable:** Agent autonomously detects issues and suggests improvements

**Success Criteria:**
- Monitoring runs continuously
- Issues detected within 5 minutes
- Recommendations generated automatically
- Users notified of suggestions

---

### Phase 6: Production Readiness (Week 5+)
**Goal:** Production-ready with observability, security, docs

**Tasks:**
- [ ] PostgreSQL implementation
  - [ ] Replace in-memory DB with PostgreSQL
  - [ ] Connection pooling
  - [ ] Migrations system
  - [ ] Indexes for performance
- [ ] Observability
  - [ ] Prometheus metrics
  - [ ] Grafana dashboards
  - [ ] Structured logging
  - [ ] Distributed tracing (optional)
- [ ] Security hardening
  - [ ] Rate limiting per tenant
  - [ ] API key rotation
  - [ ] Audit logging
  - [ ] RBAC implementation
  - [ ] Security headers
- [ ] Documentation
  - [ ] API documentation (OpenAPI/Swagger)
  - [ ] Integration guide
  - [ ] Deployment guide
  - [ ] Runbooks for operators
- [ ] Performance optimization
  - [ ] Load testing
  - [ ] Database query optimization
  - [ ] Caching improvements
  - [ ] Connection pooling
- [ ] Deployment
  - [ ] Docker image
  - [ ] Kubernetes manifests
  - [ ] CI/CD pipeline
  - [ ] Monitoring alerts

**Deliverable:** Production-ready system with full observability

**Success Criteria:**
- 99.9% uptime
- P95 latency < 3 seconds
- Comprehensive monitoring
- Security audit passed

---

## Key Implementation Details

### 1. Batch Processing Model

**Critical:** ByteFreezer processes data in batches. Configs are read once per batch and applied to the entire batch atomically. No mid-batch config changes.

```go
// Receiver pseudo-code (existing ByteFreezer)
func (r *Receiver) ProcessBatch(batch *Batch) error {
    tenantID := batch.TenantID

    // Single config read per batch
    activeVersion := r.controlDB.GetActiveVersion(tenantID)
    config := r.controlDB.GetConfig(tenantID, "production", activeVersion)

    // Configure pipeline for this batch
    pipeline := r.configurePipeline(config)

    // Process entire batch (all-or-nothing transaction)
    output, err := pipeline.Process(batch)
    if err != nil {
        // Batch fails atomically
        r.emitMetrics(tenantID, batch.ID, false, err)
        return err
    }

    // Success - emit metrics for agent monitoring
    r.emitMetrics(tenantID, batch.ID, true, nil)
    r.emitOutputSample(tenantID, output.Sample(100))

    return nil
}
```

**Implications:**
- Config changes take effect on next batch boundary
- No disruption to in-flight batches
- Clean rollback (just change active version)
- Metrics available per-batch for monitoring

---

### 2. Prompt Engineering

#### Config Generation Prompt

```go
const configGenerationPromptTemplate = `You are a ByteFreezer configuration expert. Generate a complete, production-ready configuration for network data ingestion and processing.

TENANT: {{.TenantID}}

USER INTENT:
{{.Intent}}

SAMPLE INPUT DATA:
{{.SampleData}}

CONSTRAINTS:
{{.ConstraintsYAML}}

BYTEFREEZER ARCHITECTURE:
- Receivers: Listen on UDP/TCP, buffer incoming packets
- Piper: Parse, transform, and aggregate data
- Storage: Write Parquet files to S3/MinIO with partitioning

REQUIREMENTS:
1. listeners: Configure UDP/TCP listeners
   - protocol: udp or tcp
   - port: 1-65535
   - buffer_size: bytes (default 65536)
   - workers: number of worker threads

2. storage: Configure S3/MinIO output
   - type: s3 or minio
   - bucket: bucket name
   - region: AWS region (for S3)
   - endpoint: custom endpoint (for MinIO)
   - path_template: path with variables (tenant, year, month, day, hour)
   - compression: none, snappy, gzip, zstd
   - row_group_size: rows per Parquet row group
   - partitioning_keys: array of partition keys (low cardinality!)

3. piper: Configure data processing pipeline
   - parsers: Protocol-specific parsers
     * sflow_v5, sflow_v4: sFlow protocol
     * netflow_v9, netflow_v5: NetFlow protocol
     * ipfix: IPFIX protocol
     * json: JSON parsing with schema
   
   - transformers: Field manipulation
     * extract: Extract specific fields
     * cidr_mask: Convert IPs to subnets (e.g., /24)
     * geoip: Add geographic info
     * rename: Rename fields
     * filter: Drop records matching conditions
   
   - aggregations: Time-based aggregation
     * window: time window (1m, 5m, 1h, 1d)
     * group_by: array of grouping keys
     * aggregates: array of {field, function} where function is sum, count, avg, min, max, count_distinct

BEST PRACTICES:
1. Partitioning:
   - Use LOW cardinality keys (< 10,000 unique values)
   - Prefer time-based: year, month, day, hour
   - For IPs: use /24 or /16 subnets, NOT individual IPs
   - Avoid: UUIDs, full IPs, user agents, URLs
   - Limit to 2-3 partition keys maximum

2. Memory:
   - Smaller row_group_size = less memory, more files
   - Larger buffers = better throughput, more memory
   - Aggregation windows: shorter = less memory

3. Performance:
   - Compression: snappy (fast, medium compression) or zstd (slower, better compression)
   - Avoid aggregating before parsing
   - Keep transformers simple and fast

4. Query optimization:
   - Partition by common query filters (time, tenant)
   - Pre-aggregate if queries need summaries
   - Index-friendly datatypes (int32 > string)

OUTPUT FORMAT:
Return ONLY valid YAML configuration. No markdown code blocks, no explanations before or after.

---CONFIG START---
`

// Helper function to build prompt
func buildConfigPrompt(req ConfigRequest) string {
    constraintsYAML, _ := yaml.Marshal(req.Constraints)
    
    tmpl := template.Must(template.New("config").Parse(configGenerationPromptTemplate))
    
    var buf bytes.Buffer
    tmpl.Execute(&buf, map[string]interface{}{
        "TenantID":        req.TenantID,
        "Intent":          req.Intent,
        "SampleData":      req.SampleData,
        "ConstraintsYAML": string(constraintsYAML),
    })
    
    return buf.String()
}
```

#### Validation Analysis Prompt

```go
const validationAnalysisPrompt = `Analyze this ByteFreezer configuration validation failure.

CONFIGURATION:
{{.ConfigYAML}}

VALIDATION TEST RESULTS:
- Schema Valid: {{.SchemaValid}}
- Data Loss: {{.DataLossPercent}}%
- Partition Count: {{.PartitionCount}}
- Query Latency: {{.QueryLatencyMs}}ms
- Memory Usage: {{.MemoryUsageMb}}MB
- Error Count: {{.ErrorCount}}

SAMPLE OUTPUT:
{{.SampleOutputJSON}}

ISSUES DETECTED:
{{range .Issues}}
- {{.}}
{{end}}

Identify the root cause of validation failures and provide specific fixes. Be concise and actionable.

Focus on:
1. Configuration errors (invalid syntax, missing fields)
2. Performance issues (high cardinality partitioning, memory pressure)
3. Data quality issues (field extraction errors, type mismatches)
4. Operational concerns (unsustainable growth, query performance)

Return a bullet list of recommended changes.
`
```

#### Optimization Suggestion Prompt

```go
const optimizationPromptTemplate = `Analyze production metrics and suggest configuration improvements.

TENANT: {{.TenantID}}
CURRENT CONFIG:
{{.ConfigYAML}}

PRODUCTION METRICS (last 24 hours):
- Batches Processed: {{.BatchesProcessed}}
- Average Latency: {{.AvgLatencyMs}}ms
- Error Rate: {{.ErrorRate}}%
- Data Loss Rate: {{.DataLossRate}}%
- Memory P95: {{.MemoryP95Mb}}MB
- Partition Count Growth: {{.PartitionGrowth}}/day

DETECTED ISSUES:
{{range .Issues}}
- {{.}}
{{end}}

SAMPLE OUTPUT DATA:
{{.SampleOutputJSON}}

Suggest specific configuration changes to address these issues. Focus on:
1. Reducing memory usage
2. Improving query performance
3. Reducing partition count
4. Preventing data loss
5. Optimizing cost (storage, compute)

Return:
1. Root cause analysis (2-3 sentences)
2. Recommended config changes (specific YAML fields)
3. Expected impact (quantified if possible)
`
```

---

### 3. Caching Strategy

```go
// Cache key generation - hash of normalized intent + constraints
func generateCacheKey(req ConfigRequest) string {
    // Normalize intent (lowercase, trim whitespace, remove common variations)
    normalizedIntent := strings.ToLower(strings.TrimSpace(req.Intent))
    normalizedIntent = strings.ReplaceAll(normalizedIntent, "  ", " ")
    
    // Create stable representation of constraints
    constraintKeys := []string{}
    for k := range req.Constraints {
        constraintKeys = append(constraintKeys, k)
    }
    sort.Strings(constraintKeys)
    
    data := struct {
        Intent      string
        Constraints map[string]interface{}
    }{
        Intent:      normalizedIntent,
        Constraints: req.Constraints,
    }
    
    jsonData, _ := json.Marshal(data)
    hash := sha256.Sum256(jsonData)
    return hex.EncodeToString(hash[:])
}

// Cache implementation with TTL and LRU eviction
type ConfigCache struct {
    mu        sync.RWMutex
    cache     map[string]*CacheEntry
    lruList   *list.List
    lruMap    map[string]*list.Element
    maxSize   int
    ttl       time.Duration
}

type CacheEntry struct {
    Key       string
    Response  *ConfigResponse
    ExpiresAt time.Time
    AccessCount int
}

func NewConfigCache(maxSize int, ttl time.Duration) *ConfigCache {
    return &ConfigCache{
        cache:   make(map[string]*CacheEntry),
        lruList: list.New(),
        lruMap:  make(map[string]*list.Element),
        maxSize: maxSize,
        ttl:     ttl,
    }
}

func (c *ConfigCache) Get(key string) (*ConfigResponse, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    entry, exists := c.cache[key]
    if !exists {
        return nil, false
    }
    
    // Check if expired
    if time.Now().After(entry.ExpiresAt) {
        c.evict(key)
        return nil, false
    }
    
    // Update LRU
    if elem, ok := c.lruMap[key]; ok {
        c.lruList.MoveToFront(elem)
    }
    
    entry.AccessCount++
    
    return entry.Response, true
}

func (c *ConfigCache) Set(key string, resp *ConfigResponse) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Evict if at capacity
    if len(c.cache) >= c.maxSize {
        c.evictLRU()
    }
    
    entry := &CacheEntry{
        Key:         key,
        Response:    resp,
        ExpiresAt:   time.Now().Add(c.ttl),
        AccessCount: 1,
    }
    
    c.cache[key] = entry
    elem := c.lruList.PushFront(key)
    c.lruMap[key] = elem
}

func (c *ConfigCache) evictLRU() {
    elem := c.lruList.Back()
    if elem == nil {
        return
    }
    
    key := elem.Value.(string)
    c.evict(key)
}

func (c *ConfigCache) evict(key string) {
    delete(c.cache, key)
    if elem, ok := c.lruMap[key]; ok {
        c.lruList.Remove(elem)
        delete(c.lruMap, key)
    }
}

// Statistics
func (c *ConfigCache) Stats() CacheStats {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    totalAccess := 0
    for _, entry := range c.cache {
        totalAccess += entry.AccessCount
    }
    
    return CacheStats{
        Size:        len(c.cache),
        Capacity:    c.maxSize,
        TotalAccess: totalAccess,
    }
}
```

**Cache warming strategies:**
- Pre-generate configs for common patterns on deployment
- Cache popular tenant configs
- Share cache across similar tenants (anonymized)

**Expected cache hit rate:** 30-40% (saves ~$0.006-0.01 per hit)

---

### 4. API Key Encryption

```go
// crypto/encryption.go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"
)

// Encrypt encrypts plaintext using AES-256-GCM
func Encrypt(plaintext string, masterKey []byte) (string, error) {
    if len(masterKey) != 32 {
        return "", errors.New("master key must be 32 bytes (AES-256)")
    }
    
    block, err := aes.NewCipher(masterKey)
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    // Generate random nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    
    // Encrypt and prepend nonce
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    
    // Base64 encode for storage
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func Decrypt(ciphertext string, masterKey []byte) (string, error) {
    if len(masterKey) != 32 {
        return "", errors.New("master key must be 32 bytes (AES-256)")
    }
    
    // Base64 decode
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }
    
    block, err := aes.NewCipher(masterKey)
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", errors.New("ciphertext too short")
    }
    
    // Extract nonce and ciphertext
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    
    // Decrypt
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }
    
    return string(plaintext), nil
}

// GenerateMasterKey generates a random 32-byte key
func GenerateMasterKey() ([]byte, error) {
    key := make([]byte, 32)
    if _, err := io.ReadFull(rand.Reader, key); err != nil {
        return nil, err
    }
    return key, nil
}
```

**Master key management:**
```bash
# Generate master key (one time)
openssl rand -base64 32

# Store in environment variable
export BYTEFREEZER_MASTER_KEY="<base64_key>"

# Or use secrets manager (production)
# AWS Secrets Manager, HashiCorp Vault, etc.
```

**Key rotation:**
```go
func RotateAPIKeys(db *DB, oldMasterKey, newMasterKey []byte) error {
    tenants, err := db.GetAllTenants()
    if err != nil {
        return err
    }
    
    for _, tenant := range tenants {
        if tenant.AIConfig.Provider != "byoa" {
            continue
        }
        
        // Decrypt with old key
        plaintext, err := crypto.Decrypt(tenant.AIConfig.APIKey, oldMasterKey)
        if err != nil {
            return fmt.Errorf("decrypt tenant %s: %w", tenant.ID, err)
        }
        
        // Re-encrypt with new key
        encrypted, err := crypto.Encrypt(plaintext, newMasterKey)
        if err != nil {
            return fmt.Errorf("encrypt tenant %s: %w", tenant.ID, err)
        }
        
        // Update in database
        tenant.AIConfig.APIKey = encrypted
        if err := db.UpdateTenant(tenant); err != nil {
            return fmt.Errorf("update tenant %s: %w", tenant.ID, err)
        }
    }
    
    return nil
}
```

---

### 5. Quota Management Implementation

```go
// cost/quota.go
package cost

import (
    "fmt"
    "time"
    
    "github.com/yourusername/bytefreezer-agent/controldb"
)

type QuotaManager struct {
    db      *controldb.Client
    tracker *Tracker
}

func NewQuotaManager(db *controldb.Client, tracker *Tracker) *QuotaManager {
    return &QuotaManager{
        db:      db,
        tracker: tracker,
    }
}

// CheckQuota verifies if tenant can perform operation
func (q *QuotaManager) CheckQuota(tenantID string, operation Operation) (*QuotaCheck, error) {
    tenant, err := q.db.GetTenant(tenantID)
    if err != nil {
        return nil, fmt.Errorf("get tenant: %w", err)
    }
    
    // BYOA = unlimited
    if tenant.AIConfig.Provider == "byoa" {
        return &QuotaCheck{
            Allowed:   true,
            Remaining: -1, // unlimited
        }, nil
    }
    
    // Check if monthly reset needed
    if q.needsReset(tenant) {
        if err := q.resetQuota(tenantID); err != nil {
            return nil, fmt.Errorf("reset quota: %w", err)
        }
        tenant.Usage.CurrentPeriod = UsagePeriod{}
    }
    
    quota := TierQuotas[tenant.Tier]
    current := tenant.Usage.CurrentPeriod
    
    // Check operation-specific quota
    var used, limit int
    switch operation {
    case OperationConfigGeneration:
        used = current.ConfigGenerations
        limit = quota.ConfigGenerations
    case OperationValidation:
        used = current.Validations
        limit = quota.Validations
    case OperationOptimization:
        used = current.Optimizations
        limit = quota.Optimizations
    }
    
    // Also check token budget
    tokenBudgetOK := quota.TotalTokens == -1 || current.TotalTokens < quota.TotalTokens
    
    if used >= limit || !tokenBudgetOK {
        return &QuotaCheck{
            Allowed:     false,
            Reason:      q.buildLimitMessage(tenant.Tier, operation, used, limit),
            Remaining:   0,
            ResetsAt:    q.calculateResetTime(tenant),
            UpgradePath: q.getUpgradePath(tenant.Tier),
        }, nil
    }
    
    return &QuotaCheck{
        Allowed:   true,
        Remaining: limit - used,
        ResetsAt:  q.calculateResetTime(tenant),
    }, nil
}

// IncrementUsage increments tenant's usage counters
func (q *QuotaManager) IncrementUsage(tenantID string, operation Operation, tokens int, cost float64) error {
    tenant, err := q.db.GetTenant(tenantID)
    if err != nil {
        return err
    }
    
    // Don't track usage for BYOA (they pay Claude directly)
    if tenant.AIConfig.Provider == "byoa" {
        // Still track for visibility, but don't bill
        q.tracker.Record(UsageRecord{
            TenantID:  tenantID,
            Operation: operation,
            Tokens:    tokens,
            Cost:      0, // Not charged to platform
            PaidBy:    "customer",
            Timestamp: time.Now(),
        })
        return nil
    }
    
    // Update usage counters
    updates := map[string]interface{}{}
    
    switch operation {
    case OperationConfigGeneration:
        updates["usage.current_period.config_generations"] = tenant.Usage.CurrentPeriod.ConfigGenerations + 1
    case OperationValidation:
        updates["usage.current_period.validations"] = tenant.Usage.CurrentPeriod.Validations + 1
    case OperationOptimization:
        updates["usage.current_period.optimizations"] = tenant.Usage.CurrentPeriod.Optimizations + 1
    }
    
    updates["usage.current_period.total_tokens"] = tenant.Usage.CurrentPeriod.TotalTokens + tokens
    updates["usage.current_period.total_cost"] = tenant.Usage.CurrentPeriod.TotalCost + cost
    
    // Track for platform billing
    q.tracker.Record(UsageRecord{
        TenantID:  tenantID,
        Operation: operation,
        Tokens:    tokens,
        Cost:      cost,
        PaidBy:    "platform",
        Timestamp: time.Now(),
    })
    
    return q.db.UpdateTenant(tenantID, updates)
}

// Helper functions
func (q *QuotaManager) needsReset(tenant *Tenant) bool {
    return time.Since(tenant.Usage.LastReset) > 30*24*time.Hour
}

func (q *QuotaManager) resetQuota(tenantID string) error {
    tenant, err := q.db.GetTenant(tenantID)
    if err != nil {
        return err
    }
    
    // Archive current period
    tenant.Usage.History = append(tenant.Usage.History, tenant.Usage.CurrentPeriod)
    
    // Reset current period
    updates := map[string]interface{}{
        "usage.current_period": UsagePeriod{},
        "usage.last_reset":     time.Now(),
    }
    
    return q.db.UpdateTenant(tenantID, updates)
}

func (q *QuotaManager) calculateResetTime(tenant *Tenant) time.Time {
    return tenant.Usage.LastReset.Add(30 * 24 * time.Hour)
}

func (q *QuotaManager) buildLimitMessage(tier Tier, op Operation, used, limit int) string {
    opName := map[Operation]string{
        OperationConfigGeneration: "config generation",
        OperationValidation:       "validation",
        OperationOptimization:     "optimization",
    }[op]
    
    return fmt.Sprintf(
        "Free tier limit reached: %d/%d %s operations this month. "+
        "Upgrade to Pro for unlimited AI features, or bring your own Claude API key.",
        used, limit, opName,
    )
}

func (q *QuotaManager) getUpgradePath(tier Tier) map[string]UpgradeOption {
    if tier != TierFree {
        return nil
    }
    
    return map[string]UpgradeOption{
        "pro": {
            Name:  "Pro Plan",
            Price: "$50/month",
            Features: []string{
                "Unlimited AI config generations",
                "Unlimited validations",
                "Unlimited optimizations",
                "Email support",
            },
            URL: "https://bytefreezer.io/upgrade?tier=pro",
        },
        "byoa": {
            Name:  "Bring Your Own API",
            Price: "$0/month (usage-based via your Claude account)",
            Features: []string{
                "Unlimited usage",
                "Use your own Claude API key",
                "Full control over AI costs",
                "Same features as Pro",
            },
            URL: "https://bytefreezer.io/settings/ai-config",
        },
    }
}

// Background scheduler for monthly resets
func (q *QuotaManager) StartResetScheduler(ctx context.Context) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            q.checkAndResetQuotas()
        case <-ctx.Done():
            return
        }
    }
}

func (q *QuotaManager) checkAndResetQuotas() {
    tenants, err := q.db.GetAllTenants()
    if err != nil {
        log.Printf("Error getting tenants for quota reset: %v", err)
        return
    }
    
    for _, tenant := range tenants {
        if q.needsReset(&tenant) {
            if err := q.resetQuota(tenant.ID); err != nil {
                log.Printf("Error resetting quota for tenant %s: %v", tenant.ID, err)
            } else {
                log.Printf("Reset quota for tenant %s", tenant.ID)
            }
        }
    }
}

// Types
type Operation string

const (
    OperationConfigGeneration Operation = "config_generation"
    OperationValidation       Operation = "validation"
    OperationOptimization     Operation = "optimization"
)

type QuotaCheck struct {
    Allowed     bool
    Reason      string
    Remaining   int
    ResetsAt    time.Time
    UpgradePath map[string]UpgradeOption
}

type UpgradeOption struct {
    Name     string
    Price    string
    Features []string
    URL      string
}
```

---

### 6. Error Handling & Retries

```go
// agent/client.go - Retry logic
func (c *ClaudeClient) GenerateWithRetry(ctx context.Context, tenantID, prompt, model string) (*ClaudeResponse, error) {
    var lastErr error
    
    for attempt := 0; attempt < c.maxRetries; attempt++ {
        resp, err := c.generate(ctx, tenantID, prompt, model)
        if err == nil {
            return resp, nil
        }
        
        lastErr = err
        
        // Don't retry client errors (4xx)
        if isClientError(err) {
            return nil, fmt.Errorf("client error (no retry): %w", err)
        }
        
        // Don't retry context cancellation
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return nil, err
        }
        
        // Exponential backoff for server errors (5xx) and network errors
        backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
        log.Printf("Attempt %d failed for tenant %s: %v. Retrying in %v...", 
            attempt+1, tenantID, err, backoff)
        
        select {
        case <-time.After(backoff):
            // Continue to next attempt
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    
    return nil, fmt.Errorf("failed after %d attempts: %w", c.maxRetries, lastErr)
}

func isClientError(err error) bool {
    // Check if error is HTTP 4xx
    var httpErr *HTTPError
    if errors.As(err, &httpErr) {
        return httpErr.StatusCode >= 400 && httpErr.StatusCode < 500
    }
    return false
}

type HTTPError struct {
    StatusCode int
    Body       string
}

func (e *HTTPError) Error() string {
    return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}
```

**Graceful degradation:**
```go
// If Claude API is down, fall back to cached configs
func (g *ConfigGenerator) GenerateWithFallback(ctx context.Context, req ConfigRequest) (*ConfigResponse, error) {
    // Try normal generation
    resp, err := g.Generate(ctx, req)
    if err == nil {
        return resp, nil
    }
    
    // If API is down, try cache
    cacheKey := g.generateCacheKey(req)
    if cached, ok := g.cache.Get(cacheKey); ok {
        log.Printf("Claude API unavailable, returning cached config for tenant %s", req.TenantID)
        cached.FromCache = true
        cached.Warning = "Generated from cache due to API unavailability"
        return cached, nil
    }
    
    // No cache available
    return nil, fmt.Errorf("Claude API unavailable and no cached config found: %w", err)
}
```

---

## Testing Strategy

### Unit Tests

```go
// agent/config_gen_test.go
func TestGenerateConfig_Success(t *testing.T) {
    mockClient := &MockClaudeClient{
        Response: &ClaudeResponse{
            Content: []struct{ Type, Text string }{
                {Type: "text", Text: validConfigYAML},
            },
            Usage: Usage{InputTokens: 1000, OutputTokens: 500},
        },
    }
    
    generator := NewConfigGenerator(mockClient, nil, nil)
    
    req := ConfigRequest{
        TenantID: "test",
        Intent:   "sFlow aggregation",
        Constraints: map[string]interface{}{
            "max_memory_mb": 2048,
        },
    }
    
    resp, err := generator.Generate(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, "test", resp.TenantID)
}

func TestGenerateConfig_CacheHit(t *testing.T) {
    // First call should hit API
    // Second call with same intent should hit cache
}

func TestGenerateConfig_QuotaExceeded(t *testing.T) {
    // Mock tenant at quota limit
    // Verify quota exceeded error returned
}

// cost/quota_test.go
func TestQuotaManager_CheckQuota(t *testing.T) {
    tests := []struct{
        name      string
        tier      Tier
        usage     UsagePeriod
        operation Operation
        want      bool
    }{
        {
            name: "free tier under limit",
            tier: TierFree,
            usage: UsagePeriod{ConfigGenerations: 3},
            operation: OperationConfigGeneration,
            want: true,
        },
        {
            name: "free tier at limit",
            tier: TierFree,
            usage: UsagePeriod{ConfigGenerations: 5},
            operation: OperationConfigGeneration,
            want: false,
        },
        {
            name: "BYOA unlimited",
            tier: TierBYOA,
            usage: UsagePeriod{ConfigGenerations: 10000},
            operation: OperationConfigGeneration,
            want: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test quota check logic
        })
    }
}

func TestQuotaManager_Reset(t *testing.T) {
    // Test monthly quota reset
}

// crypto/encryption_test.go
func TestEncryptDecrypt(t *testing.T) {
    masterKey, _ := GenerateMasterKey()
    plaintext := "sk-ant-api03-test-key-1234567890"
    
    encrypted, err := Encrypt(plaintext, masterKey)
    assert.NoError(t, err)
    assert.NotEqual(t, plaintext, encrypted)
    
    decrypted, err := Decrypt(encrypted, masterKey)
    assert.NoError(t, err)
    assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_DifferentKeys(t *testing.T) {
    key1, _ := GenerateMasterKey()
    key2, _ := GenerateMasterKey()
    
    encrypted, _ := Encrypt("secret", key1)
    
    // Should fail to decrypt with different key
    _, err := Decrypt(encrypted, key2)
    assert.Error(t, err)
}
```

### Integration Tests

```go
// testing/integration_test.go
func TestE2E_ConfigLifecycle(t *testing.T) {
    // Setup test environment
    db := setupTestDB(t)
    defer db.Close()
    
    server := setupTestServer(t, db)
    defer server.Close()
    
    client := setupTestClient(t, server.URL)
    
    // 1. Generate config in dev
    genResp := client.GenerateConfig(ConfigRequest{
        TenantID: "test",
        Intent:   "sFlow aggregation by /24",
    })
    assert.Equal(t, "dev", genResp.Environment)
    assert.Equal(t, 1, genResp.Version)
    
    // 2. Validate config
    valResp := client.ValidateConfig("test", 1, "dev")
    assert.Equal(t, "passed", valResp.Status)
    
    // 3. Promote to staging
    promoteResp := client.PromoteConfig("test", 1, "dev", "staging")
    assert.Equal(t, "staging", promoteResp.PromotedTo)
    
    // 4. Promote to production
    promoteResp = client.PromoteConfig("test", 1, "staging", "production")
    assert.Equal(t, "production", promoteResp.PromotedTo)
    
    // 5. Verify active config
    activeResp := client.GetActiveConfig("test")
    assert.Equal(t, 1, activeResp.Version)
    
    // 6. Rollback
    rollbackResp := client.Rollback("test", 0)
    // Should rollback to version 0 (if existed) or error
}

func TestE2E_BYOA(t *testing.T) {
    // 1. Create tenant with free tier
    // 2. Use 5 config generations (hit limit)
    // 3. Set BYOA API key
    // 4. Verify unlimited usage now available
    // 5. Generate more configs (should succeed)
    // 6. Verify costs tracked but not billed to platform
}

func TestE2E_QuotaEnforcement(t *testing.T) {
    // 1. Create free tier tenant
    // 2. Generate 5 configs (use all quota)
    // 3. Try 6th generation (should fail with quota exceeded)
    // 4. Verify upgrade options in error response
    // 5. Reset quota (simulate monthly reset)
    // 6. Try generation again (should succeed)
}
```

### Load Tests

```bash
# Using hey (HTTP load testing tool)

# Test config generation under load
hey -n 1000 -c 50 -m POST \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{"intent":"sFlow aggregation","sample_data":"..."}' \
  http://localhost:8080/api/v1/tenants/test/configs/generate

# Expected results:
# - P95 latency < 5 seconds
# - No errors
# - Cache hit rate > 30% after warmup

# Test concurrent validation
hey -n 500 -c 25 -m POST \
  -H "Authorization: Bearer test-token" \
  http://localhost:8080/api/v1/tenants/test/configs/1/validate

# Test quota enforcement under load (should handle gracefully)
```

---

## Configuration

### Environment Variables

```bash
# Required
ANTHROPIC_API_KEY=sk-ant-...              # Platform API key (for free/pro tiers)
BYTEFREEZER_MASTER_KEY=<base64>           # For encrypting tenant API keys (32 bytes)
DATABASE_URL=postgres://user:pass@host:5432/bytefreezer

# Optional
PORT=8080
LOG_LEVEL=info                            # debug, info, warn, error
LOG_FORMAT=json                           # json or text

# Cache
CACHE_TTL_HOURS=24
CACHE_MAX_SIZE_MB=1024

# Quota
QUOTA_RESET_HOUR=0                        # UTC hour for daily check

# Claude API
CLAUDE_DEFAULT_MODEL=claude-sonnet-4-5-20250929
CLAUDE_CHEAP_MODEL=claude-haiku-4-20250514
CLAUDE_MAX_TOKENS=4096
CLAUDE_TEMPERATURE=0.3
CLAUDE_TIMEOUT_SECONDS=60
CLAUDE_MAX_RETRIES=3

# Monitoring
ENABLE_METRICS=true
METRICS_PORT=9090
HEALTH_CHECK_INTERVAL=5m
COST_REPORT_INTERVAL=1h

# Security
ENABLE_RATE_LIMITING=true
RATE_LIMIT_PER_TENANT=100                 # requests per hour
RATE_LIMIT_GLOBAL=10000
API_KEY_ROTATION_DAYS=90
MAX_FAILED_AUTH_ATTEMPTS=5
AUTH_LOCKOUT_DURATION=15m

# Features
ENABLE_MONITORING=true                    # Background monitoring loop
MONITORING_INTERVAL=5m
ENABLE_OPTIMIZATION=true                  # AI optimization suggestions
```

### Configuration File (config.yaml)

```yaml
server:
  port: 8080
  address: ":8080"
  read_timeout: 15s
  write_timeout: 60s
  idle_timeout: 60s

database:
  url: "${DATABASE_URL}"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m

claude:
  api_key: "${ANTHROPIC_API_KEY}"
  default_model: claude-sonnet-4-5-20250929
  cheap_model: claude-haiku-4-20250514
  max_tokens: 4096
  temperature: 0.3
  timeout: 60s
  max_retries: 3

cache:
  ttl: 24h
  max_size_mb: 1024
  eviction_policy: lru

quotas:
  free:
    config_generations: 5
    validations: 10
    optimizations: 3
    total_tokens: 50000
  pro:
    config_generations: -1  # unlimited
    validations: -1
    optimizations: -1
    total_tokens: -1
  reset_hour: 0  # UTC

monitoring:
  enabled: true
  interval: 5m
  health_check_interval: 30s
  cost_report_interval: 1h
  metrics_port: 9090

security:
  master_key: "${BYTEFREEZER_MASTER_KEY}"
  rate_limiting:
    enabled: true
    per_tenant: 100  # per hour
    global: 10000    # per hour
  api_key_rotation_days: 90
  max_failed_attempts: 5
  lockout_duration: 15m

logging:
  level: info
  format: json
  output: stdout

features:
  enable_monitoring: true
  enable_optimization: true
  enable_byoa: true
```

---

## Deployment

### Docker

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bytefreezer-agent main.go

# Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary
COPY --from=builder /app/bytefreezer-agent .

# Copy config (optional)
COPY config.yaml .

EXPOSE 8080 9090

CMD ["./bytefreezer-agent"]
```

```bash
# Build
docker build -t bytefreezer/agent:latest .

# Run
docker run -d \
  -p 8080:8080 \
  -p 9090:9090 \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  -e BYTEFREEZER_MASTER_KEY=<base64> \
  -e DATABASE_URL=postgres://... \
  --name bytefreezer-agent \
  bytefreezer/agent:latest
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: bytefreezer
      POSTGRES_USER: bytefreezer
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
  
  agent:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
      BYTEFREEZER_MASTER_KEY: ${BYTEFREEZER_MASTER_KEY}
      DATABASE_URL: postgres://bytefreezer:secret@postgres:5432/bytefreezer?sslmode=disable
      LOG_LEVEL: info
    depends_on:
      - postgres
    restart: unless-stopped
  
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    ports:
      - "9091:9090"
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
  
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    depends_on:
      - prometheus

volumes:
  postgres_data:
  prometheus_data:
  grafana_data:
```

### Kubernetes

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bytefreezer-agent
  namespace: bytefreezer
spec:
  replicas: 3
  selector:
    matchLabels:
      app: bytefreezer-agent
  template:
    metadata:
      labels:
        app: bytefreezer-agent
    spec:
      containers:
      - name: agent
        image: bytefreezer/agent:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: bytefreezer-secrets
              key: anthropic-api-key
        - name: BYTEFREEZER_MASTER_KEY
          valueFrom:
            secretKeyRef:
              name: bytefreezer-secrets
              key: master-key
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: bytefreezer-secrets
              key: database-url
        - name: LOG_LEVEL
          value: "info"
        - name: LOG_FORMAT
          value: "json"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10

---
apiVersion: v1
kind: Service
metadata:
  name: bytefreezer-agent
  namespace: bytefreezer
spec:
  selector:
    app: bytefreezer-agent
  ports:
  - name: http
    protocol: TCP
    port: 80
    targetPort: 8080
  - name: metrics
    protocol: TCP
    port: 9090
    targetPort: 9090
  type: LoadBalancer

---
apiVersion: v1
kind: Secret
metadata:
  name: bytefreezer-secrets
  namespace: bytefreezer
type: Opaque
stringData:
  anthropic-api-key: "sk-ant-..."
  master-key: "<base64>"
  database-url: "postgres://..."

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: bytefreezer-agent-hpa
  namespace: bytefreezer
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: bytefreezer-agent
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## Security

### 1. API Key Storage
- **Encryption**: AES-256-GCM for API keys at rest
- **Master key**: Stored in environment variable or secrets manager
- **Key rotation**: Support for rotating master key and re-encrypting
- **No logging**: Never log decrypted API keys

### 2. Authentication & Authorization
```go
// Middleware for authentication
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Missing authorization", http.StatusUnauthorized)
            return
        }
        
        // Remove "Bearer " prefix
        token = strings.TrimPrefix(token, "Bearer ")
        
        // Validate token and get tenant
        tenant, err := validateToken(token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        // Add tenant to context
        ctx := context.WithValue(r.Context(), "tenant", tenant)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 3. Rate Limiting
```go
// Per-tenant rate limiting
type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
}

func (rl *RateLimiter) Allow(tenantID string) bool {
    rl.mu.RLock()
    limiter, exists := rl.limiters[tenantID]
    rl.mu.RUnlock()
    
    if !exists {
        rl.mu.Lock()
        limiter = rate.NewLimiter(rate.Limit(100/3600), 10) // 100/hour, burst 10
        rl.limiters[tenantID] = limiter
        rl.mu.Unlock()
    }
    
    return limiter.Allow()
}
```

### 4. Input Validation
```go
// Validate and sanitize user input
func validateConfigRequest(req *ConfigRequest) error {
    // Intent length limit (prevent prompt injection)
    if len(req.Intent) > 10000 {
        return errors.New("intent too long (max 10,000 chars)")
    }
    
    // Sample data size limit
    if len(req.SampleData) > 10*1024*1024 { // 10MB
        return errors.New("sample data too large (max 10MB)")
    }
    
    // Sanitize intent (basic XSS prevention)
    req.Intent = sanitize(req.Intent)
    
    return nil
}
```

### 5. Audit Logging
```go
// Log all sensitive operations
func auditLog(tenantID, operation, actor string, details map[string]interface{}) {
    log.WithFields(log.Fields{
        "type":      "audit",
        "tenant_id": tenantID,
        "operation": operation,
        "actor":     actor,
        "timestamp": time.Now(),
        "details":   details,
    }).Info("Audit log")
}

// Examples:
auditLog(tenant, "config_generated", "ai_agent", map[string]interface{}{
    "version": 8,
    "environment": "dev",
})

auditLog(tenant, "api_key_updated", user, map[string]interface{}{
    "provider": "anthropic",
})

auditLog(tenant, "config_promoted", user, map[string]interface{}{
    "version": 7,
    "from": "staging",
    "to": "production",
})
```

---

## Monitoring

### Prometheus Metrics

```go
// pkg/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Config generation metrics
    ConfigGenerationsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytefreezer_config_generations_total",
            Help: "Total number of config generation requests",
        },
        []string{"tenant", "tier", "status"},
    )
    
    ConfigGenerationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "bytefreezer_config_generation_duration_seconds",
            Help:    "Duration of config generation requests",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
        },
        []string{"tenant"},
    )
    
    // Quota metrics
    QuotaRemaining = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "bytefreezer_quota_remaining",
            Help: "Remaining quota for tenant",
        },
        []string{"tenant", "operation"},
    )
    
    QuotaExceededTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytefreezer_quota_exceeded_total",
            Help: "Total number of quota exceeded events",
        },
        []string{"tenant", "operation"},
    )
    
    // API cost metrics
    APICallCostTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytefreezer_api_call_cost_total",
            Help: "Total cost of API calls in USD",
        },
        []string{"tenant", "model", "paid_by"},
    )
    
    APITokensTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytefreezer_api_tokens_total",
            Help: "Total tokens consumed",
        },
        []string{"tenant", "model", "type"}, // type: input or output
    )
    
    // Cache metrics
    CacheHitRate = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "bytefreezer_cache_hit_rate",
            Help: "Cache hit rate (0-1)",
        },
    )
    
    CacheSize = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "bytefreezer_cache_size_entries",
            Help: "Number of entries in cache",
        },
    )
    
    // Validation metrics
    ValidationTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bytefreezer_validation_total",
            Help: "Total number of validations",
        },
        []string{"tenant", "status"},
    )
    
    ValidationDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "bytefreezer_validation_duration_seconds",
            Help:    "Duration of validation requests",
            Buckets: []float64{1, 5, 10, 30, 60, 120},
        },
        []string{"tenant"},
    )
)
```

### Grafana Dashboards

**Dashboard 1: Cost & Usage Overview**
- Total API cost (daily/monthly)
- Cost by tier (free vs pro vs BYOA)
- Cost by tenant (top 10)
- Token usage over time
- Request rate

**Dashboard 2: Quota Management**
- Quota utilization by tier
- Quota exceeded events
- Conversion funnel (free → pro/BYOA)
- Usage trends

**Dashboard 3: Performance**
- Request latency (P50, P95, P99)
- Cache hit rate
- Error rate
- Throughput (requests/sec)

**Dashboard 4: Tenant Health**
- Per-tenant metrics
- Validation success rate
- Config promotion flow
- Rollback events

### Alert Rules

```yaml
# prometheus-alerts.yml
groups:
- name: bytefreezer_agent
  interval: 30s
  rules:
  
  # High error rate
  - alert: HighErrorRate
    expr: rate(bytefreezer_config_generations_total{status="error"}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate in config generation"
      description: "Error rate is {{ $value }} errors/sec"
  
  # API cost spike
  - alert: APICostSpike
    expr: increase(bytefreezer_api_call_cost_total[1h]) > 100
    labels:
      severity: warning
    annotations:
      summary: "API costs spiking"
      description: "${{ $value }} spent in last hour"
  
  # Service down
  - alert: ServiceDown
    expr: up{job="bytefreezer-agent"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "ByteFreezer Agent is down"
  
  # High latency
  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(bytefreezer_config_generation_duration_seconds_bucket[5m])) > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High P95 latency"
      description: "P95 latency is {{ $value }}s"
  
  # Cache degradation
  - alert: LowCacheHitRate
    expr: bytefreezer_cache_hit_rate < 0.2
    for: 15m
    labels:
      severity: info
    annotations:
      summary: "Cache hit rate below 20%"
```

---

## Success Metrics

### Technical Metrics
- **API Latency P95:** < 3 seconds for config generation
- **Cache Hit Rate:** > 30%
- **Validation Accuracy:** > 90% configs pass on first attempt
- **Uptime:** 99.9% (< 43 minutes downtime/month)
- **Error Rate:** < 1% of requests

### Business Metrics
- **Free Tier Conversion:** 10-15% convert to Pro or BYOA within 3 months
- **Cost Per Free User:** < $0.15/month in API costs
- **Time to First Config:** < 2 minutes from signup
- **Customer Satisfaction:** > 4.5/5 stars on AI features
- **Monthly Active Tenants:** Track growth

### Cost Metrics
- **Platform API Spend:** < $100/month per 1,000 free users
- **ROI:** > 50x (revenue from conversions vs API costs)
- **Cost Per Config:** < $0.02 average
- **BYOA Adoption:** 20% of paid customers

---

## Future Enhancements

### Phase 7+: Advanced Features

1. **Multi-Provider Support**
   - OpenAI GPT-4 integration
   - Google Gemini support
   - Local fine-tuned models (Llama, Mistral)
   - Provider failover

2. **Advanced Optimization**
   - A/B testing for configs
   - Automatic performance tuning
   - Cost optimization recommendations
   - Predictive scaling suggestions

3. **Collaborative Features**
   - Config templates marketplace
   - Community-validated configs
   - Share configs across tenants (opt-in)
   - Config diff and merge tools

4. **Deeper Integration**
   - VS Code extension for config editing
   - CLI tool with AI assistance
   - Slack bot for config management
   - Terraform provider

5. **Enterprise Features**
   - SSO/SAML integration
   - Advanced RBAC
   - Multi-region deployment
   - Dedicated Claude instances
   - SLA guarantees

---

## Risk Mitigation

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Claude API outage | High | Low | Cache fallback, graceful degradation |
| API cost spike from abuse | Medium | Medium | Rate limiting, quota enforcement, anomaly detection |
| Bad AI-generated configs | High | Medium | Validation layer, human approval, rollback capability |
| API key leakage (customer BYOA) | Critical | Low | Encryption at rest, audit logging, rotation support |
| Cache poisoning | Medium | Low | Cache key validation, TTL, integrity checks |
| Database failure | High | Low | Regular backups, read replicas, connection pooling |
| Free tier abuse | Medium | Medium | Rate limiting, email verification, captcha |
| Config validation bugs | Medium | Medium | Extensive testing, staged rollout, monitoring |

---

## Questions for Implementation

Before starting implementation, clarify:

1. **Database Choice:**
   - Using PostgreSQL, MySQL, or something else?
   - Existing schema or greenfield?
   - Connection pooling requirements?

2. **Authentication:**
   - Existing auth system to integrate with?
   - Token format (JWT, opaque tokens)?
   - User management system?

3. **Observability:**
   - Current stack (Prometheus + Grafana, DataDog, New Relic)?
   - Log aggregation (ELK, Splunk, CloudWatch)?
   - Distributed tracing needed?

4. **Deployment:**
   - Target environment (Kubernetes, Docker, VMs)?
   - CI/CD pipeline exists?
   - Blue-green or rolling deployments?

5. **ByteFreezer Integration:**
   - How do receivers read configs? (API, file, database?)
   - Metrics export format? (Prometheus, StatsD, custom?)
   - Test harness for validation available?

6. **Business Logic:**
   - Payment integration for Pro tier (Stripe)?
   - Email service for notifications (SendGrid)?
   - Support for trial periods?

---

## Implementation Checklist

Use this checklist to track progress:

### Phase 1: Core Infrastructure
- [ ] Project setup and structure
- [ ] Claude API client implementation
- [ ] Control DB interface (in-memory for testing)
- [ ] Config generation endpoint
- [ ] Basic cost tracking
- [ ] Unit tests for core components
- [ ] Integration test for e2e flow

### Phase 2: Quota System
- [ ] Quota manager implementation
- [ ] Usage tracking per tenant
- [ ] Monthly reset logic
- [ ] Quota-aware API responses
- [ ] Usage endpoint
- [ ] Tests for quota enforcement

### Phase 3: BYOA Support
- [ ] API key encryption/decryption
- [ ] Multi-tenant API client
- [ ] AI config management endpoints
- [ ] BYOA tier logic
- [ ] Tests for BYOA flow

### Phase 4: Validation & Promotion
- [ ] Validator service
- [ ] Environment management
- [ ] Promotion workflow
- [ ] Rollback functionality
- [ ] Tests for lifecycle management

### Phase 5: Monitoring
- [ ] Monitoring service
- [ ] Optimization engine
- [ ] Background workers
- [ ] Notification system
- [ ] Tests for monitoring

### Phase 6: Production Ready
- [ ] PostgreSQL implementation
- [ ] Prometheus metrics
- [ ] Grafana dashboards
- [ ] Security hardening
- [ ] Documentation
- [ ] Load testing
- [ ] Deployment automation

---

## Conclusion

This implementation plan provides a complete roadmap for building the ByteFreezer AI Agent. The phased approach allows for:

1. **Early validation** - Get feedback on core features quickly
2. **Iterative improvement** - Each phase builds on the previous
3. **Risk management** - Critical features (security, quotas) in early phases
4. **Business alignment** - Free tier drives adoption, BYOA provides scalability

**Key Success Factors:**
- Start simple, iterate based on usage
- Monitor costs closely in early phases
- Get customer feedback on AI quality
- Build observability from day 1
- Security and reliability are non-negotiable

**Estimated Timeline:**
- Phase 1-3: 3 weeks (MVP with BYOA)
- Phase 4-5: 2 weeks (Full lifecycle)
- Phase 6: 1-2 weeks (Production ready)
- **Total: 6-7 weeks to production**

Good luck with implementation! Feel free to ask questions as you build.
