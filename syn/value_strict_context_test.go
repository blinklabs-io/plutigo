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
	"math/big"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

func rawValueEntries(policies ...[2]any) []IConstant {
	out := make([]IConstant, 0, len(policies))
	for _, p := range policies {
		var tokens []IConstant
		for _, tok := range p[1].([][2]any) {
			tokens = append(tokens, &ProtoPair{
				FstType: &TByteString{},
				SndType: &TInteger{},
				First:   &ByteString{Inner: tok[0].([]byte)},
				Second:  &Integer{Inner: tok[1].(*big.Int)},
			})
		}
		out = append(out, &ProtoPair{
			FstType: &TByteString{},
			SndType: &TList{Typ: &TPair{First: &TByteString{}, Second: &TInteger{}}},
			First:   &ByteString{Inner: p[0].([]byte)},
			Second: &ProtoList{
				LTyp: &TPair{First: &TByteString{}, Second: &TInteger{}},
				List: tokens,
			},
		})
	}
	return out
}

// encodeRawValueProgram FLAT-encodes a Value constant without the encoder's
// canonical-form check, as an untrusted peer could.
func encodeRawValueProgram(t *testing.T, entries []IConstant) []byte {
	t.Helper()
	e := newEncoder()
	e.word(1).word(1).word(0)
	if err := e.encodeTermTag(4); err != nil {
		t.Fatalf("encodeTermTag: %v", err)
	}
	if err := encodeConstantType(e, &TValue{}); err != nil {
		t.Fatalf("encodeConstantType: %v", err)
	}
	err := EncodeList(e, entries, func(e *encoder, item IConstant) error {
		return encodeConstantValue(e, item)
	})
	if err != nil {
		t.Fatalf("EncodeList: %v", err)
	}
	e.filler()
	return e.buffer
}

// TestPublicDecodePathsRejectNonCanonicalValue checks that every public
// decoder rejects non-canonical Value constants.
func TestPublicDecodePathsRejectNonCanonicalValue(t *testing.T) {
	t.Parallel()
	aa, bb := []byte{0xaa}, []byte{0xbb}
	long := make([]byte, 33)
	one := big.NewInt(1)
	pastMax := new(big.Int).Lsh(big.NewInt(1), 127)
	pastMin := new(big.Int).Sub(new(big.Int).Neg(pastMax), big.NewInt(1))
	tok := func(k []byte, q *big.Int) [][2]any { return [][2]any{{k, q}} }

	tests := []struct {
		name    string
		entries []IConstant
	}{
		{"duplicate token", rawValueEntries([2]any{aa, [][2]any{{bb, one}, {bb, one}}})},
		{"duplicate policy", rawValueEntries([2]any{aa, tok(bb, one)}, [2]any{aa, tok([]byte{0xcc}, one)})},
		{"policies out of order", rawValueEntries([2]any{bb, tok(aa, one)}, [2]any{aa, tok(aa, one)})},
		{"tokens out of order", rawValueEntries([2]any{aa, [][2]any{{bb, one}, {aa, one}}})},
		{"empty token map", rawValueEntries([2]any{aa, [][2]any(nil)})},
		{"zero quantity", rawValueEntries([2]any{aa, tok(bb, big.NewInt(0))})},
		{"policy key too long", rawValueEntries([2]any{long, tok(bb, one)})},
		{"token key too long", rawValueEntries([2]any{aa, tok(long, one)})},
		{"quantity above range", rawValueEntries([2]any{aa, tok(bb, pastMax)})},
		{"quantity below range", rawValueEntries([2]any{aa, tok(bb, pastMin)})},
	}
	ctx := ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: 11}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			encoded := encodeRawValueProgram(t, tc.entries)
			decode := map[string]func() error{
				"context DeBruijn": func() error {
					_, err := DecodeWithContext[DeBruijn](encoded, ctx)
					return err
				},
				"context Name": func() error {
					_, err := DecodeWithContext[Name](encoded, ctx)
					return err
				},
				"DeBruijn": func() error {
					_, err := Decode[DeBruijn](encoded)
					return err
				},
				"Name": func() error {
					_, err := Decode[Name](encoded)
					return err
				},
				"DecodeDeBruijn": func() error {
					_, err := DecodeDeBruijn(encoded)
					return err
				},
				"borrowed": func() error {
					_, err := NewDeBruijnDecoder().DecodeBorrowed(encoded)
					return err
				},
			}
			for name, run := range decode {
				if err := run(); err == nil {
					t.Errorf("%s accepted a non-canonical Value", name)
				}
			}
		})
	}
}

func TestParseWithContextRejectsNonCanonicalValue(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("01", 33)
	pastMax := new(big.Int).Lsh(big.NewInt(1), 127).String()
	pastMin := "-" + new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1)).String()
	tests := map[string]string{
		"duplicate token":       `[(#aa, [(#01, 1), (#01, 2)])]`,
		"duplicate policy":      `[(#aa, [(#01, 1)]), (#aa, [(#02, 1)])]`,
		"policies out of order": `[(#bb, [(#01, 1)]), (#aa, [(#01, 1)])]`,
		"tokens out of order":   `[(#aa, [(#02, 1), (#01, 1)])]`,
		"empty token map":       `[(#aa, [])]`,
		"zero quantity":         `[(#aa, [(#bb, 0)])]`,
		"policy key too long":   `[(#` + long + `, [(#bb, 1)])]`,
		"token key too long":    `[(#aa, [(#` + long + `, 1)])]`,
		"quantity above range":  `[(#aa, [(#bb, ` + pastMax + `)])]`,
		"quantity below range":  `[(#aa, [(#bb, ` + pastMin + `)])]`,
	}
	ctx := ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: 11}
	for name, text := range tests {
		src := valuePrograms["con"](text)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseWithContext(src, ctx); err == nil {
				t.Error("ParseWithContext accepted a non-canonical Value")
			}
			if _, err := Parse(src); err == nil {
				t.Error("Parse accepted a non-canonical Value")
			}
		})
	}
}
