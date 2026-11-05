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
	"bytes"
	"encoding/hex"
	"math/big"
	"math/rand"
	"testing"
)

// requireNormalizeSharedMatches checks that NormalizeShared(pd) encodes to
// Normalize(pd)'s bytes, holds no container in value form, and is pd itself
// exactly when pd was already normalized.
func requireNormalizeSharedMatches(
	t *testing.T,
	pd PlutusData,
	wantShared bool,
) {
	t.Helper()
	want, err := Encode(Normalize(pd))
	if err != nil {
		t.Fatal(err)
	}
	shared := NormalizeShared(pd)
	got, err := Encode(shared)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("NormalizeShared encodes %x, Normalize %x", got, want)
	}
	if !isNormalized(shared) {
		t.Fatalf("NormalizeShared result %x is not normalized", got)
	}
	if same := sameData(shared, pd); same != wantShared {
		t.Fatalf("shared %v, want %v", same, wantShared)
	}
}

func sameData(a, b PlutusData) bool {
	switch av := a.(type) {
	case *Constr:
		bv, ok := b.(*Constr)
		return ok && av == bv
	case *List:
		bv, ok := b.(*List)
		return ok && av == bv
	case *Map:
		bv, ok := b.(*Map)
		return ok && av == bv
	case *Value:
		bv, ok := b.(*Value)
		return ok && av == bv
	case *Integer:
		bv, ok := b.(*Integer)
		return ok && av == bv
	case *ByteString:
		bv, ok := b.(*ByteString)
		return ok && av == bv
	}
	return false
}

func TestNormalizeSharedDecodedEncodings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cbor   string
		shared bool
	}{
		{"indefinite list", "9f0102ff", true},
		{"definite list", "820102", false},
		{"empty definite list", "80", true},
		{"empty indefinite list", "9fff", true},
		{"indefinite constr", "d8799f01ff", true},
		{"definite constr", "d8798101", false},
		{"empty constr", "d87980", true},
		{"definite map", "a10102", true},
		{"definite list key", "a182010201", false},
		{"indefinite map", "bf0102ff", false},
		{"empty indefinite map", "bfff", false},
		{"canonical nested", "d8799f9f01ffa1029f03ffff", true},
		{"definite list deep inside", "d8799f9f01ffa102820304ff", false},
		{"integer", "01", true},
		{"bytes", "4101", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := hex.DecodeString(tc.cbor)
			if err != nil {
				t.Fatal(err)
			}
			pd, err := Decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			requireNormalizeSharedMatches(t, pd, tc.shared)
		})
	}
}

func TestNormalizeSharedValueFormContainers(t *testing.T) {
	one := NewInteger(big.NewInt(1))
	for name, pd := range map[string]PlutusData{
		"constr": Constr{Tag: big.NewInt(0), Fields: []PlutusData{one}},
		"list":   List{Items: []PlutusData{one}},
		"map":    Map{Pairs: [][2]PlutusData{{one, one}}},
		"nested": NewList(List{Items: []PlutusData{one}}),
	} {
		t.Run(name, func(t *testing.T) {
			requireNormalizeSharedMatches(t, pd, false)
		})
	}
}

func TestNormalizeSharedValue(t *testing.T) {
	policy := NewByteString(bytes.Repeat([]byte{0xaa}, 28))
	tokens := NewMap(
		[][2]PlutusData{
			{NewByteString([]byte("T")), NewInteger(big.NewInt(5))},
		},
	)
	inner, ok := NewMap([][2]PlutusData{{policy, tokens}}).(*Map)
	if !ok {
		t.Fatal("NewMap did not return *Map")
	}
	v, err := NewValue(inner)
	if err != nil {
		t.Fatal(err)
	}
	requireNormalizeSharedMatches(t, v, true)
	indefInner, ok := NewMapDefIndef(true, inner.Pairs).(*Map)
	if !ok {
		t.Fatal("NewMapDefIndef did not return *Map")
	}
	requireNormalizeSharedMatches(t, &Value{Inner: indefInner}, false)
	indefTokens := NewMapDefIndef(
		true,
		[][2]PlutusData{
			{NewByteString([]byte("T")), NewInteger(big.NewInt(5))},
		},
	)
	nestedInner, ok := NewMap([][2]PlutusData{{policy, indefTokens}}).(*Map)
	if !ok {
		t.Fatal("NewMap did not return *Map")
	}
	requireNormalizeSharedMatches(t, &Value{Inner: nestedInner}, false)
}

