## Mutate Filter

The Mutate filter performs advanced field manipulation operations on event records. It supports a wide variety of transformations including splitting, joining, regex replacement, type conversion, case transformation, and more.

## Overview

The Mutate filter is one of the most versatile filters in ByteFreezer-Piper. It can perform multiple operations in a single filter pass, making it efficient for complex field transformations.

## Configuration

```json
{
  "type": "mutate",
  "config": {
    "split": {"field": "message", "separator": ","},
    "join": {"field": "tags", "separator": "|"},
    "gsub": [
      {"field": "message", "pattern": "[0-9]", "replacement": "X"}
    ],
    "convert": {"status": "integer", "duration": "float"},
    "lowercase": ["name", "city"],
    "uppercase": ["country_code"],
    "strip": ["message", "user"],
    "merge": [
      {"source": "tags", "target": "metadata"}
    ],
    "update": {"status": "processed"},
    "replace": {"version": "2.0"},
    "rename": {"old_name": "new_name"},
    "copy": {"source_field": "backup_field"},
    "remove": ["temp_field", "debug_info"]
  }
}
```

## Operations

All operations are optional. You can use any combination in a single mutate filter.

### 1. Split

Split a string field into an array.

**Configuration:**
```json
{
  "split": {
    "field": "message",
    "separator": ",",
    "target": "message_parts"
  }
}
```

**Example:**
```json
// Input
{"message": "apple,banana,orange"}

// Output
{"message": "apple,banana,orange", "message_parts": ["apple", "banana", "orange"]}

// If target is omitted, overwrites source field
{"split": {"field": "message", "separator": ","}}
// Output: {"message": ["apple", "banana", "orange"]}
```

### 2. Join

Join an array field into a string.

**Configuration:**
```json
{
  "join": {
    "field": "tags",
    "separator": "|",
    "target": "tags_string"
  }
}
```

**Example:**
```json
// Input
{"tags": ["error", "database", "timeout"]}

// Output
{"tags": ["error", "database", "timeout"], "tags_string": "error|database|timeout"}
```

### 3. Gsub (Regex Replace)

Perform regex find-and-replace operations on string fields.

**Configuration:**
```json
{
  "gsub": [
    {"field": "message", "pattern": "[0-9]", "replacement": "X"},
    {"field": "phone", "pattern": "\\d{3}-\\d{3}-(\\d{4})", "replacement": "XXX-XXX-$1"}
  ]
}
```

**Example:**
```json
// Input
{"message": "Error code 500 occurred", "phone": "555-123-4567"}

// Output
{"message": "Error code XXX occurred", "phone": "XXX-XXX-4567"}
```

### 4. Convert (Type Conversion)

Convert field types between string, integer, float, and boolean.

**Configuration:**
```json
{
  "convert": {
    "status": "integer",
    "duration": "float",
    "enabled": "boolean",
    "user_id": "string"
  }
}
```

**Supported Types:**
- `string` - Convert to string
- `integer` or `int` - Convert to integer
- `float` - Convert to float64
- `boolean` or `bool` - Convert to boolean

**Example:**
```json
// Input
{"status": "200", "duration": "1.5", "enabled": "true"}

// Output
{"status": 200, "duration": 1.5, "enabled": true}
```

### 5. Lowercase

Convert string fields to lowercase.

**Configuration:**
```json
{
  "lowercase": ["name", "email", "city"]
}
```

**Example:**
```json
// Input
{"name": "John DOE", "email": "JOHN@EXAMPLE.COM"}

// Output
{"name": "john doe", "email": "john@example.com"}
```

### 6. Uppercase

Convert string fields to uppercase.

**Configuration:**
```json
{
  "uppercase": ["country_code", "state"]
}
```

**Example:**
```json
// Input
{"country_code": "us", "state": "ca"}

// Output
{"country_code": "US", "state": "CA"}
```

### 7. Strip

Trim whitespace from string fields.

**Configuration:**
```json
{
  "strip": ["message", "user", "description"]
}
```

**Example:**
```json
// Input
{"message": "  Error occurred  ", "user": "\tjohn\n"}

// Output
{"message": "Error occurred", "user": "john"}
```

### 8. Merge

Merge two fields together intelligently based on their types.

**Configuration:**
```json
{
  "merge": [
    {"source": "tags", "target": "all_tags"},
    {"source": "extra_metadata", "target": "metadata"}
  ]
}
```

**Merge Behavior:**
- **Arrays**: Concatenated together
- **Maps/Objects**: Keys merged (source overwrites target on conflicts)
- **Strings**: Concatenated together
- **Other types**: Source replaces target

