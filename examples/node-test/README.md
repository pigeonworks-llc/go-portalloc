# Node.js Test Example

This example demonstrates using `go-parallel-test-env` with Node.js/Jest tests.

## Setup

```bash
# Install the tool
go install github.com/pigeonworks-llc/go-parallel-test-env/cmd/go-parallel-test-env@latest

# Create isolated environment with shell output
eval "$(go-parallel-test-env create --ports 5 --shell)"
```

## Integration with package.json

Add to your `package.json`:

```json
{
  "scripts": {
    "test:isolated": "go-parallel-test-env create --ports 5 --shell > .env.test && source .env.test && npm test",
    "test:parallel": "node parallel-test.js"
  }
}
```

## Example Test File (Jest)

```javascript
// test/api.test.js
describe('API Tests with Isolated Ports', () => {
  const basePort = process.env.PORT_BASE || 3000;
  const apiPort = process.env.API_PORT || basePort;

  test('starts server on isolated port', async () => {
    const server = await startServer(apiPort);
    expect(server.address().port).toBe(parseInt(apiPort));
    await server.close();
  });
});
```

## Parallel Test Execution

```javascript
// parallel-test.js
const { exec } = require('child_process');
const { promisify } = require('util');
const execAsync = promisify(exec);

async function runParallelTests(shardCount) {
  const promises = [];

  for (let i = 1; i <= shardCount; i++) {
    const promise = (async () => {
      // Create isolated environment
      const { stdout } = await execAsync(
        `go-parallel-test-env create --ports 5 --instance-id shard-${i} --json`
      );
      const env = JSON.parse(stdout);

      console.log(`Shard ${i}: Using ports ${env.ports.ports}`);

      // Run tests with environment
      process.env.ISOLATION_ID = env.isolation_id;
      process.env.PORT_BASE = env.ports.base_port;

      await execAsync('npm test', {
        env: { ...process.env }
      });

      // Cleanup
      await execAsync(`go-parallel-test-env cleanup --id ${env.isolation_id}`);
    })();

    promises.push(promise);
  }

  await Promise.all(promises);
  console.log('✅ All parallel tests completed!');
}

runParallelTests(3).catch(console.error);
```

## Cleanup

```bash
# Cleanup single environment
go-parallel-test-env cleanup --id $ISOLATION_ID

# Cleanup all environments
go-parallel-test-env cleanup --all
```
