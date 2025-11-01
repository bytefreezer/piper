# KV (Key-Value) Filter

The KV filter parses key-value pairs from string fields into structured data.

## Configuration

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

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `source_field` | string | `"message"` | Field containing key-value pairs |
| `field_split` | string | `" "` | Separator between key-value pairs |
| `value_split` | string | `"="` | Separator between key and value |
| `target_field` | string | optional | Store all pairs in nested object |
| `prefix` | string | `""` | Prefix for extracted field names |
| `include_keys` | array | [] | Only extract these keys |
| `exclude_keys` | array | [] | Skip these keys |
| `trim_key` | string | `""` | Characters to trim from keys |
| `trim_value` | string | `""` | Characters to trim from values |
| `allow_duplicate` | boolean | `true` | Allow duplicate keys |
| `default_values` | object | {} | Default values for missing keys |
| `remove_field` | boolean | `false` | Remove source field after parsing |

## Examples

### Example 1: Basic Parsing

**Input:**
```json
{
  "message": "user=john action=login status=success ip=192.168.1.1"
}
```

**Filter:**
```json
{
  "type": "kv",
  "config": {
    "source_field": "message"
  }
}
```

**Output:**
```json
{
  "message": "user=john action=login status=success ip=192.168.1.1",
  "user": "john",
  "action": "login",
  "status": "success",
  "ip": "192.168.1.1"
}
```

### Example 2: Custom Separators

**Input:**
```json
{
  "message": "user:john|action:login|status:success"
}
```

**Filter:**
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "field_split": "|",
    "value_split": ":"
  }
}
```

**Output:**
```json
{
  "message": "user:john|action:login|status:success",
  "user": "john",
  "action": "login",
  "status": "success"
}
```

### Example 3: Nested Target Field

**Input:**
```json
{
  "message": "env=prod region=us-east-1 az=us-east-1a"
}
```

**Filter:**
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "target_field": "metadata"
  }
}
```

**Output:**
```json
{
  "message": "env=prod region=us-east-1 az=us-east-1a",
  "metadata": {
    "env": "prod",
    "region": "us-east-1",
    "az": "us-east-1a"
  }
}
```

### Example 4: Field Prefix

**Input:**
```json
{
  "message": "cpu=45 memory=78 disk=92"
}
```

**Filter:**
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "prefix": "metric_"
  }
}
```

**Output:**
```json
{
  "message": "cpu=45 memory=78 disk=92",
  "metric_cpu": "45",
  "metric_memory": "78",
  "metric_disk": "92"
}
```

### Example 5: Include/Exclude Keys

**Input:**
```json
{
  "message": "user=john password=secret action=login ip=192.168.1.1"
}
```

**Filter:**
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "exclude_keys": ["password"]
  }
}
```

**Output:**
```json
{
  "message": "user=john password=secret action=login ip=192.168.1.1",
  "user": "john",
  "action": "login",
  "ip": "192.168.1.1"
}
```

### Example 6: Quote Trimming

**Input:**
```json
{
  "message": "user=\"john doe\" action=\"user login\" status=\"success\""
}
```

**Filter:**
```json
{
  "type": "kv",
  "config": {
    "source_field": "message",
    "trim_value": "\""
  }
}
```

**Output:**
```json
{
  "message": "user=\"john doe\" action=\"user login\" status=\"success\"",
  "user": "john doe",
  "action": "user login",
  "status": "success"
}
```

## See Also

- [Grok Filter](GROK_FILTER.md) - For pattern-based parsing
- [Mutate Filter](MUTATE_FILTER.md) - For field transformations
