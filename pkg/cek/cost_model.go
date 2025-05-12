package cek

import (
	"math/big"

	"github.com/blinklabs-io/plutigo/pkg/data"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type CostModel struct {
	machineCosts MachineCosts
	builtinCosts BuiltinCosts
}

var DefaultCostModel = CostModel{
	machineCosts: DefaultMachineCosts,
	builtinCosts: DefaultBuiltinCosts,
}

type ExMem int

func valueExMem[T syn.Eval](v Value[T]) func() ExMem {
	return func() ExMem {
		return v.toExMem()
	}
}

func iconstantExMem(c syn.IConstant) func() ExMem {
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

	return ex
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
			accExMem += iconstantExMem(item)() + 3
		}

		return ExMem(1 + accExMem)
	}
}

func pairExMem(x syn.IConstant, y syn.IConstant) func() ExMem {
	return func() ExMem {
		return ExMem(1 + iconstantExMem(x)() + iconstantExMem(y)())
	}
}

func dataExMem(x data.PlutusData) func() ExMem {
	return func() ExMem {
		var acc ExMem
		costStack := []data.PlutusData{
			x,
		}

		for {
			if len(costStack) == 0 {
				break
			}

			d := costStack[0]
			costStack = costStack[1:]
			// Cost 4 per item switch
			acc += 4
			switch dat := d.(type) {
			case data.Constr:
				for _, field := range dat.Fields {
					costStack = append(costStack, field)
				}
			case data.List:
				for _, item := range dat.Items {
					costStack = append(costStack, item)
				}
			case data.Map:
				for _, pair := range dat.Pairs {
					costStack = append(costStack, pair[0])
					costStack = append(costStack, pair[1])
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
