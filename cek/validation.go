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
	"math/big"

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
	trustedData       map[*syn.Data]struct{}
	discharge         *dischargeWorkBudget
	credits           int
	// Machine-created materializations derive their types from values that
	// were validated before evaluation.
	skipConstantTypes bool
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
	if b.discharge != nil {
		if b.credits > 0 {
			b.credits--
		} else if err := b.discharge.consume(1); err != nil {
			return err
		}
	}
	b.remaining--
	return nil
}

func (b *structureValidationBudget) consumeScalarBytes(size int64) error {
	if b.discharge == nil {
		return nil
	}
	return b.discharge.consumeScalarBytes(size)
}

func integerMagnitudeBytes(value *big.Int) int64 {
	bits := value.BitLen()
	bytes := bits / 8
	if bits%8 != 0 {
		bytes++
	}
	return int64(bytes)
}

func scheduleValidationItem[T any](
	budget *structureValidationBudget,
	kind string,
	work *[]T,
	item T,
) error {
	if err := budget.consume(kind); err != nil {
		return err
	}
	*work = append(*work, item)
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
	trustedData map[*syn.Data]struct{},
) error {
	budget := structureValidationBudget{
		ctx:               ctx,
		checkCancellation: checkCancellation,
		remaining:         maxDischargeWork,
		trustedData:       trustedData,
	}
	work := make([]termValidationItem[T], 0, 1)
	if err := scheduleValidationItem(
		&budget,
		"term",
		&work,
		termValidationItem[T]{term: term},
	); err != nil {
		return err
	}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
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
			if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Term, depth: nextDepth}); err != nil {
				return err
			}
		case *syn.Force[T]:
			if t == nil {
				return malformedASTError("term contains a nil force")
			}
			if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Term, depth: nextDepth}); err != nil {
				return err
			}
		case *syn.Lambda[T]:
			if t == nil {
				return malformedASTError("term contains a nil lambda")
			}
			if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Body, depth: nextDepth}); err != nil {
				return err
			}
		case *syn.Apply[T]:
			if t == nil {
				return malformedASTError("term contains a nil application")
			}
			if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Function, depth: nextDepth}); err != nil {
				return err
			}
			if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Argument, depth: nextDepth}); err != nil {
				return err
			}
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
				if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: field, depth: nextDepth}); err != nil {
					return err
				}
			}
		case *syn.Case[T]:
			if t == nil {
				return malformedASTError("term contains a nil case")
			}
			if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Constr, depth: nextDepth}); err != nil {
				return err
			}
			for _, branch := range t.Branches {
				if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: branch, depth: nextDepth}); err != nil {
					return err
				}
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
	work := make([]constantValidationItem, 0, 1)
	if err := scheduleValidationItem(
		budget,
		"constant",
		&work,
		constantValidationItem{constant: constant},
	); err != nil {
		return err
	}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if item.depth > maxDischargeDepth {
			return malformedASTError("constant structure exceeds depth %d", maxDischargeDepth)
		}

		nextDepth := item.depth + 1
		switch c := item.constant.(type) {
		case *syn.Integer:
			if c == nil || c.Inner == nil {
				return malformedASTError("constant contains a nil Integer")
			}
			if err := budget.consumeScalarBytes(integerMagnitudeBytes(c.Inner)); err != nil {
				return err
			}
		case *syn.ByteString:
			if c == nil {
				return malformedASTError("constant contains a nil ByteString")
			}
			if err := budget.consumeScalarBytes(int64(len(c.Inner))); err != nil {
				return err
			}
		case *syn.String:
			if c == nil {
				return malformedASTError("constant contains a nil String")
			}
			if err := budget.consumeScalarBytes(int64(len(c.Inner))); err != nil {
				return err
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
			if !budget.skipConstantTypes {
				if err := validateTypeStructureWithBudget(c.LTyp, budget); err != nil {
					return err
				}
			}
			for _, child := range c.List {
				if err := scheduleValidationItem(budget, "constant", &work, constantValidationItem{constant: child, depth: nextDepth}); err != nil {
					return err
				}
			}
		case *syn.ProtoArray:
			if c == nil {
				return malformedASTError("constant contains a nil ProtoArray")
			}
			if !budget.skipConstantTypes {
				if err := validateTypeStructureWithBudget(c.ATyp, budget); err != nil {
					return err
				}
			}
			for _, child := range c.Array {
				if err := scheduleValidationItem(budget, "constant", &work, constantValidationItem{constant: child, depth: nextDepth}); err != nil {
					return err
				}
			}
		case *syn.Value:
			if c == nil {
				return malformedASTError("constant contains a nil Value")
			}
			if _, _, _, ok := c.Sizes(); !ok {
				return malformedASTError("constant contains an uninitialized Value")
			}
		case *syn.ProtoPair:
			if c == nil {
				return malformedASTError("constant contains a nil ProtoPair")
			}
			if !budget.skipConstantTypes {
				if err := validateTypeStructureWithBudget(c.FstType, budget); err != nil {
					return err
				}
				if err := validateTypeStructureWithBudget(c.SndType, budget); err != nil {
					return err
				}
			}
			if err := scheduleValidationItem(budget, "constant", &work, constantValidationItem{constant: c.First, depth: nextDepth}); err != nil {
				return err
			}
			if err := scheduleValidationItem(budget, "constant", &work, constantValidationItem{constant: c.Second, depth: nextDepth}); err != nil {
				return err
			}
		case *syn.Data:
			if c == nil {
				return malformedASTError("constant contains a nil Data")
			}
			if _, ok := budget.trustedData[c]; ok {
				continue
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
	work := make([]typeValidationItem, 0, 1)
	if err := scheduleValidationItem(
		budget,
		"constant type",
		&work,
		typeValidationItem{value: value},
	); err != nil {
		return err
	}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
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
			if err := scheduleValidationItem(budget, "constant type", &work, typeValidationItem{value: typ.Typ, depth: nextDepth}); err != nil {
				return err
			}
		case *syn.TArray:
			if typ == nil {
				return malformedASTError("constant contains a nil Array type")
			}
			if err := scheduleValidationItem(budget, "constant type", &work, typeValidationItem{value: typ.Typ, depth: nextDepth}); err != nil {
				return err
			}
		case *syn.TPair:
			if typ == nil {
				return malformedASTError("constant contains a nil Pair type")
			}
			if err := scheduleValidationItem(budget, "constant type", &work, typeValidationItem{value: typ.First, depth: nextDepth}); err != nil {
				return err
			}
			if err := scheduleValidationItem(budget, "constant type", &work, typeValidationItem{value: typ.Second, depth: nextDepth}); err != nil {
				return err
			}
		default:
			return malformedASTError("unsupported constant type metadata %T", item.value)
		}
	}
	return nil
}

