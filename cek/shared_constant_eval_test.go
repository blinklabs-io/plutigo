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
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// balancedUnitPair returns a complete pair tree of the given depth with unit
// leaves, and its type.
func balancedUnitPair(depth int) (syn.Typ, syn.IConstant) {
	if depth == 0 {
		return &syn.TUnit{}, &syn.Unit{}
	}
	firstType, first := balancedUnitPair(depth - 1)
	secondType, second := balancedUnitPair(depth - 1)
	return &syn.TPair{First: firstType, Second: secondType}, &syn.ProtoPair{
		FstType: firstType,
		SndType: secondType,
		First:   first,
		Second:  second,
	}
}

// zeroWidthListConstant returns a list of n balanced unit-pair trees of the
// given depth. When shared is true every item is the same pointer (the shape
// the FLAT decoders produce); otherwise each item is an independent tree (the
// shape main's decoder produced).
func zeroWidthListConstant(depth, n int, shared bool) *syn.Constant {
	elemType, elem := balancedUnitPair(depth)
	items := make([]syn.IConstant, n)
	for i := range items {
		if shared {
			items[i] = elem
		} else {
			_, items[i] = balancedUnitPair(depth)
		}
	}
	return &syn.Constant{Con: &syn.ProtoList{LTyp: elemType, List: items}}
}

func forced(fn builtin.DefaultFunction, forces int) syn.Term[syn.DeBruijn] {
	var t syn.Term[syn.DeBruijn] = &syn.Builtin{DefaultFunction: fn}
	for range forces {
		t = &syn.Force[syn.DeBruijn]{Term: t}
	}
	return t
}

func apply(f syn.Term[syn.DeBruijn], args ...syn.Term[syn.DeBruijn]) syn.Term[syn.DeBruijn] {
	for _, a := range args {
		f = &syn.Apply[syn.DeBruijn]{Function: f, Argument: a}
	}
	return f
}

func intConst(v int64) syn.Term[syn.DeBruijn] {
	return &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(v)}}
}

// zeroWidthProbes are programs over a list constant whose cost does not
// depend on the list's length or element size in the Plutus cost model
// (chooseList, headList, tailList, nullList, mkCons and sndPair are
// constant-cost, and every CEK step costs the same), so their consumed budget for N items
// must equal the budget for one item: that is the independent oracle.
func zeroWidthProbes(list *syn.Constant) map[string]syn.Term[syn.DeBruijn] {
	return map[string]syn.Term[syn.DeBruijn]{
		"constant": list,
		"chooseList": &syn.Force[syn.DeBruijn]{Term: apply(
			forced(builtin.ChooseList, 2),
			list,
			&syn.Delay[syn.DeBruijn]{Term: intConst(0)},
			&syn.Delay[syn.DeBruijn]{Term: intConst(1)},
		)},
		"sndPair-headList": apply(
			forced(builtin.SndPair, 2),
			apply(forced(builtin.HeadList, 1), list),
		),
		"nullList": apply(forced(builtin.NullList, 1), list),
		"tailList-headList": apply(
			forced(builtin.HeadList, 1),
			apply(forced(builtin.TailList, 1), apply(
				forced(builtin.MkCons, 1),
				apply(forced(builtin.HeadList, 1), list),
				list,
			)),
		),
	}
}

func runDecoded(
	t *testing.T,
	term syn.Term[syn.DeBruijn],
	proto uint,
) (syn.Term[syn.DeBruijn], cek.ExBudget, error) {
	t.Helper()
	input, err := syn.Encode(&syn.Program[syn.DeBruijn]{
		Version: lang.LanguageVersion{1, 1, 0},
		Term:    term,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	program, err := syn.DecodeDeBruijnForExecution(input, syn.ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  proto,
	})
	if err != nil {
		t.Fatalf("decode (%d bytes): %v", len(input), err)
	}
	ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: proto})
	machine := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
	start := machine.ExBudget
	result, runErr := machine.Run(program.Term)
	return result, start.Sub(&machine.ExBudget), runErr
}

