package cek

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha3"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/data"
	"github.com/blinklabs-io/plutigo/pkg/syn"
	"golang.org/x/crypto/blake2b"
)

func (m *Machine[T]) CostOne(b *builtin.DefaultFunction, x func() ExMem) error {
	model := m.costs.builtinCosts[*b]

	mem, _ := model.mem.(OneArgument)
	cpu, _ := model.cpu.(OneArgument)

	cf := CostingFunc[OneArgument]{
		mem: mem,
		cpu: cpu,
	}

	err := m.spendBudget(CostSingle(cf, x))

	return err
}

func (m *Machine[T]) CostTwo(b *builtin.DefaultFunction, x, y func() ExMem) error {
	model := m.costs.builtinCosts[*b]

	mem, _ := model.mem.(TwoArgument)
	cpu, _ := model.cpu.(TwoArgument)

	cf := CostingFunc[TwoArgument]{
		mem: mem,
		cpu: cpu,
	}

	err := m.spendBudget(CostPair(cf, x, y))

	return err
}

func (m *Machine[T]) CostThree(b *builtin.DefaultFunction, x, y, z func() ExMem) error {
	model := m.costs.builtinCosts[*b]

	mem, _ := model.mem.(ThreeArgument)
	cpu, _ := model.cpu.(ThreeArgument)

	cf := CostingFunc[ThreeArgument]{
		mem: mem,
		cpu: cpu,
	}

	err := m.spendBudget(CostTriple(cf, x, y, z))

	return err
}

