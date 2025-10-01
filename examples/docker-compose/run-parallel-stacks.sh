#!/bin/bash
# Example: Run multiple docker-compose stacks in parallel with dynamic port allocation

set -euo pipefail

PARALLEL_COUNT=${1:-2}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🚀 Starting $PARALLEL_COUNT parallel docker-compose stacks"
echo "=============================================="

# Arrays to track processes and IDs
declare -a PIDS=()
declare -a ISOLATION_IDS=()

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up all stacks..."

    # Stop all background processes
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done

    # Cleanup all environments
    for iso_id in "${ISOLATION_IDS[@]}"; do
        echo "  Cleaning up environment: $iso_id"
        docker-compose -p "stack-$iso_id" down 2>/dev/null || true
        go-portalloc cleanup --id "$iso_id" || true
    done

    echo "✅ Cleanup completed"
}

# Trap cleanup on exit
trap cleanup EXIT INT TERM

# Function to run isolated docker-compose stack
run_isolated_stack() {
    local shard=$1
    local log_file="/tmp/docker-stack-$shard.log"

    {
        echo "================================"
        echo "Shard $shard: Starting"
        echo "================================"

        # Allocate ports
        ENV_JSON=$(go-portalloc create --ports 5 --instance-id "docker-shard-$shard" --json)
        ISOLATION_ID=$(echo "$ENV_JSON" | jq -r '.isolation_id')

        # Parse ports
        POSTGRES_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[0]')
        REDIS_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[1]')
        API_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[2]')

        echo "Shard $shard: Allocated ports"
        echo "  PostgreSQL: $POSTGRES_PORT"
        echo "  Redis: $REDIS_PORT"
        echo "  API: $API_PORT"
        echo "  Isolation ID: $ISOLATION_ID"

        # Create minimal docker-compose for this shard
        COMPOSE_FILE="/tmp/docker-compose-$ISOLATION_ID.yml"
        cat > "$COMPOSE_FILE" <<EOF
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    ports:
      - "$POSTGRES_PORT:5432"
    environment:
      POSTGRES_PASSWORD: test$shard
      POSTGRES_DB: testdb$shard
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 2s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "$REDIS_PORT:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 2s
      timeout: 5s
      retries: 5
EOF

        # Start services with unique project name
        PROJECT_NAME="stack-$ISOLATION_ID"
        echo "Shard $shard: Starting docker-compose (project: $PROJECT_NAME)"
        docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d

        # Wait for services to be healthy
        echo "Shard $shard: Waiting for services to be healthy..."
        sleep 3

        # Verify services are running
        if docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" ps | grep -q "Up"; then
            echo "Shard $shard: ✅ Services are running"

            # Simulate test execution
            echo "Shard $shard: Running tests..."
            sleep 2
            echo "Shard $shard: ✅ Tests completed"
        else
            echo "Shard $shard: ❌ Services failed to start"
        fi

        # Cleanup this shard
        echo "Shard $shard: Stopping services..."
        docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v
        rm "$COMPOSE_FILE"
        go-portalloc cleanup --id "$ISOLATION_ID"

        echo "Shard $shard: ✅ Completed and cleaned up"
        echo ""

    } | tee "$log_file"
}

# Start parallel stacks
for i in $(seq 1 "$PARALLEL_COUNT"); do
    run_isolated_stack "$i" &
    PIDS+=($!)
done

# Wait for all stacks to complete
echo "⏳ Waiting for all stacks to complete..."
for pid in "${PIDS[@]}"; do
    wait "$pid"
done

echo ""
echo "✅ All $PARALLEL_COUNT docker-compose stacks completed successfully!"
echo "=============================================="
echo "Logs available at: /tmp/docker-stack-*.log"
