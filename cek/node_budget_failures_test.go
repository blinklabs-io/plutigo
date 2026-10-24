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
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

// nodeBudgetFailures records scripts run on a real cardano-node with an
// explicit budget (see the fixture's provenance): whether the ledger
// accepted them, and for an out-of-budget failure the remaining budget the
// ledger reported.
type nodeBudgetFailures struct {
	Provenance map[string]string  `json:"provenance"`
	CostModels map[string][]int64 `json:"costModels"`
	Protocol   uint               `json:"protocolMajor"`
	Cases      []nodeBudgetCase   `json:"cases"`
}

type nodeBudgetCase struct {
	ID      string      `json:"id"`
	Lang    string      `json:"lang"`
	Purpose string      `json:"purpose"`
	Flat    string      `json:"flat"`
	Budget  nodeExUnits `json:"budget"`
	// Outcome is ok, budget (out of budget) or error (another
	// evaluation failure).
	Outcome string `json:"outcome"`
	// Remaining is the ledger's "budget when the machine terminated"
	// for a budget outcome; negative values are the overspend.
	Remaining *nodeExUnits `json:"remaining,omitempty"`
	// EvaluationError is the ledger's text for an error outcome.
	EvaluationError string `json:"evaluationError,omitempty"`
}

// TestNodeBudgetFailures replays every recorded case with the recorded
// budget and requires the ledger's outcome and, after an out-of-budget
// failure, exactly the remaining budget the ledger reported, for every way a
// ledger caller can configure the machine: the ledger's slippage, the
// "unset" slippages 0 and 1 (which select it), and a metered machine (which
// runs the generic loop instead of the De Bruijn one).
func TestNodeBudgetFailures(t *testing.T) {
	fx := loadNodeFixture[nodeBudgetFailures](t, "node_budget_failures_pv11.json.gz")
	if len(fx.Cases) < 100 {
		t.Fatalf("fixture has %d cases, want at least 100", len(fx.Cases))
	}
	type config struct {
		name     string
		slippage uint32
		metered  bool
	}
	configs := []config{
		{"slippage200", cek.DefaultSlippage, false},
		{"slippage0", 0, false},
		{"slippage1", 1, false},
		{"metered", 0, true},
	}
	for _, sc := range fx.Cases {
		t.Run(sc.ID, func(t *testing.T) {
			lv, ln := groundTruthLang(t, sc.Lang)
			flat, err := hex.DecodeString(sc.Flat)
			if err != nil {
				t.Fatal(err)
			}
			pc := syn.ProgramContext{LedgerLanguage: lv, ProtocolMajor: fx.Protocol}
			prog, err := syn.DecodeDeBruijnWithContext(flat, pc)
			if err != nil {
				t.Fatal(err)
			}
			ec, err := cek.NewEvalContext(lv, cek.ProtoVersion{Major: fx.Protocol}, fx.CostModels[ln])
			if err != nil {
				t.Fatal(err)
			}
			args := ledgerArgs(sc.Lang, sc.Purpose)
			budget := sc.Budget.budget()
			for _, cfg := range configs {
				m := cek.NewMachine[syn.DeBruijn](lv, cfg.slippage, ec)
				if cfg.metered {
					m.EnableMetrics()
				}
				// Run twice on the same machine: a reused machine must
				// report the same failure.
				for run := range 2 {
					m.ExBudget = budget
					isUnit, err := m.RunScriptContext(context.Background(), prog.Term, args...)
					where := fmt.Sprintf("%s run %d", cfg.name, run)
					checkNodeBudgetCase(t, where, &sc, isUnit, err, m)
				}
			}
		})
	}
}

func checkNodeBudgetCase(
	t *testing.T,
	where string,
	sc *nodeBudgetCase,
	isUnit bool,
	err error,
	m *cek.Machine[syn.DeBruijn],
) {
	t.Helper()
	var budgetErr *cek.BudgetError
	isBudget := errors.As(err, &budgetErr)
	switch sc.Outcome {
	case "ok":
		if err != nil || (sc.Lang == "V3" && !isUnit) {
			t.Fatalf("%s: got error %v (unit %t), node accepted", where, err, isUnit)
		}
	case "budget":
		if sc.Remaining == nil {
			t.Fatalf("fixture budget outcome has no remaining budget")
			return
		}
		if !isBudget || budgetErr == nil || budgetErr.Code != cek.ErrCodeBudgetExhausted {
			t.Fatalf("%s: got %v, node ran out of budget", where, err)
			return
		}
		want := sc.Remaining.budget()
		if m.ExBudget != want {
			t.Fatalf("%s: remaining budget mem=%d cpu=%d, node mem=%d cpu=%d",
				where, m.ExBudget.Mem, m.ExBudget.Cpu, want.Mem, want.Cpu)
		}
		if got := budgetErr.Available.Sub(&budgetErr.Requested); got != want {
			t.Fatalf("%s: BudgetError available-requested = %+v, want remaining %+v",
				where, got, want)
		}
	case "error":
		if err == nil || isBudget {
			t.Fatalf("%s: got %v, node failed with %q", where, err, sc.EvaluationError)
		}
		var scriptErr *cek.ScriptError
		var builtinErr *cek.BuiltinError
		// The ledger names the failing term after "Caused by: ": error
		// for an explicit error, the builtin application otherwise.
		_, cause, ok := strings.Cut(sc.EvaluationError, "Caused by: ")
		if !ok {
			t.Fatalf("fixture error %q names no cause", sc.EvaluationError)
		}
		if strings.HasPrefix(cause, "error") {
			if !errors.As(err, &scriptErr) {
				t.Fatalf("%s: got %T %v, node failed with an explicit error", where, err, err)
			}
		} else if !errors.As(err, &builtinErr) {
			t.Fatalf("%s: got %T %v, node failed in a builtin: %s", where, err, err, cause)
		}
	default:
		t.Fatalf("unknown outcome %q", sc.Outcome)
	}
}