// randomData builds a tree of every node kind with every encoding choice.
func randomData(rng *rand.Rand, depth int) PlutusData {
	if depth == 0 || rng.Intn(4) == 0 {
		if rng.Intn(2) == 0 {
			return NewInteger(big.NewInt(rng.Int63n(1000) - 500))
		}
		b := make([]byte, rng.Intn(5))
		rng.Read(b)
		return NewByteString(b)
	}
	children := func() []PlutusData {
		items := make([]PlutusData, rng.Intn(4))
		for i := range items {
			items[i] = randomData(rng, depth-1)
		}
		return items
	}
	switch rng.Intn(7) {
	case 0:
		return NewConstrDefIndef(
			rng.Intn(2) == 0,
			uint64(rng.Intn(10)),
			children()...)
	case 1:
		return NewConstr(uint64(rng.Intn(10)), children()...)
	case 2:
		return NewListDefIndef(rng.Intn(2) == 0, children()...)
	case 3:
		return NewList(children()...)
	case 4:
		items := children()
		pairs := make([][2]PlutusData, len(items))
		for i, item := range items {
			pairs[i] = [2]PlutusData{NewInteger(big.NewInt(int64(i))), item}
		}
		if rng.Intn(2) == 0 {
			return NewMapDefIndef(rng.Intn(2) == 0, pairs)
		}
		return NewMap(pairs)
	case 5:
		return List{Items: children()}
	default:
		return Constr{Tag: big.NewInt(int64(rng.Intn(10))), Fields: children()}
	}
}

func TestNormalizeSharedMatchesNormalizeOnRandomData(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	shared := 0
	for range 5000 {
		pd := randomData(rng, 5)
		want, err := Encode(Normalize(pd))
		if err != nil {
			t.Fatal(err)
		}
		got, err := Encode(NormalizeShared(pd))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("NormalizeShared encodes %x, Normalize %x", got, want)
		}
		if sameData(NormalizeShared(pd), pd) {
			shared++
		}
		// Normalize's own output is always shared.
		normalized := Normalize(pd)
		if !sameData(NormalizeShared(normalized), normalized) {
			t.Fatalf("NormalizeShared copied the already normalized %x", want)
		}
	}
	if shared == 0 || shared == 5000 {
		t.Fatalf(
			"%d of 5000 random values shared; the corpus must mix both",
			shared,
		)
	}
}

func TestNormalizeSharedDoesNotAllocateForNormalizedData(t *testing.T) {
	raw, err := hex.DecodeString("d8799f9f01ffa1029f03ffff")
	if err != nil {
		t.Fatal(err)
	}
	pd, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(100, func() { _ = NormalizeShared(pd) }); allocs != 0 {
		t.Fatalf("NormalizeShared allocated %v times", allocs)
	}
}

func BenchmarkNormalizeShared(b *testing.B) {
	// A datum-sized value: a constructor of 40 fields, each a constructor
	// holding bytes, an integer and a two-entry map.
	fields := make([]PlutusData, 40)
	for i := range fields {
		fields[i] = NewConstr(uint64(i%3),
			NewByteString(bytes.Repeat([]byte{byte(i)}, 28)),
			NewInteger(big.NewInt(int64(i)*1_000_003)),
			NewMap([][2]PlutusData{
				{NewByteString([]byte("a")), NewInteger(big.NewInt(1))},
				{
					NewByteString([]byte("b")),
					NewList(NewInteger(big.NewInt(2))),
				},
			}),
		)
	}
	raw, err := Encode(NewConstr(0, fields...))
	if err != nil {
		b.Fatal(err)
	}
	decoded, err := Decode(raw)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("Normalize", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = Normalize(decoded)
		}
	})
	b.Run("NormalizeShared", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = NormalizeShared(decoded)
		}
	})
}

func TestNormalizeSharedWideContainersDoNotAllocate(t *testing.T) {
	items := make([]PlutusData, 1000)
	pairs := make([][2]PlutusData, 1000)
	for i := range items {
		items[i] = NewInteger(big.NewInt(int64(i)))
		pairs[i] = [2]PlutusData{items[i], items[i]}
	}
	wide := NewList(NewConstr(0, items...), NewMap(pairs))
	if got := NormalizeShared(wide); got != PlutusData(wide) {
		t.Fatal("expected wide normalized value to be returned as is")
	}
	if n := testing.AllocsPerRun(10, func() { _ = NormalizeShared(wide) }); n != 0 {
		t.Fatalf(
			"NormalizeShared allocated %v times on a wide normalized value",
			n,
		)
	}
}
