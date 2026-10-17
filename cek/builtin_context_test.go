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
	"github.com/blinklabs-io/plutigo/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

func TestBLSMultiScalarMulStopsOnCancellation(t *testing.T) {
	g1Gen, g2Gen, _, _ := bls.Generators()
	tests := []struct {
		name        string
		builtinFunc builtin.DefaultFunction
		elemType    syn.Typ
		elem        syn.IConstant
	}{
		{
			name:        "G1",
			builtinFunc: builtin.Bls12_381_G1_MultiScalarMul,
			elemType:    &syn.TBls12_381G1Element{},
			elem:        &syn.Bls12_381G1Element{Inner: &g1Gen},
		},
		{
			name:        "G2",
			builtinFunc: builtin.Bls12_381_G2_MultiScalarMul,
			elemType:    &syn.TBls12_381G2Element{},
			elem:        &syn.Bls12_381G2Element{Inner: &g2Gen},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachineV4()
			ctx := &cancelAfterChecksContext{
				limit: 5,
				done:  make(chan struct{}),
			}
			m.activeContext = ctx
			t.Cleanup(func() { m.activeContext = nil })

			ints := make([]syn.IConstant, 3)
			elems := make([]syn.IConstant, 3)
			for i := range ints {
				ints[i] = &syn.Integer{Inner: big.NewInt(int64(i + 1))}
				elems[i] = tt.elem
			}
			b := newTestBuiltin(tt.builtinFunc)
			b = b.ApplyArg(&Constant{&syn.ProtoList{
				LTyp: &syn.TInteger{},
				List: ints,
			}})
			b = b.ApplyArg(&Constant{&syn.ProtoList{
				LTyp: tt.elemType,
				List: elems,
			}})

			_, err := evalBuiltinWithError(t, m, b)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("multi-scalar multiplication error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestUnionValueStopsOnCancellation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		limit int
	}{
		{name: "value conversion", limit: 1},
		{name: "merge", limit: 145},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachineV4()
			ctx := &cancelAfterChecksContext{limit: tt.limit, done: make(chan struct{})}
			m.activeContext = ctx
			t.Cleanup(func() { m.activeContext = nil })

			value := newValueCostTestInput(8, 8)
			b := newTestBuiltin(builtin.UnionValue)
			b = b.ApplyArg(value)
			b = b.ApplyArg(value)

			_, err := evalBuiltinWithError(t, m, b)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("unionValue error = %v, want context.Canceled", err)
			}
			if ctx.checks != tt.limit {
				t.Fatalf("context checks = %d, want cancellation at check %d", ctx.checks, tt.limit)
			}
		})
	}
}

func TestUnionValueStopsDuringValueConversion(t *testing.T) {
	m := newTestMachineV4()
	ctx := &cancelAfterChecksContext{limit: 3, done: make(chan struct{})}
	m.activeContext = ctx
	t.Cleanup(func() { m.activeContext = nil })

	value := mustValueConstant(t, []syn.IConstant{&syn.ProtoPair{
		First: &syn.ByteString{Inner: []byte{1}},
		Second: &syn.ProtoList{List: []syn.IConstant{
			&syn.ProtoPair{
				First:  &syn.ByteString{Inner: []byte{2}},
				Second: &syn.Integer{Inner: big.NewInt(1)},
			},
			&syn.ProtoPair{
				First:  &syn.ByteString{Inner: []byte{3}},
				Second: &syn.Integer{Inner: big.NewInt(1)},
			},
		}},
	}})
	b := newTestBuiltin(builtin.UnionValue).ApplyArg(value).ApplyArg(value)

	_, err := evalBuiltinWithError(t, m, b)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unionValue error = %v, want context.Canceled", err)
	}
	if ctx.checks != ctx.limit {
		t.Fatalf("context checks = %d, want cancellation at check %d", ctx.checks, ctx.limit)
	}
}

func TestUnionValueStopsDuringMerge(t *testing.T) {
	value := func(quantity *big.Int) *Constant {
		return mustValueConstant(t, []syn.IConstant{&syn.ProtoPair{
			First: &syn.ByteString{Inner: []byte{1}},
			Second: &syn.ProtoList{List: []syn.IConstant{&syn.ProtoPair{
				First:  &syn.ByteString{Inner: []byte{2}},
				Second: &syn.Integer{Inner: quantity},
			}}},
		}})
	}
	maxQuantity := new(big.Int).Sub(
		new(big.Int).Lsh(big.NewInt(1), 127),
		big.NewInt(1),
	)

	m := newTestMachineV4()
	ctx := &cancelAfterChecksContext{limit: 6, done: make(chan struct{})}
	m.activeContext = ctx
	t.Cleanup(func() { m.activeContext = nil })
	b := newTestBuiltin(builtin.UnionValue).
		ApplyArg(value(maxQuantity)).ApplyArg(value(big.NewInt(1)))

	_, err := evalBuiltinWithError(t, m, b)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unionValue error = %v, want context.Canceled", err)
	}
	if ctx.checks != ctx.limit {
		t.Fatalf("context checks = %d, want cancellation at check %d", ctx.checks, ctx.limit)
	}
}

