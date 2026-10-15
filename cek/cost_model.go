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
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"strings"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type CostModel struct {
	machineCosts MachineCosts
	builtinCosts BuiltinCosts
	caches       *builtinCostCaches
}

// Clone deliberately leaves the shared cost caches unbuilt: a clone exists to
// be modified, and its costing functions can be rewritten in place through
// the cloned pointers, which would leave a prebuilt cache stale. A machine
// built from a cache-less model resolves the costs for itself.
func (cm CostModel) Clone() CostModel {
	return CostModel{
		machineCosts: cm.machineCosts,
		builtinCosts: cm.builtinCosts.Clone(),
	}
}

var DefaultCostModel = CostModel{
	machineCosts: DefaultMachineCosts,
	builtinCosts: DefaultBuiltinCosts,
}.withCostCaches()

func costModelFromList(
	version lang.LanguageVersion,
	semantics SemanticsVariant,
	data []int64,
) (CostModel, error) {
	cm := DefaultCostModel.Clone()
	builtinCosts, err := buildBuiltinCosts(version, semantics)
	if err != nil {
		return CostModel{}, fmt.Errorf("build builtin costs: %w", err)
	}
	cm.builtinCosts = builtinCosts
	for i, param := range lang.GetParamNamesForVersion(version) {
		// A ledger that has not yet been updated for this software version
		// supplies fewer parameters than there are names. The reference
		// implementation costs every missing parameter at maxBound rather than
		// leaving a compiled-in default in place, so that a builtin the ledger
		// has not costed cannot run within any budget, and an old builtin with
		// a newly added parameter cannot be under-costed. Extra values past
		// the name list are ignored. See plutus-ledger-api
		// PlutusLedgerApi.Common.ParamName.tagWithParamNames and Note [Cost
		// model parameters from the ledger's point of view].
		value := int64(math.MaxInt64)
		if i < len(data) {
			value = data[i]
		}
		if strings.HasPrefix(param, "cek") {
			// Update machine cost
			if err := cm.machineCosts.update(param, value); err != nil {
				return cm, err
			}
		} else {
			// Update builtin cost
			if err := cm.builtinCosts.update(param, value); err != nil {
				return cm, err
			}
		}
	}
	if err := cm.machineCosts.validate(); err != nil {
		return CostModel{}, err
	}
	return cm.withCostCaches(), nil
}

func costModelFromMap(
	version lang.LanguageVersion,
	semantics SemanticsVariant,
	data map[string]int64,
) (CostModel, error) {
	cm := DefaultCostModel.Clone()
	builtinCosts, err := buildBuiltinCosts(version, semantics)
	if err != nil {
		return CostModel{}, fmt.Errorf("build builtin costs: %w", err)
	}
	cm.builtinCosts = builtinCosts
	for param, val := range data {
		if strings.HasPrefix(param, "cek") {
			// Update machine cost
			if err := cm.machineCosts.update(param, val); err != nil {
				return cm, err
			}
		} else {
			// Update builtin cost
			if err := cm.builtinCosts.update(param, val); err != nil {
				return cm, err
			}
		}
	}
	if err := cm.machineCosts.validate(); err != nil {
		return CostModel{}, err
	}
	return cm.withCostCaches(), nil
}

const (
	PairCost = 1
	ConsCost = 3
	NilCost  = 1
	DataCost = 4
)

type ExMem int

func valueExMem[T syn.Eval](v Value[T]) func() ExMem {
	return func() ExMem {
		return v.toExMem()
	}
}

func iconstantExMem(c syn.IConstant) func() ExMem {
	return func() ExMem {
		var ex func() ExMem

		switch x := c.(type) {
		case *syn.Integer:
			ex = func() ExMem {
				return ExMem(x.ExMemWords())
			}
		case *syn.ByteString:
			ex = byteArrayExMem(x.Inner)
		case *syn.Bool:
			ex = boolExMem(x.Inner)
		case *syn.String:
			ex = stringExMem(x.Inner)
		case *syn.Unit:
			ex = unitExMem()
		case *syn.ProtoList:
			ex = listExMem(x.List)
		case *syn.ProtoArray:
			ex = listLengthExMem(x.Array)
		case *syn.Value:
			ex = valueInnerCountExMem(x.Entries)
		case *syn.ProtoPair:
			ex = pairExMem(x.First, x.Second)
		case *syn.Data:
			ex = dataExMem(x.Inner)
		case *syn.Bls12_381G1Element:
			ex = blsG1ExMem()
		case *syn.Bls12_381G2Element:
			ex = blsG2ExMem()
		case *syn.Bls12_381MlResult:
			ex = blsMlResultExMem()
		default:
			panic(fmt.Sprintf("invalid constant type: %T", c))
		}

		return ex()
	}
}

