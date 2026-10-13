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
		t.Fatalf("requested budget = %+v, want root lower bound %+v", budgetErr.Requested, want)
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
		t.Fatalf("requested budget = %+v, want root lower bound %+v", budgetErr.Requested, want)
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
	want := ExBudget{Mem: 12, Cpu: 96934}
	if budgetErr.Requested != want {
		t.Fatalf("requested budget = %+v, want root lower bound %+v", budgetErr.Requested, want)
	}
}

func TestDataSizingObservesRunContextCancellation(t *testing.T) {
	term := &syn.Apply[syn.DeBruijn]{
		Function: &syn.Builtin{DefaultFunction: builtin.SerialiseData},
		Argument: &syn.Constant{
			Con: &syn.Data{Inner: oversizedDataList(100_000)},
		},
	}
	m := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		0,
		testEvalContext(),
	)
	m.ExBudget = ExBudget{Cpu: 1 << 62, Mem: 1 << 62}
	ctx := &cancelAfterChecksContext{limit: 16, done: make(chan struct{})}

	_, err := m.RunContext(ctx, term)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunContext error = %T %v, want context.Canceled", err, err)
	}
	if ctx.checks != ctx.limit {
		t.Fatalf("context checks = %d, want %d", ctx.checks, ctx.limit)
	}
}

func TestDataWalkerKeepsWideContainerStateBounded(t *testing.T) {
	root := oversizedDataList(100_000)
	walker := newPlutusDataWalker(root)
	node, ok := walker.next()
	if !ok || node != root {
		t.Fatalf("first node = %T %v, want root list", node, ok)
	}
	if len(walker.stack) != 1 {
		t.Fatalf("pending frames = %d, want one root frame", len(walker.stack))
	}
	if walker.stack[0].next != 1 {
		t.Fatalf("root child index = %d, want 1", walker.stack[0].next)
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
				t.Fatalf("error code = %d, want %d", builtinErr.Code, ErrCodeInvalidArgument)
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
		{"node count", dataNodeCountExMem(malformed)},
		{"equals data", func() ExMem { return equalsDataMinExMem(malformed, malformed) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("malformed data returned a partial size")
				}
				if _, ok := recovered.(*BuiltinError); !ok {
					t.Fatalf("panic = %T %v, want *BuiltinError", recovered, recovered)
				}
			}()
			_ = tt.size()
		})
	}
}
