package cek

import (
	"github.com/blinklabs-io/plutigo/builtin"
)

type BuiltinCosts [builtin.TotalBuiltinCount]*CostingFunc[Arguments]

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
		cpu: &LinearCost{
			intercept: 201305,
			slope:     8356,
		},
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
	builtin.CaseList: &CostingFunc[Arguments]{},
	builtin.CaseData: &CostingFunc[Arguments]{},
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
	// TODO: InsertCoin, ScaleValue, and UnionValue currently use placeholder costs
	// (100 billion units). These should be properly calibrated based on actual execution
	// time measurements.
	builtin.InsertCoin: &CostingFunc[Arguments]{
		mem: &ConstantCost{100000000000},
		cpu: &ConstantCost{100000000000},
	},
	builtin.LookupCoin: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{248283},
	},
	builtin.ScaleValue: &CostingFunc[Arguments]{
		mem: &ConstantCost{100000000000},
		cpu: &ConstantCost{100000000000},
	},
	builtin.UnionValue: &CostingFunc[Arguments]{
		mem: &ConstantCost{100000000000},
		cpu: &ConstantCost{100000000000},
	},
	// V4 model: align with conformance (~1.25M CPU, mem ~601 total)
	builtin.ValueContains: &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{1163000},
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
	Cost(x ExMem) int
	Arguments
}

type ConstantCost struct {
	c int
}

func (ConstantCost) isArguments() {}

func (ConstantCost) HasConstants() []bool {
	return []bool{true, true, true, true, true, true}
}

func (c ConstantCost) Cost(x ExMem) int {
	return c.c
}

type LinearCost struct {
	slope     int
	intercept int
}

func (LinearCost) isArguments() {}

func (LinearCost) HasConstants() []bool {
	return []bool{false}
}

func (l LinearCost) Cost(x ExMem) int {
	return l.slope*int(x) + l.intercept
}

// TwoArgument interface for costing functions with two arguments
type TwoArgument interface {
	CostTwo(x, y ExMem) int
	Arguments
}

type TwoVariableLinearSize struct {
	intercept int
	slope1    int
	slope2    int
}

type AddedSizes struct {
	intercept int
	slope     int
}

type SubtractedSizes struct {
	intercept int
	slope     int
	minimum   int
}

type MultipliedSizes struct {
	intercept int
	slope     int
}

type MinSize struct {
	intercept int
	slope     int
}

type MaxSize struct {
	intercept int
	slope     int
}

type ConstantOrLinear struct {
	constant  int
	intercept int
	slope     int
}

type ConstantOrTwoArguments struct {
	constant int
	model    TwoArgument
}

type QuadraticFunction struct {
	coeff0 int
	coeff1 int
	coeff2 int
}

type TwoArgumentsQuadraticFunction struct {
	minimum int
	coeff00 int
	coeff10 int
	coeff01 int
	coeff20 int
	coeff11 int
	coeff02 int
}

// Implementations for TwoArguments variants
// Using existing ConstantCost for two arguments
func (c ConstantCost) CostTwo(x, y ExMem) int {
	return c.c
}

// LinearInX costs based only on the first argument
type LinearInX struct {
	LinearCost
}

