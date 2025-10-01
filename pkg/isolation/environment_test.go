// Copyright Pigeonworks LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package isolation

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPortAllocator implements PortAllocator for testing
type mockPortAllocator struct {
	basePort atomic.Int32
}

func newMockPortAllocator(startPort int) *mockPortAllocator {
	m := &mockPortAllocator{}
	m.basePort.Store(int32(startPort))
	return m
}

func (m *mockPortAllocator) AllocateRange(count int) (int, error) {
	port := m.basePort.Add(int32(count + 100))
	return int(port) - (count + 100), nil
}

func (m *mockPortAllocator) IsPortInUse(port int) bool {
	return false
}

func TestEnvironmentManager_CreateEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
		MaxRetries:   10,
	}

	idGen := NewIDGenerator(config)
	portAlloc := newMockPortAllocator(20000)
	manager := NewEnvironmentManager(idGen, portAlloc)

	t.Run("creates valid environment", func(t *testing.T) {
		env, err := manager.CreateEnvironment(5)
		require.NoError(t, err)
		defer manager.Cleanup(env)

		assert.NotEmpty(t, env.ID)
		assert.NotEmpty(t, env.TempDir)
		assert.NotEmpty(t, env.LockFile)
		assert.NotEmpty(t, env.EnvFile)
		assert.Equal(t, 5, env.Ports.Count)

		// Verify temp directory exists
		_, err = os.Stat(env.TempDir)
		assert.NoError(t, err)

		// Verify lock file exists
		_, err = os.Stat(env.LockFile)
		assert.NoError(t, err)

		// Verify env file exists
		_, err = os.Stat(env.EnvFile)
		assert.NoError(t, err)
	})

	t.Run("creates unique environments", func(t *testing.T) {
		env1, err := manager.CreateEnvironment(3)
		require.NoError(t, err)
		defer manager.Cleanup(env1)

		env2, err := manager.CreateEnvironment(3)
		require.NoError(t, err)
		defer manager.Cleanup(env2)

		assert.NotEqual(t, env1.ID, env2.ID)
		assert.NotEqual(t, env1.TempDir, env2.TempDir)
		assert.NotEqual(t, env1.Ports.BasePort, env2.Ports.BasePort)
	})
}

func TestEnvironmentManager_createEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
		MaxRetries:   10,
	}

	manager := NewEnvironmentManager(NewIDGenerator(config), nil)

	env := &Environment{
		ID:           "test-123",
		WorktreePath: tmpDir,
		TempDir:      "/tmp/test-123",
		Ports: &PortRange{
			BasePort: 20000,
			Count:    5,
		},
	}

	t.Run("creates env file with correct content", func(t *testing.T) {
		envFile, err := manager.createEnvFile(env)
		require.NoError(t, err)
		defer os.Remove(envFile)

		// Read and verify content
		data, err := os.ReadFile(envFile)
		require.NoError(t, err)

		content := string(data)
		assert.Contains(t, content, "ISOLATION_ID=test-123")
		assert.Contains(t, content, "TEMP_DIR=/tmp/test-123")
		assert.Contains(t, content, "PORT_BASE=20000")
		assert.Contains(t, content, "PORT_COUNT=5")
		assert.Contains(t, content, "FIRESTORE_PORT=20000")
		assert.Contains(t, content, "AUTH_PORT=20001")
		assert.Contains(t, content, "API_PORT=20002")
	})
}

func TestEnvironmentManager_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
		MaxRetries:   10,
	}

	idGen := NewIDGenerator(config)
	portAlloc := newMockPortAllocator(20000)
	manager := NewEnvironmentManager(idGen, portAlloc)

	t.Run("cleans up all resources", func(t *testing.T) {
		env, err := manager.CreateEnvironment(3)
		require.NoError(t, err)

		// Verify resources exist
		_, err = os.Stat(env.TempDir)
		assert.NoError(t, err)
		_, err = os.Stat(env.EnvFile)
		assert.NoError(t, err)
		assert.True(t, idGen.IsLocked(env.ID))

		// Cleanup
		err = manager.Cleanup(env)
		assert.NoError(t, err)

		// Verify resources are gone
		_, err = os.Stat(env.TempDir)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(env.EnvFile)
		assert.True(t, os.IsNotExist(err))
		assert.False(t, idGen.IsLocked(env.ID))
	})

	t.Run("cleanup is safe when resources already removed", func(t *testing.T) {
		env, err := manager.CreateEnvironment(2)
		require.NoError(t, err)

		// Remove resources manually
		os.RemoveAll(env.TempDir)
		os.Remove(env.EnvFile)
		idGen.ReleaseLock(env.ID)

		// Cleanup should not error
		err = manager.Cleanup(env)
		assert.NoError(t, err)
	})
}

