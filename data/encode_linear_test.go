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
	"encoding/hex"
	"math/big"
	"runtime"
	"strings"
	"testing"
)

func deepData(shape string, depth int) PlutusData {
	var pd PlutusData = NewInteger(big.NewInt(1))
	for range depth {
		switch shape {
		case "list":
			pd = NewList(pd)
		case "constr":
			pd = NewConstr(0, pd)
		case "map key":
			pd = NewMap([][2]PlutusData{{pd, NewInteger(big.NewInt(1))}})
		}
	}
	return pd
}

// allocatedBytes reports the bytes the process allocates while fn runs.
func allocatedBytes(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// Not t.Parallel: the measurement is the process-wide TotalAlloc counter, so
// concurrent tests would be counted into it.
func TestEncodeAllocationLinearInDepth(t *testing.T) {
	encoders := map[string]func(PlutusData) ([]byte, error){
		"cbor": Encode,
		"json": EncodeJSON,
	}
	for encName, encode := range encoders {
		for _, shape := range []string{"list", "constr", "map key"} {
			t.Run(encName+"/"+shape, func(t *testing.T) {
				const depth = 1024
				small, large := deepData(shape, depth), deepData(shape, 4*depth)
				run := func(pd PlutusData) uint64 {
					return allocatedBytes(func() {
						if _, err := encode(pd); err != nil {
							t.Fatalf("encode: %v", err)
						}
					})
				}
				smallBytes, largeBytes := run(small), run(large)
				// Linear growth quadruples the allocation for 4x the depth;
				// re-encoding every subtree at each level grows it ~16x.
				if largeBytes > 8*smallBytes {
					t.Fatalf(
						"allocation grew %.1fx for 4x depth (%d -> %d bytes), want linear",
						float64(largeBytes)/float64(smallBytes),
						smallBytes,
						largeBytes,
					)
				}
			})
		}
	}
}

func TestEncodeNestedLengthForms(t *testing.T) {
	t.Parallel()
	one := NewInteger(big.NewInt(1))
	pairs := make([][2]PlutusData, 24)
	items := make([]PlutusData, 256)
	for i := range pairs {
		pairs[i] = [2]PlutusData{one, one}
	}
	for i := range items {
		items[i] = one
	}

	tests := []struct {
		name string
		pd   PlutusData
		want string
	}{
		{
			"definite list holding indefinite list",
			NewListDefIndef(false, NewListDefIndef(true, one)),
			"819f01ff",
		},
		{
			"indefinite constr holding indefinite map holding empty list",
			NewConstrDefIndef(true, 1, NewMapDefIndef(true, [][2]PlutusData{{one, NewList()}})),
			"d87a9fbf0180ffff",
		},
		{
			"constr above 127 uses the general form",
			NewConstrDefIndef(false, 200, one),
			"d86682" + "18c8" + "8101",
		},
		{"constr in the extended tag range", NewConstr(7), "d905008" + "0"},
		{"definite map needs a one byte length", NewMap(pairs), "b818" + strings.Repeat("0101", 24)},
		{"definite list needs a two byte length", NewListDefIndef(false, items...), "990100" + strings.Repeat("01", 256)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Encode(tt.pd)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if hex.EncodeToString(got) != tt.want {
				t.Fatalf("Encode = %x, want %s", got, tt.want)
			}
		})
	}
}
