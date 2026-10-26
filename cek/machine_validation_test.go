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
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestRunMethodsRejectMalformedAST(t *testing.T) {
	typedNilString := (*syn.String)(nil)
	cyclicDelay := &syn.Delay[syn.DeBruijn]{}
	cyclicDelay.Term = cyclicDelay
	cyclicList := &syn.ProtoList{LTyp: &syn.TInteger{}}
	cyclicList.List = []syn.IConstant{cyclicList}
	cyclicType := &syn.TList{}
	cyclicType.Typ = cyclicType
	typedNilListType := (*syn.TList)(nil)
	tests := []struct {
		name string
		term syn.Term[syn.DeBruijn]
	}{
		{
			name: "builtin tag outside table",
			term: &syn.Builtin{
				DefaultFunction: builtin.MaxDefaultFunction + 1,
			},
		},
		{
			name: "typed nil constant",
			term: &syn.Constant{Con: typedNilString},
		},
		{
			name: "nil composite child",
			term: &syn.Constant{Con: &syn.ProtoList{
				LTyp: &syn.TInteger{},
				List: []syn.IConstant{nil},
			}},
		},
		{
			name: "nil PlutusData integer",
			term: &syn.Constant{Con: &syn.Data{
				Inner: &data.Integer{},
			}},
		},
		{
			name: "nil integer case tag",
			term: &syn.Case[syn.DeBruijn]{
				Constr: &syn.Constant{Con: &syn.Integer{}},
				Branches: []syn.Term[syn.DeBruijn]{
					&syn.Constant{Con: &syn.Unit{}},
				},
			},
		},
		{
			name: "nil BLS point builtin argument",
			term: &syn.Apply[syn.DeBruijn]{
				Function: &syn.Builtin{
					DefaultFunction: builtin.Bls12_381_G1_Compress,
				},
				Argument: &syn.Constant{Con: &syn.Bls12_381G1Element{}},
			},
		},
		{
			name: "typed nil composite type builtin argument",
			term: &syn.Apply[syn.DeBruijn]{
				Function: &syn.Apply[syn.DeBruijn]{
					Function: &syn.Force[syn.DeBruijn]{
						Term: &syn.Builtin{DefaultFunction: builtin.MkCons},
					},
					Argument: &syn.Constant{Con: &syn.ProtoList{
						LTyp: &syn.TInteger{},
					}},
				},
				Argument: &syn.Constant{Con: &syn.ProtoList{
					LTyp: typedNilListType,
				}},
			},
		},
		{
			name: "malformed unreachable branch",
			term: &syn.Case[syn.DeBruijn]{
				Constr: &syn.Constr[syn.DeBruijn]{Tag: 0},
				Branches: []syn.Term[syn.DeBruijn]{
					&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}},
					&syn.Builtin{DefaultFunction: builtin.MaxDefaultFunction + 1},
				},
			},
		},
		{
			name: "cyclic term",
			term: cyclicDelay,
		},
		{
			name: "cyclic constant",
			term: &syn.Constant{Con: cyclicList},
		},
		{
			name: "cyclic constant type",
			term: &syn.Constant{Con: &syn.ProtoList{LTyp: cyclicType}},
		},
	}
	runners := []struct {
		name string
		run  func(*Machine[syn.DeBruijn], syn.Term[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error)
	}{
		{
			name: "Run",
			run: func(machine *Machine[syn.DeBruijn], term syn.Term[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error) {
				return machine.Run(term)
			},
		},
		{
			name: "RunContext",
			run: func(machine *Machine[syn.DeBruijn], term syn.Term[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error) {
				return machine.RunContext(context.Background(), term)
			},
		},
	}

	for _, runner := range runners {
		t.Run(runner.name, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					_, err := runner.run(newTestMachineFlow(), test.term)
					var typeErr *TypeError
					if !errors.As(err, &typeErr) {
						t.Fatalf("error = %T %v, want TypeError", err, err)
					}
					if typeErr.Code != ErrCodeTypeMismatch {
						t.Fatalf("error code = %d, want %d", typeErr.Code, ErrCodeTypeMismatch)
					}
				})
			}
		})
	}
}

