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

// Package main demonstrates full environment management with go-portalloc.
//
// This example shows how to:
//   - Create isolated environments with unique IDs
//   - Use environment locking
//   - Manage complete test isolation
//   - Validate and cleanup environments
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
)

func main() {
	fmt.Println("=== Full Environment Management Example ===\n")

	// Get current working directory
	worktree, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get working directory:", err)
	}

	// Example 1: Create environment manager
	fmt.Println("1. Creating environment manager...")
	config := &isolation.Config{
		WorktreePath: worktree,
		InstanceID:   "example-test",
		MaxRetries:   999,
	}

	idGen := isolation.NewIDGenerator(config)
	portAlloc := ports.NewAllocator(nil)
	manager := isolation.NewEnvironmentManager(idGen, portAlloc)
	fmt.Println("   ✓ Environment manager created\n")

	// Example 2: Create isolated environment
	fmt.Println("2. Creating isolated environment with 5 ports...")
	env, err := manager.CreateEnvironment(5)
	if err != nil {
		log.Fatal("Failed to create environment:", err)
	}
	defer manager.Cleanup(env)

	fmt.Printf("   ✓ Environment created successfully\n")
	fmt.Printf("     Isolation ID:   %s\n", env.ID)
	fmt.Printf("     Temp Directory: %s\n", env.TempDir)
	fmt.Printf("     Lock File:      %s\n", env.LockFile)
	fmt.Printf("     Base Port:      %d\n", env.Ports.BasePort)
	fmt.Printf("     Port Count:     %d\n", env.Ports.Count)
	fmt.Printf("     All Ports:      %v\n\n", env.Ports.Ports())

	// Example 3: Use allocated ports
	fmt.Println("3. Using allocated ports...")
	serverPort, _ := env.Ports.GetPort(0)
	dbPort, _ := env.Ports.GetPort(1)
	cachePort, _ := env.Ports.GetPort(2)

	fmt.Printf("   Server Port:  %d\n", serverPort)
	fmt.Printf("   DB Port:      %d\n", dbPort)
	fmt.Printf("   Cache Port:   %d\n\n", cachePort)

	// Example 4: Validate environment
	fmt.Println("4. Validating environment...")
	if err := manager.Validate(env); err != nil {
		log.Fatal("Environment validation failed:", err)
	}
	fmt.Println("   ✓ Environment validation passed\n")

	// Example 5: Check environment files
	fmt.Println("5. Checking environment files...")

	// Check temp directory
	if _, err := os.Stat(env.TempDir); err == nil {
		fmt.Printf("   ✓ Temp directory exists: %s\n", env.TempDir)
	}

	// Check lock file
	if _, err := os.Stat(env.LockFile); err == nil {
		fmt.Printf("   ✓ Lock file exists: %s\n", env.LockFile)
	}

	// Check env file if it exists
	if env.EnvFile != "" {
		if _, err := os.Stat(env.EnvFile); err == nil {
			fmt.Printf("   ✓ Env file exists: %s\n", env.EnvFile)
		}
	}
	fmt.Println()

	// Example 6: Manual lock management (advanced usage)
	fmt.Println("6. Demonstrating manual lock management...")

	// Generate another isolation ID
	isolationID2, err := idGen.Generate()
	if err != nil {
		log.Fatal("Failed to generate ID:", err)
	}
	fmt.Printf("   Generated ID: %s\n", isolationID2)

	// Create lock
	lockPath2, err := idGen.CreateLock(isolationID2)
	if err != nil {
		log.Fatal("Failed to create lock:", err)
	}
	fmt.Printf("   ✓ Lock created: %s\n", lockPath2)

	// Check if locked
	if idGen.IsLocked(isolationID2) {
		fmt.Printf("   ✓ Lock verified\n")
	}

	// Release lock
	if err := idGen.ReleaseLock(isolationID2); err != nil {
		log.Fatal("Failed to release lock:", err)
	}
	fmt.Printf("   ✓ Lock released\n\n")

	// Example 7: Cleanup
	fmt.Println("7. Cleaning up environment...")
	if err := manager.Cleanup(env); err != nil {
		log.Fatal("Failed to cleanup:", err)
	}
	fmt.Println("   ✓ Environment cleaned up successfully")

	// Verify cleanup
	if _, err := os.Stat(env.TempDir); os.IsNotExist(err) {
		fmt.Println("   ✓ Temp directory removed")
	}
	if _, err := os.Stat(env.LockFile); os.IsNotExist(err) {
		fmt.Println("   ✓ Lock file removed")
	}

	fmt.Println("\n=== Example completed successfully ===")
}
