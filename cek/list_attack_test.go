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
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// UPLC text helpers for building loop-based list workloads. Loops use a
// strict fixpoint combinator so program size stays constant while the
// number of builtin calls scales with the loop count.

const uplcFix = "(lam f [(lam x [f (lam v [x x v])]) (lam x [f (lam v [x x v])])])"

func uIte(c, t, e string) string {
	return fmt.Sprintf("(force [(force (builtin ifThenElse)) %s (delay %s) (delay %s)])", c, t, e)
}

func uCons(h, t string) string { return fmt.Sprintf("[(force (builtin mkCons)) %s %s]", h, t) }
func uHead(l string) string    { return fmt.Sprintf("[(force (builtin headList)) %s]", l) }
func uTail(l string) string    { return fmt.Sprintf("[(force (builtin tailList)) %s]", l) }
func uSub1(n string) string    { return fmt.Sprintf("[(builtin subtractInteger) %s (con integer 1)]", n) }
func uIsZero(n string) string  { return fmt.Sprintf("[(builtin equalsInteger) %s (con integer 0)]", n) }
func uAdd(a, b string) string  { return fmt.Sprintf("[(builtin addInteger) %s %s]", a, b) }
func uIData(n string) string   { return fmt.Sprintf("[(builtin iData) %s]", n) }
func uInt(n int) string        { return fmt.Sprintf("(con integer %d)", n) }

func uPairData(n string) string {
	return fmt.Sprintf("[(builtin mkPairData) %s %s]", uIData(n), uIData(n))
}

// uLoop builds [fix (lam self (lam k (lam s ... body)))] where body may
// refer to self, k and the state variables.
func uLoop(state []string, body string) string {
	inner := body
	for i := len(state) - 1; i >= 0; i-- {
		inner = fmt.Sprintf("(lam %s %s)", state[i], inner)
	}
	return fmt.Sprintf("[%s (lam self (lam k %s))]", uplcFix, inner)
}

// uBuild returns a term building a list of n elements by chained mkCons onto
// nilTerm, where elem(k) yields the k-th element.
func uBuild(n int, nilTerm string, elem func(string) string) string {
	loop := uLoop([]string{"acc"}, uIte(uIsZero("k"), "acc",
		fmt.Sprintf("[self %s %s]", uSub1("k"), uCons(elem("k"), "acc"))))
	return fmt.Sprintf("[%s %s %s]", loop, uInt(n), nilTerm)
}

func uIdent(s string) string { return s }

const (
	uNilInt  = "(con (list integer) [])"
	uNilData = "(con (list data) [])"
	uNilPair = "(con (list (pair data data)) [])"
)

// dataLiteral returns a (list data) constant [I from, ..., I from+n-1].
func dataLiteral(from, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("I %d", from+i)
	}
	return "(con (list data) [" + strings.Join(parts, ", ") + "])"
}

func uProgram(term string) string { return "(program 1.1.0 " + term + ")" }

type listAttack struct {
	name  string
	proto uint
	// prog builds a program performing O(n) list operations.
	prog func(n int) string
	// residual marks patterns that are still O(n) per call by design (the
	// result must be a distinct contiguous data.List); they are measured but
	// carry no regression bound.
	residual bool
}

