package syn

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestDecodeFlatUniverseTypes(t *testing.T) {
	tests := []struct {
		name     string
		hexInput string
		check    func(*testing.T, IConstant)
	}{
		{
			name:     "empty list of BLS G1 elements",
			hexInput: "0101004bd721",
			check: func(t *testing.T, constant IConstant) {
				t.Helper()
				list, ok := constant.(*ProtoList)
				if !ok {
					t.Fatalf("constant type = %T, want *ProtoList", constant)
				}
				if _, ok := list.LTyp.(*TBls12_381G1Element); !ok {
					t.Fatalf("list element type = %T, want *TBls12_381G1Element", list.LTyp)
				}
				if len(list.List) != 0 {
					t.Fatalf("list length = %d, want 0", len(list.List))
				}
			},
		},
		{
			name:     "empty list of BLS G2 elements",
			hexInput: "0101004bd741",
			check: func(t *testing.T, constant IConstant) {
				t.Helper()
				list, ok := constant.(*ProtoList)
				if !ok {
					t.Fatalf("constant type = %T, want *ProtoList", constant)
				}
				if _, ok := list.LTyp.(*TBls12_381G2Element); !ok {
					t.Fatalf("list element type = %T, want *TBls12_381G2Element", list.LTyp)
				}
				if len(list.List) != 0 {
					t.Fatalf("list length = %d, want 0", len(list.List))
				}
			},
		},
		{
			name:     "empty list of BLS Miller-loop results",
			hexInput: "0101004bd761",
			check: func(t *testing.T, constant IConstant) {
				t.Helper()
				list, ok := constant.(*ProtoList)
				if !ok {
					t.Fatalf("constant type = %T, want *ProtoList", constant)
				}
				if _, ok := list.LTyp.(*TBls12_381MlResult); !ok {
					t.Fatalf("list element type = %T, want *TBls12_381MlResult", list.LTyp)
				}
				if len(list.List) != 0 {
					t.Fatalf("list length = %d, want 0", len(list.List))
				}
			},
		},
		{
			name:     "empty array of BLS G1 elements",
			hexInput: "0101004bf321",
			check: func(t *testing.T, constant IConstant) {
				t.Helper()
				if got := fmt.Sprintf("%T", constant); got != "*syn.ProtoArray" {
					t.Fatalf("constant type = %s, want *syn.ProtoArray", got)
				}
				if got := fmt.Sprintf("%T", constant.Typ()); got != "*syn.TArray" {
					t.Fatalf("constant Typ() = %s, want *syn.TArray", got)
				}
			},
		},
		{
			name:     "empty integer array",
			hexInput: "0101004bf201",
			check: func(t *testing.T, constant IConstant) {
				t.Helper()
				if got := fmt.Sprintf("%T", constant); got != "*syn.ProtoArray" {
					t.Fatalf("constant type = %s, want *syn.ProtoArray", got)
				}
				if got := fmt.Sprintf("%T", constant.Typ()); got != "*syn.TArray" {
					t.Fatalf("constant Typ() = %s, want *syn.TArray", got)
				}
			},
		},
		{
			name:     "empty value",
			hexInput: "0101004e81",
			check: func(t *testing.T, constant IConstant) {
				t.Helper()
				if got := fmt.Sprintf("%T", constant); got != "*syn.Value" {
					t.Fatalf("constant type = %s, want *syn.Value", got)
				}
				if _, ok := constant.Typ().(*TValue); !ok {
					t.Fatalf("constant Typ() = %T, want *TValue", constant.Typ())
				}
			},
		},
		{
			name:     "single-entry value",
			hexInput: "0101004ea10081000201",
			check: func(t *testing.T, constant IConstant) {
				t.Helper()
				if got := fmt.Sprintf("%T", constant); got != "*syn.Value" {
					t.Fatalf("constant type = %s, want *syn.Value", got)
				}
				if _, ok := constant.Typ().(*TValue); !ok {
					t.Fatalf("constant Typ() = %T, want *TValue", constant.Typ())
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := hex.DecodeString(test.hexInput)
			if err != nil {
				t.Fatalf("decode fixture: %v", err)
			}

			program, err := Decode[DeBruijn](input)
			if err != nil {
				t.Fatalf("Decode[DeBruijn](): %v", err)
			}
			constant, ok := program.Term.(*Constant)
			if !ok {
				t.Fatalf("term type = %T, want *Constant", program.Term)
			}
			test.check(t, constant.Con)

			encoded, err := Encode(program)
			if err != nil {
				t.Fatalf("Encode(): %v", err)
			}
			if !bytes.Equal(encoded, input) {
				t.Fatalf("re-encoded bytes = %x, want %x", encoded, input)
			}

			genericProgram, err := Decode[NamedDeBruijn](input)
			if err != nil {
				t.Fatalf("Decode[NamedDeBruijn](): %v", err)
			}
			genericConstant, ok := genericProgram.Term.(*Constant)
			if !ok {
				t.Fatalf("generic term type = %T, want *Constant", genericProgram.Term)
			}
			test.check(t, genericConstant.Con)
		})
	}
}

func TestDecodeUnsupportedBLSConstantsFailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		hexInput string
		wantType string
	}{
		{name: "G1", hexInput: "0101004c81", wantType: "bls12_381_G1_element"},
		{name: "G2", hexInput: "0101004d01", wantType: "bls12_381_G2_element"},
		{name: "Miller-loop result", hexInput: "0101004d81", wantType: "bls12_381_mlresult"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := hex.DecodeString(test.hexInput)
			if err != nil {
				t.Fatalf("decode fixture: %v", err)
			}

			for name, decode := range map[string]func([]byte) error{
				"DeBruijn": func(input []byte) error {
					_, err := Decode[DeBruijn](input)
					return err
				},
				"NamedDeBruijn": func(input []byte) error {
					_, err := Decode[NamedDeBruijn](input)
					return err
				},
			} {
				err := decode(input)
				if err == nil {
					t.Fatalf("Decode[%s]() error = nil, want unsupported BLS constant error", name)
				}
				if !strings.Contains(err.Error(), test.wantType) {
					t.Fatalf("Decode[%s]() error = %q, want type %q", name, err, test.wantType)
				}
			}
		})
	}
}

func TestDecodeValueRejectsZeroQuantity(t *testing.T) {
	input, err := hex.DecodeString("0101004ea10081000001")
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	_, err = Decode[DeBruijn](input)
	if err == nil {
		t.Fatal("Decode() error = nil, want zero-quantity rejection")
	}
	if !strings.Contains(err.Error(), "zero quantity") {
		t.Fatalf("Decode() error = %q, want zero-quantity rejection", err)
	}
}
