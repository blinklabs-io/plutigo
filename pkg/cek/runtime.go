package cek

import (
	"bytes"
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
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapByteString[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := make([]byte, len(arg1)+len(arg2))

		copy(res, arg1)
		copy(res[len(arg1):], arg2)

		evalValue = Constant{
			Constant: &syn.ByteString{
				Inner: res,
			},
		}
	case builtin.ConsByteString:
		panic("implement ConsByteString")
	case builtin.SliceByteString:
		arg1, err := unwrapInteger[T](b.Args[0]) // skip
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1]) // take
		if err != nil {
			return nil, err
		}

		arg3, err := unwrapByteString[T](b.Args[2]) // byte string
		if err != nil {
			return nil, err
		}

		// Convert skip and take to int
		skip := 0
		if arg1.Sign() > 0 {
			if skip64, ok := arg1.Int64(), arg1.IsInt64(); ok {
				skip = int(skip64)
			} else {
				skip = len(arg3) // Clamp to max if too large
			}
		}

		take := 0
		if arg2.Sign() > 0 {
			if take64, ok := arg2.Int64(), arg2.IsInt64(); ok {
				take = int(take64)
			} else {
				take = len(arg3) - skip // Take as much as possible
			}
		}

		// Clamp end to len(arg3) to avoid out-of-bounds
		end := skip + take

		if end > len(arg3) {
			end = len(arg3)
		}

		if skip > len(arg3) {
			skip = len(arg3)
			end = len(arg3)
		}

		// Slice
		res := arg3[skip:end]

		evalValue = Constant{
			Constant: &syn.ByteString{
				Inner: res,
			},
		}
	case builtin.LengthOfByteString:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := len(arg1)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: big.NewInt(int64(res)),
			},
		}
	case builtin.IndexByteString:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		index, ok := arg2.Int64(), arg2.IsInt64()
		if !ok {
			return nil, errors.New("byte string out of bounds")
		}

		// demorgan's law on: index >= 0 && int(index) < len(arg1)
		if index < 0 || int(index) >= len(arg1) {
			return nil, errors.New("byte string out of bounds")
		}

		res := int64(arg1[index])

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: big.NewInt(res),
			},
		}
	case builtin.EqualsByteString:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapByteString[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := bytes.Equal(arg1, arg2)

		evalValue = Constant{
			Constant: &syn.Bool{
				Inner: res,
			},
		}
	case builtin.LessThanByteString:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapByteString[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := bytes.Compare(arg1, arg2)

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
	case builtin.LessThanEqualsByteString:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapByteString[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		res := bytes.Compare(arg1, arg2)

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
		arg1, err := unwrapBool[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2 := b.Args[1]
		arg3 := b.Args[2]

		// TODO: The budgeting

		if arg1 {
			evalValue = arg2
		} else {
			evalValue = arg3
		}
	case builtin.ChooseUnit:
		panic("implement ChooseUnit")
	case builtin.Trace:
		arg1, err := unwrapString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2 := b.Args[1]

		// TODO: The budgeting
		m.Logs = append(m.Logs, arg1)

		evalValue = arg2
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

func unwrapByteString[T syn.Eval](value Value[T]) ([]byte, error) {
	var i []byte

	switch v := value.(type) {
	case Constant:
		switch c := v.Constant.(type) {
		case *syn.ByteString:
			i = c.Inner
		default:
			return nil, errors.New("Value not a ByteString")
		}
	default:
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}

func unwrapString[T syn.Eval](value Value[T]) (string, error) {
	var i string

	switch v := value.(type) {
	case Constant:
		switch c := v.Constant.(type) {
		case *syn.String:
			i = c.Inner
		default:
			return "", errors.New("Value not a ByteString")
		}
	default:
		return "", errors.New("Value not a Constant")
	}

	return i, nil
}

func unwrapBool[T syn.Eval](value Value[T]) (bool, error) {
	var i bool

	switch v := value.(type) {
	case Constant:
		switch c := v.Constant.(type) {
		case *syn.Bool:
			i = c.Inner
		default:
			return false, errors.New("Value not a ByteString")
		}
	default:
		return false, errors.New("Value not a Constant")
	}

	return i, nil
}
