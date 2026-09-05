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

import (
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
)

// addIntegerCpuIntercept reads the one cost value the tests below drive, so a
// failure names a number rather than a pointer.
func addIntegerCpuIntercept(t *testing.T, costs BuiltinCosts) int64 {
	t.Helper()
	costingFunc := costs[builtin.AddInteger]
	if costingFunc == nil {
		t.Fatal("no costing function for addInteger")
	}
	model, ok := costingFunc.cpu.(*MaxSizeModel)
	if !ok {
		t.Fatalf("addInteger cpu model is %T, want *MaxSizeModel", costingFunc.cpu)
	}
	return model.intercept
}

// TestBuiltinCostsCloneDoesNotAliasSource pins the array-of-pointers hazard:
// Clone copies pointers unless it allocates, and update writes through them.
func TestBuiltinCostsCloneDoesNotAliasSource(t *testing.T) {
	source := DefaultBuiltinCosts.Clone()
	before := addIntegerCpuIntercept(t, source)

	cloned := source.Clone()
	if source[builtin.AddInteger] == cloned[builtin.AddInteger] {
		t.Errorf(
			"clone shares the addInteger CostingFunc pointer with its source: %p",
			source[builtin.AddInteger],
		)
	}
	if source[builtin.AddInteger].cpu == cloned[builtin.AddInteger].cpu {
		t.Errorf(
			"clone shares the addInteger cpu model with its source: %#v",
			source[builtin.AddInteger].cpu,
		)
	}

	if err := cloned.update("addInteger-cpu-arguments-intercept", 999999); err != nil {
		t.Fatalf("update clone: %v", err)
	}
	if got := addIntegerCpuIntercept(t, source); got != before {
		t.Errorf(
			"updating the clone changed its source: intercept %d -> %d",
			before,
			got,
		)
	}
	if got := addIntegerCpuIntercept(t, cloned); got != 999999 {
		t.Errorf("update did not take effect on the clone: intercept = %d", got)
	}
}

// TestBuildBuiltinCostsAreIndependent covers the production entry point rather
// than Clone directly: two cost models built from the same defaults must not
// see each other's protocol parameters.
func TestBuildBuiltinCostsAreIndependent(t *testing.T) {
	first, err := buildBuiltinCosts(lang.LanguageVersionV3, SemanticsVariantE)
	if err != nil {
		t.Fatalf("build first: %v", err)
	}
	second, err := buildBuiltinCosts(lang.LanguageVersionV3, SemanticsVariantE)
	if err != nil {
		t.Fatalf("build second: %v", err)
	}
	before := addIntegerCpuIntercept(t, second)

	if err := first.update("addInteger-cpu-arguments-intercept", 111111); err != nil {
		t.Fatalf("update first: %v", err)
	}
	if got := addIntegerCpuIntercept(t, second); got != before {
		t.Errorf(
			"updating one built cost model changed another: intercept %d -> %d",
			before,
			got,
		)
	}
}

// TestDefaultBuiltinCostsSurviveCostModelConstruction is the consequence that
// makes this a correctness defect rather than a hygiene one: costModelFromList
// runs update for every protocol parameter, so a leaking clone leaves those
// values in the package-level defaults for every later evaluation.
func TestDefaultBuiltinCostsSurviveCostModelConstruction(t *testing.T) {
	before := addIntegerCpuIntercept(t, DefaultBuiltinCosts)

	params := make([]int64, len(lang.GetParamNamesForVersion(lang.LanguageVersionV3)))
	for i := range params {
		params[i] = 1
	}
	if _, err := costModelFromList(
		lang.LanguageVersionV3,
		SemanticsVariantE,
		params,
	); err != nil {
		t.Fatalf("costModelFromList: %v", err)
	}

	if got := addIntegerCpuIntercept(t, DefaultBuiltinCosts); got != before {
		t.Errorf(
			"building a cost model mutated DefaultBuiltinCosts: intercept %d -> %d",
			before,
			got,
		)
	}
}

// TestNewEvalContextIsRaceFree drives the exported entry point Dingo calls per
// script evaluation. Without independent cost models this reports a data race
// under -race and returns differing costs without it.
func TestNewEvalContextIsRaceFree(t *testing.T) {
	params := make([]int64, len(lang.GetParamNamesForVersion(lang.LanguageVersionV3)))
	for i := range params {
		params[i] = int64(i + 1)
	}
	var wg sync.WaitGroup
	intercepts := make([]int64, 16)
	for i := range intercepts {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, err := NewEvalContext(
				lang.LanguageVersionV3,
				ProtoVersion{Major: 10},
				params,
			)
			if err != nil {
				t.Errorf("NewEvalContext: %v", err)
				return
			}
			costingFunc := ctx.CostModel.builtinCosts[builtin.AddInteger]
			if model, ok := costingFunc.cpu.(*MaxSizeModel); ok {
				intercepts[idx] = model.intercept
			}
		}(i)
	}
	wg.Wait()
	for i, got := range intercepts {
		if got != intercepts[0] {
			t.Errorf(
				"concurrent NewEvalContext produced differing costs: [0]=%d [%d]=%d",
				intercepts[0],
				i,
				got,
			)
		}
	}
}
