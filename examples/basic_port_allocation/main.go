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

// Package main demonstrates basic port allocation using go-portalloc.
//
// This example shows how to:
//   - Create a simple port allocator
//   - Allocate a range of consecutive ports
//   - Check if specific ports are in use
//   - Verify port availability
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
)

func main() {
	fmt.Println("=== Basic Port Allocation Example ===\n")

	// Create allocator with default config (ports 20000-30000)
	allocator := ports.NewAllocator(nil)

	// Example 1: Allocate a range of consecutive ports
	fmt.Println("1. Allocating 5 consecutive ports...")
	basePort, err := allocator.AllocateRange(5)
	if err != nil {
		log.Fatal("Failed to allocate ports:", err)
	}
	fmt.Printf("   ✓ Allocated ports: %d-%d\n\n", basePort, basePort+4)

	// Example 2: Check if a specific port is in use
	fmt.Println("2. Checking if port 8080 is in use...")
	if allocator.IsPortInUse(8080) {
		fmt.Println("   ✓ Port 8080 is already in use")
	} else {
		fmt.Println("   ✓ Port 8080 is available")
	}

	// Example 3: Verify specific ports are available
	fmt.Println("\n3. Verifying ports 3000, 3001, 3002 are available...")
	err = allocator.AllocateSpecific(3000, 3001, 3002)
	if err != nil {
		fmt.Printf("   ✗ Some ports unavailable: %v\n", err)
	} else {
		fmt.Println("   ✓ All specified ports are available")
	}

	// Example 4: Use allocated ports with actual servers
	fmt.Printf("\n4. Starting test servers on allocated ports %d-%d...\n", basePort, basePort+2)

	// Start 3 test servers on allocated ports
	var listeners []net.Listener
	for i := 0; i < 3; i++ {
		port := basePort + i
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			log.Fatalf("Failed to start server on port %d: %v", port, err)
		}
		listeners = append(listeners, listener)
		fmt.Printf("   ✓ Server started on port %d\n", port)
	}

	// Keep servers running briefly
	fmt.Println("\n   Servers running for 2 seconds...")
	time.Sleep(2 * time.Second)

	// Verify ports are now in use
	fmt.Println("\n5. Verifying ports are now in use...")
	for i := 0; i < 3; i++ {
		port := basePort + i
		if allocator.IsPortInUse(port) {
			fmt.Printf("   ✓ Port %d is in use (as expected)\n", port)
		}
	}

	// Cleanup
	fmt.Println("\n6. Cleaning up...")
	for i, listener := range listeners {
		listener.Close()
		fmt.Printf("   ✓ Closed server on port %d\n", basePort+i)
	}

	// Verify ports are free again
	fmt.Println("\n7. Verifying ports are free after cleanup...")
	time.Sleep(100 * time.Millisecond) // Brief wait for OS to release ports
	for i := 0; i < 3; i++ {
		port := basePort + i
		if !allocator.IsPortInUse(port) {
			fmt.Printf("   ✓ Port %d is free again\n", port)
		}
	}

	fmt.Println("\n=== Example completed successfully ===")
}
