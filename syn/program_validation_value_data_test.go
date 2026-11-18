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
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

// valueData is the unsupported Value form in UPLC text syntax.
const valueData = `V [(#aa, [(#bb, 1)])]`

// valueDataPrograms place a Value form at every data-constant position where
// the parser previously accepted it.
var valueDataPrograms = map[string]string{
	"top level":         `(con data (` + valueData + `))`,
	"in data list":      `(con data (List [I 1, ` + valueData + `]))`,
	"in data map key":   `(con data (Map [(` + valueData + `, I 1)]))`,
	"in data map value": `(con data (Map [(I 1, ` + valueData + `)]))`,
	"in data constr":    `(con data (Constr 0 [B #00, List [` + valueData + `]]))`,
	"in list constant":  `(con (list data) [I 1, ` + valueData + `])`,
	"in pair constant":  `(con (pair integer data) (1, ` + valueData + `))`,
	"in array constant": `(con (array data) [` + valueData + `])`,
	"in pair first":     `(con (pair data integer) (` + valueData + `, 1))`,
	"nested list":       `(con (list (list data)) [[` + valueData + `]])`,
	"in array of lists": `(con (array (list data)) [[I 1, ` + valueData + `]])`,
	"in dead lambda":    `(lam x (con data (` + valueData + `)))`,
}

func valueDataContext(language lang.LanguageVersion) ProgramContext {
	return ProgramContext{
		LedgerLanguage: language,
		ProtocolMajor:  builtin.VanRossemProtoVersion + 1,
	}
}

func TestParseRejectsValueData(t *testing.T) {
	for name, term := range valueDataPrograms {
		src := `(program 1.1.0 ` + term + `)`
		t.Run(name, func(t *testing.T) {
			_, err := Parse(src)
			if err == nil || !strings.Contains(
				err.Error(),
				"expected PlutusData constructor (I, B, List, Map, Constr)",
			) {
				t.Fatalf("Parse error = %v, want unsupported Value form", err)
			}
			for _, language := range []lang.LanguageVersion{
				lang.LanguageVersionV1,
				lang.LanguageVersionV2,
				lang.LanguageVersionV3,
				lang.LanguageVersionV4,
			} {
				for _, major := range []uint{0, builtin.VanRossemProtoVersion + 1} {
					_, err := ParseWithContext(src, ProgramContext{
						LedgerLanguage: language,
						ProtocolMajor:  major,
					})
					if err == nil || !strings.Contains(
						err.Error(),
						"expected PlutusData constructor (I, B, List, Map, Constr)",
					) {
						t.Fatalf(
							"ParseWithContext(%v, %d) error = %v, want unsupported Value form",
							language,
							major,
							err,
						)
					}
				}
			}
		})
	}
}

func TestValidateProgramRejectsValueData(t *testing.T) {
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersion{1, 1, 0},
		Term:    &Constant{Con: &Data{Inner: newValueData()}},
	}
	for _, language := range []lang.LanguageVersion{
		lang.LanguageVersionV1,
		lang.LanguageVersionV2,
		lang.LanguageVersionV3,
		lang.LanguageVersionV4,
	} {
		err := ValidateProgram(program, valueDataContext(language))
		if !errors.Is(err, errValueData) {
			t.Errorf(
				"ledger language %v: got %v, want %v",
				language,
				err,
				errValueData,
			)
		}
	}
}

// TestValidateProgramAcceptsDataWithoutValue checks that ordinary nested data,
// which shares every container with the rejected cases, is still accepted.
func TestValidateProgramAcceptsDataWithoutValue(t *testing.T) {
	src := `(program 1.1.0 [ (lam x (con (list data) [I 1, Map [(B #00, List [Constr 0 [I 2]])]])) (con (pair integer data) (1, B #00)) ])`
	for _, language := range []lang.LanguageVersion{
		lang.LanguageVersionV1,
		lang.LanguageVersionV2,
		lang.LanguageVersionV3,
		lang.LanguageVersionV4,
	} {
		if _, err := ParseWithContext(src, valueDataContext(language)); err != nil {
			t.Fatalf(
				"ledger language %v rejected ordinary data: %v",
				language,
				err,
			)
		}
	}
}

// TestValidateProgramDataTraversalUnbounded checks that the data traversal has
// no node limit for a tree, as the reference has none, but rejects a
// caller-constructed constant whose containers are shared, which a tree walk
// could take exponential time over.
func TestValidateProgramDataTraversalUnbounded(t *testing.T) {
	item := data.NewInteger(big.NewInt(0))
	items := make([]data.PlutusData, maxProgramNodes+1)
	for i := range items {
		items[i] = item
	}
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersion{1, 1, 0},
		Term:    &Constant{Con: &Data{Inner: data.NewList(items...)}},
	}
	if err := ValidateProgram(program, valueDataContext(lang.LanguageVersionV3)); err != nil {
		t.Fatalf("wide data constant rejected: %v", err)
	}

	shared := item
	for range 40 {
		shared = data.NewList(shared, shared)
	}
	program.Term = &Constant{Con: &Data{Inner: shared}}
	err := ValidateProgram(program, valueDataContext(lang.LanguageVersionV3))
	if !errors.Is(err, data.ErrSharedNodes) {
		t.Fatalf("shared data constant: got %v, want ErrSharedNodes", err)
	}
}

// valueDataV3 validates a single caller-constructed constant as a V3 program.
func valueDataV3(con IConstant) error {
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersion{1, 1, 0},
		Term:    &Constant{Con: con},
	}
	return ValidateProgram(program, valueDataContext(lang.LanguageVersionV3))
}

