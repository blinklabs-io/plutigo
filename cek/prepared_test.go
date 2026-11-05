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
	"math/big"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// TestPreparedNodeGroundTruth replays the node ground-truth scripts through
// PrepareProgram and RunPreparedScriptContext. The outcome and ExUnits must
// equal the node's, and the remaining budget must equal RunScriptContext's
// for the same program.
func TestPreparedNodeGroundTruth(t *testing.T) {
	gt := loadNodeFixture[nodeGroundTruth](t, "node_ground_truth_pv11.json.gz")
	for _, sc := range gt.Scripts {
		t.Run(sc.ID, func(t *testing.T) {
			lv, ln := groundTruthLang(t, sc.Lang)
			flat, err := hex.DecodeString(sc.Flat)
			if err != nil {
				t.Fatal(err)
			}
			pc := syn.ProgramContext{
				LedgerLanguage: lv,
				ProtocolMajor:  gt.Protocol,
			}
			got, ex := "ok", cek.ExBudget{}
			prepared, err := cek.PrepareProgram(flat, pc)
			switch {
			case err != nil:
				got = "phase1-reject"
			case syn.ValidateTermVersionForExecution(prepared.Version(), pc) != nil:
				got = "phase2-fail"
			default:
				ec, err := cek.NewEvalContext(
					lv,
					cek.ProtoVersion{Major: gt.Protocol},
					gt.CostModels[ln],
				)
				if err != nil {
					t.Fatal(err)
				}
				budget := nodeTxBudget
				if sc.Budget != nil {
					budget = sc.Budget.budget()
				}
				args := ledgerArgs(sc.Lang, sc.Purpose)
				m := cek.NewMachine[syn.DeBruijn](lv, 0, ec)
				m.ExBudget = budget
				isUnit, runErr := cek.RunPreparedScriptContext(
					context.Background(),
					m,
					prepared,
					args...)
				ex = budget.Sub(&m.ExBudget)
				if runErr != nil || (sc.Lang == "V3" && !isUnit) {
					got = "phase2-fail"
				}

				prog, err := syn.DecodeDeBruijnWithContext(flat, pc)
				if err != nil {
					t.Fatal(err)
				}
				ref := cek.NewMachine[syn.DeBruijn](lv, 0, ec)
				ref.ExBudget = budget
				refUnit, refErr := ref.RunScriptContext(
					context.Background(),
					prog.Term,
					args...)
				if refUnit != isUnit || (refErr == nil) != (runErr == nil) ||
					ref.ExBudget != m.ExBudget {
					t.Fatalf(
						"prepared run (unit %t, err %v, budget %+v) differs from RunScriptContext (unit %t, err %v, budget %+v)",
						isUnit,
						runErr,
						m.ExBudget,
						refUnit,
						refErr,
						ref.ExBudget,
					)
				}
				if refErr != nil && runErr.Error() != refErr.Error() {
					t.Fatalf(
						"prepared error %q differs from RunScriptContext error %q",
						runErr,
						refErr,
					)
				}
			}
			if got != sc.Outcome {
				t.Fatalf("outcome %s, node %s", got, sc.Outcome)
			}
			if sc.Outcome == "ok" && sc.ExUnits != nil &&
				ex != sc.ExUnits.budget() {
				t.Fatalf("exunits mem=%d steps=%d, node mem=%d steps=%d",
					ex.Mem, ex.Cpu, sc.ExUnits.Mem, sc.ExUnits.Steps)
			}
		})
	}
}

// TestPreparedNodeBudgetFailures requires the node's outcome and, after an
// out-of-budget failure, its exact remaining budget, for a prepared program
// run twice on each machine configuration.
func TestPreparedNodeBudgetFailures(t *testing.T) {
	fx := loadNodeFixture[nodeBudgetFailures](
		t,
		"node_budget_failures_pv11.json.gz",
	)
	for _, sc := range fx.Cases {
		t.Run(sc.ID, func(t *testing.T) {
			lv, ln := groundTruthLang(t, sc.Lang)
			flat, err := hex.DecodeString(sc.Flat)
			if err != nil {
				t.Fatal(err)
			}
			pc := syn.ProgramContext{
				LedgerLanguage: lv,
				ProtocolMajor:  fx.Protocol,
			}
			prepared, err := cek.PrepareProgram(flat, pc)
			if err != nil {
				t.Fatal(err)
			}
			ec, err := cek.NewEvalContext(
				lv,
				cek.ProtoVersion{Major: fx.Protocol},
				fx.CostModels[ln],
			)
			if err != nil {
				t.Fatal(err)
			}
			args := ledgerArgs(sc.Lang, sc.Purpose)
			budget := sc.Budget.budget()
			for _, slippage := range []uint32{cek.DefaultSlippage, 0, 1} {
				m := cek.NewMachine[syn.DeBruijn](lv, slippage, ec)
				for run := range 2 {
					m.ExBudget = budget
					isUnit, err := cek.RunPreparedScriptContext(
						context.Background(),
						m,
						prepared,
						args...)
					checkNodeBudgetCase(
						t,
						fmt.Sprintf("slippage %d run %d", slippage, run),
						&sc,
						isUnit,
						err,
						m,
					)
				}
			}
		})
	}
}

