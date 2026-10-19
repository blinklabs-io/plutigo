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
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func oversizedDataList(size int) *data.List {
	item := data.NewInteger(big.NewInt(1))
	items := make([]data.PlutusData, size)
	for i := range items {
		items[i] = item
	}
	return &data.List{Items: items}
}

func TestSerialiseDataSizingStopsAtBudgetLowerBound(t *testing.T) {
	m := newTestMachine()
	m.ExBudget = ExBudget{}
	b := newTestBuiltin(builtin.SerialiseData).ApplyArg(
		&Constant{Constant: &syn.Data{Inner: oversizedDataList(100_000)}},
	)

	_, err := evalBuiltinWithError(t, m, b)
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("serialiseData error = %T %v, want BudgetError", err, err)
	}
	// The root list alone contributes DataCost. Its charge already exceeds an
	// empty budget, so none of its children need to be visited.
	want := ExBudget{Mem: 2 * DataCost, Cpu: 955506 + 213312*DataCost}
	if budgetErr.Requested != want {
		t.Fatalf(
			"requested budget = %+v, want root lower bound %+v",
			budgetErr.Requested,
			want,
		)
	}
}

func TestEqualsDataSizingStopsAtBudgetLowerBound(t *testing.T) {
	m := newTestMachine()
	m.ExBudget = ExBudget{}
	left := &Constant{Constant: &syn.Data{Inner: oversizedDataList(100_000)}}
	right := &Constant{Constant: &syn.Data{Inner: oversizedDataList(100_000)}}
	b := newTestBuiltin(builtin.EqualsData).ApplyArg(left).ApplyArg(right)

	_, err := evalBuiltinWithError(t, m, b)
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("equalsData error = %T %v, want BudgetError", err, err)
	}
	want := ExBudget{Mem: 1, Cpu: 898148 + 27279*DataCost}
	if budgetErr.Requested != want {
		t.Fatalf(
			"requested budget = %+v, want root lower bound %+v",
			budgetErr.Requested,
			want,
		)
	}
}

func TestUnValueDataSizingStopsAtBudgetLowerBound(t *testing.T) {
	m := newTestMachineV4()
	m.ExBudget = ExBudget{}
	b := newTestBuiltin(builtin.UnValueData).ApplyArg(
		&Constant{Constant: &syn.Data{Inner: oversizedDataList(100_000)}},
	)

	_, err := evalBuiltinWithError(t, m, b)
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("unValueData error = %T %v, want BudgetError", err, err)
	}
	want := ExBudget{Mem: 32, Cpu: 129596}
	if budgetErr.Requested != want {
		t.Fatalf(
			"requested budget = %+v, want root lower bound %+v",
			budgetErr.Requested,
			want,
		)
	}
}

func TestDataSizingObservesRunContextCancellation(t *testing.T) {
	for _, fn := range []builtin.DefaultFunction{builtin.SerialiseData, builtin.EqualsData, builtin.UnValueData} {
		t.Run(fmt.Sprint(fn), func(t *testing.T) {
			arg := &syn.Constant{
				Con: &syn.Data{Inner: oversizedDataList(100_000)},
			}
			var term syn.Term[syn.DeBruijn] = &syn.Apply[syn.DeBruijn]{Function: &syn.Builtin{DefaultFunction: fn}, Argument: arg}
			if fn == builtin.EqualsData {
				term = &syn.Apply[syn.DeBruijn]{Function: term, Argument: arg}
			}
			version := lang.LanguageVersionV3
			if fn == builtin.UnValueData {
				version = lang.LanguageVersionV4
			}
			m := NewMachine[syn.DeBruijn](version, 0, testEvalContext())
			m.ExBudget = ExBudget{Cpu: 1 << 62, Mem: 1 << 62}
			ctx := &cancelAfterChecksContext{
				limit: 16,
				done:  make(chan struct{}),
			}
			_, err := m.RunContext(ctx, term)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf(
					"RunContext error = %T %v, want context.Canceled",
					err,
					err,
				)
			}
			if ctx.checks != ctx.limit {
				t.Fatalf("context checks = %d, want %d", ctx.checks, ctx.limit)
			}
		})
	}
}

func TestDataWalkerKeepsWideContainerStateBounded(t *testing.T) {
	root := oversizedDataList(100_000)
	walker := newPlutusDataWalker(root)
	node, stack, ok := walker.next(nil)
	if !ok || node != root {
		t.Fatalf("first node = %T %v, want root list", node, ok)
	}
	if len(stack) != 1 {
		t.Fatalf("pending frames = %d, want one root frame", len(stack))
	}
	if stack[0].next != 1 {
		t.Fatalf("root child index = %d, want 1", stack[0].next)
	}
}

