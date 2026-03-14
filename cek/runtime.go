package cek

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

const IntegerToByteStringMaximumOutputLength = 8192

type Builtins[T syn.Eval] [builtin.TotalBuiltinCount]func(*Machine[T], *Builtin[T]) (Value[T], error)

var (
	availableBuiltinsV10 = buildAvailableBuiltins(lang.LanguageVersionV1, 0)
	availableBuiltinsV20 = buildAvailableBuiltins(lang.LanguageVersionV2, 0)
	availableBuiltinsV30 = buildAvailableBuiltins(lang.LanguageVersionV3, 0)
	availableBuiltinsV40 = buildAvailableBuiltins(lang.LanguageVersionV4, 0)
	filteredBuiltinsV10  = buildFilteredBuiltins(lang.LanguageVersionV1, 0)
	filteredBuiltinsV20  = buildFilteredBuiltins(lang.LanguageVersionV2, 0)
	filteredBuiltinsV30  = buildFilteredBuiltins(lang.LanguageVersionV3, 0)
	filteredBuiltinsV40  = buildFilteredBuiltins(lang.LanguageVersionV4, 0)
)

func buildAvailableBuiltins(
	version lang.LanguageVersion,
	protoMajor uint,
) *[builtin.TotalBuiltinCount]bool {
	plutusVersion := builtin.LanguageVersionToPlutusVersion(version)
	available := new([builtin.TotalBuiltinCount]bool)
	for i := 0; i < int(builtin.TotalBuiltinCount); i++ {
		fn := builtin.DefaultFunction(i)
		available[i] = fn.IsAvailableInWithProto(plutusVersion, protoMajor)
	}
	return available
}

func buildFilteredBuiltins(
	version lang.LanguageVersion,
	protoMajor uint,
) *Builtins[syn.DeBruijn] {
	ret := newBuiltins[syn.DeBruijn]()
	available := buildAvailableBuiltins(version, protoMajor)
	for i := 0; i < int(builtin.TotalBuiltinCount); i++ {
		if !available[i] {
			ret[i] = nil
		}
	}
	return &ret
}

func newAvailableBuiltins(
	version lang.LanguageVersion,
	protoMajor uint,
) *[builtin.TotalBuiltinCount]bool {
	if protoMajor == 0 {
		switch version {
		case lang.LanguageVersionV1:
			return availableBuiltinsV10
		case lang.LanguageVersionV2:
			return availableBuiltinsV20
		case lang.LanguageVersionV3:
			return availableBuiltinsV30
		case lang.LanguageVersionV4:
			return availableBuiltinsV40
		}
	}
	return buildAvailableBuiltins(version, protoMajor)
}

