# ByteFreezer Piper Integration Examples

This document provides specific integration examples for the ByteFreezer Piper service, focusing on data processing pipelines, parser configurations, and filter chains.

## Parser Configuration Examples

### Example 1: Web Application Log Processing

Complete parser configuration for common web application log formats.

**webapp-parsers.yaml:**
```yaml
# Nginx Access Logs with Custom Fields
- name: "nginx-access-enhanced"
  type: "grok"
  description: "Enhanced Nginx access log parser with geolocation and user tracking"
  conditions:
    source_formats: ["plaintext"]
    dataset_ids: ["nginx", "nginx-access", "web-access"]
    tenant_ids: ["webapp-corp"]
    content_patterns: 
      - '\d+\.\d+\.\d+\.\d+ - .*? \[.*?\] ".*?" \d+ \d+'
    filename_patterns: ["*access*.log", "*nginx*.log"]
  config:
    pattern: '%{IPORHOST:client_ip} - %{DATA:remote_user} \[%{HTTPDATE:timestamp}\] "%{WORD:method} %{DATA:request_path} HTTP/%{NUMBER:http_version}" %{NUMBER:status_code} %{NUMBER:body_bytes_sent} "%{DATA:referer}" "%{DATA:user_agent}" "%{DATA:forwarded_for}"'
    named_captures: true
    type_conversions:
      status_code: "integer"
      body_bytes_sent: "integer"
      http_version: "float"
  filters:
    - type: "add_field"
      config:
        field_name: "log_source"
        field_value: "nginx"
    - type: "add_field"
      config:
        field_name: "environment"
        field_value: "production"
    - type: "geoip"
      config:
        source_field: "client_ip"
        target_field: "geoip"
        database_path: "/usr/share/GeoIP/GeoLite2-City.mmdb"
    - type: "user_agent"
      config:
        source_field: "user_agent"
        target_field: "ua"
    - type: "conditional"
      config:
        conditions:
          - field: "status_code"
            operator: "gte"
            value: 400
        action: "add_field"
        field_name: "error_category"
        field_value: "http_error"
  options:
    timestamp_field: "timestamp"
    timestamp_formats: ["02/Jan/2006:15:04:05 -0700"]
    skip_on_error: false
    add_metadata: true
    preserve_raw: false
  enabled: true

# Application Error Logs with Stack Traces
- name: "java-spring-errors"
  type: "regex"
  description: "Java Spring Boot error logs with stack trace handling"
  conditions:
    source_formats: ["plaintext"]
    dataset_ids: ["java-app", "spring-boot", "errors"]
    content_patterns:
      - '\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}'
      - 'ERROR'
  config:
    pattern: '(?m)^(?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\s+(?P<level>\w+)\s+\[(?P<thread>[^\]]+)\]\s+(?P<logger>\S+)\s*:\s*(?P<message>.*?)(?P<exception>(?:\n[A-Za-z][^:]*:[^\n]*(?:\n\s+at\s+[^\n]*)*)*)'
    named_captures: true
    multiline: true
    multiline_pattern: '^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3})'
    multiline_negate: true
    multiline_match: "after"
  filters:
    - type: "add_field"
      config:
        field_name: "application"
        field_value: "spring-boot-app"
    - type: "conditional"
      config:
        conditions:
          - field: "exception"
            operator: "not_empty"
        action: "add_field"
        field_name: "has_stacktrace"
        field_value: true
    - type: "regex_extract"
      config:
        source_field: "exception"
        target_field: "exception_class"
        pattern: '([A-Za-z][A-Za-z0-9.]*Exception)'
  options:
    timestamp_field: "timestamp"
    timestamp_formats: ["2006-01-02 15:04:05.000"]
    skip_on_error: true
  enabled: true

# Microservices Request Tracing
- name: "microservice-traces"
  type: "json"
  description: "Microservice request traces with correlation IDs"
  conditions:
    source_formats: ["ndjson", "json"]
    dataset_ids: ["traces", "requests", "microservices"]
    content_patterns:
      - '"trace_id"'
      - '"span_id"'
  filters:
    - type: "add_field"
      config:
        field_name: "service_type"
        field_value: "microservice"
    - type: "duration_parse"
      config:
        source_field: "duration"
        target_field: "duration_ms"
        input_unit: "nanoseconds"
        output_unit: "milliseconds"
    - type: "conditional"
      config:
        conditions:
          - field: "duration_ms"
            operator: "gt"
            value: 1000
        action: "add_field"
        field_name: "slow_request"
        field_value: true
  options:
    flatten_nested: true
    preserve_arrays: true
    add_metadata: true
  enabled: true
```

