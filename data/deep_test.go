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
	"errors"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

// largeSize is the nesting depth, or width, of the large-data tests: deep
// enough that a walker recursing once per level overflows the stack limit
// of withSmallStack.
const largeSize = 50_000

// largeShape builds a PlutusData encoding of size n, and the canonical
// encoding serialiseData produces for it (Normalize then Encode).
type largeShape struct {
	name  string
	build func(n int) (raw, canonical []byte)
	// reencode, when set, is what Encode produces for the decoded value;
	// otherwise Encode reproduces raw.
	reencode func(n int) []byte
}

// nest returns n copies of open, the integer 0, then n copies of close.
func nest(open []byte, n int, close []byte) []byte {
	out := bytes.Repeat(open, n)
	out = append(out, 0)
	return append(out, bytes.Repeat(close, n)...)
}

// wide returns the shortest definite-length head of a majorType container
// claiming n entries, followed by n copies of entry.
func wide(majorType uint8, n int, entry []byte) []byte {
	var head bytes.Buffer
	appendCBORHead(&head, majorType, uint64(n))
	return append(head.Bytes(), bytes.Repeat(entry, n)...)
}

var largeShapes = []largeShape{
	{"definite list", func(n int) ([]byte, []byte) {
		return nest([]byte{0x81}, n, nil), nest([]byte{0x9f}, n, []byte{0xff})
	}, nil},
	{"indefinite list", func(n int) ([]byte, []byte) {
		b := nest([]byte{0x9f}, n, []byte{0xff})
		return b, b
	}, nil},
	{"constr", func(n int) ([]byte, []byte) {
		return nest([]byte{0xd8, 0x79, 0x81}, n, nil),
			nest([]byte{0xd8, 0x79, 0x9f}, n, []byte{0xff})
	}, nil},
	{"tag 102 constr", func(n int) ([]byte, []byte) {
		// 102([7, [child]]) re-encodes as tag 1280 (alternative 7).
		return nest([]byte{0xd8, 0x66, 0x82, 0x07, 0x81}, n, nil),
			nest([]byte{0xd9, 0x05, 0x00, 0x9f}, n, []byte{0xff})
	}, func(n int) []byte {
		return nest([]byte{0xd9, 0x05, 0x00, 0x81}, n, nil)
	}},
	{"map value", func(n int) ([]byte, []byte) {
		b := nest([]byte{0xa1, 0x00}, n, nil)
		return b, b
	}, nil},
	{"indefinite map key", func(n int) ([]byte, []byte) {
		// {{...: 0}: 0}: nesting through map keys.
		return nest([]byte{0xbf}, n, []byte{0x00, 0xff}),
			nest([]byte{0xa1}, n, []byte{0x00})
	}, nil},
	{"wide list", func(n int) ([]byte, []byte) {
		return wide(CborTypeArray, n, []byte{0}),
			append(append([]byte{0x9f}, make([]byte, n)...), 0xff)
	}, nil},
	{"wide map", func(n int) ([]byte, []byte) {
		b := wide(CborTypeMap, n, []byte{0, 0})
		return b, b
	}, nil},
}

// withSmallStack runs f with the goroutine stack limit lowered to 512 KiB,
// so a walker that recursed once per level of a largeSize-deep value (more
// than 1 MiB of stack even for a minimal frame) would abort the test binary
// instead of passing. The limit is process-wide, so callers must not run in
// parallel.
func withSmallStack(f func()) {
	old := debug.SetMaxStack(512 << 10)
	defer debug.SetMaxStack(old)
	f()
}

