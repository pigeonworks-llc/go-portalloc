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

// Package parallel_testing demonstrates parallel test execution with port isolation.
//
// This example shows how go-portalloc enables safe parallel test execution
// by ensuring each test gets isolated ports, preventing "port already in use" errors.
//
// Run with:
//
//	go test -v -parallel 4
package parallel_testing

import (
	"fmt"
	"net"
	"testing"

	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
)

// TestParallelService1 demonstrates parallel test isolation.
func TestParallelService1(t *testing.T) {
	t.Parallel()

	allocator := ports.NewAllocator(nil)
	basePort, err := allocator.AllocateRange(2)
	if err != nil {
		t.Fatal("Failed to allocate ports:", err)
	}

	t.Logf("Test 1: Allocated ports %d-%d", basePort, basePort+1)

	// Start test server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", basePort))
	if err != nil {
		t.Fatal("Failed to start server:", err)
	}
	defer listener.Close()

	t.Logf("Test 1: Server running on port %d", basePort)

	// Your test logic here
	// No port conflicts even when running in parallel!
}

// TestParallelService2 runs concurrently with TestParallelService1.
func TestParallelService2(t *testing.T) {
	t.Parallel()

	allocator := ports.NewAllocator(nil)
	basePort, err := allocator.AllocateRange(2)
	if err != nil {
		t.Fatal("Failed to allocate ports:", err)
	}

	t.Logf("Test 2: Allocated ports %d-%d", basePort, basePort+1)

	// Start test server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", basePort))
	if err != nil {
		t.Fatal("Failed to start server:", err)
	}
	defer listener.Close()

	t.Logf("Test 2: Server running on port %d", basePort)

	// Your test logic here
}

// TestParallelService3 runs concurrently with other tests.
func TestParallelService3(t *testing.T) {
	t.Parallel()

	allocator := ports.NewAllocator(nil)
	basePort, err := allocator.AllocateRange(3)
	if err != nil {
		t.Fatal("Failed to allocate ports:", err)
	}

	t.Logf("Test 3: Allocated ports %d-%d", basePort, basePort+2)

	// Start multiple test servers
	var listeners []net.Listener
	for i := 0; i < 3; i++ {
		port := basePort + i
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			t.Fatal("Failed to start server:", err)
		}
		defer listener.Close()
		listeners = append(listeners, listener)
		t.Logf("Test 3: Server %d running on port %d", i+1, port)
	}

	// Your test logic here
}

// TestParallelService4 demonstrates microservices testing.
func TestParallelService4(t *testing.T) {
	t.Parallel()

	allocator := ports.NewAllocator(nil)
	basePort, err := allocator.AllocateRange(4)
	if err != nil {
		t.Fatal("Failed to allocate ports:", err)
	}

	// Assign semantic names to ports
	apiPort := basePort
	dbPort := basePort + 1
	cachePort := basePort + 2
	metricsPort := basePort + 3

	t.Logf("Test 4: API=%d, DB=%d, Cache=%d, Metrics=%d",
		apiPort, dbPort, cachePort, metricsPort)

	// Start mock services
	apiListener, _ := net.Listen("tcp", fmt.Sprintf(":%d", apiPort))
	defer apiListener.Close()

	dbListener, _ := net.Listen("tcp", fmt.Sprintf(":%d", dbPort))
	defer dbListener.Close()

	cacheListener, _ := net.Listen("tcp", fmt.Sprintf(":%d", cachePort))
	defer cacheListener.Close()

	metricsListener, _ := net.Listen("tcp", fmt.Sprintf(":%d", metricsPort))
	defer metricsListener.Close()

	t.Logf("Test 4: All services started successfully")

	// Your microservices integration test logic here
}
