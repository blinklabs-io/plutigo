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

// TestEvalContextReuseIsRaceFree proves the reuse guarantee documented on
// EvalContext (plutigo#402 acceptance criterion 2): a single *EvalContext,
// constructed once, is safe to share across many goroutines that each build
// their own Machine from it and evaluate concurrently.
//
// This is a different phase than the existing races the cek package already
// covers: TestNewEvalContextIsRaceFree (cost_model_clone_test.go) and
// TestCostModelConstructionIsolation (cost_model_isolation_test.go) both call
// NewEvalContext / costModelFromList *inside* each goroutine, so they prove
// concurrent construction is isolated. This test calls NewEvalContext exactly
// once, outside every goroutine, and only reads the resulting *EvalContext
// from then on -- proving reuse of one built context is race-free, not just
// that building many contexts concurrently is.
//
// Run with -race; every goroutine evaluating the same program against the
// same shared EvalContext must observe the same result and the same
// consumed budget.

import (
	"fmt"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestEvalContextReuseIsRaceFree(t *testing.T) {
	t.Parallel()

	names := lang.GetParamNamesForVersion(lang.LanguageVersionV3)
	params := make([]int64, len(names))
	for i := range params {
		params[i] = int64(i + 1)
	}

	// Constructed exactly once. Every goroutine below shares this same
	// pointer; none of them ever calls NewEvalContext or costModelFromList
	// themselves.
	sharedCtx, err := NewEvalContext(
		lang.LanguageVersionV3,
		ProtoVersion{Major: 10},
		params,
	)
	if err != nil {
		t.Fatalf("NewEvalContext: %v", err)
	}

	term := mustEvalContextReuseTerm(t, buildBuiltinHeavyProgram(64))

	const goroutines = 32
	const iterations = 25

	var wg sync.WaitGroup
	results := make([]int64, goroutines)
	budgets := make([]ExBudget, goroutines)
	errCh := make(chan error, goroutines)

	for g := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			var lastResult int64
			var lastBudget ExBudget
			for iter := range iterations {
				// A fresh Machine per iteration, all sharing sharedCtx
				// concurrently with every other goroutine's Machine.
				machine := NewMachine[syn.DeBruijn](
					lang.LanguageVersionV3,
					200,
					sharedCtx,
				)
				out, err := machine.Run(term)
				if err != nil {
					errCh <- fmt.Errorf(
						"goroutine %d iteration %d: Run: %w",
						idx,
						iter,
						err,
					)
					return
				}
				result, err := evalContextReuseResultInt64(out)
				if err != nil {
					errCh <- fmt.Errorf(
						"goroutine %d iteration %d: %w",
						idx,
						iter,
						err,
					)
					return
				}
				lastResult = result
				lastBudget = machine.ExBudget
			}
			results[idx] = lastResult
			budgets[idx] = lastBudget
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}

	for i := range results {
		if results[i] != results[0] || budgets[i] != budgets[0] {
			t.Errorf(
				"goroutine %d observed result=%d budget=%+v, want result=%d budget=%+v (shared EvalContext reuse diverged)",
				i,
				results[i],
				budgets[i],
				results[0],
				budgets[0],
			)
		}
	}
}

func mustEvalContextReuseTerm(
	t *testing.T,
	program *syn.Program[syn.Name],
) syn.Term[syn.DeBruijn] {
	t.Helper()

	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn: %v", err)
	}
	return dbProgram.Term
}

func evalContextReuseResultInt64(term syn.Term[syn.DeBruijn]) (int64, error) {
	constant, ok := term.(*syn.Constant)
	if !ok {
		return 0, fmt.Errorf("Run result is %T, want *syn.Constant", term)
	}
	integer, ok := constant.Con.(*syn.Integer)
	if !ok {
		return 0, fmt.Errorf(
			"Run result constant is %T, want *syn.Integer",
			constant.Con,
		)
	}
	return integer.Inner.Int64(), nil
}
