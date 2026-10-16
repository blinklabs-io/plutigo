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
		{name: "token counting", limit: 3},
		{name: "value conversion", limit: 17},
		{name: "merge", limit: 161},
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
	ctx := &cancelAfterChecksContext{limit: 5, done: make(chan struct{})}
	m.activeContext = ctx
	t.Cleanup(func() { m.activeContext = nil })

	value := &Constant{&syn.Value{Entries: []syn.IConstant{&syn.ProtoPair{
		First: &syn.ByteString{Inner: []byte{1}},
		Second: &syn.ProtoList{List: []syn.IConstant{
			&syn.ProtoPair{
				First:  &syn.ByteString{Inner: []byte{2}},
				Second: &syn.Integer{Inner: big.NewInt(1)},
			},
			&syn.ProtoPair{
				First:  &syn.ByteString{Inner: []byte{3}},
				Second: &syn.Integer{},
			},
		}},
	}}}}
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
		return &Constant{&syn.Value{Entries: []syn.IConstant{&syn.ProtoPair{
			First: &syn.ByteString{Inner: []byte{1}},
			Second: &syn.ProtoList{List: []syn.IConstant{&syn.ProtoPair{
				First:  &syn.ByteString{Inner: []byte{2}},
				Second: &syn.Integer{Inner: quantity},
			}}},
		}}}}
	}
	maxQuantity := new(big.Int).Sub(
		new(big.Int).Lsh(big.NewInt(1), 127),
		big.NewInt(1),
	)

	m := newTestMachineV4()
	ctx := &cancelAfterChecksContext{limit: 8, done: make(chan struct{})}
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
