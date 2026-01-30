package cek

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/jinzhu/copier"
)

type BuiltinCosts [builtin.TotalBuiltinCount]*CostingFunc[Arguments]

func (b *BuiltinCosts) Clone() BuiltinCosts {
	var ret BuiltinCosts
	if err := copier.CopyWithOption(&ret, b, copier.Option{DeepCopy: true}); err != nil {
		panic(fmt.Sprintf("failure cloning BuiltinCosts: %s", err))
	}
	return ret
}

func (b *BuiltinCosts) update(param string, val int64) error {
	paramParts := strings.Split(param, "-")
	if len(paramParts) < 3 {
		return errors.New("invalid param format: " + param)
	}
	builtinName := paramParts[0]
	// Remap some builtin names that changed over time
	switch builtinName {
	case "verifySignature":
		builtinName = "verifyEd25519Signature"
	case "blake2b":
		builtinName = "blake2b_256"
	}
	builtinIdx, ok := builtin.Builtins[builtinName]
	if !ok {
		return errors.New("unknown builtin: " + builtinName)
	}
	builtinCost := b[builtinIdx]
	if builtinCost == nil {
		return errors.New("no existing cost info for builtin: " + builtinName)
	}
	var args Arguments
	switch paramParts[1] {
	case "cpu":
		args = builtinCost.cpu
	case "mem", "memory":
		args = builtinCost.mem
	default:
		return fmt.Errorf("unknown cost subkey for builtin %s: %s", builtinName, paramParts[1])
	}
	if args == nil {
		return fmt.Errorf("no existing cost info for builtin %s with arg: %s", builtinName, paramParts[1])
	}

	// Determine the parameter name index based on pattern
	// Format can be:
	//   builtin-cpu/memory-arguments (3 parts)
	//   builtin-cpu/memory-arguments-param (4 parts)
	//   builtin-cpu/memory-arguments-model-arguments-param (6 parts)
	paramIdx := 3
	if len(paramParts) > 5 && paramParts[3] == "model" && paramParts[4] == "arguments" {
		paramIdx = 5
	}

	// For 3-part parameter names (builtin-cpu/memory-arguments), only ConstantCost is valid
	if len(paramParts) == 3 {
		if constCost, ok := args.(*ConstantCost); ok {
			constCost.c = val
			return nil
		} else {
			// For non-ConstantCost types with 3-part names, throw an error
			// This can happen when genesis file format differs from code expectations
			return errors.New("cannot map parameter name to costing info: " + param)
		}
	}

	switch a := args.(type) {
	case *AddedSizesModel:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ConstAboveDiagonalIntoQuadraticXAndYModel:
		switch paramParts[paramIdx] {
		case "constant":
			a.constant = val
		case "minimum":
			a.minimum = val
		case "coeff00", "c00":
			a.coeff00 = val
		case "coeff10", "c10":
			a.coeff10 = val
		case "coeff01", "c01":
			a.coeff01 = val
		case "coeff20", "c20":
			a.coeff20 = val
		case "coeff11", "c11":
			a.coeff11 = val
		case "coeff02", "c02":
			a.coeff02 = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ConstAboveDiagonalModel:
		if len(paramParts) > 5 && paramParts[3] == "model" && paramParts[4] == "arguments" {
			// Accessing the nested model (e.g., divideInteger-cpu-arguments-model-arguments-intercept)
			if model, ok := a.model.(*MultipliedSizesModel); ok {
				switch paramParts[5] {
				case "intercept":
					model.intercept = val
				case "slope":
					model.slope = val
				default:
					return fmt.Errorf("unknown model param for builtin %s: %s", builtinName, paramParts[5])
				}
			} else {
				return errors.New("unexpected model type for builtin " + builtinName)
			}
		} else {
			switch paramParts[paramIdx] {
			case "constant":
				a.constant = val
			default:
				return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
			}
		}
	case *ConstBelowDiagonalModel:
		if len(paramParts) > 5 && paramParts[3] == "model" && paramParts[4] == "arguments" {
			// Accessing the nested model (e.g., modInteger-cpu-arguments-model-arguments-intercept)
			if model, ok := a.model.(*MultipliedSizesModel); ok {
				switch paramParts[5] {
				case "intercept":
					model.intercept = val
				case "slope":
					model.slope = val
				default:
					return fmt.Errorf("unknown model param for builtin %s: %s", builtinName, paramParts[5])
				}
			} else {
				return errors.New("unexpected model type for builtin " + builtinName)
			}
		} else {
			switch paramParts[paramIdx] {
			case "constant":
				a.constant = val
			default:
				return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
			}
		}
	case *ConstantCost:
		switch paramParts[paramIdx] {
		case "c":
			a.c = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ExpMod:
		switch paramParts[paramIdx] {
		case "coeff00":
			a.coeff00 = val
		case "coeff11":
			a.coeff11 = val
		case "coeff12":
			a.coeff12 = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *LinearCost:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *LinearInXAndY:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope1":
			a.slope1 = val
		case "slope2":
			a.slope2 = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *LinearInX:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *LinearInY:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *LinearOnDiagonalModel:
		switch paramParts[paramIdx] {
		case "constant":
			a.constant = val
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *MaxSizeModel:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *MinSizeModel:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *MultipliedSizesModel:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *QuadraticInYModel:
		switch paramParts[paramIdx] {
		case "coeff0", "c0":
			a.coeff0 = val
		case "coeff1", "c1":
			a.coeff1 = val
		case "coeff2", "c2":
			a.coeff2 = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *SubtractedSizesModel:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		case "minimum":
			a.minimum = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeAddedSizesModel:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeLinearInMaxYZ:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeLinearInX:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeLinearInYandZ:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope1":
			a.slope1 = val
		case "slope2":
			a.slope2 = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeLinearInY:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeLinearInZ:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeLiteralInYorLinearInZ:
		switch paramParts[paramIdx] {
		case "intercept":
			a.intercept = val
		case "slope":
			a.slope = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	case *ThreeQuadraticInZ:
		switch paramParts[paramIdx] {
		case "coeff0", "c0":
			a.coeff0 = val
		case "coeff1", "c1":
			a.coeff1 = val
		case "coeff2", "c2":
			a.coeff2 = val
		default:
			return fmt.Errorf("unknown cost param for builtin %s: %s", builtinName, paramParts[paramIdx])
		}
	default:
		return fmt.Errorf("unknown cost type for builtin %s: %T", builtinName, args)
	}
	return nil
}

var DefaultBuiltinCosts = BuiltinCosts{
	builtin.AddInteger: &CostingFunc[Arguments]{
		mem: &MaxSizeModel{MaxSize{
			intercept: 1,
			slope:     1,
		}},
		cpu: &MaxSizeModel{MaxSize{
			intercept: 100788,
			slope:     420,
		}},
	},
	builtin.SubtractInteger: &CostingFunc[Arguments]{
		mem: &MaxSizeModel{MaxSize{
			intercept: 1,
			slope:     1,
		}},
		cpu: &MaxSizeModel{MaxSize{
			intercept: 100788,
			slope:     420,
		}},
	},
	builtin.MultiplyInteger: &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 0,
			slope:     1,
		}},
		cpu: &MultipliedSizesModel{MultipliedSizes{
			intercept: 90434,
			slope:     519,
		}},
	},
	builtin.DivideInteger: &CostingFunc[Arguments]{
		mem: &SubtractedSizesModel{SubtractedSizes{
			intercept: 0,
			slope:     1,
			minimum:   1,
		}},
		cpu: &ConstAboveDiagonalIntoQuadraticXAndYModel{
			constant: 85848,
			TwoArgumentsQuadraticFunction: TwoArgumentsQuadraticFunction{
				minimum: 85848,
				coeff00: 123203,
				coeff01: 7305,
				coeff02: -900,
				coeff10: 1716,
				coeff11: 549,
				coeff20: 57,
			},
		},
	},
	builtin.QuotientInteger: &CostingFunc[Arguments]{
		mem: &SubtractedSizesModel{SubtractedSizes{
			intercept: 0,
			slope:     1,
			minimum:   1,
		}},
		cpu: &ConstAboveDiagonalIntoQuadraticXAndYModel{
			85848,
			TwoArgumentsQuadraticFunction{
				minimum: 85848,
				coeff00: 123203,
				coeff01: 7305,
				coeff02: -900,
				coeff10: 1716,
				coeff11: 549,
				coeff20: 57,
			},
		},
	},
	builtin.RemainderInteger: &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ConstAboveDiagonalIntoQuadraticXAndYModel{
			constant: 85848,
			TwoArgumentsQuadraticFunction: TwoArgumentsQuadraticFunction{
				minimum: 85848,
				coeff00: 123203,
				coeff01: 7305,
				coeff02: -900,
				coeff10: 1716,
				coeff11: 549,
				coeff20: 57,
			},
		},
	},
	builtin.ModInteger: &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ConstAboveDiagonalIntoQuadraticXAndYModel{
			constant: 85848,
			TwoArgumentsQuadraticFunction: TwoArgumentsQuadraticFunction{
				minimum: 85848,
				coeff00: 123203,
				coeff01: 7305,
				coeff02: -900,
				coeff10: 1716,
				coeff11: 549,
				coeff20: 57,
			},
		},
	},
	builtin.EqualsInteger: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &MinSizeModel{MinSize{
			intercept: 51775,
			slope:     558,
		}},
	},
	builtin.LessThanInteger: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &MinSizeModel{MinSize{
			intercept: 44749,
			slope:     541,
		}},
	},
	builtin.LessThanEqualsInteger: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &MinSizeModel{MinSize{
			intercept: 43285,
			slope:     552,
		}},
	},
	// ByteString functions
	builtin.AppendByteString: &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 0,
			slope:     1,
		}},
		cpu: &AddedSizesModel{AddedSizes{
			intercept: 1000,
			slope:     173,
		}},
	},
	builtin.ConsByteString: &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 0,
			slope:     1,
		}},
		cpu: &LinearInY{LinearCost{
			intercept: 72010,
			slope:     178,
		}},
	},
	builtin.SliceByteString: &CostingFunc[Arguments]{
		mem: &ThreeLinearInZ{LinearCost{
			intercept: 4,
			slope:     0,
		}},
		cpu: &ThreeLinearInZ{LinearCost{
			intercept: 20467,
			slope:     1,
		}},
	},
	builtin.LengthOfByteString: &CostingFunc[Arguments]{
		mem: &ConstantCost{10},
		cpu: &ConstantCost{22100},
	},
	builtin.IndexByteString: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{13169},
	},
	builtin.EqualsByteString: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &LinearOnDiagonalModel{ConstantOrLinear{
			constant:  24548,
			intercept: 29498,
			slope:     38,
		}},
	},
	builtin.LessThanByteString: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &MinSizeModel{MinSize{
			intercept: 28999,
			slope:     74,
		}},
	},
	builtin.LessThanEqualsByteString: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &MinSizeModel{MinSize{
			intercept: 28999,
			slope:     74,
		}},
	},
	// Cryptography and hash functions
	builtin.Sha2_256: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &LinearCost{
			intercept: 270652,
			slope:     22588,
		},
	},
	builtin.Sha3_256: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &LinearCost{
			intercept: 1457325,
			slope:     64566,
		},
	},
	builtin.Blake2b_256: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &LinearInX{LinearCost{
			intercept: 201305,
			slope:     8356,
		}},
	},
	builtin.Blake2b_224: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &LinearCost{
			intercept: 207616,
			slope:     8310,
		},
	},
	builtin.Keccak_256: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &LinearCost{
			intercept: 2261318,
			slope:     64571,
		},
	},
	builtin.VerifyEd25519Signature: &CostingFunc[Arguments]{
		mem: &ConstantCost{10},
		cpu: &ThreeLinearInY{LinearCost{
			intercept: 53384111,
			slope:     14333,
		}},
	},
	builtin.VerifyEcdsaSecp256k1Signature: &CostingFunc[Arguments]{
		mem: &ConstantCost{10},
		cpu: &ConstantCost{43053543},
	},
	builtin.VerifySchnorrSecp256k1Signature: &CostingFunc[Arguments]{
		mem: &ConstantCost{10},
		cpu: &ThreeLinearInY{LinearCost{
			intercept: 43574283,
			slope:     26308,
		}},
	},
	// String functions
	builtin.AppendString: &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 4,
			slope:     1,
		}},
		cpu: &AddedSizesModel{AddedSizes{
			intercept: 1000,
			slope:     59957,
		}},
	},
	builtin.EqualsString: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &LinearOnDiagonalModel{ConstantOrLinear{
			constant:  39184,
			intercept: 1000,
			slope:     60594,
		}},
	},
	builtin.EncodeUtf8: &CostingFunc[Arguments]{
		mem: &LinearCost{
			intercept: 4,
			slope:     2,
		},
		cpu: &LinearCost{
			intercept: 1000,
			slope:     42921,
		},
	},
	builtin.DecodeUtf8: &CostingFunc[Arguments]{
		mem: &LinearCost{
			intercept: 4,
			slope:     2,
		},
		cpu: &LinearCost{
			intercept: 91189,
			slope:     769,
		},
	},
	// Bool function
	builtin.IfThenElse: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{76049},
	},
	// Unit function
	builtin.ChooseUnit: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{61462},
	},
	// Tracing function
	builtin.Trace: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{59498},
	},
	// Pairs functions
	builtin.FstPair: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{141895},
	},
	builtin.SndPair: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{141992},
	},
	// List functions
	builtin.ChooseList: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{132994},
	},
	builtin.MkCons: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{72362},
	},
	builtin.HeadList: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{83150},
	},
	builtin.TailList: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{81663},
	},
	builtin.NullList: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{74433},
	},
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	builtin.ChooseData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{94375},
	},
	builtin.ConstrData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{22151},
	},
	builtin.MapData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{68246},
	},
	builtin.ListData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{33852},
	},
	builtin.IData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{15299},
	},
	builtin.BData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{11183},
	},
	builtin.UnConstrData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{24588},
	},
	builtin.UnMapData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{24623},
	},
	builtin.UnListData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{25933},
	},
	builtin.UnIData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{20744},
	},
	builtin.UnBData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{20142},
	},
	builtin.EqualsData: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &MinSizeModel{MinSize{
			intercept: 898148,
			slope:     27279,
		}},
	},
	builtin.SerialiseData: &CostingFunc[Arguments]{
		mem: &LinearCost{
			intercept: 0,
			slope:     2,
		},
		cpu: &LinearCost{
			intercept: 955506,
			slope:     213312,
		},
	},
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	builtin.MkPairData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{11546},
	},
	builtin.MkNilData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{7243},
	},
	builtin.MkNilPairData: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{7391},
	},
	builtin.Bls12_381_G1_Add: &CostingFunc[Arguments]{
		mem: &ConstantCost{18},
		cpu: &ConstantCost{962335},
	},
	builtin.Bls12_381_G1_Neg: &CostingFunc[Arguments]{
		mem: &ConstantCost{18},
		cpu: &ConstantCost{267929},
	},
	builtin.Bls12_381_G1_ScalarMul: &CostingFunc[Arguments]{
		mem: &ConstantCost{18},
		cpu: &LinearInX{LinearCost{
			intercept: 76433006,
			slope:     8868,
		}},
	},
	builtin.Bls12_381_G1_Equal: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{442008},
	},
	builtin.Bls12_381_G1_Compress: &CostingFunc[Arguments]{
		mem: &ConstantCost{6},
		cpu: &ConstantCost{2780678},
	},
	builtin.Bls12_381_G1_Uncompress: &CostingFunc[Arguments]{
		mem: &ConstantCost{18},
		cpu: &ConstantCost{52948122},
	},
	builtin.Bls12_381_G1_HashToGroup: &CostingFunc[Arguments]{
		mem: &ConstantCost{18},
		cpu: &LinearInX{LinearCost{
			intercept: 52538055,
			slope:     3756,
		}},
	},
	builtin.Bls12_381_G2_Add: &CostingFunc[Arguments]{
		mem: &ConstantCost{36},
		cpu: &ConstantCost{1995836},
	},
	builtin.Bls12_381_G2_Neg: &CostingFunc[Arguments]{
		mem: &ConstantCost{36},
		cpu: &ConstantCost{284546},
	},
	builtin.Bls12_381_G2_ScalarMul: &CostingFunc[Arguments]{
		mem: &ConstantCost{36},
		cpu: &LinearInX{LinearCost{
			intercept: 158221314,
			slope:     26549,
		}},
	},
	builtin.Bls12_381_G1_MultiScalarMul: &CostingFunc[Arguments]{
		mem: &ConstantCost{18},
		cpu: &LinearInX{LinearCost{
			intercept: 321837444,
			slope:     25087669,
		}},
	},
	builtin.Bls12_381_G2_MultiScalarMul: &CostingFunc[Arguments]{
		mem: &ConstantCost{36},
		cpu: &LinearInX{LinearCost{
			intercept: 617887431,
			slope:     67302824,
		}},
	},
	builtin.Bls12_381_G2_Equal: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{901022},
	},
	builtin.Bls12_381_G2_Compress: &CostingFunc[Arguments]{
		mem: &ConstantCost{12},
		cpu: &ConstantCost{3227919},
	},
	builtin.Bls12_381_G2_Uncompress: &CostingFunc[Arguments]{
		mem: &ConstantCost{36},
		cpu: &ConstantCost{74698472},
	},
	builtin.Bls12_381_G2_HashToGroup: &CostingFunc[Arguments]{
		mem: &ConstantCost{36},
		cpu: &LinearInX{LinearCost{
			intercept: 166917843,
			slope:     4307,
		}},
	},
	builtin.Bls12_381_MillerLoop: &CostingFunc[Arguments]{
		mem: &ConstantCost{72},
		cpu: &ConstantCost{254006273},
	},
	builtin.Bls12_381_MulMlResult: &CostingFunc[Arguments]{
		mem: &ConstantCost{72},
		cpu: &ConstantCost{2174038},
	},
	builtin.Bls12_381_FinalVerify: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{333849714},
	},
	builtin.IntegerToByteString: &CostingFunc[Arguments]{
		mem: &ThreeLiteralInYorLinearInZ{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ThreeQuadraticInZ{QuadraticFunction{
			coeff0: 1293828,
			coeff1: 28716,
			coeff2: 63,
		}},
	},
	builtin.ByteStringToInteger: &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &QuadraticInYModel{QuadraticFunction{
			coeff0: 1006041,
			coeff1: 43623,
			coeff2: 251,
		}},
	},
	builtin.AndByteString: &CostingFunc[Arguments]{
		mem: &ThreeLinearInMaxYZ{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ThreeLinearInYandZ{TwoVariableLinearSize{
			intercept: 100181,
			slope1:    726,
			slope2:    719,
		}},
	},
	builtin.OrByteString: &CostingFunc[Arguments]{
		mem: &ThreeLinearInMaxYZ{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ThreeLinearInYandZ{TwoVariableLinearSize{
			intercept: 100181,
			slope1:    726,
			slope2:    719,
		}},
	},
	builtin.XorByteString: &CostingFunc[Arguments]{
		mem: &ThreeLinearInMaxYZ{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ThreeLinearInYandZ{TwoVariableLinearSize{
			intercept: 100181,
			slope1:    726,
			slope2:    719,
		}},
	},
	builtin.ComplementByteString: &CostingFunc[Arguments]{
		mem: &LinearCost{
			intercept: 0,
			slope:     1,
		},
		cpu: &LinearCost{
			intercept: 107878,
			slope:     680,
		},
	},
	builtin.ReadBit: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{95336},
	},
	builtin.WriteBits: &CostingFunc[Arguments]{
		mem: &ThreeLinearInX{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ThreeLinearInY{LinearCost{
			intercept: 281145,
			slope:     18848,
		}},
	},
	builtin.ReplicateByte: &CostingFunc[Arguments]{
		mem: &LinearInX{LinearCost{
			intercept: 1,
			slope:     1,
		}},
		cpu: &LinearInX{LinearCost{
			intercept: 180194,
			slope:     159,
		}},
	},
	builtin.ShiftByteString: &CostingFunc[Arguments]{
		mem: &LinearInX{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &LinearInX{LinearCost{
			intercept: 158519,
			slope:     8942,
		}},
	},
	builtin.RotateByteString: &CostingFunc[Arguments]{
		mem: &LinearInX{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &LinearInX{LinearCost{
			intercept: 159378,
			slope:     8813,
		}},
	},
	builtin.CountSetBits: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &LinearInX{LinearCost{
			intercept: 107490,
			slope:     3298,
		}},
	},
	builtin.FindFirstSetBit: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &LinearInX{LinearCost{
			intercept: 106057,
			slope:     655,
		}},
	},
	builtin.Ripemd_160: &CostingFunc[Arguments]{
		mem: &ConstantCost{3},
		cpu: &LinearInX{LinearCost{
			intercept: 1964219,
			slope:     24520,
		}},
	},
	builtin.ExpModInteger: &CostingFunc[Arguments]{
		mem: &ThreeLinearInZ{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ExpMod{
			coeff00: 607153,
			coeff11: 231697,
			coeff12: 53144,
		},
	},
	builtin.DropList: &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		// Raise baseline CPU so n=0 matches expected total budget better
		cpu: &ConstantCost{116711},
	},
	builtin.LengthOfArray: &CostingFunc[Arguments]{
		mem: &ConstantCost{10},
		cpu: &ConstantCost{231883},
	},
	builtin.ListToArray: &CostingFunc[Arguments]{
		mem: &LinearCost{
			intercept: 7,
			slope:     1,
		},
		cpu: &LinearCost{
			intercept: 1000,
			slope:     24838,
		},
	},
	builtin.IndexArray: &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{232010},
	},
	// Value/coin builtins
	builtin.InsertCoin: &CostingFunc[Arguments]{
		mem: &FourLinearInU{LinearCost{
			intercept: 45,
			slope:     21,
		}},
		cpu: &FourLinearInU{LinearCost{
			intercept: 356924,
			slope:     18413,
		}},
	},
	builtin.LookupCoin: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ThreeLinearInZ{LinearCost{
			intercept: 219951,
			slope:     9444,
		}},
	},
	builtin.ScaleValue: &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 12,
			slope:     21,
		}},
		cpu: &LinearInY{LinearCost{
			intercept: 1000,
			slope:     277577,
		}},
	},
	builtin.UnionValue: &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 24,
			slope:     21,
		}},
		cpu: &WithInteractionInXAndY{
			c00: 1000,
			c01: 183150,
			c10: 172116,
			c11: 6,
		},
	},
	builtin.ValueContains: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstAboveDiagonalModel{
			ConstantOrTwoArguments{
				constant: 213283,
				model: &LinearInXAndY{TwoVariableLinearSize{
					intercept: 618401,
					slope1:    1998,
					slope2:    28258,
				}},
			},
		},
	},
	// V4 model: placeholder for multiIndexArray
	builtin.MultiIndexArray: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{1000000},
	},
	// Value/Data conversion builtins
	builtin.ValueData: &CostingFunc[Arguments]{
		mem: &LinearCost{
			intercept: 2,
			slope:     22,
		},
		cpu: &LinearCost{
			intercept: 199604,
			slope:     39211,
		},
	},
	builtin.UnValueData: &CostingFunc[Arguments]{
		mem: &LinearCost{
			intercept: 11,
			slope:     1,
		},
		cpu: &QuadraticInXModel{QuadraticFunction{
			coeff0: 1000,
			coeff1: 204904,
			coeff2: 1,
		}},
	},
}

