# go-portalloc

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

> **Kill "Port Already in Use" Forever: Zero-Overhead Port Allocation**

Collision-free dynamic port allocation for parallel testing and development.
Pure Go stdlib. Zero external dependencies. Just works.

```bash
eval "$(go-portalloc create --ports 10 --shell)"
docker-compose up -d  # Port conflicts? Never again.
```

## 🎯 Features

- **🔐 Complete Isolation**: SHA256-based unique ID generation with multi-layer entropy
- **🔌 Dynamic Port Allocation**: Automatic port conflict resolution (20000-30000 range)
- **🔒 Atomic Locking**: Concurrent-safe environment creation with collision detection
- **🧹 Automatic Cleanup**: Safe resource cleanup with idempotent operations
- **⚡ Zero Dependencies**: Pure Go standard library (except CLI framework)
- **🌐 Language Agnostic**: Works with Go, Node.js, Python, or any test framework

## 🚀 Quick Start

### Installation

```bash
go install github.com/pigeonworks-llc/go-portalloc/cmd/go-portalloc@latest
```

### Basic Usage

```bash
# Create isolated environment
go-portalloc create --ports 5

# Output:
# ✅ Environment created successfully!
#   Isolation ID:  abc123def456
#   Base Port:     23086
#   Allocated Ports: [23086 23087 23088 23089 23090]

# Use with docker-compose
eval "$(go-portalloc create --ports 10 --shell)"
docker-compose up -d

# Run your tests
go test ./...

# Cleanup
go-portalloc cleanup --id abc123def456
```

## 📦 Use Cases

### CI/CD Parallel Testing

```yaml
# GitHub Actions
jobs:
  test:
    strategy:
      matrix:
        shard: [1, 2, 3, 4]
    steps:
      - name: Create isolated environment
        run: |
          eval "$(go-portalloc create --ports 5 --shell)"
          echo "ISOLATION_ID=$ISOLATION_ID" >> $GITHUB_ENV

      - name: Run tests
        run: go test ./... -parallel 4

      - name: Cleanup
        if: always()
        run: go-portalloc cleanup --id $ISOLATION_ID
```

### Local Development

```bash
# Terminal 1
go-portalloc create --ports 5 --instance-id branch-feature-a
source .env.isolation
npm test

# Terminal 2 (simultaneously)
go-portalloc create --ports 5 --instance-id branch-feature-b
source .env.isolation
npm test

# No port conflicts! 🎉
```

### Microservices Integration Testing

```bash
# Allocate ports for multiple services
go-portalloc create --ports 10 --json > env.json

# Parse and use in docker-compose
export POSTGRES_PORT=$(jq -r '.ports.ports[0]' env.json)
export REDIS_PORT=$(jq -r '.ports.ports[1]' env.json)
export API_PORT=$(jq -r '.ports.ports[2]' env.json)

docker-compose up -d
go test ./integration/...
```

## 🛠️ Commands

### `create` - Create Isolated Environment

```bash
go-portalloc create [flags]

Flags:
  -p, --ports int          Number of ports to allocate (default 5)
  -i, --instance-id string Custom instance ID
  -w, --worktree string    Working directory path
      --json               Output as JSON
      --shell              Output as shell eval format
```

**Output Formats:**

**Human (default):**
```
✅ Environment created successfully!
  Isolation ID:  abc123def456
  Temp Directory: /tmp/portalloc-abc123def456
  Base Port:      23086
  Allocated Ports: [23086 23087 23088 23089 23090]
```

**JSON:**
```json
{
  "isolation_id": "abc123def456",
  "worktree_path": "/path/to/project",
  "temp_dir": "/tmp/portalloc-abc123def456",
  "lock_file": "/tmp/portalloc-locks/env-abc123def456.lock",
  "env_file": "/path/to/project/.env.isolation",
  "ports": {
    "base_port": 23086,
    "count": 5,
    "ports": [23086, 23087, 23088, 23089, 23090]
  }
}
```

**Shell:**
```bash
export ISOLATION_ID=abc123def456
export TEMP_DIR=/tmp/portalloc-abc123def456
export PORT_BASE=23086
export PORT_COUNT=5
export FIRESTORE_PORT=23086
export AUTH_PORT=23087
export API_PORT=23088
```

### `validate` - Validate Environment

```bash
go-portalloc validate --id <isolation-id>

# Checks:
# ✓ Lock file exists
# ✓ Temp directory exists
# ✓ Env file exists
# ✓ Ports are accessible
```

