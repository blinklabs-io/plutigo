package syn

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

func TestEncodeDecodeConstant(t *testing.T) {
	// Simple test first
	constant := &Integer{Inner: big.NewInt(42)}
	term := &Constant{Con: constant}

	// Encode the term
	encoded, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV1,
		Term:    term,
	})
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("Encoded bytes: %x", encoded)

	// Decode the program
	program, err := Decode[DeBruijn](encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Check if it's a constant term
	constantTerm, ok := program.Term.(*Constant)
	if !ok {
		t.Fatalf("Decoded term is not a Constant: %T", program.Term)
	}

	// Compare constants
	if constant.Inner.Cmp(constantTerm.Con.(*Integer).Inner) != 0 {
		t.Errorf(
			"Constants not equal: got %v, want %v",
			constantTerm.Con.(*Integer).Inner,
			constant.Inner,
		)
	}
}

func TestEncodeDecodeBuiltin(t *testing.T) {
	tests := []struct {
		name string
		fn   builtin.DefaultFunction
	}{
		{"AddInteger", builtin.AddInteger},
		{"Sha2_256", builtin.Sha2_256},
		{"Sha3_256", builtin.Sha3_256},
		{"Blake2b_256", builtin.Blake2b_256},
		{"Keccak_256", builtin.Keccak_256},
		{"IfThenElse", builtin.IfThenElse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Builtin{DefaultFunction: tt.fn}

			// Encode the builtin term
			encoded, err := Encode(&Program[DeBruijn]{
				Version: lang.LanguageVersionV1,
				Term:    original,
			})
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode the program
			program, err := Decode[DeBruijn](encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Check if it's a builtin
			builtinTerm, ok := program.Term.(*Builtin)
			if !ok {
				t.Fatalf("Decoded term is not a Builtin: %T", program.Term)
			}

			// Compare
			if builtinTerm.DefaultFunction != tt.fn {
				t.Errorf(
					"Builtin functions not equal: got %v, want %v",
					builtinTerm.DefaultFunction,
					tt.fn,
				)
			}
		})
	}
}