func TestEnvironmentManager_Validate(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
		MaxRetries:   10,
	}

	portAlloc := newMockPortAllocator(20000)
	manager := NewEnvironmentManager(NewIDGenerator(config), portAlloc)

	t.Run("validates healthy environment", func(t *testing.T) {
		env, err := manager.CreateEnvironment(3)
		require.NoError(t, err)
		defer manager.Cleanup(env)

		err = manager.Validate(env)
		assert.NoError(t, err)
	})

	t.Run("fails when lock missing", func(t *testing.T) {
		env, err := manager.CreateEnvironment(2)
		require.NoError(t, err)
		defer manager.Cleanup(env)

		// Remove lock
		manager.idGen.ReleaseLock(env.ID)

		err = manager.Validate(env)
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "lock")
	})

	t.Run("fails when temp directory missing", func(t *testing.T) {
		env, err := manager.CreateEnvironment(2)
		require.NoError(t, err)
		defer manager.Cleanup(env)

		// Remove temp directory
		os.RemoveAll(env.TempDir)

		err = manager.Validate(env)
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "temp")
	})

	t.Run("fails when env file missing", func(t *testing.T) {
		env, err := manager.CreateEnvironment(2)
		require.NoError(t, err)
		defer manager.Cleanup(env)

		// Remove env file
		os.Remove(env.EnvFile)

		err = manager.Validate(env)
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "env")
	})
}

func TestEnvironmentManager_ConcurrentEnvironments(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
		MaxRetries:   10,
	}

	portAlloc := newMockPortAllocator(20000)
	manager := NewEnvironmentManager(NewIDGenerator(config), portAlloc)

	t.Run("creates multiple environments concurrently", func(t *testing.T) {
		const goroutines = 10
		envs := make(chan *Environment, goroutines)
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				env, err := manager.CreateEnvironment(3)
				if err == nil {
					envs <- env
				}
			}()
		}

		wg.Wait()
		close(envs)

		// Collect and verify
		created := []*Environment{}
		for env := range envs {
			created = append(created, env)
		}

		// Verify uniqueness
		ids := make(map[string]bool)
		tempDirs := make(map[string]bool)
		ports := make(map[int]bool)

		for _, env := range created {
			assert.False(t, ids[env.ID], "duplicate ID: %s", env.ID)
			assert.False(t, tempDirs[env.TempDir], "duplicate temp dir: %s", env.TempDir)

			for i := 0; i < env.Ports.Count; i++ {
				port, _ := env.Ports.GetPort(i)
				assert.False(t, ports[port], "duplicate port: %d", port)
				ports[port] = true
			}

			ids[env.ID] = true
			tempDirs[env.TempDir] = true
		}

		// Cleanup all
		for _, env := range created {
			manager.Cleanup(env)
		}
	})
}

func TestNewEnvironmentManager(t *testing.T) {
	t.Run("uses provided components", func(t *testing.T) {
		idGen := NewIDGenerator(nil)
		portAlloc := newMockPortAllocator(20000)

		manager := NewEnvironmentManager(idGen, portAlloc)
		assert.Equal(t, idGen, manager.idGen)
		assert.Equal(t, portAlloc, manager.portAlloc)
	})

	t.Run("creates default idGen when nil", func(t *testing.T) {
		portAlloc := newMockPortAllocator(20000)
		manager := NewEnvironmentManager(nil, portAlloc)
		assert.NotNil(t, manager.idGen)
	})
}
