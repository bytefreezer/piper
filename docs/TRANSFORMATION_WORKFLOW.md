# Transformation Workflow Quick Reference

## Overview

The Transformation API provides a 6-step workflow for developing, testing, and deploying data transformations on your datasets.

```
┌─────────────────────────────────────────────────────────────────┐
│                 Transformation Development Workflow             │
└─────────────────────────────────────────────────────────────────┘

Step 1: DISCOVER          Step 2: DEVELOP           Step 3: VALIDATE
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│ Get Schema   │    →    │ Test on      │    →    │ Test on      │
│ & Samples    │         │ Samples      │         │ Fresh Data   │
└──────────────┘         └──────────────┘         └──────────────┘
      ↓                         ↓                         ↓
  View fields            Iterate filters           Verify at scale
  Pick samples          See input/output           Check errors

                               ↓

Step 4: DEPLOY            Step 5: MONITOR          Step 6: DEBUG
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│ Activate     │    →    │ View Stats   │    ←→   │ Preview      │
│ Pipeline     │         │ & Metrics    │         │ Samples      │
└──────────────┘         └──────────────┘         └──────────────┘
      ↓                         ↓                         ↓
  Enable/disable        Track performance        Debug live data
  Save config          Monitor errors            View diffs
```

## Step-by-Step Guide

### Step 1: Discover Schema (GET /schema)

**Purpose**: See what data you have and what fields are available

**Quick Command**:
```bash
curl "http://localhost:8090/api/v1/transformations/{tenant}/{dataset}/schema?count=10"
```

**What You Get**:
- List of all fields with types (string, number, boolean, etc.)
- Sample values for each field
- 10 random sample records (configurable up to 100)
- Total record count in dataset

**Use This To**:
- Identify field names for filter configuration
- Understand data structure
- Pick representative samples for testing

---

### Step 2: Develop Filters (POST /test)

**Purpose**: Build and test your transformation logic iteratively

**Quick Command**:
```bash
curl -X POST http://localhost:8090/api/v1/transformations/test \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "my-dataset",
    "filters": [
      {"type": "grok", "config": {...}, "enabled": true}
    ],
    "samples": [...]
  }'
```

**What You Get**:
- Input/output comparison for each sample
- List of applied filters
- Skipped/dropped records
- Processing time per record
- Error messages if any

**Use This To**:
- Test filter syntax and configuration
- See transformation results before deployment
- Debug filter logic with known data
- Build filter chains (up to 10 filters)

**Common Filters**:
- `grok` - Parse unstructured logs
- `mutate` - Add/remove/modify fields
- `geoip` - Enrich with geographic data
- `date_parse` - Parse timestamps
- `drop` - Conditionally drop records
- `include` - Keep only matching records
- `exclude` - Drop matching records
- See docs/FILTERS.md for all 24 filters

---

### Step 3: Validate at Scale (POST /validate)

**Purpose**: Test transformation on fresh data at scale before activation

**Quick Command**:
```bash
curl -X POST http://localhost:8090/api/v1/transformations/validate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "my-dataset",
    "filters": [...],
    "count": 100
  }'
```

**What You Get**:
- Results from N records (up to 1000) from latest file
- Source file name
- Success/error/skipped counts
- Average processing time per row
- Performance metrics

**Use This To**:
- Verify transformation works on new data
- Test at production scale
- Identify performance issues
- Find edge cases not in original samples

**Performance Guidelines**:
- < 5ms per row: Good
- 5-10ms per row: Acceptable
- \> 10ms per row: May need optimization

---

### Step 4: Activate Pipeline (POST /activate)

**Purpose**: Enable transformation for production processing

**Quick Command**:
```bash
# Activate
curl -X POST http://localhost:8090/api/v1/transformations/activate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "my-dataset",
    "filters": [...],
    "enabled": true
  }'

# Deactivate
curl -X POST http://localhost:8090/api/v1/transformations/activate \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "my-tenant",
    "dataset_id": "my-dataset",
    "filters": [],
    "enabled": false
  }'
```

**What You Get**:
- Success confirmation
- Configuration version number
- Activation timestamp

**Use This To**:
- Deploy tested transformation to production
- Update existing transformation configuration
- Disable transformation temporarily

**Best Practices**:
- Always validate on fresh data first (Step 3)
- Start with small sample percentages if using sampling
- Monitor stats closely after activation (Step 5)

---

### Step 5: Monitor Performance (GET /stats)

**Purpose**: Track real-time transformation metrics and errors

**Quick Command**:
```bash
curl "http://localhost:8090/api/v1/transformations/{tenant}/{dataset}/stats"

# Watch continuously
watch -n 5 'curl -s "http://localhost:8090/api/v1/transformations/{tenant}/{dataset}/stats" | jq .'
```

**What You Get**:
- Total records processed
- Success/error/skipped counts
- Average rows per second
- Last error message
- Last processed timestamp
- Filter count
- Enabled status

**Use This To**:
- Monitor production performance
- Track error rates
- Identify processing bottlenecks
- Alert on failures

**Warning Signs**:
- Error rate > 5%: Check filter configuration
- Rows/sec dropping: Performance issue
- High skipped count: Review drop/exclude filters

