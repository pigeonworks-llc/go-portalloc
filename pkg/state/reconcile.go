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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Reconcile rebuilds the state file from lock files.
func (m *Manager) Reconcile(lockDir string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Scan lock files
	lockFiles, err := filepath.Glob(filepath.Join(lockDir, "env-*.lock"))
	if err != nil {
		return 0, fmt.Errorf("failed to scan lock files: %w", err)
	}

	// Build new state
	newState := &State{
		Version:          CurrentVersion,
		Environments:     make([]*EnvironmentState, 0, len(lockFiles)),
		LastReconciledAt: time.Now(),
	}

	for _, lockFile := range lockFiles {
		envState, err := m.parseLockFile(lockFile)
		if err != nil {
			// Skip invalid lock files
			continue
		}

		newState.Environments = append(newState.Environments, envState)
	}

	// Write new state
	f, err := os.OpenFile(m.statePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("failed to open state file: %w", err)
	}
	defer f.Close()

	if err := m.lockFile(f); err != nil {
		return 0, fmt.Errorf("failed to lock state file: %w", err)
	}
	defer m.unlockFile(f)

	if err := m.writeState(f, newState); err != nil {
		return 0, err
	}

	return len(newState.Environments), nil
}

// parseLockFile parses a lock file and returns an EnvironmentState.
func (m *Manager) parseLockFile(lockFile string) (*EnvironmentState, error) {
	// Extract isolation ID from lock file name
	base := filepath.Base(lockFile)
	if !strings.HasPrefix(base, "env-") || !strings.HasSuffix(base, ".lock") {
		return nil, fmt.Errorf("invalid lock file name: %s", base)
	}
	isolationID := base[4 : len(base)-5] // Remove "env-" prefix and ".lock" suffix

	// Read lock file metadata
	f, err := os.Open(lockFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}
	defer f.Close()

	metadata := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			metadata[parts[0]] = parts[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	// Parse metadata
	pid, _ := strconv.Atoi(metadata["PID"])
	timestamp, _ := strconv.ParseInt(metadata["Timestamp"], 10, 64)
	worktree := metadata["Worktree"]

	// Reconstruct paths
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("aigis-test-%s", isolationID))
	envFile := filepath.Join(worktree, ".env.isolation")

	// Try to read port information from env file
	ports := m.parseEnvFile(envFile)

	return &EnvironmentState{
		ID:           isolationID,
		PID:          pid,
		CreatedAt:    time.Unix(timestamp, 0),
		WorktreePath: worktree,
		TempDir:      tmpDir,
		LockFile:     lockFile,
		EnvFile:      envFile,
		Ports:        ports,
	}, nil
}

// parseEnvFile attempts to parse port information from an env file.
func (m *Manager) parseEnvFile(envFile string) *PortsState {
	f, err := os.Open(envFile)
	if err != nil {
		return &PortsState{}
	}
	defer f.Close()

	ports := &PortsState{
		Allocated: []int{},
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PORT_BASE=") {
			if val, err := strconv.Atoi(strings.TrimPrefix(line, "PORT_BASE=")); err == nil {
				ports.BasePort = val
			}
		} else if strings.HasPrefix(line, "PORT_COUNT=") {
			if val, err := strconv.Atoi(strings.TrimPrefix(line, "PORT_COUNT=")); err == nil {
				ports.Count = val
			}
		}
	}

	// Reconstruct allocated ports
	if ports.BasePort > 0 && ports.Count > 0 {
		for i := 0; i < ports.Count; i++ {
			ports.Allocated = append(ports.Allocated, ports.BasePort+i)
		}
	}

	return ports
}

// IsProcessRunning checks if a process is running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Send signal 0 to check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, signal 0 can be used to check process existence
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetEnvironmentStatus returns the status of an environment.
func GetEnvironmentStatus(env *EnvironmentState) EnvironmentStatus {
	if IsProcessRunning(env.PID) {
		return StatusActive
	}
	return StatusStale
}
