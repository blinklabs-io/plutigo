package cek

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha3"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"reflect"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/pkg/data"
	"github.com/blinklabs-io/plutigo/pkg/syn"
	"github.com/btcsuite/btcd/btcec/v2"
	ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	schnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"golang.org/x/crypto/blake2b"
	legacyripemd160 "golang.org/x/crypto/ripemd160" //nolint:staticcheck
	legacysha3 "golang.org/x/crypto/sha3"
)

func addInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func subtractInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func multiplyInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func divideInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func quotientInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func remainderInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func modInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func equalsInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func lessThanInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func lessThanEqualsInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func appendByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.ByteString{
		Inner: res,
	}}

	return value, nil
}

func consByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.ByteString{
		Inner: res,
	}}

	return value, nil
}

func sliceByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.ByteString{
		Inner: res,
	}}

	return value, nil
}

func lengthOfByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, byteArrayExMem(arg1))
	if err != nil {
		return nil, err
	}

	res := len(arg1)

	value := &Constant{&syn.Integer{
		Inner: big.NewInt(int64(res)),
	}}

	return value, nil
}

func indexByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{
		Constant: &syn.Integer{
			Inner: big.NewInt(res),
		},
	}

	return value, nil
}

func equalsByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Bool{
		Inner: res,
	}}

	return value, nil
}

func lessThanByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func lessThanEqualsByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func sha2256[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func sha3256[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func blake2B256[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func verifyEd25519Signature[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func blake2B224[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func keccak256[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func verifyEcdsaSecp256K1Signature[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	key, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return nil, err
	}

	sig, err := ecdsa.ParseSignature(signature)
	if err != nil {
		return nil, err
	}

	res := sig.Verify(message, key)

	con := &syn.Bool{
		Inner: res,
	}

	value := &Constant{con}

	return value, nil
}

func verifySchnorrSecp256K1Signature[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	key, err := schnorr.ParsePubKey(publicKey)
	if err != nil {
		return nil, err
	}

	sig, err := schnorr.ParseSignature(signature)
	if err != nil {
		return nil, err
	}

	res := sig.Verify(message, key)

	con := &syn.Bool{
		Inner: res,
	}

	value := &Constant{con}

	return value, nil
}

func appendString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func equalsString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func encodeUtf8[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func decodeUtf8[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{con}

	return value, nil
}

func ifThenElse[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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
		return arg2, nil
	} else {
		return arg3, nil
	}
}

func chooseUnit[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	if err := unwrapUnit[T](b.Args[0]); err != nil {
		return nil, err
	}

	arg2 := b.Args[1]

	err := m.CostTwo(&b.Func, unitExMem(), valueExMem[T](arg2))
	if err != nil {
		return nil, err
	}

	value := arg2

	return value, nil
}

func trace[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := arg2

	return value, nil
}

func fstPair[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	fstPair, sndPair, err := unwrapPair[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, pairExMem(fstPair, sndPair))
	if err != nil {
		return nil, err
	}

	value := &Constant{fstPair}

	return value, nil
}

func sndPair[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	fstPair, sndPair, err := unwrapPair[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, pairExMem(fstPair, sndPair))
	if err != nil {
		return nil, err
	}

	value := &Constant{sndPair}

	return value, nil
}

func chooseList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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
		return branchEmpty, nil
	} else {
		return branchOtherwise, nil
	}
}

func mkCons[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.ProtoList{
		LTyp: typ,
		List: consList,
	}}

	return value, nil
}

func headList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{arg1.List[0]}

	return value, nil
}

func tailList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.ProtoList{
		LTyp: arg1.LTyp,
		List: tailList,
	}}

	return value, nil
}

func nullList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapList[T](nil, b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, listExMem(arg1.List))
	if err != nil {
		return nil, err
	}

	value := &Constant{&syn.Bool{
		Inner: len(arg1.List) == 0,
	}}

	return value, nil
}

