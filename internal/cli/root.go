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

// Package cli provides the command-line interface for go-portalloc.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is set during build time
	Version = "dev"

	rootCmd = &cobra.Command{
		Use:   "go-portalloc",
		Short: "Dynamic port allocator for parallel test execution",
		Long: `go-portalloc provides collision-free port allocation for parallel test execution.

Features:
  - Dynamic port allocation with automatic conflict resolution (20000-30000 range)
  - SHA256-based unique environment ID generation
  - Atomic locking mechanism for concurrent safety
  - Automatic cleanup with idempotent operations
  - Zero external dependencies (pure Go standard library)

Example:
  # Allocate 5 consecutive ports
  go-portalloc create --ports 5

  # Validate allocated environment
  go-portalloc validate --id <isolation-id>

  # Cleanup allocated resources
  go-portalloc cleanup --id <isolation-id>`,
		Version: Version,
	}
)

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(reconcileCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("go-portalloc version %s\n", Version)
	},
}
