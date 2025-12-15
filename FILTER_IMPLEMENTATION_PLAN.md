# ByteFreezer-Piper Filter Implementation Plan

## Overview
Adding 10 new transformation filters to match Logstash capabilities (Phases 1 & 2)

## Phase 1: Critical Filters (Must Have)

### 1. Grok Filter
**Purpose**: Pattern-based parsing of unstructured log data into structured fields

**Implementation Details**:
- Pattern library with common patterns (IP, TIMESTAMP, NUMBER, WORD, etc.)
- Named capture groups
- Pattern composition (patterns referencing other patterns)
- Multiple pattern matching attempts
- Custom pattern support

**Common Patterns to Include**:
```
IP: \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}
TIMESTAMP_ISO8601: \d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}
NUMBER: (?:[+-]?(?:[0-9]+))
WORD: \b\w+\b
QUOTEDSTRING: "(?:[^"\\]|\\.)*"
USERNAME: [a-zA-Z0-9._-]+
PATH: (?:/[^/\s]+)+
COMMONAPACHELOG: %{IP:client} %{USER:ident} %{USER:auth} \[%{TIMESTAMP:timestamp}\] "(?:%{WORD:verb} %{NOTSPACE:request}(?: HTTP/%{NUMBER:httpversion})?|%{DATA:rawrequest})" %{NUMBER:response} (?:%{NUMBER:bytes}|-)
```

**Configuration**:
```json
{
  "type": "grok",
  "config": {
    "source_field": "message",
    "pattern": "%{COMMONAPACHELOG}",
    "patterns_dir": "/etc/piper/patterns",
    "break_on_match": true,
    "keep_empty_captures": false,
    "named_captures_only": true
  }
}
```

**Files to Create/Modify**:
- `pipeline/filters/grok.go` - Main implementation
- `pipeline/filters/grok_patterns.go` - Pattern library
- `pipeline/filter_registry.go` - Register grok filter

---

### 2. Mutate Filter (Enhanced)
**Purpose**: Advanced field manipulation operations

**Operations to Implement**:
- `split` - Split string into array
- `join` - Join array into string
- `gsub` - Regex find and replace
- `convert` - Type conversion (string, integer, float, boolean)
- `lowercase` - Convert to lowercase
- `uppercase` - Convert to uppercase
- `strip` - Remove whitespace
- `merge` - Merge two fields
- `update` - Update field value
- `replace` - Replace field value

**Configuration Examples**:
```json
{
  "type": "mutate",
  "config": {
    "split": {"field": "message", "separator": ","},
    "gsub": [
      {"field": "message", "pattern": "[0-9]", "replacement": "X"}
    ],
    "convert": {"status": "integer", "duration": "float"},
    "lowercase": ["name", "city"],
    "strip": ["message"],
    "merge": {"source": "tags", "target": "metadata"}
  }
}
```

**Files to Create/Modify**:
- `pipeline/filters/mutate.go` - Enhanced mutate implementation
- `pipeline/filter_registry.go` - Update mutate registration

---

### 3. Drop Filter
**Purpose**: Conditionally drop events from the pipeline

**Implementation Details**:
- Condition evaluation (field comparisons)
- Percentage-based dropping
- Pattern matching for drop decisions

**Configuration**:
```json
{
  "type": "drop",
  "config": {
    "if_field": "status",
    "equals": "200",
    "percentage": 90
  }
}
```

**Alternative** - Can be combined with existing `conditional` filter by adding "drop" action

**Files to Create/Modify**:
- Option A: `pipeline/filters/drop.go` - Standalone implementation
- Option B: Extend `pipeline/filters.go` conditional filter with "drop" action

---

### 4. KV (Key-Value) Filter
**Purpose**: Parse key-value pairs from strings

**Implementation Details**:
- Configurable field separator (default: space)
- Configurable value separator (default: =)
- Quote handling for values
- Prefix support for extracted fields
- Whitelist/blacklist for keys

