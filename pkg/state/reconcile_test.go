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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Reconcile(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	// Create temporary lock directory
	lockDir := t.TempDir()

	t.Run("reconciles from empty lock directory", func(t *testing.T) {
		count, err := mgr.Reconcile(lockDir)
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		assert.Empty(t, envs)
	})

	t.Run("reconciles from lock files", func(t *testing.T) {
		// Create mock lock files
		worktree := t.TempDir()

		for i := 0; i < 3; i++ {
			isolationID := fmt.Sprintf("test%d", i)
			lockFile := filepath.Join(lockDir, fmt.Sprintf("env-%s.lock", isolationID))

			// Write lock file with metadata
			content := fmt.Sprintf("PID=%d\nTimestamp=%d\nWorktree=%s\n",
				os.Getpid(),
				time.Now().Unix(),
				worktree)

			err := os.WriteFile(lockFile, []byte(content), 0o600)
			require.NoError(t, err)

			// Create corresponding env file
			envFile := filepath.Join(worktree, ".env.isolation")
			envContent := fmt.Sprintf("PORT_BASE=%d\nPORT_COUNT=%d\n", 20000+i*100, 5)
			err = os.WriteFile(envFile, []byte(envContent), 0o644)
			require.NoError(t, err)
		}

		count, err := mgr.Reconcile(lockDir)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		assert.Len(t, envs, 3)

		// Verify environment details
		for _, env := range envs {
			assert.NotEmpty(t, env.ID)
			assert.Equal(t, worktree, env.WorktreePath)
			assert.NotNil(t, env.Ports)
		}
	})

	t.Run("overwrites existing state", func(t *testing.T) {
		// First reconcile creates state
		count, err := mgr.Reconcile(lockDir)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Remove one lock file
		lockFiles, err := filepath.Glob(filepath.Join(lockDir, "env-*.lock"))
		require.NoError(t, err)
		require.NotEmpty(t, lockFiles)

		err = os.Remove(lockFiles[0])
		require.NoError(t, err)

		// Reconcile again
		count, err = mgr.Reconcile(lockDir)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		envs, err := mgr.ListEnvironments()
		require.NoError(t, err)
		assert.Len(t, envs, 2)
	})

	t.Run("handles invalid lock files gracefully", func(t *testing.T) {
		// Create invalid lock file
		invalidLock := filepath.Join(lockDir, "env-invalid.lock")
		err := os.WriteFile(invalidLock, []byte("invalid content"), 0o600)
		require.NoError(t, err)

		count, err := mgr.Reconcile(lockDir)
		require.NoError(t, err)
		// Should skip invalid file
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("sets last_reconciled_at timestamp", func(t *testing.T) {
		beforeReconcile := time.Now()
		time.Sleep(10 * time.Millisecond)

		_, err := mgr.Reconcile(lockDir)
		require.NoError(t, err)

		// Read state file and verify timestamp
		f, err := os.Open(mgr.statePath)
		require.NoError(t, err)
		defer f.Close()

		var state State
		require.NoError(t, json.NewDecoder(f).Decode(&state))

		assert.True(t, state.LastReconciledAt.After(beforeReconcile))
	})
}

func TestManager_parseLockFile(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	lockDir := t.TempDir()
	worktree := t.TempDir()

	t.Run("parses valid lock file", func(t *testing.T) {
		isolationID := "parseme"
		lockFile := filepath.Join(lockDir, fmt.Sprintf("env-%s.lock", isolationID))

		content := fmt.Sprintf("PID=%d\nTimestamp=%d\nWorktree=%s\n",
			12345,
			time.Now().Unix(),
			worktree)

		err := os.WriteFile(lockFile, []byte(content), 0o600)
		require.NoError(t, err)

		envState, err := mgr.parseLockFile(lockFile)
		require.NoError(t, err)

		assert.Equal(t, isolationID, envState.ID)
		assert.Equal(t, 12345, envState.PID)
		assert.Equal(t, worktree, envState.WorktreePath)
		assert.NotEmpty(t, envState.TempDir)
		assert.NotEmpty(t, envState.LockFile)
	})

	t.Run("returns error for invalid lock file name", func(t *testing.T) {
		invalidLock := filepath.Join(lockDir, "invalid.lock")
		err := os.WriteFile(invalidLock, []byte("content"), 0o600)
		require.NoError(t, err)

		_, err = mgr.parseLockFile(invalidLock)
		assert.Error(t, err)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := mgr.parseLockFile("/non/existent/file.lock")
		assert.Error(t, err)
	})
}

func TestManager_parseEnvFile(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	t.Run("parses valid env file", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), ".env.isolation")
		content := "PORT_BASE=25000\nPORT_COUNT=5\n"
		err := os.WriteFile(envFile, []byte(content), 0o644)
		require.NoError(t, err)

		ports := mgr.parseEnvFile(envFile)
		require.NotNil(t, ports)

		assert.Equal(t, 25000, ports.BasePort)
		assert.Equal(t, 5, ports.Count)
		assert.Len(t, ports.Allocated, 5)
		assert.Equal(t, 25000, ports.Allocated[0])
		assert.Equal(t, 25004, ports.Allocated[4])
	})

	t.Run("returns empty PortsState for non-existent file", func(t *testing.T) {
		ports := mgr.parseEnvFile("/non/existent/.env")
		require.NotNil(t, ports)
		assert.Equal(t, 0, ports.BasePort)
		assert.Equal(t, 0, ports.Count)
		assert.Empty(t, ports.Allocated)
	})

	t.Run("handles partial env file", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), ".env.isolation")
		content := "PORT_BASE=30000\n" // Missing PORT_COUNT
		err := os.WriteFile(envFile, []byte(content), 0o644)
		require.NoError(t, err)

		ports := mgr.parseEnvFile(envFile)
		require.NotNil(t, ports)

		assert.Equal(t, 30000, ports.BasePort)
		assert.Equal(t, 0, ports.Count)
		assert.Empty(t, ports.Allocated)
	})

	t.Run("handles invalid port values", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), ".env.isolation")
		content := "PORT_BASE=invalid\nPORT_COUNT=notanumber\n"
		err := os.WriteFile(envFile, []byte(content), 0o644)
		require.NoError(t, err)

		ports := mgr.parseEnvFile(envFile)
		require.NotNil(t, ports)

		assert.Equal(t, 0, ports.BasePort)
		assert.Equal(t, 0, ports.Count)
	})
}

