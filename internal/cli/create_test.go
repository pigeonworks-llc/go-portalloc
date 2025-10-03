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

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests using actual CLI binary
func TestCLIIntegration(t *testing.T) {
	// Build CLI binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/go-portalloc-test", "../../cmd/go-portalloc")
	require.NoError(t, buildCmd.Run(), "Failed to build CLI")
	defer os.Remove("/tmp/go-portalloc-test")

	t.Run("create command with JSON output", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("/tmp/go-portalloc-test", "create", "--json")
		cmd.Dir = tmpDir
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(output, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "isolation_id")
		assert.Contains(t, result, "compose_project_name")
		assert.Contains(t, result, "ports")

		// Cleanup
		isolationID := result["isolation_id"].(string)
		cleanupCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", isolationID)
		cleanupCmd.Dir = tmpDir
		_ = cleanupCmd.Run()
	})

	t.Run("create command with custom port count", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("/tmp/go-portalloc-test", "create", "--ports", "15", "--json")
		cmd.Dir = tmpDir
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(output, &result)
		require.NoError(t, err)

		ports := result["ports"].(map[string]interface{})
		assert.Equal(t, float64(15), ports["count"])

		// Cleanup
		isolationID := result["isolation_id"].(string)
		cleanupCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", isolationID)
		cleanupCmd.Dir = tmpDir
		_ = cleanupCmd.Run()
	})

	t.Run("create and cleanup lifecycle", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create
		createCmd := exec.Command("/tmp/go-portalloc-test", "create", "--json")
		createCmd.Dir = tmpDir
		createOutput, err := createCmd.CombinedOutput()
		require.NoError(t, err)

		var createResult map[string]interface{}
		require.NoError(t, json.Unmarshal(createOutput, &createResult))
		isolationID := createResult["isolation_id"].(string)

		// Validate
		validateCmd := exec.Command("/tmp/go-portalloc-test", "validate", "--id", isolationID)
		validateCmd.Dir = tmpDir
		validateOutput, err := validateCmd.CombinedOutput()
		require.NoError(t, err)
		assert.Contains(t, string(validateOutput), "valid")

		// Cleanup
		cleanupCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", isolationID)
		cleanupCmd.Dir = tmpDir
		cleanupOutput, err := cleanupCmd.CombinedOutput()
		require.NoError(t, err)
		assert.Contains(t, string(cleanupOutput), "cleaned up successfully")
	})

	t.Run("shell output format", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("/tmp/go-portalloc-test", "create", "--shell")
		cmd.Dir = tmpDir
		output, err := cmd.CombinedOutput()

		require.NoError(t, err)

		outputStr := string(output)
		assert.Contains(t, outputStr, "export ISOLATION_ID=")
		assert.Contains(t, outputStr, "export COMPOSE_PROJECT_NAME=portalloc-")
		assert.Contains(t, outputStr, "export TEMP_DIR=")
		assert.Contains(t, outputStr, "export PORT_BASE=")

		// Extract isolation ID for cleanup
		lines := strings.Split(outputStr, "\n")
		var isolationID string
		for _, line := range lines {
			if strings.HasPrefix(line, "export ISOLATION_ID=") {
				isolationID = strings.TrimPrefix(line, "export ISOLATION_ID=")
				break
			}
		}

		if isolationID != "" {
			cleanupCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", isolationID)
			cleanupCmd.Dir = tmpDir
			_ = cleanupCmd.Run()
		}
	})

	t.Run("cleanup is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", "nonexistent-id")
		cmd.Dir = tmpDir
		_, err := cmd.CombinedOutput()

		// Cleanup of non-existent environment should not error
		assert.NoError(t, err)
	})

	t.Run("list command shows environments", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an environment
		createCmd := exec.Command("/tmp/go-portalloc-test", "create", "--json")
		createCmd.Dir = tmpDir
		createOutput, err := createCmd.CombinedOutput()
		require.NoError(t, err)

		var createResult map[string]interface{}
		require.NoError(t, json.Unmarshal(createOutput, &createResult))
		isolationID := createResult["isolation_id"].(string)

		// List environments (table format)
		listCmd := exec.Command("/tmp/go-portalloc-test", "list")
		listCmd.Dir = tmpDir
		listOutput, err := listCmd.CombinedOutput()
		require.NoError(t, err)

		outputStr := string(listOutput)
		assert.Contains(t, outputStr, isolationID)
		assert.Contains(t, outputStr, "STATUS")

		// List environments (JSON format)
		listJSONCmd := exec.Command("/tmp/go-portalloc-test", "list", "--format", "json")
		listJSONCmd.Dir = tmpDir
		listJSONOutput, err := listJSONCmd.CombinedOutput()
		require.NoError(t, err)

		var listResult []map[string]interface{}
		require.NoError(t, json.Unmarshal(listJSONOutput, &listResult))
		assert.NotEmpty(t, listResult)

		// Find our environment in the list
		found := false
		for _, env := range listResult {
			if env["id"] == isolationID {
				found = true
				assert.Contains(t, env, "status")
				assert.Contains(t, env, "ports")
				break
			}
		}
		assert.True(t, found, "environment not found in list")

		// Cleanup
		cleanupCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", isolationID)
		cleanupCmd.Dir = tmpDir
		_ = cleanupCmd.Run()
	})

	t.Run("reconcile command rebuilds state", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an environment
		createCmd := exec.Command("/tmp/go-portalloc-test", "create", "--json")
		createCmd.Dir = tmpDir
		createOutput, err := createCmd.CombinedOutput()
		require.NoError(t, err)

		var createResult map[string]interface{}
		require.NoError(t, json.Unmarshal(createOutput, &createResult))
		isolationID := createResult["isolation_id"].(string)

		// Run reconcile
		reconcileCmd := exec.Command("/tmp/go-portalloc-test", "reconcile")
		reconcileCmd.Dir = tmpDir
		reconcileOutput, err := reconcileCmd.CombinedOutput()
		require.NoError(t, err)

		outputStr := string(reconcileOutput)
		assert.Contains(t, outputStr, "Reconciling state")
		assert.Contains(t, outputStr, "State file updated")

		// Verify environment still exists after reconcile
		listCmd := exec.Command("/tmp/go-portalloc-test", "list", "--format", "json")
		listCmd.Dir = tmpDir
		listOutput, err := listCmd.CombinedOutput()
		require.NoError(t, err)

		var listResult []map[string]interface{}
		require.NoError(t, json.Unmarshal(listOutput, &listResult))

		found := false
		for _, env := range listResult {
			if env["id"] == isolationID {
				found = true
				break
			}
		}
		assert.True(t, found, "environment should exist after reconcile")

		// Cleanup
		cleanupCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", isolationID)
		cleanupCmd.Dir = tmpDir
		_ = cleanupCmd.Run()
	})

	t.Run("cleanup --stale removes dead processes", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create multiple environments
		var isolationIDs []string
		for i := 0; i < 3; i++ {
			createCmd := exec.Command("/tmp/go-portalloc-test", "create", "--json")
			createCmd.Dir = tmpDir
			createOutput, err := createCmd.CombinedOutput()
			require.NoError(t, err)

			var createResult map[string]interface{}
			require.NoError(t, json.Unmarshal(createOutput, &createResult))
			isolationIDs = append(isolationIDs, createResult["isolation_id"].(string))
		}

		// All environments should be stale (created by different process)
		staleCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--stale")
		staleCmd.Dir = tmpDir
		staleOutput, err := staleCmd.CombinedOutput()
		require.NoError(t, err)

		outputStr := string(staleOutput)
		assert.Contains(t, outputStr, "stale environment(s)")
		assert.Contains(t, outputStr, "Cleaned up")

		// Verify all environments were cleaned
		listCmd := exec.Command("/tmp/go-portalloc-test", "list", "--format", "json")
		listCmd.Dir = tmpDir
		listOutput, err := listCmd.CombinedOutput()
		require.NoError(t, err)

		var listResult []map[string]interface{}
		if len(listOutput) > 0 && string(listOutput) != "No environments found\n" {
			require.NoError(t, json.Unmarshal(listOutput, &listResult))

			// Verify our test environments are not in the list
			for _, env := range listResult {
				for _, id := range isolationIDs {
					assert.NotEqual(t, id, env["id"], "stale environment should be cleaned")
				}
			}
		}
	})

	t.Run("list --reconcile forces state rebuild", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an environment
		createCmd := exec.Command("/tmp/go-portalloc-test", "create", "--json")
		createCmd.Dir = tmpDir
		createOutput, err := createCmd.CombinedOutput()
		require.NoError(t, err)

		var createResult map[string]interface{}
		require.NoError(t, json.Unmarshal(createOutput, &createResult))
		isolationID := createResult["isolation_id"].(string)

		// List with reconcile
		listCmd := exec.Command("/tmp/go-portalloc-test", "list", "--reconcile")
		listCmd.Dir = tmpDir
		listOutput, err := listCmd.CombinedOutput()
		require.NoError(t, err)

		outputStr := string(listOutput)
		assert.Contains(t, outputStr, isolationID)

		// Cleanup
		cleanupCmd := exec.Command("/tmp/go-portalloc-test", "cleanup", "--id", isolationID)
		cleanupCmd.Dir = tmpDir
		_ = cleanupCmd.Run()
	})
}
