# ByteFreezer Piper Documentation

Comprehensive documentation for ByteFreezer Piper filters and pipeline configuration.

## Documentation Index

### Getting Started
- **[Transformation Workflow](TRANSFORMATION_WORKFLOW.md)** - Step-by-step guide for developing and deploying transformations
- **[Transformation API Testing](TRANSFORMATION_API_TESTING.md)** - Complete API reference with testing examples
- **[Quick Reference](QUICK_REFERENCE.md)** - Fast lookup for filter syntax and common patterns
- **[Filter Reference Guide](FILTERS.md)** - Complete documentation for all 24 filters
- **[Filter Chaining Examples](FILTER_CHAINING_EXAMPLES.md)** - 10 real-world use cases with complete pipelines

### Specific Filter Guides
- **[Grok Filter](filters/GROK_FILTER.md)** - Pattern-based log parsing
- **[Mutate Filter](filters/MUTATE_FILTER.md)** - Advanced field manipulation
- **[Drop Filter](filters/DROP_FILTER.md)** - Conditional event dropping
- **[KV Filter](filters/KV_FILTER.md)** - Key-value pair parsing

## Quick Links

### By Use Case

**Web Server Logs**:
- [Apache/Nginx Log Processing](FILTER_CHAINING_EXAMPLES.md#1-web-server-log-processing)
- [Bot Traffic Filtering](FILTER_CHAINING_EXAMPLES.md#7-bot-traffic-filtering)

**Application Logs**:
- [Structured Log Analysis](FILTER_CHAINING_EXAMPLES.md#2-application-log-analysis)
- [Multi-Source Normalization](FILTER_CHAINING_EXAMPLES.md#8-multi-source-log-normalization)

**Security & Compliance**:
- [Security Event Processing](FILTER_CHAINING_EXAMPLES.md#3-security-event-processing)
- [Data Privacy & Redaction](FILTER_CHAINING_EXAMPLES.md#10-data-privacy-and-redaction)

**Analytics & Monitoring**:
- [API Analytics](FILTER_CHAINING_EXAMPLES.md#4-api-analytics)
- [Cost Optimization with Sampling](FILTER_CHAINING_EXAMPLES.md#9-cost-optimization-with-sampling)

**IoT & Transactions**:
- [IoT Sensor Data](FILTER_CHAINING_EXAMPLES.md#5-iot-sensor-data)
- [E-commerce Transactions](FILTER_CHAINING_EXAMPLES.md#6-e-commerce-transaction-logs)

### By Filter Type

**Filtering Events**:
- [Include Filter](FILTERS.md#include-filter) - Keep only matching events
- [Exclude Filter](FILTERS.md#exclude-filter) - Drop matching events
- [Sample Filter](FILTERS.md#sample-filter) - Random sampling
- [Drop Filter](FILTERS.md#drop-filter) - Advanced conditional dropping

**Parsing Data**:
- [Grok Filter](FILTERS.md#grok-filter) - Pattern-based parsing
- [KV Filter](FILTERS.md#kv-filter) - Key-value pairs
- [Date Parse Filter](FILTERS.md#date-parse-filter) - Timestamps
- [Split Filter](FILTERS.md#split-filter) - Split events

**Transforming Data**:
- [Mutate Filter](FILTERS.md#mutate-filter) - Field manipulation
- [Regex Replace Filter](FILTERS.md#regex-replace-filter) - Regex operations
- [Add/Remove/Rename Field Filters](FILTERS.md#add-field-filter) - Basic field ops

**Enriching Data**:
- [GeoIP Filter](FILTERS.md#geoip-filter) - Geographic data
- [UserAgent Filter](FILTERS.md#useragent-filter) - Browser/device info
- [DNS Filter](FILTERS.md#dns-filter) - DNS lookups
- [Fingerprint Filter](FILTERS.md#fingerprint-filter) - Event hashing

## Available Filters (24 Total)

### Event Filtering (4)
1. `include` - Keep only matching events
2. `exclude` - Drop matching events
3. `sample` - Random sampling
4. `drop` - Advanced conditional dropping

### Data Parsing (4)
5. `grok` - Pattern-based parsing with 50+ built-in patterns
6. `kv` - Key-value pair parsing
7. `date_parse` - Timestamp parsing (multiple formats)
8. `split` - Split events into multiple records

### Data Transformation (5)
9. `mutate` - 13 field manipulation operations
10. `regex_replace` - Regex find and replace
11. `add_field` - Add fields with template support
12. `remove_field` - Remove sensitive/temporary fields
13. `rename_field` - Rename for normalization

### Data Enrichment (4)
14. `geoip` - Geographic information from IPs
15. `useragent` - Parse user agent strings
16. `dns` - DNS lookups with caching
17. `fingerprint` - Event hashing (MD5, SHA256, etc.)

### Utility (7)
18. `conditional` - Conditional processing with 10 operators
19. `json_validate` - Validate JSON before processing
20. `json_flatten` - Flatten nested JSON structures
21. `uppercase_keys` - Normalize key casing

## Pipeline Structure

Every pipeline configuration follows this structure:

```json
{
  "tenant_id": "your-tenant",
  "dataset_id": "your-dataset",
  "filters": [
    {
      "type": "filter_type",
      "config": {
        "parameter": "value"
      }
    }
  ]
}
```

## Quick Start Examples

### Parse Apache Logs
```json
{
  "filters": [
    {"type": "grok", "config": {"pattern": "%{COMMONAPACHELOG}"}},
    {"type": "date_parse", "config": {"source_field": "timestamp"}},
    {"type": "mutate", "config": {"convert": {"status": "integer"}}}
  ]
}
```

### Filter and Sample
```json
{
  "filters": [
    {"type": "exclude", "config": {"field": "path", "contains": "/health"}},
    {"type": "sample", "config": {"percentage": 10.0}}
  ]
}
```

### Enrich with GeoIP
```json
{
  "filters": [
    {"type": "grok", "config": {"pattern": "%{COMMONAPACHELOG}"}},
    {"type": "geoip", "config": {"source_field": "client_ip", "target_field": "geo"}},
    {"type": "useragent", "config": {"source_field": "user_agent", "target_field": "ua"}}
  ]
}
```

## Best Practices

### Filter Order
1. Sample/Filter first (reduce volume)
2. Parse (extract structure)
3. Date parsing (normalize timestamps)
4. Type conversion
5. Conditional logic
6. Enrichment (GeoIP, DNS, UserAgent)
7. Transformation (mutate, regex)
8. Metadata (add fields)
9. Cleanup (remove fields)

### Performance
- Filter early to reduce processing
- Avoid enriching events you'll drop
- Use DNS/GeoIP caching
- Remove unused fields
- Combine mutate operations

### Security
- Redact PII early with `regex_replace`
- Remove sensitive fields with `remove_field`
- Create hashed identifiers with `fingerprint`
- Validate data before processing

## Common Patterns

### Redact Credit Cards
```json
{
  "type": "regex_replace",
  "config": {
    "source_field": "card",
    "pattern": "\\d{4}-\\d{4}-\\d{4}-(\\d{4})",
    "replacement": "XXXX-XXXX-XXXX-$1"
  }
}
```

### Keep Only Errors
```json
{
  "type": "include",
  "config": {
    "field": "level",
    "matches": "^(error|critical)$"
  }
}
```

### Parse Query Parameters
```json
{
  "type": "kv",
  "config": {
    "source_field": "query_string",
    "field_split": "&",
    "value_split": "="
  }
}
```

## Testing Your Pipeline

1. Start with sample data
2. Add filters one at a time
3. Validate output at each stage
4. Check performance metrics
5. Test edge cases
6. Monitor drop rates

## Troubleshooting

### Common Issues

**Events Being Dropped**:
- Check include/exclude/drop filters
- Verify conditional logic
- Review sample percentage

**Parsing Failures**:
- Validate grok pattern with sample data
- Check date format strings
- Verify JSON is valid

**Performance Issues**:
- Move filtering earlier in pipeline
- Reduce enrichment operations
- Check DNS/GeoIP cache settings
- Remove unnecessary fields

**Missing Fields**:
- Check field names (case-sensitive)
- Verify parsing extracted fields
- Check conditional filters

## Getting Help

1. **Quick Reference**: [QUICK_REFERENCE.md](QUICK_REFERENCE.md)
2. **Full Docs**: [FILTERS.md](FILTERS.md)
3. **Examples**: [FILTER_CHAINING_EXAMPLES.md](FILTER_CHAINING_EXAMPLES.md)
4. **Implementation**: [../FILTER_IMPLEMENTATION_PLAN.md](../FILTER_IMPLEMENTATION_PLAN.md)
5. **Release Notes**: [../RELEASENOTES.md](../RELEASENOTES.md)

## Contributing

When adding new filters:
1. Follow existing filter patterns
2. Add comprehensive tests
3. Document all parameters
4. Provide usage examples
5. Update this documentation

## Version History

- **2025-10-31**: Added include, exclude, sample filters + comprehensive documentation
- **2025-10-30**: Added 10 transformation filters (grok, mutate, drop, kv, date_parse, split, useragent, dns, geoip, fingerprint)
- **Earlier**: Base filters (add_field, remove_field, rename_field, conditional, etc.)
