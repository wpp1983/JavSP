#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
TEST_TYPE="all"
COVERAGE_THRESHOLD="80.0"
VERBOSE=false

# Function to show help
show_help() {
    echo "JavSP Go Test Runner"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -t, --type TYPE         Test type: unit, integration, benchmark, all (default: all)"
    echo "  -c, --coverage          Run with coverage analysis"
    echo "  --coverage-threshold N  Set coverage threshold percentage (default: 80.0)"
    echo "  -v, --verbose           Verbose output"
    echo "  -q, --quick             Quick tests (no race detection, shorter timeout)"
    echo "  --ci                    CI mode (optimized for continuous integration)"
    echo "  -h, --help              Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 --type unit --coverage"
    echo "  $0 --type integration --verbose"
    echo "  $0 --type benchmark"
    echo "  $0 --coverage --coverage-threshold 85"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--type)
            TEST_TYPE="$2"
            shift 2
            ;;
        -c|--coverage)
            COVERAGE=true
            shift
            ;;
        --coverage-threshold)
            COVERAGE_THRESHOLD="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -q|--quick)
            QUICK=true
            shift
            ;;
        --ci)
            CI_MODE=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

echo -e "${BLUE}JavSP Go Test Suite${NC}"
echo -e "${BLUE}===================${NC}"

# Set test flags based on mode
TEST_FLAGS="-v"
if [[ "$QUICK" != "true" && "$CI_MODE" != "true" ]]; then
    TEST_FLAGS="$TEST_FLAGS -race"
fi

# Run unit tests
run_unit_tests() {
    echo -e "${YELLOW}Running unit tests...${NC}"
    local timeout="30s"
    if [[ "$QUICK" == "true" ]]; then
        timeout="15s"
    fi
    
    if [[ "$COVERAGE" == "true" ]]; then
        go test $TEST_FLAGS -timeout=$timeout -tags=unit -coverprofile=coverage-unit.out -covermode=atomic ./...
    else
        go test $TEST_FLAGS -timeout=$timeout -tags=unit ./...
    fi
}

# Run integration tests
run_integration_tests() {
    echo -e "${YELLOW}Running integration tests...${NC}"
    local timeout="60s"
    if [[ "$QUICK" == "true" ]]; then
        timeout="30s"
    fi
    
    if [[ "$COVERAGE" == "true" ]]; then
        go test $TEST_FLAGS -timeout=$timeout -tags=integration -coverprofile=coverage-integration.out -covermode=atomic ./test/integration/...
    else
        go test $TEST_FLAGS -timeout=$timeout -tags=integration ./test/integration/...
    fi
}

# Run benchmark tests
run_benchmark_tests() {
    echo -e "${YELLOW}Running benchmark tests...${NC}"
    go test -run=^$$ -bench=. -benchmem -timeout=300s -tags=benchmark ./test/benchmark/...
}

# Generate coverage report
generate_coverage_report() {
    if [[ "$COVERAGE" == "true" ]]; then
        echo -e "${YELLOW}Generating coverage report...${NC}"
        
        # Combine coverage files if multiple exist
        if [[ -f "coverage-unit.out" && -f "coverage-integration.out" ]]; then
            echo "mode: atomic" > coverage.out
            grep -h -v "mode: atomic" coverage-unit.out coverage-integration.out >> coverage.out
        elif [[ -f "coverage-unit.out" ]]; then
            cp coverage-unit.out coverage.out
        elif [[ -f "coverage-integration.out" ]]; then
            cp coverage-integration.out coverage.out
        fi
        
        if [[ -f "coverage.out" ]]; then
            go tool cover -html=coverage.out -o coverage.html
            coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
            
            echo -e "${GREEN}✓ Coverage: $coverage${NC}"
            echo -e "${GREEN}✓ Coverage report saved to coverage.html${NC}"
            
            # Check coverage threshold
            coverage_num=$(echo $coverage | sed 's/%//')
            threshold_num=$(echo $COVERAGE_THRESHOLD | sed 's/%//')
            
            if command -v bc >/dev/null 2>&1; then
                if (( $(echo "$coverage_num >= $threshold_num" | bc -l) )); then
                    echo -e "${GREEN}✓ Coverage meets minimum threshold ($COVERAGE_THRESHOLD%)${NC}"
                else
                    echo -e "${RED}✗ Coverage below minimum threshold ($COVERAGE_THRESHOLD%)${NC}"
                    exit 1
                fi
            else
                echo -e "${YELLOW}⚠ bc command not found, skipping coverage threshold check${NC}"
            fi
        fi
    fi
}

# Main test execution
case "$TEST_TYPE" in
    unit)
        run_unit_tests
        ;;
    integration)
        run_integration_tests
        ;;
    benchmark)
        run_benchmark_tests
        ;;
    all)
        run_unit_tests
        if [[ "$CI_MODE" != "true" ]]; then
            run_integration_tests
        fi
        ;;
    *)
        echo -e "${RED}✗ Invalid test type: $TEST_TYPE${NC}"
        echo "Valid types: unit, integration, benchmark, all"
        exit 1
        ;;
esac

# Generate coverage report if requested
generate_coverage_report

echo ""
echo -e "${GREEN}✓ Test suite completed successfully!${NC}"

# Show summary
if [[ "$VERBOSE" == "true" ]]; then
    echo ""
    echo -e "${BLUE}Test Summary:${NC}"
    echo "  Test Type: $TEST_TYPE"
    echo "  Coverage: ${COVERAGE:-false}"
    if [[ "$COVERAGE" == "true" ]]; then
        echo "  Coverage Threshold: $COVERAGE_THRESHOLD%"
    fi
    echo "  Quick Mode: ${QUICK:-false}"
    echo "  CI Mode: ${CI_MODE:-false}"
    echo "  Race Detection: $([[ "$QUICK" != "true" && "$CI_MODE" != "true" ]] && echo "enabled" || echo "disabled")"
fi