func TestEncodeDecodeConstantTerm(t *testing.T) {
	tests := []struct {
		name     string
		constant IConstant
	}{
		{
			name:     "constant_integer",
			constant: &Integer{Inner: big.NewInt(99)},
		},
		{
			name:     "constant_string",
			constant: &String{Inner: "hello"},
		},
		{
			name:     "constant_bytestring",
			constant: &ByteString{Inner: []byte{0xde, 0xad, 0xbe, 0xef}},
		},
		{
			name:     "constant_unit",
			constant: &Unit{},
		},
		{
			name:     "constant_bool_true",
			constant: &Bool{Inner: true},
		},
		{
			name:     "constant_bool_false",
			constant: &Bool{Inner: false},
		},
		{
			name: "constant_list_integer",
			constant: &ProtoList{
				LTyp: &TInteger{},
				List: []IConstant{
					&Integer{Inner: big.NewInt(1)},
					&Integer{Inner: big.NewInt(2)},
					&Integer{Inner: big.NewInt(3)},
				},
			},
		},
		{
			name: "constant_pair_integer_bytestring",
			constant: &ProtoPair{
				FstType: &TInteger{},
				SndType: &TByteString{},
				First:   &Integer{Inner: big.NewInt(42)},
				Second:  &ByteString{Inner: []byte{0xca, 0xfe}},
			},
		},
		{
			name: "constant_list_of_pairs",
			constant: &ProtoList{
				LTyp: &TPair{First: &TInteger{}, Second: &TByteString{}},
				List: []IConstant{
					&ProtoPair{
						FstType: &TInteger{},
						SndType: &TByteString{},
						First:   &Integer{Inner: big.NewInt(1)},
						Second:  &ByteString{Inner: []byte{0xab}},
					},
				},
			},
		},
		{
			name: "constant_pair_of_lists",
			constant: &ProtoPair{
				FstType: &TList{Typ: &TInteger{}},
				SndType: &TList{Typ: &TByteString{}},
				First:   &ProtoList{LTyp: &TInteger{}, List: []IConstant{&Integer{Inner: big.NewInt(1)}}},
				Second:  &ProtoList{LTyp: &TByteString{}, List: []IConstant{&ByteString{Inner: []byte{0xff}}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Constant{Con: tt.constant}

			// Encode the constant term
			encoded, err := Encode[DeBruijn](&Program[DeBruijn]{
				Version: lang.LanguageVersionV1,
				Term:    original,
			})
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode the program
			program, err := Decode[DeBruijn](encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Check if it's a constant
			constantTerm, ok := program.Term.(*Constant)
			if !ok {
				t.Fatalf("Decoded term is not a Constant: %T", program.Term)
			}

			// Compare constants
			if !constantsEqual(tt.constant, constantTerm.Con) {
				t.Errorf("Constants not equal after encode/decode")
				t.Errorf("Original: %+v", tt.constant)
				t.Errorf("Decoded: %+v", constantTerm.Con)
			}
		})
	}
}

func TestFlatRoundtrip(t *testing.T) {
	// Build a synthetic UPLC program that exercises all term types and
	// encoding paths: Lambda, Apply, Var, Constant (Integer, ByteString,
	// Data, Bool), Force, Delay, Builtin, Constr, Case, Error.
	// This ensures the encoder and decoder are inverses for non-trivial ASTs.
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersionV3,
		Term: &Lambda[DeBruijn]{
			ParameterName: DeBruijn(0),
			Body: &Lambda[DeBruijn]{
				ParameterName: DeBruijn(0),
				Body: &Lambda[DeBruijn]{
					ParameterName: DeBruijn(0),
					Body: &Case[DeBruijn]{
						Constr: &Apply[DeBruijn]{
							Function: &Apply[DeBruijn]{
								Function: &Force[DeBruijn]{
									Term: &Builtin{DefaultFunction: builtin.IfThenElse},
								},
								Argument: &Apply[DeBruijn]{
									Function: &Apply[DeBruijn]{
										Function: &Builtin{DefaultFunction: builtin.EqualsInteger},
										Argument: &Constant{Con: &Integer{Inner: big.NewInt(42)}},
									},
									Argument: &Constant{Con: &Integer{Inner: big.NewInt(42)}},
								},
							},
							Argument: &Constr[DeBruijn]{
								Tag:    0,
								Fields: []Term[DeBruijn]{},
							},
						},
						Branches: []Term[DeBruijn]{
							&Constr[DeBruijn]{
								Tag: 1,
								Fields: []Term[DeBruijn]{
									&Constant{Con: &Data{Inner: &data.Constr{Tag: 0, Fields: []data.PlutusData{}}}},
									&Constant{Con: &ByteString{Inner: []byte{0xca, 0xfe}}},
									&Constant{Con: &Bool{Inner: true}},
								},
							},
							&Delay[DeBruijn]{
								Term: &Apply[DeBruijn]{
									Function: &Var[DeBruijn]{Name: DeBruijn(3)},
									Argument: &Var[DeBruijn]{Name: DeBruijn(1)},
								},
							},
							&Error{},
						},
					},
				},
			},
		},
	}

	encoded, err := Encode(program)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode[DeBruijn](encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	reencoded, err := Encode(decoded)
	if err != nil {
		t.Fatalf("Re-encode failed: %v", err)
	}

	if !bytes.Equal(encoded, reencoded) {
		t.Errorf("roundtrip mismatch: %d bytes -> %d bytes", len(encoded), len(reencoded))
		origHex := hex.EncodeToString(encoded)
		reHex := hex.EncodeToString(reencoded)
		for i := 0; i < len(origHex) && i < len(reHex); i++ {
			if origHex[i] != reHex[i] {
				t.Logf("first diff at hex pos %d (byte %d)", i, i/2)
				start := max(0, i-20)
				t.Logf("  orig: ...%s...", origHex[start:min(len(origHex), i+40)])
				t.Logf("  ours: ...%s...", reHex[start:min(len(reHex), i+40)])
				break
			}
		}
	}
}

func TestFlatRoundtripConstantTypes(t *testing.T) {
	tests := []struct {
		name     string
		constant IConstant
	}{
		{
			name: "list_of_integers",
			constant: &ProtoList{
				LTyp: &TInteger{},
				List: []IConstant{
					&Integer{Inner: big.NewInt(1)},
					&Integer{Inner: big.NewInt(2)},
					&Integer{Inner: big.NewInt(3)},
				},
			},
		},
		{
			name:     "empty_list_of_integers",
			constant: &ProtoList{LTyp: &TInteger{}, List: []IConstant{}},
		},
		{
			name: "list_of_bytestrings",
			constant: &ProtoList{
				LTyp: &TByteString{},
				List: []IConstant{
					&ByteString{Inner: []byte{0xca, 0xfe}},
					&ByteString{Inner: []byte{0xde, 0xad}},
				},
			},
		},
		{
			name: "pair_integer_bytestring",
			constant: &ProtoPair{
				FstType: &TInteger{},
				SndType: &TByteString{},
				First:   &Integer{Inner: big.NewInt(42)},
				Second:  &ByteString{Inner: []byte{0xca, 0xfe}},
			},
		},
		{
			name: "pair_of_lists",
			constant: &ProtoPair{
				FstType: &TList{Typ: &TInteger{}},
				SndType: &TList{Typ: &TByteString{}},
				First:   &ProtoList{LTyp: &TInteger{}, List: []IConstant{&Integer{Inner: big.NewInt(1)}}},
				Second:  &ProtoList{LTyp: &TByteString{}, List: []IConstant{&ByteString{Inner: []byte{0xff}}}},
			},
		},
		{
			name: "list_of_pairs",
			constant: &ProtoList{
				LTyp: &TPair{First: &TInteger{}, Second: &TByteString{}},
				List: []IConstant{
					&ProtoPair{
						FstType: &TInteger{},
						SndType: &TByteString{},
						First:   &Integer{Inner: big.NewInt(10)},
						Second:  &ByteString{Inner: []byte{0xab}},
					},
					&ProtoPair{
						FstType: &TInteger{},
						SndType: &TByteString{},
						First:   &Integer{Inner: big.NewInt(20)},
						Second:  &ByteString{Inner: []byte{0xcd}},
					},
				},
			},
		},
		{
			name: "nested_list_of_lists",
			constant: &ProtoList{
				LTyp: &TList{Typ: &TInteger{}},
				List: []IConstant{
					&ProtoList{LTyp: &TInteger{}, List: []IConstant{&Integer{Inner: big.NewInt(1)}, &Integer{Inner: big.NewInt(2)}}},
					&ProtoList{LTyp: &TInteger{}, List: []IConstant{&Integer{Inner: big.NewInt(3)}}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := &Program[DeBruijn]{
				Version: lang.LanguageVersionV1,
				Term:    &Constant{Con: tt.constant},
			}

			encoded, err := Encode(program)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded, err := Decode[DeBruijn](encoded)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			reencoded, err := Encode(decoded)
			if err != nil {
				t.Fatalf("Re-encode failed: %v", err)
			}

			if !bytes.Equal(encoded, reencoded) {
				t.Errorf("roundtrip mismatch: %d bytes -> %d bytes", len(encoded), len(reencoded))
				t.Errorf("  encoded:    %x", encoded)
				t.Errorf("  re-encoded: %x", reencoded)
			}
		})
	}
}

// typsEqual performs deep equality on Typ values, including nested TList/TPair.
func typsEqual(a, b Typ) bool {
	switch va := a.(type) {
	case *TList:
		if vb, ok := b.(*TList); ok {
			return typsEqual(va.Typ, vb.Typ)
		}
		return false
	case *TPair:
		if vb, ok := b.(*TPair); ok {
			return typsEqual(va.First, vb.First) && typsEqual(va.Second, vb.Second)
		}
		return false
	default:
		return reflect.TypeOf(a) == reflect.TypeOf(b)
	}
}

// Helper function to compare constants for equality
func constantsEqual(a, b IConstant) bool {
	switch va := a.(type) {
	case *Integer:
		if vb, ok := b.(*Integer); ok {
			return va.Inner.Cmp(vb.Inner) == 0
		}
	case *ByteString:
		if vb, ok := b.(*ByteString); ok {
			if len(va.Inner) != len(vb.Inner) {
				return false
			}
			for i := range va.Inner {
				if va.Inner[i] != vb.Inner[i] {
					return false
				}
			}
			return true
		}
	case *String:
		if vb, ok := b.(*String); ok {
			return va.Inner == vb.Inner
		}
	case *Unit:
		if _, ok := b.(*Unit); ok {
			return true
		}
	case *Bool:
		if vb, ok := b.(*Bool); ok {
			return va.Inner == vb.Inner
		}
	case *ProtoList:
		if vb, ok := b.(*ProtoList); ok {
			if !typsEqual(va.LTyp, vb.LTyp) {
				return false
			}
			if len(va.List) != len(vb.List) {
				return false
			}
			for i := range va.List {
				if !constantsEqual(va.List[i], vb.List[i]) {
					return false
				}
			}
			return true
		}
	case *ProtoPair:
		if vb, ok := b.(*ProtoPair); ok {
			if !typsEqual(va.FstType, vb.FstType) ||
				!typsEqual(va.SndType, vb.SndType) {
				return false
			}
			return constantsEqual(va.First, vb.First) && constantsEqual(va.Second, vb.Second)
		}
	}
	return false
}
