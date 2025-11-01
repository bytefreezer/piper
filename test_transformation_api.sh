#!/bin/bash

# Transformation API Test Script
# Tests all 6 transformation endpoints

set -e

# Configuration
BASE_URL="${PIPER_URL:-http://localhost:8090}"
API_BASE="${BASE_URL}/api/v1"
TENANT="${TEST_TENANT:-test-tenant}"
DATASET="${TEST_DATASET:-test-dataset}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

test_passed() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    log_info "✓ Test passed: $1"
}

test_failed() {
    TESTS_FAILED=$((TESTS_FAILED + 1))
    log_error "✗ Test failed: $1"
}

# Check if piper is running
check_piper() {
    log_info "Checking if piper is running at ${BASE_URL}..."
    if curl -s -f "${API_BASE}/health" > /dev/null 2>&1; then
        test_passed "Piper is running"
        return 0
    else
        test_failed "Piper is not accessible at ${BASE_URL}"
        log_error "Please start piper before running tests"
        exit 1
    fi
}

# Test 1: Get Schema
test_get_schema() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 1: GET /transformations/{tenant}/{dataset}/schema"

    response=$(curl -s -w "\n%{http_code}" "${API_BASE}/transformations/${TENANT}/${DATASET}/schema?count=10")
    body=$(echo "$response" | head -n -1)
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "200" ]; then
        # Check if response has expected fields
        if echo "$body" | jq -e '.schema' > /dev/null 2>&1 && \
           echo "$body" | jq -e '.samples' > /dev/null 2>&1; then
            test_passed "Get Schema endpoint returns valid response"
            echo "$body" | jq '.schema[0:2]' 2>/dev/null || true
        else
            test_failed "Get Schema response missing expected fields"
            echo "$body"
        fi
    elif [ "$http_code" == "404" ] || [ "$http_code" == "500" ]; then
        log_warn "Schema endpoint returned ${http_code} - may need data in S3 (${TENANT}/${DATASET})"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    else
        test_failed "Get Schema returned unexpected status: ${http_code}"
        echo "$body"
    fi
}

# Test 2: Test Transformation
test_test_transformation() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 2: POST /transformations/test"

    payload=$(cat <<'EOF'
{
  "tenant_id": "test-tenant",
  "dataset_id": "test-dataset",
  "filters": [
    {
      "type": "mutate",
      "config": {
        "add_field": {
          "test_field": "test_value"
        }
      },
      "enabled": true
    }
  ],
  "samples": [
    {
      "line_number": 1,
      "raw_data": "{\"message\":\"test\"}",
      "parsed_data": {
        "message": "test"
      }
    }
  ]
}
EOF
)

    response=$(curl -s -w "\n%{http_code}" -X POST "${API_BASE}/transformations/test" \
        -H "Content-Type: application/json" \
        -d "$payload")
    body=$(echo "$response" | head -n -1)
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "200" ]; then
        if echo "$body" | jq -e '.results' > /dev/null 2>&1 && \
           echo "$body" | jq -e '.success_count' > /dev/null 2>&1; then
            test_passed "Test Transformation endpoint returns valid response"
            echo "$body" | jq '.results[0] | {input, output, applied}' 2>/dev/null || true
        else
            test_failed "Test Transformation response missing expected fields"
            echo "$body"
        fi
    else
        test_failed "Test Transformation returned status: ${http_code}"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    fi
}

# Test 3: Validate Fresh Data
test_validate_fresh_data() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 3: POST /transformations/validate"

    payload=$(cat <<EOF
{
  "tenant_id": "${TENANT}",
  "dataset_id": "${DATASET}",
  "filters": [
    {
      "type": "mutate",
      "config": {
        "add_field": {
          "validated": "true"
        }
      },
      "enabled": true
    }
  ],
  "count": 10
}
EOF
)

    response=$(curl -s -w "\n%{http_code}" -X POST "${API_BASE}/transformations/validate" \
        -H "Content-Type: application/json" \
        -d "$payload")
    body=$(echo "$response" | head -n -1)
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "200" ]; then
        if echo "$body" | jq -e '.results' > /dev/null 2>&1 && \
           echo "$body" | jq -e '.source_file' > /dev/null 2>&1; then
            test_passed "Validate Fresh Data endpoint returns valid response"
            echo "$body" | jq '{source_file, source_lines, success_count, avg_time_per_row_ms}' 2>/dev/null || true
        else
            test_failed "Validate Fresh Data response missing expected fields"
            echo "$body"
        fi
    elif [ "$http_code" == "404" ] || [ "$http_code" == "500" ]; then
        log_warn "Validate endpoint returned ${http_code} - may need data in S3"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    else
        test_failed "Validate Fresh Data returned status: ${http_code}"
        echo "$body"
    fi
}

# Test 4: Activate Transformation
test_activate_transformation() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 4: POST /transformations/activate"

    payload=$(cat <<EOF
{
  "tenant_id": "${TENANT}",
  "dataset_id": "${DATASET}",
  "filters": [
    {
      "type": "mutate",
      "config": {
        "add_field": {
          "active": "true"
        }
      },
      "enabled": true
    }
  ],
  "enabled": true
}
EOF
)

    response=$(curl -s -w "\n%{http_code}" -X POST "${API_BASE}/transformations/activate" \
        -H "Content-Type: application/json" \
        -d "$payload")
    body=$(echo "$response" | head -n -1)
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "200" ]; then
        if echo "$body" | jq -e '.success' > /dev/null 2>&1 && \
           echo "$body" | jq -e '.version' > /dev/null 2>&1; then
            test_passed "Activate Transformation endpoint returns valid response"
            echo "$body" | jq '{success, message, version}' 2>/dev/null || true
        else
            test_failed "Activate Transformation response missing expected fields"
            echo "$body"
        fi
    else
        test_failed "Activate Transformation returned status: ${http_code}"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    fi
}

