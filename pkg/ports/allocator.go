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

// Package ports provides dynamic port allocation for parallel test environments.
package ports

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	// DefaultStartPort is the default starting port for allocation
	DefaultStartPort = 20000
	// DefaultEndPort is the default ending port for allocation
	DefaultEndPort = 30000
	// DefaultMaxRetries is the default number of allocation retries
	DefaultMaxRetries = 10
)

// AllocatorConfig holds configuration for port allocation.
type AllocatorConfig struct {
	StartPort  int
	EndPort    int
	MaxRetries int
	RetryDelay time.Duration
}

// DefaultAllocatorConfig returns default configuration.
func DefaultAllocatorConfig() *AllocatorConfig {
	return &AllocatorConfig{
		StartPort:  DefaultStartPort,
		EndPort:    DefaultEndPort,
		MaxRetries: DefaultMaxRetries,
		RetryDelay: 1 * time.Second,
	}
}

// Allocator allocates available ports for test environments.
type Allocator struct {
	config *AllocatorConfig
}

// NewAllocator creates a new port allocator.
func NewAllocator(config *AllocatorConfig) *Allocator {
	if config == nil {
		config = DefaultAllocatorConfig()
	}

	return &Allocator{
		config: config,
	}
}

// randomIntn generates a cryptographically secure random integer in range [0, n).
func randomIntn(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("invalid range: n must be positive")
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	val := binary.BigEndian.Uint64(b[:])
	// #nosec G115 - Modulo operation ensures result fits in int range
	return int(val % uint64(n)), nil
}

// AllocateRange allocates a range of consecutive available ports.
func (a *Allocator) AllocateRange(portsNeeded int) (int, error) {
	if portsNeeded <= 0 {
		return 0, fmt.Errorf("portsNeeded must be positive, got %d", portsNeeded)
	}

	portRange := a.config.EndPort - a.config.StartPort - portsNeeded
	if portRange <= 0 {
		return 0, fmt.Errorf("insufficient port range for %d ports", portsNeeded)
	}

	for attempt := 0; attempt < a.config.MaxRetries; attempt++ {
		// Random starting point to reduce collision probability
		offset, err := randomIntn(portRange)
		if err != nil {
			return 0, fmt.Errorf("failed to generate random offset: %w", err)
		}
		basePort := a.config.StartPort + offset

		// Check if all required ports are available
		if a.arePortsAvailable(basePort, portsNeeded) {
			return basePort, nil
		}

		// Wait before retry
		time.Sleep(a.config.RetryDelay)
	}

	return 0, fmt.Errorf("unable to allocate %d consecutive ports after %d attempts", portsNeeded, a.config.MaxRetries)
}

// arePortsAvailable checks if a range of ports is available.
func (a *Allocator) arePortsAvailable(basePort, count int) bool {
	for i := 0; i < count; i++ {
		port := basePort + i
		if !a.isPortAvailable(port) {
			return false
		}
	}
	return true
}

// isPortAvailable checks if a specific port is available.
func (a *Allocator) isPortAvailable(port int) bool {
	// Try to bind to the port
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

// IsPortInUse checks if a port is currently in use.
func (a *Allocator) IsPortInUse(port int) bool {
	return !a.isPortAvailable(port)
}

// AllocateSpecific attempts to allocate specific ports.
func (a *Allocator) AllocateSpecific(ports ...int) error {
	unavailable := []int{}

	for _, port := range ports {
		if !a.isPortAvailable(port) {
			unavailable = append(unavailable, port)
		}
	}

	if len(unavailable) > 0 {
		return fmt.Errorf("ports unavailable: %v", unavailable)
	}

	return nil
}

// PortRange represents an allocated range of ports.
type PortRange struct {
	BasePort int
	Count    int
}

// Ports returns all ports in the range.
func (pr *PortRange) Ports() []int {
	ports := make([]int, pr.Count)
	for i := 0; i < pr.Count; i++ {
		ports[i] = pr.BasePort + i
	}
	return ports
}

// GetPort returns a specific port by index.
func (pr *PortRange) GetPort(index int) (int, error) {
	if index < 0 || index >= pr.Count {
		return 0, fmt.Errorf("index %d out of range [0,%d)", index, pr.Count)
	}
	return pr.BasePort + index, nil
}