### `cleanup` - Cleanup Environment

```bash
# Single environment
go-portalloc cleanup --id <isolation-id>

# All environments
go-portalloc cleanup --all
```

## 🏗️ Architecture

### Isolation ID Generation

```
SHA256(worktree + instance_id + timestamp + random + hostname + pid)
└─> 12-char hash (e.g., abc123def456)
    └─> Collision detection with automatic retry (max 999 attempts)
```

**Entropy Sources:**
1. Worktree path (project-specific)
2. Instance ID (user-provided or auto-generated)
3. Nanosecond timestamp
4. Cryptographic random number
5. Hostname
6. Process ID

**Collision Probability:** < 0.0001% with retry mechanism

### Port Allocation Algorithm

```
1. Random base port selection (20000-30000)
2. TCP listener test for availability
3. Consecutive port range validation
4. Automatic retry on conflict (max 10 attempts)
```

**Features:**
- Real-time port availability detection
- Consecutive port range guarantee
- No lsof/netstat dependency (pure Go)

### Lock Mechanism

```
Atomic file creation: O_CREATE | O_EXCL | O_WRONLY
├─> Fails if lock exists (prevents race conditions)
├─> Metadata: PID, timestamp, worktree
└─> Safe cleanup on process termination
```

## 📊 Performance

```
Benchmark Results (Apple M1, Go 1.24+):

ID Generation:           ~100 μs/op
Port Allocation (5):     ~2 ms/op
Environment Creation:    ~5 ms/op
Concurrent Creation (10):~50 ms total
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Integration test
./bin/go-portalloc create --ports 5
./bin/go-portalloc validate --id <id>
./bin/go-portalloc cleanup --id <id>
```

## 🔧 Programmatic Usage

### Go API Overview

go-portalloc provides three main packages for programmatic use:

| Package | Description | Use Case |
|---------|-------------|----------|
| `pkg/ports` | Port allocation and availability checking | Standalone port management |
| `pkg/isolation` | ID generation and locking | Unique environment isolation |
| `pkg/isolation` | Full environment management | Complete test isolation |

### Package: `pkg/ports`

**Simple port allocation without environment isolation.**

```go
import "github.com/pigeonworks-llc/go-portalloc/pkg/ports"

// Create allocator with default config (ports 20000-30000)
allocator := ports.NewAllocator(nil)

// Allocate consecutive ports
basePort, err := allocator.AllocateRange(5)
if err != nil {
    log.Fatal(err)
}

// Use allocated ports
fmt.Printf("Allocated ports: %d-%d\n", basePort, basePort+4)

// Check specific port availability
if allocator.IsPortInUse(8080) {
    log.Println("Port 8080 is already in use")
}

// Verify specific ports are available
err = allocator.AllocateSpecific(3000, 3001, 3002)
if err != nil {
    log.Fatal("Ports unavailable:", err)
}
```

**Custom port range configuration:**

```go
config := &ports.AllocatorConfig{
    StartPort:  10000,
    EndPort:    20000,
    MaxRetries: 20,
    RetryDelay: 500 * time.Millisecond,
}

allocator := ports.NewAllocator(config)
basePort, err := allocator.AllocateRange(10)
```

### Package: `pkg/isolation`

**Full environment management with ID generation, locking, and cleanup.**

```go
import (
    "github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
    "github.com/pigeonworks-llc/go-portalloc/pkg/ports"
)

// Create environment manager
config := &isolation.Config{
    WorktreePath: "/path/to/project",
    InstanceID:   "test-123",
    MaxRetries:   999,
}

idGen := isolation.NewIDGenerator(config)
portAlloc := ports.NewAllocator(nil)
manager := isolation.NewEnvironmentManager(idGen, portAlloc)

// Create isolated environment
env, err := manager.CreateEnvironment(5)
if err != nil {
    log.Fatal(err)
}
defer manager.Cleanup(env)

// Use environment
fmt.Printf("Isolation ID: %s\n", env.ID)
fmt.Printf("Base Port: %d\n", env.Ports.BasePort)
fmt.Printf("All Ports: %v\n", env.Ports.Ports())

// Validate isolation
if err := manager.Validate(env); err != nil {
    log.Fatal("validation failed:", err)
}
```

### Advanced Usage Examples

**Parallel test isolation:**

