package cek

import (
	"math/big"

	"github.com/blinklabs-io/plutigo/pkg/data"
	"github.com/blinklabs-io/plutigo/pkg/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type CostModel struct {
	machineCosts MachineCosts
	builtinCosts BuiltinCosts
}

var DefaultCostModel = CostModel{
	machineCosts: DefaultMachineCosts,
	builtinCosts: DefaultBuiltinCosts,
}

const (
	PAIR_COST = 1
	CONS_COST = 3
	NIL_COST  = 1
	DATA_COST = 4
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
		case syn.Integer:
			ex = bigIntExMem(x.Inner)
		case syn.ByteString:
			ex = byteArrayExMem(x.Inner)
		case syn.Bool:
			ex = boolExMem(x.Inner)
		case syn.String:
			ex = stringExMem(x.Inner)
		case syn.Unit:
			ex = unitExMem()
		case syn.ProtoList:
			ex = listExMem(x.List)
		case syn.ProtoPair:
			ex = pairExMem(x.First, x.Second)
		default:
			panic("Oh no!")
		}

		return ex()
	}
}

// Return a function so we can have lazy
// costing of params for the case of constant functions
func bigIntExMem(i *big.Int) func() ExMem {
	return func() ExMem {
		x := big.NewInt(0)

		if x.Cmp(i) == 0 {
			return ExMem(1)
		} else {
			x := big.NewInt(0)

			x.Abs(i)

			logResult := x.BitLen() - 1

			return ExMem(logResult/64 + 1)
		}
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

// According to the Haskell code they are charging 8 bytes of value per byte contained in the string
// So returning just the string length as ExMem matches the Haskell behavior
func stringExMem(s string) func() ExMem {
	return func() ExMem {
		x := len(s)

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

func listExMem(l []syn.IConstant) func() ExMem {
	return func() ExMem {
		var accExMem ExMem

		for _, item := range l {
			accExMem += iconstantExMem(item)() + CONS_COST
		}

		return ExMem(NIL_COST + accExMem)
	}
}

func pairExMem(x syn.IConstant, y syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(PAIR_COST + iconstantExMem(x)() + iconstantExMem(y)())
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
			acc += DATA_COST
			switch dat := d.(type) {
			case data.Constr:
				costStack = append(costStack, dat.Fields...)
			case data.List:
				costStack = append(costStack, dat.Items...)
			case data.Map:
				for _, pair := range dat.Pairs {
					costStack = append(costStack, pair[0], pair[1])
				}
			case data.Integer:
				acc += bigIntExMem(dat.Inner)()
			case data.ByteString:
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
			xAcc += DATA_COST
			d := costStackX[0]
			costStackX = costStackX[1:]
			switch dat := d.(type) {
			case data.Constr:
				costStackX = append(costStackX, dat.Fields...)
			case data.List:
				costStackX = append(costStackX, dat.Items...)
			case data.Map:
				for _, pair := range dat.Pairs {
					costStackX = append(costStackX, pair[0], pair[1])
				}
			case data.Integer:
				xAcc += bigIntExMem(dat.Inner)()
			case data.ByteString:
				xAcc += byteArrayExMem(dat.Inner)()
			default:
				panic("Unreachable")
			}
		}

		if yLen {
			// Cost 4 per item switch
			yAcc += DATA_COST
			d := costStackY[0]
			costStackY = costStackY[1:]
			switch dat := d.(type) {
			case data.Constr:
				costStackY = append(costStackY, dat.Fields...)
			case data.List:
				costStackY = append(costStackY, dat.Items...)
			case data.Map:
				for _, pair := range dat.Pairs {
					costStackY = append(costStackY, pair[0], pair[1])
				}
			case data.Integer:
				yAcc += bigIntExMem(dat.Inner)()
			case data.ByteString:
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