type CostingFunc[T Arguments] struct {
	mem T
	cpu T
}

func CostSingle[T OneArgument](cf CostingFunc[T], x func() ExMem) ExBudget {
	memConstants := cf.mem.HasConstants()

	cpuConstants := cf.cpu.HasConstants()

	var usedBudget ExBudget

	if memConstants[0] && cpuConstants[0] {
		usedBudget = ExBudget{
			Mem: int64(cf.mem.Cost(ExMem(0))),
			Cpu: int64(cf.cpu.Cost(ExMem(0))),
		}
	} else {
		i := x()
		usedBudget = ExBudget{
			Mem: int64(cf.mem.Cost(i)),
			Cpu: int64(cf.cpu.Cost(i)),
		}
	}

	return usedBudget
}

// Function to cost TwoArguments similar to CostSingle for OneArgument
func CostPair[T TwoArgument](cf CostingFunc[T], x, y func() ExMem) ExBudget {
	memConstants := cf.mem.HasConstants()
	cpuConstants := cf.cpu.HasConstants()

	var xMem ExMem
	var yMem ExMem

	if memConstants[0] && cpuConstants[0] {
		xMem = ExMem(0)
	} else {
		xMem = x()
	}

	if memConstants[1] && cpuConstants[1] {
		yMem = ExMem(1)
	} else {
		yMem = y()
	}

	return ExBudget{
		Mem: int64(cf.mem.CostTwo(xMem, yMem)),
		Cpu: int64(cf.cpu.CostTwo(xMem, yMem)),
	}
}

