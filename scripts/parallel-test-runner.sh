#!/bin/bash
# parallel-test-runner.sh
# Simple parallel test runner with environment isolation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    local module="$1"
    local message="$2"
    echo -e "${CYAN}[TEST:$module]${NC} $message"
}

# Configuration
PARALLEL_JOBS=${PARALLEL_JOBS:-$(nproc || echo 4)}
TEST_TIMEOUT=${TEST_TIMEOUT:-"10m"}
COVERAGE_THRESHOLD=${COVERAGE_THRESHOLD:-80}
RACE_DETECTION=${RACE_DETECTION:-"true"}
VERBOSE=${VERBOSE:-"false"}
TEST_MODE=${TEST_MODE:-"parallel"}

# Test categories
UNIT_TEST_PATTERN="./..."
INTEGRATION_TEST_PATTERN="./test/integration/..."
E2E_TEST_PATTERN="./test/e2e/..."

# Test modules for parallel execution
TEST_MODULES=(
    "api/internal/modules/users"
    "api/internal/modules/auth"
    "api/internal/modules/payment"
    "api/internal/modules/notification"
    "api/internal/modules/compliance"
    "api/internal/modules/data"
    "api/internal/shared"
    "pkg"
)

# Test directories
COVERAGE_DIR="${PROJECT_ROOT}/coverage"
REPORTS_DIR="${PROJECT_ROOT}/test-reports"
CACHE_DIR="${PROJECT_ROOT}/.test-cache"

# Setup test environment
setup_test_env() {
    log_info "Setting up parallel test environment..."

    # Create directories
    mkdir -p "$COVERAGE_DIR" "$REPORTS_DIR" "$CACHE_DIR"

    # Set Go test environment
    export GOCACHE="$CACHE_DIR/go-test"
    export GOMODCACHE="$CACHE_DIR/go-mod"
    export GO_TEST_TIMEOUT="$TEST_TIMEOUT"

    # Test isolation environment variables
    export TEST_PARALLEL="true"
    export TEST_ISOLATION="true"
    export TEST_ENV="ci"

    mkdir -p "$GOCACHE" "$GOMODCACHE"

    log_success "Test environment configured"
    log_info "Parallel jobs: $PARALLEL_JOBS"
    log_info "Test timeout: $TEST_TIMEOUT"
    log_info "Coverage threshold: $COVERAGE_THRESHOLD%"
}

# Discover test packages
discover_test_packages() {
    log_info "Discovering test packages..."

    local packages=()

    for module in "${TEST_MODULES[@]}"; do
        if [ -d "$PROJECT_ROOT/$module" ]; then
            # Find packages with tests
            local module_packages=$(find "$PROJECT_ROOT/$module" -name "*_test.go" -type f | \
                                  sed "s|$PROJECT_ROOT/||g" | \
                                  sed 's|/[^/]*$||g' | \
                                  sort -u)

            while IFS= read -r pkg; do
                if [ -n "$pkg" ]; then
                    packages+=("$pkg")
                fi
            done <<< "$module_packages"
        fi
    done

    # Output discovered packages
    echo "${packages[@]}"
}