func newBuiltins[T syn.Eval]() Builtins[T] {
	return Builtins[T]{
		builtin.AddInteger:            addInteger[T],
		builtin.SubtractInteger:       subtractInteger[T],
		builtin.MultiplyInteger:       multiplyInteger[T],
		builtin.DivideInteger:         divideInteger[T],
		builtin.QuotientInteger:       quotientInteger[T],
		builtin.RemainderInteger:      remainderInteger[T],
		builtin.ModInteger:            modInteger[T],
		builtin.EqualsInteger:         equalsInteger[T],
		builtin.LessThanInteger:       lessThanInteger[T],
		builtin.LessThanEqualsInteger: lessThanEqualsInteger[T],

		builtin.AppendByteString:         appendByteString[T],
		builtin.ConsByteString:           consByteString[T],
		builtin.SliceByteString:          sliceByteString[T],
		builtin.LengthOfByteString:       lengthOfByteString[T],
		builtin.IndexByteString:          indexByteString[T],
		builtin.EqualsByteString:         equalsByteString[T],
		builtin.LessThanByteString:       lessThanByteString[T],
		builtin.LessThanEqualsByteString: lessThanEqualsByteString[T],

		builtin.Sha2_256:                        sha2256[T],
		builtin.Sha3_256:                        sha3256[T],
		builtin.Blake2b_256:                     blake2B256[T],
		builtin.VerifyEd25519Signature:          verifyEd25519Signature[T],
		builtin.Blake2b_224:                     blake2B224[T],
		builtin.Keccak_256:                      keccak256[T],
		builtin.VerifyEcdsaSecp256k1Signature:   verifyEcdsaSecp256K1Signature[T],
		builtin.VerifySchnorrSecp256k1Signature: verifySchnorrSecp256K1Signature[T],

		builtin.AppendString: appendString[T],
		builtin.EqualsString: equalsString[T],
		builtin.EncodeUtf8:   encodeUtf8[T],
		builtin.DecodeUtf8:   decodeUtf8[T],

		builtin.IfThenElse:    ifThenElse[T],
		builtin.ChooseUnit:    chooseUnit[T],
		builtin.Trace:         trace[T],
		builtin.FstPair:       fstPair[T],
		builtin.SndPair:       sndPair[T],
		builtin.ChooseList:    chooseList[T],
		builtin.MkCons:        mkCons[T],
		builtin.HeadList:      headList[T],
		builtin.TailList:      tailList[T],
		builtin.NullList:      nullList[T],
		builtin.ChooseData:    chooseData[T],
		builtin.ConstrData:    constrData[T],
		builtin.MapData:       mapData[T],
		builtin.ListData:      listData[T],
		builtin.IData:         iData[T],
		builtin.BData:         bData[T],
		builtin.UnConstrData:  unConstrData[T],
		builtin.UnMapData:     unMapData[T],
		builtin.UnListData:    unListData[T],
		builtin.UnIData:       unIData[T],
		builtin.UnBData:       unBData[T],
		builtin.EqualsData:    equalsData[T],
		builtin.SerialiseData: serialiseData[T],
		builtin.MkPairData:    mkPairData[T],
		builtin.MkNilData:     mkNilData[T],
		builtin.MkNilPairData: mkNilPairData[T],

		builtin.Bls12_381_G1_Add:            bls12381G1Add[T],
		builtin.Bls12_381_G1_Neg:            bls12381G1Neg[T],
		builtin.Bls12_381_G1_ScalarMul:      bls12381G1ScalarMul[T],
		builtin.Bls12_381_G1_Equal:          bls12381G1Equal[T],
		builtin.Bls12_381_G1_Compress:       bls12381G1Compress[T],
		builtin.Bls12_381_G1_Uncompress:     bls12381G1Uncompress[T],
		builtin.Bls12_381_G1_HashToGroup:    bls12381G1HashToGroup[T],
		builtin.Bls12_381_G2_Add:            bls12381G2Add[T],
		builtin.Bls12_381_G2_Neg:            bls12381G2Neg[T],
		builtin.Bls12_381_G2_ScalarMul:      bls12381G2ScalarMul[T],
		builtin.Bls12_381_G1_MultiScalarMul: bls12381G1MultiScalarMul[T],
		builtin.Bls12_381_G2_MultiScalarMul: bls12381G2MultiScalarMul[T],
		builtin.Bls12_381_G2_Equal:          bls12381G2Equal[T],
		builtin.Bls12_381_G2_Compress:       bls12381G2Compress[T],
		builtin.Bls12_381_G2_Uncompress:     bls12381G2Uncompress[T],
		builtin.Bls12_381_G2_HashToGroup:    bls12381G2HashToGroup[T],
		builtin.Bls12_381_MillerLoop:        bls12381MillerLoop[T],
		builtin.Bls12_381_MulMlResult:       bls12381MulMlResult[T],
		builtin.Bls12_381_FinalVerify:       bls12381FinalVerify[T],

		builtin.IntegerToByteString:  integerToByteString[T],
		builtin.ByteStringToInteger:  byteStringToInteger[T],
		builtin.AndByteString:        andByteString[T],
		builtin.OrByteString:         orByteString[T],
		builtin.XorByteString:        xorByteString[T],
		builtin.ComplementByteString: complementByteString[T],
		builtin.ReadBit:              readBit[T],
		builtin.WriteBits:            writeBits[T],
		builtin.ReplicateByte:        replicateByte[T],
		builtin.ShiftByteString:      shiftByteString[T],
		builtin.RotateByteString:     rotateByteString[T],
		builtin.CountSetBits:         countSetBits[T],
		builtin.FindFirstSetBit:      findFirstSetBit[T],
		builtin.Ripemd_160:           ripemd160[T],
		// Batch 6
		builtin.ExpModInteger: expModInteger[T],
		builtin.DropList:      dropList[T],
		// Arrays
		builtin.LengthOfArray: lengthOfArray[T],
		builtin.ListToArray:   listToArray[T],
		builtin.IndexArray:    indexArray[T],
		// Value/coin builtins
		builtin.InsertCoin:      insertCoin[T],
		builtin.LookupCoin:      lookupCoin[T],
		builtin.ScaleValue:      scaleValue[T],
		builtin.UnionValue:      unionValue[T],
		builtin.ValueContains:   valueContains[T],
		builtin.MultiIndexArray: multiIndexArray[T],
		// Value/Data conversion
		builtin.ValueData:   valueData[T],
		builtin.UnValueData: unValueData[T],
	}
}

