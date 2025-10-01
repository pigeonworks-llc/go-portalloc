#!/bin/bash
# Example: Parallel test execution with isolated environments

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
PARALLEL_COUNT=${PARALLEL_COUNT:-3}
TEST_COMMAND=${TEST_COMMAND:-"echo 'Running test'"}

echo "🚀 Starting $PARALLEL_COUNT parallel test environments"
echo "---------------------------------------------------"

# Array to store environment IDs
declare -a ENV_IDS=()

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up environments..."
    for env_id in "${ENV_IDS[@]}"; do
        go-parallel-test-env cleanup --id "$env_id" || true
    done
}

# Trap cleanup on exit
trap cleanup EXIT INT TERM

# Function to run test in isolated environment
run_test() {
    local shard=$1
    local log_file="/tmp/test-shard-$shard.log"

    {
        echo "Starting shard $shard..."

        # Create isolated environment
        ENV_JSON=$(go-parallel-test-env create --ports 5 --instance-id "shard-$shard" --json)
        ENV_ID=$(echo "$ENV_JSON" | jq -r '.isolation_id')
        BASE_PORT=$(echo "$ENV_JSON" | jq -r '.ports.base_port')

        echo "Shard $shard: Environment $ENV_ID created (ports: $BASE_PORT-$((BASE_PORT + 4)))"

        # Export environment variables
        export ISOLATION_ID="$ENV_ID"
        export SHARD_ID="$shard"
        export PORT_BASE="$BASE_PORT"

        # Load environment file
        source .env.isolation 2>/dev/null || true

        # Run test command
        echo "Shard $shard: Running tests..."
        eval "$TEST_COMMAND"

        echo "Shard $shard: ✅ Tests completed"
        echo "$ENV_ID"
    } | tee "$log_file"
}

# Start parallel test execution
for i in $(seq 1 "$PARALLEL_COUNT"); do
    ENV_ID=$(run_test "$i") &
    ENV_IDS+=("$ENV_ID")
done

# Wait for all tests to complete
echo ""
echo "⏳ Waiting for all tests to complete..."
wait

echo ""
echo "✅ All parallel tests completed successfully!"
echo "---------------------------------------------------"

# Logs are available at /tmp/test-shard-*.log