# Run parallel unit tests
run_parallel_unit_tests() {
    log_info "Running parallel unit tests..."

    local packages=($(discover_test_packages))
    local pids=()
    local test_results=()
    local start_time=$(date +%s%N)

    if [ ${#packages[@]} -eq 0 ]; then
        log_warning "No test packages found"
        return 0
    fi

    log_info "Found ${#packages[@]} test packages"

    # Start parallel test execution
    for pkg in "${packages[@]}"; do
        log_test "$(basename "$pkg")" "Starting tests..."

        (
            local pkg_start=$(date +%s%N)
            local pkg_name=$(basename "$pkg")
            local output_file="$REPORTS_DIR/test-${pkg_name}-$(date +%s).json"
            local coverage_file="$COVERAGE_DIR/coverage-${pkg_name}.out"

            # Build test flags
            local test_flags="-json"
            [ "$RACE_DETECTION" = "true" ] && test_flags+=" -race"
            [ "$VERBOSE" = "true" ] && test_flags+=" -v"

            # Add coverage flags
            test_flags+=" -coverprofile=$coverage_file"
            test_flags+=" -covermode=atomic"

            # Run tests with timeout
            if timeout "$TEST_TIMEOUT" go test $test_flags "./$pkg" > "$output_file" 2>&1; then
                local pkg_end=$(date +%s%N)
                local duration=$(( (pkg_end - pkg_start) / 1000000 ))

                # Parse test results
                local test_count=$(grep -c '"Action":"pass"' "$output_file" 2>/dev/null || echo "0")
                local fail_count=$(grep -c '"Action":"fail"' "$output_file" 2>/dev/null || echo "0")

                echo "TEST_SUCCESS:$pkg_name:${duration}ms:$test_count:$fail_count" > "/tmp/test-result-$pkg_name"
                log_test "$pkg_name" "Completed ($test_count passed, $fail_count failed, ${duration}ms)"
            else
                echo "TEST_FAILED:$pkg_name" > "/tmp/test-result-$pkg_name"
                log_test "$pkg_name" "Failed or timed out"
            fi
        ) &

        pids+=($!)

        # Limit concurrent tests
        if [ ${#pids[@]} -ge "$PARALLEL_JOBS" ]; then
            wait ${pids[0]}
            pids=("${pids[@]:1}")
        fi
    done

    # Wait for all tests to complete
    for pid in "${pids[@]}"; do
        wait "$pid"
    done

    # Collect and analyze results
    local total_tests=0
    local total_failures=0
    local successful_packages=0

    for pkg in "${packages[@]}"; do
        local pkg_name=$(basename "$pkg")
        local result_file="/tmp/test-result-$pkg_name"

        if [ -f "$result_file" ]; then
            local result=$(cat "$result_file")
            local status=$(echo "$result" | cut -d':' -f1)

            if [ "$status" = "TEST_SUCCESS" ]; then
                local duration=$(echo "$result" | cut -d':' -f3)
                local passed=$(echo "$result" | cut -d':' -f4)
                local failed=$(echo "$result" | cut -d':' -f5)

                total_tests=$((total_tests + passed + failed))
                total_failures=$((total_failures + failed))
                successful_packages=$((successful_packages + 1))

                if [ "$failed" -gt 0 ]; then
                    log_warning "$pkg_name: $passed passed, $failed failed ($duration)"
                else
                    log_success "$pkg_name: $passed passed ($duration)"
                fi
            else
                log_error "$pkg_name: Package tests failed"
            fi

            rm -f "$result_file"
        fi
    done

    local end_time=$(date +%s%N)
    local total_duration=$(( (end_time - start_time) / 1000000000 ))

    # Test summary
    log_info "=== Unit Test Summary ==="
    log_info "Packages tested: $successful_packages/${#packages[@]}"
    log_info "Total tests: $total_tests"
    log_info "Failed tests: $total_failures"
    log_info "Success rate: $(( (total_tests - total_failures) * 100 / total_tests ))%"
    log_info "Total execution time: ${total_duration}s"

    if [ $total_failures -eq 0 ] && [ $successful_packages -eq ${#packages[@]} ]; then
        log_success "All unit tests passed!"
        return 0
    else
        log_error "Some unit tests failed!"
        return 1
    fi
}

# Generate coverage report
generate_coverage_report() {
    log_info "Generating coverage report..."

    # Merge coverage files
    local coverage_files=("$COVERAGE_DIR"/coverage-*.out)
    if [ ${#coverage_files[@]} -eq 0 ]; then
        log_warning "No coverage files found"
        return 0
    fi

    # Create merged coverage file
    local merged_coverage="$COVERAGE_DIR/coverage-merged.out"
    echo "mode: atomic" > "$merged_coverage"

    for file in "${coverage_files[@]}"; do
        if [ -f "$file" ]; then
            tail -n +2 "$file" >> "$merged_coverage"
        fi
    done

    # Generate coverage summary
    local coverage_percent=$(go tool cover -func="$merged_coverage" | \
                           tail -1 | \
                           awk '{print $3}' | \
                           sed 's/%//')

    log_info "Overall coverage: ${coverage_percent}%"

    # Check coverage threshold
    if (( $(echo "$coverage_percent >= $COVERAGE_THRESHOLD" | bc -l 2>/dev/null || echo "0") )); then
        log_success "Coverage threshold met: ${coverage_percent}% >= ${COVERAGE_THRESHOLD}%"
    else
        log_warning "Coverage below threshold: ${coverage_percent}% < ${COVERAGE_THRESHOLD}%"
    fi

    # Generate HTML report
    if command -v go >/dev/null 2>&1; then
        go tool cover -html="$merged_coverage" -o "$COVERAGE_DIR/coverage.html"
        log_success "Coverage HTML report: $COVERAGE_DIR/coverage.html"
    fi

    # Generate detailed coverage report
    go tool cover -func="$merged_coverage" > "$COVERAGE_DIR/coverage-summary.txt"

    # Create coverage JSON report
    cat > "$COVERAGE_DIR/coverage-report.json" << EOF
{
  "timestamp": "$(date -Iseconds)",
  "overall_coverage": $coverage_percent,
  "threshold": $COVERAGE_THRESHOLD,
  "threshold_met": $([ $(echo "$coverage_percent >= $COVERAGE_THRESHOLD" | bc -l 2>/dev/null || echo "0") -eq 1 ] && echo "true" || echo "false"),
  "coverage_files": [
EOF

    local first=true
    for file in "${coverage_files[@]}"; do
        if [ -f "$file" ]; then
            [ "$first" = false ] && echo "," >> "$COVERAGE_DIR/coverage-report.json"
            first=false

            local pkg_name=$(basename "$file" .out | sed 's/coverage-//')
            echo "    \"$pkg_name\"" >> "$COVERAGE_DIR/coverage-report.json"
        fi
    done

    cat >> "$COVERAGE_DIR/coverage-report.json" << EOF
  ]
}
EOF

    log_success "Coverage analysis completed"
}

# Run integration tests
run_integration_tests() {
    log_info "Running integration tests..."

    if [ ! -d "$PROJECT_ROOT/test/integration" ]; then
        log_info "No integration tests found, skipping..."
        return 0
    fi

    local start_time=$(date +%s%N)

    # Set integration test environment
    export TEST_INTEGRATION=true
    export FIRESTORE_EMULATOR_HOST="localhost:8080"
    export FIREBASE_AUTH_EMULATOR_HOST="localhost:9099"

    # Run integration tests
    if go test -v -timeout="$TEST_TIMEOUT" "$INTEGRATION_TEST_PATTERN"; then
        local end_time=$(date +%s%N)
        local duration=$(( (end_time - start_time) / 1000000000 ))

        log_success "Integration tests completed in ${duration}s"
        return 0
    else
        log_error "Integration tests failed"
        return 1
    fi
}

# Run benchmark tests
run_benchmark_tests() {
    log_info "Running benchmark tests..."

    local benchmarks=$(find "$PROJECT_ROOT" -name "*_test.go" -exec grep -l "func Benchmark" {} \; 2>/dev/null)

    if [ -z "$benchmarks" ]; then
        log_info "No benchmark tests found, skipping..."
        return 0
    fi

    # Run benchmarks
    go test -bench=. -benchmem -run=^$ ./... > "$REPORTS_DIR/benchmarks.txt" 2>&1

    if [ $? -eq 0 ]; then
        log_success "Benchmark tests completed"
        log_info "Benchmark results: $REPORTS_DIR/benchmarks.txt"
    else
        log_warning "Some benchmarks failed"
    fi
}

# Test performance analysis
analyze_test_performance() {
    log_info "Analyzing test performance..."

    # Analyze test execution times
    local report_files=("$REPORTS_DIR"/test-*.json)
    local total_time=0
    local package_count=0

    for file in "${report_files[@]}"; do
        if [ -f "$file" ]; then
            # Extract timing information from test JSON
            local test_time=$(grep '"Elapsed":' "$file" | \
                            sed 's/.*"Elapsed":\([0-9.]*\).*/\1/' | \
                            awk '{sum+=$1} END {print sum}')

            if [ -n "$test_time" ]; then
                total_time=$(echo "$total_time + $test_time" | bc -l 2>/dev/null || echo "$total_time")
                package_count=$((package_count + 1))
            fi
        fi
    done

    # Performance summary
    if [ $package_count -gt 0 ]; then
        local avg_time=$(echo "scale=2; $total_time / $package_count" | bc -l 2>/dev/null || echo "0")
        log_info "Average test time per package: ${avg_time}s"
    fi

    # Create performance report
    cat > "$REPORTS_DIR/performance-analysis.json" << EOF
{
  "timestamp": "$(date -Iseconds)",
  "parallel_jobs": $PARALLEL_JOBS,
  "total_packages": $package_count,
  "total_execution_time": $total_time,
  "average_time_per_package": $(echo "scale=2; $total_time / $package_count" | bc -l 2>/dev/null || echo "0"),
  "performance_rating": "$([ $(echo "$total_time < 60" | bc -l 2>/dev/null || echo "0") -eq 1 ] && echo "excellent" || echo "good")"
}
EOF

    log_success "Test performance analysis completed"
}

# Cleanup function
cleanup_tests() {
    log_info "Cleaning up test artifacts..."

    # Clean temporary files
    rm -f /tmp/test-result-*

    # Optionally clean test cache
    if [ "$1" = "clean" ]; then
        rm -rf "$CACHE_DIR"
        go clean -testcache
        log_info "Test cache cleaned"
    fi
}

# Main test function
main() {
    log_info "=== AIGIS Monolith - Parallel Test Runner ==="
    log_info "Starting parallel test execution..."

    local start_time=$(date +%s%N)

    cd "$PROJECT_ROOT"

    # Trap for cleanup
    trap cleanup_tests EXIT

    # Run test phases
    setup_test_env

    local test_success=true

    # Unit tests
    if ! run_parallel_unit_tests; then
        test_success=false
    fi

    # Coverage analysis
    generate_coverage_report

    # Optional integration tests
    if [ "$TEST_MODE" = "full" ] || [ "$1" = "integration" ]; then
        if ! run_integration_tests; then
            test_success=false
        fi
    fi

    # Optional benchmark tests
    if [ "$1" = "bench" ]; then
        run_benchmark_tests
    fi

    # Performance analysis
    analyze_test_performance

    local end_time=$(date +%s%N)
    local total_duration=$(( (end_time - start_time) / 1000000000 ))

    log_info "=== Test Execution Summary ==="
    log_info "Total execution time: ${total_duration}s"
    log_info "Test reports: $REPORTS_DIR"
    log_info "Coverage reports: $COVERAGE_DIR"

    if [ "$test_success" = true ]; then
        log_success "All tests completed successfully!"
        return 0
    else
        log_error "Some tests failed!"
        return 1
    fi
}

# Handle command line arguments
case "${1:-unit}" in
    unit)
        main
        ;;
    integration)
        TEST_MODE="full"
        main integration
        ;;
    bench)
        main bench
        ;;
    full)
        TEST_MODE="full"
        main integration
        ;;
    clean)
        cleanup_tests clean
        log_success "Test environment cleaned"
        ;;
    help)
        echo "Usage: $0 {unit|integration|bench|full|clean|help}"
        echo "  unit        - Run unit tests in parallel (default)"
        echo "  integration - Run unit and integration tests"
        echo "  bench       - Run benchmark tests"
        echo "  full        - Run all tests including integration"
        echo "  clean       - Clean test cache and artifacts"
        echo "  help        - Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  PARALLEL_JOBS       - Number of parallel test jobs (default: nproc)"
        echo "  TEST_TIMEOUT        - Test timeout (default: 10m)"
        echo "  COVERAGE_THRESHOLD  - Coverage threshold % (default: 80)"
        echo "  RACE_DETECTION      - Enable race detection (default: true)"
        echo "  VERBOSE             - Verbose test output (default: false)"
        ;;
    *)
        echo "Unknown command: $1"
        echo "Run '$0 help' for usage information"
        exit 1
        ;;
esac