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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
)

// Manager handles state file operations with file locking.
type Manager struct {
	statePath string
	mu        sync.Mutex
}

// NewManager creates a new state manager.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	stateDir := filepath.Join(homeDir, ".go-portalloc")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	statePath := filepath.Join(stateDir, "state.json")

	return &Manager{
		statePath: statePath,
	}, nil
}

// lockFile locks the state file for exclusive access.
func (m *Manager) lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlockFile unlocks the state file.
func (m *Manager) unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// readState reads the state file (must be called with lock held).
func (m *Manager) readState(f *os.File) (*State, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat state file: %w", err)
	}

	// Empty file, return new state
	if stat.Size() == 0 {
		return &State{
			Version:      CurrentVersion,
			Environments: []*EnvironmentState{},
		}, nil
	}

	var state State
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode state file: %w", err)
	}

	return &state, nil
}

// writeState writes the state file (must be called with lock held).
func (m *Manager) writeState(f *os.File, state *State) error {
	// Truncate file
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate state file: %w", err)
	}

	// Seek to beginning
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek to beginning: %w", err)
	}

	// Write JSON
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		return fmt.Errorf("failed to encode state: %w", err)
	}

	return f.Sync()
}

// RecordEnvironment records a new environment to the state file.
func (m *Manager) RecordEnvironment(env *isolation.Environment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Open state file
	f, err := os.OpenFile(m.statePath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open state file: %w", err)
	}
	defer f.Close()

	// Lock file
	if err := m.lockFile(f); err != nil {
		return fmt.Errorf("failed to lock state file: %w", err)
	}
	defer func() { _ = m.unlockFile(f) }()

	// Read current state
	state, err := m.readState(f)
	if err != nil {
		return err
	}

	// Add new environment
	envState := &EnvironmentState{
		ID:           env.ID,
		PID:          os.Getpid(),
		CreatedAt:    time.Now(),
		WorktreePath: env.WorktreePath,
		TempDir:      env.TempDir,
		LockFile:     env.LockFile,
		EnvFile:      env.EnvFile,
		Ports: &PortsState{
			BasePort:  env.Ports.BasePort,
			Count:     env.Ports.Count,
			Allocated: env.Ports.Ports(),
		},
	}

	// Check if environment already exists
	for i, existing := range state.Environments {
		if existing.ID == env.ID {
			// Update existing
			state.Environments[i] = envState
			return m.writeState(f, state)
		}
	}

	// Add new
	state.Environments = append(state.Environments, envState)

	return m.writeState(f, state)
}

// RemoveEnvironment removes an environment from the state file.
func (m *Manager) RemoveEnvironment(isolationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Open state file
	f, err := os.OpenFile(m.statePath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open state file: %w", err)
	}
	defer f.Close()

	// Lock file
	if err := m.lockFile(f); err != nil {
		return fmt.Errorf("failed to lock state file: %w", err)
	}
	defer func() { _ = m.unlockFile(f) }()

	// Read current state
	state, err := m.readState(f)
	if err != nil {
		return err
	}

	// Remove environment
	newEnvs := make([]*EnvironmentState, 0, len(state.Environments))
	for _, env := range state.Environments {
		if env.ID != isolationID {
			newEnvs = append(newEnvs, env)
		}
	}

	state.Environments = newEnvs

	return m.writeState(f, state)
}

// ListEnvironments lists all environments from the state file.
func (m *Manager) ListEnvironments() ([]*EnvironmentState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if state file exists
	if _, err := os.Stat(m.statePath); os.IsNotExist(err) {
		return []*EnvironmentState{}, nil
	}

	// Open state file
	f, err := os.OpenFile(m.statePath, os.O_RDONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open state file: %w", err)
	}
	defer f.Close()

	// Lock file (shared lock for reading)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("failed to lock state file: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	// Read state
	state, err := m.readState(f)
	if err != nil {
		return nil, err
	}

	return state.Environments, nil
}

// GetEnvironment gets a specific environment by ID.
func (m *Manager) GetEnvironment(isolationID string) (*EnvironmentState, error) {
	envs, err := m.ListEnvironments()
	if err != nil {
		return nil, err
	}

	for _, env := range envs {
		if env.ID == isolationID {
			return env, nil
		}
	}

	return nil, fmt.Errorf("environment %s not found", isolationID)
}