func TestRunMethodsPreserveValidASTSemantics(t *testing.T) {
	runners := []struct {
		name string
		run  func(*Machine[syn.DeBruijn], syn.Term[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error)
	}{
		{name: "Run", run: func(machine *Machine[syn.DeBruijn], term syn.Term[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error) {
			return machine.Run(term)
		}},
		{name: "RunContext", run: func(machine *Machine[syn.DeBruijn], term syn.Term[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error) {
			return machine.RunContext(context.Background(), term)
		}},
	}

	for _, runner := range runners {
		t.Run(runner.name, func(t *testing.T) {
			result, err := runner.run(
				newTestMachineFlow(),
				&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(42)}},
			)
			if err != nil {
				t.Fatalf("valid term returned error: %v", err)
			}
			constant, ok := result.(*syn.Constant)
			if !ok {
				t.Fatalf("result type = %T, want *syn.Constant", result)
			}
			integer, ok := constant.Con.(*syn.Integer)
			if !ok || integer.Inner.Cmp(big.NewInt(42)) != 0 {
				t.Fatalf("result constant = %#v, want integer 42", constant.Con)
			}
		})
	}
}

func TestRunContextDataConstantsHaveNoAggregateLimit(t *testing.T) {
	leaf := &data.Integer{Inner: big.NewInt(0)}
	fields := make([]data.PlutusData, maxDischargeWork/2)
	for i := range fields {
		fields[i] = leaf
	}
	firstData := data.Constr{Tag: big.NewInt(1), Fields: fields}
	secondData := data.Constr{Tag: big.NewInt(2), Fields: fields}
	term := syn.Term[syn.DeBruijn](&syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body: &syn.Lambda[syn.DeBruijn]{
			ParameterName: syn.DeBruijn(0),
			Body:          &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(2)},
		},
	})
	callerAppliedTerm := &syn.Apply[syn.DeBruijn]{
		Function: &syn.Apply[syn.DeBruijn]{
			Function: term,
			Argument: &syn.Constant{Con: &syn.Data{Inner: firstData}},
		},
		Argument: &syn.Constant{Con: &syn.Data{Inner: secondData}},
	}

	// Data constants have no node limit, as in the reference (a node
	// evaluates Data constants as large as a transaction holds): a tree past
	// the former aggregate limit is accepted. Leaves may be shared.
	if _, err := newTestMachineFlow().RunContext(context.Background(), callerAppliedTerm); err != nil {
		t.Fatalf("RunContext() error = %v, want success", err)
	}

	// Shared containers make a graph no decoder produces; past the work
	// limit the walk must prove a tree and rejects this one.
	shared := data.PlutusData(leaf)
	for range 40 {
		shared = data.NewList(shared, shared)
	}
	_, err := newTestMachineFlow().RunContext(context.Background(), &syn.Apply[syn.DeBruijn]{
		Function: &syn.Lambda[syn.DeBruijn]{Body: &syn.Constant{Con: &syn.Unit{}}},
		Argument: &syn.Constant{Con: &syn.Data{Inner: shared}},
	})
	var typeErr *TypeError
	if !errors.As(err, &typeErr) || !strings.Contains(typeErr.Message, "not a tree") {
		t.Fatalf("RunContext() on shared data error = %T %v, want not-a-tree TypeError", err, err)
	}

	result, err := newTestMachineFlow().RunContextWithTrustedDataArguments(
		context.Background(),
		term,
		firstData,
		secondData,
	)
	if err != nil {
		t.Fatalf("RunContextWithTrustedDataArguments() error = %v", err)
	}
	constant, ok := result.(*syn.Constant)
	if !ok {
		t.Fatalf("RunContextWithTrustedDataArguments() result = %T, want *syn.Constant", result)
	}
	resultData, ok := constant.Con.(*syn.Data)
	if !ok {
		t.Fatalf("RunContextWithTrustedDataArguments() constant = %T, want *syn.Data", constant.Con)
	}
	if !firstData.Equal(resultData.Inner) {
		t.Fatalf("RunContextWithTrustedDataArguments() returned data different from the first argument")
	}
}

