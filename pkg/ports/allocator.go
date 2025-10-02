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
//
// This package offers concurrent-safe port allocation without external dependencies,
// using pure Go standard library. It's designed for test isolation and development
// environments where port conflicts must be avoided.
//
// Basic usage:
//
//	allocator := ports.NewAllocator(nil)
//	basePort, err := allocator.AllocateRange(5)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use ports: basePort, basePort+1, ..., basePort+4
//
// The allocator uses TCP listener tests to verify port availability,
// ensuring accurate detection without requiring system tools like lsof or netstat.
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
//
// Fields:
//   - StartPort: Lower bound of port range (inclusive, default: 20000)
//   - EndPort: Upper bound of port range (exclusive, default: 30000)
//   - MaxRetries: Maximum number of allocation attempts (default: 10)
//   - RetryDelay: Wait time between retries (default: 1s)
//
// Example custom configuration:
//
//	config := &AllocatorConfig{
//	    StartPort:  10000,
//	    EndPort:    20000,
//	    MaxRetries: 20,
//	    RetryDelay: 500 * time.Millisecond,
//	}
type AllocatorConfig struct {
	StartPort  int
	EndPort    int
	MaxRetries int
	RetryDelay time.Duration
}

// DefaultAllocatorConfig returns default configuration.
//
// Default values:
//   - StartPort: 20000
//   - EndPort: 30000
//   - MaxRetries: 10
//   - RetryDelay: 1 second
//
// This range (20000-30000) is chosen to avoid conflicts with:
//   - Well-known ports (0-1023)
//   - Registered ports (1024-49151)
//   - Most ephemeral port ranges (varies by OS, typically 32768-60999)
func DefaultAllocatorConfig() *AllocatorConfig {
	return &AllocatorConfig{
		StartPort:  DefaultStartPort,
		EndPort:    DefaultEndPort,
		MaxRetries: DefaultMaxRetries,
		RetryDelay: 1 * time.Second,
	}
}

// Allocator allocates available ports for test environments.
//
// The allocator is stateless and thread-safe. It checks port availability
// at allocation time using TCP listeners, ensuring accurate detection
// without caching or state management.
//
// Thread-safety: All methods are safe for concurrent use.
type Allocator struct {
	config *AllocatorConfig
}

// NewAllocator creates a new port allocator.
//
// If config is nil, DefaultAllocatorConfig() is used.
//
// Example with default config:
//
//	allocator := ports.NewAllocator(nil)
//
// Example with custom config:
//
//	config := &ports.AllocatorConfig{
//	    StartPort:  15000,
//	    EndPort:    25000,
//	    MaxRetries: 5,
//	    RetryDelay: 100 * time.Millisecond,
//	}
//	allocator := ports.NewAllocator(config)
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
//
// Parameters:
//   - portsNeeded: Number of consecutive ports to allocate (must be > 0)
//
// Returns:
//   - int: Base port number (subsequent ports are basePort+1, basePort+2, ...)
//   - error: Non-nil if allocation fails after MaxRetries attempts
//
// The method randomly selects a starting port within the configured range
// and verifies all requested ports are available. If any port in the range
// is unavailable, it retries with a different random starting point.
//
// Example:
//
//	basePort, err := allocator.AllocateRange(5)
//	// Allocated ports: basePort, basePort+1, basePort+2, basePort+3, basePort+4
//
// Thread-safety: Safe for concurrent use.
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
//
// Parameters:
//   - port: Port number to check
//
// Returns:
//   - bool: true if port is in use, false if available
//
// This method attempts to bind a TCP listener to the port.
// If binding succeeds, the port is available (returns false).
// If binding fails, the port is in use (returns true).
//
// Example:
//
//	if allocator.IsPortInUse(8080) {
//	    log.Println("Port 8080 is already in use")
//	}
//
// Thread-safety: Safe for concurrent use.
func (a *Allocator) IsPortInUse(port int) bool {
	return !a.isPortAvailable(port)
}

// AllocateSpecific attempts to allocate specific ports.
//
// Parameters:
//   - ports: Variable number of port numbers to check
//
// Returns:
//   - error: Non-nil if any port is unavailable (error includes list of unavailable ports)
//
// This method verifies all specified ports are available without actually
// reserving them. It's useful for pre-flight checks before starting services.
//
// Example:
//
//	err := allocator.AllocateSpecific(8080, 8081, 8082)
//	if err != nil {
//	    log.Fatal("Required ports unavailable:", err)
//	}
//
// Thread-safety: Safe for concurrent use.
// Note: This is a point-in-time check; ports may become unavailable
// immediately after this method returns.
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
//
// Fields:
//   - BasePort: Starting port number
//   - Count: Number of consecutive ports in the range
//
// The range includes ports: [BasePort, BasePort+1, ..., BasePort+Count-1]
//
// Example:
//
//	pr := &PortRange{BasePort: 23000, Count: 5}
//	// Represents ports: 23000, 23001, 23002, 23003, 23004
type PortRange struct {
	BasePort int
	Count    int
}

// Ports returns all ports in the range as a slice.
//
// Returns:
//   - []int: Slice containing all port numbers in the range
//
// Example:
//
//	pr := &PortRange{BasePort: 23000, Count: 3}
//	ports := pr.Ports() // [23000, 23001, 23002]
//
// The returned slice is a new allocation and can be modified without
// affecting the PortRange.
func (pr *PortRange) Ports() []int {
	ports := make([]int, pr.Count)
	for i := 0; i < pr.Count; i++ {
		ports[i] = pr.BasePort + i
	}
	return ports
}

// GetPort returns a specific port by index.
//
// Parameters:
//   - index: Zero-based index (0 returns BasePort, 1 returns BasePort+1, etc.)
//
// Returns:
//   - int: Port number at the specified index
//   - error: Non-nil if index is out of range [0, Count)
//
// Example:
//
//	pr := &PortRange{BasePort: 23000, Count: 5}
//	port, err := pr.GetPort(2) // Returns 23002
//
// This is safer than direct slice indexing as it validates the index bounds.
func (pr *PortRange) GetPort(index int) (int, error) {
	if index < 0 || index >= pr.Count {
		return 0, fmt.Errorf("index %d out of range [0,%d)", index, pr.Count)
	}
	return pr.BasePort + index, nil
}
