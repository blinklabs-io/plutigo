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
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/syn"
	"golang.org/x/crypto/blake2b"
)

func evalCachedBlake(m *Machine[syn.DeBruijn], input []byte) ([]byte, error) {
	b := newTestBuiltin(builtin.Blake2b_256)
	b = b.ApplyArg(&Constant{&syn.ByteString{Inner: input}})
	value, err := m.evalBuiltinApp(b)
	if err != nil {
		return nil, err
	}
	return unwrapByteString[syn.DeBruijn](value)
}

func TestBlake2bCachePreservesBytesAndCharges(t *testing.T) {
	t.Parallel()
	m := newTestMachine()
	budget := ExBudget{Cpu: 1 << 60, Mem: 1 << 60}
	input := bytes.Repeat([]byte{0x5a}, 64<<10)
	original := bytes.Clone(input)
	expected := blake2b.Sum256(input)
	var remaining ExBudget
	for i := range 2 {
		m.ExBudget = budget
		argument := input
		if i != 0 {
			argument = bytes.Clone(input)
		}
		out, err := evalCachedBlake(m, argument)
		if err != nil || !bytes.Equal(out, expected[:]) {
			t.Fatalf("hash %d changed: %v", i, err)
		}
		if i == 0 {
			remaining = m.ExBudget
		} else if m.ExBudget != remaining {
			t.Fatal("cache hit changed the execution charge")
		}
		out[0] ^= 0xff
	}
	if remaining == budget {
		t.Fatal("hash was not charged")
	}
	if m.blake2bCache == nil || len(m.blake2bCache.values) != 1 {
		t.Fatal("builtin did not admit one exact input")
	}
	m.ExBudget = ExBudget{}
	if _, err := evalCachedBlake(m, input); !IsBudgetError(err) {
		t.Fatalf("warm hit bypassed budget: %v", err)
	}
	// The key must own a copy even when the input's backing array changes.
	input[0] ^= 0xff
	if m.blake2bCache.keys[m.blake2bCache.head] != string(original) {
		t.Fatal("mutating admitted input changed the retained cache key")
	}
	m.ExBudget = budget
	out, err := evalCachedBlake(m, input)
	expected = blake2b.Sum256(input)
	if err != nil || !bytes.Equal(out, expected[:]) {
		t.Fatal("mutated input aliased a cached key")
	}
}

func TestBlake2bCacheBounds(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, blake2bCacheMinInput - 1, blake2bCacheMinInput, 64 << 10, blake2bCacheMaxInput, blake2bCacheMaxInput + 1} {
		m := newTestMachine()
		input := make([]byte, size)
		for i := range blake2bCacheEntries + 3 {
			if len(input) >= 8 {
				binary.LittleEndian.PutUint64(input[:8], uint64(i))
			}
			got := m.blake2b256(input)
			if got != blake2b.Sum256(input) {
				t.Fatalf("size %d: incorrect digest", size)
			}
			c := m.blake2bCache
			eligible := size >= blake2bCacheMinInput && size <= blake2bCacheMaxInput
			if !eligible {
				if c != nil {
					t.Fatalf("size %d was retained", size)
				}
				continue
			}
			total, count := 0, 0
			for _, key := range c.keys {
				total += len(key)
				if key != "" {
					count++
				}
			}
			if c.size != len(c.values) || c.size != count || c.bytes != total || c.size > blake2bCacheEntries || c.bytes > blake2bCacheMaxBytes {
				t.Fatalf("size %d exceeded entry or byte bounds", size)
			}
		}
		if c := m.blake2bCache; c != nil {
			key := string(input)
			if _, ok := c.values[key]; !ok {
				t.Fatal("latest input not admitted after eviction")
			}
			head, size, payload := c.head, c.size, c.bytes
			m.blake2b256(bytes.Clone(input))
			if c.head != head || c.size != size || c.bytes != payload {
				t.Fatal("cache hit changed FIFO admission state")
			}
			if _, ok := c.values[key]; !ok {
				t.Fatal("exact repeated bytes were not retained")
			}
		}
	}
}

func blakeHashTerm(input []byte) syn.Term[syn.DeBruijn] {
	return &syn.Apply[syn.DeBruijn]{Function: &syn.Builtin{DefaultFunction: builtin.Blake2b_256}, Argument: &syn.Constant{Con: &syn.ByteString{Inner: input}}}
}

