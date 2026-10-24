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

package syn

import (
	"bytes"
	"reflect"
	"runtime"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

type ampBitWriter struct {
	b []byte
	n int
}

func (w *ampBitWriter) bit(v int) {
	if w.n%8 == 0 {
		w.b = append(w.b, 0)
	}
	if v != 0 {
		w.b[len(w.b)-1] |= 128 >> (w.n % 8)
	}
	w.n++
}

func (w *ampBitWriter) bits(v byte, k int) {
	for i := k - 1; i >= 0; i-- {
		w.bit(int(v>>i) & 1)
	}
}

// pairChainTags returns the type tags of
// pair unit (pair unit (... unit)) nested depth levels deep.
func pairChainTags(depth int) []byte {
	tags := make([]byte, 0, 4*depth+1)
	for range depth {
		tags = append(tags, ProtoPairOneTag, ProtoPairTwoTag, ProtoPairThreeTag, UnitTag)
	}
	return append(tags, UnitTag)
}

// buildConstantProgram builds `(program 1.0.0 (con T v))` from the type tags
// of T and a writer of v's FLAT bits.
func buildConstantProgram(tags []byte, value func(w *ampBitWriter)) []byte {
	w := &ampBitWriter{b: []byte{1, 0, 0}, n: 24}
	w.bits(ConstantTag, 4)
	for _, tag := range tags {
		w.bit(1)
		w.bits(tag, 4)
	}
	w.bit(0)
	value(w)
	for w.n%8 != 7 {
		w.bit(0)
	}
	w.bit(1)
	return w.b
}

// buildAmplifyProgram builds `(program 1.0.0 (con (list T) [..items..]))`
// where T is a pair-of-unit chain nested depth levels deep. Each item is
// encoded as a single bit, so a decoder that materializes every item
// separately allocates depth nodes per input bit.
func buildAmplifyProgram(depth, items int) []byte {
	tags := append([]byte{ProtoListOneTag, ProtoListTwoTag}, pairChainTags(depth)...)
	return buildConstantProgram(tags, func(w *ampBitWriter) {
		for range items {
			w.bit(1)
		}
		w.bit(0)
	})
}

// buildNestedAmplifyProgram builds a constant of type
// list (list T) holding items singleton inner lists, with T as in
// buildAmplifyProgram, so each element costs three bits.
func buildNestedAmplifyProgram(depth, items int) []byte {
	tags := append(
		[]byte{ProtoListOneTag, ProtoListTwoTag, ProtoListOneTag, ProtoListTwoTag},
		pairChainTags(depth)...,
	)
	return buildConstantProgram(tags, func(w *ampBitWriter) {
		for range items {
			w.bit(1) // outer item
			w.bit(1) // inner item
			w.bit(0) // inner end
		}
		w.bit(0)
	})
}

// buildMixedAmplifyProgram builds a constant of type
// array (pair bool T) with T as in buildAmplifyProgram, so a zero-width
// value sits under a pair that is not zero-width and each element costs two
// bits.
func buildMixedAmplifyProgram(depth, items int) []byte {
	tags := append(
		[]byte{ProtoListOneTag, ProtoArrayTag, ProtoPairOneTag, ProtoPairTwoTag, ProtoPairThreeTag, BoolTag},
		pairChainTags(depth)...,
	)
	return buildConstantProgram(tags, func(w *ampBitWriter) {
		for i := range items {
			w.bit(1)
			w.bit(i & 1)
		}
		w.bit(0)
	})
}

// balancedPairType returns pair^depth unit: a complete binary tree of pairs
// with unit leaves.
func balancedPairType(depth int) Typ {
	if depth == 0 {
		return &TUnit{}
	}
	return &TPair{First: balancedPairType(depth - 1), Second: balancedPairType(depth - 1)}
}

func balancedPairValue(depth int) IConstant {
	if depth == 0 {
		return &Unit{}
	}
	first, second := balancedPairValue(depth-1), balancedPairValue(depth-1)
	return &ProtoPair{
		FstType: first.Typ(),
		SndType: second.Typ(),
		First:   first,
		Second:  second,
	}
}

// zeroWidthDecodePaths are the two FLAT decoder implementations: the
// arena-backed De Bruijn decoder and the generic binder decoder.
var zeroWidthDecodePaths = []struct {
	name   string
	decode func([]byte) (Term[DeBruijn], func() ([]byte, error), error)
}{
	{"arena", func(b []byte) (Term[DeBruijn], func() ([]byte, error), error) {
		p, err := DecodeDeBruijn(b)
		if err != nil {
			return nil, nil, err
		}
		return p.Term, func() ([]byte, error) { return Encode(p) }, nil
	}},
	{"generic-named-debruijn", func(b []byte) (Term[DeBruijn], func() ([]byte, error), error) {
		p, err := Decode[NamedDeBruijn](b)
		if err != nil {
			return nil, nil, err
		}
		c, ok := p.Term.(*Constant)
		if !ok {
			return nil, nil, nil
		}
		return c, func() ([]byte, error) { return Encode(p) }, nil
	}},
}

// TestDecodeZeroWidthListOfPairsIsAccepted guards against charging unit and
// pair values against the program node budget: they occupy no FLAT bits, and
// the reference decoder accepts lists of them up to the item limit.
func TestDecodeZeroWidthListOfPairsIsAccepted(t *testing.T) {
	tests := []struct {
		depth int
		items int
	}{
		{depth: 5, items: 30_000},
		{depth: 4, items: 40_000},
	}
	for _, tc := range tests {
		elemType := balancedPairType(tc.depth)
		elem := balancedPairValue(tc.depth)
		list := make([]IConstant, tc.items)
		for i := range list {
			list[i] = elem
		}
		want := &ProtoList{LTyp: elemType, List: list}
		input, err := Encode(&Program[DeBruijn]{
			Version: [3]uint32{1, 1, 0},
			Term:    &Constant{Con: want},
		})
		if err != nil {
			t.Fatalf("encode depth %d: %v", tc.depth, err)
		}
		for _, path := range zeroWidthDecodePaths {
			t.Run(path.name, func(t *testing.T) {
				term, encode, err := path.decode(input)
				if err != nil {
					t.Fatalf("depth %d x %d (%d bytes): %v", tc.depth, tc.items, len(input), err)
				}
				c, ok := term.(*Constant)
				if !ok {
					t.Fatalf("decoded term is %T, want *Constant", term)
				}
				got, ok := c.Con.(*ProtoList)
				if !ok {
					t.Fatalf("decoded constant is %T, want *ProtoList", c.Con)
				}
				if !reflect.DeepEqual(got.LTyp, want.LTyp) {
					t.Fatalf("decoded list type %v, want %v", got.LTyp, want.LTyp)
				}
				if len(got.List) != tc.items {
					t.Fatalf("decoded %d items, want %d", len(got.List), tc.items)
				}
				for i, item := range got.List {
					if i > 0 && item == got.List[0] {
						continue
					}
					if !reflect.DeepEqual(item, elem) {
						t.Fatalf("item %d = %v, want %v", i, item, elem)
					}
				}
				reencoded, err := encode()
				if err != nil {
					t.Fatalf("re-encode: %v", err)
				}
				if !bytes.Equal(reencoded, input) {
					t.Fatal("decode/encode round trip changed the program")
				}
				if _, err := DecodeDeBruijnWithContext(input, ProgramContext{
					LedgerLanguage: lang.LanguageVersionV3,
					ProtocolMajor:  10,
				}); err != nil {
					t.Fatalf("validated decode: %v", err)
				}
			})
		}
	}
}

// TestDecodeConstantValueAmplificationBounded checks that a list of
// zero-width values, each encoded in a bit or two, decodes with allocation
// linear in the input size rather than the input size times the value size.
func TestDecodeConstantValueAmplificationBounded(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		depth int
		items int
		check func(t *testing.T, con IConstant, depth, items int)
	}{
		{"list-d1000-n10000", buildAmplifyProgram(1000, 10_000), 1000, 10_000, checkAmplifyList},
		{"list-d400-n4000", buildAmplifyProgram(400, 4000), 400, 4000, checkAmplifyList},
		{"nested-list-d1000-n10000", buildNestedAmplifyProgram(1000, 10_000), 1000, 10_000, checkNestedAmplifyList},
		{"array-of-mixed-pair-d1000-n10000", buildMixedAmplifyProgram(1000, 10_000), 1000, 10_000, checkMixedAmplifyArray},
	}
	for _, tc := range tests {
		for _, path := range zeroWidthDecodePaths {
			t.Run(tc.name+"/"+path.name, func(t *testing.T) {
				// TotalAlloc is process-wide; this test and its subtests are
				// not parallel, so no other test runs during the
				// measurement.
				runtime.GC()
				var m0, m1 runtime.MemStats
				runtime.ReadMemStats(&m0)
				term, _, err := path.decode(tc.input)
				runtime.ReadMemStats(&m1)
				if err != nil {
					t.Fatalf("decode %d byte input: %v", len(tc.input), err)
				}
				// Before memoization these inputs allocated hundreds of
				// MB (d1000/n10000: 683 MB from 3.7 KB). Linear decoding
				// needs a few tens of bytes per item plus the type tree.
				alloc := m1.TotalAlloc - m0.TotalAlloc
				if limit := uint64(8 << 20); alloc > limit {
					t.Fatalf(
						"decode allocated %d KB for %d byte input, limit %d KB",
						alloc>>10,
						len(tc.input),
						limit>>10,
					)
				}
				c, ok := term.(*Constant)
				if !ok {
					t.Fatalf("decoded term is %T, want *Constant", term)
				}
				tc.check(t, c.Con, tc.depth, tc.items)
			})
		}
		if _, err := DecodeDeBruijnWithContext(tc.input, ProgramContext{
			LedgerLanguage: lang.LanguageVersionV3,
			ProtocolMajor:  10,
		}); err != nil {
			t.Fatalf("%s: validated decode: %v", tc.name, err)
		}
	}
}

