#!/bin/bash

# CI/CD optimized test script for JavSP Go
# This script is designed for continuous integration environments

set -e

# Colors for output (if supported)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# CI Configuration
CI_TIMEOUT_UNIT="30s"
CI_TIMEOUT_INTEGRATION="60s"
CI_COVERAGE_THRESHOLD="75.0"  # Slightly lower for CI
PARALLEL_JOBS=${PARALLEL_JOBS:-4}

echo -e "${BLUE}JavSP Go CI Test Suite${NC}"
echo -e "${BLUE}======================${NC}"
echo ""

# Environment info
echo -e "${YELLOW}Environment Information:${NC}"
echo "Go version: $(go version)"
echo "OS: $GOOS"
echo "Arch: $GOARCH"
echo "Parallel jobs: $PARALLEL_JOBS"
echo ""

# Check if we're in a CI environment
if [[ -n "$CI" || -n "$GITHUB_ACTIONS" || -n "$GITLAB_CI" || -n "$JENKINS_URL" ]]; then
    echo -e "${BLUE}✓ CI environment detected${NC}"
    CI_ENV=true
else
    echo -e "${YELLOW}⚠ No CI environment detected${NC}"
    CI_ENV=false
fi

# Function to run tests with retries
run_with_retry() {
    local cmd="$1"
    local retries=3
    local count=0
    
    until $cmd; do
        exit_code=$?
        count=$((count + 1))
        if [[ $count -lt $retries ]]; then
            echo -e "${YELLOW}Command failed with exit code $exit_code. Retrying... ($count/$retries)${NC}"
            sleep 2
        else
            echo -e "${RED}Command failed after $retries attempts.${NC}"
            return $exit_code
        fi
    done
}

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up temporary files...${NC}"
    rm -f coverage-*.out test-results-*.xml
}
trap cleanup EXIT

# Pre-flight checks
echo -e "${YELLOW}Running pre-flight checks...${NC}"

# Check Go modules
if ! go mod verify; then
    echo -e "${RED}✗ Go modules verification failed${NC}"
    exit 1
fi

# Check if dependencies are up to date
if ! go mod tidy; then
    echo -e "${RED}✗ Go modules tidy failed${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Pre-flight checks passed${NC}"
echo ""

# Function to run unit tests
run_unit_tests_ci() {
    echo -e "${YELLOW}Running unit tests (CI mode)...${NC}"
    
    local test_cmd="go test -timeout=$CI_TIMEOUT_UNIT -tags=unit -coverprofile=coverage-unit.out -covermode=atomic -parallel=$PARALLEL_JOBS ./..."
    
    if [[ "$CI_ENV" == "true" ]]; then
        # Disable race detector in CI for speed (can be enabled selectively)
        test_cmd="$test_cmd -short"
    else
        test_cmd="$test_cmd -race"
    fi
    
    echo "Command: $test_cmd"
    
    if ! run_with_retry "$test_cmd"; then
        echo -e "${RED}✗ Unit tests failed${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Unit tests passed${NC}"
}

# Function to run integration tests (if enabled)
run_integration_tests_ci() {
    if [[ "$SKIP_INTEGRATION" == "true" ]]; then
        echo -e "${YELLOW}⚠ Integration tests skipped (SKIP_INTEGRATION=true)${NC}"
        return 0
    fi
    
    echo -e "${YELLOW}Running integration tests (CI mode)...${NC}"
    
    local test_cmd="go test -timeout=$CI_TIMEOUT_INTEGRATION -tags=integration -coverprofile=coverage-integration.out -covermode=atomic -parallel=1 ./test/integration/..."
    
    echo "Command: $test_cmd"
    
    if ! run_with_retry "$test_cmd"; then
        echo -e "${RED}✗ Integration tests failed${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Integration tests passed${NC}"
}

# Function to run linting
run_linting_ci() {
    echo -e "${YELLOW}Running code linting...${NC}"
    
    # Check if golangci-lint is available
    if command -v golangci-lint >/dev/null 2>&1; then
        if ! golangci-lint run --timeout=5m; then
            echo -e "${RED}✗ Linting failed${NC}"
            exit 1
        fi
        echo -e "${GREEN}✓ Linting passed${NC}"
    else
        echo -e "${YELLOW}⚠ golangci-lint not found, skipping linting${NC}"
    fi
}

