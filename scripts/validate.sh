#!/bin/bash
# validate.sh - Run all pre-commit validation checks
#
# This script runs the same checks as CI. Run before committing to catch issues early.
#
# Usage:
#   ./scripts/validate.sh         # Run all checks
#   ./scripts/validate.sh --quick # Skip slow checks (benchmarks)
#   ./scripts/validate.sh --fix   # Auto-fix formatting issues

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Parse arguments
QUICK=false
FIX=false
for arg in "$@"; do
    case $arg in
        --quick)
            QUICK=true
            ;;
        --fix)
            FIX=true
            ;;
    esac
done

# Track failures
FAILED=0

echo_step() {
    echo -e "\n${YELLOW}=== $1 ===${NC}"
}

echo_pass() {
    echo -e "${GREEN}PASS${NC}: $1"
}

echo_fail() {
    echo -e "${RED}FAIL${NC}: $1"
    FAILED=1
}

# Step 1: Format check (or fix)
echo_step "Checking code formatting"
if [ "$FIX" = true ]; then
    if make format; then
        echo_pass "Code formatted"
    else
        echo_fail "make format failed"
    fi
else
    # Check if formatting would change anything
    UNFORMATTED=$(gofmt -s -l . 2>/dev/null | grep -v vendor | head -10)
    if [ -n "$UNFORMATTED" ]; then
        echo_fail "Files need formatting:"
        echo "$UNFORMATTED"
        echo "Run './scripts/validate.sh --fix' to auto-fix"
    else
        echo_pass "Code formatting"
    fi
fi

# Step 2: Run tests with race detection
echo_step "Running tests"
if go test -race ./... > /dev/null 2>&1; then
    echo_pass "All tests"
else
    echo_fail "Tests failed"
    echo "Run 'make test' for details"
fi

# Step 3: Run golangci-lint
echo_step "Running golangci-lint"
if command -v golangci-lint &> /dev/null; then
    if golangci-lint run ./... 2>/dev/null; then
        echo_pass "golangci-lint"
    else
        echo_fail "golangci-lint found issues"
        echo "Run 'golangci-lint run ./...' for details"
    fi
else
    echo -e "${YELLOW}SKIP${NC}: golangci-lint not installed"
    echo "Install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
fi

# Step 4: Run nilaway
echo_step "Running nilaway"
if command -v nilaway &> /dev/null; then
    if nilaway ./... 2>/dev/null; then
        echo_pass "nilaway"
    else
        echo_fail "nilaway found nil safety issues"
        echo "Run 'nilaway ./...' for details"
    fi
else
    echo -e "${YELLOW}SKIP${NC}: nilaway not installed"
    echo "Install: go install go.uber.org/nilaway/cmd/nilaway@latest"
fi

# Step 5: Check for common issues
echo_step "Checking for common issues"

# Check for TODO/FIXME that should be tracked
TODO_COUNT=$(grep -rE "TODO|FIXME" --include="*.go" . 2>/dev/null | grep -v vendor | grep -v "_test.go" | wc -l)
if [ "$TODO_COUNT" -gt 0 ]; then
    echo -e "${YELLOW}NOTE${NC}: Found $TODO_COUNT TODO/FIXME comments in non-test code"
fi

# Check for debug code left behind
if grep -rE "fmt\.Println|log\.Println" --include="*.go" . 2>/dev/null | grep -v vendor | grep -v "_test.go" | grep -v "// debug" > /dev/null; then
    echo -e "${YELLOW}NOTE${NC}: Found print statements in non-test code (may be intentional)"
fi

echo_pass "Common issues check complete"

# Step 6: Benchmarks (skip if --quick)
if [ "$QUICK" = false ]; then
    echo_step "Running quick benchmark sanity check"
    if go test -bench=BenchmarkMachine -benchtime=1s -run='^$' ./cek/ > /dev/null 2>&1; then
        echo_pass "Benchmark sanity check"
    else
        echo_fail "Benchmarks failed"
    fi
else
    echo_step "Skipping benchmarks (--quick mode)"
fi

# Summary
echo ""
echo "========================================"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All checks passed!${NC}"
    echo "Safe to commit."
    exit 0
else
    echo -e "${RED}Some checks failed.${NC}"
    echo "Please fix issues before committing."
    exit 1
fi