func (m *Machine[T]) Costsix(b *builtin.DefaultFunction, x, y, z, xx, yy, zz func() ExMem) error {
	model := m.costs.builtinCosts[*b]

	mem, _ := model.mem.(SixArgument)
	cpu, _ := model.cpu.(SixArgument)

	cf := CostingFunc[SixArgument]{
		mem: mem,
		cpu: cpu,
	}

	err := m.spendBudget(CostSextuple(cf, x, y, z, xx, yy, zz))

	return err
}

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

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

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

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

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

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Mul(arg1, arg2)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.DivideInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}
		// Check for division by zero

		if arg2.Sign() == 0 {
			return nil, fmt.Errorf("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Div(arg1, arg2) // Division (rounds toward zero)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.QuotientInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// Check for division by zero

		if arg2.Sign() == 0 {
			return nil, fmt.Errorf("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Quo(arg1, arg2) // Floor division (rounds toward negative infinity)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.RemainderInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// Check for division by zero

		if arg2.Sign() == 0 {
			return nil, fmt.Errorf("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Rem(arg1, arg2) // Remainder (consistent with Div, can be negative)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.ModInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// Check for division by zero
		if arg2.Sign() == 0 {
			return nil, fmt.Errorf("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Mod(arg1, arg2) // Modulus (always non-negative)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}
	case builtin.EqualsInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := arg1.Cmp(arg2)

		con := &syn.Bool{
			Inner: res == 0,
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

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := arg1.Cmp(arg2)

		con := &syn.Bool{
			Inner: res == -1,
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

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := arg1.Cmp(arg2)

		con := &syn.Bool{
			Inner: res == -1 || res == 0,
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

		err = m.CostTwo(&b.Func, byteArrayExMem(arg1), byteArrayExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := make([]byte, len(arg1)+len(arg2))

		copy(res, arg1)
		copy(res[len(arg1):], arg2)

		evalValue = Constant{
			Constant: &syn.ByteString{
				Inner: res,
			},
		}
	case builtin.ConsByteString:

		arg1, err := unwrapInteger[T](b.Args[0]) // skip
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapByteString[T](b.Args[1]) // byte string
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), byteArrayExMem(arg2))
		if err != nil {
			return nil, err
		}

		// TODO: handle PlutusSemantic Versioning

		if !arg1.IsInt64() {
			return nil, errors.New("int does not fit into a byte")
		}

		int_val := arg1.Int64()

		if int_val < 0 || int_val > 255 {
			return nil, errors.New("int does not fit into a byte")
		}

		res := append([]byte{byte(int_val)}, arg2...)

		evalValue = Constant{
			Constant: &syn.ByteString{
				Inner: res,
			},
		}
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

		err = m.CostThree(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2), byteArrayExMem(arg3))
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
		end := min(skip+take, len(arg3))

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

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

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

		err = m.CostTwo(&b.Func, byteArrayExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

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

		err = m.CostTwo(&b.Func, byteArrayExMem(arg1), byteArrayExMem(arg2))
		if err != nil {
			return nil, err
		}

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

		err = m.CostTwo(&b.Func, byteArrayExMem(arg1), byteArrayExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := bytes.Compare(arg1, arg2)

		con := &syn.Bool{
			Inner: res == -1,
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

		err = m.CostTwo(&b.Func, byteArrayExMem(arg1), byteArrayExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := bytes.Compare(arg1, arg2)

		con := &syn.Bool{
			Inner: res == -1 || res == 0,
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.Sha2_256:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		res := sha256.Sum256(arg1)

		con := &syn.ByteString{
			Inner: res[:],
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.Sha3_256:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		res := sha3.Sum256(arg1)

		con := &syn.ByteString{
			Inner: res[:],
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.Blake2b_256:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		res := blake2b.Sum256(arg1)

		con := &syn.ByteString{
			Inner: res[:],
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.VerifyEd25519Signature:
		publicKey, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		message, err := unwrapByteString[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		signature, err := unwrapByteString[T](b.Args[2])
		if err != nil {
			return nil, err
		}

		err = m.CostThree(&b.Func, byteArrayExMem(publicKey), byteArrayExMem(message), byteArrayExMem(signature))
		if err != nil {
			return nil, err
		}

		if len(publicKey) != ed25519.PublicKeySize { // 32 bytes
			return nil, fmt.Errorf("invalid public key length: got %d, expected 32", len(publicKey))
		}

		if len(signature) != ed25519.SignatureSize { // 64 bytes
			return nil, fmt.Errorf("invalid signature length: got %d, expected 64", len(signature))
		}

		res := ed25519.Verify(publicKey, message, signature)

		con := &syn.Bool{
			Inner: res,
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.VerifyEcdsaSecp256k1Signature:
		panic("implement VerifyEcdsaSecp256k1Signature")
	case builtin.VerifySchnorrSecp256k1Signature:
		panic("implement VerifySchnorrSecp256k1Signature")
	case builtin.AppendString:
		arg1, err := unwrapString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapString[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, stringExMem(arg1), stringExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := arg1 + arg2

		con := &syn.String{
			Inner: res,
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.EqualsString:
		arg1, err := unwrapString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapString[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, stringExMem(arg1), stringExMem(arg2))
		if err != nil {
			return nil, err
		}

		res := arg1 == arg2

		con := &syn.Bool{
			Inner: res,
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.EncodeUtf8:
		arg1, err := unwrapString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, stringExMem(arg1))
		if err != nil {
			return nil, err
		}

		res := []byte(arg1)

		con := &syn.ByteString{
			Inner: res,
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.DecodeUtf8:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		if !utf8.Valid(arg1) {
			return nil, errors.New("error decoding utf8 bytes")
		}

		res := string(arg1)

		con := &syn.String{
			Inner: res,
		}

		evalValue = Constant{
			Constant: con,
		}
	case builtin.IfThenElse:
		arg1, err := unwrapBool[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2 := b.Args[1]
		arg3 := b.Args[2]

		err = m.CostThree(&b.Func, boolExMem(arg1), valueExMem[T](arg2), valueExMem[T](arg3))
		if err != nil {
			return nil, err
		}

		if arg1 {
			evalValue = arg2
		} else {
			evalValue = arg3
		}
	case builtin.ChooseUnit:
		if err := unwrapUnit[T](b.Args[0]); err != nil {
			return nil, err
		}

		arg2 := b.Args[1]

		err := m.CostTwo(&b.Func, unitExMem(), valueExMem[T](arg2))
		if err != nil {
			return nil, err
		}

		evalValue = arg2
	case builtin.Trace:
		arg1, err := unwrapString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2 := b.Args[1]

		err = m.CostTwo(&b.Func, stringExMem(arg1), valueExMem[T](arg2))
		if err != nil {
			return nil, err
		}

		m.Logs = append(m.Logs, arg1)

		evalValue = arg2
	case builtin.FstPair:
		fstPair, sndPair, err := unwrapPair[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, pairExMem(fstPair, sndPair))
		if err != nil {
			return nil, err
		}

		evalValue = Constant{
			Constant: fstPair,
		}
	case builtin.SndPair:
		fstPair, sndPair, err := unwrapPair[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, pairExMem(fstPair, sndPair))
		if err != nil {
			return nil, err
		}

		evalValue = Constant{
			Constant: sndPair,
		}
	case builtin.ChooseList:
		l, err := unwrapList[T](nil, b.Args[0])
		if err != nil {
			return nil, err
		}

		branchEmpty := b.Args[1]

		branchOtherwise := b.Args[2]

		err = m.CostThree(&b.Func, listExMem(l.List), valueExMem[T](branchEmpty), valueExMem[T](branchOtherwise))
		if err != nil {
			return nil, err
		}

		if len(l.List) == 0 {
			evalValue = branchEmpty
		} else {
			evalValue = branchOtherwise
		}
	case builtin.MkCons:
		arg1, err := unwrapConstant[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		typ := arg1.Constant.Typ()

		arg2, err := unwrapList[T](typ, b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, valueExMem[T](arg1), listExMem(arg2.List))
		if err != nil {
			return nil, err
		}

		consList := append([]syn.IConstant{arg1.Constant}, arg2.List...)

		evalValue = Constant{
			Constant: &syn.ProtoList{
				LTyp: typ,
				List: consList,
			},
		}
	case builtin.HeadList:
		arg1, err := unwrapList[T](nil, b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, listExMem(arg1.List))
		if err != nil {
			return nil, err
		}

		if len(arg1.List) == 0 {
			return nil, errors.New("headList on an empty list")
		}

		evalValue = Constant{
			Constant: arg1.List[0],
		}
	case builtin.TailList:
		arg1, err := unwrapList[T](nil, b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, listExMem(arg1.List))
		if err != nil {
			return nil, err
		}

		if len(arg1.List) == 0 {
			return nil, errors.New("tailList on an empty list")
		}

		tailList := arg1.List[1:]

		evalValue = Constant{
			Constant: &syn.ProtoList{
				LTyp: arg1.LTyp,
				List: tailList,
			},
		}
	case builtin.NullList:
		arg1, err := unwrapList[T](nil, b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, listExMem(arg1.List))
		if err != nil {
			return nil, err
		}

		evalValue = Constant{
			Constant: &syn.Bool{
				Inner: len(arg1.List) == 0,
			},
		}
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
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, dataExMem(arg1))
		if err != nil {
			return nil, err
		}

		var pair *syn.ProtoPair
		switch constr := arg1.(type) {
		case *data.Constr:
			var fields []syn.IConstant

			for _, field := range constr.Fields {
				con := &syn.Data{
					Inner: field,
				}

				fields = append(fields, con)
			}

			pair = &syn.ProtoPair{
				FstType: syn.TInteger{},
				SndType: syn.TList{
					Typ: syn.TData{},
				},
				First: &syn.Integer{
					Inner: big.NewInt(int64(constr.Tag)),
				},
				Second: &syn.ProtoList{
					LTyp: syn.TData{},
					List: fields,
				},
			}
		default:
			return nil, errors.New("data is not a constr")
		}

		evalValue = Constant{
			Constant: pair,
		}
	case builtin.UnMapData:
		panic("implement UnMapData")
	case builtin.UnListData:
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, dataExMem(arg1))
		if err != nil {
			return nil, err
		}

		var list *syn.ProtoList
		switch l := arg1.(type) {
		case *data.List:
			var items []syn.IConstant

			for _, item := range l.Items {
				dataList := &syn.Data{
					Inner: item,
				}

				items = append(items, dataList)
			}

			list = &syn.ProtoList{
				LTyp: syn.TData{},
				List: items,
			}
		default:
			return nil, errors.New("data is not a list")
		}

		evalValue = Constant{
			Constant: list,
		}
	case builtin.UnIData:
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, dataExMem(arg1))
		if err != nil {
			return nil, err
		}

		var integer syn.IConstant

		switch b := arg1.(type) {
		case *data.Integer:
			integer = &syn.Integer{
				Inner: b.Inner,
			}

		default:
			return nil, errors.New("data is not a integer")
		}

		evalValue = Constant{
			Constant: integer,
		}
	case builtin.UnBData:
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, dataExMem(arg1))
		if err != nil {
			return nil, err
		}

		var bytes syn.IConstant

		switch b := arg1.(type) {
		case *data.ByteString:
			bytes = &syn.ByteString{
				Inner: b.Inner,
			}

		default:
			return nil, errors.New("data is not a bytearray")
		}

		evalValue = Constant{
			Constant: bytes,
		}
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

func unwrapConstant[T syn.Eval](value Value[T]) (*Constant, error) {
	var i *Constant

	switch v := value.(type) {
	case Constant:
		i = &v

	default:
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
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
			return "", errors.New("Value not a String")
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
			return false, errors.New("Value not a Bool")
		}
	default:
		return false, errors.New("Value not a Constant")
	}

	return i, nil
}

func unwrapUnit[T syn.Eval](value Value[T]) error {
	switch v := value.(type) {
	case Constant:
		switch v.Constant.(type) {
		case *syn.Unit:
			return nil
		default:
			return errors.New("Value not a Unit")
		}
	default:
		return errors.New("Value not a Constant")
	}
}

func unwrapList[T syn.Eval](typ syn.Typ, value Value[T]) (*syn.ProtoList, error) {
	var i *syn.ProtoList

	switch v := value.(type) {
	case Constant:
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
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}

func unwrapPair[T syn.Eval](value Value[T]) (syn.IConstant, syn.IConstant, error) {
	var i syn.IConstant
	var j syn.IConstant

	switch v := value.(type) {
	case Constant:
		switch c := v.Constant.(type) {
		case *syn.ProtoPair:
			i = c.First
			j = c.Second
		default:
			return nil, nil, errors.New("Value not a Pair")
		}
	default:
		return nil, nil, errors.New("Value not a Constant")
	}

	return i, j, nil
}

func unwrapData[T syn.Eval](value Value[T]) (data.PlutusData, error) {
	var i data.PlutusData

	switch v := value.(type) {
	case Constant:
		switch c := v.Constant.(type) {
		case *syn.Data:
			i = c.Inner
		default:
			return nil, errors.New("Value not a Data")
		}
	default:
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}