**Configuration**:
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "field_split": " ",
    "value_split": "=",
    "target_field": "kv",
    "prefix": "kv_",
    "include_keys": ["user", "action", "status"],
    "trim_key": "\"",
    "trim_value": "\""
  }
}
```

**Example**:
```
Input: user=john action=login status=success
Output: {"kv_user": "john", "kv_action": "login", "kv_status": "success"}
```

**Files to Create/Modify**:
- `pipeline/filters/kv.go` - KV parser implementation
- `pipeline/filter_registry.go` - Register kv filter

---

### 5. Date Parse Filter (Complete Implementation)
**Purpose**: Parse date strings with format specifications

**Implementation Details**:
- Multiple date format support
- Timezone handling
- Fallback formats
- Unix timestamp support (seconds, milliseconds, microseconds)
- Target field specification

**Configuration**:
```json
{
  "type": "date_parse",
  "config": {
    "source_field": "timestamp",
    "target_field": "@timestamp",
    "formats": [
      "2006-01-02T15:04:05Z07:00",
      "2006-01-02 15:04:05",
      "02/Jan/2006:15:04:05 -0700",
      "unix",
      "unix_ms"
    ],
    "timezone": "UTC",
    "locale": "en"
  }
}
```

**Files to Create/Modify**:
- `pipeline/filters/date_parse.go` - Complete implementation (stub exists)
- `pipeline/filter_registry.go` - Update registration

---

## Phase 2: Important Filters (Should Have)

### 6. Split Filter
**Purpose**: Split a single event into multiple events

**Implementation Details**:
- Split by field value (array or string)
- Delimiter-based splitting for strings
- Preserve original event fields in split events

**Configuration**:
```json
{
  "type": "split",
  "config": {
    "field": "items",
    "terminator": ","
  }
}
```

**Example**:
```
Input: {"items": ["a", "b", "c"], "id": 1}
Output:
  {"item": "a", "id": 1}
  {"item": "b", "id": 1}
  {"item": "c", "id": 1}