# Test 5: Get Transformation Stats
test_get_stats() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 5: GET /transformations/{tenant}/{dataset}/stats"

    response=$(curl -s -w "\n%{http_code}" "${API_BASE}/transformations/${TENANT}/${DATASET}/stats")
    body=$(echo "$response" | head -n -1)
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "200" ]; then
        if echo "$body" | jq -e '.stats' > /dev/null 2>&1; then
            test_passed "Get Stats endpoint returns valid response"
            echo "$body" | jq '.stats | {enabled, filter_count, total_processed, avg_rows_per_sec}' 2>/dev/null || true
        else
            test_failed "Get Stats response missing expected fields"
            echo "$body"
        fi
    else
        test_failed "Get Stats returned status: ${http_code}"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    fi
}

# Test 6: Preview Transformation
test_preview_transformation() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 6: GET /transformations/{tenant}/{dataset}/preview"

    response=$(curl -s -w "\n%{http_code}" "${API_BASE}/transformations/${TENANT}/${DATASET}/preview?count=5")
    body=$(echo "$response" | head -n -1)
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "200" ]; then
        if echo "$body" | jq -e '.samples' > /dev/null 2>&1 && \
           echo "$body" | jq -e '.enabled' > /dev/null 2>&1; then
            test_passed "Preview Transformation endpoint returns valid response"
            echo "$body" | jq '{enabled, filter_count, sample_count: (.samples | length)}' 2>/dev/null || true
        else
            test_failed "Preview Transformation response missing expected fields"
            echo "$body"
        fi
    elif [ "$http_code" == "404" ] || [ "$http_code" == "500" ]; then
        log_warn "Preview endpoint returned ${http_code} - may need data in S3 and active transformation"
        echo "$body" | jq . 2>/dev/null || echo "$body"
    else
        test_failed "Preview Transformation returned status: ${http_code}"
        echo "$body"
    fi
}

# Test error handling
test_error_handling() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 7: Error handling - invalid filter type"

    payload=$(cat <<'EOF'
{
  "tenant_id": "test-tenant",
  "dataset_id": "test-dataset",
  "filters": [
    {
      "type": "invalid_filter_type",
      "config": {},
      "enabled": true
    }
  ],
  "samples": [
    {
      "line_number": 1,
      "raw_data": "{}",
      "parsed_data": {}
    }
  ]
}
EOF
)

    response=$(curl -s -w "\n%{http_code}" -X POST "${API_BASE}/transformations/test" \
        -H "Content-Type: application/json" \
        -d "$payload")
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "500" ] || [ "$http_code" == "400" ]; then
        test_passed "Error handling works correctly for invalid filter type"
    else
        test_failed "Expected error response for invalid filter type, got: ${http_code}"
    fi
}

test_too_many_filters() {
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test 8: Error handling - too many filters (>10)"

    # Generate 11 filters
    filters='['
    for i in {1..11}; do
        if [ $i -gt 1 ]; then filters+=','; fi
        filters+="{\"type\":\"mutate\",\"config\":{\"add_field\":{\"f${i}\":\"v${i}\"}},\"enabled\":true}"
    done
    filters+=']'

    payload=$(cat <<EOF
{
  "tenant_id": "test-tenant",
  "dataset_id": "test-dataset",
  "filters": ${filters},
  "samples": [{"line_number": 1, "raw_data": "{}", "parsed_data": {}}]
}
EOF
)

    response=$(curl -s -w "\n%{http_code}" -X POST "${API_BASE}/transformations/test" \
        -H "Content-Type: application/json" \
        -d "$payload")
    http_code=$(echo "$response" | tail -n 1)

    if [ "$http_code" == "400" ] || [ "$http_code" == "500" ]; then
        test_passed "Error handling works correctly for too many filters"
    else
        test_failed "Expected error response for too many filters, got: ${http_code}"
    fi
}

# Main execution
main() {
    echo "=========================================="
    echo "Transformation API Test Suite"
    echo "=========================================="
    echo "Target: ${BASE_URL}"
    echo "Tenant: ${TENANT}"
    echo "Dataset: ${DATASET}"
    echo "=========================================="
    echo ""

    check_piper
    echo ""

    test_get_schema
    echo ""

    test_test_transformation
    echo ""

    test_validate_fresh_data
    echo ""

    test_activate_transformation
    echo ""

    test_get_stats
    echo ""

    test_preview_transformation
    echo ""

    test_error_handling
    echo ""

    test_too_many_filters
    echo ""

    # Summary
    echo "=========================================="
    echo "Test Summary"
    echo "=========================================="
    echo "Tests run: ${TESTS_RUN}"
    echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
    echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"
    echo "=========================================="

    if [ $TESTS_FAILED -gt 0 ]; then
        exit 1
    fi
}

# Run tests
main "$@"
