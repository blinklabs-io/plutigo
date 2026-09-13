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

package cek

// Benchmarks for plutigo#402: cost-model construction via NewEvalContext,
// for full-length parameter lists, and an end-to-end benchmark showing what
// share of a small script evaluation construction takes.

import (
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// BenchmarkNewEvalContext measures cost-model construction for each language
// version's full-length parameter list, at proto-major versions selected to
// exercise the semantics variant that version reaches (cek/semantics.go).
// V1 and V3 use the real preview-network cost models; V2 has no real fixture
// in this repository (see cost_model_fixtures_test.go), so it uses a
// synthetic full-length list of the same shape.
func BenchmarkNewEvalContext(b *testing.B) {
	cases := []struct {
		name         string
		version      lang.LanguageVersion
		protoVersion ProtoVersion
		params       []int64
	}{
		{
			"V1/VariantA",
			lang.LanguageVersionV1,
			ProtoVersion{Major: 5},
			v1ParamsFromRealMap(),
		},
		{
			"V1/VariantD",
			lang.LanguageVersionV1,
			ProtoVersion{Major: 11},
			v1ParamsFromRealMap(),
		},
		{
			"V2/VariantB",
			lang.LanguageVersionV2,
			ProtoVersion{Major: 9},
			synthCostModelParams(lang.CostModelParamNamesV2),
		},
		{
			"V3/VariantC",
			lang.LanguageVersionV3,
			ProtoVersion{Major: 10},
			realPreviewV3CostModelParams,
		},
		{
			"V3/VariantE",
			lang.LanguageVersionV3,
			ProtoVersion{Major: 11},
			realPreviewV3CostModelParams,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ctx, err := NewEvalContext(tc.version, tc.protoVersion, tc.params)
				if err != nil {
					b.Fatalf("NewEvalContext: %v", err)
				}
				benchmarkEvalContextSink = ctx
			}
		})
	}
}

// v1ParamsFromRealMap converts realPreviewV1CostModelParams (a map, matching
// how the existing V1 fixture and costModelFromMap are used elsewhere in
// this package) into a positional list ordered by
// lang.CostModelParamNamesV1, so BenchmarkNewEvalContext can exercise
// NewEvalContext (which takes []int64) with real V1 values.
func v1ParamsFromRealMap() []int64 {
	names := lang.CostModelParamNamesV1
	params := make([]int64, 0, len(names))
	for _, name := range names {
		val, ok := realPreviewV1CostModelParams[name]
		if !ok {
			// The real preview fixture predates a handful of later V1 param
			// names; fall back to a distinguishable synthetic value so the
			// benchmark still exercises the full parameter count.
			val = 1_000_000
		}
		params = append(params, val)
	}
	return params
}

var benchmarkEvalContextSink *EvalContext

// BenchmarkNewEvalContextAndEvaluate constructs an EvalContext from a real
// V3 cost model and evaluates a small script with it, once per iteration,
// showing what share of a redeemer evaluation construction (the per-call
// cost this issue is about) actually accounts for.
func BenchmarkNewEvalContextAndEvaluate(b *testing.B) {
	protoVersion := ProtoVersion{Major: 10}
	term := mustBenchmarkTerm(b, buildLambdaChainProgram(16))

	b.ReportAllocs()
	for b.Loop() {
		ctx, err := NewEvalContext(lang.LanguageVersionV3, protoVersion, realPreviewV3CostModelParams)
		if err != nil {
			b.Fatalf("NewEvalContext: %v", err)
		}
		machine := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 200, ctx)
		result, err := machine.Run(term)
		if err != nil {
			b.Fatalf("Run: %v", err)
		}
		benchmarkTermSink = result
	}
}

// BenchmarkEvaluateOnly is the same evaluation as
// BenchmarkNewEvalContextAndEvaluate but with the EvalContext built once
// outside the timed loop, isolating steady-state evaluation cost from
// construction cost. The delta between this and
// BenchmarkNewEvalContextAndEvaluate is what NewEvalContext costs per call
// for this script.
func BenchmarkEvaluateOnly(b *testing.B) {
	protoVersion := ProtoVersion{Major: 10}
	term := mustBenchmarkTerm(b, buildLambdaChainProgram(16))
	ctx, err := NewEvalContext(lang.LanguageVersionV3, protoVersion, realPreviewV3CostModelParams)
	if err != nil {
		b.Fatalf("NewEvalContext: %v", err)
	}

	b.ReportAllocs()
	for b.Loop() {
		machine := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 200, ctx)
		result, err := machine.Run(term)
		if err != nil {
			b.Fatalf("Run: %v", err)
		}
		benchmarkTermSink = result
	}
}
