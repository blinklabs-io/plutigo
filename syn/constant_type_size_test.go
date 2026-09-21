package syn

import (
	"fmt"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

// nestedListConstant returns an empty list constant whose type is `integer`
// wrapped in depth list constructors, which encodes to 2*depth+1 type tags.
func nestedListConstant(depth int) IConstant {
	var typ Typ = &TInteger{}
	for i := 1; i < depth; i++ {
		typ = &TList{Typ: typ}
	}
	return &ProtoList{LTyp: typ}
}

// TestContextualValidationConstantTypeSizeLimit covers the PV11 bound on the
// size of a constant's encoded type (mbHeader in plutus-ledger-api's
// MaxBounds, checked by scriptCBORDecoder's checkConstant against
// defaultUniSize). Encoded type sizes are always odd -- a base type is one
// tag, a list or array adds two, a pair adds three -- so 32 itself is
// unreachable and the first rejected size is 33.
func TestContextualValidationConstantTypeSizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		protocol uint
		depth    int
		wantErr  string
	}{
		{
			name:     "pre-PV11 remains unbounded",
			protocol: 10,
			depth:    64,
		},
		{
			name:     "PV11 below limit",
			protocol: 11,
			depth:    14,
		},
		{
			name:     "PV11 at largest reachable size",
			protocol: 11,
			depth:    15,
		},
		{
			name:     "PV11 above limit",
			protocol: 11,
			depth:    16,
			wantErr:  "constant type of size 33 is not available in protocol version 11",
		},
		{
			name:     "post-PV11 above limit",
			protocol: 12,
			depth:    16,
			wantErr:  "constant type of size 33 is not available in protocol version 12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			program := &Program[DeBruijn]{
				Version: uplcVersion100,
				Term:    &Constant{Con: nestedListConstant(tt.depth)},
			}
			context := ProgramContext{
				LedgerLanguage: lang.LanguageVersionV3,
				ProtocolMajor:  tt.protocol,
			}
			encoded, err := Encode(program)
			if err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}

			entryPoints := []struct {
				name string
				run  func() error
			}{
				{
					name: "ValidateProgram",
					run:  func() error { return ValidateProgram(program, context) },
				},
				{
					name: "DecodeWithContext generic",
					run: func() error {
						decoded, err := DecodeWithContext[Name](encoded, context)
						if err != nil {
							return err
						}
						return assertNestedListConstant(decoded.Term, tt.depth)
					},
				},
				{
					name: "DecodeDeBruijnWithContext",
					run: func() error {
						decoded, err := DecodeDeBruijnWithContext(encoded, context)
						if err != nil {
							return err
						}
						return assertNestedListConstant(decoded.Term, tt.depth)
					},
				},
			}

			for _, entryPoint := range entryPoints {
				t.Run(entryPoint.name, func(t *testing.T) {
					t.Parallel()

					err := entryPoint.run()
					if tt.wantErr != "" {
						if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
							t.Fatalf("error = %v, want %q", err, tt.wantErr)
						}
						return
					}
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
				})
			}
		})
	}
}

// assertNestedListConstant reports whether term is the constant built by
// nestedListConstant(depth), so that an accepted program is checked to have
// decoded intact rather than merely to have not errored.
func assertNestedListConstant(term any, depth int) error {
	constant, ok := term.(*Constant)
	if !ok {
		return fmt.Errorf("term is %T, want *Constant", term)
	}
	typ := constant.Con.Typ()
	for i := 0; i < depth; i++ {
		list, ok := typ.(*TList)
		if !ok {
			return fmt.Errorf("type nesting level %d is %T, want *TList", i, typ)
		}
		typ = list.Typ
	}
	if _, ok := typ.(*TInteger); !ok {
		return fmt.Errorf("innermost type is %T, want *TInteger", typ)
	}
	return nil
}
