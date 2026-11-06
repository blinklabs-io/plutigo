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
	"bytes"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

func resetBLSG1UncompressCache() {
	for i := range blsG1UncompressCache.shards {
		shard := &blsG1UncompressCache.shards[i]
		shard.Lock()
		clear(shard.values)
		shard.Unlock()
	}
}

func evalG1Uncompress(
	raw []byte,
	budget ExBudget,
) (*bls.G1Jac, ExBudget, error) {
	m := newTestMachine()
	m.ExBudget = budget
	b := newTestBuiltin(builtin.Bls12_381_G1_Uncompress)
	b = b.ApplyArg(&Constant{&syn.ByteString{Inner: raw}})
	value, err := m.evalBuiltinApp(b)
	if err != nil {
		return nil, m.ExBudget, err
	}
	point, err := unwrapBls12_381G1Element[syn.DeBruijn](value)
	return point, m.ExBudget, err
}

func TestBLSG1UncompressCachePreservesPointAndBudget(t *testing.T) {
	// Not t.Parallel: observes admission into the process-wide builtin cache.
	resetBLSG1UncompressCache()
	generator, _, affine, _ := bls.Generators()
	encoded := affine.Bytes()
	budget := ExBudget{Cpu: 1 << 60, Mem: 1 << 60}
	first, cold, err := evalG1Uncompress(encoded[:], budget)
	if err != nil || first == nil {
		t.Fatalf("cold decode: %v", err)
	}
	if !first.Equal(&generator) {
		t.Fatal("cold point differs from generator")
	}
	key := blsG1UncompressKey(encoded)
	if _, found := blsG1UncompressCache.get(&key); !found {
		t.Fatal("successful builtin decode did not populate the cache")
	}
	first.Neg(first)
	second, warm, err := evalG1Uncompress(encoded[:], budget)
	if err != nil || second == nil {
		t.Fatalf("warm decode: %v", err)
	}
	if first == second || !second.Equal(&generator) {
		t.Fatal("mutating a returned point corrupted the cached point")
	}
	if cold != warm || cold == budget {
		t.Fatalf(
			"execution charge changed: cold=%v warm=%v initial=%v",
			cold,
			warm,
			budget,
		)
	}
	if _, _, err := evalG1Uncompress(encoded[:], ExBudget{}); !IsBudgetError(
		err,
	) {
		t.Fatalf("warm cache bypassed exhausted budget: %v", err)
	}
	for _, raw := range [][]byte{encoded[:47], append(bytes.Clone(encoded[:]), 0)} {
		if _, _, err := evalG1Uncompress(raw, budget); err == nil {
			t.Fatal("cached compressed prefix bypassed exact length check")
		}
	}
}

func TestBLSG1UncompressCacheRejectsInvalidPoints(t *testing.T) {
	// Not t.Parallel: observes admission into the process-wide builtin cache.
	resetBLSG1UncompressCache()
	var torsion bls.G1Affine
	torsion.Y.SetUint64(2)
	if !torsion.IsOnCurve() || torsion.IsInSubGroup() {
		t.Fatal("fixture must be on curve and outside the subgroup")
	}
	encoded := torsion.Bytes()
	for _, raw := range [][]byte{make([]byte, 48), encoded[:]} {
		for range 2 {
			if _, _, err := evalG1Uncompress(raw, ExBudget{Cpu: 1 << 60, Mem: 1 << 60}); err == nil {
				t.Fatal("invalid curve or subgroup encoding was accepted")
			}
			key := blsG1UncompressKey(raw)
			if _, found := blsG1UncompressCache.get(&key); found {
				t.Fatal("failed decode populated the cache")
			}
		}
	}
	var infinity bls.G1Affine
	infinity.SetInfinity()
	raw := infinity.Bytes()
	for range 2 {
		point, _, err := evalG1Uncompress(
			raw[:],
			ExBudget{Cpu: 1 << 60, Mem: 1 << 60},
		)
		if err != nil || point == nil || !point.Z.IsZero() {
			t.Fatalf("valid infinity decode changed: %v", err)
		}
	}
}

func TestBLSG1UncompressCacheBoundAndConcurrency(t *testing.T) {
	t.Parallel()
	var cache blsG1UncompressStore
	generator, _, _, _ := bls.Generators()
	var latest blsG1UncompressKey
	for i := range blsG1UncompressShardLimit + 1 {
		// These bytes do not affect shard selection; all keys fill one shard.
		binary.LittleEndian.PutUint64(latest[8:16], uint64(i))
		cache.store(&latest, generator)
	}
	shard := cache.shard(&latest)
	if len(shard.values) != 1 {
		t.Fatalf("full shard did not reset: %d", len(shard.values))
	}
	if _, ok := cache.get(&latest); !ok {
		t.Fatal("new key not admitted after eviction")
	}
	otherKey := latest
	otherKey[8] ^= 0x80
	var negated bls.G1Jac
	negated.Neg(&generator)
	cache.store(&otherKey, negated)
	if point, ok := cache.get(&latest); !ok || !point.Equal(&generator) {
		t.Fatal("distinct full keys in one shard alias each other")
	}
	var failed atomic.Bool
	var wg sync.WaitGroup
	for worker := range 16 {
		wg.Go(func() {
			for i := range 1000 {
				key := latest
				key[0] = byte(worker)
				binary.LittleEndian.PutUint64(key[8:16], uint64(i))
				cache.store(&key, generator)
				value, ok := cache.get(&key)
				if ok && !value.Equal(&generator) {
					failed.Store(true)
				}
			}
		})
	}
	wg.Wait()
	if failed.Load() {
		t.Fatal("concurrent cache access changed a point")
	}
	for i := range cache.shards {
		if len(cache.shards[i].values) > blsG1UncompressShardLimit {
			t.Fatal("cache exceeded its per-shard bound")
		}
	}
}

func BenchmarkBLSG1Uncompress(b *testing.B) {
	_, _, affine, _ := bls.Generators()
	encoded := affine.Bytes()
	budget := ExBudget{Cpu: 1 << 60, Mem: 1 << 60}
	m := newTestMachine()
	fn := newTestBuiltin(builtin.Bls12_381_G1_Uncompress)
	fn = fn.ApplyArg(&Constant{&syn.ByteString{Inner: encoded[:]}})
	m.ExBudget = budget
	if _, err := m.evalBuiltinApp(fn); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.ExBudget = budget
		if _, err := m.evalBuiltinApp(fn); err != nil {
			b.Fatal(err)
		}
	}
}