**Example:**
```json
// Input
{"tags": ["error"], "all_tags": ["info", "web"]}

// Output
{"tags": ["error"], "all_tags": ["info", "web", "error"]}

// Map merge example
// Input
{"metadata": {"env": "prod"}, "extra": {"region": "us-east-1"}}

// After merge: {"source": "extra", "target": "metadata"}
// Output
{"metadata": {"env": "prod", "region": "us-east-1"}, "extra": {"region": "us-east-1"}}
```

### 9. Update

Update field value only if the field already exists.

**Configuration:**
```json
{
  "update": {
    "status": "processed",
    "stage": "completed"
  }
}
```

**Example:**
```json
// Input
{"status": "pending", "new_field": "value"}

// Output (status exists, so updated; stage doesn't exist, so not added)
{"status": "processed", "new_field": "value"}
```

### 10. Replace

Set field value regardless of whether it exists.

**Configuration:**
```json
{
  "replace": {
    "version": "2.0",
    "processor": "bytefreezer-piper"
  }
}
```

**Example:**
```json
// Input
{"version": "1.0"}

// Output (version replaced, processor added)
{"version": "2.0", "processor": "bytefreezer-piper"}
```

### 11. Rename

Rename fields.

**Configuration:**
```json
{
  "rename": {
    "old_field": "new_field",
    "temp_name": "permanent_name"
  }
}
```

**Example:**
```json
// Input
{"old_field": "value", "other": "data"}

// Output
{"new_field": "value", "other": "data"}
```

### 12. Copy

Copy field values to new fields.

**Configuration:**
```json
{
  "copy": {
    "message": "message_backup",
    "timestamp": "original_timestamp"
  }
}
```

**Example:**
```json
// Input
{"message": "Error", "timestamp": "2024-10-30T14:00:00Z"}

// Output
{
  "message": "Error",
  "message_backup": "Error",
  "timestamp": "2024-10-30T14:00:00Z",
  "original_timestamp": "2024-10-30T14:00:00Z"
}
```

### 13. Remove

Remove fields from the record.

**Configuration:**
```json
{
  "remove": ["temp_field", "debug_info", "internal_id"]
}
```

**Example:**
```json
// Input
{"message": "Error", "temp_field": "xyz", "debug_info": "..."}

// Output
{"message": "Error"}
```

## Operation Order

Operations are applied in this order:

1. **Copy** - Duplicate fields first
2. **Rename** - Rename fields
3. **Convert** - Type conversions
4. **Split** - Split strings to arrays
5. **Join** - Join arrays to strings
6. **Gsub** - Regex replacements
7. **Lowercase** - Lowercase transformations
8. **Uppercase** - Uppercase transformations
9. **Strip** - Trim whitespace
10. **Merge** - Merge fields
11. **Update** - Update existing fields
12. **Replace** - Replace/add fields
13. **Remove** - Remove fields (last)

## Complete Examples

### Example 1: Clean and Parse User Input

**Input:**
```json
{
  "user_input": "  John Doe  ",
  "email": "  JOHN@EXAMPLE.COM  ",
  "tags": "web,api,user",
  "status": "200"
}
```

**Filter:**
```json
{
  "type": "mutate",
  "config": {
    "strip": ["user_input", "email"],
    "lowercase": ["email"],
    "split": {"field": "tags", "separator": ","},
    "convert": {"status": "integer"}
  }
}
```

**Output:**
```json
{
  "user_input": "John Doe",
  "email": "john@example.com",
  "tags": ["web", "api", "user"],
  "status": 200
}
```

### Example 2: Sanitize Sensitive Data

**Input:**
```json
{
  "message": "User 12345 logged in from IP 192.168.1.100",
  "phone": "555-123-4567",
  "ssn": "123-45-6789"
}
```

**Filter:**
```json
{
  "type": "mutate",
  "config": {
    "gsub": [
      {"field": "message", "pattern": "\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}", "replacement": "XXX.XXX.XXX.XXX"},
      {"field": "phone", "pattern": "\\d{3}-(\\d{3})-(\\d{4})", "replacement": "XXX-$1-$2"},
      {"field": "ssn", "pattern": "\\d{3}-\\d{2}-(\\d{4})", "replacement": "XXX-XX-$1"}
    ]
  }
}
```

**Output:**
```json
{
  "message": "User 12345 logged in from IP XXX.XXX.XXX.XXX",
  "phone": "XXX-123-4567",
  "ssn": "XXX-XX-6789"
}
```

### Example 3: Restructure Log Fields

