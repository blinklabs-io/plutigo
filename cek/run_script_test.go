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

package cek_test

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// exponentialClosureScript returns a two-argument (V1/V2 minting shaped)
// script whose result is a closure that captures a chain of n closures, each
// referring to the previous one twice: cheap to build (O(n) steps) but
// 2^n nodes once discharged into a term.
func exponentialClosureScript(t *testing.T, n int, args int) syn.Term[syn.DeBruijn] {
	t.Helper()
	body := "(lam y y)"
	for range n {
		body = "[(lam x (lam y [x x])) " + body + "]"
	}
	return scriptTakingArgs(t, body, args)
}

func unitScript(t *testing.T, args int) syn.Term[syn.DeBruijn] {
	t.Helper()
	return scriptTakingArgs(t, "(con unit ())", args)
}

// scriptTakingArgs parses body wrapped in args lambdas that ignore their
// arguments.
func scriptTakingArgs(t *testing.T, body string, args int) syn.Term[syn.DeBruijn] {
	t.Helper()
	for i := range args {
		body = "(lam arg" + string(rune('a'+i)) + " " + body + ")"
	}
	program, err := syn.Parse("(program 1.0.0 " + body + ")")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	db, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("debruijn: %v", err)
	}
	return db.Term
}

// TestRunScriptContextDoesNotDischarge is the regression for a consensus
// divergence: the reference ledger never discharges a script's result (it is
// lazy and only matched against unit), so a Plutus V1/V2 script whose result
// is a closure too large to discharge succeeds. Run fails on it with
// ErrCodeResultTooLarge; RunScriptContext must succeed with the same budget.
func TestRunScriptContextDoesNotDischarge(t *testing.T) {
	ctxArgs := []data.PlutusData{
		data.NewInteger(big.NewInt(0)),
		data.NewConstr(0),
	}
	for _, language := range []lang.LanguageVersion{
		lang.LanguageVersionV1,
		lang.LanguageVersionV2,
		lang.LanguageVersionV3,
	} {
		evalCtx := cek.NewDefaultEvalContext(language, cek.ProtoVersion{Major: 10})
		term := exponentialClosureScript(t, 40, len(ctxArgs))

		m := cek.NewMachine[syn.DeBruijn](language, 0, evalCtx)
		start := m.ExBudget
		isUnit, err := m.RunScriptContext(context.Background(), term, ctxArgs...)
		if err != nil {
			t.Fatalf("%v: RunScriptContext: %v", language, err)
		}
		if isUnit {
			t.Fatalf("%v: closure result reported as unit", language)
		}
		consumed := start.Sub(&m.ExBudget)

		// Run evaluates identically, then fails to discharge the result.
		m2 := cek.NewMachine[syn.DeBruijn](language, 0, evalCtx)
		_, err = m2.RunContextWithTrustedDataArguments(context.Background(), term, ctxArgs...)
		var budgetErr *cek.BudgetError
		if !errors.As(err, &budgetErr) || budgetErr.Code != cek.ErrCodeResultTooLarge {
			t.Fatalf("%v: Run error = %v, want ErrCodeResultTooLarge", language, err)
		}
		if got := start.Sub(&m2.ExBudget); got != consumed {
			t.Fatalf("%v: Run consumed %+v, RunScriptContext %+v", language, got, consumed)
		}

		// A small chain discharges: both methods agree on the budget.
		small := exponentialClosureScript(t, 3, len(ctxArgs))
		m3 := cek.NewMachine[syn.DeBruijn](language, 0, evalCtx)
		if _, err := m3.RunScriptContext(context.Background(), small, ctxArgs...); err != nil {
			t.Fatalf("%v: small RunScriptContext: %v", language, err)
		}
		m4 := cek.NewMachine[syn.DeBruijn](language, 0, evalCtx)
		result, err := m4.RunContextWithTrustedDataArguments(context.Background(), small, ctxArgs...)
		if err != nil {
			t.Fatalf("%v: small Run: %v", language, err)
		}
		// The returned closure plus 2, 4 and 8 discharged copies below it.
		if got := strings.Count(syn.PrettyTerm[syn.DeBruijn](result), "(lam"); got != 1+2+4+8 {
			t.Fatalf("%v: small result has %d lambdas", language, got)
		}
		if m3.ExBudget != m4.ExBudget {
			t.Fatalf("%v: budgets differ: %+v vs %+v", language, m3.ExBudget, m4.ExBudget)
		}
	}
}

func TestRunScriptContextReportsUnit(t *testing.T) {
	evalCtx := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 10})
	arg := data.NewConstr(0)
	m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, evalCtx)
	isUnit, err := m.RunScriptContext(context.Background(), unitScript(t, 1), arg)
	if err != nil || !isUnit {
		t.Fatalf("unit script: isUnit=%v err=%v", isUnit, err)
	}
	// The machine is reusable and Run discharges again afterwards.
	result, err := m.RunContextWithTrustedDataArguments(context.Background(), unitScript(t, 1), arg)
	if err != nil {
		t.Fatalf("Run after RunScriptContext: %v", err)
	}
	if got := syn.PrettyTerm[syn.DeBruijn](result); got != "(con unit ())" {
		t.Fatalf("Run result %s", got)
	}
	// Errors are reported as by Run.
	m2 := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, evalCtx)
	m2.ExBudget = cek.ExBudget{Cpu: 1, Mem: 1}
	if _, err := m2.RunScriptContext(context.Background(), unitScript(t, 1), arg); err == nil {
		t.Fatal("RunScriptContext succeeded without budget")
	}
}