```go
func TestParallelExecution(t *testing.T) {
    t.Parallel()

    manager := createTestManager()
    env, err := manager.CreateEnvironment(3)
    require.NoError(t, err)
    defer manager.Cleanup(env)

    // Each parallel test gets isolated ports
    serverPort, _ := env.Ports.GetPort(0)
    dbPort, _ := env.Ports.GetPort(1)
    cachePort, _ := env.Ports.GetPort(2)

    // Run test with isolated resources
    server := startTestServer(serverPort)
    defer server.Close()
}
```

**Manual ID generation and locking:**

```go
import "github.com/pigeonworks-llc/go-portalloc/pkg/isolation"

// Generate unique isolation ID
config := &isolation.Config{
    WorktreePath: ".",
    InstanceID:   "custom-id",
}
idGen := isolation.NewIDGenerator(config)
isolationID, err := idGen.Generate()

// Create lock
lockPath, err := idGen.CreateLock(isolationID)
if err != nil {
    log.Fatal("failed to create lock:", err)
}
defer idGen.ReleaseLock(isolationID)
```

**PortRange utilities:**

```go
portRange := &ports.PortRange{
    BasePort: 23000,
    Count:    5,
}

// Get all ports as slice
allPorts := portRange.Ports() // [23000, 23001, 23002, 23003, 23004]

// Get specific port by index
apiPort, err := portRange.GetPort(0)    // 23000
dbPort, err := portRange.GetPort(1)     // 23001
cachePort, err := portRange.GetPort(2)  // 23002
```

### Shell Script Integration

```bash
#!/bin/bash
set -e

# Create environment
ENV_JSON=$(go-portalloc create --ports 5 --json)
ISOLATION_ID=$(echo "$ENV_JSON" | jq -r '.isolation_id')

# Trap cleanup on exit
trap "go-portalloc cleanup --id $ISOLATION_ID" EXIT

# Use environment
source .env.isolation
echo "Running tests with ports: $PORT_BASE-$((PORT_BASE + PORT_COUNT - 1))"

# Run tests
go test ./...
```

## 🆚 Comparison

| Feature | go-portalloc | testcontainers | localstack | docker-compose |
|---------|---------------------|----------------|------------|----------------|
| **Language** | ✅ Agnostic | ⚠️ Java/Go/Python | ⚠️ AWS only | ✅ Agnostic |
| **Dependencies** | ✅ Zero | ❌ Docker required | ❌ Docker + AWS | ❌ Docker |
| **Overhead** | ✅ ~5ms | ⚠️ ~5s (container start) | ⚠️ ~10s | ⚠️ ~10s |
| **Port Allocation** | ✅ Dynamic | ⚠️ Random | ⚠️ Fixed | ⚠️ Manual |
| **Parallel Safety** | ✅ Atomic locks | ⚠️ Container isolation | ⚠️ Container isolation | ⚠️ Network isolation |
| **Cleanup** | ✅ Automatic | ⚠️ Manual/auto | ⚠️ Manual | ⚠️ Manual |

**When to use `go-portalloc`:**
- ✅ Lightweight parallel test isolation
- ✅ No Docker dependency needed
- ✅ Pure port/directory isolation sufficient
- ✅ Fast iteration cycles (< 10ms overhead)

**When NOT to use:**
- ❌ Need actual service containers (use testcontainers)
- ❌ Need AWS service mocks (use localstack)
- ❌ Complex multi-service orchestration (use docker-compose)

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

```bash
# Clone repository
git clone https://github.com/pigeonworks-llc/go-portalloc.git
cd go-portalloc

# Install dependencies
go mod download

# Run tests
go test ./...

# Run linter
golangci-lint run

# Build CLI
go build -o bin/go-portalloc ./cmd/go-portalloc
```

## 📝 License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

Copyright Pigeonworks LLC

## 🙏 Acknowledgments

- Inspired by Firebase parallel test isolation patterns
- Built following UNIX philosophy: "Do One Thing Well"
- Part of the AIGIS Monolith OSS extraction initiative

## 📚 Documentation

- [Architecture Design](docs/architecture.md)
- [API Reference](https://pkg.go.dev/github.com/pigeonworks-llc/go-portalloc)
- [Examples](examples/)
- [Troubleshooting](docs/troubleshooting.md)

## 🔗 Related Projects

- [go-entity-id](https://github.com/pigeonworks-llc/go-entity-id) - Type-safe entity ID system
- [go-domain-event](https://github.com/pigeonworks-llc/go-domain-event) - Lightweight domain events
- [go-event-bus](https://github.com/pigeonworks-llc/go-event-bus) - In-memory event bus

---

**Made with ❤️ by Pigeonworks LLC**
