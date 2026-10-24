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
	"encoding/hex"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

// TestFlatDecodeMatchesReferenceRules pins FLAT decoding details where the
// reference (plutus-core's flat decoder and PlutusCore.Data) accepts or
// rejects inputs differently from a naive decoder. Each input is a whole
// program; want is the pretty-printed term, or "" for a decode failure.
func TestFlatDecodeMatchesReferenceRules(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Var indices and constr tags are Word64 (dWord64): at most ten
		// 7-bit groups, the tenth ending the varint with value 0 or 1.
		{"index in ten groups", "0100002081" + strings.Repeat("80", 8) + "0001", "(lam i_0 i_1)"},
		{"index in eleven groups", "0100002081" + strings.Repeat("80", 9) + "0001", ""},
		{"index tenth group above 1", "0100002081" + strings.Repeat("80", 8) + "0201", ""},
		// An index above MaxInt is a valid Word64; it is free at any scope.
		{"index 2^63+1", "0100002081" + strings.Repeat("80", 8) + "0101", "(lam i_0 i_9223372036854775807)"},
		{"constr tag in eleven groups", "010100880808080808080808080001", ""},
		// A Data constant is decoded with deserialiseOrFail, which ignores
		// the bytes after the first CBOR item, valid CBOR or not.
		{"data with trailing byte", "0100004c010201000001", "(con data (I 1))"},
		{"data with trailing break", "0100004c0104d87980ff0001", "(con data (Constr 0 []))"},
		// Only the one-byte heads 0xc2/0xc3 are bignums; tag 2 with a longer
		// head is an unknown tag to PlutusCore.Data.decodeData.
		{"bignum one-byte head", "0100004c0103c241050001", "(con data (I 5))"},
		{"bignum two-byte head", "0100004c0104d80241050001", ""},
	}
	ctx := ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: 10}
	for _, tt := range tests {
		input, err := hex.DecodeString(tt.input)
		if err != nil {
			t.Fatalf("%s: bad hex: %v", tt.name, err)
		}
		for path, decode := range map[string]func([]byte) (Term[DeBruijn], error){
			"arena": func(b []byte) (Term[DeBruijn], error) {
				p, err := DecodeDeBruijnWithContext(b, ctx)
				if err != nil {
					return nil, err
				}
				return p.Term, nil
			},
			"generic": func(b []byte) (Term[DeBruijn], error) {
				p, err := Decode[DeBruijn](b)
				if err != nil {
					return nil, err
				}
				return p.Term, nil
			},
		} {
			term, err := decode(input)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("%s (%s): decoded %s, want failure", tt.name, path, PrettyTerm[DeBruijn](term))
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s (%s): %v", tt.name, path, err)
			}
			got := strings.Join(strings.Fields(PrettyTerm[DeBruijn](term)), " ")
			got = strings.ReplaceAll(strings.ReplaceAll(got, "( ", "("), " )", ")")
			if got != tt.want {
				t.Fatalf("%s (%s): got %s, want %s", tt.name, path, got, tt.want)
			}
		}
	}
}