---

### Step 6: Debug Issues (GET /preview)

**Purpose**: Quick view of live transformation effect with side-by-side comparison

**Quick Command**:
```bash
curl "http://localhost:8090/api/v1/transformations/{tenant}/{dataset}/preview?count=10"
```

**What You Get**:
- Random samples showing input → output
- Applied filter list
- Enabled status
- Processing time per sample
- Last processed timestamp

**Use This To**:
- Debug transformation issues
- Verify changes are being applied
- Compare input/output quickly
- Spot-check production data

**Troubleshooting Tips**:
- Check which filters are in "applied" list
- Compare input/output for unexpected changes
- Look for fields being dropped unexpectedly
- Verify filter order is correct

---

## Common Workflows

### Workflow A: New Transformation

1. Get schema → Pick sample (10 records)
2. Test grok pattern → Verify parsing works
3. Add enrichment filters → Test chain
4. Validate on 100 fresh records → Check performance
5. Activate → Deploy to production
6. Monitor stats → Watch for errors

### Workflow B: Debug Existing Transformation

1. Check stats → See error count
2. Preview samples → Find problematic records
3. Get schema → Verify field names
4. Test with failing samples → Reproduce issue
5. Fix filter config → Test again
6. Activate updated config → Redeploy

### Workflow C: Performance Optimization

1. Check stats → Note avg rows/sec
2. Preview samples → Check filter applied list
3. Test individual filters → Measure each filter time
4. Remove unnecessary filters → Simplify chain
5. Validate at scale → Verify improvement
6. Activate → Deploy optimized version

### Workflow D: Add Sampling

1. Get current config (from activate request)
2. Add sample filter with 10% → Test on samples
3. Validate on fresh data → Check sample rate
4. Activate with sampling → Reduce volume
5. Monitor stats → Verify 90% reduction
6. Adjust percentage if needed → Iterate

---

## Filter Chaining Best Practices

### Order Matters

```
Good Order:
1. parse (grok, kv, split)
2. enrich (geoip, dns)
3. normalize (date_parse, mutate)
4. filter (drop, include, exclude)
5. clean (mutate remove_field)
6. sample (if needed)

Why: Parse first to extract fields, enrich with external data,
normalize formats, filter unwanted data, clean up, then sample.
```

### Example: Web Log Processing

```json
{
  "filters": [
    {
      "type": "grok",
      "config": {"pattern": "%{COMMONAPACHELOG}"},
      "enabled": true
    },
    {
      "type": "geoip",
      "config": {"source_field": "clientip"},
      "enabled": true
    },
    {
      "type": "date_parse",
      "config": {
        "source_field": "timestamp",
        "formats": ["dd/MMM/yyyy:HH:mm:ss Z"]
      },
      "enabled": true
    },
    {
      "type": "exclude",
      "config": {
        "field": "useragent",
        "matches": "(bot|crawler)"
      },
      "enabled": true
    },
    {
      "type": "mutate",
      "config": {"remove_field": ["message"]},
      "enabled": true
    }
  ]
}
```

---

## Quick Reference Table

| Step | Endpoint | Method | Purpose | Max Samples |
|------|----------|--------|---------|-------------|
| 1. Discover | `/transformations/{t}/{d}/schema` | GET | Get fields & samples | 100 |
| 2. Develop | `/transformations/test` | POST | Test filters on samples | - |
| 3. Validate | `/transformations/validate` | POST | Test on fresh data | 1000 |
| 4. Deploy | `/transformations/activate` | POST | Enable/disable pipeline | - |
| 5. Monitor | `/transformations/{t}/{d}/stats` | GET | View performance metrics | - |
| 6. Debug | `/transformations/{t}/{d}/preview` | GET | Preview live samples | 100 |

**Filter Limits**:
- Maximum filters per transformation: **10**
- All filters validated before activation
- Filters applied in array order

---

## Testing Script

Run the automated test suite:

```bash
./test_transformation_api.sh

# With custom settings
PIPER_URL=http://localhost:8090 \
TEST_TENANT=my-tenant \
TEST_DATASET=my-dataset \
./test_transformation_api.sh
```

---

## Documentation Links

- **Full API Guide**: `docs/TRANSFORMATION_API_TESTING.md`
- **Filter Reference**: `docs/FILTERS.md` (all 24 filters)
- **Pipeline Examples**: `docs/FILTER_CHAINING_EXAMPLES.md` (10 real-world cases)
- **Quick Filter Lookup**: `docs/QUICK_REFERENCE.md`
- **OpenAPI Docs**: `http://localhost:8090/docs` (interactive Swagger UI)

---

## Support

**Common Issues**:
- No data found: Check S3 bucket has files in `{tenant}/{dataset}/` prefix
- Filter errors: Validate configuration against `docs/FILTERS.md`
- Performance issues: Review filter chain complexity and data volume
- Preview returns errors: Ensure transformation is activated first

**Get Help**:
- Check piper logs for detailed error messages
- Review filter documentation for syntax
- Test filters individually to isolate issues
- Use test script to verify API health
