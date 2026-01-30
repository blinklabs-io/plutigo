package cek

import (
	"fmt"
	"math/big"
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
}

func (cm CostModel) Clone() CostModel {
	return CostModel{
		machineCosts: cm.machineCosts,
		builtinCosts: cm.builtinCosts.Clone(),
	}
}

var DefaultCostModel = CostModel{
	machineCosts: DefaultMachineCosts,
	builtinCosts: DefaultBuiltinCosts,
}

func costModelFromList(version lang.LanguageVersion, semantics SemanticsVariant, data []int64) (CostModel, error) {
	cm := DefaultCostModel.Clone()
	builtinCosts, err := buildBuiltinCosts(version, semantics)
	if err != nil {
		return CostModel{}, fmt.Errorf("build builtin costs: %w", err)
	}
	cm.builtinCosts = builtinCosts
	for i, param := range lang.GetParamNamesForVersion(version) {
		// Stop processing when we reach the end of our input data
		if i >= len(data) {
			break
		}
		if strings.HasPrefix(param, "cek") {
			// Update machine cost
			if err := cm.machineCosts.update(param, data[i]); err != nil {
				return cm, err
			}
		} else {
			// Update builtin cost
			if err := cm.builtinCosts.update(param, data[i]); err != nil {
				return cm, err
			}
		}
	}
	return cm, nil
}

func costModelFromMap(version lang.LanguageVersion, semantics SemanticsVariant, data map[string]int64) (CostModel, error) {
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
	return cm, nil
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
			ex = bigIntExMem(x.Inner)
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
		// Use Sign() to check for zero without allocation (avoids big.Int comparison)
		if i.Sign() == 0 {
			return ExMem(1)
		}
		// Calculate number of 64-bit words needed: (bitLength - 1) / 64 + 1
		return ExMem((i.BitLen()-1)/64 + 1)
	}
}

func byteArrayExMem(b []byte) func() ExMem {
	return func() ExMem {
		length := len(b)

		if length == 0 {
			return ExMem(1)
		} else {
			i := ((length - 1) / 8) + 1

			return ExMem(i)
		}
	}
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

// valueListSizeExMem calculates the "size" of a Value for V4 builtin costing.
// The size is: outer_count + inner_count when sum <= 2, else sum - 1.
// Used by insertCoin which needs the full count for small values.
func valueListSizeExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		if len(l) == 0 {
			return ExMem(0)
		}
		// Count outer entries (currency symbols)
		outerCount := len(l)
		// Count total inner entries (tokens)
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
		sum := outerCount + innerCount
		// For insertCoin: sum when sum <= 2, else sum - 1
		if sum > 2 {
			return ExMem(sum - 1)
		}
		return ExMem(sum)
	}
}

// valueListSizeMinusOneExMem calculates the "size" as max(0, sum - 1).
// Used by scaleValue, valueContains, valueData which use this simpler formula.
func valueListSizeMinusOneExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		if len(l) == 0 {
			return ExMem(0)
		}
		// Count outer entries (currency symbols)
		outerCount := len(l)
		// Count total inner entries (tokens)
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
		sum := outerCount + innerCount
		// For scaleValue, unionValue, valueContains: max(0, sum - 1)
		if sum > 1 {
			return ExMem(sum - 1)
		}
		return ExMem(0)
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

// valueOuterCountExMem returns just the outer list length (number of policies).
// Used by unionValue which costs based on policy count only.
func valueOuterCountExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(len(l))
	}
}

// valueInnerCountExMem returns the total number of tokens across all policies.
// Used by valueContains which costs based on token count.
func valueInnerCountExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
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
		return ExMem(innerCount)
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
				stack = append(stack, n.Fields...)
			case *data.List:
				stack = append(stack, n.Items...)
			case *data.Map:
				for _, pair := range n.Pairs {
					stack = append(stack, pair[0], pair[1])
				}
				// Integer and ByteString are leaf nodes, just counted
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

// Equals Data is an exceptional case where the cost for the full traversal
// of 2 plutus data objects may far exceed what ends up being costed by the builtin cpu wise (Uses Min Size)
// this is possible via having one super large object equals data with a tiny object
// like script context vs a Data of bytearray of 0 bytes. In this case the cpu ExBudget would far underestimate
// the cost for calculating the ExMem for the entire script context thus causing a lot of free work to be done
// by the node
func equalsDataExMem(
	x data.PlutusData,
	y data.PlutusData,
) (func() ExMem, func() ExMem) {
	var xAcc ExMem
	var yAcc ExMem
	var minAcc ExMem
	costStackX := []data.PlutusData{
		x,
	}

	costStackY := []data.PlutusData{
		y,
	}

	for xLen, yLen := true, true; (xLen || xAcc > yAcc) && (yLen || yAcc > xAcc); xLen,
		yLen = len(costStackX) != 0, len(costStackY) != 0 {
		if xLen {
			// Cost 4 per item switch
			xAcc += DataCost
			d := costStackX[0]
			costStackX = costStackX[1:]
			switch dat := d.(type) {
			case *data.Constr:
				costStackX = append(costStackX, dat.Fields...)
			case *data.List:
				costStackX = append(costStackX, dat.Items...)
			case *data.Map:
				for _, pair := range dat.Pairs {
					costStackX = append(costStackX, pair[0], pair[1])
				}
			case *data.Integer:
				xAcc += bigIntExMem(dat.Inner)()
			case *data.ByteString:
				xAcc += byteArrayExMem(dat.Inner)()
			default:
				panic("Unreachable")
			}
		}

		if yLen {
			// Cost 4 per item switch
			yAcc += DataCost
			d := costStackY[0]
			costStackY = costStackY[1:]
			switch dat := d.(type) {
			case *data.Constr:
				costStackY = append(costStackY, dat.Fields...)
			case *data.List:
				costStackY = append(costStackY, dat.Items...)
			case *data.Map:
				for _, pair := range dat.Pairs {
					costStackY = append(costStackY, pair[0], pair[1])
				}
			case *data.Integer:
				yAcc += bigIntExMem(dat.Inner)()
			case *data.ByteString:
				yAcc += byteArrayExMem(dat.Inner)()
			default:
				panic("Unreachable")
			}
		}
	}

	minAcc = min(xAcc, yAcc)

	final_func := func() ExMem {
		return minAcc
	}

	return final_func, final_func
}