// Return a function so we can have lazy
// costing of params for the case of constant functions
func sizeExMem(i int) func() ExMem {
	return func() ExMem {
		if i == 0 {
			return ExMem(0)
		}

		return ExMem(((i - 1) / 8) + 1)
	}
}

// Return a function so we can have lazy
// costing of params for the case of constant functions
func bigIntExMem(i *big.Int) func() ExMem {
	return func() ExMem {
		return bigIntExMemValue(i)
	}
}

func bigIntExMemValue(i *big.Int) ExMem {
	if i.Sign() == 0 {
		return ExMem(1)
	}
	if i.IsInt64() || i.IsUint64() {
		return ExMem(1)
	}
	return ExMem((i.BitLen()-1)/64 + 1)
}

func byteArrayExMem(b []byte) func() ExMem {
	return func() ExMem {
		return byteArrayExMemValue(b)
	}
}

func byteArrayExMemValue(b []byte) ExMem {
	length := len(b)
	if length == 0 {
		return ExMem(1)
	}
	return ExMem(((length - 1) / 8) + 1)
}

// Haskell uses T.length which returns the number of Unicode codepoints (characters),
// not the number of bytes. In Go, we use utf8.RuneCountInString to match this behavior.
// See: plutus-core/src/PlutusCore/Evaluation/Machine/ExMemoryUsage.hs
func stringExMem(s string) func() ExMem {
	return func() ExMem {
		x := utf8.RuneCountInString(s)

		return ExMem(x)
	}
}

// textByteLengthExMem is the D/E semantics metric for Text-backed builtins.
// The reference implementation stores UTF-8 text and charges one memory unit
// per four complete bytes.
func textByteLengthExMem(s string) func() ExMem {
	return func() ExMem {
		return ExMem(len(s) / 4)
	}
}

func boolExMem(bool) func() ExMem {
	return func() ExMem {
		return ExMem(1)
	}
}

func unitExMem() func() ExMem {
	return func() ExMem {
		return ExMem(1)
	}
}

func listLengthExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(len(l))
	}
}

// valueTotalSizeExMem is the ValueTotalSize metric: the number of distinct
// (policy, token) pairs, i.e. the sum of every policy's token count. A Value
// is a nested Map PolicyId (Map TokenName Quantity) and totalSize is that map's
// total entry count. Used by scaleValue and unionValue; valueContains costs
// the same count through valueInnerCountExMem.
func valueTotalSizeExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(valueTokenCount(l))
	}
}

// valueMaxDepthExMem is the ValueMaxDepth metric: the sum of the bit lengths
// (floor(log2 n) + 1, or 0 when n is 0) of the outer (policy) count and the
// largest inner (token) count. It models a two-level map
// lookup as O(log m + log k), which is why insertCoin and lookupCoin -- the
// two builtins that search a Value by policy and then by token -- are costed
// on depth rather than on size.
func valueMaxDepthExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		maxInner := valueMaxInnerCount(l)
		return ExMem(bits.Len(uint(len(l))) + bits.Len(uint(maxInner)))
	}
}

func listExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		var accExMem ExMem

		for _, item := range l {
			accExMem += iconstantExMem(item)() + ConsCost
		}

		return ExMem(NilCost + accExMem)
	}
}

func pairExMem(x syn.IConstant, y syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(PairCost + iconstantExMem(x)() + iconstantExMem(y)())
	}
}

func blsG1ExMem() func() ExMem {
	return func() ExMem {
		return ExMem(bls.SizeOfG1AffineCompressed * 3 / 8)
	}
}

func blsG2ExMem() func() ExMem {
	return func() ExMem {
		return ExMem(bls.SizeOfG2AffineCompressed * 3 / 8)
	}
}

func blsMlResultExMem() func() ExMem {
	return func() ExMem {
		return ExMem(bls.SizeOfGT / 8)
	}
}

// valueOuterCountExMem is the ValueOuterSize metric: the number of policies,
// i.e. the outer map's entry count. Used by policies.
func valueOuterCountExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(len(l))
	}
}

