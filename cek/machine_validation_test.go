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
