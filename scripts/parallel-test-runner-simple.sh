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

# Generate unique environment ID
generate_env_id() {
    echo -n "$(pwd)-$$-$(date +%s)" | sha256sum | cut -c1-8
}

# Run tests with isolated environment
run_isolated_test() {
    local test_path="$1"
    local env_id="$2"
    local base_port="$3"

    log_info "Running tests: $test_path (env: $env_id, port: $base_port)"

    # Set up isolated environment
    export ENVIRONMENT_ID="$env_id"
    export FIRESTORE_EMULATOR_PORT=$((base_port + 1))
    export POSTGRES_PORT=$((base_port + 2))
    export API_PORT=$((base_port + 3))
    export ADMIN_API_PORT=$((base_port + 4))
    export USER_API_PORT=$((base_port + 5))

    # Set up test environment
    export FIRESTORE_EMULATOR_HOST="localhost:$FIRESTORE_EMULATOR_PORT"
    export USE_FIRESTORE_EMULATOR=true
    export DATABASE_TYPE=memory
    export TEST_ENVIRONMENT=true
    export DEBUG=false

    # Create temporary test data directory
    local test_data_dir="/tmp/aigis-test-$env_id"
    mkdir -p "$test_data_dir"
    export TEST_DATA_DIR="$test_data_dir"

    # Run the test
    local output_file="/tmp/test-output-$env_id.log"
    local exit_code=0

    if timeout 300s go test -v -short "$test_path" > "$output_file" 2>&1; then
        log_success "Tests passed: $test_path"
        cat "$output_file"
    else
        exit_code=$?
        log_error "Tests failed: $test_path (exit code: $exit_code)"
        cat "$output_file"
    fi

    # Cleanup
    rm -rf "$test_data_dir" "$output_file"

    return $exit_code
}

# Main function
main() {
    local test_pattern="${1:-./...}"
    local max_parallel="${2:-4}"

    log_info "Starting parallel test runner"
    log_info "Test pattern: $test_pattern"
    log_info "Max parallel: $max_parallel"

    cd "$PROJECT_ROOT"

    # Get list of test packages
    local test_packages
    if ! test_packages=$(go list "$test_pattern" 2>/dev/null); then
        log_error "Failed to list test packages for pattern: $test_pattern"
        return 1
    fi

    if [ -z "$test_packages" ]; then
        log_warning "No test packages found for pattern: $test_pattern"
        return 0
    fi

    log_info "Found $(echo "$test_packages" | wc -l) test package(s)"

    # Run tests in parallel with environment isolation
    local pids=()
    local env_counter=0
    local failed_tests=()

    for package in $test_packages; do
        # Wait if we have too many parallel processes
        while [ ${#pids[@]} -ge $max_parallel ]; do
            for i in "${!pids[@]}"; do
                if ! kill -0 "${pids[$i]}" 2>/dev/null; then
                    wait "${pids[$i]}"
                    local exit_code=$?
                    if [ $exit_code -ne 0 ]; then
                        failed_tests+=("$package")
                    fi
                    unset "pids[$i]"
                    break
                fi
            done
            sleep 0.1
        done

        # Start new test process
        local env_id=$(printf "%s-%02d" $(generate_env_id) $env_counter)
        local base_port=$((20000 + env_counter * 10))

        run_isolated_test "$package" "$env_id" "$base_port" &
        pids+=($!)

        env_counter=$((env_counter + 1))
    done

    # Wait for all remaining processes
    for pid in "${pids[@]}"; do
        if [ -n "$pid" ]; then
            wait "$pid"
            local exit_code=$?
            if [ $exit_code -ne 0 ]; then
                failed_tests+=("unknown")
            fi
        fi
    done

    # Report results
    echo
    echo "📊 Parallel Test Results"
    echo "========================"

    if [ ${#failed_tests[@]} -eq 0 ]; then
        log_success "All tests passed successfully!"
        return 0
    else
        log_error "Some tests failed:"
        for failed in "${failed_tests[@]}"; do
            echo "  ❌ $failed"
        done
        return 1
    fi
}

# Print usage if requested
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "Usage: $0 [test_pattern] [max_parallel]"
    echo
    echo "Arguments:"
    echo "  test_pattern   Go test pattern (default: ./...)"
    echo "  max_parallel   Maximum parallel tests (default: 4)"
    echo
    echo "Examples:"
    echo "  $0                              # Run all tests with 4 parallel workers"
    echo "  $0 ./api/... 2                  # Run API tests with 2 parallel workers"
    echo "  $0 ./api/internal/modules/... 8 # Run module tests with 8 parallel workers"
    exit 0
fi

# Execute main function
main "$@"