func newValueData() *data.Value {
	return &data.Value{Inner: data.NewMap(nil).(*data.Map)}
}

// TestValidateProgramRejectsValueDataInCallerConstants checks caller-built
// constants the parser and decoder never produce: children that do not match
// their container's declared type, and constants and data in value form
// rather than pointer form. The Value must be found in each.
func TestValidateProgramRejectsValueDataInCallerConstants(t *testing.T) {
	integer := &Integer{Inner: big.NewInt(1)}
	// sharedPair claims a zero-width type but hides a Value; the walk visits
	// such a pair once per pointer, and that one visit must find it.
	sharedPair := &ProtoPair{
		FstType: &TUnit{},
		SndType: &TUnit{},
		First:   &Unit{},
		Second:  &Data{Inner: newValueData()},
	}
	unitPairType := &TPair{First: &TUnit{}, Second: &TUnit{}}
	cases := map[string]IConstant{
		"shared pair under a declared unit pair list": WrapProtoList(unitPairType,
			[]IConstant{sharedPair, sharedPair, sharedPair}),

		"shared pair nested twice in a pair": WrapProtoList(&TPair{First: unitPairType, Second: unitPairType},
			[]IConstant{&ProtoPair{
				FstType: unitPairType,
				SndType: unitPairType,
				First:   sharedPair,
				Second:  sharedPair,
			}}),

		"pair child under a declared integer pair": WrapProtoList(&TPair{First: &TInteger{}, Second: &TInteger{}},
			[]IConstant{&ProtoPair{
				FstType: &TInteger{},
				SndType: &TInteger{},
				First:   integer,
				Second:  &Data{Inner: newValueData()},
			}}),

		"data child under a declared integer list": WrapProtoList(&TInteger{},
			[]IConstant{&Data{Inner: newValueData()}}),

		"data child under a declared integer array": &ProtoArray{
			ATyp:  &TInteger{},
			Array: []IConstant{&Data{Inner: newValueData()}},
		},
		"value-form Data":       Data{Inner: newValueData()},
		"value-form data.Value": &Data{Inner: *newValueData()},
		"value-form containers": *WrapProtoList(&TData{},
			[]IConstant{ProtoPair{
				FstType: &TData{},
				SndType: &TData{},
				First: Data{
					Inner: *data.WrapList([]data.PlutusData{
						*data.WrapConstr(nil, []data.PlutusData{
							*data.WrapMap([][2]data.PlutusData{
								{data.Integer{}, *newValueData()},
							}),
						}),
					}),
				},
				Second: Data{Inner: data.Integer{}},
			}}),

		"value-form array": ProtoArray{
			ATyp:  &TData{},
			Array: []IConstant{Data{Inner: newValueData()}},
		},
	}
	for name, con := range cases {
		t.Run(name, func(t *testing.T) {
			err := valueDataV3(con)
			if !errors.Is(err, errValueData) {
				t.Fatalf("got %v, want %v", err, errValueData)
			}
		})
	}
}

// TestValidateProgramRejectsDeeplyNestedValueData nests a Value deeper than
// the walk's inline frame stack, so the stack must grow and the Value must
// still be found.
func TestValidateProgramRejectsDeeplyNestedValueData(t *testing.T) {
	var nested data.PlutusData = newValueData()
	for range 40 {
		nested = data.NewList(data.NewInteger(big.NewInt(0)), nested)
	}
	err := valueDataV3(&Data{Inner: nested})
	if !errors.Is(err, errValueData) {
		t.Fatalf("got %v, want %v", err, errValueData)
	}
}

// TestValidateProgramValueDataMatchesDecoderAccounting checks that the walk
// charges what the decoders charge, so programs they accept stay valid: one
// node per constant list item with pair fields free, and data counted per data
// constant (each FLAT data blob has its own decode limit) rather than across
// the program.
func TestValidateProgramValueDataMatchesDecoderAccounting(t *testing.T) {
	pairs := make([]IConstant, 400_000)
	pair := &ProtoPair{
		FstType: &TInteger{},
		SndType: &TInteger{},
		First:   &Integer{Inner: big.NewInt(1)},
		Second:  &Integer{Inner: big.NewInt(2)},
	}
	for i := range pairs {
		pairs[i] = pair
	}
	pairList := WrapProtoList(&TPair{First: &TInteger{}, Second: &TInteger{}},
		pairs)

	if err := valueDataV3(pairList); err != nil {
		t.Fatalf("list of 400,000 pairs rejected: %v", err)
	}

	item := data.NewInteger(big.NewInt(0))
	items := make([]data.PlutusData, 600_000)
	for i := range items {
		items[i] = item
	}
	blob := &Data{Inner: data.NewList(items...)}
	twoBlobs := &ProtoPair{
		FstType: &TData{},
		SndType: &TData{},
		First:   blob,
		Second:  blob,
	}
	if err := valueDataV3(twoBlobs); err != nil {
		t.Fatalf("two data constants of 600,001 nodes each rejected: %v", err)
	}
}

// TestValidateProgramBoundsConstantItems checks that constant list items are
// charged program-wide against the node limit.
func TestValidateProgramBoundsConstantItems(t *testing.T) {
	integer := &Integer{Inner: big.NewInt(1)}
	list := make([]IConstant, maxProgramNodes+1)
	for i := range list {
		list[i] = integer
	}
	err := valueDataV3(WrapProtoList(&TInteger{}, list))
	if err == nil || !strings.Contains(err.Error(), "too many nodes") {
		t.Fatalf("expected node limit error, got %v", err)
	}
}
