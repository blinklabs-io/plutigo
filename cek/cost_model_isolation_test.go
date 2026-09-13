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

// TestCostModelConstructionIsolation guards the plutigo#395/#396 class of
// defect: building a CostModel from one parameter list must not leak into a
// CostModel built from a different list, and must never mutate the
// package-level DefaultCostModel/DefaultBuiltinCosts. plutigo#402 adds shared
// read-only state (paramNameParts, cost_model_param_names.go); this test's
// concurrent phase, run under -race, checks that it does not reopen the class
// BuiltinCosts.Clone() was fixed for.

import (
	"fmt"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

func TestCostModelConstructionIsolation(t *testing.T) {
	t.Parallel()

	names := lang.CostModelParamNamesV3
	paramsA := synthCostModelParams(names)
	paramsB := make([]int64, len(names))
	for i := range paramsB {
		paramsB[i] = 5_000_000 + int64(i)
	}

	baselineDefault := dumpCostModel(DefaultCostModel)

	cmA, err := costModelFromList(lang.LanguageVersionV3, SemanticsVariantC, paramsA)
	if err != nil {
		t.Fatalf("costModelFromList(paramsA): %v", err)
	}
	cmB, err := costModelFromList(lang.LanguageVersionV3, SemanticsVariantC, paramsB)
	if err != nil {
		t.Fatalf("costModelFromList(paramsB): %v", err)
	}

	dumpA := dumpCostModel(cmA)
	dumpB := dumpCostModel(cmB)

	if dumpA == dumpB {
		t.Fatalf("two distinct parameter lists produced identical cost models")
	}
	if dumpCostModel(cmA) != dumpA {
		t.Fatalf("re-dumping cmA changed its value; something is mutating it after construction")
	}
	if dumpCostModel(DefaultCostModel) != baselineDefault {
		t.Fatalf("building cmA/cmB mutated the package-level DefaultCostModel (plutigo#395 class)")
	}

	// Concurrent phase: many goroutines alternate building from paramsA and
	// paramsB. Run with -race; each goroutine must observe exactly the
	// CostModel for the params it built, and DefaultCostModel must never be
	// seen in a mutated state.
	const goroutines = 48
	const iterations = 40

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := range goroutines {
		params, want := paramsA, dumpA
		if g%2 == 1 {
			params, want = paramsB, dumpB
		}

		wg.Add(1)
		go func(params []int64, want string) {
			defer wg.Done()

			for range iterations {
				cm, err := costModelFromList(lang.LanguageVersionV3, SemanticsVariantC, params)
				if err != nil {
					errCh <- fmt.Errorf("costModelFromList: %w", err)
					return
				}
				if got := dumpCostModel(cm); got != want {
					errCh <- fmt.Errorf("concurrent build produced an unexpected cost model")
					return
				}
				if got := dumpCostModel(DefaultCostModel); got != baselineDefault {
					errCh <- fmt.Errorf("DefaultCostModel observed mutated during concurrent construction")
					return
				}
			}
		}(params, want)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}
