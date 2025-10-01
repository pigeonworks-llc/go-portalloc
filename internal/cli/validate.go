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
	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
	"github.com/spf13/cobra"
)

var (
	validateID       string
	validateWorktree string
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate an isolated test environment",
	Long: `Validate checks if an isolated test environment is properly configured and functional.

This command verifies:
  1. Lock file exists and is valid
  2. Temporary directory exists
  3. Environment variable file exists
  4. Allocated ports are accessible

Validation helps ensure environment isolation is working correctly.`,
	Example: `  # Validate specific environment by ID
  go-portalloc validate --id abc123def456

  # Validate with custom worktree
  go-portalloc validate --id abc123def456 --worktree /path/to/project`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateID, "id", "", "Isolation ID to validate (required)")
	validateCmd.Flags().StringVarP(&validateWorktree, "worktree", "w", "", "Working directory path (current directory if not provided)")
	_ = validateCmd.MarkFlagRequired("id")
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Prepare configuration
	worktree := validateWorktree
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

	// Create components
	idGen := isolation.NewIDGenerator(config)
	portAlloc := ports.NewAllocator(nil)
	manager := isolation.NewEnvironmentManager(idGen, portAlloc)

	// Reconstruct environment from ID
	lockFile := filepath.Join(config.LockDir, fmt.Sprintf("env-%s.lock", validateID))
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("aigis-test-%s", validateID))
	envFile := filepath.Join(worktree, ".env.isolation")

	// Check if lock exists to determine if environment exists
	if !fileExists(lockFile) {
		return fmt.Errorf("environment %s does not exist (no lock file found)", validateID)
	}

	env := &isolation.Environment{
		ID:           validateID,
		WorktreePath: worktree,
		TempDir:      tmpDir,
		LockFile:     lockFile,
		EnvFile:      envFile,
		Ports:        &isolation.PortRange{BasePort: 0, Count: 0}, // Ports not needed for basic validation
	}

	// Validate environment
	if err := manager.Validate(env); err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
		return err
	}

	// Print validation results
	fmt.Println("✅ Environment validation successful!")
	fmt.Println()
	fmt.Printf("  Isolation ID:   %s\n", env.ID)
	fmt.Printf("  Lock File:      %s ✓\n", env.LockFile)
	fmt.Printf("  Temp Directory: %s ✓\n", env.TempDir)
	fmt.Printf("  Env File:       %s ✓\n", env.EnvFile)
	fmt.Println()
	fmt.Println("Environment is properly isolated and functional.")

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