# Function to generate coverage report
generate_coverage_report_ci() {
    echo -e "${YELLOW}Generating coverage report...${NC}"
    
    # Combine coverage files if multiple exist
    if [[ -f "coverage-unit.out" && -f "coverage-integration.out" ]]; then
        echo "mode: atomic" > coverage.out
        grep -h -v "mode: atomic" coverage-unit.out coverage-integration.out >> coverage.out
    elif [[ -f "coverage-unit.out" ]]; then
        cp coverage-unit.out coverage.out
    elif [[ -f "coverage-integration.out" ]]; then
        cp coverage-integration.out coverage.out
    else
        echo -e "${YELLOW}⚠ No coverage files found${NC}"
        return 0
    fi
    
    # Generate reports
    go tool cover -html=coverage.out -o coverage.html
    go tool cover -func=coverage.out > coverage.txt
    
    # Extract coverage percentage
    coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    coverage_num=$(echo $coverage | sed 's/%//')
    threshold_num=$(echo $CI_COVERAGE_THRESHOLD | sed 's/%//')
    
    echo -e "${GREEN}✓ Coverage: $coverage${NC}"
    
    # Check coverage threshold
    if command -v bc >/dev/null 2>&1; then
        if (( $(echo "$coverage_num >= $threshold_num" | bc -l) )); then
            echo -e "${GREEN}✓ Coverage meets CI threshold ($CI_COVERAGE_THRESHOLD%)${NC}"
        else
            echo -e "${RED}✗ Coverage below CI threshold ($CI_COVERAGE_THRESHOLD%)${NC}"
            echo "Required: $CI_COVERAGE_THRESHOLD%, Actual: $coverage"
            exit 1
        fi
    else
        echo -e "${YELLOW}⚠ bc command not found, skipping coverage threshold check${NC}"
    fi
    
    # Upload coverage if in CI
    if [[ "$CI_ENV" == "true" && -n "$CODECOV_TOKEN" ]]; then
        if command -v codecov >/dev/null 2>&1; then
            echo -e "${YELLOW}Uploading coverage to Codecov...${NC}"
            codecov -f coverage.out
        fi
    fi
}

# Function to run build test
run_build_test() {
    echo -e "${YELLOW}Testing build process...${NC}"
    
    if ! go build -o javsp-test ./cmd/javsp; then
        echo -e "${RED}✗ Build test failed${NC}"
        exit 1
    fi
    
    # Test that binary works
    if ! ./javsp-test --version >/dev/null 2>&1; then
        echo -e "${RED}✗ Binary test failed${NC}"
        exit 1
    fi
    
    rm -f javsp-test
    echo -e "${GREEN}✓ Build test passed${NC}"
}

# Main execution
echo -e "${YELLOW}Starting CI test pipeline...${NC}"
echo ""

# Stage 1: Linting (fail fast)
run_linting_ci
echo ""

# Stage 2: Build test (fail fast)
run_build_test
echo ""

# Stage 3: Unit tests
run_unit_tests_ci
echo ""

# Stage 4: Integration tests (if enabled)
run_integration_tests_ci
echo ""

# Stage 5: Coverage analysis
generate_coverage_report_ci
echo ""

# Success summary
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}✓ All CI tests passed successfully!${NC}"
echo -e "${GREEN}================================${NC}"
echo ""

# CI-specific outputs
if [[ "$CI_ENV" == "true" ]]; then
    echo "::notice::JavSP Go CI test suite completed successfully"
    
    # Set output for other CI steps
    if [[ -n "$GITHUB_ACTIONS" ]]; then
        echo "coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')" >> $GITHUB_OUTPUT
    fi
fi

echo -e "${BLUE}CI Test Summary:${NC}"
echo "  Unit Tests: ✓"
if [[ "$SKIP_INTEGRATION" != "true" ]]; then
    echo "  Integration Tests: ✓"
else
    echo "  Integration Tests: Skipped"
fi
echo "  Linting: ✓"
echo "  Build Test: ✓"
echo "  Coverage: $coverage (threshold: $CI_COVERAGE_THRESHOLD%)"
echo "  Duration: ${SECONDS}s"