func TestDataSizingAcceptsValueNodes(t *testing.T) {
	tests := []struct {
		name string
		data data.PlutusData
		want ExMem
	}{
		{"constr", data.Constr{Tag: big.NewInt(0)}, DataCost},
		{"list", data.List{}, DataCost},
		{"map", data.Map{}, DataCost},
		{"value", data.Value{}, DataCost},
		{"integer", data.Integer{Inner: big.NewInt(1)}, DataCost + 1},
		{"bytes", data.ByteString{Inner: make([]byte, 9)}, DataCost + 2},
		{
			"nested",
			data.Constr{
				Tag: big.NewInt(0),
				Fields: []data.PlutusData{
					data.List{Items: []data.PlutusData{
						data.Integer{Inner: big.NewInt(1)},
						data.ByteString{Inner: make([]byte, 9)},
					}},
					data.Map{},
					data.Value{},
				},
			},
			DataCost*6 + 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dataExMem(tt.data)()
			if got != tt.want {
				t.Fatalf("data size = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDataSizingRejectsNilNodes(t *testing.T) {
	tests := []struct {
		name string
		data data.PlutusData
	}{
		{"nil interface", nil},
		{"nil integer", (*data.Integer)(nil)},
		{"integer with nil inner", &data.Integer{}},
		{"value integer with nil inner", data.Integer{}},
		{"nil bytes", (*data.ByteString)(nil)},
		{"nil list", (*data.List)(nil)},
		{
			"nested nil integer",
			data.List{Items: []data.PlutusData{(*data.Integer)(nil)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			m.ExBudget = ExBudget{Cpu: 1 << 30, Mem: 1 << 30}
			b := newTestBuiltin(builtin.SerialiseData).ApplyArg(
				&Constant{Constant: &syn.Data{Inner: tt.data}},
			)
			_, err := evalBuiltinWithError(t, m, b)
			var builtinErr *BuiltinError
			if !errors.As(err, &builtinErr) {
				t.Fatalf("error = %T %v, want BuiltinError", err, err)
			}
			if builtinErr.Code != ErrCodeInvalidArgument {
				t.Fatalf(
					"error code = %d, want %d",
					builtinErr.Code,
					ErrCodeInvalidArgument,
				)
			}
		})
	}
}

func TestConstantDataCostDoesNotEvaluateSizingThunk(t *testing.T) {
	m := newTestMachine()
	m.ExBudget = ExBudget{Cpu: 1 << 30, Mem: 1 << 30}
	b := newTestBuiltin(builtin.UnListData).ApplyArg(
		&Constant{Constant: &syn.Data{Inner: &data.List{
			Items: []data.PlutusData{(*data.Integer)(nil)},
		}}},
	)
	if _, err := evalBuiltinWithError(t, m, b); err != nil {
		t.Fatalf("constant-cost unListData evaluated its sizing thunk: %v", err)
	}
}

func TestLegacyDataSizingClosuresFailClosed(t *testing.T) {
	malformed := data.List{
		Items: []data.PlutusData{
			data.Integer{Inner: big.NewInt(1)},
			(*data.Integer)(nil),
		},
	}
	tests := []struct {
		name string
		size func() ExMem
	}{
		{"data size", dataExMem(malformed)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("malformed data returned a partial size")
				}
				if _, ok := recovered.(*BuiltinError); !ok {
					t.Fatalf(
						"panic = %T %v, want *BuiltinError",
						recovered,
						recovered,
					)
				}
			}()
			_ = tt.size()
		})
	}
}

func BenchmarkEqualsDataSizing(b *testing.B) {
	for _, n := range []int{2, 2000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			m := newTestMachine()
			arg := &Constant{
				Constant: &syn.Data{
					Inner: &data.Constr{
						Tag:    big.NewInt(0),
						Fields: []data.PlutusData{oversizedDataList(n)},
					},
				},
			}
			fn := newTestBuiltin(builtin.EqualsData).ApplyArg(arg).ApplyArg(arg)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				m.ExBudget = ExBudget{Cpu: 1 << 62, Mem: 1 << 62}
				if _, err := m.evalBuiltinApp(fn); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestDataSizingBudgetBoundary(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fn    builtin.DefaultFunction
		input data.PlutusData
		cost  ExBudget
		v4    bool
	}{
		{"serialiseData root", builtin.SerialiseData, &data.List{}, ExBudget{Mem: 8, Cpu: 1808754}, false},
		{"equalsData root", builtin.EqualsData, &data.List{}, ExBudget{Mem: 1, Cpu: 1007264}, false},
		{"unValueData root", builtin.UnValueData, &data.Map{}, ExBudget{Mem: 32, Cpu: 129596}, true},
		{"serialiseData list", builtin.SerialiseData, oversizedDataList(128), ExBudget{Mem: 1288, Cpu: 138328434}, false},
		{"equalsData list", builtin.EqualsData, oversizedDataList(128), ExBudget{Mem: 1, Cpu: 18465824}, false},
		{"unValueData map", builtin.UnValueData, unValueDataInput(unValueDataPolicy(nil, unValueDataToken(nil, big.NewInt(1)))), ExBudget{Mem: 104, Cpu: 644000}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, short := range []int64{0, 1} {
				t.Run(fmt.Sprintf("cpu short %d", short), func(t *testing.T) {
					m := newTestMachine()
					if tc.v4 {
						m = newTestMachineV4()
					}
					m.ExBudget = tc.cost
					m.ExBudget.Cpu -= short
					arg := &Constant{Constant: &syn.Data{Inner: tc.input}}
					fn := newTestBuiltin(tc.fn).ApplyArg(arg)
					if tc.fn == builtin.EqualsData {
						fn = fn.ApplyArg(arg)
					}
					_, err := m.evalBuiltinApp(fn)
					if short == 0 {
						if err != nil {
							t.Fatalf("exact budget: %v", err)
						}
						if m.ExBudget != (ExBudget{}) {
							t.Fatalf(
								"remaining budget = %+v, want zero",
								m.ExBudget,
							)
						}
					} else {
						var budgetErr *BudgetError
						if !errors.As(err, &budgetErr) {
							t.Fatalf("one CPU short: %T %v, want BudgetError", err, err)
						}
					}
				})
			}
		})
	}
}
