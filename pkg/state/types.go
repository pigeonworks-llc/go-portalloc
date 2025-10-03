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

// Package state provides state management for go-portalloc environments.
package state

import "time"

// State represents the entire state file structure.
type State struct {
	Version           string                `json:"version"`
	Environments      []*EnvironmentState   `json:"environments"`
	LastReconciledAt  time.Time             `json:"last_reconciled_at"`
}

// EnvironmentState represents a single environment's state.
type EnvironmentState struct {
	ID           string      `json:"id"`
	PID          int         `json:"pid"`
	CreatedAt    time.Time   `json:"created_at"`
	WorktreePath string      `json:"worktree_path"`
	TempDir      string      `json:"temp_dir"`
	LockFile     string      `json:"lock_file"`
	EnvFile      string      `json:"env_file"`
	Ports        *PortsState `json:"ports"`
}

// PortsState represents the port allocation state.
type PortsState struct {
	BasePort  int   `json:"base_port"`
	Count     int   `json:"count"`
	Allocated []int `json:"allocated"`
}

// EnvironmentStatus represents the status of an environment.
type EnvironmentStatus string

const (
	// StatusActive indicates the environment is active (process running).
	StatusActive EnvironmentStatus = "active"
	// StatusStale indicates the environment is stale (process not running).
	StatusStale EnvironmentStatus = "stale"
)

// CurrentVersion is the current version of the state file format.
const CurrentVersion = "1.0"
