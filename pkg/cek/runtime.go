package cek

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/data"
	"github.com/blinklabs-io/plutigo/pkg/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

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

func (m *Machine[T]) evalBuiltinApp(b *Builtin[T]) (Value[T], error) {
	switch b.Func {
	case builtin.AddInteger:
		return addInteger(m, b)

	case builtin.SubtractInteger:
		return subtractInteger(m, b)

	case builtin.MultiplyInteger:
		return multiplyInteger(m, b)

	case builtin.DivideInteger:
		return divideInteger(m, b)

	case builtin.QuotientInteger:
		return quotientInteger(m, b)

	case builtin.RemainderInteger:
		return remainderInteger(m, b)

	case builtin.ModInteger:
		return modInteger(m, b)

	case builtin.EqualsInteger:
		return equalsInteger(m, b)

	case builtin.LessThanInteger:
		return lessThanInteger(m, b)

	case builtin.LessThanEqualsInteger:
		return lessThanEqualsInteger(m, b)

	case builtin.AppendByteString:
		return appendByteString(m, b)

	case builtin.ConsByteString:
		return consByteString(m, b)

	case builtin.SliceByteString:
		return sliceByteString(m, b)

	case builtin.LengthOfByteString:
		return lengthOfByteString(m, b)

	case builtin.IndexByteString:
		return indexByteString(m, b)

	case builtin.EqualsByteString:
		return equalsByteString(m, b)

	case builtin.LessThanByteString:
		return lessThanByteString(m, b)

	case builtin.LessThanEqualsByteString:
		return lessThanEqualsByteString(m, b)

	case builtin.Sha2_256:
		return sha2256(m, b)

	case builtin.Sha3_256:
		return sha3256(m, b)

	case builtin.Blake2b_256:
		return blake2B256(m, b)

	case builtin.VerifyEd25519Signature:
		return verifyEd25519Signature(m, b)

	case builtin.Blake2b_224:
		return blake2B224(m, b)

	case builtin.Keccak_256:
		return keccak256(m, b)

	case builtin.VerifyEcdsaSecp256k1Signature:
		return verifyEcdsaSecp256K1Signature(m, b)

	case builtin.VerifySchnorrSecp256k1Signature:
		return verifySchnorrSecp256K1Signature(m, b)

	case builtin.AppendString:
		return appendString(m, b)

	case builtin.EqualsString:
		return equalsString(m, b)

	case builtin.EncodeUtf8:
		return encodeUtf8(m, b)

	case builtin.DecodeUtf8:
		return decodeUtf8(m, b)

	case builtin.IfThenElse:
		return ifThenElse(m, b)

	case builtin.ChooseUnit:
		return chooseUnit(m, b)

	case builtin.Trace:
		return trace(m, b)

	case builtin.FstPair:
		return fstPair(m, b)

	case builtin.SndPair:
		return sndPair(m, b)

	case builtin.ChooseList:
		return chooseList(m, b)

	case builtin.MkCons:
		return mkCons(m, b)

	case builtin.HeadList:
		return headList(m, b)

	case builtin.TailList:
		return tailList(m, b)

	case builtin.NullList:
		return nullList(m, b)

	case builtin.ChooseData:
		return chooseData(m, b)

	case builtin.ConstrData:
		return constrData(m, b)

	case builtin.MapData:
		return mapData(m, b)

	case builtin.ListData:
		return listData(m, b)

	case builtin.IData:
		return iData(m, b)

	case builtin.BData:
		return bData(m, b)

	case builtin.UnConstrData:
		return unConstrData(m, b)

	case builtin.UnMapData:
		return unMapData(m, b)

	case builtin.UnListData:
		return unListData(m, b)

	case builtin.UnIData:
		return unIData(m, b)

	case builtin.UnBData:
		return unBData(m, b)

	case builtin.EqualsData:
		return equalsData(m, b)

	case builtin.SerialiseData:
		return serialiseData(m, b)

	case builtin.MkPairData:
		return mkPairData(m, b)

	case builtin.MkNilData:
		return mkNilData(m, b)

	case builtin.MkNilPairData:
		return mkNilPairData(m, b)

	case builtin.Bls12_381_G1_Add:
		return bls12381G1Add(m, b)

	case builtin.Bls12_381_G1_Neg:
		return bls12381G1Neg(m, b)

	case builtin.Bls12_381_G1_ScalarMul:
		return bls12381G1ScalarMul(m, b)

	case builtin.Bls12_381_G1_Equal:
		return bls12381G1Equal(m, b)

	case builtin.Bls12_381_G1_Compress:
		return bls12381G1Compress(m, b)

	case builtin.Bls12_381_G1_Uncompress:
		return bls12381G1Uncompress(m, b)

	case builtin.Bls12_381_G1_HashToGroup:
		return bls12381G1HashToGroup(m, b)

	case builtin.Bls12_381_G2_Add:
		return bls12381G2Add(m, b)

	case builtin.Bls12_381_G2_Neg:
		return bls12381G2Neg(m, b)

	case builtin.Bls12_381_G2_ScalarMul:
		return bls12381G2ScalarMul(m, b)

	case builtin.Bls12_381_G2_Equal:
		return bls12381G2Equal(m, b)

	case builtin.Bls12_381_G2_Compress:
		return bls12381G2Compress(m, b)

	case builtin.Bls12_381_G2_Uncompress:
		return bls12381G2Uncompress(m, b)

	case builtin.Bls12_381_G2_HashToGroup:
		return bls12381G2HashToGroup(m, b)

	case builtin.Bls12_381_MillerLoop:
		return bls12381MillerLoop(m, b)

	case builtin.Bls12_381_MulMlResult:
		return bls12381MulMlResult(m, b)

	case builtin.Bls12_381_FinalVerify:
		return bls12381FinalVerify(m, b)

	case builtin.IntegerToByteString:
		return integerToByteString(m, b)

	case builtin.ByteStringToInteger:
		return byteStringToInteger(m, b)

	case builtin.AndByteString:
		return andByteString(m, b)

	case builtin.OrByteString:
		return orByteString(m, b)

	case builtin.XorByteString:
		return xorByteString(m, b)

	case builtin.ComplementByteString:
		return complementByteString(m, b)

	case builtin.ReadBit:
		return readBit(m, b)

	case builtin.WriteBits:
		return writeBits(m, b)

	case builtin.ReplicateByte:
		return replicateByte(m, b)

	case builtin.ShiftByteString:
		return shiftByteString(m, b)

	case builtin.RotateByteString:
		return rotateByteString(m, b)

	case builtin.CountSetBits:
		return countSetBits(m, b)

	case builtin.FindFirstSetBit:
		return findFirstSetBit(m, b)

	case builtin.Ripemd_160:
		return ripemd160(m, b)

	default:
		panic(fmt.Sprintf("unknown builtin: %v", b.Func))
	}
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
				return nil, fmt.Errorf("Value not a List of type %v", typ)
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