func mustValueConstant(t *testing.T, entries []syn.IConstant) *Constant {
	t.Helper()
	value, err := syn.NewValue(entries)
	if err != nil {
		t.Fatalf("construct value: %v", err)
	}
	return &Constant{value}
}

// sweepBuiltinCancellation cancels the context at every successive check
// until the builtin completes, so each cancellation point must surface
// context.Canceled rather than a different error or a result.
func sweepBuiltinCancellation(t *testing.T, newBuiltin func() *Builtin[syn.DeBruijn]) {
	t.Helper()
	const maxChecks = 100000
	for limit := 1; limit <= maxChecks; limit++ {
		m := newTestMachineV4()
		ctx := &cancelAfterChecksContext{limit: limit, done: make(chan struct{})}
		m.activeContext = ctx
		_, err := evalBuiltinWithError(t, m, newBuiltin())
		m.activeContext = nil
		if err == nil {
			if limit == 1 {
				t.Fatal("builtin never checked for cancellation")
			}
			return
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelling at check %d returned %v, want context.Canceled", limit, err)
		}
	}
	t.Fatalf("builtin still cancelled after %d checks", maxChecks)
}

func TestValueBuiltinsStopAtEveryCancellationPoint(t *testing.T) {
	value := newValueCostTestInput(3, 2)
	policy := &Constant{&syn.ByteString{Inner: []byte{0, 1}}}
	token := &Constant{&syn.ByteString{Inner: []byte{0, 0, 0, 1}}}
	amount := &Constant{&syn.Integer{Inner: big.NewInt(7)}}
	tests := []struct {
		name string
		fn   builtin.DefaultFunction
		args []*Constant
	}{
		{"insertCoin existing token", builtin.InsertCoin, []*Constant{policy, token, amount, value}},
		{"insertCoin new policy", builtin.InsertCoin, []*Constant{
			{Constant: &syn.ByteString{Inner: []byte{9, 9}}}, token, amount, value,
		}},
		{"insertCoin zero removes", builtin.InsertCoin, []*Constant{
			policy, token, {Constant: &syn.Integer{Inner: big.NewInt(0)}}, value,
		}},
		{"scaleValue", builtin.ScaleValue, []*Constant{amount, value}},
		{"unionValue", builtin.UnionValue, []*Constant{value, value}},
		{"valueContains", builtin.ValueContains, []*Constant{value, value}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sweepBuiltinCancellation(t, func() *Builtin[syn.DeBruijn] {
				b := newTestBuiltin(tt.fn)
				for _, arg := range tt.args {
					b = b.ApplyArg(arg)
				}
				return b
			})
		})
	}
}

func TestBLSMultiScalarMulStopsAtEveryCancellationPoint(t *testing.T) {
	g1Gen, g2Gen, _, _ := bls.Generators()
	tests := []struct {
		name     string
		fn       builtin.DefaultFunction
		elemType syn.Typ
		elem     syn.IConstant
	}{
		{"G1", builtin.Bls12_381_G1_MultiScalarMul, &syn.TBls12_381G1Element{}, &syn.Bls12_381G1Element{Inner: &g1Gen}},
		{"G2", builtin.Bls12_381_G2_MultiScalarMul, &syn.TBls12_381G2Element{}, &syn.Bls12_381G2Element{Inner: &g2Gen}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sweepBuiltinCancellation(t, func() *Builtin[syn.DeBruijn] {
				ints := make([]syn.IConstant, 3)
				elems := make([]syn.IConstant, 3)
				for i := range ints {
					ints[i] = &syn.Integer{Inner: big.NewInt(int64(i + 1))}
					elems[i] = tt.elem
				}
				return newTestBuiltin(tt.fn).
					ApplyArg(&Constant{&syn.ProtoList{LTyp: &syn.TInteger{}, List: ints}}).
					ApplyArg(&Constant{&syn.ProtoList{LTyp: tt.elemType, List: elems}})
			})
		})
	}
}

func TestValueConstructionErrorKeepsCancellationMatchable(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		if got := valueConstructionError("construct value", err); !errors.Is(got, err) {
			t.Fatalf("valueConstructionError(%v) = %v, want it to match %v", err, got, err)
		}
	}

	cause := errors.New("invalid entry")
	got := valueConstructionError("construct value", cause)
	var internal *InternalError
	if !errors.As(got, &internal) || internal.Code != ErrCodeInternalError {
		t.Fatalf("valueConstructionError = %v, want an InternalError", got)
	}
	if internal.Message != "construct value: invalid entry" {
		t.Fatalf("message = %q", internal.Message)
	}
}
