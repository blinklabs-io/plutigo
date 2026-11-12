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
	"fmt"
	"testing"
)

// TestFixedWidthBitReadsMatchBits8 checks the fixed-width bit readers, and
// the one-byte fast path of the variable index reader, against the general
// reader at every bit offset of every position, including the last byte and
// past the end of the buffer.
func TestFixedWidthBitReadsMatchBits8(t *testing.T) {
	buffers := [][]byte{
		{},
		{0x00},
		{0xff},
		{0xa5},
		{0x5a, 0x3c},
		{0x7f, 0x80},
		{0x12, 0xfe, 0x01},
		{0x81, 0x7f, 0xc3},
	}
	type state struct {
		value byte
		err   string
		pos   int
		used  int64
	}
	read := func(
		buf []byte,
		pos int,
		used int64,
		f func(*decoder) (byte, error),
	) state {
		d := &decoder{buffer: buf, pos: pos, usedBits: used}
		v, err := f(d)
		s := state{value: v, pos: d.pos, used: d.usedBits}
		if err != nil {
			// A failed read leaves the value undefined, but its effect on
			// the position must still match.
			s.value = 0
			s.err = err.Error()
		}
		return s
	}
	readers := []struct {
		name string
		got  func(*decoder) (byte, error)
		want func(*decoder) (byte, error)
	}{
		{
			name: "bits4",
			got:  (*decoder).bits4,
			want: func(d *decoder) (byte, error) { return d.bits8(4) },
		},
		{
			name: "bits7",
			got:  (*decoder).bits7,
			want: func(d *decoder) (byte, error) { return d.bits8(7) },
		},
		{
			name: "bit",
			got: func(d *decoder) (byte, error) {
				b, err := d.bit()
				if b {
					return 1, err
				}
				return 0, err
			},
			want: func(d *decoder) (byte, error) { return d.bits8(1) },
		},
		{
			name: "binder",
			got: func(d *decoder) (byte, error) {
				name, err := decodeDeBruijnBinder(d)
				return byte(name), err
			},
			want: func(d *decoder) (byte, error) {
				// The fast path covers single-byte indices; longer
				// ones fall through to word, as the reference does.
				name, err := d.word()
				return byte(name), err
			},
		},
	}
	for _, r := range readers {
		for _, buf := range buffers {
			for pos := 0; pos <= len(buf); pos++ {
				for used := range int64(8) {
					if pos == len(buf) && used != 0 {
						continue
					}
					name := fmt.Sprintf("%s/%x/pos=%d/used=%d", r.name, buf, pos, used)
					got := read(buf, pos, used, r.got)
					want := read(buf, pos, used, r.want)
					if got != want {
						t.Errorf("%s: got %+v, want %+v", name, got, want)
					}
				}
			}
		}
	}
}
