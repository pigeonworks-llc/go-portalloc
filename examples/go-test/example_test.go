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

package example_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/pigeonworks-llc/go-portalloc/pkg/isolation"
	"github.com/pigeonworks-llc/go-portalloc/pkg/ports"
)

// TestWithIsolatedEnvironment demonstrates creating an isolated environment
// within a test suite.
func TestWithIsolatedEnvironment(t *testing.T) {
	// Create environment manager
	config := &isolation.Config{
		WorktreePath: t.TempDir(),
		InstanceID:   "test-example",
		MaxRetries:   10,
	}

	idGen := isolation.NewIDGenerator(config)
	portAlloc := ports.NewAllocator(nil)
	manager := isolation.NewEnvironmentManager(idGen, portAlloc)

	// Create isolated environment
	env, err := manager.CreateEnvironment(3)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}
	defer func() {
		if err := manager.Cleanup(env); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	}()

	// Use isolated environment
	t.Logf("Isolation ID: %s", env.ID)
	t.Logf("Allocated ports: %v", env.Ports.Ports())

	// Set environment variables for subtests
	os.Setenv("TEST_PORT_1", fmt.Sprintf("%d", env.Ports.BasePort))
	os.Setenv("TEST_PORT_2", fmt.Sprintf("%d", env.Ports.BasePort+1))
	os.Setenv("TEST_PORT_3", fmt.Sprintf("%d", env.Ports.BasePort+2))

	// Run subtests with isolated environment
	t.Run("subtest1", func(t *testing.T) {
		port := os.Getenv("TEST_PORT_1")
		t.Logf("Using port: %s", port)
		// Your test logic here
	})

	t.Run("subtest2", func(t *testing.T) {
		port := os.Getenv("TEST_PORT_2")
		t.Logf("Using port: %s", port)
		// Your test logic here
	})
}

// TestParallelWithIsolation demonstrates running parallel tests
// with isolated environments.
func TestParallelWithIsolation(t *testing.T) {
	t.Parallel()

	// Each parallel test gets its own environment
	config := &isolation.Config{
		WorktreePath: t.TempDir(),
		InstanceID:   t.Name(),
		MaxRetries:   10,
	}

	idGen := isolation.NewIDGenerator(config)
	portAlloc := ports.NewAllocator(nil)
	manager := isolation.NewEnvironmentManager(idGen, portAlloc)

	env, err := manager.CreateEnvironment(2)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}
	defer manager.Cleanup(env)

	t.Logf("Test %s using ports %v", t.Name(), env.Ports.Ports())

	// Your parallel test logic here
	// Tests run concurrently without port conflicts
}
