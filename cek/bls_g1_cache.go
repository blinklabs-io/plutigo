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

import (
	"hash/maphash"
	"sync"

	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

const (
	blsG1UncompressCacheShards = 64
	blsG1UncompressShardLimit  = 64
)

type blsG1UncompressKey [bls.SizeOfG1AffineCompressed]byte

type blsG1UncompressShard struct {
	sync.Mutex
	values map[blsG1UncompressKey]bls.G1Jac
}

type blsG1UncompressStore struct {
	seed   maphash.Seed
	shards [blsG1UncompressCacheShards]blsG1UncompressShard
}

var blsG1UncompressCache = newBLSG1UncompressStore()

func newBLSG1UncompressStore() *blsG1UncompressStore {
	return &blsG1UncompressStore{seed: maphash.MakeSeed()}
}

func (c *blsG1UncompressStore) shard(
	key *blsG1UncompressKey,
) *blsG1UncompressShard {
	hash := maphash.Bytes(c.seed, key[:])
	return &c.shards[hash&(blsG1UncompressCacheShards-1)]
}

func (c *blsG1UncompressStore) get(key *blsG1UncompressKey) (bls.G1Jac, bool) {
	shard := c.shard(key)
	shard.Lock()
	value, ok := shard.values[*key]
	shard.Unlock()
	return value, ok
}

func (c *blsG1UncompressStore) store(key *blsG1UncompressKey, value bls.G1Jac) {
	shard := c.shard(key)
	shard.Lock()
	defer shard.Unlock()
	if shard.values == nil {
		shard.values = make(
			map[blsG1UncompressKey]bls.G1Jac,
			blsG1UncompressShardLimit,
		)
	}
	if _, exists := shard.values[*key]; !exists &&
		len(shard.values) >= blsG1UncompressShardLimit {
		clear(shard.values)
	}
	// G1Jac contains fixed-size field elements. Value copies isolate cached
	// points from mutable values returned to the evaluator.
	shard.values[*key] = value
}
