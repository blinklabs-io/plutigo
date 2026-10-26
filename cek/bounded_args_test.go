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
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

func bigPow2(n uint) *big.Int { return new(big.Int).Lsh(big.NewInt(1), n) }

func addBig(x *big.Int, d int64) *big.Int { return new(big.Int).Add(x, big.NewInt(d)) }

// TestBoundedBuiltinArguments pins the van Rossem (semantics variants D and
// E) argument bounds of the reference, PlutusCore.Default.Universe.Cardano:
// CInteger arguments must lie in [-(2^262143), 2^262143-1] and CByteString
// arguments must be at most 65536 bytes. Earlier variants are unbounded, and
// builtin results are never bounded.
func TestBoundedBuiltinArguments(t *testing.T) {
	t.Parallel()

	maxInt := addBig(bigPow2(262143), -1)
	minInt := new(big.Int).Neg(bigPow2(262143))
	// bs builds a byte string of n zero bytes without a large literal.
	var bs func(n int) string
	bs = func(n int) string {
		switch {
		case n <= 8192:
			return fmt.Sprintf("[(builtin replicateByte) (con integer %d) (con integer 0)]", n)
		default:
			return fmt.Sprintf("[(builtin appendByteString) %s %s]", bs(n/2), bs(n-n/2))
		}
	}
	ci := func(v *big.Int) string { return fmt.Sprintf("(con integer %s)", v) }
	type tc struct {
		name     string
		src      string
		v3Fails  bool // at protocol 11 (variant E)
		v1Fails  bool // at protocol 11 (variant D)
		anyFails bool // at protocol 10 (variants B and C)
	}
	tests := []tc{
		{name: "addInteger at maxBound", src: "[(builtin addInteger) " + ci(maxInt) + " (con integer 0)]"},
		{name: "addInteger above maxBound", src: "[(builtin addInteger) " + ci(bigPow2(262143)) + " (con integer 0)]", v3Fails: true, v1Fails: true},
		{name: "subtractInteger second above maxBound", src: "[(builtin subtractInteger) (con integer 0) " + ci(bigPow2(262143)) + "]", v3Fails: true, v1Fails: true},
		{name: "multiplyInteger at minBound", src: "[(builtin multiplyInteger) " + ci(minInt) + " (con integer 1)]"},
		{name: "multiplyInteger below minBound", src: "[(builtin multiplyInteger) " + ci(addBig(minInt, -1)) + " (con integer 1)]", v3Fails: true, v1Fails: true},
		{name: "multiplyInteger result beyond the bound", src: "[(builtin multiplyInteger) " + ci(maxInt) + " " + ci(maxInt) + "]"},
		{name: "lessThanInteger above maxBound", src: "[(builtin lessThanInteger) " + ci(bigPow2(262143)) + " (con integer 0)]", v3Fails: true, v1Fails: true},
		{name: "equalsInteger is unbounded", src: "[(builtin equalsInteger) " + ci(bigPow2(262143)) + " (con integer 0)]"},
		{name: "divideInteger above maxBound", src: "[(builtin divideInteger) " + ci(bigPow2(262144)) + " (con integer 3)]", v3Fails: true, v1Fails: true},
		{name: "sha2_256 of 65536 bytes", src: "[(builtin sha2_256) " + bs(65536) + "]"},
		{name: "sha2_256 of 65537 bytes", src: "[(builtin sha2_256) " + bs(65537) + "]", v3Fails: true, v1Fails: true},
		{name: "appendByteString result beyond the bound", src: "[(builtin lengthOfByteString) [(builtin appendByteString) " + bs(65536) + " " + bs(65536) + "]]"},
		{name: "appendByteString of 65537 bytes", src: "[(builtin appendByteString) " + bs(65537) + " (con bytestring #)]", v3Fails: true, v1Fails: true},
		{name: "equalsByteString second of 65537 bytes", src: "[(builtin equalsByteString) (con bytestring #) " + bs(65537) + "]", v3Fails: true, v1Fails: true},
		{name: "lengthOfByteString is unbounded", src: "[(builtin lengthOfByteString) " + bs(65537) + "]"},
		{name: "sliceByteString of 65537 bytes", src: "[(builtin sliceByteString) (con integer 0) (con integer 1) " + bs(65537) + "]", v3Fails: true, v1Fails: true},
		{name: "andByteString of 65537 bytes", src: "[(builtin andByteString) (con bool False) " + bs(65537) + " (con bytestring #00)]", v3Fails: true, v1Fails: true},
		{name: "verifyEd25519Signature message of 65537 bytes", src: "[(builtin verifyEd25519Signature) " + bs(32) + " " + bs(65537) + " " + bs(64) + "]", v3Fails: true, v1Fails: true},
		{name: "consByteString integer above maxBound", src: "[(builtin consByteString) " + ci(bigPow2(262143)) + " (con bytestring #)]", v3Fails: true, v1Fails: true},
		{name: "consByteString integer 256", src: "[(builtin consByteString) (con integer 256) (con bytestring #)]", v3Fails: true},
		// sliceByteString unlifts skip and take as Int in every variant.
		{name: "sliceByteString skip beyond Int64", src: "[(builtin sliceByteString) " + ci(bigPow2(63)) + " (con integer 1) (con bytestring #0102)]", v3Fails: true, v1Fails: true, anyFails: true},
		{name: "sliceByteString take beyond Int64", src: "[(builtin sliceByteString) (con integer 0) " + ci(new(big.Int).Neg(addBig(bigPow2(63), 1))) + " (con bytestring #0102)]", v3Fails: true, v1Fails: true, anyFails: true},
		{name: "sliceByteString Int64 extremes", src: "[(builtin sliceByteString) " + ci(addBig(bigPow2(63), -1)) + " " + ci(new(big.Int).Neg(bigPow2(63))) + " (con bytestring #0102)]"},
		// scalarMul is bounded like multiScalarMul from van Rossem.
		{name: "G1 scalarMul at 2^4095-1", src: "[(builtin bls12_381_G1_scalarMul) " + ci(addBig(bigPow2(4095), -1)) + " [(builtin bls12_381_G1_hashToGroup) (con bytestring #) (con bytestring #00)]]"},
		{name: "G1 scalarMul at 2^4095", src: "[(builtin bls12_381_G1_scalarMul) " + ci(bigPow2(4095)) + " [(builtin bls12_381_G1_hashToGroup) (con bytestring #) (con bytestring #00)]]", v3Fails: true, v1Fails: true},
		{name: "G2 scalarMul at -(2^4095)-1", src: "[(builtin bls12_381_G2_scalarMul) " + ci(addBig(new(big.Int).Neg(bigPow2(4095)), -1)) + " [(builtin bls12_381_G2_hashToGroup) (con bytestring #) (con bytestring #00)]]", v3Fails: true, v1Fails: true},
		{name: "G2 scalarMul at -(2^4095)", src: "[(builtin bls12_381_G2_scalarMul) " + ci(new(big.Int).Neg(bigPow2(4095))) + " [(builtin bls12_381_G2_hashToGroup) (con bytestring #) (con bytestring #00)]]"},
	}
	for _, tt := range tests {
		for _, run := range []struct {
			language lang.LanguageVersion
			proto    uint
			wantFail bool
		}{
			{lang.LanguageVersionV3, 11, tt.v3Fails},
			{lang.LanguageVersionV1, 11, tt.v1Fails},
			// V3 unlifts the consByteString byte as a Word8 in variant C.
			{lang.LanguageVersionV3, 10, tt.anyFails || strings.HasPrefix(tt.name, "consByteString integer")},
		} {
			src := "(program 1.0.0 " + tt.src + ")"
			_, err := runTextProgramIn(t, run.language, src, run.proto)
			if (err != nil) != run.wantFail {
				t.Fatalf("%s (%v, protocol %d): error = %v, want failure %v",
					tt.name, run.language, run.proto, err, run.wantFail)
			}
		}
	}
}

