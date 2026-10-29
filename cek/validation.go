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
	term syn.Term[T]
	// depth and scope are 32-bit to keep the item at 24 bytes: the depth
	// is bounded by maxDischargeDepth and the scope by the depth. scope is
	// the number of lambdas enclosing term, or noScopeCheck when term sits
	// in a part of the program the scope check does not cover.
	depth int32
	scope int32
}

// noScopeCheck marks terms whose variables the pre-evaluation scope check
// does not inspect (see validateTermStructure).
const noScopeCheck int32 = -1

type constantValidationItem struct {
	constant syn.IConstant
	depth    int
	// exit marks the point where every descendant of constant has been
	// walked (see structureValidationBudget.seenComposites).
	exit bool
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
	// seenComposites records each list, array and pair constant already
	// walked, with the shallowest depth it was walked at, and seenTypes each
	// composite constant type already walked. Constants and types are
	// immutable, so a node reached again through another reference has
	// already been validated: the FLAT decoders share one value among all
	// occurrences of a zero-width pair type, and a list of N such items is N
	// references to one tree. Walking each shared node once keeps the walk
	// linear in the physical (decoded) size, which is what the node cap
	// bounds, rather than charging the logical size of every reference: the
	// reference evaluates such constants, so they must not be rejected.
	seenComposites map[syn.IConstant]compositeVisit
	seenTypes      map[syn.Typ]struct{}
}

type compositeVisit struct {
	depth int
	done  bool
}

// enterConstant reports whether the composite constant c must be walked at
// depth: false when a completed walk already covered it at the same or a
// shallower depth (its subtree then fits within the depth bound from here
// too). Reaching c again while its own walk is in progress means the
// constant graph is cyclic, which no decoder produces. When c must be
// walked, enterConstant schedules the exit item that completes the walk
// once all of c's children, pushed after it, have been popped.
func (b *structureValidationBudget) enterConstant(
	work *[]constantValidationItem,
	c syn.IConstant,
	depth int,
) (bool, error) {
	if prev, ok := b.seenComposites[c]; ok {
		if !prev.done {
			return false, malformedASTError("constant structure is cyclic")
		}
		if prev.depth <= depth {
			return false, nil
		}
	}
	if b.seenComposites == nil {
		b.seenComposites = make(map[syn.IConstant]compositeVisit)
	}
	b.seenComposites[c] = compositeVisit{depth: depth}
	*work = append(*work, constantValidationItem{constant: c, depth: depth, exit: true})
	return true, nil
}

// leaveConstant records that the walk of c, entered at depth, is complete.
func (b *structureValidationBudget) leaveConstant(c syn.IConstant, depth int) {
	b.seenComposites[c] = compositeVisit{depth: depth, done: true}
}

