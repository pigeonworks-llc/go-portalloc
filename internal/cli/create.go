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
	"fmt"
	"os"
	"path/filepath"

	"github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
	"github.com/spf13/cobra"
)

var (
	createPortsCount  int
	createInstanceID  string
	createWorktree    string
	createOutputJSON  bool
	createOutputShell bool
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an isolated test environment",
	Long: `Create a new isolated test environment with unique ID, allocated ports, and temporary directory.

This command:
  1. Generates a collision-resistant unique isolation ID
  2. Allocates the specified number of consecutive available ports
  3. Creates a temporary directory for the environment
  4. Generates an atomic lock file
  5. Creates an environment variable file (.env.isolation)

The environment is guaranteed to be isolated from other concurrent environments.`,
	Example: `  # Create environment with 5 ports
  go-portalloc create --ports 5

  # Create with custom instance ID
  go-portalloc create --ports 3 --instance-id ci-build-123

  # Output as JSON for programmatic use
  go-portalloc create --ports 5 --json

  # Output as shell eval format
  go-portalloc create --ports 5 --shell`,
	RunE: runCreate,
}

func init() {
	createCmd.Flags().IntVarP(&createPortsCount, "ports", "p", 5, "Number of ports to allocate")
	createCmd.Flags().StringVarP(&createInstanceID, "instance-id", "i", "", "Custom instance ID (auto-generated if not provided)")
	createCmd.Flags().StringVarP(&createWorktree, "worktree", "w", "", "Working directory path (current directory if not provided)")
	createCmd.Flags().BoolVar(&createOutputJSON, "json", false, "Output environment details as JSON")
	createCmd.Flags().BoolVar(&createOutputShell, "shell", false, "Output as shell eval format (eval \"$(go-portalloc create --shell)\")")
}

func runCreate(cmd *cobra.Command, args []string) error {
	// Prepare configuration
	worktree := createWorktree
	if worktree == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		worktree = wd
	}

	config := &isolation.Config{
		WorktreePath: worktree,
		InstanceID:   createInstanceID,
		LockDir:      filepath.Join(os.TempDir(), "go-portalloc-locks"),
		MaxRetries:   999,
	}

	// Create components
	idGen := isolation.NewIDGenerator(config)
	portAlloc := ports.NewAllocator(nil)
	manager := isolation.NewEnvironmentManager(idGen, portAlloc)

	// Create environment
	env, err := manager.CreateEnvironment(createPortsCount)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	// Output based on format
	switch {
	case createOutputJSON:
		return outputJSON(env)
	case createOutputShell:
		return outputShell(env)
	default:
		return outputHuman(env)
	}
}

func outputJSON(env *isolation.Environment) error {
	output := map[string]interface{}{
		"isolation_id":         env.ID,
		"compose_project_name": fmt.Sprintf("portalloc-%s", env.ID),
		"worktree_path":        env.WorktreePath,
		"temp_dir":             env.TempDir,
		"lock_file":            env.LockFile,
		"env_file":             env.EnvFile,
		"ports": map[string]interface{}{
			"base_port": env.Ports.BasePort,
			"count":     env.Ports.Count,
			"ports":     env.Ports.Ports(),
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputShell(env *isolation.Environment) error {
	fmt.Printf("export ISOLATION_ID=%s\n", env.ID)
	fmt.Printf("export COMPOSE_PROJECT_NAME=portalloc-%s\n", env.ID)
	fmt.Printf("export TEMP_DIR=%s\n", env.TempDir)
	fmt.Printf("export PORT_BASE=%d\n", env.Ports.BasePort)
	fmt.Printf("export PORT_COUNT=%d\n", env.Ports.Count)

	portNames := []string{"FIRESTORE_PORT", "AUTH_PORT", "API_PORT", "METRICS_PORT", "DEBUG_PORT"}
	for i := 0; i < env.Ports.Count && i < len(portNames); i++ {
		port, err := env.Ports.GetPort(i)
		if err != nil {
			continue
		}
		fmt.Printf("export %s=%d\n", portNames[i], port)
	}

	return nil
}

func outputHuman(env *isolation.Environment) error {
	fmt.Println("✅ Environment created successfully!")
	fmt.Println()
	fmt.Printf("  Isolation ID:  %s\n", env.ID)
	fmt.Printf("  Temp Directory: %s\n", env.TempDir)
	fmt.Printf("  Lock File:      %s\n", env.LockFile)
	fmt.Printf("  Env File:       %s\n", env.EnvFile)
	fmt.Println()
	fmt.Printf("  Base Port:      %d\n", env.Ports.BasePort)
	fmt.Printf("  Port Count:     %d\n", env.Ports.Count)
	fmt.Printf("  Allocated Ports: %v\n", env.Ports.Ports())
	fmt.Println()
	fmt.Println("To use this environment:")
	fmt.Printf("  source %s\n", env.EnvFile)
	fmt.Println()
	fmt.Println("To cleanup:")
	fmt.Printf("  go-portalloc cleanup --id %s\n", env.ID)

	return nil
}
