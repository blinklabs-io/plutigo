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
	"math/rand"
	"testing"
)

// shiftBytesOracle is the original bit-array implementation of
// shiftByteString, kept as a reference for differential testing.
// Requires |shift| < 8*len(in).
func shiftBytesOracle(in []byte, shiftVal int) []byte {
	totalBits := len(in) * 8
	bits := make([]bool, totalBits)
	for i := range totalBits {
		bits[i] = (in[i/8] & (1 << (7 - (i % 8)))) != 0
	}
	resultBits := make([]bool, totalBits)
	if shiftVal > 0 {
		for i := range totalBits - shiftVal {
			resultBits[i] = bits[i+shiftVal]
		}
	} else {
		shiftAmount := -shiftVal
		for i := shiftAmount; i < totalBits; i++ {
			resultBits[i] = bits[i-shiftAmount]
		}
	}
	result := make([]byte, len(in))
	for i := range totalBits {
		if resultBits[i] {
			result[i/8] |= 1 << (7 - (i % 8))
		}
	}
	return result
}

func TestShiftBytesMatchesOracleExhaustiveSmall(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for n := 1; n <= 5; n++ {
		in := make([]byte, n)
		for iter := 0; iter < 50; iter++ {
			rng.Read(in)
			for s := -(n*8 - 1); s <= n*8-1; s++ {
				got := shiftBytes(in, s)
				want := shiftBytesOracle(in, s)
				if !bytes.Equal(got, want) {
					t.Fatalf("n=%d in=%x shift=%d got=%x want=%x", n, in, s, got, want)
				}
			}
		}
	}
}

func TestShiftBytesMatchesOracleRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for iter := 0; iter < 2000; iter++ {
		n := 1 + rng.Intn(300)
		in := make([]byte, n)
		rng.Read(in)
		s := rng.Intn(n*16-1) - (n*8 - 1)
		got := shiftBytes(in, s)
		want := shiftBytesOracle(in, s)
		if !bytes.Equal(got, want) {
			t.Fatalf("n=%d shift=%d mismatch", n, s)
		}
	}
}

func TestShiftBytesZeroShiftAndEmpty(t *testing.T) {
	in := []byte{0xde, 0xad}
	if got := shiftBytes(in, 0); !bytes.Equal(got, in) {
		t.Fatalf("zero shift changed bytes: %x", got)
	}
	if got := shiftBytes([]byte{}, 0); len(got) != 0 {
		t.Fatalf("empty input: %x", got)
	}
}

func BenchmarkShiftBytes(b *testing.B) {
	in := make([]byte, 4<<20)
	rand.New(rand.NewSource(3)).Read(in)
	b.Run("new", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = shiftBytes(in, 12345)
		}
	})
	b.Run("oracle", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = shiftBytesOracle(in, 12345)
		}
	})
}
