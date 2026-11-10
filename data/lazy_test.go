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

package data

import (
	"context"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/internal/lazydata"
)

// countingSource is a lazy sequence of n integers that counts its
// materializations.
type countingSource[E any] struct {
	n           int
	elem        func(int) E
	materialize int
}

func (s *countingSource[E]) Len() int { return s.n }

func (s *countingSource[E]) Materialize() []E {
	s.materialize++
	out := make([]E, s.n)
	for i := range out {
		out[i] = s.elem(i)
	}
	return out
}

func (*countingSource[E]) Seal(lazydata.Token) {}

func intItem(i int) PlutusData { return NewInteger(big.NewInt(int64(i))) }

// TestCloneContextBoundsLazyElementsBeforeBuildingThem checks that the clone
// node bound rejects a lazy container from its length, without building its
// elements.
func TestCloneContextBoundsLazyElementsBeforeBuildingThem(t *testing.T) {
	const n = 100
	items := &countingSource[PlutusData]{n: n, elem: intItem}
	fields := &countingSource[PlutusData]{n: n, elem: intItem}
	pairs := &countingSource[[2]PlutusData]{n: n, elem: func(i int) [2]PlutusData {
		return [2]PlutusData{intItem(i), intItem(i)}
	}}
	tests := map[string]struct {
		pd    PlutusData
		count *int
	}{
		"list":   {&List{hdr: &itemsHeader{src: items}}, &items.materialize},
		"constr": {&Constr{Tag: new(big.Int), hdr: &itemsHeader{src: fields}}, &fields.materialize},
		"map":    {&Map{hdr: &pairsHeader{src: pairs}}, &pairs.materialize},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CloneContext(context.Background(), tt.pd, 100, n/2); err == nil {
				t.Fatal("CloneContext accepted a container over the node bound")
			}
			if got := *tt.count; got != 0 {
				t.Fatalf("CloneContext built the elements %d times before rejecting", got)
			}
		})
	}
}

// TestLazyContainersReadThroughTheirSource checks the accessors of
// containers built over a source.
func TestLazyContainersReadThroughTheirSource(t *testing.T) {
	src := &countingSource[PlutusData]{n: 3, elem: intItem}
	list := lazydata.Hooks.NewList(src).(*List)
	if list.Len() != 3 || src.materialize != 0 {
		t.Fatalf("Len = %d after %d materializations", list.Len(), src.materialize)
	}
	if !list.Equal(NewList(intItem(0), intItem(1), intItem(2))) {
		t.Fatalf("lazy list = %v", list)
	}
	if lazydata.Hooks.SourceOf(list) != src {
		t.Fatal("SourceOf does not return the list's source")
	}
	if lazydata.Hooks.SourceOf(NewList(intItem(0))) != nil {
		t.Fatal("SourceOf returned a source for a plain list")
	}
}