// TestDeepDataWalkers decodes deep and wide values with every decoder and
// runs every walker over them: decoding and walking use an explicit stack, so
// none of them may exhaust a small goroutine stack.
func TestDeepDataWalkers(t *testing.T) {
	n := largeSize
	for _, shape := range largeShapes {
		t.Run(shape.name, func(t *testing.T) {
			raw, canonical := shape.build(n)
			reencoded := raw
			if shape.reencode != nil {
				reencoded = shape.reencode(n)
			}
			withSmallStack(func() {
				decoded, err := Decode(raw)
				if err != nil {
					t.Fatalf("Decode: %v", err)
				}
				arena, err := NewDecoder().Decode(raw)
				if err != nil {
					t.Fatalf("Decoder.Decode: %v", err)
				}
				prefix, err := DecodePrefix(append(append([]byte{}, raw...), 0xff, 0x00))
				if err != nil {
					t.Fatalf("DecodePrefix: %v", err)
				}
				if !decoded.Equal(arena) || !arena.Equal(prefix) {
					t.Error("decoders disagree")
				}
				if err := CheckTree(decoded); err != nil {
					t.Errorf("CheckTree: %v", err)
				}
				if encoded, err := Encode(decoded); err != nil {
					t.Errorf("Encode: %v", err)
				} else if !bytes.Equal(encoded, reencoded) {
					t.Error("Encode does not reproduce the input")
				}
				if normalized, err := Encode(Normalize(decoded)); err != nil {
					t.Errorf("Encode(Normalize): %v", err)
				} else if !bytes.Equal(normalized, canonical) {
					t.Error("Encode(Normalize) is not canonical")
				}
				if !decoded.Clone().Equal(decoded) {
					t.Error("Clone differs")
				}
				if s := decoded.String(); len(s) < n {
					t.Error("String too short")
				}
				if _, err := EncodeJSON(decoded); err != nil {
					t.Errorf("EncodeJSON: %v", err)
				}
				var w PlutusDataWrapper
				if err := w.UnmarshalCBOR(raw); err != nil {
					t.Errorf("PlutusDataWrapper.UnmarshalCBOR: %v", err)
				} else if b, err := w.MarshalCBOR(); err != nil || !bytes.Equal(b, reencoded) {
					t.Error("PlutusDataWrapper round trip differs")
				}
			})
		})
	}
}

// TestDeepDataUnequalAtBottom checks Equal on deep values that differ only
// at the innermost leaf.
func TestDeepDataUnequalAtBottom(t *testing.T) {
	n := largeSize
	a := nest([]byte{0x81}, n, nil)
	b := append(a[:n:n], 1)
	withSmallStack(func() {
		x, errX := Decode(a)
		y, errY := Decode(b)
		if errX != nil || errY != nil {
			t.Fatalf("Decode: %v, %v", errX, errY)
		}
		if x.Equal(y) || y.Equal(x) {
			t.Error("Equal reports values differing at the bottom as equal")
		}
		if !strings.HasSuffix(y.String(), "Integer(1)"+strings.Repeat("]", n)) {
			t.Error("String does not end with the innermost leaf")
		}
	})
}

// TestDecodeMemoryLinear checks that decoding allocates memory in
// proportion to the input: a nested definite list (the most allocation per
// input byte: a List, its one-item slice and a frame per byte) and a wide
// list whose header claims as many items as bytes remain.
func TestDecodeMemoryLinear(t *testing.T) {
	n := largeSize
	// A header claiming more items than bytes remain is rejected before any
	// allocation sized from the claim.
	lying := []byte{0x9b, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00}
	if _, err := Decode(lying); err == nil {
		t.Fatal("Decode accepted a list claiming 2^63 items")
	}
	for name, raw := range map[string][]byte{
		"deep": nest([]byte{0x81}, n, nil),
		"wide": wide(CborTypeArray, n, []byte{0}),
	} {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		if _, err := Decode(raw); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		runtime.ReadMemStats(&after)
		perByte := float64(after.TotalAlloc-before.TotalAlloc) / float64(len(raw))
		t.Logf("%s: %d input bytes, %.0f bytes allocated per input byte", name, len(raw), perByte)
		if perByte > 256 {
			t.Errorf("%s: %.0f bytes allocated per input byte", name, perByte)
		}
	}
}

// TestCheckTreeRejectsSharedChildren checks the guard used before walking
// hand-built values without a node limit.
func TestCheckTreeRejectsSharedChildren(t *testing.T) {
	leaf := NewInteger(nil)
	shared := NewList(leaf)
	if err := CheckTree(NewList(shared, NewList(leaf))); err != nil {
		t.Fatalf("CheckTree rejected a tree: %v", err)
	}
	if err := CheckTree(NewList(shared, shared)); !errors.Is(err, ErrSharedNodes) {
		t.Fatalf("CheckTree on shared children = %v, want ErrSharedNodes", err)
	}
	cyclic := &List{Items: make([]PlutusData, 1)}
	cyclic.Items[0] = cyclic
	if err := CheckTree(cyclic); !errors.Is(err, ErrSharedNodes) {
		t.Fatalf("CheckTree on a cycle = %v, want ErrSharedNodes", err)
	}
}