func TestReconcile_IntegrationWithRealEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	mgr, err := NewManager()
	require.NoError(t, err)
	defer os.Remove(mgr.statePath)

	// Use the actual lock directory
	lockDir := filepath.Join(os.TempDir(), "go-portalloc-locks-test-reconcile")
	defer os.RemoveAll(lockDir)

	err = os.MkdirAll(lockDir, 0o755)
	require.NoError(t, err)

	worktree := t.TempDir()

	t.Run("reconciles and maintains consistency", func(t *testing.T) {
		// Create multiple lock files with realistic data
		for i := 0; i < 5; i++ {
			isolationID := fmt.Sprintf("integration-%d", i)
			lockFile := filepath.Join(lockDir, fmt.Sprintf("env-%s.lock", isolationID))

			content := fmt.Sprintf("PID=%d\nTimestamp=%d\nWorktree=%s\n",
				os.Getpid(),
				time.Now().Unix(),
				worktree)

			err := os.WriteFile(lockFile, []byte(content), 0o600)
			require.NoError(t, err)
		}

		// Reconcile multiple times
		for j := 0; j < 3; j++ {
			count, err := mgr.Reconcile(lockDir)
			require.NoError(t, err)
			assert.Equal(t, 5, count, "reconcile iteration %d", j)

			envs, err := mgr.ListEnvironments()
			require.NoError(t, err)
			assert.Len(t, envs, 5, "reconcile iteration %d", j)
		}
	})
}