func (m *Machine[T]) evalBuiltinApp(b *Builtin[T]) (Value[T], error) {
	fn := (*m.builtins)[b.Func]
	if fn == nil || (m.available != nil && !m.available[b.Func]) {
		plutusVersion := builtin.LanguageVersionToPlutusVersion(m.version)
		return nil, &BuiltinError{
			Code:    ErrCodeBuiltinFailure,
			Builtin: b.Func.String(),
			Message: fmt.Sprintf(
				"builtin %s is not available in Plutus %s at protocol version %d (introduced in %s)",
				b.Func.String(),
				plutusVersionName(plutusVersion),
				m.protoMajor,
				plutusVersionName(b.Func.IntroducedIn()),
			),
		}
	}
	return fn(m, b)
}

type twoArgCost struct {
	mem       TwoArgument
	cpu       TwoArgument
	memConstX bool
	memConstY bool
	cpuConstX bool
	cpuConstY bool
}

func newTwoArgCostCache(
	costs BuiltinCosts,
) [builtin.TotalBuiltinCount]twoArgCost {
	var ret [builtin.TotalBuiltinCount]twoArgCost
	for i, model := range costs {
		if model == nil {
			continue
		}

		mem, memOK := model.mem.(TwoArgument)
		cpu, cpuOK := model.cpu.(TwoArgument)
		if !memOK || !cpuOK {
			continue
		}

		memConstants := mem.HasConstants()
		cpuConstants := cpu.HasConstants()
		ret[i] = twoArgCost{
			mem:       mem,
			cpu:       cpu,
			memConstX: memConstants[0],
			memConstY: memConstants[1],
			cpuConstX: cpuConstants[0],
			cpuConstY: cpuConstants[1],
		}
	}
	return ret
}

// plutusVersionName returns a human-readable name for the Plutus version
func plutusVersionName(v builtin.PlutusVersion) string {
	switch v {
	case builtin.PlutusV1:
		return "V1"
	case builtin.PlutusV2:
		return "V2"
	case builtin.PlutusV3:
		return "V3"
	case builtin.PlutusV4:
		return "V4"
	case builtin.PlutusVUnreleased:
		return "unreleased"
	default:
		return fmt.Sprintf("V%d", v)
	}
}

func (m *Machine[T]) CostOne(b *builtin.DefaultFunction, x func() ExMem) error {
	model := m.costs.builtinCosts[*b]

	mem := model.mem.(OneArgument)
	cpu := model.cpu.(OneArgument)

	cf := CostingFunc[OneArgument]{
		mem: mem,
		cpu: cpu,
	}

	cost := CostSingle(cf, x)
	return m.spendBudget(cost)
}

func (m *Machine[T]) CostTwo(
	b *builtin.DefaultFunction,
	x, y func() ExMem,
) error {
	model := m.costs.builtinCosts[*b]

	mem := model.mem.(TwoArgument)
	cpu := model.cpu.(TwoArgument)

	cf := CostingFunc[TwoArgument]{
		mem: mem,
		cpu: cpu,
	}

	cost := CostPair(cf, x, y)
	return m.spendBudget(cost)
}

func (m *Machine[T]) CostTwoExMem(
	b *builtin.DefaultFunction,
	x, y ExMem,
) error {
	model := &m.twoArgCosts[*b]
	mem := model.mem
	cpu := model.cpu
	if mem == nil || cpu == nil {
		fallbackModel := m.costs.builtinCosts[*b]
		mem = fallbackModel.mem.(TwoArgument)
		cpu = fallbackModel.cpu.(TwoArgument)
		model.mem = mem
		model.cpu = cpu
		memConstants := mem.HasConstants()
		cpuConstants := cpu.HasConstants()
		model.memConstX = memConstants[0]
		model.memConstY = memConstants[1]
		model.cpuConstX = cpuConstants[0]
		model.cpuConstY = cpuConstants[1]
	}

	xMem := x
	if model.memConstX && model.cpuConstX {
		xMem = ExMem(0)
	}
	yMem := y
	if model.memConstY && model.cpuConstY {
		yMem = ExMem(0)
	}
	return m.spendBudget(ExBudget{
		Mem: int64(mem.CostTwo(xMem, yMem)),
		Cpu: int64(cpu.CostTwo(xMem, yMem)),
	})
}

func (m *Machine[T]) CostThree(
	b *builtin.DefaultFunction,
	x, y, z func() ExMem,
) error {
	model := m.costs.builtinCosts[*b]

	mem := model.mem.(ThreeArgument)
	cpu := model.cpu.(ThreeArgument)

	cf := CostingFunc[ThreeArgument]{
		mem: mem,
		cpu: cpu,
	}

	cost := CostTriple(cf, x, y, z)
	return m.spendBudget(cost)
}

func (m *Machine[T]) CostFour(
	b *builtin.DefaultFunction,
	x, y, z, u func() ExMem,
) error {
	model := m.costs.builtinCosts[*b]

	mem := model.mem.(FourArgument)
	cpu := model.cpu.(FourArgument)

	cf := CostingFunc[FourArgument]{
		mem: mem,
		cpu: cpu,
	}

	cost := CostQuadtuple(cf, x, y, z, u)
	return m.spendBudget(cost)
}

