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
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"

	"golang.org/x/crypto/blake2b"
	legacysha3 "golang.org/x/crypto/sha3"
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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
			return nil, errors.New("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Div(arg1, arg2) // Division (rounds toward zero)

		evalValue = &Constant{
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
			return nil, errors.New("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Quo(
			arg1,
			arg2,
		) // Floor division (rounds toward negative infinity)

		evalValue = &Constant{
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
			return nil, errors.New("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Rem(
			arg1,
			arg2,
		) // Remainder (consistent with Div, can be negative)

		evalValue = &Constant{
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
			return nil, errors.New("division by zero")
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
		if err != nil {
			return nil, err
		}

		var newInt big.Int

		newInt.Mod(arg1, arg2) // Modulus (always non-negative)

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		err = m.CostThree(
			&b.Func,
			bigIntExMem(arg1),
			bigIntExMem(arg2),
			byteArrayExMem(arg3),
		)
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		err = m.CostThree(
			&b.Func,
			byteArrayExMem(publicKey),
			byteArrayExMem(message),
			byteArrayExMem(signature),
		)
		if err != nil {
			return nil, err
		}

		if len(publicKey) != ed25519.PublicKeySize { // 32 bytes
			return nil, fmt.Errorf(
				"invalid public key length: got %d, expected 32",
				len(publicKey),
			)
		}

		if len(signature) != ed25519.SignatureSize { // 64 bytes
			return nil, fmt.Errorf(
				"invalid signature length: got %d, expected 64",
				len(signature),
			)
		}

		res := ed25519.Verify(publicKey, message, signature)

		con := &syn.Bool{
			Inner: res,
		}

		evalValue = &Constant{
			Constant: con,
		}
	case builtin.Blake2b_224:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		hasher, err := blake2b.New(28, nil) // 28 bytes = 224 bits
		if err != nil {
			return nil, err
		}

		// Write data and compute the hash
		hasher.Write(arg1)

		res := hasher.Sum(nil)

		con := &syn.ByteString{
			Inner: res[:],
		}

		evalValue = &Constant{
			Constant: con,
		}
	case builtin.Keccak_256:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		hash := legacysha3.NewLegacyKeccak256()

		// Write data and compute the hash
		hash.Write(arg1)

		res := hash.Sum(nil)

		con := &syn.ByteString{
			Inner: res[:],
		}

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
			Constant: con,
		}
	case builtin.IfThenElse:
		arg1, err := unwrapBool[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2 := b.Args[1]
		arg3 := b.Args[2]

		err = m.CostThree(
			&b.Func,
			boolExMem(arg1),
			valueExMem[T](arg2),
			valueExMem[T](arg3),
		)
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

		evalValue = &Constant{
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

		evalValue = &Constant{
			Constant: sndPair,
		}
	case builtin.ChooseList:
		l, err := unwrapList[T](nil, b.Args[0])
		if err != nil {
			return nil, err
		}

		branchEmpty := b.Args[1]

		branchOtherwise := b.Args[2]

		err = m.CostThree(
			&b.Func,
			listExMem(l.List),
			valueExMem[T](branchEmpty),
			valueExMem[T](branchOtherwise),
		)
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
			Constant: &syn.Bool{
				Inner: len(arg1.List) == 0,
			},
		}
	case builtin.ChooseData:
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		constrBranch := b.Args[1]
		mapBranch := b.Args[2]
		listBranch := b.Args[3]
		integerBranch := b.Args[4]
		bytesBranch := b.Args[5]

		err = m.CostSix(&b.Func,
			dataExMem(arg1),
			valueExMem[T](constrBranch),
			valueExMem[T](mapBranch),
			valueExMem[T](listBranch),
			valueExMem[T](integerBranch),
			valueExMem[T](bytesBranch),
		)
		if err != nil {
			return nil, err
		}

		switch arg1.(type) {
		case *data.Constr:
			evalValue = constrBranch
		case *data.Map:
			evalValue = mapBranch
		case *data.List:
			evalValue = listBranch
		case *data.Integer:
			evalValue = integerBranch
		case *data.ByteString:
			evalValue = bytesBranch
		default:
			panic("unreachable")
		}
	case builtin.ConstrData:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapList[T](syn.TData{}, b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), listExMem(arg2.List))
		if err != nil {
			return nil, err
		}

		var dataList []data.PlutusData

		for _, item := range arg2.List {
			itemData := item.(syn.Data)
			dataList = append(dataList, itemData.Inner)
		}

		evalValue = Constant{
			Constant: &syn.Data{
				Inner: &data.Constr{
					Tag:    uint(arg1.Int64()),
					Fields: dataList,
				},
			},
		}
	case builtin.MapData:

		pairType := syn.TPair{
			First:  syn.TData{},
			Second: syn.TData{},
		}
		arg1, err := unwrapList[T](pairType, b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, listExMem(arg1.List))
		if err != nil {
			return nil, err
		}

		var dataList [][2]data.PlutusData

		for _, item := range arg1.List {
			pair := item.(syn.ProtoPair)
			fst := pair.First.(syn.Data)
			snd := pair.Second.(syn.Data)

			dataList = append(dataList, [2]data.PlutusData{fst.Inner, snd.Inner})
		}

		evalValue = Constant{
			Constant: &syn.Data{
				Inner: &data.Map{
					Pairs: dataList,
				},
			},
		}
	case builtin.ListData:
		arg1, err := unwrapList[T](syn.TData{}, b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, listExMem(arg1.List))
		if err != nil {
			return nil, err
		}

		var dataList []data.PlutusData

		for _, item := range arg1.List {
			itemData := item.(syn.Data)
			dataList = append(dataList, itemData.Inner)
		}

		evalValue = Constant{
			Constant: &syn.Data{
				Inner: &data.List{
					Items: dataList,
				},
			},
		}
	case builtin.IData:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, bigIntExMem(arg1))
		if err != nil {
			return nil, err
		}

		evalValue = &Constant{&syn.Data{
			Inner: &data.Integer{Inner: arg1},
		}}
	case builtin.BData:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		evalValue = &Constant{&syn.Data{
			Inner: &data.ByteString{Inner: arg1},
		}}
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

		evalValue = &Constant{
			Constant: pair,
		}
	case builtin.UnMapData:
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, dataExMem(arg1))
		if err != nil {
			return nil, err
		}

		var dataMap *syn.ProtoList
		switch l := arg1.(type) {
		case *data.Map:
			var items []syn.IConstant

			for _, item := range l.Pairs {
				pair := &syn.ProtoPair{
					FstType: syn.TData{},
					SndType: syn.TData{},
					First: syn.Data{
						Inner: item[0],
					},
					Second: syn.Data{
						Inner: item[1],
					},
				}

				items = append(items, pair)
			}

			dataMap = &syn.ProtoList{
				LTyp: syn.TPair{
					First:  syn.TData{},
					Second: syn.TData{},
				},
				List: items,
			}
		default:
			return nil, errors.New("data is not a map")
		}

		evalValue = &Constant{
			Constant: dataMap,
		}
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

		evalValue = &Constant{
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

		evalValue = &Constant{
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

		evalValue = &Constant{
			Constant: bytes,
		}
	case builtin.EqualsData:
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapData[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		costX, costY := equalsDataExMem(arg1, arg2)
		err = m.CostTwo(&b.Func, costX, costY)
		if err != nil {
			return nil, err
		}

		result := &syn.Bool{
			Inner: reflect.DeepEqual(arg1, arg2),
		}

		evalValue = Constant{
			Constant: result,
		}
	case builtin.SerialiseData:
		panic("implement SerialiseData")
	case builtin.MkPairData:
		arg1, err := unwrapData[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapData[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, dataExMem(arg1), dataExMem(arg2))
		if err != nil {
			return nil, err
		}

		pair := syn.ProtoPair{
			FstType: syn.TData{},
			SndType: syn.TData{},
			First: &syn.Data{
				Inner: arg1,
			},
			Second: &syn.Data{
				Inner: arg2,
			},
		}

		evalValue = Constant{
			Constant: &pair,
		}

	case builtin.MkNilData:
		err := unwrapUnit[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, unitExMem())
		if err != nil {
			return nil, err
		}

		l := syn.ProtoList{
			LTyp: syn.TData{},
			List: []syn.IConstant{},
		}

		evalValue = Constant{
			Constant: &l,
		}
	case builtin.MkNilPairData:
		err := unwrapUnit[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, unitExMem())
		if err != nil {
			return nil, err
		}

		l := syn.ProtoList{
			LTyp: syn.TPair{
				First:  syn.TData{},
				Second: syn.TData{},
			},
			List: []syn.IConstant{},
		}

		evalValue = Constant{
			Constant: &l,
		}
	case builtin.Bls12_381_G1_Add:
		arg1, err := unwrapBls12_381G1Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381G1Element[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, blsG1ExMem(), blsG1ExMem())
		if err != nil {
			return nil, err
		}

		newG1 := new(bls.G1Affine).Add(arg1, arg2)

		evalValue = &Constant{
			Constant: &syn.Bls12_381G1Element{
				Inner: newG1,
			},
		}
	case builtin.Bls12_381_G1_Neg:
		arg1, err := unwrapBls12_381G1Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, blsG1ExMem())
		if err != nil {
			return nil, err
		}

		g1Neg := new(bls.G1Affine).Neg(arg1)

		evalValue = &Constant{
			Constant: &syn.Bls12_381G1Element{
				Inner: g1Neg,
			},
		}
	case builtin.Bls12_381_G1_ScalarMul:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381G1Element[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), blsG1ExMem())
		if err != nil {
			return nil, err
		}

		newG1 := arg2.ScalarMultiplicationBase(arg1)

		evalValue = &Constant{
			Constant: &syn.Bls12_381G1Element{
				Inner: newG1,
			},
		}
	case builtin.Bls12_381_G1_Equal:
		arg1, err := unwrapBls12_381G1Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381G1Element[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, blsG1ExMem(), blsG1ExMem())
		if err != nil {
			return nil, err
		}

		isEqual := arg1.Equal(arg2)

		evalValue = &Constant{
			Constant: &syn.Bool{
				Inner: isEqual,
			},
		}
	case builtin.Bls12_381_G1_Compress:
		arg1, err := unwrapBls12_381G1Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, blsG1ExMem())
		if err != nil {
			return nil, err
		}

		b := arg1.Bytes()

		evalValue = &Constant{
			Constant: &syn.ByteString{
				Inner: b[:],
			},
		}
	case builtin.Bls12_381_G1_Uncompress:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		uncompressed := new(bls.G1Affine)

		_, err = uncompressed.SetBytes(arg1)
		if err != nil {
			return nil, err
		}

		evalValue = &Constant{
			Constant: &syn.Bls12_381G1Element{
				Inner: uncompressed,
			},
		}
	case builtin.Bls12_381_G1_HashToGroup:
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

		if len(arg2) > 255 {
			return nil, errors.New("hash to curve dst too big")
		}

		point, err := bls.HashToG1(arg1, arg2)
		if err != nil {
			return nil, err
		}

		evalValue = &Constant{
			Constant: &syn.Bls12_381G1Element{
				Inner: &point,
			},
		}
	case builtin.Bls12_381_G2_Add:
		arg1, err := unwrapBls12_381G2Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381G2Element[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, blsG2ExMem(), blsG2ExMem())
		if err != nil {
			return nil, err
		}

		newG2 := new(bls.G2Affine).Add(arg1, arg2)

		evalValue = &Constant{
			Constant: &syn.Bls12_381G2Element{
				Inner: newG2,
			},
		}
	case builtin.Bls12_381_G2_Neg:
		arg1, err := unwrapBls12_381G2Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, blsG2ExMem())
		if err != nil {
			return nil, err
		}

		g1Neg := new(bls.G2Affine).Neg(arg1)

		evalValue = &Constant{
			Constant: &syn.Bls12_381G2Element{
				Inner: g1Neg,
			},
		}
	case builtin.Bls12_381_G2_ScalarMul:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381G2Element[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, bigIntExMem(arg1), blsG2ExMem())
		if err != nil {
			return nil, err
		}

		newG2 := arg2.ScalarMultiplicationBase(arg1)

		evalValue = &Constant{
			Constant: &syn.Bls12_381G2Element{
				Inner: newG2,
			},
		}
	case builtin.Bls12_381_G2_Equal:
		arg1, err := unwrapBls12_381G2Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381G2Element[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, blsG2ExMem(), blsG2ExMem())
		if err != nil {
			return nil, err
		}

		isEqual := arg1.Equal(arg2)

		evalValue = &Constant{
			Constant: &syn.Bool{
				Inner: isEqual,
			},
		}
	case builtin.Bls12_381_G2_Compress:
		arg1, err := unwrapBls12_381G2Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, blsG2ExMem())
		if err != nil {
			return nil, err
		}

		b := arg1.Bytes()

		evalValue = &Constant{
			Constant: &syn.ByteString{
				Inner: b[:],
			},
		}
	case builtin.Bls12_381_G2_Uncompress:
		arg1, err := unwrapByteString[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		err = m.CostOne(&b.Func, byteArrayExMem(arg1))
		if err != nil {
			return nil, err
		}

		uncompressed := new(bls.G2Affine)

		_, err = uncompressed.SetBytes(arg1)
		if err != nil {
			return nil, err
		}

		evalValue = &Constant{
			Constant: &syn.Bls12_381G2Element{
				Inner: uncompressed,
			},
		}
	case builtin.Bls12_381_G2_HashToGroup:
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

		if len(arg2) > 255 {
			return nil, errors.New("hash to curve dst too big")
		}

		point, err := bls.HashToG2(arg1, arg2)
		if err != nil {
			return nil, err
		}

		evalValue = &Constant{
			Constant: &syn.Bls12_381G2Element{
				Inner: &point,
			},
		}
	case builtin.Bls12_381_MillerLoop:
		arg1, err := unwrapBls12_381G1Element[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381G2Element[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, blsG1ExMem(), blsG2ExMem())
		if err != nil {
			return nil, err
		}

		mlResult, err := bls.MillerLoop([]bls.G1Affine{*arg1}, []bls.G2Affine{*arg2})

		evalValue = &Constant{
			Constant: &syn.Bls12_381MlResult{
				Inner: &mlResult,
			},
		}
	case builtin.Bls12_381_MulMlResult:
		arg1, err := unwrapBls12_381MlResult[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381MlResult[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, blsMlResultExMem(), blsMlResultExMem())
		if err != nil {
			return nil, err
		}

		newMl := new(bls.GT).Mul(arg1, arg2)

		evalValue = &Constant{
			Constant: &syn.Bls12_381MlResult{
				Inner: newMl,
			},
		}
	case builtin.Bls12_381_FinalVerify:
		arg1, err := unwrapBls12_381MlResult[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapBls12_381MlResult[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		err = m.CostTwo(&b.Func, blsMlResultExMem(), blsMlResultExMem())
		if err != nil {
			return nil, err
		}

		verified := arg1.Equal(arg2)

		evalValue = &Constant{
			Constant: &syn.Bool{
				Inner: verified,
			},
		}
	case builtin.IntegerToByteString:
		panic("implement IntegerToByteString")
	case builtin.ByteStringToInteger:
		panic("implement ByteStringToInteger")
	case builtin.AndByteString:
		panic("implement AndByteString")
	case builtin.OrByteString:
		panic("implement OrByteString")
	case builtin.XorByteString:
		panic("implement XorByteString")
	case builtin.ComplementByteString:
		panic("implement ComplementByteString")
	case builtin.ReadBit:
		panic("implement ReadBit")
	case builtin.WriteBits:
		panic("implement WriteBits")
	case builtin.ReplicateByte:
		panic("implement ReplicateByte")
	case builtin.ShiftByteString:
		panic("implement ShiftByteString")
	case builtin.RotateByteString:
		panic("implement RotateByteString")
	case builtin.CountSetBits:
		panic("implement CountSetBits")
	case builtin.FindFirstSetBit:
		panic("implement FindFirstSetBit")
	case builtin.Ripemd_160:
		panic("implement Ripemd_160")
	default:
		panic(fmt.Sprintf("unknown builtin: %v", b.Func))
	}

	return evalValue, nil
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
		return nil, errors.New("Value not a Constant")
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
		return nil, errors.New("Value not a Constant")
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
		return "", errors.New("Value not a Constant")
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
		return false, errors.New("Value not a Constant")
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
		return errors.New("Value not a Constant")
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
		return nil, errors.New("Value not a Constant")
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
		return nil, nil, errors.New("Value not a Constant")
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
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}

func unwrapBls12_381G1Element[T syn.Eval](value Value[T]) (*bls.G1Affine, error) {
	var i *bls.G1Affine

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Bls12_381G1Element:
			i = c.Inner
		default:
			return nil, errors.New("Value not a G1Element")
		}
	default:
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}

func unwrapBls12_381G2Element[T syn.Eval](value Value[T]) (*bls.G2Affine, error) {
	var i *bls.G2Affine

	switch v := value.(type) {
	case *Constant:
		switch c := v.Constant.(type) {
		case *syn.Bls12_381G2Element:
			i = c.Inner
		default:
			return nil, errors.New("Value not a G2Element")
		}
	default:
		return nil, errors.New("Value not a Constant")
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
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}
