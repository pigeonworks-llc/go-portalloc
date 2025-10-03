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

// Package main demonstrates custom port allocator configuration.
//
// This example shows how to:
//   - Configure custom port ranges
//   - Adjust retry behavior
//   - Use PortRange utilities
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
)

func main() {
	fmt.Println("=== Custom Configuration Example ===")

	// Example 1: Custom port range
	fmt.Println("1. Creating allocator with custom port range (10000-15000)...")
	config1 := &ports.AllocatorConfig{
		StartPort:  10000,
		EndPort:    15000,
		MaxRetries: 10,
		RetryDelay: 1 * time.Second,
	}
	allocator1 := ports.NewAllocator(config1)
	basePort1, err := allocator1.AllocateRange(5)
	if err != nil {
		log.Fatal("Failed to allocate ports:", err)
	}
	fmt.Printf("   ✓ Allocated ports in custom range: %d-%d\n\n", basePort1, basePort1+4)

	// Example 2: Aggressive retry configuration
	fmt.Println("2. Creating allocator with aggressive retry settings...")
	config2 := &ports.AllocatorConfig{
		StartPort:  20000,
		EndPort:    30000,
		MaxRetries: 50,
		RetryDelay: 100 * time.Millisecond, // Faster retries
	}
	allocator2 := ports.NewAllocator(config2)
	basePort2, err := allocator2.AllocateRange(10)
	if err != nil {
		log.Fatal("Failed to allocate ports:", err)
	}
	fmt.Printf("   ✓ Allocated 10 ports with fast retry: %d-%d\n\n", basePort2, basePort2+9)

	// Example 3: Using PortRange utilities
	fmt.Println("3. Using PortRange utilities...")
	portRange := &ports.PortRange{
		BasePort: basePort2,
		Count:    10,
	}

	// Get all ports as slice
	allPorts := portRange.Ports()
	fmt.Printf("   All ports: %v\n", allPorts)

	// Get specific ports by index
	apiPort, _ := portRange.GetPort(0)
	dbPort, _ := portRange.GetPort(1)
	cachePort, _ := portRange.GetPort(2)
	fmt.Printf("   API Port:   %d (index 0)\n", apiPort)
	fmt.Printf("   DB Port:    %d (index 1)\n", dbPort)
	fmt.Printf("   Cache Port: %d (index 2)\n\n", cachePort)

	// Example 4: Conservative configuration for production
	fmt.Println("4. Creating allocator with conservative settings...")
	config3 := &ports.AllocatorConfig{
		StartPort:  40000,
		EndPort:    50000,
		MaxRetries: 100,
		RetryDelay: 2 * time.Second, // Longer delays between retries
	}
	allocator3 := ports.NewAllocator(config3)
	basePort3, err := allocator3.AllocateRange(3)
	if err != nil {
		log.Fatal("Failed to allocate ports:", err)
	}
	fmt.Printf("   ✓ Allocated ports with conservative config: %d-%d\n\n", basePort3, basePort3+2)

	// Example 5: Boundary checking with GetPort
	fmt.Println("5. Demonstrating boundary checking...")
	pr := &ports.PortRange{BasePort: 25000, Count: 5}

	// Valid access
	port, err := pr.GetPort(0)
	if err == nil {
		fmt.Printf("   ✓ GetPort(0) = %d (valid)\n", port)
	}

	// Out of bounds access
	_, err = pr.GetPort(10)
	if err != nil {
		fmt.Printf("   ✓ GetPort(10) = error: %v (expected)\n", err)
	}

	// Negative index
	_, err = pr.GetPort(-1)
	if err != nil {
		fmt.Printf("   ✓ GetPort(-1) = error: %v (expected)\n\n", err)
	}

	fmt.Println("=== Example completed successfully ===")
}