func (m *Machine[T]) CostSix(
	b *builtin.DefaultFunction,
	x, y, z, xx, yy, zz func() ExMem,
) error {
	model := m.costs.builtinCosts[*b]

	mem := model.mem.(SixArgument)
	cpu := model.cpu.(SixArgument)

	cf := CostingFunc[SixArgument]{
		mem: mem,
		cpu: cpu,
	}

	cost := CostSextuple(cf, x, y, z, xx, yy, zz)
	return m.spendBudget(cost)
}

func unwrapConstant[T syn.Eval](value Value[T]) (*Constant, error) {
	var i *Constant

	switch v := value.(type) {
	case *Constant:
		i = v

	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapInteger[T syn.Eval](value Value[T]) (*big.Int, error) {
	var i *big.Int

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Integer:
			i = c.Inner
		default:
			return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Integer", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant Integer", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

type integerInfo struct {
	value    *big.Int
	exMem    ExMem
	int64Val int64
	isInt64  bool
}

func unwrapIntegerInfo[T syn.Eval](value Value[T]) (integerInfo, error) {
	var info integerInfo

	c, ok := value.(*Constant)
	if !ok {
		return integerInfo{}, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Constant Integer",
			Got:      fmt.Sprintf("%T", value),
			Message:  "type mismatch",
		}
	}

	i, ok := c.Constant.(*syn.Integer)
	if !ok {
		return integerInfo{}, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Integer",
			Got:      fmt.Sprintf("%T", c.Constant),
			Message:  "type mismatch",
		}
	}

	info.value = i.Inner
	if i.Inner.IsInt64() {
		info.isInt64 = true
		info.int64Val = i.Inner.Int64()
		info.exMem = ExMem(1)
		return info, nil
	}
	if i.Inner.IsUint64() {
		info.exMem = ExMem(1)
		return info, nil
	}

	info.exMem = bigIntExMemValue(i.Inner)
	return info, nil
}

func unwrapByteString[T syn.Eval](value Value[T]) ([]byte, error) {
	var i []byte

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.ByteString:
			i = c.Inner
		default:
			return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "ByteString", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant ByteString", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapString[T syn.Eval](value Value[T]) (string, error) {
	var i string

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.String:
			i = c.Inner
		default:
			return "", &TypeError{Code: ErrCodeTypeMismatch, Expected: "String", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return "", &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant String", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapBool[T syn.Eval](value Value[T]) (bool, error) {
	var i bool

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Bool:
			i = c.Inner
		default:
			return false, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Bool", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return false, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant Bool", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapUnit[T syn.Eval](value Value[T]) error {
	switch v := value.(type) {
	case *Constant:
		switch v.Constant.(type) {
		case *syn.Unit:
			return nil
		default:
			return &TypeError{Code: ErrCodeTypeMismatch, Expected: "Unit", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant Unit", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}
}

func unwrapList[T syn.Eval](
	typ syn.Typ,
	value Value[T],
) (*syn.ProtoList, error) {
	var i *syn.ProtoList

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.ProtoList:
			if typ != nil && !reflect.DeepEqual(typ, c.LTyp) {
				return nil, fmt.Errorf("Value not a List of type %T", typ)
			}

			i = c
		default:
			return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "List", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant List", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapPair[T syn.Eval](
	value Value[T],
) (syn.IConstant, syn.IConstant, error) {
	var i syn.IConstant
	var j syn.IConstant

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.ProtoPair:
			i = c.First
			j = c.Second
		default:
			return nil, nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Pair", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant Pair", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, j, nil
}

func unwrapData[T syn.Eval](value Value[T]) (data.PlutusData, error) {
	var i data.PlutusData

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Data:
			i = c.Inner
		default:
			return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Data", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant Data", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapBls12_381G1Element[T syn.Eval](value Value[T]) (*bls.G1Jac, error) {
	var i *bls.G1Jac

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Bls12_381G1Element:
			i = c.Inner
		default:
			return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "G1Element", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant G1Element", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapBls12_381G2Element[T syn.Eval](value Value[T]) (*bls.G2Jac, error) {
	var i *bls.G2Jac

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Bls12_381G2Element:
			i = c.Inner
		default:
			return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "G2Element", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant G2Element", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}

func unwrapBls12_381MlResult[T syn.Eval](value Value[T]) (*bls.GT, error) {
	var i *bls.GT

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Bls12_381MlResult:
			i = c.Inner
		default:
			return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "MlResult", Got: fmt.Sprintf("%T", v.Constant), Message: "type mismatch"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant MlResult", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
	}

	return i, nil
}