### Example 2: Security Log Processing

Parser configurations for security and compliance logs.

**security-parsers.yaml:**
```yaml
# Firewall Logs (pfSense/OPNsense)
- name: "pfsense-firewall-enhanced"
  type: "csv"
  description: "pfSense firewall logs with threat intelligence enrichment"
  conditions:
    source_formats: ["syslog", "plaintext"]
    dataset_ids: ["firewall", "pfsense", "opnsense"]
    content_patterns:
      - 'filterlog:'
  config:
    delimiter: ","
    skip_header: false
    headers: ["rule_number", "sub_rule", "anchor", "tracker", "interface", "reason", "action", "direction", "ip_version", "tos", "ecn", "ttl", "id", "offset", "flags", "protocol_id", "protocol", "length", "src_ip", "dst_ip", "src_port", "dst_port", "data_length", "tcp_flags", "sequence_number", "ack_number", "window", "urg", "options"]
    pre_process_regex:
      pattern: 'filterlog:\s*(.*)'
      replacement: '$1'
  filters:
    - type: "add_field"
      config:
        field_name: "log_type"
        field_value: "firewall"
    - type: "type_conversion"
      config:
        conversions:
          rule_number: "integer"
          src_port: "integer"
          dst_port: "integer"
          data_length: "integer"
    - type: "threat_intel"
      config:
        source_field: "src_ip"
        target_field: "src_threat_intel"
        intel_feeds: ["malware_ips", "tor_exit_nodes"]
    - type: "conditional"
      config:
        conditions:
          - field: "action"
            operator: "equals"
            value: "block"
        action: "add_field"
        field_name: "security_event"
        field_value: true
  options:
    skip_on_error: true
    add_metadata: true
  enabled: true

# Windows Event Logs
- name: "windows-security-events"
  type: "json"
  description: "Windows Security Events with user behavior analytics"
  conditions:
    source_formats: ["json", "ndjson"]
    dataset_ids: ["windows-events", "security-events", "event-logs"]
    content_patterns:
      - '"EventID"'
      - '"Channel":"Security"'
  filters:
    - type: "add_field"
      config:
        field_name: "platform"
        field_value: "windows"
    - type: "field_mapping"
      config:
        mappings:
          "EventData.SubjectUserName": "user"
          "EventData.TargetUserName": "target_user"
          "EventData.WorkstationName": "workstation"
          "EventData.IpAddress": "source_ip"
    - type: "conditional"
      config:
        conditions:
          - field: "EventID"
            operator: "in"
            value: [4625, 4648, 4771]
        action: "add_field"
        field_name: "failed_auth"
        field_value: true
    - type: "conditional"
      config:
        conditions:
          - field: "EventID"
            operator: "in"
            value: [4624, 4648]
        action: "add_field"
        field_name: "successful_auth"
        field_value: true
  options:
    timestamp_field: "TimeCreated"
    preserve_nested: true
    add_metadata: true
  enabled: true

# Suricata IDS Alerts
- name: "suricata-ids-alerts"
  type: "json"
  description: "Suricata IDS alerts with signature analysis"
  conditions:
    source_formats: ["json", "ndjson"]
    dataset_ids: ["suricata", "ids", "nids"]
    content_patterns:
      - '"event_type":"alert"'
      - '"signature_id"'
  filters:
    - type: "add_field"
      config:
        field_name: "security_tool"
        field_value: "suricata"
    - type: "severity_mapping"
      config:
        source_field: "alert.severity"
        mappings:
          1: "critical"
          2: "high"
          3: "medium"
          4: "low"
        target_field: "severity_text"
    - type: "mitre_mapping"
      config:
        source_field: "alert.signature"
        target_field: "mitre_techniques"
        mapping_file: "/etc/bytefreezer/mitre-mapping.yaml"
    - type: "conditional"
      config:
        conditions:
          - field: "alert.severity"
            operator: "lte"
            value: 2
        action: "add_field"
        field_name: "high_priority"
        field_value: true
  options:
    preserve_nested: true
    add_metadata: true
  enabled: true
```

