package cek

import (
	"errors"
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

func TestNewDefaultEvalContext(t *testing.T) {
	ctx := NewDefaultEvalContext(
		lang.LanguageVersionV1,
		ProtoVersion{Major: 11},
	)

	if ctx.CostModel != DefaultCostModel {
		t.Fatal("NewDefaultEvalContext should use DefaultCostModel")
	}
	if ctx.ProtoMajor != 11 {
		t.Fatalf("ProtoMajor = %d, want 11", ctx.ProtoMajor)
	}
	if ctx.SemanticsVariant != SemanticsVariantB {
		t.Fatalf("SemanticsVariant = %v, want %v", ctx.SemanticsVariant, SemanticsVariantB)
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
		{
			"V3 builtin in V1 at PV10",
			lang.LanguageVersionV1,
			10,
			builtin.Bls12_381_G1_Add,
			false,
		},
		// PV11: V3 builtin available in V1
		{
			"V3 builtin in V1 at PV11",
			lang.LanguageVersionV1,
			11,
			builtin.Bls12_381_G1_Add,
			true,
		},
		// PV11: V4 builtin available in V1
		{
			"V4 builtin in V1 at PV11",
			lang.LanguageVersionV1,
			11,
			builtin.LengthOfArray,
			true,
		},
		// PV11: DropList available in V1
		{
			"DropList in V1 at PV11",
			lang.LanguageVersionV1,
			11,
			builtin.DropList,
			true,
		},
		// PV11: DropList not available pre-PV11
		{
			"DropList in V4 at PV10",
			lang.LanguageVersionV4,
			10,
			builtin.DropList,
			false,
		},
		// PV11: MultiIndexArray still V4-only
		{
			"MultiIndexArray in V1 at PV11",
			lang.LanguageVersionV1,
			11,
			builtin.MultiIndexArray,
			false,
		},
		{
			"MultiIndexArray in V4 at PV11",
			lang.LanguageVersionV4,
			11,
			builtin.MultiIndexArray,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plutusVersion := builtin.LanguageVersionToPlutusVersion(
				tt.langVersion,
			)
			got := tt.builtin.IsAvailableInWithProto(
				plutusVersion,
				tt.protoMajor,
			)
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

func TestUnavailableBuiltinReturnsBuiltinError(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV1, 0, nil)
	b := &Builtin[syn.DeBruijn]{
		Func: builtin.Bls12_381_G1_Add,
	}

	_, err := m.evalBuiltinApp(b)
	if err == nil {
		t.Fatal("expected unavailable builtin error")
	}
	if !IsBuiltinError(err) {
		t.Fatalf("expected builtin-classified error, got %T: %v", err, err)
	}

	var builtinErr *BuiltinError
	if !errors.As(err, &builtinErr) {
		t.Fatalf("expected BuiltinError, got %T: %v", err, err)
	}
	if builtinErr.Code != ErrCodeBuiltinFailure {
		t.Fatalf("BuiltinError.Code = %v, want %v", builtinErr.Code, ErrCodeBuiltinFailure)
	}
	if builtinErr.Builtin != builtin.Bls12_381_G1_Add.String() {
		t.Fatalf("BuiltinError.Builtin = %q, want %q", builtinErr.Builtin, builtin.Bls12_381_G1_Add.String())
	}
	want := "builtin " + builtin.Bls12_381_G1_Add.String() + " is not available in Plutus V1 at protocol version 0 (introduced in V3)"
	if builtinErr.Error() != want {
		t.Fatalf("BuiltinError.Error() = %q, want %q", builtinErr.Error(), want)
	}
}
