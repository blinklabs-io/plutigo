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

// flatDataConstant returns a 1.1.0 FLAT program whose body is a data constant
// holding the CBOR item cborHex.
func flatDataConstant(t *testing.T, cborHex string) []byte {
	t.Helper()
	item, err := hex.DecodeString(cborHex)
	if err != nil || len(item) == 0 || len(item) > 255 {
		t.Fatalf("bad data constant %q: %v", cborHex, err)
	}
	program := append([]byte{0x01, 0x01, 0x00, 0x4c, 0x01, byte(len(item))}, item...)
	return append(program, 0x00, 0x01)
}

// flatDecoders are the exported FLAT program decoders, by name. Each takes
// the program for ledger language language at protocol version 12.
var flatDecoders = map[string]func([]byte, lang.LanguageVersion) error{
	"DecodeDeBruijn": func(b []byte, _ lang.LanguageVersion) error {
		_, err := DecodeDeBruijn(b)
		return err
	},
	"DeBruijnDecoder.Decode": func(b []byte, _ lang.LanguageVersion) error {
		_, err := NewDeBruijnDecoder().Decode(b)
		return err
	},
	"DeBruijnDecoder.DecodeBorrowed": func(b []byte, _ lang.LanguageVersion) error {
		_, err := NewDeBruijnDecoder().DecodeBorrowed(b)
		return err
	},
	"DeBruijnDecoder.DecodeBorrowedWithScopeCheck": func(b []byte, _ lang.LanguageVersion) error {
		_, err := NewDeBruijnDecoder().DecodeBorrowedWithScopeCheck(b, true)
		return err
	},
	"Decode[DeBruijn]": func(b []byte, _ lang.LanguageVersion) error {
		_, err := Decode[DeBruijn](b)
		return err
	},
	"Decode[NamedDeBruijn]": func(b []byte, _ lang.LanguageVersion) error {
		_, err := Decode[NamedDeBruijn](b)
		return err
	},
	"Decode[Name]": func(b []byte, _ lang.LanguageVersion) error {
		_, err := Decode[Name](b)
		return err
	},
	"DecodeDeBruijnWithContext": func(b []byte, l lang.LanguageVersion) error {
		_, err := DecodeDeBruijnWithContext(b, flatContext(l))
		return err
	},
	"DecodeWithContext[NamedDeBruijn]": func(b []byte, l lang.LanguageVersion) error {
		_, err := DecodeWithContext[NamedDeBruijn](b, flatContext(l))
		return err
	},
	"DecodeDeBruijnWithContextInfo": func(b []byte, l lang.LanguageVersion) error {
		_, _, err := DecodeDeBruijnWithContextInfo(b, flatContext(l), true)
		return err
	},
	"DecodeDeBruijnForExecution": func(b []byte, l lang.LanguageVersion) error {
		_, err := DecodeDeBruijnForExecution(b, flatContext(l))
		return err
	},
}

func flatContext(l lang.LanguageVersion) ProgramContext {
	return ProgramContext{LedgerLanguage: l, ProtocolMajor: 12}
}

var flatLanguages = []lang.LanguageVersion{
	lang.LanguageVersionV1,
	lang.LanguageVersionV2,
	lang.LanguageVersionV3,
	lang.LanguageVersionV4,
}

// TestFlatDecodeRejectsDataTag1401 checks that no FLAT decoder admits the tag
// 1401 Value constructor in a data constant, for any ledger language. The
// reference Data type has no such constructor, and its decoder fails on the
// tag, so the program does not deserialise.
func TestFlatDecodeRejectsDataTag1401(t *testing.T) {
	t.Parallel()
	const want = "unknown CBOR tag for PlutusData: 1401"
	// Tag 1401 around {h'01': {h'': 1}}, bare and inside a list.
	for name, item := range map[string]string{
		"top level": "d90579a14101a14001",
		"in list":   "81d90579a14101a14001",
	} {
		program := flatDataConstant(t, item)
		for decoder, decode := range flatDecoders {
			for _, language := range flatLanguages {
				err := decode(program, language)
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf(
						"%s/%s/%v: got %v, want error containing %q",
						name,
						decoder,
						language,
						err,
						want,
					)
				}
			}
		}
	}
}

// TestFlatDecodeRejectsEncodedValueData checks that a program encoded from
// Value data constants, in each position the text parser accepts them, does
// not decode in any language through the context-free decoders or the V4
// context.
func TestFlatDecodeRejectsEncodedValueData(t *testing.T) {
	t.Parallel()
	for name, term := range valueDataPrograms {
		encoded := encodeValueDataProgram(t, `(program 1.1.0 `+term+`)`)
		for decoder, decode := range flatDecoders {
			for _, language := range flatLanguages {
				if err := decode(encoded, language); err == nil {
					t.Errorf("%s/%s/%v: accepted Value data", name, decoder, language)
				}
			}
		}
	}
}

// TestFlatDecodeDataTagBoundaries checks the constructor tags the reference
// decoder admits (102, 121-127, 1280-1400) and rejects (1279 and 1401
// onwards, apart from them) in a data constant, through every decoder.
func TestFlatDecodeDataTagBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		item   string
		accept bool
	}{
		{"tag 102", "d8668200" + "80", true},
		{"tag 121", "d87980", true},
		{"tag 127", "d87f80", true},
		{"tag 128", "d8808" + "0", false},
		{"tag 1279", "d904ff80", false},
		{"tag 1280", "d9050080", true},
		{"tag 1400", "d9057880", true},
		{"tag 1401", "d90579a0", false},
		{"tag 1402", "d9057a80", false},
	}
	for _, tt := range tests {
		program := flatDataConstant(t, tt.item)
		for decoder, decode := range flatDecoders {
			for _, language := range flatLanguages {
				err := decode(program, language)
				if (err == nil) != tt.accept {
					t.Errorf(
						"%s/%s/%v: error = %v, want accept %v",
						tt.name,
						decoder,
						language,
						err,
						tt.accept,
					)
				}
			}
		}
	}
}