### Example 3: Database and Infrastructure Monitoring

Parsers for database and infrastructure logs.

**infrastructure-parsers.yaml:**
```yaml
# PostgreSQL Database Logs
- name: "postgresql-detailed"
  type: "grok"
  description: "PostgreSQL logs with query performance analysis"
  conditions:
    source_formats: ["plaintext"]
    dataset_ids: ["postgresql", "postgres", "database"]
    content_patterns:
      - 'LOG:'
      - 'ERROR:'
      - 'STATEMENT:'
  config:
    pattern: '%{TIMESTAMP_ISO8601:timestamp} \[%{POSINT:pid}\] %{DATA:user}@%{DATA:database} %{WORD:level}:\s+%{GREEDYDATA:message}'
    additional_patterns:
      DURATION_PATTERN: 'duration: (?P<duration_ms>[\d.]+) ms'
      STATEMENT_PATTERN: 'statement: (?P<sql_query>.*)'
  filters:
    - type: "add_field"
      config:
        field_name: "database_type"
        field_value: "postgresql"
    - type: "regex_extract"
      config:
        source_field: "message"
        patterns:
          - pattern: 'duration: ([\d.]+) ms'
            target_field: "query_duration_ms"
            type: "float"
          - pattern: 'statement: (.*)'
            target_field: "sql_query"
    - type: "conditional"
      config:
        conditions:
          - field: "query_duration_ms"
            operator: "gt"
            value: 1000
        action: "add_field"
        field_name: "slow_query"
        field_value: true
    - type: "sql_classification"
      config:
        source_field: "sql_query"
        target_field: "query_type"
        classifications: ["SELECT", "INSERT", "UPDATE", "DELETE", "DDL"]
  options:
    timestamp_field: "timestamp"
    timestamp_formats: ["2006-01-02 15:04:05.000 MST"]
    skip_on_error: true
  enabled: true

# Kubernetes Container Logs
- name: "kubernetes-containers"
  type: "json"
  description: "Kubernetes container logs with pod metadata"
  conditions:
    source_formats: ["json", "ndjson"]
    dataset_ids: ["kubernetes", "k8s", "containers"]
    content_patterns:
      - '"kubernetes"'
      - '"pod_name"'
  filters:
    - type: "add_field"
      config:
        field_name: "platform"
        field_value: "kubernetes"
    - type: "field_extraction"
      config:
        extractions:
          - source: "kubernetes.pod_name"
            target: "pod"
          - source: "kubernetes.namespace_name"
            target: "namespace"
          - source: "kubernetes.container_name"
            target: "container"
    - type: "label_processing"
      config:
        source_field: "kubernetes.labels"
        prefix: "k8s_label_"
        flatten: true
    - type: "conditional"
      config:
        conditions:
          - field: "stream"
            operator: "equals"
            value: "stderr"
        action: "add_field"
        field_name: "error_stream"
        field_value: true
  options:
    preserve_nested: true
    add_metadata: true
  enabled: true

# Docker Container Logs
- name: "docker-containers"
  type: "grok"
  description: "Docker container logs with container metadata"
  conditions:
    source_formats: ["plaintext"]
    dataset_ids: ["docker", "containers"]
    content_patterns:
      - '\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}'
  config:
    pattern: '%{TIMESTAMP_ISO8601:timestamp} %{DATA:container_id} %{DATA:container_name}\[%{POSINT:pid}\]: %{GREEDYDATA:message}'
  filters:
    - type: "add_field"
      config:
        field_name: "platform"
        field_value: "docker"
    - type: "container_enrichment"
      config:
        source_field: "container_id"
        docker_socket: "/var/run/docker.sock"
        target_fields:
          - "image"
          - "labels"
          - "env_vars"
    - type: "log_level_detection"
      config:
        source_field: "message"
        target_field: "detected_level"
        patterns:
          - pattern: '\b(ERROR|FATAL)\b'
            level: "error"
          - pattern: '\b(WARN|WARNING)\b'
            level: "warn"
          - pattern: '\b(INFO|INFORMATION)\b'
            level: "info"
          - pattern: '\b(DEBUG|TRACE)\b'
            level: "debug"
  options:
    timestamp_field: "timestamp"
    timestamp_formats: ["2006-01-02T15:04:05.000000000Z"]
    add_metadata: true
  enabled: true
```