func assertBlakeCacheEmpty(t *testing.T, m *Machine[syn.DeBruijn]) {
	t.Helper()
	c := m.blake2bCache
	if c == nil {
		t.Fatal("evaluation did not exercise hash admission")
	}
	for _, key := range c.keys {
		if key != "" {
			t.Fatal("evaluation retained input through FIFO")
		}
	}
	if len(c.values) != 0 || c.bytes != 0 || c.size != 0 || c.head != 0 {
		t.Fatal("evaluation retained hash inputs")
	}
}

func TestBlake2bCacheClearedAfterEvaluation(t *testing.T) {
	t.Parallel()
	input := bytes.Repeat([]byte{0x42}, 64<<10)
	for _, fail := range []bool{false, true} {
		m := newTestMachine()
		for range 2 {
			m.ExBudget = ExBudget{Cpu: 1 << 60, Mem: 1 << 60}
			term := blakeHashTerm(input)
			if fail {
				term = &syn.Apply[syn.DeBruijn]{Function: &syn.Lambda[syn.DeBruijn]{Body: &syn.Error{}}, Argument: term}
			}
			_, err := m.Run(term)
			if (err != nil) != fail {
				t.Fatalf("unexpected result: %v", err)
			}
			assertBlakeCacheEmpty(t, m)
		}
	}
}

func TestBlake2bCacheClearedAfterCancellation(t *testing.T) {
	t.Parallel()
	m := newTestMachine()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &blakeCancelContext{Context: base, m: m, cancel: cancel}
	term := &syn.Apply[syn.DeBruijn]{
		Function: &syn.Lambda[syn.DeBruijn]{Body: divergentTerm()},
		Argument: blakeHashTerm(bytes.Repeat([]byte{3}, 64<<10)),
	}
	m.ExBudget = ExBudget{Cpu: 1 << 60, Mem: 1 << 60}
	_, err := m.RunContext(ctx, term)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation after admission, got %v", err)
	}
	assertBlakeCacheEmpty(t, m)
}

type blakeCancelContext struct {
	context.Context
	m      *Machine[syn.DeBruijn]
	cancel context.CancelFunc
}

func (c *blakeCancelContext) Err() error {
	if c.m.blake2bCache != nil && c.m.blake2bCache.size > 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestBlake2bCacheClearedAfterPanic(t *testing.T) {
	t.Parallel()
	m := newTestMachine()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &blakePanicContext{Context: base, m: m}
	term := &syn.Apply[syn.DeBruijn]{
		Function: &syn.Lambda[syn.DeBruijn]{Body: divergentTerm()},
		Argument: blakeHashTerm(bytes.Repeat([]byte{3}, 64<<10)),
	}
	m.ExBudget = ExBudget{Cpu: 1 << 60, Mem: 1 << 60}
	_, err := m.RunContext(ctx, term)
	if !IsInternalError(err) {
		t.Fatalf("want recovered evaluator panic, got %v", err)
	}
	assertBlakeCacheEmpty(t, m)
}

type blakePanicContext struct {
	context.Context
	m *Machine[syn.DeBruijn]
}

func (c *blakePanicContext) Err() error {
	if c.m.blake2bCache != nil && c.m.blake2bCache.size > 0 {
		panic("test cancellation check after hash admission")
	}
	return nil
}

func TestBlake2bCacheIndependentMachines(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for worker := range 8 {
		wg.Go(func() {
			m := newTestMachine()
			input := bytes.Repeat([]byte{byte(worker)}, 64<<10)
			for range 8 {
				m.ExBudget = ExBudget{Cpu: 1 << 60, Mem: 1 << 60}
				if _, err := m.Run(blakeHashTerm(input)); err != nil {
					errs <- err
					return
				}
				if m.blake2bCache == nil || len(m.blake2bCache.values) != 0 {
					errs <- errors.New("machine retained input")
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

var blakeBenchmarkResult [32]byte

func BenchmarkBlake2bRepeatedInput(b *testing.B) {
	input := bytes.Repeat([]byte{0x42}, 64<<10)
	for _, cached := range []bool{false, true} {
		name := "hash"
		if cached {
			name = "cache"
		}
		b.Run(name, func(b *testing.B) {
			m := newTestMachine()
			m.blake2b256(input)
			other := bytes.Clone(input)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if cached {
					blakeBenchmarkResult = m.blake2b256(other)
				} else {
					blakeBenchmarkResult = blake2b.Sum256(other)
				}
			}
		})
	}
}