// valueTokenCount returns the total number of token entries across every
// policy, which is what ValueTotalSize and the valueData entry-limit check
// both need. It is the same count valueInnerCountExMem exposes; valueMaxInnerCount
// needs the per-policy counts too.
func valueTokenCount(l []syn.IConstant) int {
	innerCount := 0
	for _, item := range l {
		pair, ok := item.(*syn.ProtoPair)
		if !ok {
			continue
		}
		innerList, ok := pair.Second.(*syn.ProtoList)
		if !ok {
			continue
		}
		innerCount += len(innerList.List)
	}
	return innerCount
}

// valueMaxInnerCount returns the token count of the largest single policy,
// matching Value.maxInnerSize. A Value with no policies has an empty size
// cache, so the reference's lookupMax yields 0.
func valueMaxInnerCount(l []syn.IConstant) int {
	maxInner := 0
	for _, item := range l {
		pair, ok := item.(*syn.ProtoPair)
		if !ok {
			continue
		}
		innerList, ok := pair.Second.(*syn.ProtoList)
		if !ok {
			continue
		}
		if len(innerList.List) > maxInner {
			maxInner = len(innerList.List)
		}
	}
	return maxInner
}

// valueInnerCountExMem returns the total number of tokens across all policies.
// Used by assetCount and the valueData entry-limit check.
func valueInnerCountExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(valueTokenCount(l))
	}
}

// valueMaxCountExMem returns max(outer, inner) for value size calculation.
// Used by valueData which costs based on the larger of policy or token count.
func valueMaxCountExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		outerCount := len(l)
		innerCount := 0
		for _, item := range l {
			pair, ok := item.(*syn.ProtoPair)
			if !ok {
				continue
			}
			innerList, ok := pair.Second.(*syn.ProtoList)
			if !ok {
				continue
			}
			innerCount += len(innerList.List)
		}
		if outerCount > innerCount {
			return ExMem(outerCount)
		}
		return ExMem(innerCount)
	}
}

func dataExMem(x data.PlutusData) func() ExMem {
	return func() ExMem {
		size, err := dataExMemChecked(x, nil)
		if err != nil {
			panic(err)
		}
		return size
	}
}

type dataWalkFrame struct {
	node data.PlutusData
	next int
}

type plutusDataWalker struct {
	nextNode data.PlutusData
	hasNext  bool
}

func newPlutusDataWalker(root data.PlutusData) plutusDataWalker {
	return plutusDataWalker{
		nextNode: root,
		hasNext:  true,
	}
}

func nextDataChild(frame *dataWalkFrame) (data.PlutusData, bool) {
	switch node := frame.node.(type) {
	case *data.Constr:
		if node != nil && frame.next < len(node.Fields) {
			child := node.Fields[frame.next]
			frame.next++
			return child, true
		}
	case data.Constr:
		if frame.next < len(node.Fields) {
			child := node.Fields[frame.next]
			frame.next++
			return child, true
		}
	case *data.List:
		if node != nil && frame.next < len(node.Items) {
			child := node.Items[frame.next]
			frame.next++
			return child, true
		}
	case data.List:
		if frame.next < len(node.Items) {
			child := node.Items[frame.next]
			frame.next++
			return child, true
		}
	case *data.Map:
		if node != nil && frame.next < len(node.Pairs)*2 {
			pair := node.Pairs[frame.next/2]
			child := pair[frame.next%2]
			frame.next++
			return child, true
		}
	case data.Map:
		if frame.next < len(node.Pairs)*2 {
			pair := node.Pairs[frame.next/2]
			child := pair[frame.next%2]
			frame.next++
			return child, true
		}
	case *data.Value:
		if node != nil && node.Inner != nil && frame.next == 0 {
			frame.next++
			return node.Inner, true
		}
	case data.Value:
		if node.Inner != nil && frame.next == 0 {
			frame.next++
			return node.Inner, true
		}
	}
	return nil, false
}

func (w *plutusDataWalker) next(
	stack []dataWalkFrame,
) (data.PlutusData, []dataWalkFrame, bool) {
	if !w.hasNext {
		return nil, stack, false
	}
	node := w.nextNode
	frame := dataWalkFrame{node: node}
	if child, ok := nextDataChild(&frame); ok {
		stack = append(stack, frame)
		w.nextNode = child
		return node, stack, true
	}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if child, ok := nextDataChild(top); ok {
			w.nextNode = child
			return node, stack, true
		}
		stack = stack[:len(stack)-1]
	}
	w.hasNext = false
	return node, stack, true
}

