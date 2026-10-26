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
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// End-to-end tests of list sharing: programs that interleave cons chains,
// cons repeatedly onto shared tails, cons after tailList/dropList and convert
// the results must produce exactly the lists a copying implementation would.

func intList(xs ...int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprint(x)
	}
	return "(con (list integer) [" + strings.Join(parts, ", ") + "])"
}

func dataItems(xs ...int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("I %d", x)
	}
	return strings.Join(parts, ", ")
}

func dataList(xs ...int) string {
	return "(con data (List [" + dataItems(xs...) + "]))"
}

func seq(from, to int) []int {
	var out []int
	for i := from; i <= to; i++ {
		out = append(out, i)
	}
	return out
}

func cat(parts ...[]int) []int {
	var out []int
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// interleaveModel computes the result of the interleave loop with copying
// lists: each step conses k onto a and swaps the two lists.
func interleaveModel(n int, a, b []int) string {
	for k := n; k > 0; k-- {
		a, b = b, append([]int{k}, a...)
	}
	return uConstr(intList(a...), intList(b...))
}

func uLet(name, value, body string) string {
	return fmt.Sprintf("[(lam %s %s) %s]", name, body, value)
}

func uConstr(fields ...string) string {
	return "(constr 0 " + strings.Join(fields, " ") + ")"
}

func uListData(l string) string { return "[(builtin listData) " + l + "]" }
func uDrop(n int, l string) string {
	return fmt.Sprintf("[(force (builtin dropList)) (con integer %d) %s]", n, l)
}

func evalPretty(t *testing.T, prog string, proto uint) string {
	t.Helper()
	res, _, err := evalV3(prog, proto, cek.ExBudget{Cpu: 1 << 50, Mem: 1 << 50})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	return syn.PrettyTerm[syn.DeBruijn](res)
}

func expectPretty(t *testing.T, prog string, proto uint, want string) {
	t.Helper()
	got := evalPretty(t, prog, proto)
	wantPretty := evalPretty(t, uProgram(want), proto)
	if got != wantPretty {
		t.Fatalf("got\n%s\nwant\n%s", got, wantPretty)
	}
}

func TestListSharingPrograms(t *testing.T) {
	// t10 = [1..10], built by a cons chain so it is the front of a buffer.
	t10 := uBuild(10, uNilInt, uIdent)
	t10d := uBuild(10, uNilData, uIData)
	interleave := func(n int, nilA, nilB string) string {
		loop := uLoop([]string{"a", "b"}, uIte(uIsZero("k"), uConstr("a", "b"),
			fmt.Sprintf("[self %s b %s]", uSub1("k"), uCons("k", "a"))))
		return fmt.Sprintf("[%s %s %s %s]", loop, uInt(n), nilA, nilB)
	}
	tests := []struct {
		name  string
		proto uint
		prog  string
		want  string
	}{
		{
			name: "interleaved chains",
			prog: interleave(9, uNilInt, uNilInt),
			want: interleaveModel(9, nil, nil),
		},
		{
			name: "interleaved chains on long literal tails",
			prog: interleave(4, intList(seq(20, 32)...), intList(seq(40, 52)...)),
			want: interleaveModel(4, seq(20, 32), seq(40, 52)),
		},
		{
			name: "cons onto the same tail",
			prog: uLet("t", t10, uLet("a", uCons("(con integer 100)", "t"),
				uLet("b", uCons("(con integer 200)", "t"),
					uLet("c", uCons("(con integer 300)", "a"),
						uLet("d", uCons("(con integer 400)", "t"),
							uLet("e", uCons("(con integer 500)", "a"),
								uConstr("t", "a", "b", "c", "d", "e"))))))),
			want: uConstr(intList(seq(1, 10)...), intList(cat([]int{100}, seq(1, 10))...),
				intList(cat([]int{200}, seq(1, 10))...), intList(cat([]int{300, 100}, seq(1, 10))...),
				intList(cat([]int{400}, seq(1, 10))...), intList(cat([]int{500, 100}, seq(1, 10))...)),
		},
		{
			name: "cons onto a short tail built by mkCons",
			prog: uLet("l", uCons("(con integer 5)", uCons("(con integer 4)", uNilInt)),
				uLet("a", uCons("(con integer 7)", "l"),
					uLet("b", uCons("(con integer 9)", "l"),
						uLet("c", uCons("(con integer 3)", "a"),
							uConstr("l", "a", "b", "c", uHead(uTail("b")), uHead(uTail(uTail("c")))))))),
			want: uConstr(intList(5, 4), intList(7, 5, 4), intList(9, 5, 4), intList(3, 7, 5, 4),
				"(con integer 5)", "(con integer 5)"),
		},
		{
			name: "listData of branches and of the shared tail",
			prog: uLet("t", t10d, uLet("a", uCons(uIData("(con integer 100)"), "t"),
				uLet("b", uCons(uIData("(con integer 200)"), "t"),
					uLet("c", uCons(uIData("(con integer 300)"), "a"),
						uConstr(uListData("b"), uListData("a"), uListData("c"), uListData("t"),
							uListData(uCons(uIData("(con integer 400)"), "b")), uListData("b")))))),
			want: uConstr(dataList(cat([]int{200}, seq(1, 10))...), dataList(cat([]int{100}, seq(1, 10))...),
				dataList(cat([]int{300, 100}, seq(1, 10))...), dataList(seq(1, 10)...),
				dataList(cat([]int{400, 200}, seq(1, 10))...), dataList(cat([]int{200}, seq(1, 10))...)),
		},
		{
			name: "cons after tailList",
			prog: uLet("t", t10, uLet("x", uCons("(con integer 0)", uTail("t")),
				uLet("y", uCons("(con integer 9)", uTail("t")),
					uLet("z", uCons("(con integer 7)", uTail("x")),
						uConstr("x", "y", "z", uTail(uTail("z")), "t"))))),
			want: uConstr(intList(cat([]int{0}, seq(2, 10))...), intList(cat([]int{9}, seq(2, 10))...),
				intList(cat([]int{7}, seq(2, 10))...), intList(seq(3, 10)...), intList(seq(1, 10)...)),
		},
		{
			name:  "cons after dropList",
			proto: 11,
			prog: uLet("t", t10, uLet("x", uCons("(con integer 0)", uDrop(3, "t")),
				uLet("y", uCons("(con integer 9)", uDrop(3, "t")),
					uConstr("x", "y", uDrop(2, "x"), uDrop(20, "y"), uDrop(-1, "x"), uDrop(0, "y"), "t")))),
			want: uConstr(intList(cat([]int{0}, seq(4, 10))...), intList(cat([]int{9}, seq(4, 10))...),
				intList(seq(5, 10)...), intList(), intList(cat([]int{0}, seq(4, 10))...),
				intList(cat([]int{9}, seq(4, 10))...), intList(seq(1, 10)...)),
		},
		{
			name: "data list chains via unListData",
			prog: uLet("d", "[(builtin unListData) "+dataList(seq(1, 12)...)+"]",
				uLet("a", uCons(uIData("(con integer 50)"), "d"),
					uLet("b", uCons(uIData("(con integer 60)"), "d"),
						uConstr(uListData("a"), uListData("b"), uListData(uCons(uIData("(con integer 70)"), "a")),
							"[(builtin constrData) (con integer 3) b]")))),
			want: uConstr(dataList(cat([]int{50}, seq(1, 12))...), dataList(cat([]int{60}, seq(1, 12))...),
				dataList(cat([]int{70, 50}, seq(1, 12))...),
				"(con data (Constr 3 ["+dataItems(cat([]int{60}, seq(1, 12))...)+"]))"),
		},
		{
			name: "list of lists with shared inner lists",
			prog: uLet("t", t10d, uLet("a", uCons(uIData("(con integer 100)"), "t"),
				uLet("b", uCons(uIData("(con integer 200)"), "t"),
					uLet("o", uCons("a", uCons("b", uCons("t", "(con (list (list data)) [])"))),
						uConstr("o", uListData(uHead(uTail("o"))), uListData(uHead("o"))))))),
			want: uConstr(
				"(con (list (list data)) [["+dataItems(cat([]int{100}, seq(1, 10))...)+"], ["+
					dataItems(cat([]int{200}, seq(1, 10))...)+"], ["+dataItems(seq(1, 10)...)+"]])",
				dataList(cat([]int{200}, seq(1, 10))...), dataList(cat([]int{100}, seq(1, 10))...)),
		},
		{
			name:  "case on cons cells",
			proto: 11,
			prog: func() string {
				// sum via case: case l of [] -> 0; h:t -> h + sum t
				sum := fmt.Sprintf("[%s (lam self (lam l (case l (lam h (lam tl [(builtin addInteger) h [self tl]])) (con integer 0))))]", uplcFix)
				return uLet("sum", sum, uLet("t", t10, uLet("a", uCons("(con integer 100)", "t"),
					uLet("b", uCons("(con integer 1000)", "t"),
						uConstr("[sum a]", "[sum b]", "[sum t]", "[sum "+uCons("(con integer 7)", "b")+"]")))))
			}(),
			want: uConstr("(con integer 155)", "(con integer 1055)", "(con integer 55)", "(con integer 1062)"),
		},
		{
			name: "mapData of interleaved pair chains",
			prog: func() string {
				loop := uLoop([]string{"a", "b"}, uIte(uIsZero("k"),
					uConstr("[(builtin mapData) a]", "[(builtin mapData) b]"),
					fmt.Sprintf("[self %s b %s]", uSub1("k"), uCons(uPairData("k"), "a"))))
				return fmt.Sprintf("[%s (con integer 4) %s [(builtin mkNilPairData) (con unit ())]]", loop, uNilPair)
			}(),
			want: uConstr("(con data (Map [(I 2, I 2), (I 4, I 4)]))", "(con data (Map [(I 1, I 1), (I 3, I 3)]))"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectPretty(t, uProgram(tc.prog), tc.proto, tc.want)
		})
	}
}

func parseDeBruijn(t testing.TB, prog string) *syn.Program[syn.DeBruijn] {
	t.Helper()
	p, err := syn.Parse(prog)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	db, err := syn.NameToDeBruijn(p)
	if err != nil {
		t.Fatalf("debruijn: %v", err)
	}
	return db
}

// listShareWorkload builds several interleaved chains on top of a literal
// tail, branches off old tails, and returns the lists and their listData.
func listShareWorkload(n int) string {
	lit := dataList(seq(1, 16)...)
	loop := uLoop([]string{"a", "b", "c"}, uIte(uIsZero("k"),
		uConstr("a", "b", "c", uListData("a"), uListData("b"), uListData(uCons(uIData("(con integer -1)"), "c"))),
		fmt.Sprintf("[self %s b c %s]", uSub1("k"), uCons(uIData("k"), uIte(
			"[(builtin equalsInteger) [(builtin modInteger) k (con integer 5)] (con integer 0)]",
			uTail("a"), "a")))))
	d := "[(builtin unListData) " + lit + "]"
	return uProgram(fmt.Sprintf("[%s %s %s %s %s]", loop, uInt(n), d, d, uCons(uIData("(con integer 0)"), d)))
}

func TestListSharingMachineReuse(t *testing.T) {
	progA := parseDeBruijn(t, listShareWorkload(200))
	progB := parseDeBruijn(t, listShareWorkload(333))
	ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 10})
	m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
	m.ExBudget = cek.ExBudget{Cpu: 1 << 50, Mem: 1 << 50}

	resA, err := m.Run(progA.Term)
	if err != nil {
		t.Fatal(err)
	}
	prettyA := syn.PrettyTerm[syn.DeBruijn](resA)
	remainingA := m.ExBudget
	for range 3 {
		if _, err := m.Run(progB.Term); err != nil {
			t.Fatal(err)
		}
		if got := syn.PrettyTerm[syn.DeBruijn](resA); got != prettyA {
			t.Fatal("result of an earlier run changed after the machine was reused")
		}
		resA2, err := m.Run(progA.Term)
		if err != nil {
			t.Fatal(err)
		}
		if got := syn.PrettyTerm[syn.DeBruijn](resA2); got != prettyA {
			t.Fatal("re-running a program on a reused machine changed its result")
		}
		if m.ExBudget != remainingA {
			t.Fatalf("re-run budget %+v, first run %+v", m.ExBudget, remainingA)
		}
	}
	// A fresh machine must agree with the reused one.
	fresh := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
	fresh.ExBudget = cek.ExBudget{Cpu: 1 << 50, Mem: 1 << 50}
	resF, err := fresh.Run(progA.Term)
	if err != nil {
		t.Fatal(err)
	}
	if syn.PrettyTerm[syn.DeBruijn](resF) != prettyA {
		t.Fatal("fresh machine disagrees with reused machine")
	}
}