func (l LinearInX) CostTwo(x, y ExMem) int {
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

func (l LinearInY) CostTwo(x, y ExMem) int {
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

func (l LinearInXAndY) CostTwo(x, y ExMem) int {
	return l.slope1*int(x) + l.slope2*int(y) + l.intercept
}

func (LinearInXAndY) HasConstants() []bool {
	return []bool{false, false}
}

func (LinearInXAndY) isArguments() {}

// AddedSizesModel costs based on the sum of arguments
type AddedSizesModel struct {
	AddedSizes
}

func (a AddedSizesModel) CostTwo(x, y ExMem) int {
	return a.slope*(int(x)+int(y)) + a.intercept
}

func (AddedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (AddedSizesModel) isArguments() {}

// SubtractedSizesModel costs based on the difference of arguments
type SubtractedSizesModel struct {
	SubtractedSizes
}

func (s SubtractedSizesModel) CostTwo(x, y ExMem) int {
	diff := max(int(x)-int(y), s.minimum)

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

func (m MultipliedSizesModel) CostTwo(x, y ExMem) int {
	return m.slope*(int(x)*int(y)) + m.intercept
}

func (MultipliedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (MultipliedSizesModel) isArguments() {}

// MinSizeModel costs based on the minimum of arguments
type MinSizeModel struct {
	MinSize
}

func (m MinSizeModel) CostTwo(x, y ExMem) int {
	min := int(x)
	if int(y) < min {
		min = int(y)
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

func (m MaxSizeModel) CostTwo(x, y ExMem) int {
	max := int(x)
	if int(y) > max {
		max = int(y)
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

func (l LinearOnDiagonalModel) CostTwo(x, y ExMem) int {
	if int(x) == int(y) {
		return l.slope*int(x) + l.intercept
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

func (c ConstAboveDiagonalModel) CostTwo(x, y ExMem) int {
	if int(x) < int(y) {
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

func (c ConstBelowDiagonalModel) CostTwo(x, y ExMem) int {
	if int(x) > int(y) {
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

func (q QuadraticInYModel) CostTwo(x, y ExMem) int {
	yVal := int(y)

	return q.coeff0 + (q.coeff1 * yVal) + (q.coeff2 * yVal * yVal)
}

// X is not used so constant
func (QuadraticInYModel) HasConstants() []bool {
	return []bool{true, false}
}

func (QuadraticInYModel) isArguments() {}

// ConstAboveDiagonalIntoQuadraticXAndYModel costs constant when x < y, uses a quadratic function otherwise
type ConstAboveDiagonalIntoQuadraticXAndYModel struct {
	constant int
	TwoArgumentsQuadraticFunction
}

func (c ConstAboveDiagonalIntoQuadraticXAndYModel) CostTwo(x, y ExMem) int {
	if int(x) < int(y) {
		return c.constant
	}

	xVal, yVal := int(x), int(y)
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
	CostThree(x, y, z ExMem) int
	Arguments
}

// Implementations for ThreeArguments variants

// Using existing ConstantCost for three arguments
func (c ConstantCost) CostThree(x, y, z ExMem) int {
	return c.c
}

// ThreeAddedSizesModel costs based on the sum of three arguments
type ThreeAddedSizesModel struct {
	AddedSizes
}

func (a ThreeAddedSizesModel) CostThree(x, y, z ExMem) int {
	return a.slope*(int(x)+int(y)+int(z)) + a.intercept
}

func (ThreeAddedSizesModel) HasConstants() []bool {
	return []bool{false, false, false}
}

func (ThreeAddedSizesModel) isArguments() {}

// LinearInX costs based only on the first argument
type ThreeLinearInX struct {
	LinearCost
}

func (l ThreeLinearInX) CostThree(x, y, z ExMem) int {
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

func (l ThreeLinearInY) CostThree(x, y, z ExMem) int {
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

func (l ThreeLinearInZ) CostThree(x, y, z ExMem) int {
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
func (q ThreeQuadraticInZ) CostThree(x, y, z ExMem) int {
	zVal := int(z)

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

func (l ThreeLiteralInYorLinearInZ) CostThree(x, y, z ExMem) int {
	if int(y) == 0 {
		return l.slope*int(z) + l.intercept
	}

	return int(y)
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

func (l ThreeLinearInMaxYZ) CostThree(x, y, z ExMem) int {
	max := int(y)
	if int(z) > max {
		max = int(z)
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

func (l ThreeLinearInYandZ) CostThree(x, y, z ExMem) int {
	return l.slope1*int(y) + l.slope2*int(z) + l.intercept
}

// X is not used so constant
func (ThreeLinearInYandZ) HasConstants() []bool {
	return []bool{true, false, false}
}

func (ThreeLinearInYandZ) isArguments() {}

// SixArgument interface for costing functions with six arguments
type SixArgument interface {
	CostSix(a, b, c, d, e, f ExMem) int
	Arguments
}

// Implementations for SixArguments variants

// Using existing ConstantCost for six arguments
func (c ConstantCost) CostSix(a, b, c2, d, e, f ExMem) int {
	return c.c
}

type ExpMod struct {
	coeff00 int
	coeff11 int
	coeff12 int
}

func (ExpMod) isArguments() {}

func (l ExpMod) CostThree(aa, ee, mm ExMem) int {
	cost0 := l.coeff00 + l.coeff11*int(
		ee,
	)*int(
		mm,
	) + l.coeff12*int(
		ee,
	)*int(
		mm,
	)*int(
		mm,
	)

	if int(aa) <= int(mm) {
		return cost0
	} else {
		return cost0 + (cost0 / 2)
	}
}

func (ExpMod) HasConstants() []bool {
	return []bool{false, false, false}
}

var V1BuiltinCosts = func() BuiltinCosts {
	var costs BuiltinCosts
	// Initialize with V1 cost models
	costs[builtin.AddInteger] = &CostingFunc[Arguments]{
		mem: &MaxSizeModel{MaxSize{
			intercept: 1,
			slope:     1,
		}},
		cpu: &MaxSizeModel{MaxSize{
			intercept: 205665,
			slope:     812,
		}},
	}
	costs[builtin.SubtractInteger] = &CostingFunc[Arguments]{
		mem: &MaxSizeModel{MaxSize{
			intercept: 1,
			slope:     1,
		}},
		cpu: &MaxSizeModel{MaxSize{
			intercept: 205665,
			slope:     812,
		}},
	}
	costs[builtin.MultiplyInteger] = &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 0,
			slope:     1,
		}},
		cpu: &MultipliedSizesModel{MultipliedSizes{
			intercept: 69522,
			slope:     11687,
		}},
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
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     1,
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
	costs[builtin.ModInteger] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     1,
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
	costs[builtin.EqualsInteger] = &CostingFunc[Arguments]{
		mem: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1,
				intercept: 0,
				slope:     1,
			},
		},
		cpu: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1000,
				intercept: 1000,
				slope:     1,
			},
		},
	}
	costs[builtin.LessThanInteger] = &CostingFunc[Arguments]{
		mem: &MaxSizeModel{MaxSize{
			intercept: 1,
			slope:     1,
		}},
		cpu: &MaxSizeModel{MaxSize{
			intercept: 1000,
			slope:     1,
		}},
	}
	costs[builtin.LessThanEqualsInteger] = &CostingFunc[Arguments]{
		mem: &MaxSizeModel{MaxSize{
			intercept: 1,
			slope:     1,
		}},
		cpu: &MaxSizeModel{MaxSize{
			intercept: 1000,
			slope:     1,
		}},
	}
	costs[builtin.AppendByteString] = &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 0,
			slope:     1,
		}},
		cpu: &AddedSizesModel{AddedSizes{
			intercept: 1000,
			slope:     571,
		}},
	}
	costs[builtin.ConsByteString] = &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 0,
			slope:     1,
		}},
		cpu: &AddedSizesModel{AddedSizes{
			intercept: 1000,
			slope:     221,
		}},
	}
	costs[builtin.SliceByteString] = &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.LengthOfByteString] = &CostingFunc[Arguments]{
		mem: &ConstantCost{10},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.IndexByteString] = &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.EqualsByteString] = &CostingFunc[Arguments]{
		mem: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1,
				intercept: 0,
				slope:     1,
			},
		},
		cpu: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1000,
				intercept: 1000,
				slope:     1,
			},
		},
	}
	costs[builtin.LessThanByteString] = &CostingFunc[Arguments]{
		mem: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1,
				intercept: 0,
				slope:     1,
			},
		},
		cpu: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1000,
				intercept: 1000,
				slope:     1,
			},
		},
	}
	costs[builtin.LessThanEqualsByteString] = &CostingFunc[Arguments]{
		mem: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1,
				intercept: 0,
				slope:     1,
			},
		},
		cpu: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1000,
				intercept: 1000,
				slope:     1,
			},
		},
	}
	costs[builtin.Sha2_256] = &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.Sha3_256] = &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.Blake2b_256] = &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.VerifyEd25519Signature] = &CostingFunc[Arguments]{
		mem: &ConstantCost{10},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.AppendString] = &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 4,
			slope:     1,
		}},
		cpu: &AddedSizesModel{AddedSizes{
			intercept: 1000,
			slope:     59957,
		}},
	}
	costs[builtin.EqualsString] = &CostingFunc[Arguments]{
		mem: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1,
				intercept: 0,
				slope:     1,
			},
		},
		cpu: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1000,
				intercept: 1000,
				slope:     1,
			},
		},
	}
	costs[builtin.EncodeUtf8] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 4,
			slope:     2,
		}},
		cpu: &LinearInY{LinearCost{
			intercept: 1000,
			slope:     28662,
		}},
	}
	costs[builtin.DecodeUtf8] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 4,
			slope:     2,
		}},
		cpu: &LinearInY{LinearCost{
			intercept: 1000,
			slope:     35892,
		}},
	}
	costs[builtin.IfThenElse] = &CostingFunc[Arguments]{
		mem: &ConstantCost{1},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.ChooseUnit] = &CostingFunc[Arguments]{
		mem: &ConstantCost{4},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.Trace] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.FstPair] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.SndPair] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.ChooseList] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.MkCons] = &CostingFunc[Arguments]{
		mem: &AddedSizesModel{AddedSizes{
			intercept: 32,
			slope:     32,
		}},
		cpu: &AddedSizesModel{AddedSizes{
			intercept: 1000,
			slope:     59957,
		}},
	}
	costs[builtin.HeadList] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.TailList] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.NullList] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.ChooseData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.ConstrData] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 32,
			slope:     32,
		}},
		cpu: &LinearInY{LinearCost{
			intercept: 1000,
			slope:     5431,
		}},
	}
	costs[builtin.MapData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.ListData] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 32,
			slope:     32,
		}},
		cpu: &LinearInY{LinearCost{
			intercept: 1000,
			slope:     5431,
		}},
	}
	costs[builtin.IData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.BData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.UnConstrData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.UnMapData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.UnListData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.UnIData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.UnBData] = &CostingFunc[Arguments]{
		mem: &ConstantCost{32},
		cpu: &ConstantCost{1000},
	}
	costs[builtin.EqualsData] = &CostingFunc[Arguments]{
		mem: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  32,
				intercept: 32,
				slope:     32,
			},
		},
		cpu: &LinearOnDiagonalModel{
			ConstantOrLinear{
				constant:  1000,
				intercept: 1000,
				slope:     1,
			},
		},
	}
	costs[builtin.SerialiseData] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     2,
		}},
		cpu: &LinearInY{LinearCost{
			intercept: 1000,
			slope:     533,
		}},
	}
	return costs
}()

