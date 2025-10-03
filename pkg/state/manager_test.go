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

package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	t.Run("creates manager successfully", func(t *testing.T) {
		mgr, err := NewManager()
		require.NoError(t, err)
		require.NotNil(t, mgr)
		assert.NotEmpty(t, mgr.statePath)
	})

	t.Run("creates state directory if not exists", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		require.NoError(t, err)

		stateDir := filepath.Join(homeDir, ".go-portalloc")

		mgr, err := NewManager()
		require.NoError(t, err)
		require.NotNil(t, mgr)

		// Verify directory exists
		info, err := os.Stat(stateDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestManager_RecordEnvironment(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)

	// Clean up state file before test
	defer os.Remove(mgr.statePath)

	t.Run("records new environment", func(t *testing.T) {
		env := &isolation.Environment{
			ID:           "test-123",
			WorktreePath: "/path/to/project",
			TempDir:      "/tmp/test-123",
			LockFile:     "/tmp/locks/test-123.lock",
			EnvFile:      "/path/to/.env",
			Ports: &ports.PortRange{
				BasePort: 20000,
				Count:    5,
			},
		}

		err := mgr.RecordEnvironment(env)
		require.NoError(t, err)

		// Verify environment was recorded
		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		require.Len(t, envs, 1)

		assert.Equal(t, "test-123", envs[0].ID)
		assert.Equal(t, "/path/to/project", envs[0].WorktreePath)
		assert.Equal(t, 20000, envs[0].Ports.BasePort)
		assert.Equal(t, 5, envs[0].Ports.Count)
	})

	t.Run("updates existing environment", func(t *testing.T) {
		env := &isolation.Environment{
			ID:           "test-123",
			WorktreePath: "/updated/path",
			TempDir:      "/tmp/test-123-updated",
			LockFile:     "/tmp/locks/test-123.lock",
			EnvFile:      "/updated/.env",
			Ports: &ports.PortRange{
				BasePort: 25000,
				Count:    3,
			},
		}

		err := mgr.RecordEnvironment(env)
		require.NoError(t, err)

		// Verify environment was updated
		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		require.Len(t, envs, 1)

		assert.Equal(t, "test-123", envs[0].ID)
		assert.Equal(t, "/updated/path", envs[0].WorktreePath)
		assert.Equal(t, 25000, envs[0].Ports.BasePort)
		assert.Equal(t, 3, envs[0].Ports.Count)
	})

	t.Run("records multiple environments", func(t *testing.T) {
		env2 := &isolation.Environment{
			ID:           "test-456",
			WorktreePath: "/another/path",
			TempDir:      "/tmp/test-456",
			LockFile:     "/tmp/locks/test-456.lock",
			EnvFile:      "/another/.env",
			Ports: &ports.PortRange{
				BasePort: 30000,
				Count:    2,
			},
		}

		err := mgr.RecordEnvironment(env2)
		require.NoError(t, err)

		// Verify both environments exist
		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		require.Len(t, envs, 2)
	})
}

func TestManager_RemoveEnvironment(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	// Add test environments
	env1 := &isolation.Environment{
		ID:           "test-111",
		WorktreePath: "/path1",
		TempDir:      "/tmp/test-111",
		LockFile:     "/tmp/locks/test-111.lock",
		EnvFile:      "/path1/.env",
		Ports:        &ports.PortRange{BasePort: 20000, Count: 2},
	}
	env2 := &isolation.Environment{
		ID:           "test-222",
		WorktreePath: "/path2",
		TempDir:      "/tmp/test-222",
		LockFile:     "/tmp/locks/test-222.lock",
		EnvFile:      "/path2/.env",
		Ports:        &ports.PortRange{BasePort: 20100, Count: 2},
	}

	require.NoError(t, mgr.RecordEnvironment(env1))
	require.NoError(t, mgr.RecordEnvironment(env2))

	t.Run("removes specific environment", func(t *testing.T) {
		err := mgr.RemoveEnvironment("test-111")
		require.NoError(t, err)

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		require.Len(t, envs, 1)
		assert.Equal(t, "test-222", envs[0].ID)
	})

	t.Run("removing non-existent environment is safe", func(t *testing.T) {
		err := mgr.RemoveEnvironment("non-existent")
		require.NoError(t, err)

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		require.Len(t, envs, 1)
	})
}

func TestManager_ListEnvironments(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	t.Run("returns empty list when no state file", func(t *testing.T) {
		os.Remove(mgr.statePath) // Ensure file doesn't exist

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		assert.Empty(t, envs)
	})

	t.Run("returns empty list when state file is empty", func(t *testing.T) {
		// Create empty state file
		f, err := os.Create(mgr.statePath)
		require.NoError(t, err)
		f.Close()

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		assert.Empty(t, envs)
	})

	t.Run("returns all environments", func(t *testing.T) {
		// Add multiple environments
		for i := 0; i < 3; i++ {
			env := &isolation.Environment{
				ID:           fmt.Sprintf("test-%d", i),
				WorktreePath: fmt.Sprintf("/path%d", i),
				TempDir:      fmt.Sprintf("/tmp/test-%d", i),
				LockFile:     fmt.Sprintf("/tmp/locks/test-%d.lock", i),
				EnvFile:      fmt.Sprintf("/path%d/.env", i),
				Ports:        &ports.PortRange{BasePort: 20000 + (i * 100), Count: 2},
			}
			require.NoError(t, mgr.RecordEnvironment(env))
		}

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		assert.Len(t, envs, 3)
	})
}

func TestManager_GetEnvironment(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	env := &isolation.Environment{
		ID:           "test-get",
		WorktreePath: "/path",
		TempDir:      "/tmp/test-get",
		LockFile:     "/tmp/locks/test-get.lock",
		EnvFile:      "/path/.env",
		Ports:        &ports.PortRange{BasePort: 20000, Count: 2},
	}
	require.NoError(t, mgr.RecordEnvironment(env))

	t.Run("gets existing environment", func(t *testing.T) {
		retrieved, err := mgr.GetEnvironment("test-get")
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, "test-get", retrieved.ID)
		assert.Equal(t, "/path", retrieved.WorktreePath)
	})

	t.Run("returns error for non-existent environment", func(t *testing.T) {
		_, err := mgr.GetEnvironment("non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	t.Run("handles concurrent writes safely", func(t *testing.T) {
		const goroutines = 10
		done := make(chan bool, goroutines)

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				env := &isolation.Environment{
					ID:           fmt.Sprintf("concurrent-%d", id),
					WorktreePath: fmt.Sprintf("/path%d", id),
					TempDir:      fmt.Sprintf("/tmp/test-%d", id),
					LockFile:     fmt.Sprintf("/tmp/locks/test-%d.lock", id),
					EnvFile:      fmt.Sprintf("/path%d/.env", id),
					Ports:        &ports.PortRange{BasePort: 20000 + id, Count: 2},
				}
				err := mgr.RecordEnvironment(env)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < goroutines; i++ {
			<-done
		}

		// Verify all environments were recorded
		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		assert.Len(t, envs, goroutines)
	})
}

func TestManager_StateFilePersistence(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	env := &isolation.Environment{
		ID:           "persist-test",
		WorktreePath: "/path",
		TempDir:      "/tmp/persist-test",
		LockFile:     "/tmp/locks/persist-test.lock",
		EnvFile:      "/path/.env",
		Ports:        &ports.PortRange{BasePort: 20000, Count: 2},
	}

	t.Run("persists across manager instances", func(t *testing.T) {
		// Record with first manager
		err := mgr.RecordEnvironment(env)
		require.NoError(t, err)

		// Create new manager instance
		mgr2, err := NewManager()
		require.NoError(t, err)

		// Verify data persisted
		envs, err := mgr2.ListEnvironments()
		require.NoError(t, err)
		require.Len(t, envs, 1)
		assert.Equal(t, "persist-test", envs[0].ID)
	})
}

func TestIsProcessRunning(t *testing.T) {
	t.Run("returns false for invalid PID", func(t *testing.T) {
		assert.False(t, IsProcessRunning(0))
		assert.False(t, IsProcessRunning(-1))
	})

	t.Run("returns true for current process", func(t *testing.T) {
		currentPID := os.Getpid()
		assert.True(t, IsProcessRunning(currentPID))
	})

	t.Run("returns false for non-existent process", func(t *testing.T) {
		// Use a very high PID that's unlikely to exist
		assert.False(t, IsProcessRunning(999999))
	})
}

func TestGetEnvironmentStatus(t *testing.T) {
	t.Run("returns active for running process", func(t *testing.T) {
		env := &EnvironmentState{
			ID:        "test",
			PID:       os.Getpid(),
			CreatedAt: time.Now(),
		}

		status := GetEnvironmentStatus(env)
		assert.Equal(t, StatusActive, status)
	})

	t.Run("returns stale for non-running process", func(t *testing.T) {
		env := &EnvironmentState{
			ID:        "test",
			PID:       999999, // Non-existent PID
			CreatedAt: time.Now(),
		}

		status := GetEnvironmentStatus(env)
		assert.Equal(t, StatusStale, status)
	})

	t.Run("returns stale for zero PID", func(t *testing.T) {
		env := &EnvironmentState{
			ID:        "test",
			PID:       0,
			CreatedAt: time.Now(),
		}

		status := GetEnvironmentStatus(env)
		assert.Equal(t, StatusStale, status)
	})
}