```

**Files to Create/Modify**:
- `pipeline/filters/split.go` - Split implementation
- `pipeline/filter_registry.go` - Register split filter
- `pipeline/pipeline.go` - Handle multiple events from single filter

---

### 7. UserAgent Filter
**Purpose**: Parse user agent strings into structured data

**Implementation Details**:
- Use `github.com/ua-parser/uap-go` library
- Extract browser, OS, device information
- Version parsing

**Configuration**:
```json
{
  "type": "useragent",
  "config": {
    "source_field": "user_agent",
    "target_field": "ua",
    "regexes_file": "/etc/piper/regexes.yaml"
  }
}
```

**Example Output**:
```json
{
  "ua": {
    "name": "Chrome",
    "version": "96.0.4664.110",
    "os": "Windows",
    "os_version": "10",
    "device": "Desktop"
  }
}
```

**Files to Create/Modify**:
- `pipeline/filters/useragent.go` - UserAgent parser
- `pipeline/filter_registry.go` - Register useragent filter
- Add dependency: `go get github.com/ua-parser/uap-go`

---

### 8. DNS Filter
**Purpose**: Perform DNS lookups to enrich events

**Implementation Details**:
- Forward lookup (A records)
- Reverse lookup (PTR records)
- Caching for performance
- Timeout configuration
- Failure handling

**Configuration**:
```json
{
  "type": "dns",
  "config": {
    "resolve": ["ip_address"],
    "action": "replace",
    "target_field": "hostname",
    "nameserver": ["8.8.8.8", "8.8.4.4"],
    "timeout": 2,
    "cache_size": 1000,
    "cache_ttl": 3600
  }
}
```

**Files to Create/Modify**:
- `pipeline/filters/dns.go` - DNS lookup implementation
- `pipeline/filter_registry.go` - Register dns filter

---

### 9. GeoIP Filter (Complete Implementation)
**Purpose**: Add geographic information for IP addresses

**Implementation Details**:
- Use MaxMind GeoLite2 database
- Leverage existing `/home/andrew/workspace/bytefreezer/bytefreezer-piper/geoip/updater.go` infrastructure
- Extract: country, city, latitude, longitude, timezone

**Configuration**:
```json
{
  "type": "geoip",
  "config": {
    "source_field": "ip_address",
    "target_field": "geoip",
    "database": "/opt/geoip/GeoLite2-City.mmdb",
    "fields": ["country_name", "city_name", "latitude", "longitude"]
  }
}
```

**Example Output**:
```json
{
  "geoip": {
    "country_name": "United States",
    "country_code": "US",
    "city_name": "New York",
    "latitude": 40.7128,
    "longitude": -74.0060,
    "timezone": "America/New_York"
  }
}
```

**Files to Create/Modify**:
- `pipeline/filters/geoip.go` - Complete implementation (stub exists)
- `pipeline/filter_registry.go` - Update registration
- Integrate with `geoip/updater.go` for database management

---

### 10. Fingerprint Filter
**Purpose**: Generate event fingerprints/hashes for deduplication

**Implementation Details**:
- Hash algorithms: MD5, SHA1, SHA256, SHA512
- Concatenate selected fields for hashing
- Key separator configuration

**Configuration**:
```json
{
  "type": "fingerprint",
  "config": {
    "source_fields": ["host", "message"],
    "target_field": "fingerprint",
    "method": "SHA256",
    "key_separator": "|",
    "base64encode": false
  }
}
```

**Example**:
```
Input: {"host": "server1", "message": "error occurred"}
Output: {"fingerprint": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}
```

**Files to Create/Modify**:
- `pipeline/filters/fingerprint.go` - Hash generation
- `pipeline/filter_registry.go` - Register fingerprint filter

---

## Implementation Strategy

### Step 1: Study Existing Patterns (30 mins)
- Review `/home/andrew/workspace/bytefreezer/bytefreezer-piper/pipeline/filters.go`
- Understand filter interface and registration pattern
- Review existing filters: `add_field`, `remove_field`, `conditional`, `json_flatten`

### Step 2: Phase 1 Implementation (4-6 hours)
1. **Grok** (2 hours) - Most complex, pattern library
2. **Mutate** (1.5 hours) - Multiple operations
3. **Drop** (30 mins) - Extend conditional or new filter
4. **KV** (1 hour) - Parser logic
5. **Date Parse** (1 hour) - Complete stub implementation

**After each filter**: Build, deploy to tp1, test end-to-end

### Step 3: Phase 2 Implementation (4-5 hours)
6. **Split** (1 hour) - Event multiplication logic
7. **UserAgent** (1 hour) - Library integration
8. **DNS** (1.5 hours) - Network calls + caching
9. **GeoIP** (1 hour) - Use existing infrastructure
10. **Fingerprint** (30 mins) - Hash generation

**After each filter**: Build, deploy to tp1, test end-to-end

### Step 4: Testing & Documentation (2 hours)
- Add unit tests for each filter
- Integration tests with sample data
- Update documentation with examples
- Create example pipeline configurations

---

## Testing Strategy

### Unit Tests
Each filter gets test file: `pipeline/filters/[filter]_test.go`

**Test Cases**:
- Valid input processing
- Invalid input handling
- Edge cases (empty fields, null values)
- Configuration validation

### Integration Tests
Test complete pipelines with multiple filters:
```json
{
  "filters": [
    {"type": "grok", "config": {"pattern": "%{COMMONAPACHELOG}"}},
    {"type": "date_parse", "config": {"source_field": "timestamp"}},
    {"type": "geoip", "config": {"source_field": "client"}},
    {"type": "useragent", "config": {"source_field": "user_agent"}}
  ]
}
```

### End-to-End Tests on tp1
1. Upload test data to S3 source bucket
2. Verify piper processes with new filters
3. Check output in S3 destination bucket
4. Validate transformed data structure

---

## Dependencies to Add

```bash
go get github.com/ua-parser/uap-go/uaparser  # UserAgent parsing
go get github.com/oschwald/geoip2-golang     # GeoIP (may already exist)
```

---

## Files to Create/Modify

### New Filter Files:
- `pipeline/filters/grok.go`
- `pipeline/filters/grok_patterns.go`
- `pipeline/filters/mutate_enhanced.go` (or enhance existing)
- `pipeline/filters/drop.go` (or extend conditional)
- `pipeline/filters/kv.go`
- `pipeline/filters/date_parse.go` (complete existing)
- `pipeline/filters/split.go`
- `pipeline/filters/useragent.go`
- `pipeline/filters/dns.go`
- `pipeline/filters/geoip.go` (complete existing)
- `pipeline/filters/fingerprint.go`

### Test Files:
- `pipeline/filters/grok_test.go`
- `pipeline/filters/mutate_enhanced_test.go`
- `pipeline/filters/drop_test.go`
- `pipeline/filters/kv_test.go`
- `pipeline/filters/date_parse_test.go`
- `pipeline/filters/split_test.go`
- `pipeline/filters/useragent_test.go`
- `pipeline/filters/dns_test.go`
- `pipeline/filters/geoip_test.go`
- `pipeline/filters/fingerprint_test.go`

### Modified Files:
- `pipeline/filter_registry.go` - Register all new filters
- `pipeline/pipeline.go` - Handle split filter (multiple events)
- `go.mod` - Add new dependencies

---

## Success Criteria

✅ All 10 filters implemented
✅ All filters registered in filter_registry.go
✅ Unit tests pass for all filters
✅ Integration tests pass
✅ End-to-end test on tp1 successful
✅ Documentation updated with examples
✅ No breaking changes to existing filters
✅ Performance acceptable (< 10% overhead per filter)

---

## Rollback Plan

If issues arise:
1. Each filter is independent - can disable in registry
2. Git revert specific filter commits
3. Deploy previous piper binary from backup
4. Filter configs in control DB can disable problematic filters

---

## Timeline Estimate

- **Phase 1**: 6-8 hours
- **Phase 2**: 5-6 hours
- **Testing & Documentation**: 2-3 hours
- **Total**: 13-17 hours of focused work

---

## Notes

- Verify end-to-end after EACH filter implementation
- All filters must handle nil/missing fields gracefully
- All filters must preserve existing fields unless explicitly configured to remove
- Grok patterns should be easily extensible (external pattern files)
- DNS filter must have timeout protection to prevent pipeline stalls
- GeoIP database updates should use existing geoip/updater.go infrastructure