// TestExpModIntegerBounds pins PlutusCore.Crypto.ExpMod.expMod: the modulus
// must lie in [1, 2^8191-1]; a modulus of 1 gives 0 before any other check;
// zero to a negative power fails; base and exponent must lie in
// [-(2^8191), 2^8191-1].
func TestExpModIntegerBounds(t *testing.T) {
	t.Parallel()

	lim := bigPow2(8191)
	neg := func(x *big.Int) *big.Int { return new(big.Int).Neg(x) }
	tests := []struct {
		b, e, m *big.Int
		want    string // "" means failure
	}{
		{big.NewInt(2), big.NewInt(3), addBig(lim, -1), "(con integer 8)"},
		{big.NewInt(2), big.NewInt(3), lim, ""},
		{big.NewInt(2), big.NewInt(3), big.NewInt(0), ""},
		{big.NewInt(2), big.NewInt(3), big.NewInt(-7), ""},
		{lim, lim, big.NewInt(1), "(con integer 0)"},
		{big.NewInt(0), big.NewInt(-1), big.NewInt(7), ""},
		{big.NewInt(2), lim, big.NewInt(7), ""},
		{big.NewInt(2), addBig(lim, -1), big.NewInt(7), "(con integer 2)"},
		{neg(addBig(lim, 1)), big.NewInt(0), big.NewInt(7), ""},
		{neg(lim), big.NewInt(0), big.NewInt(7), "(con integer 1)"},
		{big.NewInt(3), big.NewInt(-1), big.NewInt(7), "(con integer 5)"},
		{big.NewInt(2), big.NewInt(-1), big.NewInt(4), ""},
	}
	for _, tt := range tests {
		src := fmt.Sprintf(
			"(program 1.0.0 [(builtin expModInteger) (con integer %s) (con integer %s) (con integer %s)])",
			tt.b, tt.e, tt.m,
		)
		got, err := runTextProgram(t, src, 11)
		if tt.want == "" {
			if err == nil {
				t.Fatalf("expMod(%d bits, %d bits, %d bits) = %s, want failure",
					tt.b.BitLen(), tt.e.BitLen(), tt.m.BitLen(), got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Fatalf("expMod(%d bits, %d bits, %d bits) = %q, %v; want %s",
				tt.b.BitLen(), tt.e.BitLen(), tt.m.BitLen(), got, err, tt.want)
		}
	}
}

// TestBoundedIntegerArgumentsEndAtProtocol12 pins that Dijkstra (variants F
// and G in the reference) unbounds integer arguments while byte strings stay
// bounded. Not verified on a node: PV12 cannot run on node 11.1.3.
func TestBoundedIntegerArgumentsEndAtProtocol12(t *testing.T) {
	t.Parallel()

	big := bigPow2(262143).String()
	src := "(program 1.1.0 [[(builtin addInteger) (con integer " + big + ")] (con integer 1)])"
	if _, err := runTextProgramIn(t, lang.LanguageVersionV3, src, 11); err == nil {
		t.Fatal("protocol 11: expected out-of-bound integer to fail")
	}
	if _, err := runTextProgramIn(t, lang.LanguageVersionV3, src, 12); err != nil {
		t.Fatalf("protocol 12: integer bounds must not apply: %v", err)
	}
}