// openCaseBranchProgram returns a UPLC 1.1.0 program whose only free variable
// sits in a case branch: NoCaseConstr scope mode (before protocol version 12)
// accepts it, Full mode rejects it. The scrutinee selects branch 0, so the
// free variable is never evaluated.
func openCaseBranchProgram(t *testing.T) []byte {
	t.Helper()
	unit := &syn.Constant{Con: &syn.Unit{}}
	program := &syn.Program[syn.DeBruijn]{
		Version: lang.LanguageVersion{1, 1, 0},
		Term: &syn.Case[syn.DeBruijn]{
			Constr: &syn.Constr[syn.DeBruijn]{Tag: 0},
			Branches: []syn.Term[syn.DeBruijn]{
				unit,
				&syn.Var[syn.DeBruijn]{Name: 7},
			},
		},
	}
	flat, err := syn.Encode(program)
	if err != nil {
		t.Fatal(err)
	}
	return flat
}

// TestPreparedScopeModes checks that a program prepared in one scope mode is
// run with the walk when the machine needs a stricter mode, so it fails or
// succeeds exactly as RunScriptContext does.
func TestPreparedScopeModes(t *testing.T) {
	flat := openCaseBranchProgram(t)
	for _, tc := range []struct {
		name         string
		prepareMajor uint
		machineMajor uint
	}{
		{"prepare PV11, run PV11", 11, 11},
		{"prepare PV11, run PV12", 11, 12},
		{"prepare PV12, run PV11", 12, 11},
		{"prepare PV12, run PV12", 12, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pc := syn.ProgramContext{
				LedgerLanguage: lang.LanguageVersionV3,
				ProtocolMajor:  tc.prepareMajor,
			}
			prepared, err := cek.PrepareProgram(flat, pc)
			if err != nil {
				t.Fatal(err)
			}
			prog, err := syn.DecodeDeBruijnWithContext(flat, pc)
			if err != nil {
				t.Fatal(err)
			}
			ec := cek.NewDefaultEvalContext(
				lang.LanguageVersionV3,
				cek.ProtoVersion{Major: tc.machineMajor},
			)
			budget := cek.ExBudget{Mem: 1_000_000, Cpu: 1_000_000_000}

			m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
			m.ExBudget = budget
			gotUnit, gotErr := cek.RunPreparedScriptContext(
				context.Background(),
				m,
				prepared,
			)

			ref := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
			ref.ExBudget = budget
			wantUnit, wantErr := ref.RunScriptContext(
				context.Background(),
				prog.Term,
			)

			wantFail := tc.machineMajor >= 12
			if (wantErr != nil) != wantFail {
				t.Fatalf(
					"RunScriptContext error %v, want failure %t",
					wantErr,
					wantFail,
				)
			}
			if gotUnit != wantUnit || (gotErr == nil) != (wantErr == nil) ||
				m.ExBudget != ref.ExBudget {
				t.Fatalf(
					"prepared (unit %t, err %v, budget %+v), RunScriptContext (unit %t, err %v, budget %+v)",
					gotUnit,
					gotErr,
					m.ExBudget,
					wantUnit,
					wantErr,
					ref.ExBudget,
				)
			}
			if wantErr != nil {
				var typeErr *cek.TypeError
				if !errors.As(gotErr, &typeErr) ||
					typeErr.Code != cek.ErrCodeOpenTerm {
					t.Fatalf(
						"prepared error %v, want open-term TypeError",
						gotErr,
					)
				}
			}
		})
	}
}

// TestPreparedConcurrentUse runs one prepared program on many machines at
// once; with -race it checks that evaluation never writes to the shared term.
func TestPreparedConcurrentUse(t *testing.T) {
	gt := loadNodeFixture[nodeGroundTruth](t, "node_ground_truth_pv11.json.gz")
	var flat []byte
	sc := gt.Scripts[0]
	for _, s := range gt.Scripts {
		if s.Outcome == "ok" && s.Lang == "V3" {
			sc = s
			break
		}
	}
	flat, err := hex.DecodeString(sc.Flat)
	if err != nil {
		t.Fatal(err)
	}
	lv, ln := groundTruthLang(t, sc.Lang)
	pc := syn.ProgramContext{LedgerLanguage: lv, ProtocolMajor: gt.Protocol}
	prepared, err := cek.PrepareProgram(flat, pc)
	if err != nil {
		t.Fatal(err)
	}
	ec, err := cek.NewEvalContext(
		lv,
		cek.ProtoVersion{Major: gt.Protocol},
		gt.CostModels[ln],
	)
	if err != nil {
		t.Fatal(err)
	}
	args := ledgerArgs(sc.Lang, sc.Purpose)
	var wg sync.WaitGroup
	results := make([]cek.ExBudget, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := cek.NewMachine[syn.DeBruijn](lv, 0, ec)
			for range 20 {
				m.ExBudget = nodeTxBudget
				if _, err := cek.RunPreparedScriptContext(context.Background(), m, prepared, args...); err != nil {
					t.Error(err)
					return
				}
			}
			results[i] = m.ExBudget
		}()
	}
	wg.Wait()
	for i := range results {
		if results[i] != results[0] {
			t.Fatalf(
				"goroutine %d remaining budget %+v, goroutine 0 %+v",
				i,
				results[i],
				results[0],
			)
		}
	}
}