func listAttacks() []listAttack {
	return []listAttack{
		{
			name: "single-chain-mkCons",
			prog: func(n int) string {
				return uProgram(uHead(uBuild(n, uNilInt, uIdent)))
			},
		},
		{
			// Two chains consed alternately defeat a single cons buffer.
			name: "interleaved-chains-mkCons",
			prog: func(n int) string {
				loop := uLoop([]string{"a", "b"}, uIte(uIsZero("k"),
					uAdd(uHead("a"), uHead("b")),
					fmt.Sprintf("[self %s b %s]", uSub1("k"), uCons("k", "a"))))
				return uProgram(fmt.Sprintf("[%s %s (con (list integer) [0]) (con (list integer) [0])]", loop, uInt(n)))
			},
		},
		{
			name: "interleaved-three-data-chains",
			prog: func(n int) string {
				loop := uLoop([]string{"a", "b", "c"}, uIte(uIsZero("k"),
					"(con integer 0)",
					fmt.Sprintf("[self %s b c %s]", uSub1("k"), uCons(uIData("k"), "a"))))
				nilD := "[(builtin mkNilData) (con unit ())]"
				return uProgram(fmt.Sprintf("[%s %s %s %s %s]", loop, uInt(n),
					uCons(uIData("(con integer 0)"), nilD), uCons(uIData("(con integer 0)"), nilD), uCons(uIData("(con integer 0)"), nilD)))
			},
		},
		{
			// Repeatedly cons onto the same (non-front) tail.
			name: "cons-same-old-tail",
			prog: func(n int) string {
				loop := uLoop([]string{"t", "acc"}, uIte(uIsZero("k"), "acc",
					fmt.Sprintf("[self %s t %s]", uSub1("k"), uAdd("acc", uHead(uCons("k", "t"))))))
				return uProgram(fmt.Sprintf("[%s %s %s (con integer 0)]", loop, uInt(n), uBuild(n, uNilInt, uIdent)))
			},
		},
		{
			name: "cons-same-old-data-tail",
			prog: func(n int) string {
				loop := uLoop([]string{"t", "acc"}, uIte(uIsZero("k"), "acc",
					fmt.Sprintf("[self %s t %s]", uSub1("k"), uHead(uCons(uIData("k"), "t")))))
				d := fmt.Sprintf("[(builtin unListData) [(builtin listData) %s]]", uBuild(n, uNilData, uIData))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), d, uIData("(con integer 0)")))
			},
		},
		{
			// x := k : tail x keeps the length fixed but every tail has its
			// front slot taken.
			name: "cons-after-tailList",
			prog: func(n int) string {
				loop := uLoop([]string{"x"}, uIte(uIsZero("k"), uHead("x"),
					fmt.Sprintf("[self %s %s]", uSub1("k"), uCons("k", uTail("x")))))
				return uProgram(fmt.Sprintf("[%s %s %s]", loop, uInt(n), uBuild(n, uNilInt, uIdent)))
			},
		},
		{
			name:  "cons-after-dropList",
			proto: 11,
			prog: func(n int) string {
				loop := uLoop([]string{"x"}, uIte(uIsZero("k"), uHead("x"),
					fmt.Sprintf("[self %s %s]", uSub1("k"),
						uCons("k", "[(force (builtin dropList)) (con integer 1) x]"))))
				return uProgram(fmt.Sprintf("[%s %s %s]", loop, uInt(n), uBuild(n, uNilInt, uIdent)))
			},
		},
		{
			// listData after every cons of a growing constant list.
			name: "listData-growing-chain",
			prog: func(n int) string {
				loop := uLoop([]string{"l", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[(lam l2 [self %s l2 [(builtin listData) l2]]) %s]",
						uSub1("k"), uCons(uIData("k"), "l"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), uNilData, uIData("(con integer 0)")))
			},
		},
		{
			name: "listData-successive-tails",
			prog: func(n int) string {
				loop := uLoop([]string{"l", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[self %s %s [(builtin listData) l]]", uSub1("k"), uTail("l"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), uBuild(n, uNilData, uIData), uIData("(con integer 0)")))
			},
		},
		{
			name: "constrData-successive-tails",
			prog: func(n int) string {
				loop := uLoop([]string{"l", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[self %s %s [(builtin constrData) (con integer 0) l]]", uSub1("k"), uTail("l"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), uBuild(n, uNilData, uIData), uIData("(con integer 0)")))
			},
		},
		{
			name: "mapData-successive-tails",
			prog: func(n int) string {
				loop := uLoop([]string{"l", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[self %s %s [(builtin mapData) l]]", uSub1("k"), uTail("l"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), uBuild(n, uNilPair, uPairData), uIData("(con integer 0)")))
			},
		},
		{
			name: "mapData-interleaved-growing",
			prog: func(n int) string {
				loop := uLoop([]string{"a", "b", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[(lam a2 [self %s b a2 [(builtin mapData) a2]]) %s]",
						uSub1("k"), uCons(uPairData("k"), "a"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s %s]", loop, uInt(n), uNilPair, uNilPair, uIData("(con integer 0)")))
			},
		},
		{
			// A data list as the head of a list-of-lists must be converted
			// to a constant on every mkCons.
			name: "mkCons-data-list-head",
			prog: func(n int) string {
				loop := uLoop([]string{"d", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[self %s d %s]", uSub1("k"),
						uCons("[(builtin unListData) d]", "(con (list (list data)) [])"))))
				d := fmt.Sprintf("[(builtin listData) %s]", uBuild(n, uNilData, uIData))
				return uProgram(fmt.Sprintf("[%s %s %s (con (list (list data)) [])]", loop, uInt(n), d))
			},
		},
		{
			name: "mkCons-onto-unListData",
			prog: func(n int) string {
				loop := uLoop([]string{"d", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[self %s d %s]", uSub1("k"),
						uCons(uIData("k"), "[(builtin unListData) d]"))))
				d := fmt.Sprintf("[(builtin listData) %s]", uBuild(n, uNilData, uIData))
				return uProgram(fmt.Sprintf("[%s %s %s (con integer 0)]", loop, uInt(n), d))
			},
		},
		{
			// Successive tails of a program literal (not built by mkCons).
			name: "listData-tails-of-literal",
			prog: func(n int) string {
				lit := dataLiteral(0, 2000)
				loop := uLoop([]string{"cur", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[(lam c [self %s %s [(builtin listData) c]]) %s]", uSub1("k"), uTail("c"),
						uIte("[(force (builtin nullList)) cur]", lit, "cur"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), lit, uIData("(con integer 0)")))
			},
		},
		{
			// listData of one literal over and over.
			name: "listData-same-literal",
			prog: func(n int) string {
				loop := uLoop([]string{"l", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[self %s l [(builtin listData) l]]", uSub1("k"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), dataLiteral(0, 2000), uIData("(con integer 0)")))
			},
		},
		{
			// Round-robin over many literal lists, so a small fixed-size
			// conversion cache would miss on every call.
			name: "listData-cycling-literals",
			prog: func(n int) string {
				var inner []string
				for i := range 64 {
					inner = append(inner, strings.TrimSuffix(strings.TrimPrefix(dataLiteral(i*1000, 200), "(con (list data) "), ")"))
				}
				outer := "(con (list (list data)) [" + strings.Join(inner, ", ") + "])"
				loop := uLoop([]string{"cur", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[(lam c [self %s %s [(builtin listData) %s]]) %s]", uSub1("k"), uTail("c"), uHead("c"),
						uIte("[(force (builtin nullList)) cur]", outer, "cur"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), outer, uIData("(con integer 0)")))
			},
		},
		{
			// Each branch k : t is a distinct list; listData must produce a
			// distinct contiguous data.List, so this stays O(n) per call.
			name:     "listData-branches-of-one-tail",
			residual: true,
			prog: func(n int) string {
				loop := uLoop([]string{"t", "acc"}, uIte(uIsZero("k"), "(con integer 0)",
					fmt.Sprintf("[self %s t [(builtin listData) %s]]", uSub1("k"), uCons(uIData("k"), "t"))))
				return uProgram(fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), uBuild(n, uNilData, uIData), uIData("(con integer 0)")))
			},
		},
	}
}

// evalV3 evaluates a textual program as Plutus V3 at protocol version proto
// (0 means PV10).
func evalV3(prog string, proto uint, budget cek.ExBudget) (syn.Term[syn.DeBruijn], cek.ExBudget, error) {
	if proto == 0 {
		proto = 10
	}
	ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: proto})
	return cek.EvaluateText(context.Background(), prog, lang.LanguageVersionV3, ec, budget)
}

// evalAlloc returns the bytes allocated while parsing and evaluating prog.
// TotalAlloc is process-wide, so the measurement is only meaningful while no
// other test allocates. Callers must not use t.Parallel (neither at the top
// level nor in subtests): Go starts parallel tests only after every
// sequential top-level test in the binary has finished, so a sequential
// caller never overlaps the package's parallel tests.
func evalAlloc(t testing.TB, prog string, proto uint) uint64 {
	t.Helper()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, _, err := evalV3(prog, proto, cek.ExBudget{Cpu: 1 << 62, Mem: 1 << 62})
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	return after.TotalAlloc - before.TotalAlloc
}

// TestListAttacksAreNotQuadratic bounds the allocation per loop iteration
// for each list pattern. Copying the list on every call allocates O(n) bytes
// per iteration (megabytes per iteration at this size); sharing keeps it to a
// small constant. It and its subtests must stay sequential; see evalAlloc.
func TestListAttacksAreNotQuadratic(t *testing.T) {
	const n = 8000
	// Each loop iteration allocates closures, frames and values; ~2KB per
	// iteration is generous. A copying implementation allocates >= 16*n bytes
	// (128KB) per iteration here.
	const perIterBound = 4 << 10
	for _, a := range listAttacks() {
		if a.residual {
			continue
		}
		t.Run(a.name, func(t *testing.T) {
			if alloc := evalAlloc(t, a.prog(n), a.proto); alloc > uint64(perIterBound*n) {
				t.Fatalf("allocated %d MB for n=%d (%d B/iter): quadratic list work",
					alloc>>20, n, alloc/uint64(n))
			}
		})
	}
}

// TestUnrolledMkConsChainIsNotQuadratic guards against copying the whole list
// on every mkCons of a chain written out in the program, while only a
// constant cost is charged. It must stay sequential; see evalAlloc.
func TestUnrolledMkConsChainIsNotQuadratic(t *testing.T) {
	const n = 16000
	prog := uProgram(strings.Repeat("[(force (builtin mkCons)) (con integer 1) ", n) +
		uNilInt + strings.Repeat("]", n))
	// Quadratic copying allocated ~2GB here; linear is tens of MB.
	if alloc := evalAlloc(t, prog, 10); alloc > 400<<20 {
		t.Fatalf("chained mkCons allocated %d MB", alloc>>20)
	}
}