**Input:**
```json
{
  "log_level": "error",
  "log_msg": "Database connection failed",
  "log_timestamp": "2024-10-30T14:00:00Z",
  "temp_debug": "internal_data"
}
```

**Filter:**
```json
{
  "type": "mutate",
  "config": {
    "rename": {
      "log_level": "level",
      "log_msg": "message",
      "log_timestamp": "timestamp"
    },
    "uppercase": ["level"],
    "remove": ["temp_debug"],
    "replace": {
      "processor": "bytefreezer-piper",
      "version": "1.0"
    }
  }
}
```

**Output:**
```json
{
  "level": "ERROR",
  "message": "Database connection failed",
  "timestamp": "2024-10-30T14:00:00Z",
  "processor": "bytefreezer-piper",
  "version": "1.0"
}
```

### Example 4: Combine Multiple Tags

**Input:**
```json
{
  "system_tags": ["linux", "production"],
  "user_tags": ["web", "api"],
  "tags": ["monitoring"]
}
```

**Filter:**
```json
{
  "type": "mutate",
  "config": {
    "merge": [
      {"source": "system_tags", "target": "tags"},
      {"source": "user_tags", "target": "tags"}
    ],
    "remove": ["system_tags", "user_tags"]
  }
}
```

**Output:**
```json
{
  "tags": ["monitoring", "linux", "production", "web", "api"]
}
```

## Pipeline Configuration Example

Complete pipeline with mutate filter:

```json
{
  "tenant_id": "tenant-001",
  "dataset_id": "dataset-001",
  "filters": [
    {
      "type": "grok",
      "enabled": true,
      "config": {
        "source_field": "message",
        "pattern": "%{IP:client} - %{USER:user} \\[%{HTTPDATE:timestamp}\\] \"%{WORD:method} %{URIPATHPARAM:request}\" %{NUMBER:status} %{NUMBER:bytes}"
      }
    },
    {
      "type": "mutate",
      "enabled": true,
      "config": {
        "convert": {
          "status": "integer",
          "bytes": "integer"
        },
        "uppercase": ["method"],
        "strip": ["user"],
        "remove": ["ident"]
      }
    },
    {
      "type": "date_parse",
      "enabled": true,
      "config": {
        "source_field": "timestamp",
        "target_field": "@timestamp",
        "formats": ["02/Jan/2006:15:04:05 -0700"]
      }
    }
  ]
}
```

## Performance Considerations

1. **Operation Order**: Operations are applied in a specific order. Place expensive operations (like gsub) after operations that might skip the record.

2. **Regex Patterns**: Complex regex patterns in gsub can be slow. Pre-compile patterns when possible (done automatically).

3. **Multiple Mutate Filters**: Instead of one large mutate filter, consider splitting into multiple smaller filters if you need conditional application.

4. **Type Conversion**: Converting between types may fail silently on invalid data. Check logs for conversion errors.

## Common Use Cases

### Data Cleansing
```json
{
  "strip": ["*"],
  "lowercase": ["email", "username"],
  "uppercase": ["country_code", "state"],
  "remove": ["internal_*", "debug_*"]
}
```

### Field Standardization
```json
{
  "rename": {
    "user": "username",
    "ip": "client_ip",
    "ts": "timestamp"
  },
  "convert": {
    "status_code": "integer",
    "response_time_ms": "float"
  }
}
```

### Privacy Protection
```json
{
  "gsub": [
    {"field": "email", "pattern": "(.{2})[^@]+", "replacement": "$1***"},
    {"field": "ip", "pattern": "\\d+$", "replacement": "XXX"},
    {"field": "ssn", "pattern": "\\d{3}-\\d{2}-(\\d{4})", "replacement": "XXX-XX-$1"}
  ]
}
```

## Troubleshooting

### Type Conversion Fails

If type conversion isn't working:
- Check input value format (e.g., "true" for boolean, "123" for integer)
- Check logs for conversion error messages
- Verify field names are correct

### Gsub Pattern Not Matching

If regex replacement isn't working:
- Test your regex pattern separately
- Remember to escape special characters in JSON (`\` becomes `\\`)
- Use tools like regex101.com to test patterns

### Fields Not Being Removed

If remove isn't working:
- Verify field names are exactly correct (case-sensitive)
- Check if the field is being added by a later filter
- Remember remove happens last, so it will remove fields added by earlier operations in the same mutate filter

## See Also

- [Grok Filter](GROK_FILTER.md) - For parsing unstructured data
- [KV Filter](KV_FILTER.md) - For parsing key-value pairs
- [Conditional Filter](../pipeline/filters.go) - For conditional mutations
