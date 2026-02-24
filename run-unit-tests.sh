#!/bin/bash
# Unit test script for Konflux pipeline
# Runs Ginkgo unit tests only

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Set test environment variables
export TEST_BUCKET="${TEST_BUCKET:-test-bucket}"
export TEST_NAMESPACE="${TEST_NAMESPACE:-test-populate}"
export MINIO_ENDPOINT="${MINIO_ENDPOINT:-localhost:9000}"
export TEST_DIR="${TEST_DIR:-build}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_section() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

# Run Ginkgo unit tests
log_section "Running Ginkgo Unit Tests"
if go run github.com/onsi/ginkgo/v2/ginkgo -r --succinct; then
    log_info "✓ All unit tests passed!"
    exit 0
else
    log_error "✗ Unit tests failed"
    exit 1
fi
