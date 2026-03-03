#!/bin/bash
# Disable exit on error for the test script since we expect some commands to fail
set -o pipefail

# Test script for valpop populate command
# This script tests the complete populate workflow including:
# - MinIO setup
# - File population
# - Manifest creation
# - Cleanup functionality

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
TEST_BUCKET="${TEST_BUCKET:-test-bucket}"
TEST_NAMESPACE="${TEST_NAMESPACE:-test-populate}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-localhost:9000}"
MINIO_USER="${MINIO_USER:-minioadmin}"
MINIO_PASS="${MINIO_PASS:-minioadmin}"
TEST_DIR="${TEST_DIR:-build}"

# Track test results
TESTS_PASSED=0
TESTS_FAILED=0

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

assert_success() {
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        log_info "✓ $1"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "✗ $1"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_file_exists() {
    if [ -f "$1" ]; then
        log_info "✓ File exists: $1"
        ((TESTS_PASSED++))
    else
        log_error "✗ File does not exist: $1"
        ((TESTS_FAILED++))
        return 1
    fi
}

cleanup() {
    log_info "Cleaning up test environment..."
    if command -v podman-compose &> /dev/null; then
        podman-compose down -v 2>/dev/null || true
    elif command -v docker-compose &> /dev/null; then
        docker-compose down -v 2>/dev/null || true
    fi
}

# Cleanup on exit
trap cleanup EXIT

log_info "Starting valpop populate integration tests"
echo "=========================================="

# 1. Check prerequisites
log_info "Checking prerequisites..."
if ! command -v go &> /dev/null; then
    log_error "Go is not installed"
    exit 1
fi
assert_success "Go is installed"

if command -v podman-compose &> /dev/null; then
    COMPOSE_CMD="podman-compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    log_error "Neither podman-compose nor docker-compose is installed"
    exit 1
fi
assert_success "Container compose command available: $COMPOSE_CMD"

# 2. Build valpop binary
log_info "Building valpop binary..."
go build -o valpop
assert_success "Built valpop binary"

assert_file_exists "valpop"

# 3. Create test data if it doesn't exist
if [ ! -d "$TEST_DIR" ]; then
    log_info "Creating test data directory..."
    mkdir -p "$TEST_DIR/js"
    echo "<html><body>Test Page</body></html>" > "$TEST_DIR/index.html"
    echo "console.log('test');" > "$TEST_DIR/js/app.js"
    echo "body { margin: 0; }" > "$TEST_DIR/js/style.css"
    assert_success "Created test data directory"
fi

# 4. Start MinIO
log_info "Starting MinIO..."
MINIO_ACCESS_KEY="$MINIO_USER" MINIO_SECRET_KEY="$MINIO_PASS" $COMPOSE_CMD up -d minio minio-createbuckets
assert_success "Started MinIO container"

# Wait for MinIO to be ready
log_info "Waiting for MinIO to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0
until curl -sf "http://${MINIO_ENDPOINT}/minio/health/live" > /dev/null 2>&1; do
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        log_error "MinIO failed to start after $MAX_RETRIES attempts"
        exit 1
    fi
    sleep 1
    ((RETRY_COUNT++))
done
assert_success "MinIO is ready"

# Wait for bucket creation
log_info "Waiting for bucket creation..."
sleep 3  # Give minio-createbuckets time to create the bucket

# 5. Test invalid endpoint (should return error, not panic)
log_info "Testing error handling with invalid endpoint..."
OUTPUT=$(./valpop populate -s "$TEST_DIR" -r "$TEST_NAMESPACE" -u "$MINIO_USER" -c "$MINIO_PASS" -b frontend -a "localhost:9000:6379" -p "" -t 3600 -n 3 2>&1 || true)
if echo "$OUTPUT" | grep -q "failed to create S3 client"; then
    assert_success "Error handling works correctly (no panic)"
else
    log_error "Expected error message not found"
    echo "$OUTPUT"
    ((TESTS_FAILED++))
fi

# 6. Test populate command
log_info "Running populate command..."
if ./valpop populate -s "$TEST_DIR" -r "$TEST_NAMESPACE" -u "$MINIO_USER" -c "$MINIO_PASS" -b frontend -a "localhost" -p 9000 -t 3600 -n 3 > /tmp/populate-output.txt 2>&1; then
    assert_success "Populate command executed successfully"
else
    log_error "Populate command failed with exit code $?"
    cat /tmp/populate-output.txt
    ((TESTS_FAILED++))
fi

# 7. Verify files were uploaded
log_info "Verifying files were uploaded..."
if grep -q "Uploading: index.html" /tmp/populate-output.txt; then
    assert_success "index.html was uploaded"
else
    log_error "index.html upload not found in output"
    ((TESTS_FAILED++))
fi

if grep -qE "(manifest|Manifest)" /tmp/populate-output.txt; then
    assert_success "Manifest was created"
else
    log_error "Manifest creation not found in output"
    cat /tmp/populate-output.txt
    ((TESTS_FAILED++))
fi

# 8. Test multiple deployments (should create multiple manifests)
log_info "Testing multiple deployments..."
sleep 2  # Ensure different timestamp
./valpop populate -s "$TEST_DIR" -r "$TEST_NAMESPACE" -u "$MINIO_USER" -c "$MINIO_PASS" -b frontend -a "localhost" -p 9000 -t 3600 -n 2 > /tmp/populate-output2.txt 2>&1
assert_success "Second populate executed successfully"

# 9. Test with missing source directory
log_info "Testing with missing source directory..."
OUTPUT=$(./valpop populate -s "nonexistent" -r "$TEST_NAMESPACE" -u "$MINIO_USER" -c "$MINIO_PASS" -b frontend -a "localhost" -p 9000 -t 3600 -n 3 2>&1 || true)
if echo "$OUTPUT" | grep -qE "(no such file|does not exist)"; then
    assert_success "Handles missing source directory correctly"
else
    log_error "Expected error for missing directory not found"
    ((TESTS_FAILED++))
fi

# 10. Test with missing required flags
log_info "Testing with missing required flags..."
OUTPUT=$(./valpop populate -s "$TEST_DIR" -u "$MINIO_USER" -c "$MINIO_PASS" 2>&1 || true)
if echo "$OUTPUT" | grep -q "no prefix arg set"; then
    assert_success "Validates required prefix flag"
else
    log_error "Expected error for missing prefix not found"
    ((TESTS_FAILED++))
fi

OUTPUT=$(./valpop populate -r "$TEST_NAMESPACE" -u "$MINIO_USER" -c "$MINIO_PASS" 2>&1 || true)
if echo "$OUTPUT" | grep -q "no source arg set"; then
    assert_success "Validates required source flag"
else
    log_error "Expected error for missing source not found"
    ((TESTS_FAILED++))
fi

# 11. Test with invalid credentials
log_info "Testing with invalid credentials..."
OUTPUT=$(./valpop populate -s "$TEST_DIR" -r "$TEST_NAMESPACE" -u "baduser" -c "badpass" -b frontend -a "localhost" -p 9000 -t 3600 -n 3 2>&1 || true)
if echo "$OUTPUT" | grep -qE "(Access Key|credentials|authentication)"; then
    assert_success "Handles invalid credentials correctly"
else
    log_error "Expected authentication error not found"
    ((TESTS_FAILED++))
fi

# 12. Test with negative min-asset-records
log_info "Testing with invalid min-asset-records..."
OUTPUT=$(./valpop populate -s "$TEST_DIR" -r "$TEST_NAMESPACE" -u "$MINIO_USER" -c "$MINIO_PASS" -b frontend -a "localhost" -p 9000 -n -1 2>&1 || true)
if echo "$OUTPUT" | grep -q "min-asset-records must be a non-negative integer"; then
    assert_success "Validates min-asset-records parameter"
else
    log_error "Expected validation error for min-asset-records not found"
    ((TESTS_FAILED++))
fi

# Print summary
echo ""
echo "=========================================="
log_info "Test Summary:"
echo "  Passed: $TESTS_PASSED"
echo "  Failed: $TESTS_FAILED"
echo "=========================================="

# Cleanup output files
rm -f /tmp/populate-output.txt /tmp/populate-output2.txt

if [ $TESTS_FAILED -gt 0 ]; then
    log_error "Some tests failed!"
    exit 1
else
    log_info "All tests passed!"
    exit 0
fi
