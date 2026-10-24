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
	"testing"
)

// TestDecodeMatchesReferenceDataDecoder pins PlutusCore.Data.decodeData
// behaviour on inputs a permissive CBOR decoder would treat differently.
func TestDecodeMatchesReferenceDataDecoder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		accept bool
	}{
		{"bignum one-byte head", "c24105", true},
		{"negative bignum one-byte head", "c34105", true},
		// Only the heads 0xc2/0xc3 are integers to cborg; a longer head
		// for tag 2 or 3 is a generic tag, an unknown constructor.
		{"bignum tag 2 two-byte head", "d8024105", false},
		{"bignum tag 3 three-byte head", "d900034105", false},
		// The reference Data type has no Value constructor (tag 1401).
		{"tag 1401", "d90579a0", false},
		{"trailing byte", "0100", false},
	}
	for _, tt := range tests {
		checkDecodersAccept(t, tt.name, tt.input, tt.accept)
	}
}

// TestDecodeMatchesNodeTransactionData encodes what a cardano-node 11.1.3
// (git 938cba99, protocol version 11.0, private devnet, 2026-10-24) did with
// transactions whose redeemer or inline output datum carried these CBOR
// bytes, submitted raw over local-tx-submission. The node rejected every
// tag 1401/1402 item while deserialising the transaction ("Invalid CBOR for
// Data ... Unrecognized tag 1401") and accepted the others.
func TestDecodeMatchesNodeTransactionData(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		accept bool
	}{
		{"redeemer tag 1401 empty map", "d90579a0", false},
		{"redeemer tag 1401 value", "d90579a141aaa1410101", false},
		{"redeemer tag 1401 inside list", "9fd90579a0ff", false},
		{"redeemer tag 1402", "d9057a80", false},
		{"redeemer indefinite list", "9f01ff", true},
		{"redeemer constr 0 [1]", "d8798101", true},
	}
	for _, tt := range tests {
		checkDecodersAccept(t, tt.name, tt.input, tt.accept)
	}
}

// checkDecodersAccept checks that Decode and Decoder.Decode both accept, or
// both reject, the hex-encoded CBOR input.
func checkDecodersAccept(t *testing.T, name, input string, accept bool) {
	t.Helper()
	b, err := hex.DecodeString(input)
	if err != nil {
		t.Fatal(err)
	}
	_, pkgErr := Decode(b)
	_, arenaErr := NewDecoder().Decode(b)
	for decoder, err := range map[string]error{"package": pkgErr, "arena": arenaErr} {
		if (err == nil) != accept {
			t.Errorf("%s (%s): error = %v, want accept %v", name, decoder, err, accept)
		}
	}
}

// TestDecodePrefixIgnoresTrailingBytes pins the decoding of a Data constant
// inside a FLAT program (FlatViaSerialise / deserialiseOrFail): bytes after
// the first item are ignored, whether or not they are valid CBOR, while the
// item itself is decoded with every usual rule.
func TestDecodePrefixIgnoresTrailingBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		want   PlutusData
		accept bool
	}{
		{"0100", NewInteger(big.NewInt(1)), true},
		{"01ff", NewInteger(big.NewInt(1)), true},
		{"d87980ff", NewConstr(0), true},
		{"d8024105", nil, false},
		{"", nil, false},
		{"5841" + "00", nil, false}, // truncated byte string
	}
	for _, tt := range tests {
		b, err := hex.DecodeString(tt.input)
		if err != nil {
			t.Fatal(err)
		}
		got, pkgErr := DecodePrefix(b)
		gotArena, arenaErr := NewDecoder().DecodePrefix(b)
		if !tt.accept {
			if pkgErr == nil || arenaErr == nil {
				t.Errorf("%s: accepted (%v, %v)", tt.input, pkgErr, arenaErr)
			}
			continue
		}
		if pkgErr != nil || arenaErr != nil {
			t.Fatalf("%s: %v, %v", tt.input, pkgErr, arenaErr)
		}
		if !got.Equal(tt.want) || !gotArena.Equal(tt.want) {
			t.Fatalf("%s: got %v / %v, want %v", tt.input, got, gotArena, tt.want)
		}
	}
}
