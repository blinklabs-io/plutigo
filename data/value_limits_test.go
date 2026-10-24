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
	"encoding/binary"
	"testing"
)

// bigValueCBOR returns a Value with the given number of single-token
// policies, encoded with 4-byte ascending keys.
func bigValueCBOR(policies int) []byte {
	out := []byte{0xd9, 0x05, 0x79, 0xba}
	out = binary.BigEndian.AppendUint32(out, uint32(policies))
	for i := range policies {
		out = append(out, 0x44)
		out = binary.BigEndian.AppendUint32(out, uint32(i))
		out = append(out, 0xa1, 0x41, 0x01, 0x01)
	}
	return out
}

// TestValueDecodersAgree checks that a small and a wide Value decode to the
// same value through every entry point that admits Value: there is no node
// limit.
func TestValueDecodersAgree(t *testing.T) {
	t.Parallel()
	for _, policies := range []int{1, largeSize} {
		encoded := bigValueCBOR(policies)
		pkg, err := DecodeWithValue(encoded)
		if err != nil {
			t.Fatalf("%d policies: Decode: %v", policies, err)
		}
		arena, err := NewDecoder().DecodeWithValue(encoded)
		if err != nil {
			t.Fatalf("%d policies: arena Decode: %v", policies, err)
		}
		var v Value
		if err := v.UnmarshalCBOR(encoded); err != nil {
			t.Fatalf("%d policies: UnmarshalCBOR: %v", policies, err)
		}
		if !pkg.Equal(arena) || !pkg.Equal(&v) {
			t.Fatalf("%d policies: decoders disagree", policies)
		}
	}
}
