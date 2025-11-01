# Drop Filter

The Drop filter conditionally removes events from the pipeline based on field values, patterns, or sampling percentages.

## Configuration

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

## Options

| Option | Type | Description |
|--------|------|-------------|
| `if_field` | string | Field to check for drop condition |
| `equals` | any | Drop if field equals this value |
| `not_equals` | any | Drop if field does NOT equal this value |
| `contains` | string | Drop if field contains this substring |
| `matches` | string | Drop if field matches this regex pattern |
| `unless_field` | string | Field to check for keep condition (inverse) |
| `unless_equals` | any | DON'T drop if field equals this value |
| `unless_matches` | string | DON'T drop if field matches this regex |
| `percentage` | float | Drop only this percentage of matching events (0-100) |
| `always_drop` | boolean | Always drop all events (for testing) |

## Examples

### Example 1: Drop Successful HTTP Requests

Drop all 200 OK responses:

```json
{
  "type": "drop",
  "config": {
    "if_field": "status",
    "equals": "200"
  }
}
```

### Example 2: Sample Events (Keep 10%)

Drop 90% of matching events, keep 10%:

```json
{
  "type": "drop",
  "config": {
    "if_field": "log_level",
    "equals": "DEBUG",
    "percentage": 90
  }
}
```

### Example 3: Drop by Pattern

Drop events containing sensitive data:

```json
{
  "type": "drop",
  "config": {
    "if_field": "message",
    "matches": "password|secret|token"
  }
}
```

### Example 4: Drop Unless Condition

Drop all events EXCEPT errors:

```json
{
  "type": "drop",
  "config": {
    "unless_field": "level",
    "unless_equals": "ERROR"
  }
}
```

### Example 5: Percentage-Only Sampling

Drop 50% of all events randomly:

```json
{
  "type": "drop",
  "config": {
    "percentage": 50
  }
}
```

## See Also

- [Conditional Filter](../pipeline/filters.go) - For conditional field operations
- [Mutate Filter](MUTATE_FILTER.md) - For field transformations
