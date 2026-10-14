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
	"fmt"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
)

type termValidationItem[T syn.Eval] struct {
	term  syn.Term[T]
	depth int
}

type constantValidationItem struct {
	constant syn.IConstant
	depth    int
}

type dataValidationItem struct {
	value data.PlutusData
	depth int
}

type typeValidationItem struct {
	value syn.Typ
	depth int
}

type structureValidationBudget struct {
	ctx               context.Context
	checkCancellation bool
	remaining         int
}

func (b *structureValidationBudget) consume(kind string) error {
	if b.checkCancellation {
		if err := b.ctx.Err(); err != nil {
			return err
		}
	}
	if b.remaining == 0 {
		return malformedASTError("%s structure exceeds %d nodes", kind, maxDischargeWork)
	}
	b.remaining--
	return nil
}

func malformedASTError(format string, args ...any) *TypeError {
	return &TypeError{
		Code:    ErrCodeTypeMismatch,
		Message: fmt.Sprintf(format, args...),
	}
}

func validateTermStructure[T syn.Eval](
	ctx context.Context,
	checkCancellation bool,
	term syn.Term[T],
) error {
	work := []termValidationItem[T]{{term: term}}
	budget := structureValidationBudget{
		ctx:               ctx,
		checkCancellation: checkCancellation,
		remaining:         maxDischargeWork,
	}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if err := budget.consume("term"); err != nil {
			return err
		}
		if item.depth > maxDischargeDepth {
			return malformedASTError("term structure exceeds depth %d", maxDischargeDepth)
		}

		nextDepth := item.depth + 1
		switch t := item.term.(type) {
		case *syn.Var[T]:
			if t == nil {
				return malformedASTError("term contains a nil variable")
			}
		case *syn.Delay[T]:
			if t == nil {
				return malformedASTError("term contains a nil delay")
			}
			work = append(work, termValidationItem[T]{term: t.Term, depth: nextDepth})
		case *syn.Force[T]:
			if t == nil {
				return malformedASTError("term contains a nil force")
			}
			work = append(work, termValidationItem[T]{term: t.Term, depth: nextDepth})
		case *syn.Lambda[T]:
			if t == nil {
				return malformedASTError("term contains a nil lambda")
			}
			work = append(work, termValidationItem[T]{term: t.Body, depth: nextDepth})
		case *syn.Apply[T]:
			if t == nil {
				return malformedASTError("term contains a nil application")
			}
			work = append(
				work,
				termValidationItem[T]{term: t.Function, depth: nextDepth},
				termValidationItem[T]{term: t.Argument, depth: nextDepth},
			)
		case *syn.Builtin:
			if t == nil {
				return malformedASTError("term contains a nil builtin")
			}
			if t.DefaultFunction > builtin.MaxDefaultFunction {
				return malformedASTError("builtin tag %d is invalid", t.DefaultFunction)
			}
		case *syn.Constr[T]:
			if t == nil {
				return malformedASTError("term contains a nil constructor")
			}
			for _, field := range t.Fields {
				work = append(work, termValidationItem[T]{term: field, depth: nextDepth})
			}
		case *syn.Case[T]:
			if t == nil {
				return malformedASTError("term contains a nil case")
			}
			work = append(work, termValidationItem[T]{term: t.Constr, depth: nextDepth})
			for _, branch := range t.Branches {
				work = append(work, termValidationItem[T]{term: branch, depth: nextDepth})
			}
		case *syn.Error:
			if t == nil {
				return malformedASTError("term contains a nil error node")
			}
		case *syn.Constant:
			if t == nil {
				return malformedASTError("term contains a nil constant node")
			}
			if err := validateConstantStructureWithBudget(t.Con, &budget); err != nil {
				return err
			}
		default:
			return malformedASTError("unsupported term type %T", item.term)
		}
	}
	return nil
}

func validateConstantStructureWithBudget(
	constant syn.IConstant,
	budget *structureValidationBudget,
) error {
	work := []constantValidationItem{{constant: constant}}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if item.depth > maxDischargeDepth {
			return malformedASTError("constant structure exceeds depth %d", maxDischargeDepth)
		}

		nextDepth := item.depth + 1
		switch c := item.constant.(type) {
		case *syn.Integer:
			if c == nil || c.Inner == nil {
				return malformedASTError("constant contains a nil Integer")
			}
		case *syn.ByteString:
			if c == nil {
				return malformedASTError("constant contains a nil ByteString")
			}
		case *syn.String:
			if c == nil {
				return malformedASTError("constant contains a nil String")
			}
		case *syn.Unit:
			if c == nil {
				return malformedASTError("constant contains a nil Unit")
			}
		case *syn.Bool:
			if c == nil {
				return malformedASTError("constant contains a nil Bool")
			}
		case *syn.ProtoList:
			if c == nil {
				return malformedASTError("constant contains a nil ProtoList")
			}
			if err := validateTypeStructureWithBudget(c.LTyp, budget); err != nil {
				return err
			}
			for _, child := range c.List {
				work = append(work, constantValidationItem{constant: child, depth: nextDepth})
			}
		case *syn.ProtoArray:
			if c == nil {
				return malformedASTError("constant contains a nil ProtoArray")
			}
			if err := validateTypeStructureWithBudget(c.ATyp, budget); err != nil {
				return err
			}
			for _, child := range c.Array {
				work = append(work, constantValidationItem{constant: child, depth: nextDepth})
			}
		case *syn.Value:
			if c == nil {
				return malformedASTError("constant contains a nil Value")
			}
			for _, child := range c.Entries {
				work = append(work, constantValidationItem{constant: child, depth: nextDepth})
			}
		case *syn.ProtoPair:
			if c == nil {
				return malformedASTError("constant contains a nil ProtoPair")
			}
			if err := validateTypeStructureWithBudget(c.FstType, budget); err != nil {
				return err
			}
			if err := validateTypeStructureWithBudget(c.SndType, budget); err != nil {
				return err
			}
			work = append(
				work,
				constantValidationItem{constant: c.First, depth: nextDepth},
				constantValidationItem{constant: c.Second, depth: nextDepth},
			)
		case *syn.Data:
			if c == nil {
				return malformedASTError("constant contains a nil Data")
			}
			if err := validateDataStructureWithBudget(c.Inner, budget); err != nil {
				return err
			}
		case *syn.Bls12_381G1Element:
			if c == nil || c.Inner == nil {
				return malformedASTError("constant contains a nil BLS G1 element")
			}
		case *syn.Bls12_381G2Element:
			if c == nil || c.Inner == nil {
				return malformedASTError("constant contains a nil BLS G2 element")
			}
		case *syn.Bls12_381MlResult:
			if c == nil || c.Inner == nil {
				return malformedASTError("constant contains a nil BLS Miller loop result")
			}
		default:
			return malformedASTError("unsupported constant type %T", item.constant)
		}
	}
	return nil
}