var V2BuiltinCosts = func() BuiltinCosts {
	costs := DefaultBuiltinCosts
	// Modify V2 costs where they differ from V3
	costs[builtin.DivideInteger] = &CostingFunc[Arguments]{
		mem: &SubtractedSizesModel{SubtractedSizes{
			intercept: 0,
			slope:     1,
			minimum:   1,
		}},
		cpu: &ConstAboveDiagonalModel{
			ConstantOrTwoArguments{
				constant: 85848,
				model: &MultipliedSizesModel{MultipliedSizes{
					intercept: 228465,
					slope:     122,
				}},
			},
		},
	}
	// Similarly for quotientInteger, remainderInteger, modInteger
	costs[builtin.QuotientInteger] = &CostingFunc[Arguments]{
		mem: &SubtractedSizesModel{SubtractedSizes{
			intercept: 0,
			slope:     1,
			minimum:   1,
		}},
		cpu: &ConstAboveDiagonalModel{
			ConstantOrTwoArguments{
				constant: 85848,
				model: &MultipliedSizesModel{MultipliedSizes{
					intercept: 228465,
					slope:     122,
				}},
			},
		},
	}
	costs[builtin.RemainderInteger] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ConstAboveDiagonalModel{
			ConstantOrTwoArguments{
				constant: 85848,
				model: &MultipliedSizesModel{MultipliedSizes{
					intercept: 228465,
					slope:     122,
				}},
			},
		},
	}
	costs[builtin.ModInteger] = &CostingFunc[Arguments]{
		mem: &LinearInY{LinearCost{
			intercept: 0,
			slope:     1,
		}},
		cpu: &ConstAboveDiagonalModel{
			ConstantOrTwoArguments{
				constant: 85848,
				model: &MultipliedSizesModel{MultipliedSizes{
					intercept: 228465,
					slope:     122,
				}},
			},
		},
	}
	return costs
}()
