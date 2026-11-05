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
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func pooledTestProgram(t testing.TB, name string) *PreparedProgram {
	t.Helper()
	raw, err := os.ReadFile("../tests/bench/" + name + ".flat")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareProgram(raw, syn.ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  11,
	})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func pooledTestMachine() *Machine[syn.DeBruijn] {
	return NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		200,
		NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: 11}),
	)
}

// TestSetExBudgetToPreviousRemaining checks that SetExBudget gives the next
// run exactly the budget it sets, also when that budget is the previous run's
// remaining budget, which an assignment to ExBudget cannot express.
func TestSetExBudgetToPreviousRemaining(t *testing.T) {
	program := pooledTestProgram(t, "guardrail-sorted-small")
	run := func(m *Machine[syn.DeBruijn]) {
		t.Helper()
		if _, err := RunPreparedScriptContext(context.Background(), m, program); err != nil {
			t.Fatal(err)
		}
	}
	start := ExBudget{Cpu: 10_000_000_000, Mem: 14_000_000}
	m := pooledTestMachine()
	m.ExBudget = start
	run(m)
	used := start.Sub(&m.ExBudget)
	remaining := m.ExBudget

	m.SetExBudget(remaining)
	run(m)
	if want := remaining.Sub(&used); m.ExBudget != want {
		t.Errorf("after SetExBudget(remaining): budget %+v, want %+v", m.ExBudget, want)
	}

	// Leaving ExBudget as the run left it restores the initial budget.
	run(m)
	if want := remaining.Sub(&used); m.ExBudget != want {
		t.Errorf("after an unchanged budget: budget %+v, want %+v", m.ExBudget, want)
	}
}

// TestPooledMachineKeepsArenasAcrossScripts runs a small and a large script
// in turn on one machine, as a pool does, and checks that after warming up the
// runs no longer allocate their arenas again.
func TestPooledMachineKeepsArenasAcrossScripts(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts differ under the race detector")
	}
	small := pooledTestProgram(t, "guardrail-sorted-small")
	large := pooledTestProgram(t, "stablecoin_1-5")
	m := pooledTestMachine()
	run := func(p *PreparedProgram) {
		m.SetExBudget(DefaultExBudget)
		if _, err := RunPreparedScriptContext(context.Background(), m, p); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		run(small)
		run(large)
	}
	const pairs = 20
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range pairs {
		run(small)
		run(large)
	}
	runtime.ReadMemStats(&after)
	// Reallocating the arenas costs megabytes per pair; reusing them,
	// kilobytes.
	if perPair := (after.TotalAlloc - before.TotalAlloc) / pairs; perPair > 256<<10 {
		t.Errorf("alternating runs allocate %d bytes per pair, want at most 256KiB", perPair)
	}
}

// TestValueArenaShrinksAfterSmallerRuns checks that a machine keeps a larger
// retained value arena chunk count through fewer than valueArenaShrinkRuns smaller runs
// and shrinks once that many follow in a row.
func TestValueArenaShrinksAfterSmallerRuns(t *testing.T) {
	small := pooledTestProgram(t, "guardrail-sorted-small")
	large := pooledTestProgram(t, "stablecoin_1-5")
	m := pooledTestMachine()
	run := func(p *PreparedProgram) {
		m.SetExBudget(DefaultExBudget)
		if _, err := RunPreparedScriptContext(context.Background(), m, p); err != nil {
			t.Fatal(err)
		}
	}
	run(large)
	run(large)
	grown := m.valueRetainChunks
	run(small)
	for i := 1; i < valueArenaShrinkRuns; i++ {
		if m.valueRetainChunks != grown {
			t.Fatalf("after %d smaller runs the retained chunk count is %d, want %d", i, m.valueRetainChunks, grown)
		}
		run(small)
	}
	if m.valueRetainChunks >= grown {
		t.Errorf("after %d smaller runs the retained chunk count is still %d", valueArenaShrinkRuns, m.valueRetainChunks)
	}
}

// TestValueArenaShrinkCountRestartsAfterLargerRun checks that a larger run
// restarts the count of smaller runs, so a machine shrinks only after
// valueArenaShrinkRuns consecutive smaller runs.
func TestValueArenaShrinkCountRestartsAfterLargerRun(t *testing.T) {
	small := pooledTestProgram(t, "guardrail-sorted-small")
	large := pooledTestProgram(t, "stablecoin_1-5")
	m := pooledTestMachine()
	run := func(p *PreparedProgram) {
		m.SetExBudget(DefaultExBudget)
		if _, err := RunPreparedScriptContext(context.Background(), m, p); err != nil {
			t.Fatal(err)
		}
	}
	run(large)
	run(large)
	grown := m.valueRetainChunks
	for range valueArenaShrinkRuns / 2 {
		run(small)
	}
	run(large)
	for i := 1; i < valueArenaShrinkRuns; i++ {
		run(small)
		if m.valueRetainChunks != grown {
			t.Fatalf("after %d smaller runs following a larger run the retained chunk count is %d, want %d",
				i, m.valueRetainChunks, grown)
		}
	}
	run(small)
	if m.valueRetainChunks >= grown {
		t.Errorf("after %d smaller runs following a larger run the retained chunk count is still %d",
			valueArenaShrinkRuns, m.valueRetainChunks)
	}
}

// TestValueArenaKeepsChunksWhenGrowing checks that a warm machine moving to a
// larger script keeps the value arena chunks it already holds and only adds
// the ones the larger script needs.
func TestValueArenaKeepsChunksWhenGrowing(t *testing.T) {
	small := pooledTestProgram(t, "guardrail-sorted-small")
	large := pooledTestProgram(t, "stablecoin_1-5")
	m := pooledTestMachine()
	run := func(p *PreparedProgram) {
		m.SetExBudget(DefaultExBudget)
		if _, err := RunPreparedScriptContext(context.Background(), m, p); err != nil {
			t.Fatal(err)
		}
	}
	run(small)
	run(small)
	if len(m.constantChunks) == 0 {
		t.Fatal("small script left no constant chunks")
	}
	kept := &m.constantChunks[0][0]
	run(large)
	if len(m.constantChunks) == 0 || &m.constantChunks[0][0] != kept {
		t.Fatal("growing to a larger script dropped the constant arena's chunks")
	}
}
