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

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

// valuePrograms wraps a Value literal in each text form that parses one: a
// data Value and a builtin value constant.
var valuePrograms = map[string]func(valueText string) string{
	"data": func(v string) string { return `(program 1.3.0 (con data (V ` + v + `)))` },
	"con":  func(v string) string { return `(program 1.3.0 (con value ` + v + `))` },
}

func TestParsePlutusValueCanonicalForm(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("01", 33)
	qtyLimit := new(big.Int).Lsh(big.NewInt(1), 127)
	maxQty := new(big.Int).Sub(qtyLimit, big.NewInt(1)).String()
	minQty := new(big.Int).Neg(qtyLimit).String()
	pastMax := qtyLimit.String()
	pastMin := new(big.Int).Sub(new(big.Int).Neg(qtyLimit), big.NewInt(1)).String()

	valid := map[string]string{
		"max quantity": `[(#aa, [(#bb, ` + maxQty + `)])]`,
		"min quantity": `[(#aa, [(#bb, ` + minQty + `)])]`,
		"ascending":    `[(#aa, [(#01, 1), (#02, -1)]), (#bb, [(#01, 1)])]`,
	}
	for form, program := range valuePrograms {
		for name, text := range valid {
			t.Run(form+"/accepts "+name, func(t *testing.T) {
				t.Parallel()
				if _, err := Parse(program(text)); err != nil {
					t.Fatalf("Parse: %v", err)
				}
			})
		}
	}

	invalid := map[string]string{
		"policy key too long":   `[(#` + long + `, [(#bb, 1)])]`,
		"token key too long":    `[(#aa, [(#` + long + `, 1)])]`,
		"policies out of order": `[(#bb, [(#01, 1)]), (#aa, [(#01, 1)])]`,
		"duplicate policy":      `[(#aa, [(#01, 1)]), (#aa, [(#02, 1)])]`,
		"tokens out of order":   `[(#aa, [(#02, 1), (#01, 1)])]`,
		"duplicate token":       `[(#aa, [(#01, 1), (#01, 2)])]`,
		"empty token map":       `[(#aa, [])]`,
		"zero quantity":         `[(#aa, [(#bb, 0)])]`,
		"quantity above range":  `[(#aa, [(#bb, ` + pastMax + `)])]`,
		"quantity below range":  `[(#aa, [(#bb, ` + pastMin + `)])]`,
	}
	for form, program := range valuePrograms {
		for name, text := range invalid {
			t.Run(form+"/rejects "+name, func(t *testing.T) {
				t.Parallel()
				if _, err := Parse(program(text)); err == nil {
					t.Fatal("Parse accepted a non-canonical Value")
				}
			})
		}
	}
}

func TestEncodeRejectsNonCanonicalDataValue(t *testing.T) {
	t.Parallel()
	tokens := &data.Map{Pairs: [][2]data.PlutusData{{
		data.NewByteString([]byte{1}),
		data.NewInteger(new(big.Int)),
	}}}
	inner := &data.Map{Pairs: [][2]data.PlutusData{{
		data.NewByteString([]byte{1}),
		tokens,
	}}}
	_, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV4,
		Term: &Constant{Con: &Data{
			Inner: &data.Value{Inner: inner},
		}},
	})
	if err == nil {
		t.Fatal("Encode accepted a data Value with a zero quantity")
	}
}

func TestParseValueRejectsInvalidPrefix(t *testing.T) {
	long := strings.Repeat("01", 33)
	limit := new(big.Int).Lsh(big.NewInt(1), 127).String()
	invalid := map[string]struct{ text, errorText string }{
		"policy key":       {`[(#` + long + `, [(#bb, 1)]), malformed]`, "key exceeds"},
		"token key":        {`[(#aa, [(#` + long + `, 1), malformed])]`, "key exceeds"},
		"quantity":         {`[(#aa, [(#bb, ` + limit + `), malformed])]`, "quantity out of signed 128-bit range"},
		"zero quantity":    {`[(#aa, [(#bb, 0), malformed])]`, "zero quantity"},
		"duplicate policy": {`[(#aa, [(#bb, 1)]), (#aa, malformed)]`, "policy keys must be strictly ascending"},
		"duplicate token":  {`[(#aa, [(#bb, 1), (#bb, malformed)])]`, "token keys must be strictly ascending"},
		"empty tokens":     {`[(#aa, []), malformed]`, "empty token map"},
	}
	for form, program := range valuePrograms {
		for name, tc := range invalid {
			t.Run(form+"/"+name, func(t *testing.T) {
				_, err := Parse(program(tc.text))
				if err == nil || !strings.Contains(err.Error(), tc.errorText) {
					t.Fatalf("Parse error = %v, want prefix validation error %q before malformed suffix", err, tc.errorText)
				}
			})
		}
	}
}

func TestValueValidationAllocationBound(t *testing.T) {
	entries := make([]IConstant, 1024)
	for i := range entries {
		entries[i] = &ProtoPair{
			First: &ByteString{Inner: []byte{byte(i >> 8), byte(i)}},
			Second: &ProtoList{List: []IConstant{&ProtoPair{
				First: &ByteString{Inner: []byte{1}}, Second: newInteger(big.NewInt(1)),
			}}},
		}
	}
	allocs := testing.AllocsPerRun(10, func() {
		if err := validateValueEntries(entries); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 1 {
		t.Fatalf("validating 1024 policies allocated %.0f objects, want at most 1", allocs)
	}
}