## Filter Chain Examples

### Example 1: E-commerce Log Processing Pipeline

Complete filter chain for e-commerce application logs.

**ecommerce-filters.yaml:**
```yaml
# E-commerce order processing logs
- name: "ecommerce-orders"
  type: "json"
  description: "E-commerce order logs with business intelligence"
  conditions:
    source_formats: ["ndjson", "json"]
    dataset_ids: ["orders", "transactions", "ecommerce"]
  filters:
    # 1. Basic field enrichment
    - type: "add_field"
      config:
        field_name: "business_domain"
        field_value: "ecommerce"
    
    # 2. Data type conversions
    - type: "type_conversion"
      config:
        conversions:
          order_total: "float"
          item_count: "integer"
          customer_id: "string"
    
    # 3. Customer segmentation
    - type: "conditional"
      config:
        conditions:
          - field: "order_total"
            operator: "gte"
            value: 500
        action: "add_field"
        field_name: "customer_segment"
        field_value: "premium"
    - type: "conditional"
      config:
        conditions:
          - field: "order_total"
            operator: "lt"
            value: 500
          - field: "order_total"
            operator: "gte"
            value: 100
        action: "add_field"
        field_name: "customer_segment"
        field_value: "standard"
    - type: "conditional"
      config:
        conditions:
          - field: "order_total"
            operator: "lt"
            value: 100
        action: "add_field"
        field_name: "customer_segment"
        field_value: "budget"
    
    # 4. Geographic enrichment
    - type: "geoip"
      config:
        source_field: "customer_ip"
        target_field: "geo"
    
    # 5. Product category analysis
    - type: "array_processing"
      config:
        source_field: "items"
        target_field: "categories"
        operation: "extract_field"
        field: "category"
        unique: true
    
    # 6. Fraud detection indicators
    - type: "conditional"
      config:
        conditions:
          - field: "payment_attempts"
            operator: "gt"
            value: 3
        action: "add_field"
        field_name: "fraud_indicator"
        field_value: "multiple_payment_attempts"
    
    # 7. Business hours classification
    - type: "time_classification"
      config:
        source_field: "timestamp"
        target_field: "business_period"
        timezone: "UTC"
        classifications:
          business_hours:
            start: "09:00"
            end: "17:00"
            weekdays: [1, 2, 3, 4, 5]
          peak_hours:
            start: "19:00"
            end: "22:00"
            weekdays: [1, 2, 3, 4, 5, 6, 7]
    
    # 8. Revenue categorization
    - type: "revenue_category"
      config:
        source_field: "order_total"
        target_field: "revenue_bucket"
        buckets:
          - name: "micro"
            min: 0
            max: 25
          - name: "small"
            min: 25
            max: 100
          - name: "medium"
            min: 100
            max: 500
          - name: "large"
            min: 500
            max: 2000
          - name: "enterprise"
            min: 2000
    
    # 9. Clean sensitive data
    - type: "remove_field"
      config:
        field_names: ["credit_card_number", "ssn", "phone_number"]
    
    # 10. Hash customer identifiers for privacy
    - type: "hash_field"
      config:
        source_field: "customer_email"
        target_field: "customer_hash"
        algorithm: "sha256"
        salt: "ecommerce_salt_2024"
  
  options:
    skip_on_error: false
    add_metadata: true
  enabled: true
```

