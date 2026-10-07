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

// dataNodeCountExMem counts the number of nodes in a Data structure.
// Used by unValueData which costs based on node count.
func dataNodeCountExMem(d data.PlutusData) func() ExMem {
	return func() ExMem {
		count := 0
		stack := []data.PlutusData{d}
		for len(stack) > 0 {
			node := stack[0]
			stack = stack[1:]
			count++
			switch n := node.(type) {
			case *data.Constr:
				if n != nil {
					stack = append(stack, n.Fields...)
				}
			case *data.List:
				if n != nil {
					stack = append(stack, n.Items...)
				}
			case *data.Map:
				if n == nil {
					continue
				}
				for _, pair := range n.Pairs {
					stack = append(stack, pair[0], pair[1])
				}
			case *data.Value:
				if n != nil && n.Inner != nil {
					stack = append(stack, n.Inner)
				}
			}
		}
		return ExMem(count)
	}
}

func dataExMem(x data.PlutusData) func() ExMem {
	return func() ExMem {
		var acc ExMem
		costStack := []data.PlutusData{
			x,
		}

		for len(costStack) != 0 {
			d := costStack[0]
			costStack = costStack[1:]
			// Cost 4 per item switch
			acc += DataCost
			switch dat := d.(type) {
			case *data.Constr:
				costStack = append(costStack, dat.Fields...)
			case *data.List:
				costStack = append(costStack, dat.Items...)
			case *data.Map:
				for _, pair := range dat.Pairs {
					costStack = append(costStack, pair[0], pair[1])
				}
			case *data.Value:
				if dat.Inner != nil {
					costStack = append(costStack, dat.Inner)
				}
			case *data.Integer:
				acc += bigIntExMem(dat.Inner)()
			case *data.ByteString:
				acc += byteArrayExMem(dat.Inner)()
			default:
				panic("Unreachable")
			}
		}

		return acc
	}
}

// equalsDataMinExMem returns min(size(x), size(y)) in the dataExMem measure.
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
// count, so the minimum is always an exact full size. That lets the pending
// nodes live on LIFO stacks whose backing arrays start on the goroutine stack.
func equalsDataMinExMem(x data.PlutusData, y data.PlutusData) ExMem {
	var xAcc ExMem
	var yAcc ExMem
	var bufX, bufY [32]data.PlutusData
	costStackX := append(bufX[:0], x)
	costStackY := append(bufY[:0], y)

	for xLen, yLen := true, true; (xLen || xAcc > yAcc) && (yLen || yAcc > xAcc); xLen,
		yLen = len(costStackX) != 0, len(costStackY) != 0 {
		if xLen {
			last := len(costStackX) - 1
			d := costStackX[last]
			costStackX = costStackX[:last]
			var size ExMem
			size, costStackX = equalsDataNodeExMem(d, costStackX)
			xAcc += size
		}

		if yLen {
			last := len(costStackY) - 1
			d := costStackY[last]
			costStackY = costStackY[:last]
			var size ExMem
			size, costStackY = equalsDataNodeExMem(d, costStackY)
			yAcc += size
		}
	}

	return min(xAcc, yAcc)
}

// equalsDataNodeExMem costs one node in the dataExMem measure and pushes its
// children onto stack.
func equalsDataNodeExMem(
	d data.PlutusData,
	stack []data.PlutusData,
) (ExMem, []data.PlutusData) {
	// Cost 4 per item switch
	size := ExMem(DataCost)
	switch dat := d.(type) {
	case *data.Constr:
		stack = append(stack, dat.Fields...)
	case *data.List:
		stack = append(stack, dat.Items...)
	case *data.Map:
		for _, pair := range dat.Pairs {
			stack = append(stack, pair[0], pair[1])
		}
	case *data.Value:
		if dat.Inner != nil {
			stack = append(stack, dat.Inner)
		}
	case *data.Integer:
		size += bigIntExMemValue(dat.Inner)
	case *data.ByteString:
		size += byteArrayExMemValue(dat.Inner)
	default:
		panic("Unreachable")
	}
	return size, stack
}
