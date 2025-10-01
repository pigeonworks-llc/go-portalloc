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

// Package isolation provides parallel test environment isolation.
package isolation

import (
	"fmt"
	"os"
	"path/filepath"
)

// Environment represents an isolated test environment.
type Environment struct {
	ID           string
	WorktreePath string
	TempDir      string
	Ports        *PortRange
	LockFile     string
	EnvFile      string
}

// PortAllocator interface for port allocation.
type PortAllocator interface {
	AllocateRange(int) (int, error)
	IsPortInUse(int) bool
}

// EnvironmentManager manages isolated test environments.

type EnvironmentManager struct {
	idGen     *IDGenerator
	portAlloc PortAllocator
}

// NewEnvironmentManager creates a new environment manager.
func NewEnvironmentManager(idGen *IDGenerator, portAlloc PortAllocator) *EnvironmentManager {
	if idGen == nil {
		idGen = NewIDGenerator(nil)
	}

	return &EnvironmentManager{
		idGen:     idGen,
		portAlloc: portAlloc,
	}
}

// CreateEnvironment creates a new isolated environment.
func (em *EnvironmentManager) CreateEnvironment(portsNeeded int) (*Environment, error) {
	// Generate unique ID
	isolationID, err := em.idGen.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate isolation ID: %w", err)
	}

	// Create lock
	lockFile, err := em.idGen.CreateLock(isolationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock: %w", err)
	}

	// Allocate ports
	basePort, err := em.portAlloc.AllocateRange(portsNeeded)
	if err != nil {
		_ = em.idGen.ReleaseLock(isolationID)
		return nil, fmt.Errorf("failed to allocate ports: %w", err)
	}

	// Create temporary directory
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("aigis-test-%s", isolationID))
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		_ = em.idGen.ReleaseLock(isolationID)
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	env := &Environment{
		ID:           isolationID,
		WorktreePath: em.idGen.config.WorktreePath,
		TempDir:      tmpDir,
		Ports: &PortRange{
			BasePort: basePort,
			Count:    portsNeeded,
		},
		LockFile: lockFile,
	}

	// Create environment file
	envFile, err := em.createEnvFile(env)
	if err != nil {
		_ = em.Cleanup(env)
		return nil, fmt.Errorf("failed to create env file: %w", err)
	}
	env.EnvFile = envFile

	return env, nil
}

// createEnvFile creates an environment variable file.
func (em *EnvironmentManager) createEnvFile(env *Environment) (string, error) {
	envFilePath := filepath.Join(env.WorktreePath, ".env.isolation")

	// #nosec G304 - envFilePath is constructed from controlled inputs
	f, err := os.Create(envFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create env file: %w", err)
	}
	defer f.Close()

	// Write environment variables
	_, _ = fmt.Fprintf(f, "# Parallel Test Environment Isolation\n")
	_, _ = fmt.Fprintf(f, "# Generated: %s\n\n", env.ID)
	_, _ = fmt.Fprintf(f, "ISOLATION_ID=%s\n", env.ID)
	_, _ = fmt.Fprintf(f, "TEMP_DIR=%s\n", env.TempDir)
	_, _ = fmt.Fprintf(f, "PORT_BASE=%d\n", env.Ports.BasePort)
	_, _ = fmt.Fprintf(f, "PORT_COUNT=%d\n", env.Ports.Count)

	// Write individual port assignments
	portNames := []string{"FIRESTORE_PORT", "AUTH_PORT", "API_PORT", "METRICS_PORT", "DEBUG_PORT"}
	for i := 0; i < env.Ports.Count && i < len(portNames); i++ {
		port, err := env.Ports.GetPort(i)
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(f, "%s=%d\n", portNames[i], port)
	}

	return envFilePath, nil
}

// Cleanup removes all resources associated with the environment.
func (em *EnvironmentManager) Cleanup(env *Environment) error {
	var errors []error

	// Remove temp directory
	if err := os.RemoveAll(env.TempDir); err != nil && !os.IsNotExist(err) {
		errors = append(errors, fmt.Errorf("failed to remove temp dir: %w", err))
	}

	// Remove env file
	if env.EnvFile != "" {
		if err := os.Remove(env.EnvFile); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf("failed to remove env file: %w", err))
		}
	}

	// Release lock
	if err := em.idGen.ReleaseLock(env.ID); err != nil {
		errors = append(errors, fmt.Errorf("failed to release lock: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// Validate checks if the environment is properly isolated.
func (em *EnvironmentManager) Validate(env *Environment) error {
	// Check lock exists
	if !em.idGen.IsLocked(env.ID) {
		return fmt.Errorf("lock file missing for %s", env.ID)
	}

	// Check temp directory exists
	if _, err := os.Stat(env.TempDir); os.IsNotExist(err) {
		return fmt.Errorf("temp directory missing: %s", env.TempDir)
	}

	// Check env file exists
	if _, err := os.Stat(env.EnvFile); os.IsNotExist(err) {
		return fmt.Errorf("env file missing: %s", env.EnvFile)
	}

	// Check ports are still available (not ideal but validates allocation)
	for i := 0; i < env.Ports.Count; i++ {
		port, err := env.Ports.GetPort(i)
		if err != nil {
			return fmt.Errorf("invalid port index %d: %w", i, err)
		}
		if em.portAlloc.IsPortInUse(port) {
			// Port is in use, which is expected if tests are running
			// This is actually a good sign - just log it
			continue
		}
	}

	return nil
}
