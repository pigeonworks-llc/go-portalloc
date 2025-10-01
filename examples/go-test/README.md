# Go Test Example

This example demonstrates using `go-parallel-test-env` with Go tests.

## Setup

```bash
# Install the tool
go install github.com/pigeonworks-llc/go-parallel-test-env/cmd/go-parallel-test-env@latest

# Create isolated environment
go-parallel-test-env create --ports 3 --instance-id go-example
source .env.isolation
```

## Run Tests

```bash
# Run tests with environment variables
go test ./... -v

# The tests will use ports from the isolated environment
# Output shows: Using ports 23086, 23087, 23088
```

## Cleanup

```bash
go-parallel-test-env cleanup --id $ISOLATION_ID
```

## Integration with Test Suite

See `example_test.go` for how to integrate environment creation within your test suite.
