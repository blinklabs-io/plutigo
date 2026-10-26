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
	"fmt"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// TestStepBatchingLoopsAgree checks that the De Bruijn loop, which unmetered
// machines run, charges steps exactly as the generic loop, which metered
// machines run: same outcome, error, logs and remaining budget for every
// slippage, at budgets around and below what each program consumes.
func TestStepBatchingLoopsAgree(t *testing.T) {
	programs := []string{
		// Countdown loop that traces and adds on every iteration.
		`(program 1.1.0 [[(lam f [f f]) (lam self (lam n (force [[[(force (builtin ifThenElse)) [[(builtin lessThanEqualsInteger) n] (con integer 0)]] (delay (con integer 0))] (delay [[(force (builtin trace)) (con string "i")] [[(builtin addInteger) n] [[self self] [[(builtin subtractInteger) n] (con integer 1)]]]])])))] (con integer 120)])`,
		// Data-heavy builtins whose costs are streamed on overdraw.
		`(program 1.1.0 [(lam d [[(builtin equalsData) d] d]) (con data (Constr 0 [List [I 1, I 2, B #0102], Map [(I 3, B #04)], I 100000000000000000000]))])`,
		// Case and constr steps around a failing script.
		`(program 1.1.0 (case (constr 0 (con integer 1) (con integer 2)) (lam a (lam b [[(force (builtin trace)) (con string "x")] (error)]))))`,
		// Endless self-application.
		`(program 1.1.0 [(lam x [x x]) (lam x [x x])])`,
	}
	for pi, src := range programs {
		prog := parseDeBruijn(t, src)
		ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 11})
		for _, slippage := range []uint32{0, 1, 2, 3, 7, cek.DefaultSlippage, 1000} {
			run := func(metered bool, budget cek.ExBudget) string {
				m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, slippage, ec)
				if metered {
					m.EnableMetrics()
				}
				m.ExBudget = budget
				_, err := m.Run(prog.Term)
				return fmt.Sprintf("%v | %+v | %s", err, m.ExBudget, strings.Join(m.Logs, ","))
			}
			full := cek.ExBudget{Mem: 1_000_000, Cpu: 1_000_000_000}
			m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, slippage, ec)
			m.ExBudget = full
			_, _ = m.Run(prog.Term)
			used := full.Sub(&m.ExBudget)
			var budgets []cek.ExBudget
			for _, d := range []int64{0, 1, 2, 50, 99, 1000, 10_000, 100_000} {
				budgets = append(budgets,
					cek.ExBudget{Mem: used.Mem, Cpu: used.Cpu - d},
					cek.ExBudget{Mem: used.Mem - d, Cpu: used.Cpu},
				)
			}
			for i := int64(1); i < 16; i++ {
				budgets = append(budgets, cek.ExBudget{Mem: used.Mem * i / 16, Cpu: used.Cpu * i / 16})
			}
			budgets = append(budgets, full)
			for _, budget := range budgets {
				if budget.Mem < 0 || budget.Cpu < 0 {
					continue
				}
				got, want := run(false, budget), run(true, budget)
				if got != want {
					t.Fatalf("program %d slippage %d budget %+v:\n De Bruijn: %s\n generic:   %s",
						pi, slippage, budget, got, want)
				}
			}
		}
	}
}
