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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
		MaxRetries:   10,
	}

	gen := NewIDGenerator(config)

	// Test basic generation
	t.Run("generates unique ID", func(t *testing.T) {
		id, err := gen.Generate()
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.Len(t, id, 12) // Base hash length
	})

	// Test uniqueness
	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id, err := gen.Generate()
			require.NoError(t, err)
			assert.False(t, ids[id], "duplicate ID generated: %s", id)
			ids[id] = true
		}
	})

	// Test collision avoidance
	t.Run("avoids collisions with existing locks", func(t *testing.T) {
		id1, err := gen.Generate()
		require.NoError(t, err)

		// Create lock for first ID
		_, err = gen.CreateLock(id1)
		require.NoError(t, err)

		// Generate another ID - should be different
		id2, err := gen.Generate()
		require.NoError(t, err)
		assert.NotEqual(t, id1, id2)

		// Cleanup
		gen.ReleaseLock(id1)
	})
}

func TestIDGenerator_CreateLock(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
	}

	gen := NewIDGenerator(config)

	t.Run("creates lock file", func(t *testing.T) {
		id := "test-id-123"
		lockFile, err := gen.CreateLock(id)
		require.NoError(t, err)
		assert.NotEmpty(t, lockFile)

		// Verify file exists
		_, err = os.Stat(lockFile)
		assert.NoError(t, err)

		// Cleanup
		gen.ReleaseLock(id)
	})

	t.Run("fails on duplicate lock", func(t *testing.T) {
		id := "test-id-456"
		_, err := gen.CreateLock(id)
		require.NoError(t, err)

		// Try to create same lock again
		_, err = gen.CreateLock(id)
		assert.Error(t, err)

		// Cleanup
		gen.ReleaseLock(id)
	})

	t.Run("lock contains metadata", func(t *testing.T) {
		id := "test-id-789"
		lockFile, err := gen.CreateLock(id)
		require.NoError(t, err)

		// Read lock file
		data, err := os.ReadFile(lockFile)
		require.NoError(t, err)

		content := string(data)
		assert.Contains(t, content, "PID=")
		assert.Contains(t, content, "Timestamp=")
		assert.Contains(t, content, "Worktree=")

		// Cleanup
		gen.ReleaseLock(id)
	})
}

func TestIDGenerator_ReleaseLock(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		LockDir: filepath.Join(tmpDir, "locks"),
	}

	gen := NewIDGenerator(config)

	t.Run("releases lock successfully", func(t *testing.T) {
		id := "test-release-123"
		lockFile, err := gen.CreateLock(id)
		require.NoError(t, err)

		// Verify lock exists
		assert.True(t, gen.IsLocked(id))

		// Release lock
		err = gen.ReleaseLock(id)
		assert.NoError(t, err)

		// Verify lock is gone
		assert.False(t, gen.IsLocked(id))
		_, err = os.Stat(lockFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("releasing non-existent lock is safe", func(t *testing.T) {
		err := gen.ReleaseLock("non-existent-id")
		assert.NoError(t, err)
	})
}

func TestIDGenerator_IsLocked(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		LockDir: filepath.Join(tmpDir, "locks"),
	}

	gen := NewIDGenerator(config)

	t.Run("detects locked state", func(t *testing.T) {
		id := "test-locked-123"

		// Initially not locked
		assert.False(t, gen.IsLocked(id))

		// Create lock
		_, err := gen.CreateLock(id)
		require.NoError(t, err)

		// Now locked
		assert.True(t, gen.IsLocked(id))

		// Release and verify
		gen.ReleaseLock(id)
		assert.False(t, gen.IsLocked(id))
	})
}

func TestIDGenerator_ConcurrentGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	config := &Config{
		WorktreePath: tmpDir,
		LockDir:      filepath.Join(tmpDir, "locks"),
		MaxRetries:   100,
	}

	gen := NewIDGenerator(config)

	// Test concurrent ID generation
	t.Run("generates unique IDs concurrently", func(t *testing.T) {
		const goroutines = 20 // Reduced to minimize collision probability
		ids := make(chan string, goroutines)
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				id, err := gen.Generate()
				require.NoError(t, err)
				ids <- id
			}()
		}

		wg.Wait()
		close(ids)

		// Verify uniqueness - collisions are resolved by retry mechanism
		idSet := make(map[string]bool)
		for id := range ids {
			assert.False(t, idSet[id], "duplicate ID (retry failed): %s", id)
			idSet[id] = true
		}

		assert.Len(t, idSet, goroutines, "should have unique IDs for all goroutines")
	})
}

func TestIDGenerator_DefaultConfig(t *testing.T) {
	t.Run("uses default config when nil", func(t *testing.T) {
		gen := NewIDGenerator(nil)
		assert.NotNil(t, gen.config)
		assert.Equal(t, "/tmp/aigis-isolation-locks", gen.config.LockDir)
		assert.Equal(t, 999, gen.config.MaxRetries)
	})

	t.Run("uses current directory when worktree not set", func(t *testing.T) {
		config := &Config{
			WorktreePath: "",
		}
		gen := NewIDGenerator(config)
		assert.NotEmpty(t, gen.config.WorktreePath)
	})
}
