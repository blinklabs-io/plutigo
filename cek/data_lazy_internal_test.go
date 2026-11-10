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
	"math"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
)

// TestLazyDataUnwrapsToTheSameList checks that a list of data, or of data
// pairs, built by mkCons is its own source: listData, mapData and constrData
// wrap the list value itself, and unwrapping the data returns that value
// without allocating.
func TestLazyDataUnwrapsToTheSameList(t *testing.T) {
	items := newDataListAlloc[syn.DeBruijn]()
	consOntoSlice(&items.c, data.PlutusData(data.NewInteger(big.NewInt(1))), nil)
	pairs := newDataMapAlloc[syn.DeBruijn]()
	consOntoSlice(
		&pairs.c,
		[2]data.PlutusData{
			data.NewInteger(big.NewInt(1)),
			data.NewInteger(big.NewInt(2)),
		},
		nil,
	)

	itemSrc, err := lazyDataItems[syn.DeBruijn](&items.v, nil)
	if err != nil || itemSrc != any(&items.v) {
		t.Fatalf("lazyDataItems = %v, %v; want the list itself", itemSrc, err)
	}
	pairSrc, err := lazyDataPairs[syn.DeBruijn](&pairs.v)
	if err != nil || pairSrc != any(&pairs.v) {
		t.Fatalf("lazyDataPairs = %v, %v; want the list itself", pairSrc, err)
	}

	for name, tc := range map[string]struct {
		pd   data.PlutusData
		want Value[syn.DeBruijn]
	}{
		"list":   {newLazyList(itemSrc), &items.v},
		"constr": {newLazyConstr(big.NewInt(0), itemSrc), &items.v},
		"map":    {newLazyMap(pairSrc), &pairs.v},
	} {
		t.Run(name, func(t *testing.T) {
			var got Value[syn.DeBruijn]
			allocs := testing.AllocsPerRun(100, func() {
				got, _ = lazyListValueOf[syn.DeBruijn](tc.pd)
			})
			if got != tc.want {
				t.Fatalf("lazyListValueOf = %v, want the original list", got)
			}
			if allocs != 0 {
				t.Fatalf("lazyListValueOf allocated %v times, want 0", allocs)
			}
		})
	}
}

// TestConsLengthRejectsOverflow checks that a cell length that does not fit
// its int32 is refused rather than wrapped.
func TestConsLengthRejectsOverflow(t *testing.T) {
	if got := consLength(math.MaxInt32 - 1); got != math.MaxInt32 {
		t.Fatalf("consLength(MaxInt32-1) = %d, want %d", got, math.MaxInt32)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("consLength(MaxInt32) did not panic")
		}
	}()
	consLength(math.MaxInt32)
}
