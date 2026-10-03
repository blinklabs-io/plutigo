package cek

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

const addProgram = `(program 1.1.0
	[ [ (builtin addInteger) (con integer 1) ] (con integer 1) ])`

func TestEvaluateTextRunsValidatedProgram(t *testing.T) {
	t.Parallel()

	evalCtx := NewDefaultEvalContext(
		lang.LanguageVersionV3,
		ProtoVersion{Major: 11},
	)
	term, consumed, err := EvaluateText(
		context.Background(),
		addProgram,
		lang.LanguageVersionV3,
		evalCtx,
		DefaultExBudget,
	)
	if err != nil {
		t.Fatalf("EvaluateText: %v", err)
	}
	if got, want := syn.PrettyTerm[syn.DeBruijn](term), "(con integer 2)"; got != want {
		t.Fatalf("result = %s, want %s", got, want)
	}
	if consumed.Cpu <= 0 || consumed.Mem <= 0 {
		t.Fatalf("consumed budget = %+v, want positive", consumed)
	}
}

func TestEvaluateTextTakesLanguageFromCaller(t *testing.T) {
	t.Parallel()

	// andByteString is available to V3 but not V2 at PV10. The header is
	// identical in both runs, so only the caller's language can decide.
	const src = `(program 1.0.0 (builtin andByteString))`
	for _, tt := range []struct {
		language lang.LanguageVersion
		proto    uint
		wantErr  bool
	}{
		{lang.LanguageVersionV3, 10, false},
		{lang.LanguageVersionV2, 10, true},
	} {
		evalCtx := NewDefaultEvalContext(tt.language, ProtoVersion{Major: tt.proto})
		_, _, err := EvaluateText(
			context.Background(),
			src,
			tt.language,
			evalCtx,
			DefaultExBudget,
		)
		if (err != nil) != tt.wantErr {
			t.Fatalf("language %v: error = %v, wantErr %t", tt.language, err, tt.wantErr)
		}
	}
}

func TestEvaluateTextRejectsProgramsFailingValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		src       string
		language  lang.LanguageVersion
		proto     uint
		wantError string
	}{
		{
			name:      "builtin not yet available",
			src:       `(program 1.0.0 (builtin expModInteger))`,
			language:  lang.LanguageVersionV3,
			proto:     10,
			wantError: "not available",
		},
		{
			name:      "execution version gate",
			src:       `(program 1.1.0 (con integer 1))`,
			language:  lang.LanguageVersionV1,
			proto:     10,
			wantError: "UPLC version 1.1.0",
		},
		{
			name:      "unsupported term version",
			src:       `(program 1.2.0 (con integer 1))`,
			language:  lang.LanguageVersionV3,
			proto:     11,
			wantError: "unsupported UPLC program version",
		},
		{
			name:      "language not yet introduced",
			src:       `(program 1.0.0 (con integer 1))`,
			language:  lang.LanguageVersionV3,
			proto:     8,
			wantError: "not available at protocol version 8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			evalCtx := NewDefaultEvalContext(tt.language, ProtoVersion{Major: tt.proto})
			term, consumed, err := EvaluateText(
				context.Background(),
				tt.src,
				tt.language,
				evalCtx,
				DefaultExBudget,
			)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
			if term != nil || consumed != (ExBudget{}) {
				t.Fatalf("evaluated a rejected program: %v, %+v", term, consumed)
			}
		})
	}
}

func TestEvaluateTextRejectsMismatchedContext(t *testing.T) {
	t.Parallel()

	v1Ctx := NewDefaultEvalContext(lang.LanguageVersionV1, ProtoVersion{Major: 11})
	for name, evalCtx := range map[string]*EvalContext{
		"nil":                nil,
		"other language":     v1Ctx,
		"no protocol":        testEvalContext(),
		"zero machine costs": {SemanticsVariant: SemanticsVariantE, ProtoMajor: 11},
	} {
		_, _, err := EvaluateText(
			context.Background(),
			addProgram,
			lang.LanguageVersionV3,
			evalCtx,
			DefaultExBudget,
		)
		if err == nil {
			t.Fatalf("%s: EvaluateText accepted the context", name)
		}
	}
}

func TestEvaluateTextHonorsCancellation(t *testing.T) {
	t.Parallel()

	evalCtx := NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: 11})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := EvaluateText(ctx, addProgram, lang.LanguageVersionV3, evalCtx, DefaultExBudget)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}
