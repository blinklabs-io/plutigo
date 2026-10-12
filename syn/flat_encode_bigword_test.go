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
	"math/big"
	"math/rand/v2"
	"testing"
)

// bigWordReference is the original shift-based base-128 encoder, kept as the
// byte-for-byte oracle for bigWord.
func bigWordReference(e *encoder, c *big.Int) {
	d := new(big.Int).Set(c)
	for {
		w := uint8(new(big.Int).Mod(d, big.NewInt(128)).Int64()) //nolint:gosec
		d.Rsh(d, 7)
		if d.Sign() != 0 {
			w |= 128
		}
		e.bits(8, w)
		if d.Sign() == 0 {
			return
		}
	}
}

// bigWordTestValues covers zero, every power of two up to 300 bits and its
// neighbours (crossing both the 7-bit group and the machine-word boundaries),
// and seeded random values up to 4096 bits.
func bigWordTestValues() []*big.Int {
	values := []*big.Int{big.NewInt(0)}
	one := big.NewInt(1)
	for k := uint(0); k <= 300; k++ {
		p := new(big.Int).Lsh(one, k)
		values = append(
			values,
			new(big.Int).Sub(p, one),
			p,
			new(big.Int).Add(p, one),
		)
	}
	rng := rand.New(rand.NewPCG(456, 456))
	for _, bitLen := range []int{65, 127, 128, 129, 448, 1000, 4096} {
		for range 8 {
			buf := make([]byte, (bitLen+7)/8)
			for i := range buf {
				buf[i] = byte(rng.Uint32())
			}
			v := new(big.Int).SetBytes(buf)
			v.SetBit(v, bitLen-1, 1)
			for i := bitLen; i < len(buf)*8; i++ {
				v.SetBit(v, i, 0)
			}
			values = append(values, v)
		}
	}
	return values
}

// TestBigWordMatchesReference checks that bigWord writes exactly the bits the
// original encoder wrote, starting from every bit offset within a byte, and
// leaves its input unchanged.
func TestBigWordMatchesReference(t *testing.T) {
	for _, v := range bigWordTestValues() {
		for offset := range 8 {
			checkBigWordMatchesReference(t, v, offset)
		}
	}
}

// FuzzBigWordMatchesReference feeds fuzzer-chosen integers of up to 1024
// bytes, at a fuzzer-chosen starting bit offset, through bigWord and the
// original encoder, requiring identical output and an unchanged input. It
// also round-trips the value, and its negation, through integer and the
// decoder.
func FuzzBigWordMatchesReference(f *testing.F) {
	f.Add([]byte{}, uint8(0))
	f.Add([]byte{0x7f}, uint8(3))
	f.Add([]byte{0x80}, uint8(7))
	f.Add(bytes.Repeat([]byte{0xff}, 8), uint8(1))
	f.Add(append([]byte{0x01}, make([]byte, 8)...), uint8(5))
	f.Add(bytes.Repeat([]byte{0xa5}, 129), uint8(6))
	f.Fuzz(func(t *testing.T, magnitude []byte, offset uint8) {
		if len(magnitude) > 1024 {
			t.Skip()
		}
		v := new(big.Int).SetBytes(magnitude)
		checkBigWordMatchesReference(t, v, int(offset%8))
		for _, n := range []*big.Int{v, new(big.Int).Neg(v)} {
			e := newEncoder()
			e.integer(n)
			e.filler()
			got, err := newDecoder(e.buffer).integer()
			if err != nil {
				t.Fatalf("decode %s: %v", n, err)
			}
			if got.Cmp(n) != 0 {
				t.Fatalf("round trip of %s decoded %s", n, got)
			}
		}
	})
}

// checkBigWordMatchesReference encodes v after offset leading bits with both
// bigWord and bigWordReference, and fails unless the buffers and the partial
// trailing byte are identical and v is unchanged.
func checkBigWordMatchesReference(t *testing.T, v *big.Int, offset int) {
	t.Helper()
	got, want := newEncoder(), newEncoder()
	for range offset {
		got.one()
		want.one()
	}
	before := new(big.Int).Set(v)
	got.bigWord(v)
	bigWordReference(want, v)
	if v.Cmp(before) != 0 {
		t.Fatalf("bigWord modified its input %s", before)
	}
	if !bytes.Equal(got.buffer, want.buffer) ||
		got.usedBits != want.usedBits ||
		got.currentByte != want.currentByte {
		t.Fatalf(
			"bigWord(%s) at bit offset %d = % x (+%d bits %08b), want % x (+%d bits %08b)",
			v, offset,
			got.buffer, got.usedBits, got.currentByte,
			want.buffer, want.usedBits, want.currentByte,
		)
	}
}

// TestIntegerRoundTrip encodes signed integers, including negatives that go
// through zigzag, and checks that the decoder reads back the same value.
func TestIntegerRoundTrip(t *testing.T) {
	for _, v := range bigWordTestValues() {
		for _, n := range []*big.Int{v, new(big.Int).Neg(v)} {
			e := newEncoder()
			e.integer(n)
			e.filler()
			got, err := newDecoder(e.buffer).integer()
			if err != nil {
				t.Fatalf("decode %s: %v", n, err)
			}
			if got.Cmp(n) != 0 {
				t.Fatalf("round trip of %s decoded %s", n, got)
			}
		}
	}
}

// TestBigWordLargeValueAllocations guards against per-group big.Int work:
// encoding a one-million-bit integer must not allocate per 7-bit group.
// The original encoder allocated twice per group (about 285,000 times here).
func TestBigWordLargeValueAllocations(t *testing.T) {
	v := new(big.Int).Lsh(big.NewInt(1), 1_000_000)
	v.Sub(v, big.NewInt(1))
	allocs := testing.AllocsPerRun(3, func() {
		newEncoder().bigWord(v)
	})
	if allocs > 100 {
		t.Fatalf("bigWord allocated %.0f times for a 1,000,000-bit value", allocs)
	}
}

func BenchmarkBigWordEncode(b *testing.B) {
	for _, bitLen := range []int{1_000, 10_000, 100_000, 1_000_000} {
		v := new(big.Int).Lsh(big.NewInt(1), uint(bitLen))
		v.Sub(v, big.NewInt(1))
		b.Run(big.NewInt(int64(bitLen)).String()+"_bits", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				newEncoder().bigWord(v)
			}
		})
	}
}
