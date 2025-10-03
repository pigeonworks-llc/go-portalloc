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

	"github.com/pigeonworks-llc/go-portalloc/pkg/state"
	"github.com/spf13/cobra"
)

var reconcileLockDir string

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile state file from lock files",
	Long: `Reconcile rebuilds the state file by scanning all lock files.

This command is useful when the state file is corrupted or out of sync
with the actual lock files. It will scan all lock files in the lock
directory and rebuild the state file from scratch.

The reconcile operation is safe and idempotent.`,
	Example: `  # Reconcile state file
  go-portalloc reconcile

  # Reconcile with custom lock directory
  go-portalloc reconcile --lock-dir /custom/path/locks`,
	RunE: runReconcile,
}

func init() {
	reconcileCmd.Flags().StringVar(&reconcileLockDir, "lock-dir", filepath.Join(os.TempDir(), "go-portalloc-locks"), "Lock directory path")
}

func runReconcile(cmd *cobra.Command, args []string) error {
	// Create state manager
	mgr, err := state.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	fmt.Println("🔄 Reconciling state...")

	// Reconcile
	count, err := mgr.Reconcile(reconcileLockDir)
	if err != nil {
		return fmt.Errorf("reconcile failed: %w", err)
	}

	fmt.Printf("✅ Found %d active environment(s)\n", count)

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		stateFile := filepath.Join(homeDir, ".go-portalloc", "state.json")
		fmt.Printf("✅ State file updated: %s\n", stateFile)
	}

	return nil
}
