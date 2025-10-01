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

// Package isolation provides unique ID generation for parallel test environment isolation.
package isolation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds configuration for isolation ID generation.
type Config struct {
	WorktreePath     string
	InstanceID       string
	LockDir          string
	MaxRetries       int
	CollisionBackoff time.Duration
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		WorktreePath:     "",
		InstanceID:       fmt.Sprintf("%d", time.Now().UnixNano()%10000000000),
		LockDir:          "/tmp/aigis-isolation-locks",
		MaxRetries:       999,
		CollisionBackoff: 1 * time.Millisecond,
	}
}

// IDGenerator generates unique isolation IDs with collision detection.
type IDGenerator struct {
	config *Config
}

// NewIDGenerator creates a new ID generator.
func NewIDGenerator(config *Config) *IDGenerator {
	if config == nil {
		config = DefaultConfig()
	}

	if config.WorktreePath == "" {
		wd, err := os.Getwd()
		if err != nil {
			config.WorktreePath = "."
		} else {
			config.WorktreePath = wd
		}
	}

	// Create lock directory
	_ = os.MkdirAll(config.LockDir, 0o750)

	return &IDGenerator{
		config: config,
	}
}

// randomInt64 generates a cryptographically secure random int64.
func randomInt64() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	// #nosec G115 - Converting random bytes to int64 for ID generation
	return int64(binary.BigEndian.Uint64(b[:])), nil
}

// Generate creates a unique isolation ID with collision avoidance.
func (g *IDGenerator) Generate() (string, error) {
	// Generate base hash from multiple entropy sources
	timestamp := time.Now().UnixNano()
	randomComponent, err := randomInt64()
	if err != nil {
		return "", fmt.Errorf("failed to generate random component: %w", err)
	}
	processID := os.Getpid()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	baseInput := fmt.Sprintf("%s-%s-%d-%d-%s-%d",
		g.config.WorktreePath,
		g.config.InstanceID,
		timestamp,
		randomComponent,
		hostname,
		processID,
	)

	hash := sha256.Sum256([]byte(baseInput))
	baseID := fmt.Sprintf("%x", hash[:6]) // 12 characters

	// Collision detection with exponential backoff
	counter := 0
	for counter < g.config.MaxRetries {
		isolationID := baseID
		if counter > 0 {
			// Add additional randomness for collision resolution
			additionalRandom, err := randomInt64()
			if err != nil {
				return "", fmt.Errorf("failed to generate collision resolution random: %w", err)
			}
			isolationID = fmt.Sprintf("%s%04d%03d", baseID, additionalRandom%10000, counter)
		}

		// Check for collisions
		lockFile := filepath.Join(g.config.LockDir, fmt.Sprintf("env-%s.lock", isolationID))
		tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("aigis-test-%s", isolationID))

		if !fileExists(lockFile) && !fileExists(tmpDir) {
			return isolationID, nil
		}

		counter++
		time.Sleep(g.config.CollisionBackoff)
	}

	return "", fmt.Errorf("unable to generate unique isolation ID after %d attempts", g.config.MaxRetries)
}

// CreateLock creates a lock file for the isolation ID.
func (g *IDGenerator) CreateLock(isolationID string) (string, error) {
	lockFile := filepath.Join(g.config.LockDir, fmt.Sprintf("env-%s.lock", isolationID))

	// Atomic file creation (fails if exists)
	// #nosec G302 - 0o600 is appropriate for lock files
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to create lock: %w", err)
	}
	defer f.Close()

	// Write metadata
	metadata := fmt.Sprintf("PID=%d\nTimestamp=%d\nWorktree=%s\n",
		os.Getpid(),
		time.Now().Unix(),
		g.config.WorktreePath,
	)
	_, err = f.WriteString(metadata)
	if err != nil {
		_ = os.Remove(lockFile)
		return "", fmt.Errorf("failed to write lock metadata: %w", err)
	}

	return lockFile, nil
}

// ReleaseLock removes the lock file.
func (g *IDGenerator) ReleaseLock(isolationID string) error {
	lockFile := filepath.Join(g.config.LockDir, fmt.Sprintf("env-%s.lock", isolationID))
	if err := os.Remove(lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}

// IsLocked checks if an isolation ID is currently locked.
func (g *IDGenerator) IsLocked(isolationID string) bool {
	lockFile := filepath.Join(g.config.LockDir, fmt.Sprintf("env-%s.lock", isolationID))
	return fileExists(lockFile)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