// Function to cost ThreeArguments
func CostTriple[T ThreeArgument](
	cf CostingFunc[T],
	x, y, z func() ExMem,
) ExBudget {
	memConstants := cf.mem.HasConstants()
	cpuConstants := cf.cpu.HasConstants()

	var xMem, yMem, zMem ExMem

	if memConstants[0] && cpuConstants[0] {
		xMem = ExMem(0)
	} else {
		xMem = x()
	}

	if memConstants[1] && cpuConstants[1] {
		yMem = ExMem(0)
	} else {
		yMem = y()
	}

	if memConstants[2] && cpuConstants[2] {
		zMem = ExMem(0)
	} else {
		zMem = z()
	}

	return ExBudget{
		Mem: int64(cf.mem.CostThree(xMem, yMem, zMem)),
		Cpu: int64(cf.cpu.CostThree(xMem, yMem, zMem)),
	}
}

// Function to cost FourArguments
func CostQuadtuple[T FourArgument](
	cf CostingFunc[T],
	x, y, z, u func() ExMem,
) ExBudget {
	memConstants := cf.mem.HasConstants()
	cpuConstants := cf.cpu.HasConstants()

	var xMem, yMem, zMem, uMem ExMem

	if memConstants[0] && cpuConstants[0] {
		xMem = ExMem(0)
	} else {
		xMem = x()
	}

	if memConstants[1] && cpuConstants[1] {
		yMem = ExMem(0)
	} else {
		yMem = y()
	}

	if memConstants[2] && cpuConstants[2] {
		zMem = ExMem(0)
	} else {
		zMem = z()
	}

	if len(memConstants) > 3 && memConstants[3] && len(cpuConstants) > 3 && cpuConstants[3] {
		uMem = ExMem(0)
	} else {
		uMem = u()
	}

	return ExBudget{
		Mem: int64(cf.mem.CostFour(xMem, yMem, zMem, uMem)),
		Cpu: int64(cf.cpu.CostFour(xMem, yMem, zMem, uMem)),
	}
}