func walkPlutusData(
	root data.PlutusData,
	visit func(data.PlutusData) error,
) error {
	var buf [32]dataWalkFrame
	walker := newPlutusDataWalker(root)
	stack := buf[:0]
	for walker.hasNext {
		var node data.PlutusData
		node, stack, _ = walker.next(stack)
		if err := visit(node); err != nil {
			return err
		}
	}
	return nil
}

func dataNodeCountExMemChecked(
	d data.PlutusData,
	check func(ExMem) error,
) (ExMem, error) {
	var count ExMem
	err := walkPlutusData(d, func(node data.PlutusData) error {
		if _, err := dataNodeExMem(node); err != nil {
			return err
		}
		count++
		if check != nil {
			return check(count)
		}
		return nil
	})
	return count, err
}

func dataExMemChecked(
	x data.PlutusData,
	check func(ExMem) error,
) (ExMem, error) {
	var acc ExMem
	err := walkPlutusData(x, func(d data.PlutusData) error {
		size, err := dataNodeExMem(d)
		if err != nil {
			return err
		}
		acc += size
		if check != nil {
			return check(acc)
		}
		return nil
	})
	return acc, err
}

// equalsDataMinExMemChecked returns min(size(x), size(y)) in the dataExMem measure.
//
// Equals Data is an exceptional case where the cost for the full traversal
// of 2 plutus data objects may far exceed what ends up being costed by the
// builtin cpu wise (it uses the minimum size). This is possible via having one
// super large object equals data with a tiny object, like a script context
// against a zero-length bytestring; fully sizing both would let a script buy
// unbounded free traversal work. Both traversals therefore advance one node
// per iteration and stop as soon as the side that has been fully counted is
// known to be the smaller, which bounds the work by about twice the smaller
// size.
//
// The result does not depend on traversal order: the loop only stops once one
// side has been counted completely and is no larger than the other's partial
// count, so the minimum is always an exact full size. Pending traversal state
// grows with nesting depth rather than container width.
func equalsDataMinExMemChecked(
	x data.PlutusData,
	y data.PlutusData,
	check func(ExMem) error,
) (ExMem, error) {
	var xAcc ExMem
	var yAcc ExMem
	var bufX, bufY [32]dataWalkFrame
	xWalker := newPlutusDataWalker(x)
	yWalker := newPlutusDataWalker(y)
	stackX := bufX[:0]
	stackY := bufY[:0]

	for (xWalker.hasNext || xAcc > yAcc) && (yWalker.hasNext || yAcc > xAcc) {
		d, nextX, ok := xWalker.next(stackX)
		stackX = nextX
		if ok {
			size, err := dataNodeExMem(d)
			if err != nil {
				return 0, err
			}
			xAcc += size
		}
		d, nextY, ok := yWalker.next(stackY)
		stackY = nextY
		if ok {
			size, err := dataNodeExMem(d)
			if err != nil {
				return 0, err
			}
			yAcc += size
		}
		if check != nil {
			if err := check(min(xAcc, yAcc)); err != nil {
				return 0, err
			}
		}
	}

	return min(xAcc, yAcc), nil
}

func dataNodeExMem(d data.PlutusData) (ExMem, error) {
	size := ExMem(DataCost)
	switch dat := d.(type) {
	case data.Constr, data.List, data.Map, data.Value:
	case *data.Constr:
		if dat == nil {
			return 0, invalidPlutusDataNode(d)
		}
	case *data.List:
		if dat == nil {
			return 0, invalidPlutusDataNode(d)
		}
	case *data.Map:
		if dat == nil {
			return 0, invalidPlutusDataNode(d)
		}
	case *data.Value:
		if dat == nil {
			return 0, invalidPlutusDataNode(d)
		}
	case data.Integer:
		if dat.Inner == nil {
			return 0, invalidPlutusDataNode(d)
		}
		size += bigIntExMemValue(dat.Inner)
	case *data.Integer:
		if dat == nil || dat.Inner == nil {
			return 0, invalidPlutusDataNode(d)
		}
		size += bigIntExMemValue(dat.Inner)
	case data.ByteString:
		size += byteArrayExMemValue(dat.Inner)
	case *data.ByteString:
		if dat == nil {
			return 0, invalidPlutusDataNode(d)
		}
		size += byteArrayExMemValue(dat.Inner)
	default:
		return 0, invalidPlutusDataNode(d)
	}
	return size, nil
}

func invalidPlutusDataNode(d data.PlutusData) *BuiltinError {
	return &BuiltinError{
		Code:    ErrCodeInvalidArgument,
		Builtin: "data sizing",
		Message: fmt.Sprintf("invalid PlutusData node %T", d),
	}
}
