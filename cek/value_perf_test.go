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
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// These regression tests count heap allocations made while evaluating, not
// wall-clock time, so they are stable on slow or race-instrumented CI. Each
// runs the same program shape at two sizes and bounds how the allocation
// count grows: the former implementations rebuilt or deep-copied the whole
// Value per operation, so their counts grew with the Value size (or
// quadratically for a chain), while the current ones copy only the touched
// policy list.
//
// The counts come from process-wide runtime.MemStats. None of these tests
// call t.Parallel, and Go only starts parallel tests after every sequential
// top-level test has finished, so no other test allocates concurrently.

// valueProgramMallocs parses prog outside the measurement and returns the
// number of heap allocations made while the machine evaluates it.
func valueProgramMallocs(t *testing.T, prog string) uint64 {
	t.Helper()
	ec := NewDefaultEvalContext(lang.LanguageVersionV4, ProtoVersion{Major: 12})
	program, err := syn.ParseWithContext(prog, syn.ProgramContext{
		LedgerLanguage: lang.LanguageVersionV4,
		ProtocolMajor:  12,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("debruijn: %v", err)
	}
	machine := NewMachine[syn.DeBruijn](lang.LanguageVersionV4, 0, ec)
	machine.ExBudget = ExBudget{Cpu: 1 << 62, Mem: 1 << 62}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = machine.RunContext(context.Background(), dbProgram.Term)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return after.Mallocs - before.Mallocs
}

// checkMallocGrowth fails if evaluating the large program allocates more
// than maxRatio times as often as the small one.
func checkMallocGrowth(t *testing.T, what string, small, large string, maxRatio float64) {
	t.Helper()
	smallMallocs := valueProgramMallocs(t, small)
	largeMallocs := valueProgramMallocs(t, large)
	ratio := float64(largeMallocs) / float64(max(smallMallocs, 1))
	if ratio > maxRatio {
		t.Fatalf(
			"%s: allocations grew %.1fx (%d -> %d), limit %.1fx",
			what, ratio, smallMallocs, largeMallocs, maxRatio,
		)
	}
}

func valueLiteral(n int) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i := range n {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "(#%02x%02x, [(#01, 1)])", i>>8, i&255)
	}
	sb.WriteString("]")
	return sb.String()
}

func insertCoinChainProgram(n int) string {
	p := "(con value [])"
	for j := range n {
		p = fmt.Sprintf(
			"[(builtin insertCoin) (con bytestring #%04x) (con bytestring #01) (con integer 5) %s]",
			j, p)
	}
	return "(program 1.1.0 " + p + ")"
}

// Regression: chained insertCoin used to rebuild the whole Value per call,
// so n calls allocated O(n^2) times (16x for 4x the calls); each call now
// allocates a constant number of times.
func TestInsertCoinChainIsNotQuadratic(t *testing.T) {
	checkMallocGrowth(
		t,
		"250 -> 1000 chained insertCoin",
		insertCoinChainProgram(250),
		insertCoinChainProgram(1000),
		8,
	)
}

func insertCoinOnValueProgram(size, inserts int) string {
	p := "(con value " + valueLiteral(size) + ")"
	for j := range inserts {
		p = fmt.Sprintf(
			"[(builtin insertCoin) (con bytestring #ff%02x) (con bytestring #01) (con integer 5) %s]",
			j, p)
	}
	return "(program 1.1.0 " + p + ")"
}

// Regression: insertCoin on a large Value used to rebuild every entry, about
// 480,000 allocations per call on a 10000-entry Value. Loading the Value
// constant itself allocates per entry, so the inserts are measured as the
// difference from the same program without them.
func TestInsertCoinOnLargeValueIsCheap(t *testing.T) {
	const (
		size           = 40_000
		inserts        = 21
		maxPerInsert   = 1000
		maxInsertTotal = inserts * maxPerInsert
	)
	base := valueProgramMallocs(t, insertCoinOnValueProgram(size, 0))
	withInserts := valueProgramMallocs(t, insertCoinOnValueProgram(size, inserts))
	if withInserts > base && withInserts-base > maxInsertTotal {
		t.Fatalf(
			"%d insertCoin on a %d entry Value made %d allocations, limit %d",
			inserts, size, withInserts-base, maxInsertTotal,
		)
	}
}

func valueContainsProgram(size int) string {
	const calls = 100
	body := "(con unit ())"
	for j := range calls {
		body = fmt.Sprintf(
			"[(lam d%d %s) [(builtin valueContains) (con value []) v]]", j, body)
	}
	return fmt.Sprintf("(program 1.1.0 [(lam v %s) (con value %s)])", body, valueLiteral(size))
}

// Regression: valueContains with a small required value is charged a constant
// but used to deep-copy and map both Values, so its allocations grew with the
// Value; they no longer depend on its size.
func TestValueContainsSmallRequiredIsCheap(t *testing.T) {
	checkMallocGrowth(
		t,
		"100 valueContains on a 1000 -> 4000 entry Value",
		valueContainsProgram(1000),
		valueContainsProgram(4000),
		2,
	)
}