// Function to cost SixArguments
func CostSextuple[T SixArgument](
	cf CostingFunc[T],
	a, b, c, d, e, f func() ExMem,
) ExBudget {
	memConstants := cf.mem.HasConstants()
	cpuConstants := cf.cpu.HasConstants()

	var aMem, bMem, cMem, dMem, eMem, fMem ExMem

	// Check each argument individually
	if memConstants[0] && cpuConstants[0] {
		aMem = ExMem(0)
	} else {
		aMem = a()
	}

	if memConstants[1] && cpuConstants[1] {
		bMem = ExMem(0)
	} else {
		bMem = b()
	}

	if memConstants[2] && cpuConstants[2] {
		cMem = ExMem(0)
	} else {
		cMem = c()
	}

	if memConstants[3] && cpuConstants[3] {
		dMem = ExMem(0)
	} else {
		dMem = d()
	}

	if memConstants[4] && cpuConstants[4] {
		eMem = ExMem(0)
	} else {
		eMem = e()
	}

	if memConstants[5] && cpuConstants[5] {
		fMem = ExMem(0)
	} else {
		fMem = f()
	}

	return ExBudget{
		Mem: int64(cf.mem.CostSix(aMem, bMem, cMem, dMem, eMem, fMem)),
		Cpu: int64(cf.cpu.CostSix(aMem, bMem, cMem, dMem, eMem, fMem)),
	}
}