func TestRunContextWithTrustedDataArgumentsRejectsNilAndMalformedInputs(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		_, err := newTestMachineFlow().RunContextWithTrustedDataArguments(
			nil,
			&syn.Constant{Con: &syn.Unit{}},
		)
		if err != nil {
			t.Fatalf("RunContextWithTrustedDataArguments() error = %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := newTestMachineFlow().RunContextWithTrustedDataArguments(
			ctx,
			&syn.Constant{Con: &syn.Unit{}},
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunContextWithTrustedDataArguments() error = %v, want context.Canceled", err)
		}
	})

	t.Run("typed nil data", func(t *testing.T) {
		var (
			constr     *data.Constr
			mapValue   *data.Map
			value      *data.Value
			integer    *data.Integer
			byteString *data.ByteString
			list       *data.List
		)
		tests := []struct {
			name  string
			value data.PlutusData
		}{
			{name: "untyped nil", value: nil},
			{name: "constructor", value: constr},
			{name: "map", value: mapValue},
			{name: "value", value: value},
			{name: "integer", value: integer},
			{name: "byte string", value: byteString},
			{name: "list", value: list},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				_, err := newTestMachineFlow().RunContextWithTrustedDataArguments(
					context.Background(),
					&syn.Constant{Con: &syn.Unit{}},
					test.value,
				)
				var typeErr *TypeError
				if !errors.As(err, &typeErr) {
					t.Fatalf("error = %T %v, want TypeError", err, err)
				}
				if !strings.Contains(typeErr.Message, "trusted data argument 0 is nil") {
					t.Fatalf("error = %q, want nil trusted data argument rejection", typeErr.Message)
				}
			})
		}
	})

	t.Run("too many arguments", func(t *testing.T) {
		arguments := make([]data.PlutusData, (maxDischargeWork-1)/3+1)
		_, err := newTestMachineFlow().RunContextWithTrustedDataArguments(
			context.Background(),
			&syn.Constant{Con: &syn.Unit{}},
			arguments...,
		)
		var typeErr *TypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("error = %T %v, want TypeError", err, err)
		}
		if !strings.Contains(typeErr.Message, "too many trusted data arguments") {
			t.Fatalf("error = %q, want trusted data argument limit rejection", typeErr.Message)
		}
	})

	t.Run("malformed program", func(t *testing.T) {
		_, err := newTestMachineFlow().RunContextWithTrustedDataArguments(
			context.Background(),
			&syn.Builtin{DefaultFunction: builtin.MaxDefaultFunction + 1},
			data.Integer{Inner: big.NewInt(0)},
		)
		var typeErr *TypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("error = %T %v, want TypeError", err, err)
		}
		if !strings.Contains(typeErr.Message, "builtin tag") {
			t.Fatalf("error = %q, want malformed program rejection", typeErr.Message)
		}
	})
}

func TestRunAcceptsValueFormPlutusData(t *testing.T) {
	tests := []struct {
		name string
		data data.PlutusData
	}{
		{name: "constructor", data: data.Constr{Tag: big.NewInt(0)}},
		{name: "map", data: data.Map{}},
		{name: "value", data: data.Value{Inner: &data.Map{}}},
		{name: "integer", data: data.Integer{Inner: big.NewInt(1)}},
		{name: "byte string", data: data.ByteString{Inner: []byte{1}}},
		{name: "list", data: data.List{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := newTestMachineFlow().Run(
				&syn.Constant{Con: &syn.Data{Inner: test.data}},
			)
			if err != nil {
				t.Fatalf("Run() returned error for value-form PlutusData: %v", err)
			}
			constant, ok := result.(*syn.Constant)
			if !ok {
				t.Fatalf("Run() result = %T, want *syn.Constant", result)
			}
			if _, ok := constant.Con.(*syn.Data); !ok {
				t.Fatalf("Run() constant = %T, want *syn.Data", constant.Con)
			}
		})
	}
}
