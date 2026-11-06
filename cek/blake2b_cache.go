// Copyright 2026 Blink Labs Software
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

package cek

import "golang.org/x/crypto/blake2b"

const (
	blake2bCacheMinInput = 4 << 10
	blake2bCacheMaxInput = 1 << 20
	blake2bCacheMaxBytes = 1 << 20
	blake2bCacheEntries  = 64
)

type blake2bCache struct {
	values map[string][32]byte
	keys   [blake2bCacheEntries]string
	head   int
	size   int
	bytes  int
}

func (m *Machine[T]) blake2b256(input []byte) [32]byte {
	// Small inputs cost less to hash than to retain. Oversized inputs must
	// not make the evaluation's auxiliary memory grow with script data.
	if len(input) < blake2bCacheMinInput || len(input) > blake2bCacheMaxInput {
		return blake2b.Sum256(input)
	}
	if m.blake2bCache == nil {
		m.blake2bCache = &blake2bCache{values: make(map[string][32]byte)}
	}
	c := m.blake2bCache
	if result, found := c.values[string(input)]; found {
		return result
	}
	result := blake2b.Sum256(input)
	// Evict before copying. Both the number of keys and their total payload
	// are bounded, and hits cannot pin old entries by refreshing their age.
	for c.size == blake2bCacheEntries || c.bytes > blake2bCacheMaxBytes-len(input) {
		old := c.keys[c.head]
		delete(c.values, old)
		c.keys[c.head] = ""
		c.head = (c.head + 1) % blake2bCacheEntries
		c.size--
		c.bytes -= len(old)
	}
	// Own the full bytes: a digest key would introduce collision assumptions,
	// and retaining a borrowed slice would allow caller mutation to change it.
	key := string(input)
	c.values[key] = result
	c.keys[(c.head+c.size)%blake2bCacheEntries] = key
	c.size++
	c.bytes += len(key)
	return result
}

func (c *blake2bCache) reset() {
	if c == nil {
		return
	}
	clear(c.values)
	clear(c.keys[:])
	c.head, c.size, c.bytes = 0, 0, 0
}