func checkAmplifyList(t *testing.T, con IConstant, depth, items int) {
	t.Helper()
	list, ok := con.(*ProtoList)
	if !ok || len(list.List) != items {
		t.Fatalf("decoded %T, want a list of %d items", con, items)
	}
	checkPairChain(t, list.List[0], depth)
	checkPairChain(t, list.List[items-1], depth)
}

func checkNestedAmplifyList(t *testing.T, con IConstant, depth, items int) {
	t.Helper()
	list, ok := con.(*ProtoList)
	if !ok || len(list.List) != items {
		t.Fatalf("decoded %T, want a list of %d items", con, items)
	}
	for _, i := range []int{0, items - 1} {
		inner, ok := list.List[i].(*ProtoList)
		if !ok || len(inner.List) != 1 {
			t.Fatalf("outer item %d is %T, want a singleton list", i, list.List[i])
		}
		checkPairChain(t, inner.List[0], depth)
	}
}

func checkMixedAmplifyArray(t *testing.T, con IConstant, depth, items int) {
	t.Helper()
	array, ok := con.(*ProtoArray)
	if !ok || len(array.Array) != items {
		t.Fatalf("decoded %T, want an array of %d items", con, items)
	}
	for _, i := range []int{0, 1, items - 1} {
		pair, ok := array.Array[i].(*ProtoPair)
		if !ok {
			t.Fatalf("item %d is %T, want a pair", i, array.Array[i])
		}
		flag, ok := pair.First.(*Bool)
		if !ok || flag.Inner != (i&1 == 1) {
			t.Fatalf("item %d first = %v, want bool %v", i, pair.First, i&1 == 1)
		}
		checkPairChain(t, pair.Second, depth)
	}
}

// checkPairChain checks that con is pair unit (pair unit (... unit)) nested
// depth levels deep, with types matching values.
func checkPairChain(t *testing.T, con IConstant, depth int) {
	t.Helper()
	for level := range depth {
		pair, ok := con.(*ProtoPair)
		if !ok {
			t.Fatalf("level %d is %T, want a pair", level, con)
		}
		if _, ok := pair.First.(*Unit); !ok {
			t.Fatalf("level %d first is %T, want unit", level, pair.First)
		}
		if _, ok := pair.FstType.(*TUnit); !ok {
			t.Fatalf("level %d first type is %T, want unit", level, pair.FstType)
		}
		if _, ok := pair.SndType.(*TPair); !ok && level < depth-1 {
			t.Fatalf("level %d second type is %T, want pair", level, pair.SndType)
		}
		con = pair.Second
	}
	if _, ok := con.(*Unit); !ok {
		t.Fatalf("innermost value is %T, want unit", con)
	}
}
