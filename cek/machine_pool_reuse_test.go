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
	"math/big"
	"os"
	"reflect"
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

// TestReusedMachineRunsWithAssignedBudget checks that a reused machine runs
// with exactly the budget in ExBudget, whether it was set through
// SetExBudget or assigned, including when it equals the previous run's
// remaining budget.
func TestReusedMachineRunsWithAssignedBudget(t *testing.T) {
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
		t.Errorf(
			"after SetExBudget(remaining): budget %+v, want %+v",
			m.ExBudget,
			want,
		)
	}

	m.ExBudget = start
	run(m)
	remaining = m.ExBudget
	m.ExBudget = remaining
	run(m)
	if want := remaining.Sub(&used); m.ExBudget != want {
		t.Errorf(
			"after assigning the remaining budget: budget %+v, want %+v",
			m.ExBudget,
			want,
		)
	}

	// A machine run again without setting its budget continues from what
	// the previous run left.
	remaining = m.ExBudget
	run(m)
	if want := remaining.Sub(&used); m.ExBudget != want {
		t.Errorf(
			"after an unchanged budget: budget %+v, want %+v",
			m.ExBudget,
			want,
		)
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
		t.Errorf(
			"alternating runs allocate %d bytes per pair, want at most 256KiB",
			perPair,
		)
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
	grown := m.valueArenaRetentions[valueArenaLambda].retainedSlots()
	run(small)
	for i := 1; i < valueArenaShrinkRuns; i++ {
		if got := m.valueArenaRetentions[valueArenaLambda].retainedSlots(); got != grown {
			t.Fatalf(
				"after %d smaller runs the retained slot count is %d, want %d",
				i,
				got,
				grown,
			)
		}
		run(small)
	}
	if got := m.valueArenaRetentions[valueArenaLambda].retainedSlots(); got >= grown {
		t.Errorf(
			"after %d smaller runs the retained slot count is still %d",
			valueArenaShrinkRuns,
			got,
		)
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
	grown := m.valueArenaRetentions[valueArenaLambda].retainedSlots()
	for range valueArenaShrinkRuns / 2 {
		run(small)
	}
	run(large)
	for i := 1; i < valueArenaShrinkRuns; i++ {
		run(small)
		if got := m.valueArenaRetentions[valueArenaLambda].retainedSlots(); got != grown {
			t.Fatalf(
				"after %d smaller runs following a larger run the retained slot count is %d, want %d",
				i,
				got,
				grown,
			)
		}
	}
	run(small)
	if got := m.valueArenaRetentions[valueArenaLambda].retainedSlots(); got >= grown {
		t.Errorf(
			"after %d smaller runs following a larger run the retained slot count is still %d",
			valueArenaShrinkRuns,
			got,
		)
	}
}

// TestValueArenasShrinkIndependently checks that continued use of one arena
// does not keep another arena at its previous high watermark.
func TestValueArenasShrinkIndependently(t *testing.T) {
	m := pooledTestMachine()
	const grownSlots = valueMinRetainSlots + valueColdChunkSize
	finishRun := func() {
		m.updateValueArenaRetentions()
		m.lazyPrepareValueArenas()
	}
	fillLambda := func() {
		for range grownSlots {
			m.allocLambda(nil, nil)
		}
	}
	fillConstant := func() {
		for range grownSlots {
			m.allocConstant(nil)
		}
	}

	fillLambda()
	fillConstant()
	finishRun()
	if got := len(m.lambdaChunks); got != 9 {
		t.Fatalf("lambda arena retained %d chunks after growing, want 9", got)
	}
	if got := len(m.constantChunks); got != 9 {
		t.Fatalf("constant arena retained %d chunks after growing, want 9", got)
	}

	for range valueArenaShrinkRuns {
		fillLambda()
		finishRun()
	}
	if got := len(m.lambdaChunks); got != 9 {
		t.Fatalf("hot lambda arena retained %d chunks, want 9", got)
	}
	if got := len(m.constantChunks); got != 8 {
		t.Fatalf("cold constant arena retained %d chunks, want 8", got)
	}
}

func retainedArenaBytes[S any](chunks [][]S) int {
	retained := cap(chunks) * int(reflect.TypeFor[[]S]().Size())
	for _, chunk := range chunks {
		retained += len(chunk) * int(reflect.TypeFor[S]().Size())
	}
	return retained
}

func retainedValueArenaBytes(m *Machine[syn.DeBruijn]) int {
	return retainedArenaBytes(m.constrChunks) +
		retainedArenaBytes(m.delayChunks) +
		retainedArenaBytes(m.lambdaChunks) +
		retainedArenaBytes(m.constantChunks) +
		retainedArenaBytes(m.dataChunks) +
		retainedArenaBytes(m.integerChunks) +
		retainedArenaBytes(m.byteStringChunks) +
		retainedArenaBytes(m.protoListChunks) +
		retainedArenaBytes(m.protoPairChunks) +
		retainedArenaBytes(m.dataValueChunks) +
		retainedArenaBytes(m.dataListValueChunks) +
		retainedArenaBytes(m.dataMapValueChunks) +
		retainedArenaBytes(m.pairValueChunks) +
		retainedArenaBytes(m.valueElemChunks) +
		retainedArenaBytes(m.builtinChunks) +
		retainedArenaBytes(m.builtinArgChunks)
}

func growValueArena(m *Machine[syn.DeBruijn], arena, slots int) {
	if arena == valueArenaElem {
		// One oversized chunk verifies that the byte cap accounts for actual
		// chunk lengths instead of assuming the normal element chunk size.
		m.allocValueElems(slots)
		return
	}
	var integer big.Int
	for range slots {
		switch arena {
		case valueArenaConstr:
			m.allocConstr(0, nil)
		case valueArenaDelay:
			m.allocDelay(nil, nil)
		case valueArenaLambda:
			m.allocLambda(nil, nil)
		case valueArenaConstant:
			m.allocConstant(nil)
		case valueArenaData:
			m.allocDataConstant(nil)
		case valueArenaInteger:
			m.allocIntegerConstant(&integer)
		case valueArenaByteString:
			m.allocByteStringConstant(nil)
		case valueArenaProtoList:
			m.allocProtoListConstant(nil, nil)
		case valueArenaProtoPair:
			m.allocProtoPairConstant(nil, nil, nil, nil)
		case valueArenaDataValue:
			m.allocDataValue(nil)
		case valueArenaDataListValue:
			m.allocDataListValue(nil)
		case valueArenaDataMapValue:
			m.allocDataMapValue(nil)
		case valueArenaPairValue:
			m.allocPairValue(nil, nil)
		case valueArenaBuiltin:
			m.allocBuiltin(0, 0, 1, nil)
		case valueArenaBuiltinArg:
			m.extendBuiltinArgs(nil, nil)
		default:
			panic("unhandled value arena")
		}
	}
}

func TestValueArenaAggregateRetentionIsBounded(t *testing.T) {
	m := pooledTestMachine()
	const slotsPerArena = 100_000
	for arena := range valueArenaCount {
		growValueArena(m, arena, slotsPerArena)
		m.updateValueArenaRetentions()
		m.lazyPrepareValueArenas()
		if got := retainedValueArenaBytes(m); got > valueArenaRetainBytes {
			t.Fatalf(
				"after arena %d retained %d bytes, want at most %d",
				arena,
				got,
				valueArenaRetainBytes,
			)
		}
	}
}

func TestValueElemArenaRetentionUsesActualChunkSize(t *testing.T) {
	m := pooledTestMachine()
	for range valueMinRetainSlots + 1 {
		m.allocValueElems(1)
	}
	m.updateValueArenaRetentions()
	m.lazyPrepareValueArenas()
	if got := len(m.valueElemChunks); got != 5 {
		t.Fatalf(
			"value element arena retained %d chunks after growing, want 5",
			got,
		)
	}
	for range valueArenaShrinkRuns {
		m.updateValueArenaRetentions()
		m.lazyPrepareValueArenas()
	}
	if got := len(m.valueElemChunks); got != 4 {
		t.Fatalf("cold value element arena retained %d chunks, want 4", got)
	}
}

func TestOversizedValueElemArenaShrinksAfterSmallerRuns(t *testing.T) {
	m := pooledTestMachine()
	m.allocValueElems(5000)
	m.updateValueArenaRetentions()
	m.lazyPrepareValueArenas()
	if got := len(m.valueElemChunks); got != 1 {
		t.Fatalf(
			"value element arena retained %d oversized chunks, want 1",
			got,
		)
	}
	for range valueArenaShrinkRuns {
		m.allocValueElems(1)
		m.updateValueArenaRetentions()
		m.lazyPrepareValueArenas()
	}
	if got := len(m.valueElemChunks); got != 0 {
		t.Fatalf(
			"cold value element arena retained %d oversized chunks, want 0",
			got,
		)
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
	if len(m.lambdaChunks) == 0 {
		t.Fatal("small script left no lambda chunks")
	}
	kept := make([]*Lambda[syn.DeBruijn], len(m.lambdaChunks))
	for i := range m.lambdaChunks {
		kept[i] = &m.lambdaChunks[i][0]
	}
	run(large)
	if len(m.lambdaChunks) <= len(kept) {
		t.Fatalf("larger script did not add lambda chunks: %d, had %d",
			len(m.lambdaChunks), len(kept))
	}
	for i, p := range kept {
		if &m.lambdaChunks[i][0] != p {
			t.Fatalf("growing to a larger script replaced lambda chunk %d", i)
		}
	}
}

func TestValueElemRetentionIgnoresSkippedChunkTails(t *testing.T) {
	m := pooledTestMachine()
	// Each 600-slot request does not fit the tail of a 1024-slot chunk, so the
	// cursor skips 424 slots per chunk while only 600 are requested.
	for range 8 {
		m.allocValueElems(600)
	}
	if m.valueElemSlotsUsed != 4800 {
		t.Fatalf("requested slots = %d, want 4800", m.valueElemSlotsUsed)
	}
	if m.valueElemChunkPos <= m.valueElemSlotsUsed {
		t.Fatalf(
			"cursor %d should exceed requested slots %d",
			m.valueElemChunkPos,
			m.valueElemSlotsUsed,
		)
	}
	var fromRequested valueArenaRetention
	updateValueArenaRetention(
		&fromRequested,
		m.valueElemSlotsUsed,
		valueElemChunkSize,
	)
	m.updateValueArenaRetentions()
	if got, want := m.valueArenaRetentions[valueArenaElem].retainedSlots(), fromRequested.retainedSlots(); got != want {
		t.Fatalf("retained slots %d, want %d from requested count", got, want)
	}
}

func TestValueArenaRetentionBudgetRotatesAcrossResets(t *testing.T) {
	m := pooledTestMachine()
	const slotsPerArena = 100_000
	retained := false
	for range valueArenaCount {
		for arena := range valueArenaCount {
			growValueArena(m, arena, slotsPerArena)
		}
		m.updateValueArenaRetentions()
		m.lazyPrepareValueArenas()
		if len(m.builtinArgChunks) > 0 {
			retained = true
			break
		}
	}
	if !retained {
		t.Fatal("last arena never retained chunks under a saturated budget")
	}
}
