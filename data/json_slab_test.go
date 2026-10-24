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
	"math/big"
	"strings"
	"testing"
)

// TestDecodeJSONIntegersAreIndependent checks that integers decoded from
// shared slab memory compare equal to math/big's own parsing and can be
// mutated without disturbing their neighbours.
func TestDecodeJSONIntegersAreIndependent(t *testing.T) {
	literals := []string{
		"0", "-0", "1", "-1", "9", "4294967295", "4294967296",
		"999999999999999999", "-999999999999999999",
		"1000000000000000000", "18446744073709551615",
		"18446744073709551616", "-18446744073709551616",
	}
	var sb strings.Builder
	sb.WriteString(`{"list":[`)
	// Repeat well past one slab so chunk boundaries are crossed.
	const reps = 3 * jsonSlabSize
	for i := range reps {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"int":` + literals[i%len(literals)] + `}`)
	}
	sb.WriteString(`]}`)
	pd, err := DecodeJSON([]byte(sb.String()))
	if err != nil {
		t.Fatal(err)
	}
	items := pd.(*List).Items
	if len(items) != reps {
		t.Fatalf("got %d items, want %d", len(items), reps)
	}
	check := func() {
		for i, item := range items {
			want, _ := new(big.Int).SetString(literals[i%len(literals)], 10)
			if got := item.(*Integer).Inner; got.Cmp(want) != 0 {
				t.Fatalf("item %d = %v, want %v", i, got, want)
			}
		}
	}
	check()
	// Grow every integer in place; neighbours must be unaffected.
	big1 := new(big.Int).Lsh(big.NewInt(1), 200)
	for i := 0; i < len(items); i += 2 {
		n := items[i].(*Integer).Inner
		n.Add(n, big1)
		n.Sub(n, big1)
	}
	check()
}