// TestEvalDecodedZeroWidthListConstant evaluates programs whose FLAT input
// holds a long list of zero-width (unit and pair only) values. The decoder
// shares one value among those items; evaluation must see the same list a
// fully materialized decode would.
func TestEvalDecodedZeroWidthListConstant(t *testing.T) {
	// These stay below the machine's structure-validation budget
	// (maxDischargeWork), which counts every pair and pair type of every
	// item, so the larger decode-only shapes of
	// TestDecodeZeroWidthListOfPairsIsAccepted are rejected at Run.
	tests := []struct {
		depth int
		items int
	}{
		{depth: 1, items: 100_000},
		{depth: 2, items: 50_000},
		{depth: 3, items: 20_000},
	}
	for _, tc := range tests {
		probes := zeroWidthProbes(zeroWidthListConstant(tc.depth, tc.items, true))
		_, wantSnd := balancedUnitPair(tc.depth - 1)
		want := map[string]string{
			"chooseList":       "(con integer 1)",
			"sndPair-headList": syn.PrettyTerm[syn.DeBruijn](&syn.Constant{Con: wantSnd}),
		}
		for name, wantResult := range want {
			t.Run(fmt.Sprintf("depth%d/%s", tc.depth, name), func(t *testing.T) {
				result, _, err := runDecoded(t, probes[name], 10)
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				if got := syn.PrettyTerm[syn.DeBruijn](result); got != wantResult {
					t.Fatalf("result %s, want %s", got, wantResult)
				}
			})
		}
	}
}

// TestEvalLargeZeroWidthListConstant is the regression for a consensus
// divergence: a FLAT program holding a list of ~130,000 zero-width pair
// trees fits in a 16 KiB script and the reference evaluates it, but the
// pre-evaluation structure walk counted every pair (and pair type) of every
// item against a fixed node cap and rejected the program. The result and the
// consumed budget must equal those of a one-item list.
func TestEvalLargeZeroWidthListConstant(t *testing.T) {
	tests := []struct {
		name  string
		depth int
		items int
		proto uint
	}{
		// list (pair^3 unit) is 31 type tags, within the PV11 bound of 32.
		{"pair3-pv11", 3, 130_000, 11},
		{"pair3-pv10", 3, 130_000, 10},
		// Larger types are only legal before PV11.
		{"pair4-pv10", 4, 130_000, 10},
		{"pair5-pv10", 5, 130_000, 10},
		{"pair1-pv11", 1, 130_000, 11},
	}
	for _, tc := range tests {
		big := zeroWidthProbes(zeroWidthListConstant(tc.depth, tc.items, true))
		small := zeroWidthProbes(zeroWidthListConstant(tc.depth, 1, true))
		for name, term := range big {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				wantResult, wantBudget, err := runDecoded(t, small[name], tc.proto)
				if err != nil {
					t.Fatalf("one-item run: %v", err)
				}
				gotResult, gotBudget, err := runDecoded(t, term, tc.proto)
				if err != nil {
					t.Fatalf("run %d items: %v", tc.items, err)
				}
				if gotBudget != wantBudget {
					t.Fatalf("budget %+v, want %+v (one-item list)", gotBudget, wantBudget)
				}
				if name == "constant" {
					// The result is the list itself.
					if c, ok := gotResult.(*syn.Constant); !ok ||
						len(c.Con.(*syn.ProtoList).List) != tc.items {
						t.Fatalf("result is not the %d-item list", tc.items)
					}
					return
				}
				if got, want := syn.PrettyTerm[syn.DeBruijn](gotResult),
					syn.PrettyTerm[syn.DeBruijn](wantResult); got != want {
					t.Fatalf("result %s, want %s", got, want)
				}
			})
		}
	}
}

// TestEvalZeroWidthListSharedMatchesIndependent checks, on sizes the
// previous walk accepted, that a list whose items share one tree evaluates
// to the same result with the same budget as one whose items are independent
// trees (the representation before the decoder shared them).
func TestEvalZeroWidthListSharedMatchesIndependent(t *testing.T) {
	for _, depth := range []int{1, 2, 3} {
		shared := zeroWidthProbes(zeroWidthListConstant(depth, 5_000, true))
		independent := zeroWidthProbes(zeroWidthListConstant(depth, 5_000, false))
		for name := range shared {
			ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 10})
			run := func(term syn.Term[syn.DeBruijn]) (string, cek.ExBudget) {
				m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
				start := m.ExBudget
				res, err := m.Run(term)
				if err != nil {
					t.Fatalf("depth %d %s: %v", depth, name, err)
				}
				return syn.PrettyTerm[syn.DeBruijn](res), start.Sub(&m.ExBudget)
			}
			gotRes, gotBudget := run(shared[name])
			wantRes, wantBudget := run(independent[name])
			if gotRes != wantRes || gotBudget != wantBudget {
				t.Fatalf("depth %d %s: shared (%v) differs from independent (%v)", depth, name, gotBudget, wantBudget)
			}
		}
	}
}
