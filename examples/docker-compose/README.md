# Docker Compose Integration Example

`go-portalloc` can dynamically allocate ports for docker-compose services, enabling parallel execution without port conflicts.

## Basic Usage

### 1. Allocate Ports

```bash
# Allocate ports and output as JSON
go-portalloc create --ports 10 --json > ports.json

# Parse ports for services
export POSTGRES_PORT=$(jq -r '.ports.ports[0]' ports.json)
export REDIS_PORT=$(jq -r '.ports.ports[1]' ports.json)
export API_PORT=$(jq -r '.ports.ports[2]' ports.json)
export WEB_PORT=$(jq -r '.ports.ports[3]' ports.json)
```

### 2. Use in docker-compose.yml

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    environment:
      POSTGRES_PASSWORD: test

  redis:
    image: redis:7
    ports:
      - "${REDIS_PORT:-6379}:6379"

  api:
    build: ./api
    ports:
      - "${API_PORT:-8080}:8080"
    depends_on:
      - postgres
      - redis
    environment:
      DATABASE_URL: postgresql://postgres:test@postgres:5432/testdb
      REDIS_URL: redis://redis:6379

  web:
    build: ./web
    ports:
      - "${WEB_PORT:-3000}:3000"
    depends_on:
      - api
```

### 3. Run with Allocated Ports

```bash
#!/bin/bash
set -euo pipefail

# Allocate ports
ENV_JSON=$(go-portalloc create --ports 10 --json)
ISOLATION_ID=$(echo "$ENV_JSON" | jq -r '.isolation_id')

# Parse ports
export POSTGRES_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[0]')
export REDIS_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[1]')
export API_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[2]')
export WEB_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[3]')

echo "Using ports:"
echo "  Postgres: $POSTGRES_PORT"
echo "  Redis: $REDIS_PORT"
echo "  API: $API_PORT"
echo "  Web: $WEB_PORT"

# Cleanup on exit
trap "docker-compose down && go-portalloc cleanup --id $ISOLATION_ID" EXIT

# Start services
docker-compose up -d

# Wait for services to be healthy
sleep 5

# Run tests
go test ./integration/... -v

echo "✅ Tests completed"
```

## Parallel docker-compose Execution

Run multiple docker-compose stacks in parallel without port conflicts:

```bash
#!/bin/bash
# parallel-docker-test.sh

PARALLEL_COUNT=${1:-3}

run_isolated_stack() {
    local shard=$1

    # Allocate ports for this shard
    ENV_JSON=$(go-portalloc create --ports 10 --instance-id "shard-$shard" --json)
    ISOLATION_ID=$(echo "$ENV_JSON" | jq -r '.isolation_id')

    # Export ports
    export POSTGRES_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[0]')
    export REDIS_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[1]')
    export API_PORT=$(echo "$ENV_JSON" | jq -r '.ports.ports[2]')

    # Use unique project name
    export COMPOSE_PROJECT_NAME="test-shard-$shard"

    echo "Shard $shard: Starting services on ports $POSTGRES_PORT, $REDIS_PORT, $API_PORT"

    # Start services
    docker-compose -f docker-compose.test.yml up -d

    # Run tests
    go test ./... -run "TestShard$shard"

    # Cleanup
    docker-compose -f docker-compose.test.yml down
    go-portalloc cleanup --id "$ISOLATION_ID"

    echo "Shard $shard: ✅ Completed"
}

# Run shards in parallel
for i in $(seq 1 "$PARALLEL_COUNT"); do
    run_isolated_stack "$i" &
done

wait
echo "✅ All parallel tests completed"
```

## GitHub Actions Integration

```yaml
name: Parallel Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        shard: [1, 2, 3, 4]

    steps:
      - uses: actions/checkout@v3

      - name: Install go-portalloc
        run: go install github.com/pigeonworks-llc/go-portalloc/cmd/go-portalloc@latest

      - name: Allocate ports
        id: ports
        run: |
          ENV_JSON=$(go-portalloc create --ports 10 --instance-id "ci-${{ matrix.shard }}" --json)
          echo "isolation_id=$(echo $ENV_JSON | jq -r '.isolation_id')" >> $GITHUB_OUTPUT
          echo "postgres_port=$(echo $ENV_JSON | jq -r '.ports.ports[0]')" >> $GITHUB_OUTPUT
          echo "redis_port=$(echo $ENV_JSON | jq -r '.ports.ports[1]')" >> $GITHUB_OUTPUT
          echo "api_port=$(echo $ENV_JSON | jq -r '.ports.ports[2]')" >> $GITHUB_OUTPUT

      - name: Start services
        env:
          POSTGRES_PORT: ${{ steps.ports.outputs.postgres_port }}
          REDIS_PORT: ${{ steps.ports.outputs.redis_port }}
          API_PORT: ${{ steps.ports.outputs.api_port }}
          COMPOSE_PROJECT_NAME: ci-shard-${{ matrix.shard }}
        run: docker-compose up -d

      - name: Run tests
        run: go test ./... -parallel 4

      - name: Cleanup
        if: always()
        run: |
          docker-compose down
          go-portalloc cleanup --id ${{ steps.ports.outputs.isolation_id }}
```

## Advanced: Dynamic Service Discovery

```bash
#!/bin/bash
# Generate docker-compose.yml with allocated ports

ENV_JSON=$(go-portalloc create --ports 20 --json)
ISOLATION_ID=$(echo "$ENV_JSON" | jq -r '.isolation_id')

# Generate docker-compose with allocated ports
cat > docker-compose.generated.yml <<EOF
version: '3.8'

services:
  postgres:
    image: postgres:15
    ports:
      - "$(echo "$ENV_JSON" | jq -r '.ports.ports[0]'):5432"
    environment:
      POSTGRES_PASSWORD: test

  redis:
    image: redis:7
    ports:
      - "$(echo "$ENV_JSON" | jq -r '.ports.ports[1]'):6379"

  api:
    build: ./api
    ports:
      - "$(echo "$ENV_JSON" | jq -r '.ports.ports[2]'):8080"
    environment:
      DATABASE_URL: postgresql://postgres:test@localhost:$(echo "$ENV_JSON" | jq -r '.ports.ports[0]')/testdb
      REDIS_URL: redis://localhost:$(echo "$ENV_JSON" | jq -r '.ports.ports[1]')
EOF

# Cleanup on exit
trap "docker-compose -f docker-compose.generated.yml down && rm docker-compose.generated.yml && go-portalloc cleanup --id $ISOLATION_ID" EXIT

# Run
docker-compose -f docker-compose.generated.yml up -d
go test ./...
```

## Benefits

✅ **No Port Conflicts**: Each docker-compose stack gets unique ports
✅ **Parallel CI/CD**: Run multiple test suites concurrently
✅ **Local Development**: Test multiple branches simultaneously
✅ **Zero Configuration**: Automatic port allocation
✅ **Clean Separation**: Each stack is completely isolated

## Tips

1. **Always use unique COMPOSE_PROJECT_NAME** to avoid container name conflicts
2. **Allocate extra ports** for potential service additions
3. **Use trap for cleanup** to ensure resources are released
4. **Test connectivity** before running actual tests
5. **Monitor port usage** with `go-portalloc validate`