// firstVisitType reports whether the constant type t must be walked. The
// type walk restarts its depth count at every root, so a type walked once is
// valid wherever it occurs. Only composite types are recorded.
func (b *structureValidationBudget) firstVisitType(t syn.Typ) bool {
	switch t.(type) {
	case *syn.TList, *syn.TArray, *syn.TPair:
	default:
		return true
	}
	if _, ok := b.seenTypes[t]; ok {
		return false
	}
	if b.seenTypes == nil {
		b.seenTypes = make(map[syn.Typ]struct{})
	}
	b.seenTypes[t] = struct{}{}
	return true
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

func (b *structureValidationBudget) consumeTerms(count int) error {
	if count > b.remaining {
		return malformedASTError("term structure exceeds %d nodes", maxDischargeWork)
	}
	b.remaining -= count
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

// validateTermStructure checks that term is a well-formed, acyclic AST and
// applies the reference's pre-evaluation scope check (plutus-ledger-api
// mkTermToEvaluate, UntypedPlutusCore.Check.Scope.checkScope): every
// variable must be bound by an enclosing lambda. Before protocol version 12
// the reference checks in NoCaseConstr mode, which does not look inside
// constr fields or case scrutinees and branches; a free variable there fails
// only if the machine evaluates it. fullScopeCheck selects the Full mode of
// protocol version 12 on.
func validateTermStructure[T syn.Eval](
	ctx context.Context,
	checkCancellation bool,
	term syn.Term[T],
	trustedData map[*syn.Data]struct{},
	fullScopeCheck bool,
) error {
	budget := structureValidationBudget{
		ctx:               ctx,
		checkCancellation: checkCancellation,
		remaining:         maxDischargeWork,
		trustedData:       trustedData,
	}
	var initialWork [16]termValidationItem[T]
	work := initialWork[:0]
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
	visitItem:
		if item.depth > maxDischargeDepth {
			return malformedASTError("term structure exceeds depth %d", maxDischargeDepth)
		}

		nextDepth := item.depth + 1
		switch t := item.term.(type) {
		case *syn.Var[T]:
			if t == nil {
				return malformedASTError("term contains a nil variable")
			}
			// A DeBruijn index must be bound by an enclosing lambda, wherever
			// the variable sits: the reference rejects an open term before
			// evaluating it, even when the free variable is never reached.
			if index, ok := any(t.Name).(syn.DeBruijn); ok &&
				item.scope != noScopeCheck &&
				(index < 1 || int(index) > int(item.scope)) {
				return &TypeError{
					Code:    ErrCodeOpenTerm,
					Message: fmt.Sprintf("open term: DeBruijn index %d at scope %d", index, item.scope),
				}
			}
		case *syn.Delay[T]:
			if t == nil {
				return malformedASTError("term contains a nil delay")
			}
			if err := budget.consume("term"); err != nil {
				return err
			}
			item.term = t.Term
			item.depth = nextDepth
			goto visitItem
		case *syn.Force[T]:
			if t == nil {
				return malformedASTError("term contains a nil force")
			}
			if err := budget.consume("term"); err != nil {
				return err
			}
			item.term = t.Term
			item.depth = nextDepth
			goto visitItem
		case *syn.Lambda[T]:
			if t == nil {
				return malformedASTError("term contains a nil lambda")
			}
			if err := budget.consume("term"); err != nil {
				return err
			}
			item.term = t.Body
			item.depth = nextDepth
			item.scope = lambdaScope(item.scope)
			goto visitItem
		case *syn.Apply[T]:
			if t == nil {
				return malformedASTError("term contains a nil application")
			}
			if budget.checkCancellation || budget.discharge != nil {
				if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Function, depth: nextDepth, scope: item.scope}); err != nil {
					return err
				}
				if err := budget.consume("term"); err != nil {
					return err
				}
			} else {
				if err := budget.consumeTerms(2); err != nil {
					return err
				}
				work = append(work, termValidationItem[T]{term: t.Function, depth: nextDepth, scope: item.scope})
			}
			item.term = t.Argument
			item.depth = nextDepth
			goto visitItem
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
			scope := caseConstrScope(item.scope, fullScopeCheck)
			if budget.checkCancellation || budget.discharge != nil {
				for _, field := range t.Fields {
					if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: field, depth: nextDepth, scope: scope}); err != nil {
						return err
					}
				}
			} else {
				if err := budget.consumeTerms(len(t.Fields)); err != nil {
					return err
				}
				for _, field := range t.Fields {
					work = append(work, termValidationItem[T]{term: field, depth: nextDepth, scope: scope})
				}
			}
		case *syn.Case[T]:
			if t == nil {
				return malformedASTError("term contains a nil case")
			}
			scope := caseConstrScope(item.scope, fullScopeCheck)
			if budget.checkCancellation || budget.discharge != nil {
				if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: t.Constr, depth: nextDepth, scope: scope}); err != nil {
					return err
				}
				for _, branch := range t.Branches {
					if err := scheduleValidationItem(&budget, "term", &work, termValidationItem[T]{term: branch, depth: nextDepth, scope: scope}); err != nil {
						return err
					}
				}
			} else {
				if err := budget.consumeTerms(1 + len(t.Branches)); err != nil {
					return err
				}
				work = append(work, termValidationItem[T]{term: t.Constr, depth: nextDepth, scope: scope})
				for _, branch := range t.Branches {
					work = append(work, termValidationItem[T]{term: branch, depth: nextDepth, scope: scope})
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

// lambdaScope is the scope of a lambda body whose lambda has scope.
func lambdaScope(scope int32) int32 {
	if scope == noScopeCheck {
		return noScopeCheck
	}
	return scope + 1
}

// caseConstrScope is the scope of the subterms of a constr or case term
// with scope: unchecked unless the Full scope-check mode applies.
func caseConstrScope(scope int32, fullScopeCheck bool) int32 {
	if fullScopeCheck {
		return scope
	}
	return noScopeCheck
}

func validateConstantStructureWithBudget(
	constant syn.IConstant,
	budget *structureValidationBudget,
) error {
	switch c := constant.(type) {
	case *syn.Integer:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil || c.Inner == nil {
			return malformedASTError("constant contains a nil Integer")
		}
		return budget.consumeScalarBytes(integerMagnitudeBytes(c.Inner))
	case *syn.ByteString:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil {
			return malformedASTError("constant contains a nil ByteString")
		}
		return budget.consumeScalarBytes(int64(len(c.Inner)))
	case *syn.String:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil {
			return malformedASTError("constant contains a nil String")
		}
		return budget.consumeScalarBytes(int64(len(c.Inner)))
	case *syn.Unit:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil {
			return malformedASTError("constant contains a nil Unit")
		}
		return nil
	case *syn.Bool:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil {
			return malformedASTError("constant contains a nil Bool")
		}
		return nil
	case *syn.Value:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil {
			return malformedASTError("constant contains a nil Value")
		}
		if _, _, _, ok := c.Sizes(); !ok {
			return malformedASTError("constant contains an uninitialized Value")
		}
		return nil
	case *syn.Bls12_381G1Element:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil || c.Inner == nil {
			return malformedASTError("constant contains a nil BLS G1 element")
		}
		return nil
	case *syn.Bls12_381G2Element:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil || c.Inner == nil {
			return malformedASTError("constant contains a nil BLS G2 element")
		}
		return nil
	case *syn.Bls12_381MlResult:
		if err := budget.consume("constant"); err != nil {
			return err
		}
		if c == nil || c.Inner == nil {
			return malformedASTError("constant contains a nil BLS Miller loop result")
		}
		return nil
	}

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
		if item.exit {
			budget.leaveConstant(item.constant, item.depth)
			continue
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
			if walk, err := budget.enterConstant(&work, c, item.depth); !walk {
				if err != nil {
					return err
				}
				continue
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
			if walk, err := budget.enterConstant(&work, c, item.depth); !walk {
				if err != nil {
					return err
				}
				continue
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
			if walk, err := budget.enterConstant(&work, c, item.depth); !walk {
				if err != nil {
					return err
				}
				continue
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
	if !budget.firstVisitType(value) {
		return nil
	}
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
	if budget.discharge == nil {
		return validateDataConstant(value, budget)
	}
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

// validateDataConstant checks a PlutusData constant of a program before
// evaluation: every node must be non-nil and of a known type. Unlike the
// term walk it has no depth or node limit, because the reference has none:
// a Cardano node evaluates Data constants, datums and redeemers nested and
// sized as far as the transaction allows. The walk is iterative, so depth
// is safe, and visits each node once, so it is linear in the decoded input.
// Only a value built by hand can make a walk longer than its memory: once a
// constant exceeds maxDischargeWork nodes, data.CheckTree confirms it is a
// tree (no shared or cyclic children), which every decoded value is.
func validateDataConstant(value data.PlutusData, budget *structureValidationBudget) error {
	var stackBuf [16]data.PlutusData
	work := append(stackBuf[:0], value)
	nodes := 0
	treeChecked := false
	for len(work) > 0 {
		last := len(work) - 1
		item := work[last]
		work[last] = nil
		work = work[:last]
		nodes++
		if nodes%4096 == 0 && budget.checkCancellation {
			if err := budget.ctx.Err(); err != nil {
				return err
			}
		}
		switch d := item.(type) {
		case *data.Constr:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData constructor")
			}
			work = append(work, d.Fields...)
		case data.Constr:
			work = append(work, d.Fields...)
		case *data.Map:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData map")
			}
			work = appendDataPairs(work, d.Pairs)
		case data.Map:
			work = appendDataPairs(work, d.Pairs)
		case *data.Value:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData Value")
			}
			if d.Inner != nil {
				work = append(work, d.Inner)
			}
		case data.Value:
			if d.Inner != nil {
				work = append(work, d.Inner)
			}
		case *data.Integer:
			if d == nil || d.Inner == nil {
				return malformedASTError("constant contains a nil PlutusData Integer")
			}
		case data.Integer:
			if d.Inner == nil {
				return malformedASTError("constant contains a nil PlutusData Integer")
			}
		case *data.ByteString:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData ByteString")
			}
		case data.ByteString:
		case *data.List:
			if d == nil {
				return malformedASTError("constant contains a nil PlutusData list")
			}
			work = append(work, d.Items...)
		case data.List:
			work = append(work, d.Items...)
		default:
			return malformedASTError("unsupported PlutusData type %T", item)
		}
		// Visited plus pending nodes bound the walk of a tree; checked before
		// they can exceed the work limit, so shared children cannot grow the
		// pending stack beyond it either.
		if !treeChecked && nodes+len(work) > maxDischargeWork {
			if err := data.CheckTree(value); err != nil {
				return malformedASTError("PlutusData structure is not a tree: %v", err)
			}
			treeChecked = true
		}
	}
	return nil
}

func appendDataPairs(work []data.PlutusData, pairs [][2]data.PlutusData) []data.PlutusData {
	for _, pair := range pairs {
		work = append(work, pair[0], pair[1])
	}
	return work
}