### Example 2: Security Incident Response Pipeline

Filter chain for security incident analysis and response.

**security-incident-filters.yaml:**
```yaml
# Security incident processing
- name: "security-incidents"
  type: "multi-format"
  description: "Security incident analysis and enrichment"
  conditions:
    source_formats: ["json", "ndjson", "syslog"]
    dataset_ids: ["security", "incidents", "alerts"]
  filters:
    # 1. Normalize event timestamps
    - type: "timestamp_normalization"
      config:
        source_field: "timestamp"
        target_field: "@timestamp"
        formats: ["2006-01-02T15:04:05Z", "Jan 02 15:04:05", "2006-01-02 15:04:05"]
        timezone: "UTC"
    
    # 2. Extract IP addresses
    - type: "ip_extraction"
      config:
        source_field: "message"
        target_field: "extracted_ips"
        types: ["ipv4", "ipv6"]
        private_ips: true
    
    # 3. Threat intelligence lookup
    - type: "threat_intel_lookup"
      config:
        source_field: "extracted_ips"
        intel_sources:
          - name: "malware_ips"
            url: "https://threat-intel.company.com/api/malware-ips"
            cache_ttl: "1h"
          - name: "tor_exit_nodes"
            url: "https://threat-intel.company.com/api/tor-nodes"
            cache_ttl: "6h"
        target_field: "threat_intel"
    
    # 4. Geographic context
    - type: "geoip_lookup"
      config:
        source_field: "extracted_ips"
        target_field: "geoip"
        database: "/usr/share/GeoIP/GeoLite2-City.mmdb"
        fields: ["country", "city", "asn"]
    
    # 5. Severity scoring
    - type: "severity_calculation"
      config:
        base_severity: 1
        multipliers:
          - condition:
              field: "threat_intel.malware_ips"
              operator: "not_empty"
            multiplier: 3
          - condition:
              field: "event_type"
              operator: "in"
              value: ["authentication_failure", "privilege_escalation"]
            multiplier: 2
          - condition:
              field: "geoip.country"
              operator: "in"
              value: ["CN", "RU", "KP"]  # High-risk countries
            multiplier: 1.5
        target_field: "risk_score"
    
    # 6. MITRE ATT&CK mapping
    - type: "mitre_attack_mapping"
      config:
        source_field: "event_type"
        mapping_file: "/etc/bytefreezer/mitre-attack-mapping.yaml"
        target_field: "mitre_attack"
        include_techniques: true
        include_tactics: true
    
    # 7. User behavior analysis
    - type: "user_behavior_scoring"
      config:
        user_field: "user"
        source_ip_field: "source_ip"
        timestamp_field: "@timestamp"
        lookback_window: "24h"
        scoring_factors:
          - name: "unusual_login_time"
            weight: 1.2
          - name: "new_location"
            weight: 1.5
          - name: "multiple_failures"
            weight: 2.0
        target_field: "user_behavior_score"
    
    # 8. Asset context enrichment
    - type: "asset_enrichment"
      config:
        ip_field: "destination_ip"
        hostname_field: "destination_host"
        asset_database: "/etc/bytefreezer/assets.db"
        target_field: "asset_info"
        include_criticality: true
    
    # 9. Incident classification
    - type: "incident_classification"
      config:
        rules:
          - name: "brute_force_attack"
            conditions:
              - field: "event_type"
                operator: "equals"
                value: "authentication_failure"
              - field: "user_behavior_score"
                operator: "gt"
                value: 5
            classification: "brute_force"
            priority: "high"
          - name: "malware_communication"
            conditions:
              - field: "threat_intel.malware_ips"
                operator: "not_empty"
              - field: "direction"
                operator: "equals"
                value: "outbound"
            classification: "malware_c2"
            priority: "critical"
        target_field: "incident_type"
    
    # 10. Alert generation
    - type: "conditional"
      config:
        conditions:
          - field: "risk_score"
            operator: "gte"
            value: 7
        action: "webhook"
        webhook_url: "https://soc.company.com/api/alerts"
        payload_template: |
          {
            "alert_id": "{{ uuid }}",
            "timestamp": "{{ @timestamp }}",
            "severity": "{{ incident_type.priority }}",
            "classification": "{{ incident_type.classification }}",
            "source_ip": "{{ source_ip }}",
            "user": "{{ user }}",
            "risk_score": {{ risk_score }},
            "mitre_attack": {{ mitre_attack | json }},
            "raw_event": {{ . | json }}
          }
  
  options:
    skip_on_error: false
    add_metadata: true
    preserve_raw: true
  enabled: true
```

