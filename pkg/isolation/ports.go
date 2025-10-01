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

package isolation

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
		return 0, ErrIndexOutOfRange
	}
	return pr.BasePort + index, nil
}
