package cek

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

func (m *Machine[T]) evalBuiltinApp(b Builtin[T]) (Value[T], error) {
	// Budgeting
	var evalValue Value[T]

	switch b.Func {
	case builtin.AddInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		var newInt big.Int

		newInt.Add(arg1, arg2)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.SubtractInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		var newInt big.Int

		newInt.Sub(arg1, arg2)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.MultiplyInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		var newInt big.Int

		newInt.Mul(arg1, arg2)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.DivideInteger:
		panic("implement DivideInteger")
	case builtin.QuotientInteger:
		panic("implement QuotientInteger")
	case builtin.RemainderInteger:
		panic("implement RemainderInteger")
	case builtin.ModInteger:
		panic("implement ModInteger")
	case builtin.EqualsInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := arg1.Cmp(arg2)

		// assume false

		con := &syn.Bool{
			Inner: false,
		}

		// it's true
		if res == 0 {
			con.Inner = true
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.LessThanInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := arg1.Cmp(arg2)

		// assume false

		con := &syn.Bool{
			Inner: false,
		}

		// it's true
		if res == -1 {
			con.Inner = true
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.LessThanEqualsInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := arg1.Cmp(arg2)

		// assume false

		con := &syn.Bool{
			Inner: false,
		}

		// it's true
		if res == -1 || res == 0 {
			con.Inner = true
		}

		evalValue = Constant{
			Constant: con,
		}

	case builtin.AppendByteString:
		panic("implement AppendByteString")
	case builtin.ConsByteString:
		panic("implement ConsByteString")
	case builtin.SliceByteString:
		panic("implement SliceByteString")
	case builtin.LengthOfByteString:
		panic("implement LengthOfByteString")
	case builtin.IndexByteString:
		panic("implement IndexByteString")
	case builtin.EqualsByteString:
		panic("implement EqualsByteString")
	case builtin.LessThanByteString:
		panic("implement LessThanByteString")
	case builtin.LessThanEqualsByteString:
		panic("implement LessThanEqualsByteString")
	case builtin.Sha2_256:
		panic("implement Sha2_256")
	case builtin.Sha3_256:
		panic("implement Sha3_256")
	case builtin.Blake2b_256:
		panic("implement Blake2b_256")
	case builtin.VerifyEd25519Signature:
		panic("implement VerifyEd25519Signature")
	case builtin.VerifyEcdsaSecp256k1Signature:
		panic("implement VerifyEcdsaSecp256k1Signature")
	case builtin.VerifySchnorrSecp256k1Signature:
		panic("implement VerifySchnorrSecp256k1Signature")
	case builtin.AppendString:
		panic("implement AppendString")
	case builtin.EqualsString:
		panic("implement EqualsString")
	case builtin.EncodeUtf8:
		panic("implement EncodeUtf8")
	case builtin.DecodeUtf8:
		panic("implement DecodeUtf8")
	case builtin.IfThenElse:
		panic("implement IfThenElse")
	case builtin.ChooseUnit:
		panic("implement ChooseUnit")
	case builtin.Trace:
		panic("implement Trace")
	case builtin.FstPair:
		panic("implement FstPair")
	case builtin.SndPair:
		panic("implement SndPair")
	case builtin.ChooseList:
		panic("implement ChooseList")
	case builtin.MkCons:
		panic("implement MkCons")
	case builtin.HeadList:
		panic("implement HeadList")
	case builtin.TailList:
		panic("implement TailList")
	case builtin.NullList:
		panic("implement NullList")
	case builtin.ChooseData:
		panic("implement ChooseData")
	case builtin.ConstrData:
		panic("implement ConstrData")
	case builtin.MapData:
		panic("implement MapData")
	case builtin.ListData:
		panic("implement ListData")
	case builtin.IData:
		panic("implement IData")
	case builtin.BData:
		panic("implement BData")
	case builtin.UnConstrData:
		panic("implement UnConstrData")
	case builtin.UnMapData:
		panic("implement UnMapData")
	case builtin.UnListData:
		panic("implement UnListData")
	case builtin.UnIData:
		panic("implement UnIData")
	case builtin.UnBData:
		panic("implement UnBData")
	case builtin.EqualsData:
		panic("implement EqualsData")
	case builtin.SerialiseData:
		panic("implement SerialiseData")
	case builtin.MkPairData:
		panic("implement MkPairData")
	case builtin.MkNilData:
		panic("implement MkNilData")
	case builtin.MkNilPairData:
		panic("implement MkNilPairData")
	default:
		panic(fmt.Sprintf("unknown builtin: %v", b.Func))
	}

	return evalValue, nil
}

func unwrapInteger[T syn.Eval](value Value[T]) (*big.Int, error) {
	var i *big.Int

	switch v := value.(type) {
	case Constant:
		switch c := v.Constant.(type) {
		case *syn.Integer:
			i = c.Inner
		default:
			return nil, errors.New("Value not an Integer")
		}
	default:
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}
