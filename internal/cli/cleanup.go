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
	"time"

	"github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
	"github.com/pigeonworks-llc/go-portalloc/pkg/state"
	"github.com/spf13/cobra"
)

var (
	cleanupID        string
	cleanupAll       bool
	cleanupStale     bool
	cleanupOlderThan string
	cleanupWorktree  string
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
	cleanupCmd.Flags().BoolVar(&cleanupStale, "stale", false, "Cleanup only stale environments (dead processes)")
	cleanupCmd.Flags().StringVar(&cleanupOlderThan, "older-than", "", "Cleanup environments older than duration (e.g., 2h, 30m)")
	cleanupCmd.Flags().StringVarP(&cleanupWorktree, "worktree", "w", "", "Working directory path (current directory if not provided)")
	cleanupCmd.MarkFlagsMutuallyExclusive("id", "all", "stale")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	if cleanupID == "" && !cleanupAll && !cleanupStale {
		return fmt.Errorf("either --id, --all, or --stale must be specified")
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

	if cleanupStale {
		return cleanupStaleEnvironments(manager, config.LockDir)
	}

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
		Ports:        &ports.PortRange{BasePort: 0, Count: 0},
	}

	if err := manager.Cleanup(env); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	// Remove from state file (best effort)
	stateMgr, err := state.NewManager()
	if err == nil {
		_ = stateMgr.RemoveEnvironment(isolationID)
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

	// Create state manager
	stateMgr, err := state.NewManager()
	if err != nil {
		// Continue without state management
		stateMgr = nil
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
			Ports:        &ports.PortRange{BasePort: 0, Count: 0},
		}

		if err := manager.Cleanup(env); err != nil {
			fmt.Printf("⚠️  Failed to cleanup %s: %v\n", isolationID, err)
			failed++
		} else {
			// Remove from state
			if stateMgr != nil {
				_ = stateMgr.RemoveEnvironment(isolationID)
			}
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

func cleanupStaleEnvironments(manager *isolation.EnvironmentManager, lockDir string) error {
	// Create state manager
	stateMgr, err := state.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	// Reconcile to get latest state
	if _, err := stateMgr.Reconcile(lockDir); err != nil {
		return fmt.Errorf("failed to reconcile state: %w", err)
	}

	// List all environments
	envs, err := stateMgr.ListEnvironments()
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		fmt.Println("No environments to cleanup")
		return nil
	}

	// Parse older-than duration if specified
	var olderThan time.Duration
	if cleanupOlderThan != "" {
		olderThan, err = time.ParseDuration(cleanupOlderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than duration: %w", err)
		}
	}

	// Filter stale environments
	var toCleanup []*state.EnvironmentState
	for _, env := range envs {
		status := state.GetEnvironmentStatus(env)

		// Check if stale (process not running)
		isStale := status == state.StatusStale

		// Check if older than threshold
		isOld := false
		if olderThan > 0 {
			isOld = time.Since(env.CreatedAt) > olderThan
		}

		// Include if stale OR old (depending on flags)
		if cleanupOlderThan != "" {
			// If --older-than is specified, cleanup old environments (regardless of stale status)
			if isOld {
				toCleanup = append(toCleanup, env)
			}
		} else {
			// Otherwise, cleanup only stale environments
			if isStale {
				toCleanup = append(toCleanup, env)
			}
		}
	}

	if len(toCleanup) == 0 {
		fmt.Println("No stale environments to cleanup")
		return nil
	}

	fmt.Printf("🧹 Found %d stale environment(s)\n", len(toCleanup))

	cleaned := 0
	failed := 0

	for _, env := range toCleanup {
		isoEnv := &isolation.Environment{
			ID:           env.ID,
			WorktreePath: env.WorktreePath,
			TempDir:      env.TempDir,
			LockFile:     env.LockFile,
			EnvFile:      env.EnvFile,
			Ports:        &ports.PortRange{BasePort: env.Ports.BasePort, Count: env.Ports.Count},
		}

		if err := manager.Cleanup(isoEnv); err != nil {
			fmt.Printf("⚠️  Failed to cleanup %s: %v\n", env.ID, err)
			failed++
		} else {
			reason := "process not found"
			if cleanupOlderThan != "" {
				reason = fmt.Sprintf("created %s ago", time.Since(env.CreatedAt).Round(time.Minute))
			}
			fmt.Printf("✅ Cleaned: %s (%s)\n", env.ID, reason)
			cleaned++

			// Remove from state
			_ = stateMgr.RemoveEnvironment(env.ID)
		}
	}

	fmt.Printf("\n✅ Cleaned up %d environment(s)", cleaned)
	if failed > 0 {
		fmt.Printf(" (%d failed)", failed)
	}
	fmt.Println()

	return nil
}
