package cek

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestDecodedFlatUniverseConstantsEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		flatHex  string
		validate func(*testing.T, syn.IConstant)
	}{
		{
			name:    "lengthOfArray",
			flatHex: "010300357b297e4001",
			validate: func(t *testing.T, constant syn.IConstant) {
				t.Helper()
				integer, ok := constant.(*syn.Integer)
				if !ok {
					t.Fatalf("result type = %T, want *syn.Integer", constant)
				}
				if integer.Inner.Cmp(big.NewInt(0)) != 0 {
					t.Fatalf("result = %s, want 0", integer.Inner)
				}
			},
		},
		{
			name:    "valueData empty",
			flatHex: "01030037c49d01",
			validate: func(t *testing.T, constant syn.IConstant) {
				t.Helper()
				wrapped, ok := constant.(*syn.Data)
				if !ok {
					t.Fatalf("result type = %T, want *syn.Data", constant)
				}
				valueMap, ok := wrapped.Inner.(*data.Map)
				if !ok {
					t.Fatalf("result data type = %T, want *data.Map", wrapped.Inner)
				}
				if len(valueMap.Pairs) != 0 {
					t.Fatalf("result map length = %d, want 0", len(valueMap.Pairs))
				}
			},
		},
		{
			name:    "valueData populated",
			flatHex: "01030037c49d410081000201",
			validate: func(t *testing.T, constant syn.IConstant) {
				t.Helper()
				wrapped, ok := constant.(*syn.Data)
				if !ok {
					t.Fatalf("result type = %T, want *syn.Data", constant)
				}
				valueMap, ok := wrapped.Inner.(*data.Map)
				if !ok {
					t.Fatalf("result data type = %T, want *data.Map", wrapped.Inner)
				}
				if len(valueMap.Pairs) != 1 {
					t.Fatalf("result map length = %d, want 1", len(valueMap.Pairs))
				}
			},
		},
	}

	ctx := NewDefaultEvalContext(
		lang.LanguageVersionV4,
		ProtoVersion{Major: 12},
	)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flat, err := hex.DecodeString(test.flatHex)
			if err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			program, err := syn.Decode[syn.DeBruijn](flat)
			if err != nil {
				t.Fatalf("decode FLAT program: %v", err)
			}

			machine := NewMachine[syn.DeBruijn](
				lang.LanguageVersionV4,
				0,
				ctx,
			)
			result, err := machine.Run(program.Term)
			if err != nil {
				t.Fatalf("evaluate decoded program: %v", err)
			}
			constant, ok := result.(*syn.Constant)
			if !ok {
				t.Fatalf("result term type = %T, want *syn.Constant", result)
			}
			test.validate(t, constant.Con)
		})
	}
}

func TestDecodedFlatUniverseConstantsPreserveTypeOnDischarge(t *testing.T) {
	for name, flatHex := range map[string]string{
		"empty BLS list":  "0101004bd721",
		"empty BLS array": "0101004bf321",
		"integer array":   "0101004bf201",
		"empty value":     "0101004e81",
		"populated value": "0101004ea10081000201",
	} {
		t.Run(name, func(t *testing.T) {
			flat, err := hex.DecodeString(flatHex)
			if err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			program, err := syn.Decode[syn.DeBruijn](flat)
			if err != nil {
				t.Fatalf("decode FLAT program: %v", err)
			}
			machine := NewMachine[syn.DeBruijn](
				lang.LanguageVersionV4,
				0,
				NewDefaultEvalContext(
					lang.LanguageVersionV4,
					ProtoVersion{Major: 12},
				),
			)
			result, err := machine.Run(program.Term)
			if err != nil {
				t.Fatalf("evaluate decoded constant: %v", err)
			}
			reencoded, err := syn.Encode(&syn.Program[syn.DeBruijn]{
				Version: program.Version,
				Term:    result,
			})
			if err != nil {
				t.Fatalf("encode discharged constant: %v", err)
			}
			if !bytes.Equal(reencoded, flat) {
				t.Fatalf("re-encoded bytes = %x, want %x", reencoded, flat)
			}
		})
	}
}

func TestArrayAndValueBuiltinsPreserveConstantTypes(t *testing.T) {
	t.Run("listToArray", func(t *testing.T) {
		machine := newTestMachineV4()
		fn := newTestBuiltin(builtin.ListToArray)
		fn = fn.ApplyArg(&Constant{Constant: &syn.ProtoList{
			LTyp: &syn.TInteger{},
		}})
		result := expectConstant(t, evalBuiltin(t, machine, fn))
		if _, ok := result.Constant.(*syn.ProtoArray); !ok {
			t.Fatalf("result type = %T, want *syn.ProtoArray", result.Constant)
		}
		assertConstantEncoding(t, result.Constant, "0103004bf201")
	})

	t.Run("unValueData", func(t *testing.T) {
		machine := newTestMachineV4()
		fn := newTestBuiltin(builtin.UnValueData)
		fn = fn.ApplyArg(&Constant{Constant: &syn.Data{Inner: &data.Map{}}})
		result := expectConstant(t, evalBuiltin(t, machine, fn))
		if _, ok := result.Constant.(*syn.Value); !ok {
			t.Fatalf("result type = %T, want *syn.Value", result.Constant)
		}
		assertConstantEncoding(t, result.Constant, "0103004e81")
	})
}

func TestFlatUniverseConstantExMem(t *testing.T) {
	t.Run("array uses element count", func(t *testing.T) {
		constant := &syn.ProtoArray{
			ATyp: &syn.TByteString{},
			Array: []syn.IConstant{
				&syn.ByteString{Inner: make([]byte, 64)},
				&syn.ByteString{Inner: make([]byte, 64)},
			},
		}
		if got, want := iconstantExMem(constant)(), ExMem(2); got != want {
			t.Fatalf("array ExMem = %d, want element count %d", got, want)
		}
	})

	t.Run("value uses total token count", func(t *testing.T) {
		constant := &syn.Value{Entries: []syn.IConstant{
			&syn.ProtoPair{
				FstType: &syn.TByteString{},
				SndType: &syn.TList{Typ: &syn.TPair{
					First:  &syn.TByteString{},
					Second: &syn.TInteger{},
				}},
				First: &syn.ByteString{Inner: []byte{1}},
				Second: &syn.ProtoList{
					LTyp: &syn.TPair{
						First:  &syn.TByteString{},
						Second: &syn.TInteger{},
					},
					List: []syn.IConstant{
						&syn.ProtoPair{
							FstType: &syn.TByteString{},
							SndType: &syn.TInteger{},
							First:   &syn.ByteString{Inner: []byte{1}},
							Second:  &syn.Integer{Inner: big.NewInt(1)},
						},
						&syn.ProtoPair{
							FstType: &syn.TByteString{},
							SndType: &syn.TInteger{},
							First:   &syn.ByteString{Inner: []byte{2}},
							Second:  &syn.Integer{Inner: big.NewInt(1)},
						},
					},
				},
			},
		}}
		if got, want := iconstantExMem(constant)(), ExMem(2); got != want {
			t.Fatalf("value ExMem = %d, want total token count %d", got, want)
		}
	})
}

func assertConstantEncoding(t *testing.T, constant syn.IConstant, wantHex string) {
	t.Helper()
	encoded, err := syn.Encode(&syn.Program[syn.DeBruijn]{
		Version: lang.LanguageVersionV4,
		Term:    &syn.Constant{Con: constant},
	})
	if err != nil {
		t.Fatalf("encode constant: %v", err)
	}
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		t.Fatalf("decode expected bytes: %v", err)
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("encoded bytes = %x, want %x", encoded, want)
	}
}
