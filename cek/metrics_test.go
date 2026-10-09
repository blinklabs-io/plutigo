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
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func metricsMachine(
	t *testing.T,
	src string,
	slippage uint32,
) (*Machine[syn.DeBruijn], syn.Term[syn.DeBruijn]) {
	t.Helper()
	program, err := syn.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	db, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn: %v", err)
	}
	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		slippage,
		NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: 11}),
	)
	return machine, db.Term
}

const threeAdds = `(program 1.1.0
	[ [ (builtin addInteger)
	    [ [ (builtin addInteger) (con integer 1) ] (con integer 2) ] ]
	  [ [ (builtin addInteger) (con integer 3) ] (con integer 4) ] ])`

func TestMetricsAreOptIn(t *testing.T) {
	t.Parallel()

	machine, term := metricsMachine(t, threeAdds, 0)
	if _, err := machine.Run(term); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if machine.Metrics() != nil {
		t.Fatal("Metrics() is non-nil without EnableMetrics")
	}
}

// TestMetricsMatchConsumedBudget checks the reported counts against the
// budget the machine actually consumed: the startup charge, every counted step
// at its own cost and every builtin's reported budget must add up to it.
func TestMetricsMatchConsumedBudget(t *testing.T) {
	t.Parallel()

	for _, slippage := range []uint32{0, 200} {
		machine, term := metricsMachine(t, threeAdds, slippage)
		machine.EnableMetrics()
		initial := ExBudget{Cpu: 1_000_000_000, Mem: 1_000_000}
		machine.ExBudget = initial
		if _, err := machine.Run(term); err != nil {
			t.Fatalf("slippage %d: Run: %v", slippage, err)
		}
		metrics := machine.Metrics()

		wantSteps := map[StepKind]uint64{ExApply: 6, ExBuiltin: 3, ExConstant: 4}
		for kind := range StepKind(numStepKinds) {
			if metrics.Steps[kind] != wantSteps[kind] {
				t.Errorf(
					"slippage %d: steps[%d] = %d, want %d",
					slippage, kind, metrics.Steps[kind], wantSteps[kind],
				)
			}
		}
		add := metrics.Builtins[builtin.AddInteger]
		if add.Calls != 3 {
			t.Errorf("slippage %d: addInteger calls = %d, want 3", slippage, add.Calls)
		}
		for fn, bm := range metrics.Builtins {
			if builtin.DefaultFunction(fn) != builtin.AddInteger && bm != (BuiltinMetrics{}) {
				t.Errorf("slippage %d: %s reported %+v", slippage, builtin.DefaultFunction(fn), bm)
			}
		}

		costs := machine.costs.machineCosts
		want := costs.startup
		for kind := range StepKind(numStepKinds) {
			step := costs.get(kind)
			want.Cpu += step.Cpu * int64(metrics.Steps[kind])
			want.Mem += step.Mem * int64(metrics.Steps[kind])
		}
		want.Cpu += add.Budget.Cpu
		want.Mem += add.Budget.Mem
		if consumed := initial.Sub(&machine.ExBudget); consumed != want {
			t.Errorf("slippage %d: consumed %+v, metrics account for %+v", slippage, consumed, want)
		}
		if add.Budget.Cpu <= 0 || add.Budget.Mem <= 0 {
			t.Errorf("slippage %d: addInteger budget = %+v, want positive", slippage, add.Budget)
		}
	}
}

// TestMetricsAccountForConsumedBudgetAcrossPrograms applies the same budget
// identity to programs that use lambdas, delays, forces, constructors, cases
// and builtins of every arity, with and without batched step charging.
func TestMetricsAccountForConsumedBudgetAcrossPrograms(t *testing.T) {
	t.Parallel()

	programs := map[string]string{
		"lambda and force": `(program 1.1.0
			[ (lam x (force (delay x))) (con integer 7) ])`,
		"unary builtin": `(program 1.1.0
			[ (builtin lengthOfByteString) (con bytestring #0102) ])`,
		"ternary builtin": `(program 1.1.0
			[ (force (builtin ifThenElse)) (con bool True)
			  (con integer 1) (con integer 2) ])`,
		"constructor and case": `(program 1.1.0
			(case (constr 1 (con integer 5))
			  (con integer 0)
			  (lam x [ [ (builtin addInteger) x ] (con integer 1) ])))`,
		"hash builtin": `(program 1.1.0
			[ (builtin sha2_256) (con bytestring #00) ])`,
	}
	for name, src := range programs {
		for _, slippage := range []uint32{0, 200} {
			machine, term := metricsMachine(t, src, slippage)
			machine.EnableMetrics()
			initial := ExBudget{Cpu: 1_000_000_000, Mem: 1_000_000}
			machine.ExBudget = initial
			if _, err := machine.Run(term); err != nil {
				t.Fatalf("%s (slippage %d): Run: %v", name, slippage, err)
			}
			metrics := machine.Metrics()

			costs := machine.costs.machineCosts
			want := costs.startup
			for kind := range StepKind(numStepKinds) {
				step := costs.get(kind)
				want.Cpu += step.Cpu * int64(metrics.Steps[kind])
				want.Mem += step.Mem * int64(metrics.Steps[kind])
			}
			builtinCalls := uint64(0)
			for _, bm := range metrics.Builtins {
				want.Cpu += bm.Budget.Cpu
				want.Mem += bm.Budget.Mem
				builtinCalls += bm.Calls
			}
			if consumed := initial.Sub(&machine.ExBudget); consumed != want {
				t.Errorf("%s (slippage %d): consumed %+v, metrics account for %+v", name, slippage, consumed, want)
			}
			if builtinCalls != metrics.Steps[ExBuiltin] {
				t.Errorf(
					"%s (slippage %d): %d builtin calls for %d builtin steps",
					name, slippage, builtinCalls, metrics.Steps[ExBuiltin],
				)
			}
		}
	}
}

func TestMetricsResetOnEachRun(t *testing.T) {
	t.Parallel()

	machine, term := metricsMachine(t, threeAdds, 0)
	machine.EnableMetrics()
	for range 2 {
		machine.ExBudget = ExBudget{Cpu: 1_000_000_000, Mem: 1_000_000}
		if _, err := machine.Run(term); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if got := machine.Metrics().Builtins[builtin.AddInteger].Calls; got != 3 {
			t.Fatalf("addInteger calls = %d, want 3 on every run", got)
		}
	}
}

func TestMetricsMaxStackDepthGrowsWithNesting(t *testing.T) {
	t.Parallel()

	// Each extra level wraps the argument in one more pending application, so
	// every level adds exactly one frame to the deepest point of the run.
	depthOf := func(levels int) int {
		src := "(program 1.1.0 " +
			strings.Repeat("[ (builtin addInteger) ", levels) +
			"(con integer 1)" +
			strings.Repeat(" ]", levels) + ")"
		machine, term := metricsMachine(t, src, 0)
		machine.EnableMetrics()
		if _, err := machine.Run(term); err != nil {
			t.Fatalf("Run: %v", err)
		}
		return machine.Metrics().MaxStackDepth
	}
	base := depthOf(2)
	if got := depthOf(6); got != base+4 {
		t.Fatalf("max stack depth: 6 levels = %d, 2 levels = %d; want a difference of 4", got, base)
	}
	if base <= 0 {
		t.Fatalf("max stack depth = %d, want positive", base)
	}
}
