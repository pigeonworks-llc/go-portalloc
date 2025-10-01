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
	"fmt"
	"os"
	"path/filepath"

	"github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
	"github.com/spf13/cobra"
)

var (
	cleanupID       string
	cleanupAll      bool
	cleanupWorktree string
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup an isolated test environment",
	Long: `Cleanup removes all resources associated with an isolated test environment.

This command:
  1. Removes the temporary directory
  2. Removes the environment variable file
  3. Releases the lock file

All cleanup operations are safe and idempotent.`,
	Example: `  # Cleanup specific environment by ID
  go-portalloc cleanup --id abc123def456

  # Cleanup all environments in current worktree
  go-portalloc cleanup --all

  # Cleanup all environments in specific worktree
  go-portalloc cleanup --all --worktree /path/to/project`,
	RunE: runCleanup,
}

func init() {
	cleanupCmd.Flags().StringVar(&cleanupID, "id", "", "Isolation ID to cleanup")
	cleanupCmd.Flags().BoolVar(&cleanupAll, "all", false, "Cleanup all environments")
	cleanupCmd.Flags().StringVarP(&cleanupWorktree, "worktree", "w", "", "Working directory path (current directory if not provided)")
	cleanupCmd.MarkFlagsMutuallyExclusive("id", "all")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	if cleanupID == "" && !cleanupAll {
		return fmt.Errorf("either --id or --all must be specified")
	}

	// Prepare configuration
	worktree := cleanupWorktree
	if worktree == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		worktree = wd
	}

	config := &isolation.Config{
		WorktreePath: worktree,
		LockDir:      filepath.Join(os.TempDir(), "go-portalloc-locks"),
	}

	idGen := isolation.NewIDGenerator(config)
	manager := isolation.NewEnvironmentManager(idGen, nil)

	if cleanupAll {
		return cleanupAllEnvironments(manager, config.LockDir)
	}

	return cleanupSingleEnvironment(manager, cleanupID, config)
}

func cleanupSingleEnvironment(manager *isolation.EnvironmentManager, isolationID string, config *isolation.Config) error {
	// Reconstruct environment from ID
	lockFile := filepath.Join(config.LockDir, fmt.Sprintf("env-%s.lock", isolationID))
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("aigis-test-%s", isolationID))
	envFile := filepath.Join(config.WorktreePath, ".env.isolation")

	env := &isolation.Environment{
		ID:           isolationID,
		WorktreePath: config.WorktreePath,
		TempDir:      tmpDir,
		LockFile:     lockFile,
		EnvFile:      envFile,
		Ports:        &isolation.PortRange{BasePort: 0, Count: 0},
	}

	if err := manager.Cleanup(env); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	fmt.Printf("✅ Environment %s cleaned up successfully\n", isolationID)
	return nil
}

func cleanupAllEnvironments(manager *isolation.EnvironmentManager, lockDir string) error {
	// Find all lock files
	lockFiles, err := filepath.Glob(filepath.Join(lockDir, "env-*.lock"))
	if err != nil {
		return fmt.Errorf("failed to find lock files: %w", err)
	}

	if len(lockFiles) == 0 {
		fmt.Println("No environments to cleanup")
		return nil
	}

	cleaned := 0
	failed := 0

	for _, lockFile := range lockFiles {
		// Extract isolation ID from lock file name
		base := filepath.Base(lockFile)
		isolationID := base[4 : len(base)-5] // Remove "env-" prefix and ".lock" suffix

		tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("aigis-test-%s", isolationID))
		envFile := filepath.Join(cleanupWorktree, ".env.isolation")

		env := &isolation.Environment{
			ID:           isolationID,
			WorktreePath: cleanupWorktree,
			TempDir:      tmpDir,
			LockFile:     lockFile,
			EnvFile:      envFile,
			Ports:        &isolation.PortRange{BasePort: 0, Count: 0},
		}

		if err := manager.Cleanup(env); err != nil {
			fmt.Printf("⚠️  Failed to cleanup %s: %v\n", isolationID, err)
			failed++
		} else {
			cleaned++
		}
	}

	fmt.Printf("\n✅ Cleaned up %d environment(s)", cleaned)
	if failed > 0 {
		fmt.Printf(" (%d failed)", failed)
	}
	fmt.Println()

	return nil
}