type Arguments interface {
	isArguments()
	HasConstants() []bool
}

type OneArgument interface {
	Cost(x ExMem) int64
	Arguments
}

type ConstantCost struct {
	c int64
}

func (ConstantCost) isArguments() {}

func (ConstantCost) HasConstants() []bool {
	return []bool{true, true, true, true, true, true}
}

func (c ConstantCost) Cost(x ExMem) int64 {
	return c.c
}

type LinearCost struct {
	slope     int64
	intercept int64
}

func (LinearCost) isArguments() {}

func (LinearCost) HasConstants() []bool {
	return []bool{false}
}

func (l LinearCost) Cost(x ExMem) int64 {
	return l.slope*int64(x) + l.intercept
}

// TwoArgument interface for costing functions with two arguments
type TwoArgument interface {
	CostTwo(x, y ExMem) int64
	Arguments
}

type TwoVariableLinearSize struct {
	intercept int64
	slope1    int64
	slope2    int64
}

type AddedSizes struct {
	intercept int64
	slope     int64
}

type SubtractedSizes struct {
	intercept int64
	slope     int64
	minimum   int64
}

type MultipliedSizes struct {
	intercept int64
	slope     int64
}

type MinSize struct {
	intercept int64
	slope     int64
}

type MaxSize struct {
	intercept int64
	slope     int64
}

