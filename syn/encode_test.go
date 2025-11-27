package syn

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
)

func TestEncodeDecodeConstant(t *testing.T) {
	// Simple test first
	constant := &Integer{Inner: big.NewInt(42)}
	term := &Constant{Con: constant}

	// Encode the term
	encoded, err := Encode[DeBruijn](&Program[DeBruijn]{
		Version: [3]uint32{1, 0, 0},
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
			encoded, err := Encode[DeBruijn](&Program[DeBruijn]{
				Version: [3]uint32{1, 0, 0},
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
			name: "constant_bool_true",
			constant: &Bool{Inner: true},
		},
		{
			name: "constant_bool_false",
			constant: &Bool{Inner: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Constant{Con: tt.constant}

			// Encode the constant term
			encoded, err := Encode[DeBruijn](&Program[DeBruijn]{
				Version: [3]uint32{1, 0, 0},
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
			return constantsEqual(va.First, vb.First) && constantsEqual(va.Second, vb.Second)
		}
	}
	return false
}
