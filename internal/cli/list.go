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
	"strings"
	"time"

	"github.com/pigeonworks-llc/go-portalloc/pkg/state"
	"github.com/spf13/cobra"
)

var (
	listFormat     string
	listLockDir    string
	listReconcile  bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environments",
	Long: `List all active and stale environments.

This command displays all environments currently tracked by go-portalloc.
It shows the environment ID, status (active/stale), allocated ports,
creation time, process ID, and worktree path.`,
	Example: `  # List all environments in table format
  go-portalloc list

  # List in JSON format
  go-portalloc list --format json

  # Force reconcile before listing
  go-portalloc list --reconcile`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listFormat, "format", "table", "Output format (table, json)")
	listCmd.Flags().StringVar(&listLockDir, "lock-dir", filepath.Join(os.TempDir(), "go-portalloc-locks"), "Lock directory path")
	listCmd.Flags().BoolVar(&listReconcile, "reconcile", false, "Force reconcile before listing")
}

func runList(cmd *cobra.Command, args []string) error {
	// Create state manager
	mgr, err := state.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	// Reconcile if requested
	if listReconcile {
		if _, err := mgr.Reconcile(listLockDir); err != nil {
			return fmt.Errorf("failed to reconcile state: %w", err)
		}
	}

	// List environments
	envs, err := mgr.ListEnvironments()
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		fmt.Println("No environments found")
		return nil
	}

	// Output based on format
	switch listFormat {
	case "json":
		return outputListJSON(envs)
	case "table":
		return outputListTable(envs)
	default:
		return fmt.Errorf("unknown format: %s", listFormat)
	}
}

func outputListJSON(envs []*state.EnvironmentState) error {
	output := make([]map[string]interface{}, 0, len(envs))

	for _, env := range envs {
		status := state.GetEnvironmentStatus(env)
		output = append(output, map[string]interface{}{
			"id":            env.ID,
			"status":        status,
			"pid":           env.PID,
			"created_at":    env.CreatedAt.Format(time.RFC3339),
			"worktree_path": env.WorktreePath,
			"temp_dir":      env.TempDir,
			"lock_file":     env.LockFile,
			"env_file":      env.EnvFile,
			"ports": map[string]interface{}{
				"base_port": env.Ports.BasePort,
				"count":     env.Ports.Count,
				"allocated": env.Ports.Allocated,
			},
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputListTable(envs []*state.EnvironmentState) error {
	// Print header
	fmt.Printf("%-15s %-8s %-15s %-20s %-8s %s\n",
		"ID", "STATUS", "PORTS", "CREATED", "PID", "WORKTREE")
	fmt.Println(strings.Repeat("-", 120))

	// Print environments
	for _, env := range envs {
		status := state.GetEnvironmentStatus(env)
		statusStr := string(status)
		if status == state.StatusStale {
			statusStr = statusStr + " ⚠️"
		}

		// Format ports
		portsStr := "-"
		if env.Ports != nil && len(env.Ports.Allocated) > 0 {
			if len(env.Ports.Allocated) > 1 {
				portsStr = fmt.Sprintf("%d-%d",
					env.Ports.Allocated[0],
					env.Ports.Allocated[len(env.Ports.Allocated)-1])
			} else {
				portsStr = fmt.Sprintf("%d", env.Ports.Allocated[0])
			}
		}

		// Format created time
		createdStr := formatTimeAgo(env.CreatedAt)

		// Format PID
		pidStr := fmt.Sprintf("%d", env.PID)
		if status == state.StatusStale {
			pidStr = "-"
		}

		// Truncate worktree if too long
		worktree := env.WorktreePath
		if len(worktree) > 40 {
			worktree = "..." + worktree[len(worktree)-37:]
		}

		fmt.Printf("%-15s %-8s %-15s %-20s %-8s %s\n",
			truncate(env.ID, 15),
			statusStr,
			portsStr,
			createdStr,
			pidStr,
			worktree)
	}

	fmt.Printf("\nTotal: %d environment(s)\n", len(envs))

	return nil
}

func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", minutes)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	default:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