func chooseData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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
		return constrBranch, nil

	case *data.Map:
		return mapBranch, nil

	case *data.List:
		return listBranch, nil

	case *data.Integer:
		return integerBranch, nil

	case *data.ByteString:
		return bytesBranch, nil

	default:
		return nil, errors.New("unexpected data variant")
	}
}

func constrData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapInteger[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapList[T](&syn.TData{}, b.Args[1])
	if err != nil {
		return nil, err
	}

	err = m.CostTwo(&b.Func, bigIntExMem(arg1), listExMem(arg2.List))
	if err != nil {
		return nil, err
	}

	dataList := []data.PlutusData{}

	for _, item := range arg2.List {
		itemData := item.(*syn.Data)
		dataList = append(dataList, itemData.Inner)
	}

	value := &Constant{&syn.Data{
		Inner: &data.Constr{
			Tag:    uint(arg1.Int64()),
			Fields: dataList,
		},
	}}

	return value, nil
}

func mapData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	pairType := &syn.TPair{
		First:  &syn.TData{},
		Second: &syn.TData{},
	}

	arg1, err := unwrapList[T](pairType, b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, listExMem(arg1.List))
	if err != nil {
		return nil, err
	}

	dataList := [][2]data.PlutusData{}

	for _, item := range arg1.List {
		pair := item.(*syn.ProtoPair)
		fst := pair.First.(*syn.Data)
		snd := pair.Second.(*syn.Data)

		dataList = append(dataList, [2]data.PlutusData{fst.Inner, snd.Inner})
	}

	value := &Constant{&syn.Data{
		Inner: &data.Map{
			Pairs: dataList,
		},
	}}

	return value, nil
}

func listData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapList[T](&syn.TData{}, b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, listExMem(arg1.List))
	if err != nil {
		return nil, err
	}

	dataList := []data.PlutusData{}

	for _, item := range arg1.List {
		itemData := item.(*syn.Data)
		dataList = append(dataList, itemData.Inner)
	}

	value := &Constant{&syn.Data{
		Inner: &data.List{
			Items: dataList,
		},
	}}

	return value, nil
}

func iData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapInteger[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, bigIntExMem(arg1))
	if err != nil {
		return nil, err
	}

	value := &Constant{&syn.Data{
		Inner: &data.Integer{Inner: arg1},
	}}

	return value, nil
}

func bData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, byteArrayExMem(arg1))
	if err != nil {
		return nil, err
	}

	value := &Constant{&syn.Data{
		Inner: &data.ByteString{Inner: arg1},
	}}

	return value, nil
}

func unConstrData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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
			FstType: &syn.TInteger{},
			SndType: &syn.TList{
				Typ: &syn.TData{},
			},
			First: &syn.Integer{
				Inner: big.NewInt(int64(constr.Tag)),
			},
			Second: &syn.ProtoList{
				LTyp: &syn.TData{},
				List: fields,
			},
		}
	default:
		return nil, errors.New("data is not a constr")
	}

	value := &Constant{pair}

	return value, nil
}

func unMapData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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
				FstType: &syn.TData{},
				SndType: &syn.TData{},
				First: &syn.Data{
					Inner: item[0],
				},
				Second: &syn.Data{
					Inner: item[1],
				},
			}

			items = append(items, pair)
		}

		dataMap = &syn.ProtoList{
			LTyp: &syn.TPair{
				First:  &syn.TData{},
				Second: &syn.TData{},
			},
			List: items,
		}
	default:
		return nil, errors.New("data is not a map")
	}

	value := &Constant{dataMap}

	return value, nil
}

func unListData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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
			LTyp: &syn.TData{},
			List: items,
		}
	default:
		return nil, errors.New("data is not a list")
	}

	value := &Constant{list}

	return value, nil
}

func unIData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{integer}

	return value, nil
}

func unBData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{bytes}

	return value, nil
}

func equalsData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{result}

	return value, nil
}

func serialiseData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	panic("implement SerialiseData")
}