func validateDataStructureWithBudget(
	value data.PlutusData,
	budget *structureValidationBudget,
) error {
	work := make([]dataValidationItem, 0, 1)
	if err := scheduleValidationItem(
		budget,
		"PlutusData",
		&work,
		dataValidationItem{value: value},
	); err != nil {
		return err
	}
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work = work[:last]
		if item.depth > maxDischargeDepth {
			return malformedASTError("PlutusData structure exceeds depth %d", maxDischargeDepth)
		}

		nextDepth := item.depth + 1
		switch d := item.value.(type) {
		case *data.Constr, data.Constr:
			var fields []data.PlutusData
			var tag *big.Int
			switch value := d.(type) {
			case *data.Constr:
				if value == nil {
					return malformedASTError("constant contains a nil PlutusData constructor")
				}
				fields = value.Fields
				if value.Tag != nil {
					tag = value.Tag
				}
			case data.Constr:
				fields = value.Fields
				if value.Tag != nil {
					tag = value.Tag
				}
			}
			if tag != nil {
				if err := budget.consumeScalarBytes(integerMagnitudeBytes(tag)); err != nil {
					return err
				}
			}
			for _, child := range fields {
				if err := scheduleValidationItem(budget, "PlutusData", &work, dataValidationItem{value: child, depth: nextDepth}); err != nil {
					return err
				}
			}
		case *data.Map, data.Map:
			var pairs [][2]data.PlutusData
			switch value := d.(type) {
			case *data.Map:
				if value == nil {
					return malformedASTError("constant contains a nil PlutusData map")
				}
				pairs = value.Pairs
			case data.Map:
				pairs = value.Pairs
			}
			for _, pair := range pairs {
				if err := scheduleValidationItem(budget, "PlutusData", &work, dataValidationItem{value: pair[0], depth: nextDepth}); err != nil {
					return err
				}
				if err := scheduleValidationItem(budget, "PlutusData", &work, dataValidationItem{value: pair[1], depth: nextDepth}); err != nil {
					return err
				}
			}
		case *data.Value, data.Value:
			var inner *data.Map
			switch value := d.(type) {
			case *data.Value:
				if value == nil {
					return malformedASTError("constant contains a nil PlutusData Value")
				}
				inner = value.Inner
			case data.Value:
				inner = value.Inner
			}
			if inner != nil {
				if err := scheduleValidationItem(budget, "PlutusData", &work, dataValidationItem{value: inner, depth: nextDepth}); err != nil {
					return err
				}
			}
		case *data.Integer:
			if d == nil || d.Inner == nil {
				return malformedASTError("constant contains a nil PlutusData Integer")
			}
			if err := budget.consumeScalarBytes(integerMagnitudeBytes(d.Inner)); err != nil {
				return err
			}
		case data.Integer:
			if d.Inner == nil {
				return malformedASTError("constant contains a nil PlutusData Integer")
			}
			if err := budget.consumeScalarBytes(integerMagnitudeBytes(d.Inner)); err != nil {
				return err
			}
		case *data.ByteString:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData ByteString")
			}
			if err := budget.consumeScalarBytes(int64(len(d.Inner))); err != nil {
				return err
			}
		case data.ByteString:
			if err := budget.consumeScalarBytes(int64(len(d.Inner))); err != nil {
				return err
			}
		case *data.List, data.List:
			var items []data.PlutusData
			switch value := d.(type) {
			case *data.List:
				if value == nil {
					return malformedASTError("constant contains a nil PlutusData list")
				}
				items = value.Items
			case data.List:
				items = value.Items
			}
			for _, child := range items {
				if err := scheduleValidationItem(budget, "PlutusData", &work, dataValidationItem{value: child, depth: nextDepth}); err != nil {
					return err
				}
			}
		default:
			return malformedASTError("unsupported PlutusData type %T", item.value)
		}
	}
	return nil
}
