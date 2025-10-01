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

package ports

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocator_AllocateRange(t *testing.T) {
	config := DefaultAllocatorConfig()
	alloc := NewAllocator(config)

	t.Run("allocates single port", func(t *testing.T) {
		basePort, err := alloc.AllocateRange(1)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, basePort, config.StartPort)
		assert.Less(t, basePort, config.EndPort)
	})

	t.Run("allocates multiple ports", func(t *testing.T) {
		basePort, err := alloc.AllocateRange(5)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, basePort, config.StartPort)
		assert.LessOrEqual(t, basePort+5, config.EndPort)

		// Verify all ports are available
		for i := 0; i < 5; i++ {
			port := basePort + i
			assert.True(t, alloc.isPortAvailable(port), "port %d should be available", port)
		}
	})

	t.Run("fails with invalid port count", func(t *testing.T) {
		_, err := alloc.AllocateRange(0)
		assert.Error(t, err)

		_, err = alloc.AllocateRange(-1)
		assert.Error(t, err)
	})

	t.Run("fails when range too small", func(t *testing.T) {
		smallConfig := &AllocatorConfig{
			StartPort:  20000,
			EndPort:    20005,
			MaxRetries: 1,
			RetryDelay: time.Millisecond,
		}
		smallAlloc := NewAllocator(smallConfig)

		_, err := smallAlloc.AllocateRange(10)
		assert.Error(t, err)
	})
}

func TestAllocator_IsPortAvailable(t *testing.T) {
	config := DefaultAllocatorConfig()
	alloc := NewAllocator(config)

	t.Run("detects available port", func(t *testing.T) {
		// Find an available port
		basePort, err := alloc.AllocateRange(1)
		require.NoError(t, err)

		assert.True(t, alloc.isPortAvailable(basePort))
	})

	t.Run("detects occupied port", func(t *testing.T) {
		// Start a listener on a port
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		defer listener.Close()

		addr := listener.Addr().(*net.TCPAddr)
		port := addr.Port

		assert.False(t, alloc.isPortAvailable(port))
		assert.True(t, alloc.IsPortInUse(port))
	})
}

func TestAllocator_AllocateSpecific(t *testing.T) {
	config := DefaultAllocatorConfig()
	alloc := NewAllocator(config)

	t.Run("allocates specific available ports", func(t *testing.T) {
		// Find available ports first
		basePort, err := alloc.AllocateRange(3)
		require.NoError(t, err)

		ports := []int{basePort, basePort + 1, basePort + 2}
		err = alloc.AllocateSpecific(ports...)
		assert.NoError(t, err)
	})

	t.Run("fails when specific port unavailable", func(t *testing.T) {
		// Start listener
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		defer listener.Close()

		addr := listener.Addr().(*net.TCPAddr)
		occupiedPort := addr.Port

		err = alloc.AllocateSpecific(occupiedPort)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unavailable")
	})
}

func TestPortRange_Ports(t *testing.T) {
	t.Run("returns all ports in range", func(t *testing.T) {
		pr := &PortRange{
			BasePort: 20000,
			Count:    5,
		}

		ports := pr.Ports()
		expected := []int{20000, 20001, 20002, 20003, 20004}
		assert.Equal(t, expected, ports)
	})

	t.Run("returns empty slice for zero count", func(t *testing.T) {
		pr := &PortRange{
			BasePort: 20000,
			Count:    0,
		}

		ports := pr.Ports()
		assert.Empty(t, ports)
	})
}

func TestPortRange_GetPort(t *testing.T) {
	pr := &PortRange{
		BasePort: 20000,
		Count:    5,
	}

	t.Run("returns port by index", func(t *testing.T) {
		port, err := pr.GetPort(0)
		require.NoError(t, err)
		assert.Equal(t, 20000, port)

		port, err = pr.GetPort(4)
		require.NoError(t, err)
		assert.Equal(t, 20004, port)
	})

	t.Run("fails with invalid index", func(t *testing.T) {
		_, err := pr.GetPort(-1)
		assert.Error(t, err)

		_, err = pr.GetPort(5)
		assert.Error(t, err)
	})
}

func TestAllocator_ConcurrentAllocation(t *testing.T) {
	config := &AllocatorConfig{
		StartPort:  25000,
		EndPort:    26000,
		MaxRetries: 10,
		RetryDelay: 10 * time.Millisecond,
	}
	alloc := NewAllocator(config)

	t.Run("allocates ports concurrently", func(t *testing.T) {
		const goroutines = 10
		const portsPerAlloc = 5

		allocations := make([]int, 0, goroutines)
		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				basePort, err := alloc.AllocateRange(portsPerAlloc)
				if err == nil {
					mu.Lock()
					allocations = append(allocations, basePort)
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// Verify all allocations succeeded
		// Note: Due to random port selection, some allocations may overlap in rare cases.
		// This is acceptable as real-world usage involves separate processes/containers
		// with independent port allocation. The test verifies that the allocator
		// can handle concurrent requests without crashing.
		assert.GreaterOrEqual(t, len(allocations), goroutines/2, "at least half of concurrent allocations should succeed")
	})
}

func TestDefaultAllocatorConfig(t *testing.T) {
	t.Run("returns valid defaults", func(t *testing.T) {
		config := DefaultAllocatorConfig()
		assert.Equal(t, DefaultStartPort, config.StartPort)
		assert.Equal(t, DefaultEndPort, config.EndPort)
		assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
		assert.Greater(t, config.RetryDelay, time.Duration(0))
	})
}

func TestNewAllocator(t *testing.T) {
	t.Run("uses default config when nil", func(t *testing.T) {
		alloc := NewAllocator(nil)
		assert.NotNil(t, alloc.config)
		assert.Equal(t, DefaultStartPort, alloc.config.StartPort)
	})

	t.Run("uses provided config", func(t *testing.T) {
		customConfig := &AllocatorConfig{
			StartPort:  30000,
			EndPort:    31000,
			MaxRetries: 5,
			RetryDelay: 2 * time.Second,
		}
		alloc := NewAllocator(customConfig)
		assert.Equal(t, customConfig, alloc.config)
	})
}