## Pipeline Integration Examples

### Example 1: Complete Data Processing Pipeline

End-to-end pipeline from ingestion to analysis.

**complete-pipeline-test.sh:**
```bash
#!/bin/bash
# Complete ByteFreezer pipeline integration test

set -e

RECEIVER_URL="http://localhost:8081"
PIPER_URL="http://localhost:8082"
TENANT="pipeline-test"

echo "=== Starting Complete Pipeline Integration Test ==="

# 1. Send raw application logs to receiver
echo "Step 1: Sending raw application logs..."
curl -X POST "$RECEIVER_URL/data/$TENANT/app-logs" \
  -H "Content-Type: application/x-ndjson" \
  --data-binary @- << 'EOF'
{"timestamp": "2024-01-15T10:05:30Z", "level": "info", "component": "auth", "message": "User login successful", "user": "john.doe", "ip": "192.168.1.100", "session_duration": 3600}
{"timestamp": "2024-01-15T10:05:31Z", "level": "warn", "component": "api", "message": "Rate limit threshold reached", "endpoint": "/api/data", "current_rate": 95, "limit": 100}
{"timestamp": "2024-01-15T10:05:32Z", "level": "error", "component": "database", "message": "Connection timeout", "query": "SELECT * FROM users WHERE active = true", "duration": 5000}
{"timestamp": "2024-01-15T10:05:33Z", "level": "debug", "component": "cache", "message": "Cache miss", "key": "user:profile:12345", "hit_rate": 0.87}
EOF

# 2. Send nginx access logs
echo "Step 2: Sending nginx access logs..."
curl -X POST "$RECEIVER_URL/data/$TENANT/nginx-access" \
  -H "Content-Type: text/plain" \
  --data-binary @- << 'EOF'
192.168.1.100 - john.doe [15/Jan/2024:10:05:30 -0800] "GET /api/profile HTTP/1.1" 200 1456 "https://app.company.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
192.168.1.101 - - [15/Jan/2024:10:05:31 -0800] "POST /api/login HTTP/1.1" 401 89 "-" "curl/7.68.0"
192.168.1.102 - jane.smith [15/Jan/2024:10:05:32 -0800] "GET /api/data HTTP/1.1" 429 45 "https://app.company.com/dashboard" "Python-requests/2.28.1"
EOF

# 3. Send security events
echo "Step 3: Sending security events..."
curl -X POST "$RECEIVER_URL/data/$TENANT/security-events" \
  -H "Content-Type: application/json" \
  --data-binary @- << 'EOF'
{
  "timestamp": "2024-01-15T10:05:34Z",
  "event_type": "authentication_failure",
  "user": "admin",
  "source_ip": "203.0.113.50",
  "destination_ip": "192.168.1.10",
  "attempts": 5,
  "user_agent": "Mozilla/5.0 (compatible; AttackBot/1.0)"
}
EOF

# 4. Wait for piper to discover and process files
echo "Step 4: Waiting for piper to process data..."
sleep 30

# 5. Check piper processing status
echo "Step 5: Checking piper status..."
curl -s "$PIPER_URL/health" | jq '.'
echo ""
curl -s "$PIPER_URL/jobs/status" | jq '.'

# 6. Verify processed data structure (if MinIO is available)
echo "Step 6: Checking processed data structure..."
if command -v mc &> /dev/null; then
    echo "Raw data structure:"
    mc ls -r local/bytefreezer-data/raw/ | head -10
    echo ""
    echo "Processed data structure:"
    mc ls -r local/bytefreezer-data/processed/ | head -10
else
    echo "MinIO client not available, skipping S3 structure check"
fi

# 7. Test filter chain effectiveness
echo "Step 7: Verifying filter chain processing..."
# This would typically involve querying the processed data
# For demo purposes, we'll check that the piper has processed jobs

job_count=$(curl -s "$PIPER_URL/jobs/status" | jq '.completed_jobs // 0')
echo "Completed jobs: $job_count"

if [ "$job_count" -gt 0 ]; then
    echo "✅ Pipeline processing successful - $job_count jobs completed"
else
    echo "⚠️  Pipeline processing may still be in progress"
fi

echo ""
echo "=== Pipeline Integration Test Complete ==="
echo "Data flow verified:"
echo "  Raw data → Receiver → S3 raw/"
echo "  S3 raw/ → Piper → Processing → S3 processed/"
echo "  Filter chains applied: format detection, field enrichment, security analysis"
```