func TestPreparedRequiredArguments(t *testing.T) {
	ec := cek.NewDefaultEvalContext(
		lang.LanguageVersionV3,
		cek.ProtoVersion{Major: 11},
	)
	m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
	if _, err := cek.RunPreparedScriptContext(context.Background(), nil, &cek.PreparedProgram{}); err == nil {
		t.Fatal("nil machine accepted")
	}
	if _, err := cek.RunPreparedScriptContext(context.Background(), m, nil); err == nil {
		t.Fatal("nil program accepted")
	}
	pc := syn.ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  11,
	}
	prepared, err := cek.PrepareProgram(openCaseBranchProgram(t), pc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cek.RunPreparedScriptContext(context.Background(), m, prepared, data.PlutusData(nil)); err == nil {
		t.Fatal("nil trusted data argument accepted")
	}
	if _, err := cek.PrepareProgram([]byte{0xff}, pc); err == nil {
		t.Fatal("malformed FLAT accepted")
	}
}

func TestPrepareDecodedProgramRequiresProgram(t *testing.T) {
	pc := syn.ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  11,
	}
	if _, err := cek.PrepareDecodedProgram(nil, pc); err == nil {
		t.Fatal("nil program accepted")
	}
	if _, err := cek.PrepareDecodedProgram(&syn.Program[syn.DeBruijn]{}, pc); err == nil {
		t.Fatal("program without a term accepted")
	}
}

// TestPreparedArgumentChainLimits checks that the application chain built for
// the trusted data arguments counts against the structure depth and work
// limits on the prepared path as it does in RunScriptContext: a program that
// passes the walk alone but not once the arguments are applied must fail with
// the same error and budget.
func TestPreparedArgumentChainLimits(t *testing.T) {
	deep := func() syn.Term[syn.DeBruijn] {
		var term syn.Term[syn.DeBruijn] = &syn.Error{}
		for range 100_000 {
			term = &syn.Delay[syn.DeBruijn]{Term: term}
		}
		return term
	}
	wide := func() syn.Term[syn.DeBruijn] {
		fields := make([]syn.Term[syn.DeBruijn], 999_999)
		for i := range fields {
			fields[i] = &syn.Error{}
		}
		return &syn.Constr[syn.DeBruijn]{Fields: fields}
	}
	for _, tc := range []struct {
		name string
		term func() syn.Term[syn.DeBruijn]
	}{
		{"depth", deep},
		{"work", wide},
	} {
		t.Run(tc.name, func(t *testing.T) {
			term := tc.term()
			pc := syn.ProgramContext{
				LedgerLanguage: lang.LanguageVersionV3,
				ProtocolMajor:  11,
			}
			prepared, err := cek.PrepareDecodedProgram(
				&syn.Program[syn.DeBruijn]{
					Version: lang.LanguageVersion{1, 1, 0},
					Term:    term,
				},
				pc,
			)
			if err != nil {
				t.Fatal(err)
			}
			ec := cek.NewDefaultEvalContext(
				lang.LanguageVersionV3,
				cek.ProtoVersion{Major: 11},
			)
			budget := cek.ExBudget{Mem: 1_000_000, Cpu: 1_000_000_000}
			args := []data.PlutusData{data.NewInteger(big.NewInt(1))}

			ref := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
			ref.ExBudget = budget
			_, wantErr := ref.RunScriptContext(
				context.Background(),
				term,
				args...)
			if wantErr == nil {
				t.Fatal(
					"RunScriptContext accepted a term over the structure limit",
				)
			}

			m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
			m.ExBudget = budget
			_, gotErr := cek.RunPreparedScriptContext(
				context.Background(),
				m,
				prepared,
				args...)
			if gotErr == nil || gotErr.Error() != wantErr.Error() ||
				m.ExBudget != ref.ExBudget {
				t.Fatalf(
					"prepared (err %v, budget %+v), RunScriptContext (err %v, budget %+v)",
					gotErr,
					m.ExBudget,
					wantErr,
					ref.ExBudget,
				)
			}
		})
	}
}