func mkPairData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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
		FstType: &syn.TData{},
		SndType: &syn.TData{},
		First: &syn.Data{
			Inner: arg1,
		},
		Second: &syn.Data{
			Inner: arg2,
		},
	}

	value := &Constant{&pair}

	return value, nil
}

func mkNilData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	err := unwrapUnit[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, unitExMem())
	if err != nil {
		return nil, err
	}

	l := syn.ProtoList{
		LTyp: &syn.TData{},
		List: []syn.IConstant{},
	}

	value := &Constant{&l}

	return value, nil
}

func mkNilPairData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	err := unwrapUnit[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, unitExMem())
	if err != nil {
		return nil, err
	}

	l := syn.ProtoList{
		LTyp: &syn.TPair{
			First:  &syn.TData{},
			Second: &syn.TData{},
		},
		List: []syn.IConstant{},
	}

	value := &Constant{&l}

	return value, nil
}

func bls12381G1Add[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	newG1 := new(bls.G1Jac).Set(arg1)

	newG1.AddAssign(arg2)

	value := &Constant{&syn.Bls12_381G1Element{
		Inner: newG1,
	}}

	return value, nil
}

func bls12381G1Neg[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapBls12_381G1Element[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, blsG1ExMem())
	if err != nil {
		return nil, err
	}

	g1Neg := new(bls.G1Jac).Neg(arg1)

	value := &Constant{&syn.Bls12_381G1Element{
		Inner: g1Neg,
	}}

	return value, nil
}

func bls12381G1ScalarMul[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	newG1 := new(bls.G1Jac).ScalarMultiplication(arg2, arg1)

	value := &Constant{&syn.Bls12_381G1Element{Inner: newG1}}

	return value, nil
}

func bls12381G1Equal[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Bool{
		Inner: arg1.Equal(arg2),
	}}

	return value, nil
}

func bls12381G1Compress[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapBls12_381G1Element[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, blsG1ExMem())
	if err != nil {
		return nil, err
	}

	compressed := new(bls.G1Affine).FromJacobian(arg1).Bytes()

	value := &Constant{&syn.ByteString{
		Inner: compressed[:],
	}}

	return value, nil
}

func bls12381G1Uncompress[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	jac := new(bls.G1Jac).FromAffine(uncompressed)

	value := &Constant{&syn.Bls12_381G1Element{
		Inner: jac,
	}}

	return value, nil
}

func bls12381G1HashToGroup[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	// TODO probably impement our own HashToG1
	// that doesn't needlessly convert final result Jac to Affine
	// That's an extremely wasteful calculation
	point, err := bls.HashToG1(arg1, arg2)
	if err != nil {
		return nil, err
	}

	jac := new(bls.G1Jac).FromAffine(&point)

	value := &Constant{&syn.Bls12_381G1Element{
		Inner: jac,
	}}

	return value, nil
}

func bls12381G2Add[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	newG2 := new(bls.G2Jac).Set(arg1)

	newG2.AddAssign(arg2)

	value := &Constant{&syn.Bls12_381G2Element{
		Inner: newG2,
	}}

	return value, nil
}

func bls12381G2Neg[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapBls12_381G2Element[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, blsG2ExMem())
	if err != nil {
		return nil, err
	}

	g1Neg := new(bls.G2Jac).Neg(arg1)

	value := &Constant{&syn.Bls12_381G2Element{
		Inner: g1Neg,
	}}

	return value, nil
}

func bls12381G2ScalarMul[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	newG2 := new(bls.G2Jac).ScalarMultiplication(arg2, arg1)

	value := &Constant{&syn.Bls12_381G2Element{
		Inner: newG2,
	}}

	return value, nil
}

func bls12381G2Equal[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Bool{
		Inner: arg1.Equal(arg2),
	}}

	return value, nil
}

func bls12381G2Compress[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	arg1, err := unwrapBls12_381G2Element[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, blsG2ExMem())
	if err != nil {
		return nil, err
	}

	bt := new(bls.G2Affine).FromJacobian(arg1).Bytes()

	value := &Constant{&syn.ByteString{
		Inner: bt[:],
	}}

	return value, nil
}

