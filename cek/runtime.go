package cek

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

const IntegerToByteStringMaximumOutputLength = 8192

type Builtins[T syn.Eval] [builtin.TotalBuiltinCount]func(*Machine[T], *Builtin[T]) (Value[T], error)

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

		builtin.Bls12_381_G1_Add:         bls12381G1Add[T],
		builtin.Bls12_381_G1_Neg:         bls12381G1Neg[T],
		builtin.Bls12_381_G1_ScalarMul:   bls12381G1ScalarMul[T],
		builtin.Bls12_381_G1_Equal:       bls12381G1Equal[T],
		builtin.Bls12_381_G1_Compress:    bls12381G1Compress[T],
		builtin.Bls12_381_G1_Uncompress:  bls12381G1Uncompress[T],
		builtin.Bls12_381_G1_HashToGroup: bls12381G1HashToGroup[T],
		builtin.Bls12_381_G2_Add:         bls12381G2Add[T],
		builtin.Bls12_381_G2_Neg:         bls12381G2Neg[T],
		builtin.Bls12_381_G2_ScalarMul:   bls12381G2ScalarMul[T],
		builtin.Bls12_381_G2_Equal:       bls12381G2Equal[T],
		builtin.Bls12_381_G2_Compress:    bls12381G2Compress[T],
		builtin.Bls12_381_G2_Uncompress:  bls12381G2Uncompress[T],
		builtin.Bls12_381_G2_HashToGroup: bls12381G2HashToGroup[T],
		builtin.Bls12_381_MillerLoop:     bls12381MillerLoop[T],
		builtin.Bls12_381_MulMlResult:    bls12381MulMlResult[T],
		builtin.Bls12_381_FinalVerify:    bls12381FinalVerify[T],

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
		builtin.CaseList:      caseList[T],
		builtin.CaseData:      caseData[T],
		builtin.DropList:      dropList[T],
		// Arrays
		builtin.LengthOfArray: lengthOfArray[T],
		builtin.ListToArray:   listToArray[T],
		builtin.IndexArray:    indexArray[T],
	}
}

func (m *Machine[T]) evalBuiltinApp(b *Builtin[T]) (Value[T], error) {
	return m.builtins[b.Func](m, b)
}

func (m *Machine[T]) CostOne(b *builtin.DefaultFunction, x func() ExMem) error {
	model := m.costs.builtinCosts[*b]

	mem := model.mem.(OneArgument)
	cpu := model.cpu.(OneArgument)

	cf := CostingFunc[OneArgument]{
		mem: mem,
		cpu: cpu,
	}

	err := m.spendBudget(CostSingle(cf, x))

	return err
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

	err := m.spendBudget(CostPair(cf, x, y))

	return err
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

	err := m.spendBudget(CostTriple(cf, x, y, z))

	return err
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

	err := m.spendBudget(CostSextuple(cf, x, y, z, xx, yy, zz))

	return err
}

func unwrapConstant[T syn.Eval](value Value[T]) (*Constant, error) {
	var i *Constant

	switch v := value.(type) {
	case *Constant:
		i = v

	default:
		return nil, errors.New("Value not a Constant")
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
			return nil, errors.New("Value not an Integer")
		}
	default:
		return nil, errors.New("Value not a Constant Integer")
	}

	return i, nil
}

func unwrapByteString[T syn.Eval](value Value[T]) ([]byte, error) {
	var i []byte

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.ByteString:
			i = c.Inner
		default:
			return nil, errors.New("Value not a ByteString")
		}
	default:
		return nil, errors.New("Value not a Constant ByteString")
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
			return "", errors.New("Value not a String")
		}
	default:
		return "", errors.New("Value not a Constant String")
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
			return false, errors.New("Value not a Bool")
		}
	default:
		return false, errors.New("Value not a Constant Bool")
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
			return errors.New("Value not a Unit")
		}
	default:
		return errors.New("Value not a Constant Unit")
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
			return nil, errors.New("Value not a List")
		}
	default:
		return nil, errors.New("Value not a Constant List")
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
			return nil, nil, errors.New("Value not a Pair")
		}
	default:
		return nil, nil, errors.New("Value not a Constant Pair")
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
			return nil, errors.New("Value not a Data")
		}
	default:
		return nil, errors.New("Value not a Constant Data")
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
			return nil, errors.New("Value not a G1Element")
		}
	default:
		return nil, errors.New("Value not a Constant G1Element")
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
			return nil, errors.New("Value not a G2Element")
		}
	default:
		return nil, errors.New("Value not a Constant G2Element")
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
			return nil, errors.New("Value not an MlResult")
		}
	default:
		return nil, errors.New("Value not a Constant MlResult")
	}

	return i, nil
}
