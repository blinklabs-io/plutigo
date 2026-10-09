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

	"github.com/blinklabs-io/plutigo/syn"
)

// TestDischargeValueDepthLimit verifies that discharging a pathologically deep
// result value graph returns an error instead of recursing until the Go stack
// overflows (a fatal, unrecoverable crash). Discharge happens after the
// budgeted evaluation loop, so it must self-limit.
func TestDischargeValueDepthLimit(t *testing.T) {
	var v Value[syn.DeBruijn] = &Constant{&syn.Integer{Inner: big.NewInt(0)}}
	for i := 0; i < 150000; i++ {
		v = &Constr[syn.DeBruijn]{Tag: 0, Fields: []Value[syn.DeBruijn]{v}}
	}
	_, err := dischargeValue[syn.DeBruijn](v)
	if err == nil {
		t.Fatal("expected depth-limit error discharging deep value, got nil")
	}
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError discharging deep value, got %T: %v", err, err)
	}
}

func TestUnwrapConstantPreservesMaterializationDepthLimit(t *testing.T) {
	leaf := &Constant{&syn.Integer{Inner: big.NewInt(0)}}
	var v Value[syn.DeBruijn] = leaf
	for i := 0; i < maxDischargeDepth+2; i++ {
		v = &pairValue[syn.DeBruijn]{first: v, second: leaf}
	}

	_, err := unwrapConstant[syn.DeBruijn](v)
	if err == nil {
		t.Fatal("expected depth-limit error unwrapping deep pair value, got nil")
	}
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError unwrapping deep pair value, got %T: %v", err, err)
	}
}

// TestAllocArenaSliceCapBounded verifies that arena-backed slices are returned
// with capacity equal to their length, so a future over-append reallocates
// instead of silently clobbering neighboring arena cells owned by other live
// values.
func TestAllocArenaSliceCapBounded(t *testing.T) {
	var chunks [][]int
	pos := 0

	// First allocation comes from a freshly made chunk (chunkSize 16).
	s1 := allocArenaSlice(&chunks, &pos, 3, 16)
	if cap(s1) != len(s1) {
		t.Fatalf("first alloc: expected cap == len == %d, got cap %d", len(s1), cap(s1))
	}

	// Second allocation reuses the remaining space in the same chunk.
	s2 := allocArenaSlice(&chunks, &pos, 4, 16)
	if cap(s2) != len(s2) {
		t.Fatalf("second alloc: expected cap == len == %d, got cap %d", len(s2), cap(s2))
	}
}

func TestDischargePairChargesEachValueOnce(t *testing.T) {
	leaves := &Constant{Constant: &syn.Integer{Inner: big.NewInt(0)}}
	pair := &pairValue[syn.DeBruijn]{first: leaves, second: leaves}
	budget := dischargeWorkBudget{remaining: 3}

	_, err := dischargeValueDepth[syn.DeBruijn](context.Background(), false, pair, &budget, 0)
	if err != nil {
		t.Fatalf("discharge pair with one work unit per value: %v", err)
	}
	if budget.remaining != 0 {
		t.Fatalf("expected three values to consume three work units, %d remain", budget.remaining)
	}
}

// A delayed term can capture a small shared value graph whose discharged tree
// is exponentially larger. Bound total work as well as recursion depth.
func TestDischargeValueBoundsCapturedExpansion(t *testing.T) {
	var captured Value[syn.DeBruijn] = &Constant{
		Constant: &syn.Integer{Inner: big.NewInt(0)},
	}
	for range 20 {
		captured = &Constr[syn.DeBruijn]{
			Tag:    0,
			Fields: []Value[syn.DeBruijn]{captured, captured},
		}
	}

	env := (*Env[syn.DeBruijn])(nil).Extend(captured)
	delayed := &Delay[syn.DeBruijn]{
		AST: &syn.Delay[syn.DeBruijn]{
			Term: &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
		},
		Env: env,
	}

	_, err := dischargeValue[syn.DeBruijn](delayed)
	if err == nil {
		t.Fatal("expected BudgetError for exponentially expanded captured value")
	}
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T: %v", err, err)
	}
	if budgetErr.Code != ErrCodeBudgetExhausted {
		t.Fatalf("expected budget exhausted code, got %v", budgetErr.Code)
	}
}

func TestUnwrapConstantBoundsCapturedExpansion(t *testing.T) {
	var captured Value[syn.DeBruijn] = &Constant{
		Constant: &syn.Integer{Inner: big.NewInt(0)},
	}
	for range 19 {
		captured = &pairValue[syn.DeBruijn]{first: captured, second: captured}
	}

	_, err := unwrapConstant[syn.DeBruijn](captured)
	if err == nil {
		t.Fatal("expected BudgetError for exponentially expanded captured constant")
	}
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError, got %T: %v", err, err)
	}
	if budgetErr.Code != ErrCodeBudgetExhausted {
		t.Fatalf("expected budget exhausted code, got %v", budgetErr.Code)
	}
}