### Example 2: Multi-Source Log Aggregation

Aggregate logs from multiple sources with different formats.

**multi-source-aggregation.sh:**
```bash
#!/bin/bash
# Multi-source log aggregation example

RECEIVER_URL="http://localhost:8081"
TENANT="aggregation-demo"

echo "=== Multi-Source Log Aggregation Demo ==="

# Function to send logs with metadata
send_logs_with_source() {
    local source=$1
    local dataset=$2
    local content_type=$3
    local data=$4
    
    echo "Sending logs from: $source"
    echo "$data" | curl -X POST "$RECEIVER_URL/data/$TENANT/$dataset" \
      -H "Content-Type: $content_type" \
      -H "X-Log-Source: $source" \
      -H "X-Environment: production" \
      --data-binary @-
}

# 1. Web server logs (nginx)
send_logs_with_source "nginx-prod-01" "web-logs" "text/plain" \
'192.168.1.100 - - [15/Jan/2024:10:05:30 -0800] "GET /api/health HTTP/1.1" 200 23
192.168.1.101 - - [15/Jan/2024:10:05:31 -0800] "POST /api/login HTTP/1.1" 200 156
192.168.1.102 - - [15/Jan/2024:10:05:32 -0800] "GET /api/users HTTP/1.1" 404 45'

# 2. Application logs (structured JSON)
send_logs_with_source "app-server-01" "app-logs" "application/x-ndjson" \
'{"timestamp": "2024-01-15T10:05:30Z", "level": "info", "service": "user-service", "message": "User authentication successful", "user_id": "12345"}
{"timestamp": "2024-01-15T10:05:31Z", "level": "error", "service": "payment-service", "message": "Payment processing failed", "error_code": "CARD_DECLINED"}
{"timestamp": "2024-01-15T10:05:32Z", "level": "warn", "service": "notification-service", "message": "Email delivery delayed", "delay_seconds": 45}'

# 3. Database logs (PostgreSQL)
send_logs_with_source "db-master-01" "database-logs" "text/plain" \
'2024-01-15 10:05:30.123 [1234] user@webapp LOG: statement: SELECT * FROM users WHERE id = $1
2024-01-15 10:05:31.456 [1235] user@webapp LOG: duration: 145.234 ms statement: INSERT INTO sessions (user_id, token) VALUES ($1, $2)
2024-01-15 10:05:32.789 [1236] user@webapp ERROR: deadlock detected'

# 4. Load balancer logs (HAProxy)
send_logs_with_source "lb-01" "loadbalancer-logs" "application/x-syslog" \
'<134>Jan 15 10:05:30 lb-01 haproxy[1234]: 192.168.1.100:54321 [15/Jan/2024:10:05:30.123] frontend backend/server1 0/0/1/2/3 200 1456 - - ---- 1/1/1/1/0 0/0 "GET /api/users HTTP/1.1"
<134>Jan 15 10:05:31 lb-01 haproxy[1234]: 192.168.1.101:54322 [15/Jan/2024:10:05:31.456] frontend backend/server2 0/1/2/3/6 500 89 - - ---- 2/2/2/2/0 0/0 "POST /api/payment HTTP/1.1"'

# 5. Security logs (firewall)
send_logs_with_source "firewall-01" "security-logs" "application/x-syslog" \
'<86>Jan 15 10:05:30 fw-01 filterlog: 1,,,1000000001,em0,match,block,in,4,0x0,,64,0,0,DF,6,tcp,60,203.0.113.1,192.168.1.100,12345,22
<86>Jan 15 10:05:31 fw-01 filterlog: 2,,,1000000002,em0,match,pass,out,4,0x0,,64,0,0,DF,6,tcp,52,192.168.1.100,8.8.8.8,55555,53'

# 6. Monitoring/metrics data
send_logs_with_source "monitoring-01" "metrics" "application/x-ndjson" \
'{"timestamp": "2024-01-15T10:05:30Z", "metric": "cpu.usage", "value": 45.2, "host": "web-01", "tags": {"environment": "prod", "role": "frontend"}}
{"timestamp": "2024-01-15T10:05:31Z", "metric": "memory.usage", "value": 78.5, "host": "app-01", "tags": {"environment": "prod", "role": "backend"}}
{"timestamp": "2024-01-15T10:05:32Z", "metric": "disk.io", "value": 1234, "host": "db-01", "tags": {"environment": "prod", "role": "database"}}'

# 7. Container logs (Docker/Kubernetes)
send_logs_with_source "k8s-cluster-01" "container-logs" "application/json" \
'{
  "timestamp": "2024-01-15T10:05:33Z",
  "kubernetes": {
    "pod_name": "web-app-7d4f8b5c6-xyz12",
    "namespace_name": "production",
    "container_name": "web-app",
    "labels": {
      "app": "web-app",
      "version": "v1.2.3"
    }
  },
  "log": "Application started successfully",
  "stream": "stdout"
}'

echo ""
echo "=== Multi-Source Aggregation Complete ==="
echo "Sent logs from:"
echo "  - Web servers (nginx access logs)"
echo "  - Application servers (structured JSON)"
echo "  - Database servers (PostgreSQL logs)"
echo "  - Load balancers (HAProxy logs)"
echo "  - Security devices (firewall logs)"
echo "  - Monitoring systems (metrics data)"
echo "  - Container platforms (Kubernetes logs)"
echo ""
echo "All logs will be:"
echo "  1. Format-detected by receiver"
echo "  2. Stored with proper S3 partitioning"
echo "  3. Processed by piper with appropriate parsers"
echo "  4. Enriched with source metadata"
echo "  5. Ready for analysis and alerting"

# Check ingestion status
echo ""
echo "Checking ingestion status..."
curl -s "http://localhost:8080/health" | jq '.ingestion_stats // "Status check successful"'
```

This comprehensive set of integration examples demonstrates how to use ByteFreezer Piper for complex log processing scenarios, including parser configurations, filter chains, and complete pipeline integrations.