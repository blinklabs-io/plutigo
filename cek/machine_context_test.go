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
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

type cancelAfterChecksContext struct {
	checks   int
	limit    int
	done     chan struct{}
	canceled bool
}

func (*cancelAfterChecksContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *cancelAfterChecksContext) Done() <-chan struct{} { return c.done }

func (c *cancelAfterChecksContext) Err() error {
	c.checks++
	if c.checks >= c.limit && !c.canceled {
		close(c.done)
		c.canceled = true
	}
	if c.canceled {
		return context.Canceled
	}
	return nil
}

func (*cancelAfterChecksContext) Value(any) any { return nil }

func TestRunContextStopsOnCancellation(t *testing.T) {
	term := divergentTerm()
	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV1,
		200,
		testEvalContext(),
	)
	machine.ExBudget = ExBudget{Cpu: 1_000_000, Mem: 1_000_000}
	ctx := &cancelAfterChecksContext{limit: 64, done: make(chan struct{})}

	_, err := machine.RunContext(ctx, term)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RunContext() error = %v, want context.Canceled", err)
	}
	if ctx.checks != ctx.limit {
		t.Errorf("context checks = %d, want %d", ctx.checks, ctx.limit)
	}
	maxEnvironments := ctx.limit + 1
	if machine.lastRunEnvHighWatermark > maxEnvironments {
		t.Errorf(
			"environment high-watermark = %d, exceeds cancellation-derived cap %d",
			machine.lastRunEnvHighWatermark,
			maxEnvironments,
		)
	}
}

func TestRunContextStopsDuringTermValidation(t *testing.T) {
	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV1,
		200,
		testEvalContext(),
	)
	initial := machine.ExBudget
	ctx := &cancelAfterChecksContext{limit: 2, done: make(chan struct{})}

	_, err := machine.RunContext(
		ctx,
		&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunContext() error = %v, want context.Canceled", err)
	}
	if machine.ExBudget != initial {
		t.Fatalf("RunContext() spent budget during term validation")
	}
}

func TestDischargeStopsDuringConstantClone(t *testing.T) {
	items := make([]syn.IConstant, 64)
	for i := range items {
		items[i] = &syn.Integer{Inner: big.NewInt(int64(i))}
	}
	ctx := &cancelAfterChecksContext{limit: 3, done: make(chan struct{})}

	_, err := dischargeValueContext[syn.DeBruijn](
		ctx,
		true,
		&Constant{Constant: &syn.ProtoList{
			LTyp: &syn.TInteger{},
			List: items,
		}},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("dischargeValueContext() error = %v, want context.Canceled", err)
	}
}

func TestDischargeStopsDuringPlutusDataClone(t *testing.T) {
	items := make([]data.PlutusData, 64)
	for i := range items {
		items[i] = &data.Integer{Inner: big.NewInt(int64(i))}
	}
	ctx := &cancelAfterChecksContext{limit: 70, done: make(chan struct{})}

	_, err := dischargeValueContext[syn.DeBruijn](
		ctx,
		true,
		&Constant{Constant: &syn.Data{Inner: &data.List{Items: items}}},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("dischargeValueContext() error = %v, want context.Canceled", err)
	}
}

func TestDischargeStopsDuringByteStringClone(t *testing.T) {
	tests := []struct {
		name     string
		constant syn.IConstant
		limit    int
	}{
		{
			name:     "constant",
			constant: &syn.ByteString{Inner: make([]byte, 1024*1024)},
			limit:    7,
		},
		{
			name: "PlutusData",
			constant: &syn.Data{Inner: &data.ByteString{
				Inner: make([]byte, 1024*1024),
			}},
			limit: 10,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := &cancelAfterChecksContext{
				limit: test.limit,
				done:  make(chan struct{}),
			}
			_, err := dischargeValueContext[syn.DeBruijn](
				ctx,
				true,
				&Constant{Constant: test.constant},
			)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("dischargeValueContext() error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestRunBudgetBoundsDivergentEnvironmentGrowth(t *testing.T) {
	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV1,
		200,
		testEvalContext(),
	)
	initial := ExBudget{Cpu: 100_000_000, Mem: 100_000}
	machine.ExBudget = initial

	_, err := machine.Run(divergentTerm())
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("Run() error = %T %v, want BudgetError", err, err)
	}

	minStepMem := int64(math.MaxInt64)
	for _, cost := range machine.stepCostMem {
		if cost > 0 && cost < minStepMem {
			minStepMem = cost
		}
	}
	if minStepMem == math.MaxInt64 {
		t.Fatal("machine cost model has no positive step memory cost")
	}
	maxEnvironments := int(initial.Mem/minStepMem) + 1
	if machine.lastRunEnvHighWatermark > maxEnvironments {
		t.Fatalf(
			"environment high-watermark = %d, exceeds budget-derived cap %d",
			machine.lastRunEnvHighWatermark,
			maxEnvironments,
		)
	}
}

func divergentTerm() syn.Term[syn.DeBruijn] {
	selfApply := &syn.Lambda[syn.DeBruijn]{
		Body: &syn.Apply[syn.DeBruijn]{
			Function: &syn.Var[syn.DeBruijn]{Name: 1},
			Argument: &syn.Var[syn.DeBruijn]{Name: 1},
		},
	}
	return &syn.Apply[syn.DeBruijn]{
		Function: selfApply,
		Argument: selfApply,
	}
}