type ConstantOrLinear struct {
	constant  int64
	intercept int64
	slope     int64
}

type ConstantOrTwoArguments struct {
	constant int64
	model    TwoArgument
}

type QuadraticFunction struct {
	coeff0 int64
	coeff1 int64
	coeff2 int64
}

type TwoArgumentsQuadraticFunction struct {
	minimum int64
	coeff00 int64
	coeff10 int64
	coeff01 int64
	coeff20 int64
	coeff11 int64
	coeff02 int64
}

// Implementations for TwoArguments variants
// Using existing ConstantCost for two arguments
func (c ConstantCost) CostTwo(x, y ExMem) int64 {
	return c.c
}

// LinearInX costs based only on the first argument
type LinearInX struct {
	LinearCost
}

func (l LinearInX) CostTwo(x, y ExMem) int64 {
	return l.Cost(x)
}

// Y is not used so constant
func (LinearInX) HasConstants() []bool {
	return []bool{false, true}
}

func (LinearInX) isArguments() {}

// LinearInY costs based only on the second argument
type LinearInY struct {
	LinearCost
}

func (l LinearInY) CostTwo(x, y ExMem) int64 {
	return l.Cost(y)
}

// X is not used so constant
func (LinearInY) HasConstants() []bool {
	return []bool{true, false}
}

func (LinearInY) isArguments() {}

// LinearInXAndY costs based on both arguments with different slopes
type LinearInXAndY struct {
	TwoVariableLinearSize
}

func (l LinearInXAndY) CostTwo(x, y ExMem) int64 {
	return l.slope1*int64(x) + l.slope2*int64(y) + l.intercept
}

func (LinearInXAndY) HasConstants() []bool {
	return []bool{false, false}
}

func (LinearInXAndY) isArguments() {}

// AddedSizesModel costs based on the sum of arguments
type AddedSizesModel struct {
	AddedSizes
}

func (a AddedSizesModel) CostTwo(x, y ExMem) int64 {
	return a.slope*(int64(x)+int64(y)) + a.intercept
}