func bls12381G2Uncompress[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	jac := new(bls.G2Jac).FromAffine(uncompressed)

	value := &Constant{&syn.Bls12_381G2Element{
		Inner: jac,
	}}

	return value, nil
}

func bls12381G2HashToGroup[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	// TODO probably impement our own HashToG2
	// that doesn't needlessly convert final result Jac to Affine
	// That's an extremely wasteful calculation
	point, err := bls.HashToG2(arg1, arg2)
	if err != nil {
		return nil, err
	}

	jac := new(bls.G2Jac).FromAffine(&point)

	value := &Constant{&syn.Bls12_381G2Element{
		Inner: jac,
	}}

	return value, nil
}

func bls12381MillerLoop[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	arg1Affine := new(bls.G1Affine).FromJacobian(arg1)

	arg2Affine := new(bls.G2Affine).FromJacobian(arg2)

	mlResult, err := bls.MillerLoop([]bls.G1Affine{*arg1Affine}, []bls.G2Affine{*arg2Affine})
	if err != nil {
		return nil, err
	}

	value := &Constant{&syn.Bls12_381MlResult{
		Inner: &mlResult,
	}}

	return value, nil
}

func bls12381MulMlResult[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	value := &Constant{&syn.Bls12_381MlResult{
		Inner: newMl,
	}}

	return value, nil
}

func bls12381FinalVerify[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
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

	// What the C code does
	// int blst_fp12_finalverify(const vec384fp12 GT1, const vec384fp12 GT2)
	// {
	//     vec384fp12 GT;

	//     vec_copy(GT, GT1, sizeof(GT));
	//     conjugate_fp12(GT);
	//     mul_fp12(GT, GT, GT2);
	//     final_exp(GT, GT);

	//     /* return GT==1 */
	//     return (int)(vec_is_equal(GT[0][0], BLS12_381_Rx.p2, sizeof(GT[0][0])) &
	//                  vec_is_zero(GT[0][1], sizeof(GT) - sizeof(GT[0][0])));
	// }

	var one bls.GT
	one.SetOne()

	arg1Conj := new(bls.GT).Conjugate(arg1)

	// Note FinalExponentiation automatically multiplies all extra args
	verify := bls.FinalExponentiation(arg1Conj, arg2)

	value := &Constant{&syn.Bool{
		Inner: one.Equal(&verify),
	}}

	return value, nil
}

func integerToByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	endianness, err := unwrapBool[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	size, err := unwrapInteger[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	input, err := unwrapInteger[T](b.Args[2])
	if err != nil {
		return nil, err
	}

	// Check for negative input
	if input.Sign() < 0 {
		return nil, fmt.Errorf("integerToByteString: negative input %v", input)
	}

	// Convert size to int64
	if !size.IsInt64() {
		return nil, errors.New("integerToByteString: size too large")
	}

	sizeInt64 := size.Int64()

	if sizeInt64 < 0 {
		return nil, fmt.Errorf("integerToByteString: negative size %v", sizeInt64)
	}

	sizeUnwrapped := int(sizeInt64)

	if sizeUnwrapped > IntegerToByteStringMaximumOutputLength {
		return nil, errors.New("integerToByteString: size too large")
	}

	// Apply cost
	err = m.CostThree(&b.Func, boolExMem(endianness), sizeExMem(sizeUnwrapped), bigIntExMem(input))
	if err != nil {
		return nil, err
	}

	// NOTE:
	// We ought to also check for negative size and too large sizes. These checks
	// however happens prior to calling the builtin as part of the costing step. So by
	// the time we reach this builtin call, the size can be assumed to be
	//
	// >= 0 && < INTEGER_TO_BYTE_STRING_MAXIMUM_OUTPUT_LENGTH

	// Check for zero size with large input
	if sizeInt64 == 0 && (input.BitLen()-1) >= 8*IntegerToByteStringMaximumOutputLength {
		required := (input.BitLen() + 7) / 8

		return nil, fmt.Errorf(
			"integerToByteString: input requires %d bytes, exceeds max %d",
			required, IntegerToByteStringMaximumOutputLength,
		)
	}

	// Handle zero input
	if input.Sign() == 0 {
		bytes := make([]byte, sizeUnwrapped)
		value := &Constant{&syn.ByteString{
			Inner: bytes,
		}}
		return value, nil
	}

	// Convert integer to bytes (big-endian by default in Go)
	bytes := input.Bytes()

	// Check if bytes exceed specified size
	if sizeInt64 != 0 && len(bytes) > sizeUnwrapped {
		return nil, fmt.Errorf(
			"integerToByteString: size %d too small, need %d",
			sizeUnwrapped, len(bytes),
		)
	}

	// Handle padding
	if sizeUnwrapped > 0 {
		paddingSize := sizeUnwrapped - len(bytes)
		padding := make([]byte, paddingSize)

		if endianness {
			// Big-endian: padding | bytes
			bytes = append(padding, bytes...)
		} else {
			// Little-endian: bytes | padding
			// Reverse bytes for little-endian
			for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
				bytes[i], bytes[j] = bytes[j], bytes[i]
			}
			bytes = append(bytes, padding...)
		}
	} else if !endianness {
		// Little-endian with zero size: reverse bytes
		for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
			bytes[i], bytes[j] = bytes[j], bytes[i]
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: bytes,
	}}

	return value, nil
}

func byteStringToInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	endianness, err := unwrapBool[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	bytes, err := unwrapByteString[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostTwo(&b.Func, boolExMem(endianness), byteArrayExMem(bytes))
	if err != nil {
		return nil, err
	}

	// Convert bytes to integer
	newInt := new(big.Int)

	if endianness {
		// Big-endian: use bytes directly
		newInt.SetBytes(bytes)
	} else {
		// Little-endian: reverse bytes before conversion
		reversed := make([]byte, len(bytes))

		for i, j := 0, len(bytes)-1; i < len(bytes); i, j = i+1, j-1 {
			reversed[i] = bytes[j]
		}

		newInt.SetBytes(reversed)
	}

	value := &Constant{&syn.Integer{
		Inner: newInt,
	}}

	return value, nil
}

func andByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	shouldPad, err := unwrapBool[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	bytes1, err := unwrapByteString[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	bytes2, err := unwrapByteString[T](b.Args[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(&b.Func, boolExMem(shouldPad), byteArrayExMem(bytes1), byteArrayExMem(bytes2))
	if err != nil {
		return nil, err
	}

	// Determine output length
	var result []byte
	if shouldPad {
		// Pad shorter string with 0xFF, use longer length
		maxLen := len(bytes1)
		if len(bytes2) > maxLen {
			maxLen = len(bytes2)
		}
		result = make([]byte, maxLen)

		for i := 0; i < maxLen; i++ {
			var b1, b2 byte
			if i < len(bytes1) {
				b1 = bytes1[i]
			} else {
				b1 = 0xFF
			}

			if i < len(bytes2) {
				b2 = bytes2[i]
			} else {
				b2 = 0xFF
			}

			result[i] = b1 & b2
		}
	} else {
		// Use shorter length, no padding
		minLen := len(bytes1)
		if len(bytes2) < minLen {
			minLen = len(bytes2)
		}

		result = make([]byte, minLen)

		for i := 0; i < minLen; i++ {
			result[i] = bytes1[i] & bytes2[i]
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func orByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	shouldPad, err := unwrapBool[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	bytes1, err := unwrapByteString[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	bytes2, err := unwrapByteString[T](b.Args[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(&b.Func, boolExMem(shouldPad), byteArrayExMem(bytes1), byteArrayExMem(bytes2))
	if err != nil {
		return nil, err
	}

	// Determine output length
	var result []byte
	if shouldPad {
		// Pad shorter string with 0x00, use longer length
		maxLen := len(bytes1)
		if len(bytes2) > maxLen {
			maxLen = len(bytes2)
		}
		result = make([]byte, maxLen)

		for i := 0; i < maxLen; i++ {
			var b1, b2 byte
			if i < len(bytes1) {
				b1 = bytes1[i]
			} else {
				b1 = 0x00
			}
			if i < len(bytes2) {
				b2 = bytes2[i]
			} else {
				b2 = 0x00
			}
			result[i] = b1 | b2
		}
	} else {
		// Use shorter length, no padding
		minLen := len(bytes1)
		if len(bytes2) < minLen {
			minLen = len(bytes2)
		}
		result = make([]byte, minLen)

		for i := 0; i < minLen; i++ {
			result[i] = bytes1[i] | bytes2[i]
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func xorByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	shouldPad, err := unwrapBool[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	bytes1, err := unwrapByteString[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	bytes2, err := unwrapByteString[T](b.Args[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(&b.Func, boolExMem(shouldPad), byteArrayExMem(bytes1), byteArrayExMem(bytes2))
	if err != nil {
		return nil, err
	}

	// Determine output length
	var result []byte
	if shouldPad {
		// Pad shorter string with 0x00, use longer length
		maxLen := len(bytes1)
		if len(bytes2) > maxLen {
			maxLen = len(bytes2)
		}

		result = make([]byte, maxLen)

		for i := 0; i < maxLen; i++ {
			var b1, b2 byte
			if i < len(bytes1) {
				b1 = bytes1[i]
			} else {
				b1 = 0x00
			}

			if i < len(bytes2) {
				b2 = bytes2[i]
			} else {
				b2 = 0x00
			}

			result[i] = b1 ^ b2
		}
	} else {
		// Use shorter length, no padding
		minLen := len(bytes1)
		if len(bytes2) < minLen {
			minLen = len(bytes2)
		}

		result = make([]byte, minLen)

		for i := 0; i < minLen; i++ {
			result[i] = bytes1[i] ^ bytes2[i]
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func complementByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap argument
	bytes, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostOne(&b.Func, byteArrayExMem(bytes))
	if err != nil {
		return nil, err
	}

	// Compute complement
	result := make([]byte, len(bytes))
	for i, b := range bytes {
		result[i] = b ^ 0xFF
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func readBit[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	bytes, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	bitIndex, err := unwrapInteger[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostTwo(&b.Func, byteArrayExMem(bytes), bigIntExMem(bitIndex))
	if err != nil {
		return nil, err
	}

	// Check for empty byte string
	if len(bytes) == 0 {
		return nil, errors.New("readBit: empty byte array")
	}

	// Validate bit index
	if bitIndex.Sign() < 0 || bitIndex.Cmp(big.NewInt(int64(len(bytes)*8))) >= 0 {
		return nil, errors.New("readBit: bit index out of bounds")
	}

	// Convert bit index to int64
	bitIndexInt, ok := bitIndex.Int64(), bitIndex.IsInt64()
	if !ok {
		return nil, errors.New("readBit: bit index too large")
	}

	// Compute byte index and bit offset
	byteIndex := bitIndexInt / 8
	bitOffset := bitIndexInt % 8

	// Flip byte index (little-endian interpretation)
	flippedIndex := len(bytes) - 1 - int(byteIndex)

	// Extract bit
	byteVal := bytes[flippedIndex]
	bitTest := (byteVal>>bitOffset)&1 == 1

	value := &Constant{&syn.Bool{
		Inner: bitTest,
	}}

	return value, nil
}

func writeBits[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	bytes, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	indices, err := unwrapList[T](&syn.TInteger{}, b.Args[1])
	if err != nil {
		return nil, err
	}

	setBit, err := unwrapBool[T](b.Args[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(&b.Func, byteArrayExMem(bytes), listLengthExMem(indices.List), boolExMem(setBit))
	if err != nil {
		return nil, err
	}

	// Clone bytes to avoid modifying the input
	result := make([]byte, len(bytes))
	copy(result, bytes)

	// Process each bit index
	for _, index := range indices.List {
		// Unwrap integer from list element
		bitIndex, ok := index.(*syn.Integer)
		if !ok {
			return nil, fmt.Errorf("writeBits: expected integer in indices list, got %T", index)
		}

		// Validate bit index
		if bitIndex.Inner.Sign() < 0 || bitIndex.Inner.Cmp(big.NewInt(int64(len(bytes)*8))) >= 0 {
			return nil, errors.New("writeBits: bit index out of bounds")
		}

		// Convert bit index to int64
		bitIndexInt, ok := bitIndex.Inner.Int64(), bitIndex.Inner.IsInt64()
		if !ok {
			return nil, errors.New("writeBits: bit index too large")
		}

		// Compute byte index and bit offset
		byteIndex := bitIndexInt / 8
		bitOffset := bitIndexInt % 8

		// Flip byte index (little-endian interpretation)
		flippedIndex := len(bytes) - 1 - int(byteIndex)

		// Create bit mask
		bitMask := byte(1) << bitOffset

		// Set or clear the bit
		if setBit {
			result[flippedIndex] |= bitMask
		} else {
			result[flippedIndex] &= ^bitMask
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func replicateByte[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	size, err := unwrapInteger[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	byteVal, err := unwrapInteger[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	// Validate size
	if size.Sign() < 0 {
		return nil, errors.New("replicateByte: negative size")
	}
	if !size.IsInt64() {
		return nil, errors.New("replicateByte: size too large")
	}
	sizeInt := int(size.Int64())

	if sizeInt > IntegerToByteStringMaximumOutputLength {
		return nil, errors.New("replicateByte: size too large")
	}

	// Validate byte
	if !byteVal.IsInt64() || byteVal.Int64() < 0 || byteVal.Int64() > 255 {
		return nil, fmt.Errorf("replicateByte: byte value %v out of bounds (0-255)", byteVal)
	}
	byteUint := byte(byteVal.Int64())

	// Apply cost
	err = m.CostTwo(&b.Func, sizeExMem(sizeInt), bigIntExMem(byteVal))
	if err != nil {
		return nil, err
	}

	// Create result
	var result []byte
	if sizeInt == 0 {
		result = []byte{}
	} else {
		result = make([]byte, sizeInt)
		for i := 0; i < sizeInt; i++ {
			result[i] = byteUint
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func shiftByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	bytes, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	shift, err := unwrapInteger[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostTwo(&b.Func, byteArrayExMem(bytes), bigIntExMem(shift))
	if err != nil {
		return nil, err
	}

	// Check if shift exceeds total bits
	totalBits := big.NewInt(int64(len(bytes) * 8))
	if shift.Abs(shift).Cmp(totalBits) >= 0 {
		result := make([]byte, len(bytes))
		return &Constant{&syn.ByteString{Inner: result}}, nil
	}

	// Convert shift to int64
	if !shift.IsInt64() {
		return nil, errors.New("shiftByteString: shift value too large")
	}
	shiftVal := shift.Int64()
	absShift := shiftVal
	if shiftVal < 0 {
		absShift = -shiftVal
	}

	// Clone input bytes
	result := make([]byte, len(bytes))
	copy(result, bytes)

	// Perform shift
	isLeftShift := shiftVal >= 0
	absShiftInt := int(absShift)
	byteShift := absShiftInt / 8
	bitShift := absShiftInt % 8

	if isLeftShift {
		// Left shift
		if byteShift > 0 {
			// Shift whole bytes
			for i := 0; i < len(result)-byteShift; i++ {
				result[i] = result[i+byteShift]
			}
			for i := len(result) - byteShift; i < len(result); i++ {
				result[i] = 0
			}
		}
		if bitShift > 0 {
			// Shift remaining bits
			carry := uint8(0)
			for i := 0; i < len(result); i++ {
				newCarry := (result[i] >> (8 - bitShift)) << (8 - bitShift)
				result[i] = (result[i] << bitShift) | (carry >> (8 - bitShift))
				carry = newCarry
			}
		}
	} else {
		// Right shift
		if byteShift > 0 {
			// Shift whole bytes
			for i := len(result) - 1; i >= byteShift; i-- {
				result[i] = result[i-byteShift]
			}
			for i := 0; i < byteShift; i++ {
				result[i] = 0
			}
		}
		if bitShift > 0 {
			// Shift remaining bits
			carry := uint8(0)
			for i := len(result) - 1; i >= 0; i-- {
				newCarry := (result[i] << (8 - bitShift)) >> (8 - bitShift)
				result[i] = (result[i] >> bitShift) | (carry << (8 - bitShift))
				carry = newCarry
			}
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func rotateByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap arguments
	bytes, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	shift, err := unwrapInteger[T](b.Args[1])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostTwo(&b.Func, byteArrayExMem(bytes), bigIntExMem(shift))
	if err != nil {
		return nil, err
	}

	// Handle empty byte string
	if len(bytes) == 0 {
		return &Constant{&syn.ByteString{Inner: []byte{}}}, nil
	}

	// Normalize shift value
	totalBits := big.NewInt(int64(len(bytes) * 8))
	normalizedShift := new(big.Int).Mod(shift, totalBits)
	if normalizedShift.Sign() < 0 {
		normalizedShift.Add(normalizedShift, totalBits)
	}

	// Convert normalized shift to int64
	if !normalizedShift.IsInt64() {
		return nil, errors.New("rotateByteString: normalized shift too large")
	}
	shiftVal := int(normalizedShift.Int64())

	// If shift is 0, return original bytes
	if shiftVal == 0 {
		result := make([]byte, len(bytes))
		copy(result, bytes)
		return &Constant{&syn.ByteString{Inner: result}}, nil
	}

	// Compute byte and bit shifts
	byteShift := shiftVal / 8
	bitShift := shiftVal % 8

	// Perform rotation
	result := make([]byte, len(bytes))
	for i := 0; i < len(bytes); i++ {
		// Source byte index (rotated left)
		srcIndex := (i + byteShift) % len(bytes)
		if srcIndex < 0 {
			srcIndex += len(bytes)
		}

		// Get current and next byte for bit rotation
		currByte := bytes[srcIndex]
		nextIndex := (srcIndex + 1) % len(bytes)
		nextByte := bytes[nextIndex]

		// Rotate bits
		result[i] = (currByte << bitShift) | (nextByte >> (8 - bitShift))
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func countSetBits[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap argument
	bytes, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostOne(&b.Func, byteArrayExMem(bytes))
	if err != nil {
		return nil, err
	}

	// Count set bits
	count := 0
	for _, b := range bytes {
		count += bits.OnesCount8(b)
	}

	value := &Constant{&syn.Integer{
		Inner: big.NewInt(int64(count)),
	}}

	return value, nil
}

func findFirstSetBit[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap argument
	bytes, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostOne(&b.Func, byteArrayExMem(bytes))
	if err != nil {
		return nil, err
	}

	// Find first set bit in little-endian order
	for byteIndex := len(bytes) - 1; byteIndex >= 0; byteIndex-- {
		value := bytes[byteIndex]
		if value == 0 {
			continue
		}

		// Check bits from LSB to MSB
		for bit := 0; bit < 8; bit++ {
			if (value & (1 << bit)) != 0 {
				// Bit index: bit position + byte offset
				bitIndex := bit + byteIndex*8

				return &Constant{&syn.Integer{
					Inner: big.NewInt(int64(bitIndex)),
				}}, nil
			}
		}
	}

	// No bits set
	return &Constant{&syn.Integer{
		Inner: big.NewInt(-1),
	}}, nil
}

func ripemd160[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	// Unwrap argument
	arg1, err := unwrapByteString[T](b.Args[0])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostOne(&b.Func, byteArrayExMem(arg1))
	if err != nil {
		return nil, err
	}

	// Compute RIPEMD-160 hash
	hasher := legacyripemd160.New()
	hasher.Write(arg1)
	bytes := hasher.Sum(nil)

	value := &Constant{&syn.ByteString{
		Inner: bytes,
	}}

	return value, nil
}
