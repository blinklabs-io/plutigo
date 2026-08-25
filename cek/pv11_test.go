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

	if ctx.ProtoMajor != 11 {
		t.Fatalf("ProtoMajor = %d, want 11", ctx.ProtoMajor)
	}
	if ctx.SemanticsVariant != SemanticsVariantD {
		t.Fatalf("SemanticsVariant = %v, want %v", ctx.SemanticsVariant, SemanticsVariantD)
	}
}

func TestGetSemanticsVanRossemMapping(t *testing.T) {
	tests := []struct {
		name    string
		version lang.LanguageVersion
		proto   uint
		want    SemanticsVariant
	}{
		{"V1 pre-Chang", lang.LanguageVersionV1, 8, SemanticsVariantA},
		{"V2 Chang", lang.LanguageVersionV2, 9, SemanticsVariantB},
		{"V1 van Rossem", lang.LanguageVersionV1, 11, SemanticsVariantD},
		{"V2 van Rossem", lang.LanguageVersionV2, 11, SemanticsVariantD},
		{"V3 pre-van Rossem", lang.LanguageVersionV3, 10, SemanticsVariantC},
		{"V3 van Rossem", lang.LanguageVersionV3, 11, SemanticsVariantE},
		{"V4 van Rossem", lang.LanguageVersionV4, 11, SemanticsVariantE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSemantics(tt.version, ProtoVersion{Major: tt.proto})
			if got != tt.want {
				t.Fatalf("GetSemantics(%v, PV%d) = %v, want %v", tt.version, tt.proto, got, tt.want)
			}
		})
	}
}

func TestBuiltinAvailabilityMatrixInMachine(t *testing.T) {
	tests := []struct {
		name        string
		langVersion lang.LanguageVersion
		protoMajor  uint
		builtin     builtin.DefaultFunction
		wantAvail   bool
	}{
		{
			"batch 3 in V2 at PV7",
			lang.LanguageVersionV2,
			7,
			builtin.VerifyEcdsaSecp256k1Signature,
			false,
		},
		{
			"batch 4b in V2 at PV10",
			lang.LanguageVersionV2,
			10,
			builtin.IntegerToByteString,
			true,
		},
		{
			"batch 5 in V2 at PV10",
			lang.LanguageVersionV2,
			10,
			builtin.AndByteString,
			false,
		},
		{
			"batch 5 in V3 at PV10",
			lang.LanguageVersionV3,
			10,
			builtin.AndByteString,
			true,
		},
		{
			"batch 6 in V1 at PV11",
			lang.LanguageVersionV1,
			11,
			builtin.DropList,
			true,
		},
		{
			"batch 1 in V4 at PV11",
			lang.LanguageVersionV4,
			11,
			builtin.AddInteger,
			false,
		},
		{
			"batch 6 in V4 at PV12",
			lang.LanguageVersionV4,
			12,
			builtin.DropList,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMachine[syn.DeBruijn](
				tt.langVersion,
				0,
				NewDefaultEvalContext(
					tt.langVersion,
					ProtoVersion{Major: tt.protoMajor},
				),
			)
			got := m.builtins[tt.builtin] != nil &&
				(m.available == nil || m.available[tt.builtin])
			if got != tt.wantAvail {
				t.Errorf(
					"machine availability for %s at PV%d = %v, want %v",
					tt.builtin,
					tt.protoMajor,
					got,
					tt.wantAvail,
				)
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
