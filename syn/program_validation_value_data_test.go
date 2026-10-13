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

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

// valueData is a V4-only PlutusData Value in UPLC text syntax.
const valueData = `V [(#aa, [(#bb, 1)])]`

// valueDataPrograms place a PlutusData Value at the top of a data constant,
// nested inside List, Map and Constr data, inside list, pair and array
// constants, and in a lambda body that evaluation never reaches.
var valueDataPrograms = map[string]string{
	"top level":         `(con data (` + valueData + `))`,
	"in data list":      `(con data (List [I 1, ` + valueData + `]))`,
	"in data map key":   `(con data (Map [(` + valueData + `, I 1)]))`,
	"in data map value": `(con data (Map [(I 1, ` + valueData + `)]))`,
	"in data constr":    `(con data (Constr 0 [B #00, List [` + valueData + `]]))`,
	"in list constant":  `(con (list data) [I 1, ` + valueData + `])`,
	"in pair constant":  `(con (pair integer data) (1, ` + valueData + `))`,
	"in array constant": `(con (array data) [` + valueData + `])`,
	"in dead lambda":    `(lam x (con data (` + valueData + `)))`,
}

func valueDataContext(language lang.LanguageVersion) ProgramContext {
	return ProgramContext{
		LedgerLanguage: language,
		ProtocolMajor:  builtin.VanRossemProtoVersion + 1,
	}
}

// encodeValueDataProgram parses a program without validation and returns its
// FLAT encoding, so the decoder path can be tested with the same input.
func encodeValueDataProgram(t *testing.T, src string) []byte {
	t.Helper()
	program, err := Parse(src)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}
	dbProgram, err := NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("convert %s: %v", src, err)
	}
	encoded, err := Encode(dbProgram)
	if err != nil {
		t.Fatalf("encode %s: %v", src, err)
	}
	return encoded
}

// TestValidateProgramRejectsValueDataBeforeV4 checks that V1, V2 and V3
// programs carrying a PlutusData Value anywhere in a constant, including
// code evaluation never reaches, are rejected by both the text parser and the
// FLAT decoder, while V4 programs with the same constants are accepted.
func TestValidateProgramRejectsValueDataBeforeV4(t *testing.T) {
	for name, term := range valueDataPrograms {
		src := `(program 1.1.0 ` + term + `)`
		encoded := encodeValueDataProgram(t, src)
		t.Run(name, func(t *testing.T) {
			for _, language := range []lang.LanguageVersion{
				lang.LanguageVersionV1,
				lang.LanguageVersionV2,
				lang.LanguageVersionV3,
			} {
				context := valueDataContext(language)
				_, err := ParseWithContext(src, context)
				requireValueDataRejected(t, "ParseWithContext", language, err)
				_, err = DecodeWithContext[DeBruijn](encoded, context)
				requireValueDataRejected(t, "DecodeWithContext", language, err)
			}
			context := valueDataContext(lang.LanguageVersionV4)
			if _, err := ParseWithContext(src, context); err != nil {
				t.Fatalf("ParseWithContext rejected V4 program: %v", err)
			}
			if _, err := DecodeWithContext[DeBruijn](encoded, context); err != nil {
				t.Fatalf("DecodeWithContext rejected V4 program: %v", err)
			}
		})
	}
}

func requireValueDataRejected(
	t *testing.T,
	path string,
	language lang.LanguageVersion,
	err error,
) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s accepted Value data for ledger language %v", path, language)
	}
	if !strings.Contains(err.Error(), "Value data is not available") {
		t.Fatalf("%s for ledger language %v: unexpected error %v", path, language, err)
	}
}

// TestValidateProgramAcceptsDataWithoutValueBeforeV4 checks that ordinary
// nested data, which shares every container with the rejected cases, is still
// accepted for V1, V2 and V3.
func TestValidateProgramAcceptsDataWithoutValueBeforeV4(t *testing.T) {
	src := `(program 1.1.0 [ (lam x (con (list data) [I 1, Map [(B #00, List [Constr 0 [I 2]])]])) (con (pair integer data) (1, B #00)) ])`
	for _, language := range []lang.LanguageVersion{
		lang.LanguageVersionV1,
		lang.LanguageVersionV2,
		lang.LanguageVersionV3,
	} {
		if _, err := ParseWithContext(src, valueDataContext(language)); err != nil {
			t.Fatalf("ledger language %v rejected ordinary data: %v", language, err)
		}
	}
}

// TestValidateProgramBoundsValueDataTraversal checks that the data traversal
// for pre-V4 programs stops at the shared program node limit instead of
// walking an arbitrarily large caller-constructed constant.
func TestValidateProgramBoundsValueDataTraversal(t *testing.T) {
	item := data.NewInteger(big.NewInt(0))
	items := make([]data.PlutusData, maxProgramNodes+1)
	for i := range items {
		items[i] = item
	}
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersion{1, 1, 0},
		Term:    &Constant{Con: &Data{Inner: data.NewList(items...)}},
	}
	err := ValidateProgram(program, valueDataContext(lang.LanguageVersionV3))
	if err == nil || !strings.Contains(err.Error(), "too many nodes") {
		t.Fatalf("expected node limit error, got %v", err)
	}
}
