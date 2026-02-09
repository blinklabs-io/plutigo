package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestPV11ProtoVersionPropagation(t *testing.T) {
	// Verify protocol version is correctly propagated through EvalContext to Machine
	protoVersion := ProtoVersion{Major: 11, Minor: 0}

	// V1 language version with PV11 protocol version
	ctx, err := NewEvalContext(lang.LanguageVersionV1, protoVersion, nil)
	if err != nil {
		t.Fatalf("NewEvalContext failed: %v", err)
	}
	if ctx.ProtoMajor != 11 {
		t.Errorf("EvalContext.ProtoMajor = %d, want 11", ctx.ProtoMajor)
	}
}

func TestPV11BuiltinAvailabilityInMachine(t *testing.T) {
	tests := []struct {
		name        string
		langVersion lang.LanguageVersion
		protoMajor  uint
		builtin     builtin.DefaultFunction
		wantAvail   bool
	}{
		// Pre-PV11: V3 builtin NOT available in V1
		{"V3 builtin in V1 at PV10", lang.LanguageVersionV1, 10, builtin.Bls12_381_G1_Add, false},
		// PV11: V3 builtin available in V1
		{"V3 builtin in V1 at PV11", lang.LanguageVersionV1, 11, builtin.Bls12_381_G1_Add, true},
		// PV11: V4 builtin available in V1
		{"V4 builtin in V1 at PV11", lang.LanguageVersionV1, 11, builtin.LengthOfArray, true},
		// PV11: DropList available in V1
		{"DropList in V1 at PV11", lang.LanguageVersionV1, 11, builtin.DropList, true},
		// PV11: DropList not available pre-PV11
		{"DropList in V4 at PV10", lang.LanguageVersionV4, 10, builtin.DropList, false},
		// PV11: MultiIndexArray still V4-only
		{"MultiIndexArray in V1 at PV11", lang.LanguageVersionV1, 11, builtin.MultiIndexArray, false},
		{"MultiIndexArray in V4 at PV11", lang.LanguageVersionV4, 11, builtin.MultiIndexArray, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plutusVersion := builtin.LanguageVersionToPlutusVersion(tt.langVersion)
			got := tt.builtin.IsAvailableInWithProto(plutusVersion, tt.protoMajor)
			if got != tt.wantAvail {
				t.Errorf("IsAvailableInWithProto(%v, %d) = %v, want %v",
					plutusVersion, tt.protoMajor, got, tt.wantAvail)
			}
		})
	}
}

func TestPV11DefaultProtoVersion(t *testing.T) {
	// When no EvalContext is provided, protoMajor should default to 0
	// which means pre-PV11 behavior (backward compatible)
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV1, 0, nil)
	if m.protoMajor != 0 {
		t.Errorf("default protoMajor = %d, want 0", m.protoMajor)
	}
}