func (AddedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (AddedSizesModel) isArguments() {}

// SubtractedSizesModel costs based on the difference of arguments
type SubtractedSizesModel struct {
	SubtractedSizes
}

func (s SubtractedSizesModel) CostTwo(x, y ExMem) int64 {
	diff := max(int64(x)-int64(y), s.minimum)

	return s.slope*diff + s.intercept
}

func (SubtractedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (SubtractedSizesModel) isArguments() {}

// MultipliedSizesModel costs based on the product of arguments
type MultipliedSizesModel struct {
	MultipliedSizes
}

func (m MultipliedSizesModel) CostTwo(x, y ExMem) int64 {
	return m.slope*(int64(x)*int64(y)) + m.intercept
}

func (MultipliedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (MultipliedSizesModel) isArguments() {}

// MinSizeModel costs based on the minimum of arguments
type MinSizeModel struct {
	MinSize
}

func (m MinSizeModel) CostTwo(x, y ExMem) int64 {
	min := int64(x)
	if int64(y) < min {
		min = int64(y)
	}

	return m.slope*min + m.intercept
}

func (MinSizeModel) HasConstants() []bool {
	return []bool{false, false}
}

func (MinSizeModel) isArguments() {}

// MaxSizeModel costs based on the maximum of arguments
type MaxSizeModel struct {
	MaxSize
}

func (m MaxSizeModel) CostTwo(x, y ExMem) int64 {
	max := int64(x)
	if int64(y) > max {
		max = int64(y)
	}

	return m.slope*max + m.intercept
}

func (MaxSizeModel) HasConstants() []bool {
	return []bool{false, false}
}

func (MaxSizeModel) isArguments() {}

// LinearOnDiagonalModel costs linearly when arguments are equal, constant otherwise
type LinearOnDiagonalModel struct {
	ConstantOrLinear
}

func (l LinearOnDiagonalModel) CostTwo(x, y ExMem) int64 {
	if int64(x) == int64(y) {
		return l.slope*int64(x) + l.intercept
	}

	return l.constant
}

func (LinearOnDiagonalModel) HasConstants() []bool {
	return []bool{false, false}
}

func (LinearOnDiagonalModel) isArguments() {}

// ConstAboveDiagonalModel costs constant when x < y, uses another model otherwise
type ConstAboveDiagonalModel struct {
	ConstantOrTwoArguments
}

func (c ConstAboveDiagonalModel) CostTwo(x, y ExMem) int64 {
	if int64(x) < int64(y) {
		return c.constant
	}

	return c.model.CostTwo(x, y)
}

func (ConstAboveDiagonalModel) HasConstants() []bool {
	return []bool{false, false}
}

func (ConstAboveDiagonalModel) isArguments() {}

// ConstBelowDiagonalModel costs constant when x > y, uses another model otherwise
type ConstBelowDiagonalModel struct {
	ConstantOrTwoArguments
}

func (c ConstBelowDiagonalModel) CostTwo(x, y ExMem) int64 {
	if int64(x) > int64(y) {
		return c.constant
	}

	return c.model.CostTwo(x, y)
}

func (ConstBelowDiagonalModel) HasConstants() []bool {
	return []bool{false, false}
}

func (ConstBelowDiagonalModel) isArguments() {}

// QuadraticInYModel costs based on a quadratic function of y
type QuadraticInYModel struct {
	QuadraticFunction
}

func (q QuadraticInYModel) CostTwo(x, y ExMem) int64 {
	yVal := int64(y)

	return q.coeff0 + (q.coeff1 * yVal) + (q.coeff2 * yVal * yVal)
}

// X is not used so constant
func (QuadraticInYModel) HasConstants() []bool {
	return []bool{true, false}
}

func (QuadraticInYModel) isArguments() {}

// QuadraticInXModel costs based on a quadratic function of x
type QuadraticInXModel struct {
	QuadraticFunction
}

func (q QuadraticInXModel) Cost(x ExMem) int64 {
	xVal := int64(x)
	return q.coeff0 + (q.coeff1 * xVal) + (q.coeff2 * xVal * xVal)
}

// X is used for computation
func (QuadraticInXModel) HasConstants() []bool {
	return []bool{false}
}

func (QuadraticInXModel) isArguments() {}

// WithInteractionInXAndY costs based on both x and y with an interaction term
// Formula: c00 + c01*y + c10*x + c11*x*y
type WithInteractionInXAndY struct {
	c00 int64
	c01 int64
	c10 int64
	c11 int64
}

func (w WithInteractionInXAndY) CostTwo(x, y ExMem) int64 {
	xVal, yVal := int64(x), int64(y)
	return w.c00 + w.c01*yVal + w.c10*xVal + w.c11*xVal*yVal
}

func (WithInteractionInXAndY) HasConstants() []bool {
	return []bool{false, false}
}

func (WithInteractionInXAndY) isArguments() {}

// ConstAboveDiagonalIntoQuadraticXAndYModel costs constant when x < y, uses a quadratic function otherwise
type ConstAboveDiagonalIntoQuadraticXAndYModel struct {
	constant int64
	TwoArgumentsQuadraticFunction
}

func (c ConstAboveDiagonalIntoQuadraticXAndYModel) CostTwo(x, y ExMem) int64 {
	if int64(x) < int64(y) {
		return c.constant
	}

	xVal, yVal := int64(x), int64(y)
	result := c.coeff00 +
		c.coeff10*xVal +
		c.coeff01*yVal +
		c.coeff20*xVal*xVal +
		c.coeff11*xVal*yVal +
		c.coeff02*yVal*yVal

	if result < c.minimum {
		return c.minimum
	}

	return result
}

func (ConstAboveDiagonalIntoQuadraticXAndYModel) HasConstants() []bool {
	return []bool{false, false}
}

func (ConstAboveDiagonalIntoQuadraticXAndYModel) isArguments() {}

// ThreeArgument interface for costing functions with three arguments
type ThreeArgument interface {
	CostThree(x, y, z ExMem) int64
	Arguments
}

// Implementations for ThreeArguments variants

// Using existing ConstantCost for three arguments
func (c ConstantCost) CostThree(x, y, z ExMem) int64 {
	return c.c
}

// ThreeAddedSizesModel costs based on the sum of three arguments
type ThreeAddedSizesModel struct {
	AddedSizes
}

func (a ThreeAddedSizesModel) CostThree(x, y, z ExMem) int64 {
	return a.slope*(int64(x)+int64(y)+int64(z)) + a.intercept
}

func (ThreeAddedSizesModel) HasConstants() []bool {
	return []bool{false, false, false}
}

func (ThreeAddedSizesModel) isArguments() {}

// LinearInX costs based only on the first argument
type ThreeLinearInX struct {
	LinearCost
}

func (l ThreeLinearInX) CostThree(x, y, z ExMem) int64 {
	return l.Cost(x)
}

// Y,Z are not used so constant
func (ThreeLinearInX) HasConstants() []bool {
	return []bool{false, true, true}
}

func (ThreeLinearInX) isArguments() {}

// LinearInY costs based only on the second argument
type ThreeLinearInY struct {
	LinearCost
}

func (l ThreeLinearInY) CostThree(x, y, z ExMem) int64 {
	return l.Cost(y)
}

// X,Z are not used so constant
func (ThreeLinearInY) HasConstants() []bool {
	return []bool{true, false, true}
}

func (ThreeLinearInY) isArguments() {}

// LinearInZ costs based only on the third argument
type ThreeLinearInZ struct {
	LinearCost
}

func (l ThreeLinearInZ) CostThree(x, y, z ExMem) int64 {
	return l.Cost(z)
}

// X,Y are not used so constant
func (ThreeLinearInZ) HasConstants() []bool {
	return []bool{true, true, false}
}

func (ThreeLinearInZ) isArguments() {}

// QuadraticInZ costs based on a quadratic function of the third argument
type ThreeQuadraticInZ struct {
	QuadraticFunction
}

// X,Y are not used so constant
func (q ThreeQuadraticInZ) CostThree(x, y, z ExMem) int64 {
	zVal := int64(z)

	return q.coeff0 + (q.coeff1 * zVal) + (q.coeff2 * zVal * zVal)
}

func (ThreeQuadraticInZ) HasConstants() []bool {
	return []bool{true, true, false}
}

func (ThreeQuadraticInZ) isArguments() {}

// LiteralInYorLinearInZ costs y if y != 0, otherwise linear in z
type ThreeLiteralInYorLinearInZ struct {
	LinearCost
}

func (l ThreeLiteralInYorLinearInZ) CostThree(x, y, z ExMem) int64 {
	if int64(y) == 0 {
		return l.slope*int64(z) + l.intercept
	}

	return int64(y)
}

// X is not used so constant
func (ThreeLiteralInYorLinearInZ) HasConstants() []bool {
	return []bool{true, false, false}
}

func (ThreeLiteralInYorLinearInZ) isArguments() {}

// LinearInMaxYZ costs based on the maximum of y and z
type ThreeLinearInMaxYZ struct {
	LinearCost
}

func (l ThreeLinearInMaxYZ) CostThree(x, y, z ExMem) int64 {
	max := int64(y)
	if int64(z) > max {
		max = int64(z)
	}

	return l.slope*max + l.intercept
}

// X is not used so constant
func (ThreeLinearInMaxYZ) HasConstants() []bool {
	return []bool{true, false, false}
}

func (ThreeLinearInMaxYZ) isArguments() {}

// LinearInYandZ costs based on both y and z arguments
type ThreeLinearInYandZ struct {
	TwoVariableLinearSize
}

func (l ThreeLinearInYandZ) CostThree(x, y, z ExMem) int64 {
	return l.slope1*int64(y) + l.slope2*int64(z) + l.intercept
}

// X is not used so constant
func (ThreeLinearInYandZ) HasConstants() []bool {
	return []bool{true, false, false}
}

func (ThreeLinearInYandZ) isArguments() {}

// FourArgument interface for costing functions with four arguments
type FourArgument interface {
	CostFour(x, y, z, u ExMem) int64
	Arguments
}

// Implementations for FourArguments variants

// Using existing ConstantCost for four arguments
func (c ConstantCost) CostFour(x, y, z, u ExMem) int64 {
	return c.c
}

// FourLinearInU costs based only on the fourth argument
type FourLinearInU struct {
	LinearCost
}

func (l FourLinearInU) CostFour(x, y, z, u ExMem) int64 {
	return l.Cost(u)
}

// X,Y,Z are not used so constant
func (FourLinearInU) HasConstants() []bool {
	return []bool{true, true, true, false}
}

func (FourLinearInU) isArguments() {}

// SixArgument interface for costing functions with six arguments
type SixArgument interface {
	CostSix(a, b, c, d, e, f ExMem) int64
	Arguments
}

// Implementations for SixArguments variants

// Using existing ConstantCost for six arguments
func (c ConstantCost) CostSix(a, b, c2, d, e, f ExMem) int64 {
	return c.c
}

type ExpMod struct {
	coeff00 int64
	coeff11 int64
	coeff12 int64
}

func (ExpMod) isArguments() {}

func (l ExpMod) CostThree(aa, ee, mm ExMem) int64 {
	cost0 := l.coeff00 + l.coeff11*int64(
		ee,
	)*int64(
		mm,
	) + l.coeff12*int64(
		ee,
	)*int64(
		mm,
	)*int64(
		mm,
	)

	if int64(aa) <= int64(mm) {
		return cost0
	} else {
		return cost0 + (cost0 / 2)
	}
}

func (ExpMod) HasConstants() []bool {
	return []bool{false, false, false}
}

func buildBuiltinCosts(version lang.LanguageVersion, semantics SemanticsVariant) (BuiltinCosts, error) {
	costs := DefaultBuiltinCosts.Clone()
	// V1 and V2 have some slight changes from the default builtin costs, some depending on the semantics variant
	if version == lang.LanguageVersionV1 || version == lang.LanguageVersionV2 {
		costs[builtin.ModInteger] = &CostingFunc[Arguments]{
			mem: &SubtractedSizesModel{SubtractedSizes{
				intercept: 0,
				slope:     1,
				minimum:   1,
			}},
			cpu: &ConstAboveDiagonalModel{
				ConstantOrTwoArguments{
					constant: 196500,
					model: &MultipliedSizesModel{MultipliedSizes{
						intercept: 453240,
						slope:     220,
					}},
				},
			},
		}
		if semantics == SemanticsVariantA {
			costs[builtin.MultiplyInteger] = &CostingFunc[Arguments]{
				mem: &AddedSizesModel{AddedSizes{
					intercept: 0,
					slope:     1,
				}},
				cpu: &AddedSizesModel{AddedSizes{
					intercept: 69522,
					slope:     11687,
				}},
			}
		}
		costs[builtin.DivideInteger] = &CostingFunc[Arguments]{
			mem: &SubtractedSizesModel{SubtractedSizes{
				intercept: 0,
				slope:     1,
				minimum:   1,
			}},
			cpu: &ConstAboveDiagonalModel{
				ConstantOrTwoArguments{
					constant: 196500,
					model: &MultipliedSizesModel{MultipliedSizes{
						intercept: 453240,
						slope:     220,
					}},
				},
			},
		}
		costs[builtin.QuotientInteger] = &CostingFunc[Arguments]{
			mem: &SubtractedSizesModel{SubtractedSizes{
				intercept: 0,
				slope:     1,
				minimum:   1,
			}},
			cpu: &ConstAboveDiagonalModel{
				ConstantOrTwoArguments{
					constant: 196500,
					model: &MultipliedSizesModel{MultipliedSizes{
						intercept: 453240,
						slope:     220,
					}},
				},
			},
		}
		costs[builtin.RemainderInteger] = &CostingFunc[Arguments]{
			mem: &SubtractedSizesModel{SubtractedSizes{
				intercept: 0,
				slope:     1,
				minimum:   1,
			}},
			cpu: &ConstAboveDiagonalModel{
				ConstantOrTwoArguments{
					constant: 196500,
					model: &MultipliedSizesModel{MultipliedSizes{
						intercept: 453240,
						slope:     220,
					}},
				},
			},
		}
		// V1/V2 use linear_in_z (signature size) instead of linear_in_y (message size) for verifyEd25519Signature
		// This is a critical difference: V2 costs based on constant signature size (64 bytes),
		// while V3 costs based on variable message size
		costs[builtin.VerifyEd25519Signature] = &CostingFunc[Arguments]{
			mem: &ConstantCost{10},
			cpu: &ThreeLinearInZ{LinearCost{
				intercept: 57996947,
				slope:     18975,
			}},
		}
	}
	return costs, nil
}