// TestListSharingConcurrentMachines runs independent machines on one shared
// parsed program (whose literal lists all of them cons onto) concurrently.
// Run with -race: a write into shared literal storage is a data race.
func TestListSharingConcurrentMachines(t *testing.T) {
	prog := parseDeBruijn(t, listShareWorkload(300))
	ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 10})
	ref := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
	ref.ExBudget = cek.ExBudget{Cpu: 1 << 50, Mem: 1 << 50}
	refRes, err := ref.Run(prog.Term)
	if err != nil {
		t.Fatal(err)
	}
	want := syn.PrettyTerm[syn.DeBruijn](refRes)
	wantBudget := ref.ExBudget

	const goroutines = 16
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
			for i := range 8 {
				m.ExBudget = cek.ExBudget{Cpu: 1 << 50, Mem: 1 << 50}
				res, err := m.Run(prog.Term)
				if err != nil {
					errs <- err
					return
				}
				if syn.PrettyTerm[syn.DeBruijn](res) != want || m.ExBudget != wantBudget {
					errs <- fmt.Errorf("goroutine %d run %d: result or budget differs", g, i)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// errorClass reduces an evaluation error to what must not depend on the
// list representation: its type and code (messages may name Go types).
func errorClass(err error) string {
	var se *cek.ScriptError
	var te *cek.TypeError
	var be *cek.BuiltinError
	var bu *cek.BudgetError
	switch {
	case err == nil:
		return "ok"
	case errors.As(err, &se):
		return fmt.Sprintf("script:%v", se.Code)
	case errors.As(err, &te):
		return fmt.Sprintf("type:%v", te.Code)
	case errors.As(err, &be):
		return fmt.Sprintf("builtin:%v", be.Code)
	case errors.As(err, &bu):
		return fmt.Sprintf("budget:%v", bu.Code)
	default:
		return "other"
	}
}

// evalResult returns the pretty-printed result and the error class of prog.
func evalResult(prog string, proto uint) (string, string) {
	res, _, err := evalV3(prog, proto, cek.ExBudget{Cpu: 1 << 40, Mem: 1 << 40})
	if err != nil {
		return "", errorClass(err)
	}
	return syn.PrettyTerm[syn.DeBruijn](res), "ok"
}

// runWork is the work a run did: the machine steps of each kind and the
// budget charged by builtins.
type runWork struct {
	steps    [9]uint64
	builtins cek.ExBudget
}

func (w runWork) minus(o runWork) runWork {
	for i := range w.steps {
		w.steps[i] -= o.steps[i]
	}
	w.builtins = w.builtins.Sub(&o.builtins)
	return w
}

// evalWork evaluates prog on a metered machine with evalResult's budget.
func evalWork(t *testing.T, prog string, proto uint) runWork {
	t.Helper()
	pc := syn.ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: proto}
	named, err := syn.ParseWithContext(prog, pc)
	if err != nil {
		t.Fatal(err)
	}
	db, err := syn.NameToDeBruijn(named)
	if err != nil {
		t.Fatal(err)
	}
	ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: proto})
	m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
	m.EnableMetrics()
	m.ExBudget = cek.ExBudget{Cpu: 1 << 40, Mem: 1 << 40}
	_, _ = m.Run(db.Term)
	metrics := m.Metrics()
	w := runWork{steps: metrics.Steps}
	for _, b := range metrics.Builtins {
		w.builtins.Cpu += b.Budget.Cpu
		w.builtins.Mem += b.Budget.Mem
	}
	return w
}

