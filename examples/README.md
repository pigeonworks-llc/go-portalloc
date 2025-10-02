# go-portalloc Examples

This directory contains comprehensive examples demonstrating various use cases of go-portalloc.

## Examples Overview

| Example | Description | Key Features |
|---------|-------------|--------------|
| [basic_port_allocation](./basic_port_allocation) | Simple port allocation | Port ranges, availability checks, basic usage |
| [parallel_testing](./parallel_testing) | Parallel test execution | Concurrent testing, test isolation |
| [custom_config](./custom_config) | Custom configuration | Port range config, retry settings, PortRange utilities |
| [full_environment](./full_environment) | Complete environment management | ID generation, locking, validation, cleanup |

## Running the Examples

### Basic Port Allocation

```bash
cd examples/basic_port_allocation
go run main.go
```

**Output:**
```
=== Basic Port Allocation Example ===

1. Allocating 5 consecutive ports...
   ✓ Allocated ports: 23456-23460

2. Checking if port 8080 is in use...
   ✓ Port 8080 is available

...
```

**What it demonstrates:**
- Creating a port allocator with default configuration
- Allocating consecutive ports
- Checking port availability
- Starting servers on allocated ports
- Verifying cleanup

### Parallel Testing

```bash
cd examples/parallel_testing
go test -v -parallel 4
```

**Output:**
```
=== RUN   TestParallelService1
=== PAUSE TestParallelService1
=== RUN   TestParallelService2
=== PAUSE TestParallelService2
...
```

**What it demonstrates:**
- Safe parallel test execution
- Automatic port conflict resolution
- Microservices integration testing
- Multiple concurrent test servers

### Custom Configuration

```bash
cd examples/custom_config
go run main.go
```

**Output:**
```
=== Custom Configuration Example ===

1. Creating allocator with custom port range (10000-15000)...
   ✓ Allocated ports in custom range: 12345-12349
...
```

**What it demonstrates:**
- Custom port ranges
- Retry behavior configuration
- PortRange utility methods
- Boundary checking
- Production-ready configurations

### Full Environment Management

```bash
cd examples/full_environment
go run main.go
```

**Output:**
```
=== Full Environment Management Example ===

1. Creating environment manager...
   ✓ Environment manager created

2. Creating isolated environment with 5 ports...
   ✓ Environment created successfully
     Isolation ID:   abc123def456
     Temp Directory: /tmp/portalloc-abc123def456
     ...
```

**What it demonstrates:**
- Complete isolation ID generation
- Environment locking mechanism
- Directory and file management
- Environment validation
- Automatic cleanup
- Manual lock management

## Common Patterns

### Pattern 1: Simple Port Allocation

```go
allocator := ports.NewAllocator(nil)
basePort, err := allocator.AllocateRange(5)
// Use ports: basePort, basePort+1, ..., basePort+4
```

### Pattern 2: Custom Port Range

```go
config := &ports.AllocatorConfig{
    StartPort:  10000,
    EndPort:    20000,
    MaxRetries: 20,
    RetryDelay: 500 * time.Millisecond,
}
allocator := ports.NewAllocator(config)
```

### Pattern 3: Parallel Testing

```go
func TestMyService(t *testing.T) {
    t.Parallel()

    allocator := ports.NewAllocator(nil)
    basePort, _ := allocator.AllocateRange(3)

    // Each test gets isolated ports
    listener, _ := net.Listen("tcp", fmt.Sprintf(":%d", basePort))
    defer listener.Close()

    // Run your test...
}
```

### Pattern 4: Full Environment Isolation

```go
config := &isolation.Config{
    WorktreePath: ".",
    InstanceID:   "test-123",
}

manager := isolation.NewEnvironmentManager(
    isolation.NewIDGenerator(config),
    ports.NewAllocator(nil),
)

env, _ := manager.CreateEnvironment(5)
defer manager.Cleanup(env)

// Use env.Ports.GetPort(0), env.Ports.GetPort(1), etc.
```

## Integration with Testing Frameworks

### With `testing.T`

```go
func TestIntegration(t *testing.T) {
    allocator := ports.NewAllocator(nil)
    basePort, err := allocator.AllocateRange(3)
    if err != nil {
        t.Fatal("Port allocation failed:", err)
    }

    // Use allocated ports
}
```

### With `testify/require`

```go
func TestWithTestify(t *testing.T) {
    allocator := ports.NewAllocator(nil)
    basePort, err := allocator.AllocateRange(3)
    require.NoError(t, err)
    require.Greater(t, basePort, 0)
}
```

### With Docker Compose

```bash
# Allocate ports
eval "$(go-portalloc create --ports 10 --shell)"

# Export to environment
export POSTGRES_PORT=$PORT_BASE
export REDIS_PORT=$((PORT_BASE + 1))
export API_PORT=$((PORT_BASE + 2))

# Run docker-compose
docker-compose up -d
```

## Troubleshooting

### All ports in range are busy

```go
config := &ports.AllocatorConfig{
    StartPort:  20000,
    EndPort:    50000,  // Wider range
    MaxRetries: 50,     // More retries
}
```

### Ports not released after test

```go
// Always use defer for cleanup
listener, _ := net.Listen("tcp", fmt.Sprintf(":%d", port))
defer listener.Close()  // Ensures cleanup on panic
```

### Race conditions in parallel tests

```go
// Each test must allocate its own ports
func TestRaceCondition(t *testing.T) {
    t.Parallel()

    // ✓ CORRECT: Allocate inside test
    allocator := ports.NewAllocator(nil)
    basePort, _ := allocator.AllocateRange(3)
}

// ❌ WRONG: Shared port allocation
var sharedPort int
func init() {
    allocator := ports.NewAllocator(nil)
    sharedPort, _ = allocator.AllocateRange(1)
}
```

## Performance Benchmarks

Run benchmarks to see allocation performance:

```bash
cd examples/basic_port_allocation
go test -bench=. -benchmem
```

## Contributing

Found a useful pattern? Submit a pull request with a new example!

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) file for details.

Copyright Pigeonworks LLC