func validateTypeStructureWithBudget(
	value syn.Typ,
	budget *structureValidationBudget,
) error {
	work := []typeValidationItem{{value: value}}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if err := budget.consume("constant type"); err != nil {
			return err
		}
		if item.depth > maxDischargeDepth {
			return malformedASTError("constant type structure exceeds depth %d", maxDischargeDepth)
		}

		nextDepth := item.depth + 1
		switch typ := item.value.(type) {
		case *syn.TBool:
			if typ == nil {
				return malformedASTError("constant contains a nil Bool type")
			}
		case *syn.TInteger:
			if typ == nil {
				return malformedASTError("constant contains a nil Integer type")
			}
		case *syn.TString:
			if typ == nil {
				return malformedASTError("constant contains a nil String type")
			}
		case *syn.TByteString:
			if typ == nil {
				return malformedASTError("constant contains a nil ByteString type")
			}
		case *syn.TUnit:
			if typ == nil {
				return malformedASTError("constant contains a nil Unit type")
			}
		case *syn.TData:
			if typ == nil {
				return malformedASTError("constant contains a nil Data type")
			}
		case *syn.TBls12_381G1Element:
			if typ == nil {
				return malformedASTError("constant contains a nil BLS G1 type")
			}
		case *syn.TBls12_381G2Element:
			if typ == nil {
				return malformedASTError("constant contains a nil BLS G2 type")
			}
		case *syn.TBls12_381MlResult:
			if typ == nil {
				return malformedASTError("constant contains a nil BLS Miller loop type")
			}
		case *syn.TValue:
			if typ == nil {
				return malformedASTError("constant contains a nil Value type")
			}
		case *syn.TList:
			if typ == nil {
				return malformedASTError("constant contains a nil List type")
			}
			work = append(work, typeValidationItem{value: typ.Typ, depth: nextDepth})
		case *syn.TArray:
			if typ == nil {
				return malformedASTError("constant contains a nil Array type")
			}
			work = append(work, typeValidationItem{value: typ.Typ, depth: nextDepth})
		case *syn.TPair:
			if typ == nil {
				return malformedASTError("constant contains a nil Pair type")
			}
			work = append(
				work,
				typeValidationItem{value: typ.First, depth: nextDepth},
				typeValidationItem{value: typ.Second, depth: nextDepth},
			)
		default:
			return malformedASTError("unsupported constant type metadata %T", item.value)
		}
	}
	return nil
}

func validateDataStructure(value data.PlutusData) error {
	budget := structureValidationBudget{
		ctx:       context.Background(),
		remaining: maxDischargeWork,
	}
	return validateDataStructureWithBudget(value, &budget)
}

func validateDataStructureWithBudget(
	value data.PlutusData,
	budget *structureValidationBudget,
) error {
	work := []dataValidationItem{{value: value}}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if err := budget.consume("PlutusData"); err != nil {
			return err
		}
		if item.depth > maxDischargeDepth {
			return malformedASTError("PlutusData structure exceeds depth %d", maxDischargeDepth)
		}

		nextDepth := item.depth + 1
		switch d := item.value.(type) {
		case *data.Constr:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData constructor")
			}
			for _, child := range d.Fields {
				work = append(work, dataValidationItem{value: child, depth: nextDepth})
			}
		case *data.Map:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData map")
			}
			for _, pair := range d.Pairs {
				work = append(
					work,
					dataValidationItem{value: pair[0], depth: nextDepth},
					dataValidationItem{value: pair[1], depth: nextDepth},
				)
			}
		case *data.Value:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData Value")
			}
			if d.Inner != nil {
				work = append(work, dataValidationItem{value: d.Inner, depth: nextDepth})
			}
		case *data.Integer:
			if d == nil || d.Inner == nil {
				return malformedASTError("constant contains a nil PlutusData Integer")
			}
		case *data.ByteString:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData ByteString")
			}
		case *data.List:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData list")
			}
			for _, child := range d.Items {
				work = append(work, dataValidationItem{value: child, depth: nextDepth})
			}
		default:
			return malformedASTError("unsupported PlutusData type %T", item.value)
		}
	}
	return nil
}