// TestListSharingErrorsMatchContiguous checks that failing operations on
// lists built by mkCons fail exactly like the same operations on the
// equivalent literal list, with the same budget consumed.
func TestListSharingErrorsMatchContiguous(t *testing.T) {
	// Built list [1,2,3] (cons cells over a literal tail) vs literal.
	built := uCons("(con integer 1)", uCons("(con integer 2)", uCons("(con integer 3)", uNilInt)))
	branch := uLet("t", uBuild(12, uNilInt, uIdent),
		uLet("a", uCons("(con integer 1)", "t"), uCons("(con integer 2)", "t")))
	lit := intList(1, 2, 3)
	litBranch := intList(cat([]int{2}, seq(1, 12))...)
	builtD := uCons(uIData("(con integer 1)"), "[(builtin mkNilData) (con unit ())]")
	litD := "[(builtin unListData) " + dataList(1) + "]"
	tests := []struct {
		name  string
		proto uint
		body  func(l string) string
		lists [][2]string
	}{
		{"case three branches", 11, func(l string) string {
			return "(case " + l + " (lam h (lam t h)) (con integer 0) (con integer 9))"
		}, [][2]string{{built, lit}, {branch, litBranch}, {builtD, litD}}},
		{"case before PV11", 10, func(l string) string {
			return "(case " + l + " (lam h (lam t h)) (con integer 0))"
		}, [][2]string{{built, lit}, {builtD, litD}}},
		{"cons wrong element type", 10, func(l string) string {
			return uCons("(con bytestring #00)", l)
		}, [][2]string{{built, lit}, {branch, litBranch}}},
		{"cons integer onto data list", 10, func(l string) string {
			return uCons("(con integer 1)", l)
		}, [][2]string{{builtD, litD}}},
		{"listData of integers", 10, func(l string) string {
			return uListData(l)
		}, [][2]string{{built, lit}, {branch, litBranch}}},
		{"mapData of data list", 10, func(l string) string {
			return "[(builtin mapData) " + l + "]"
		}, [][2]string{{builtD, litD}}},
		{"unIData of list", 10, func(l string) string {
			return "[(builtin unIData) " + l + "]"
		}, [][2]string{{built, lit}, {builtD, litD}}},
		{"headList of tail of singleton", 10, func(l string) string {
			return uHead(uTail(l))
		}, [][2]string{{builtD, litD}}},
		{"dropList then head", 11, func(l string) string {
			return uHead(uDrop(3, l))
		}, [][2]string{{built, lit}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Both programs first bind the list, then run the body on it.
			// A failed run leaves its last steps uncharged, as the reference
			// does, so its consumed budget depends on where the step batches
			// fall. Compare the work instead: the steps of each kind and the
			// builtin charges, less those of binding the list.
			run := func(list string) (string, string, runWork) {
				prog := uProgram(uLet("l", list, tc.body("l")))
				res, errClass := evalResult(prog, tc.proto)
				work := evalWork(t, prog, tc.proto).
					minus(evalWork(t, uProgram(uLet("l", list, "(con integer 0)")), tc.proto))
				return res, errClass, work
			}
			for _, pair := range tc.lists {
				r1, e1, work1 := run(pair[0])
				r2, e2, work2 := run(pair[1])
				if e1 != e2 || r1 != r2 {
					t.Fatalf("built list: %s %q; literal list: %s %q", e1, r1, e2, r2)
				}
				if work1 != work2 {
					t.Fatalf("work on built list %+v, on literal %+v", work1, work2)
				}
			}
		})
	}
}
