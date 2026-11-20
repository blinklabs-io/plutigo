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
	"testing"
	"unsafe"
)

func TestDecodePrefixBorrowedMatchesDecodePrefix(t *testing.T) {
	inputs := []string{
		"d8799f4401020304ff",                   // constr 0 [#01020304], indefinite fields
		"d87a80",                               // constr 1 []
		"9f4100410141024103ff",                 // indefinite list of byte strings
		"a2410a410b410c410d",                   // map {#0a: #0b, #0c: #0d}
		"5f420102420304ff",                     // chunked byte string
		"c249010000000000000000",               // bignum 2^64
		"d8668218c882410a1903e8",               // constr 102: alternative 200 [#0a, 1000]
		"d8799f40d87a9f4568656c6c6fffa0ff0001", // nested, trailing bytes ignored
	}
	for _, input := range inputs {
		raw, err := hex.DecodeString(input)
		if err != nil {
			t.Fatal(err)
		}
		want, err := NewDecoder().DecodePrefix(raw)
		if err != nil {
			t.Fatalf("%s: DecodePrefix() error = %v", input, err)
		}
		got, err := NewDecoder().DecodePrefixBorrowed(raw)
		if err != nil {
			t.Fatalf("%s: DecodePrefixBorrowed() error = %v", input, err)
		}
		if !want.Equal(got) {
			t.Fatalf("%s: DecodePrefixBorrowed() = %v, DecodePrefix() = %v", input, got, want)
		}
	}
}

func TestDecodePrefixBorrowedSharesDefiniteByteStrings(t *testing.T) {
	// [#0102, #0304]: each byte string is a slice of the input.
	raw := []byte{0x82, 0x42, 0x01, 0x02, 0x42, 0x03, 0x04}
	decoder := NewDecoder()
	got, err := decoder.DecodePrefixBorrowed(raw)
	if err != nil {
		t.Fatal(err)
	}
	items := got.(*List).Items()
	first := items[0].(*ByteString).Inner
	second := items[1].(*ByteString).Inner
	if unsafe.SliceData(first) != &raw[2] || unsafe.SliceData(second) != &raw[5] {
		t.Fatal("definite byte strings are not slices of the input")
	}
	if cap(first) != len(first) {
		t.Fatalf("borrowed byte string capacity = %d, want its length %d", cap(first), len(first))
	}
	// Appending to one byte string must not overwrite the next.
	_ = append(first, 0xff)
	if !bytes.Equal(second, []byte{0x03, 0x04}) {
		t.Fatalf("append to a borrowed byte string changed its neighbour: %x", second)
	}

	// The same decoder copies again in DecodePrefix.
	copied, err := decoder.DecodePrefix(raw)
	if err != nil {
		t.Fatal(err)
	}
	inner := copied.(*List).Items()[0].(*ByteString).Inner
	if unsafe.SliceData(inner) == &raw[2] {
		t.Fatal("DecodePrefix after DecodePrefixBorrowed shares the input")
	}
}

func TestDecodePrefixBorrowedCopiesChunkedByteStrings(t *testing.T) {
	// (_ #0102, #0304): a chunked byte string is joined into a copy.
	raw := []byte{0x5f, 0x42, 0x01, 0x02, 0x42, 0x03, 0x04, 0xff}
	got, err := NewDecoder().DecodePrefixBorrowed(raw)
	if err != nil {
		t.Fatal(err)
	}
	inner := got.(*ByteString).Inner
	if !bytes.Equal(inner, []byte{1, 2, 3, 4}) {
		t.Fatalf("chunked byte string = %x, want 01020304", inner)
	}
	raw[2] = 0xee
	if inner[0] != 0x01 {
		t.Fatal("chunked byte string shares the input")
	}
}
