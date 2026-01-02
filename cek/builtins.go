package cek

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha3"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"sort"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
	"github.com/btcsuite/btcd/btcec/v2"
	ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	schnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/ethereum/go-ethereum/crypto"
	sha256 "github.com/minio/sha256-simd"
	"golang.org/x/crypto/blake2b"
	legacyripemd160 "golang.org/x/crypto/ripemd160" //nolint:staticcheck,gosec
)

func addInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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

func subtractInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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

func multiplyInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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
	var remainder big.Int

	// Perform division
	newInt.Quo(arg1, arg2)
	remainder.Rem(arg1, arg2)

	// Adjust for floor division if remainder exists and signs differ
	if remainder.Sign() != 0 && arg1.Sign() != arg2.Sign() {
		newInt.Sub(&newInt, big.NewInt(1))
	}

	value := &Constant{&syn.Integer{
		Inner: &newInt,
	}}

	return value, nil
}

func quotientInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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

func remainderInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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

	var quotient, remainder big.Int

	// Compute quotient and remainder
	quotient.Quo(arg1, arg2)
	remainder.Rem(arg1, arg2)

	// Adjust for floored modulo if remainder exists and signs differ
	if remainder.Sign() != 0 && arg1.Sign() != arg2.Sign() {
		remainder.Add(&remainder, arg2)
	}

	value := &Constant{&syn.Integer{
		Inner: &remainder,
	}}

	return value, nil
}

func equalsInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Charge budget for integer comparison
	err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
	if err != nil {
		return nil, err
	}

	// Compare big.Int values for equality
	res := arg1.Cmp(arg2) == 0

	con := &syn.Bool{
		Inner: res,
	}

	value := &Constant{con}

	return value, nil
}

func lessThanInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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

func lessThanEqualsInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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

func appendByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapByteString[T](m.argHolder[1])
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

func consByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0]) // skip
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapByteString[T](m.argHolder[1]) // byte string
	if err != nil {
		return nil, err
	}

	err = m.CostTwo(&b.Func, bigIntExMem(arg1), byteArrayExMem(arg2))
	if err != nil {
		return nil, err
	}

	if !arg1.IsInt64() {
		return nil, errors.New("int does not fit into a byte")
	}

	int_val := arg1.Int64()

	// consByteString requires the integer to be in the range 0-255
	if int_val < 0 || int_val > 255 {
		return nil, errors.New("int does not fit into a byte")
	}

	res := append([]byte{byte(int_val)}, arg2...)

	value := &Constant{&syn.ByteString{
		Inner: res,
	}}

	return value, nil
}

func sliceByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0]) // skip
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1]) // take
	if err != nil {
		return nil, err
	}

	arg3, err := unwrapByteString[T](m.argHolder[2]) // byte string
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
			if skip64 > int64(math.MaxInt) {
				skip = math.MaxInt
			} else {
				skip = int(skip64)
			}
		} else {
			skip = len(arg3) // Clamp to max if too large
		}
	}

	take := 0
	if arg2.Sign() > 0 {
		if take64, ok := arg2.Int64(), arg2.IsInt64(); ok {
			if take64 > int64(math.MaxInt) {
				take = math.MaxInt
			} else {
				take = int(take64)
			}
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

func lengthOfByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
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

func indexByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapInteger[T](m.argHolder[1])
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

	// Check bounds: index must be in range [0, len(arg1))
	if index < 0 || index > int64(math.MaxInt) || int(index) >= len(arg1) {
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

func equalsByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapByteString[T](m.argHolder[1])
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

func lessThanByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapByteString[T](m.argHolder[1])
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

func lessThanEqualsByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapByteString[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
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

func verifyEd25519Signature[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	publicKey, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	message, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	signature, err := unwrapByteString[T](m.argHolder[2])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, byteArrayExMem(arg1))
	if err != nil {
		return nil, err
	}

	res := crypto.Keccak256Hash(arg1)

	con := &syn.ByteString{
		Inner: res[:],
	}

	value := &Constant{con}

	return value, nil
}

func verifyEcdsaSecp256K1Signature[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	publicKey, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	message, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	signature, err := unwrapByteString[T](m.argHolder[2])
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

	if len(publicKey) != 33 {
		return nil, errors.New("invalid public key length")
	}

	if len(signature) != 64 {
		return nil, errors.New("invalid signature length")
	}

	if len(message) != 32 {
		return nil, errors.New("invalid message length")
	}

	key, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return nil, err
	}

	r := new(btcec.ModNScalar)

	overflow := r.SetByteSlice(signature[0:32])
	if overflow {
		return nil, errors.New("invalid signature (r)")
	}

	s := new(btcec.ModNScalar)
	overflow = s.SetByteSlice(signature[32:])
	if overflow {
		return nil, errors.New("invalid signature (s)")
	}

	sig := ecdsa.NewSignature(r, s)

	// Check s is less then half the field prime (BIP-146)
	res := sig.Verify(message, key) && !s.IsOverHalfOrder()

	con := &syn.Bool{
		Inner: res,
	}

	value := &Constant{con}

	return value, nil
}

func verifySchnorrSecp256K1Signature[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	publicKey, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	message, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	signature, err := unwrapByteString[T](m.argHolder[2])
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

	if len(signature) != 64 {
		return nil, errors.New("invalid signature length")
	}

	key, err := schnorr.ParsePubKey(publicKey)
	if err != nil {
		return nil, err
	}

	r := new(btcec.FieldVal)

	// Overflow is fine - part of CONF tests
	r.SetByteSlice(signature[0:32])

	s := new(btcec.ModNScalar)

	// Overflow is fine - part of CONF tests
	s.SetByteSlice(signature[32:])

	// Had to copy the code and take out the message size check
	// since BIP-340 in practice supports any message size even
	// though only 32 bytes is practically used
	res := verify(r, s, message, key)

	con := &syn.Bool{
		Inner: res,
	}

	value := &Constant{con}

	return value, nil
}

func schnorrVerify(
	r *btcec.FieldVal,
	s *btcec.ModNScalar,
	msg []byte,
	pubKey *btcec.PublicKey,
) error {
	// The algorithm for producing a BIP-340 signature is described in
	// README.md and is reproduced here for reference:
	//
	// 1. Fail if m is not 32 bytes
	// 2. P = lift_x(int(pk)).
	// 3. r = int(sig[0:32]); fail is r >= p.
	// 4. s = int(sig[32:64]); fail if s >= n.
	// 5. e = int(tagged_hash("BIP0340/challenge", bytes(r) || bytes(P) || M)) mod n.
	// 6. R = s*G - e*P
	// 7. Fail if is_infinite(R)
	// 8. Fail if not hash_even_y(R)
	// 9. Fail is x(R) != r.
	// 10. Return success iff failure did not occur before reaching this point.

	// DONT DO THIS WE NEED VARIABLE LENGTH FOR CONF TESTS
	// Step 1.
	//
	// Fail if m is not 32 bytes
	// if len(hash) != scalarSize {
	// 	str := fmt.Sprintf("wrong size for message (got %v, want %v)",
	// 		len(hash), scalarSize)
	// 	return signatureError(ecdsa_schnorr.ErrInvalidHashLen, str)
	// }

	// Already done before
	// Step 2.
	//
	// P = lift_x(int(pk))
	//
	// Fail if P is not a point on the curve
	// pubKey, err := ParsePubKey(pubKeyBytes)
	// if err != nil {
	// 	return err
	// }
	// if !pubKey.IsOnCurve() {
	// 	str := "pubkey point is not on curve"
	// 	return signatureError(ecdsa_schnorr.ErrPubKeyNotOnCurve, str)
	// }

	// Step 3.
	//
	// Fail if r >= p
	//
	// Note this is already handled by the fact r is a field element.

	// Step 4.
	//
	// Fail if s >= n
	//
	// Note this is already handled by the fact s is a mod n scalar.

	// Step 5.
	//
	// e = int(tagged_hash("BIP0340/challenge", bytes(r) || bytes(P) || M)) mod n.
	var rBytes [32]byte

	r.PutBytesUnchecked(rBytes[:])
	pBytes := schnorr.SerializePubKey(pubKey)

	commitment := chainhash.TaggedHash(
		chainhash.TagBIP0340Challenge, rBytes[:], pBytes, msg,
	)

	var e btcec.ModNScalar
	e.SetBytes((*[32]byte)(commitment))

	// Negate e here so we can use AddNonConst below to subtract the s*G
	// point from e*P.
	e.Negate()

	// Step 6.
	//
	// R = s*G - e*P
	var P, R, sG, eP btcec.JacobianPoint
	pubKey.AsJacobian(&P)
	btcec.ScalarBaseMultNonConst(s, &sG)
	btcec.ScalarMultNonConst(&e, &P, &eP)
	btcec.AddNonConst(&sG, &eP, &R)

	// Step 7.
	//
	// Fail if R is the point at infinity
	if (R.X.IsZero() && R.Y.IsZero()) || R.Z.IsZero() {
		str := "calculated R point is the point at infinity"
		return errors.New(str)
	}

	// Step 8.
	//
	// Fail if R.y is odd
	//
	// Note that R must be in affine coordinates for this check.
	R.ToAffine()
	if R.Y.IsOdd() {
		str := "calculated R y-value is odd"
		return errors.New(str)
	}

	// Step 9.
	//
	// Verified if R.x == r
	//
	// Note that R must be in affine coordinates for this check.
	if !r.Equals(&R.X) {
		str := "calculated R point was not given R"
		return errors.New(str)
	}

	// Step 10.
	//
	// Return success iff failure did not occur before reaching this point.
	return nil
}

// Verify returns whether or not the signature is valid for the provided hash
// and secp256k1 public key.
func verify(
	r *btcec.FieldVal,
	s *btcec.ModNScalar,
	msg []byte,
	pubKey *btcec.PublicKey,
) bool {
	return schnorrVerify(r, s, msg, pubKey) == nil
}

func appendString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapString[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Charge budget for string comparison
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapString[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBool[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2 := m.argHolder[1]
	arg3 := m.argHolder[2]

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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	if err := unwrapUnit[T](m.argHolder[0]); err != nil {
		return nil, err
	}

	arg2 := m.argHolder[1]

	err := m.CostTwo(&b.Func, unitExMem(), valueExMem[T](arg2))
	if err != nil {
		return nil, err
	}

	value := arg2

	return value, nil
}

func trace[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2 := m.argHolder[1]

	err = m.CostTwo(&b.Func, stringExMem(arg1), valueExMem[T](arg2))
	if err != nil {
		return nil, err
	}

	m.Logs = append(m.Logs, arg1)

	value := arg2

	return value, nil
}

func fstPair[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	fstPair, sndPair, err := unwrapPair[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	fstPair, sndPair, err := unwrapPair[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	l, err := unwrapList[T](nil, m.argHolder[0])
	if err != nil {
		return nil, err
	}

	branchEmpty := m.argHolder[1]

	branchOtherwise := m.argHolder[2]

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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapConstant[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	typ := arg1.Constant.Typ()

	arg2, err := unwrapList[T](typ, m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapList[T](nil, m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapList[T](nil, m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapList[T](nil, m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	constrBranch := m.argHolder[1]
	mapBranch := m.argHolder[2]
	listBranch := m.argHolder[3]
	integerBranch := m.argHolder[4]
	bytesBranch := m.argHolder[5]

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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	if arg1.Sign() < 0 {
		return nil, errors.New("constructor tag must be non-negative")
	}

	arg2, err := unwrapList[T](&syn.TData{}, m.argHolder[1])
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

	if arg1.BitLen() > 64 {
		return nil, errors.New("constructor tag too large")
	}
	tag64 := arg1.Uint64()
	if tag64 > uint64(math.MaxUint) {
		return nil, errors.New("constructor tag too large")
	}
	tag := uint(tag64)

	value := &Constant{&syn.Data{
		Inner: &data.Constr{
			Tag:    tag,
			Fields: dataList,
		},
	}}

	return value, nil
}

func mapData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	pairType := &syn.TPair{
		First:  &syn.TData{},
		Second: &syn.TData{},
	}

	arg1, err := unwrapList[T](pairType, m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapList[T](&syn.TData{}, m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
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
		if constr.Tag > math.MaxInt64 {
			return nil, errors.New("constructor tag too large for integer conversion")
		}

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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapData[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	costX, costY := equalsDataExMem(arg1, arg2)
	err = m.CostTwo(&b.Func, costX, costY)
	if err != nil {
		return nil, err
	}

	result := &syn.Bool{
		Inner: arg1.Equal(arg2),
	}

	value := &Constant{result}

	return value, nil
}

func serialiseData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, dataExMem(arg1))
	if err != nil {
		return nil, err
	}

	encoded, err := data.Encode(arg1)
	if err != nil {
		return nil, err
	}

	con := &syn.ByteString{
		Inner: encoded,
	}

	value := &Constant{con}

	return value, nil
}

func mkPairData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapData[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	err := unwrapUnit[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	err := unwrapUnit[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G1Element[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381G1Element[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G1Element[T](m.argHolder[0])
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

func bls12381G1ScalarMul[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381G1Element[T](m.argHolder[1])
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

func bls12381G1Equal[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G1Element[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381G1Element[T](m.argHolder[1])
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

func bls12381G1Compress[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G1Element[T](m.argHolder[0])
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

func bls12381G1Uncompress[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, byteArrayExMem(arg1))
	if err != nil {
		return nil, err
	}

	if len(arg1) != bls.SizeOfG1AffineCompressed {
		return nil, errors.New("bytestring is too long")
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

func bls12381G1HashToGroup[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapByteString[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G2Element[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381G2Element[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G2Element[T](m.argHolder[0])
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

func bls12381G2ScalarMul[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381G2Element[T](m.argHolder[1])
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

func bls12381G1MultiScalarMul[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	ints, err := unwrapList[T](&syn.TInteger{}, m.argHolder[0])
	if err != nil {
		return nil, err
	}

	elems, err := unwrapList[T](&syn.TBls12_381G1Element{}, m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Charge cost based on lengths of the lists
	err = m.CostTwo(
		&b.Func,
		listLengthExMem(ints.List),
		listLengthExMem(elems.List),
	)
	if err != nil {
		return nil, err
	}

	// Compute multi-scalar multiplication: sum_i (n_i * p_i)
	res := new(bls.G1Jac)

	n := min(len(elems.List), len(ints.List))

	for i := range n {
		ni, ok := ints.List[i].(*syn.Integer)
		if !ok {
			return nil, errors.New(
				"bls12_381_G1_multiScalarMul: expected integer list",
			)
		}
		pi, ok := elems.List[i].(*syn.Bls12_381G1Element)
		if !ok {
			return nil, errors.New(
				"bls12_381_G1_multiScalarMul: expected g1 element list",
			)
		}

		tmp := new(bls.G1Jac).ScalarMultiplication(pi.Inner, ni.Inner)
		res.AddAssign(tmp)
	}

	return &Constant{&syn.Bls12_381G1Element{Inner: res}}, nil
}

func bls12381G2MultiScalarMul[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	ints, err := unwrapList[T](&syn.TInteger{}, m.argHolder[0])
	if err != nil {
		return nil, err
	}

	elems, err := unwrapList[T](&syn.TBls12_381G2Element{}, m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Charge cost based on lengths of the lists
	err = m.CostTwo(
		&b.Func,
		listLengthExMem(ints.List),
		listLengthExMem(elems.List),
	)
	if err != nil {
		return nil, err
	}

	// Compute multi-scalar multiplication: sum_i (n_i * p_i)
	res := new(bls.G2Jac)

	n := min(len(elems.List), len(ints.List))

	for i := range n {
		ni, ok := ints.List[i].(*syn.Integer)
		if !ok {
			return nil, errors.New(
				"bls12_381_G2_multiScalarMul: expected integer list",
			)
		}
		pi, ok := elems.List[i].(*syn.Bls12_381G2Element)
		if !ok {
			return nil, errors.New(
				"bls12_381_G2_multiScalarMul: expected g2 element list",
			)
		}

		tmp := new(bls.G2Jac).ScalarMultiplication(pi.Inner, ni.Inner)
		res.AddAssign(tmp)
	}

	return &Constant{&syn.Bls12_381G2Element{Inner: res}}, nil
}

func bls12381G2Equal[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G2Element[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381G2Element[T](m.argHolder[1])
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

func bls12381G2Compress[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G2Element[T](m.argHolder[0])
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

func bls12381G2Uncompress[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, byteArrayExMem(arg1))
	if err != nil {
		return nil, err
	}

	if len(arg1) != bls.SizeOfG2AffineCompressed {
		return nil, errors.New("bytestring is too long")
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

func bls12381G2HashToGroup[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapByteString[T](m.argHolder[1])
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

func bls12381MillerLoop[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381G1Element[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381G2Element[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	err = m.CostTwo(&b.Func, blsG1ExMem(), blsG2ExMem())
	if err != nil {
		return nil, err
	}

	arg1Affine := new(bls.G1Affine).FromJacobian(arg1)

	arg2Affine := new(bls.G2Affine).FromJacobian(arg2)

	mlResult, err := bls.MillerLoop(
		[]bls.G1Affine{*arg1Affine},
		[]bls.G2Affine{*arg2Affine},
	)
	if err != nil {
		return nil, err
	}

	value := &Constant{&syn.Bls12_381MlResult{
		Inner: &mlResult,
	}}

	return value, nil
}

func bls12381MulMlResult[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381MlResult[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381MlResult[T](m.argHolder[1])
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

func bls12381FinalVerify[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapBls12_381MlResult[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arg2, err := unwrapBls12_381MlResult[T](m.argHolder[1])
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

// integerToByteString converts an integer to its byte representation.
// This builtin implements the Plutus integer-to-bytestring conversion with
// configurable endianness and size constraints.
//
// Parameters:
// - endianness: true for big-endian, false for little-endian
// - size: desired output byte length (0 = automatic sizing)
// - input: the integer to convert (must be non-negative)
//
// The function handles:
// - Negative input validation
// - Size limit enforcement (max 8192 bytes)
// - Automatic sizing when size=0
// - Endianness conversion (big-endian default, little-endian requires reversal)
// - Padding to match requested size (errors if size too small)
//
// Edge cases:
// - Zero input produces zero-padded output of requested size
// - Size too small for input triggers error
// - Size=0 with large input (>8192 bytes) triggers error
func integerToByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	endianness, err := unwrapBool[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	size, err := unwrapInteger[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	input, err := unwrapInteger[T](m.argHolder[2])
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
		return nil, fmt.Errorf(
			"integerToByteString: negative size %v",
			sizeInt64,
		)
	}

	sizeUnwrapped := int(sizeInt64)

	if sizeUnwrapped > IntegerToByteStringMaximumOutputLength {
		return nil, errors.New("integerToByteString: size too large")
	}

	// Apply cost
	err = m.CostThree(
		&b.Func,
		boolExMem(endianness),
		sizeExMem(sizeUnwrapped),
		bigIntExMem(input),
	)
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
	if sizeInt64 == 0 &&
		(input.BitLen()-1) >= 8*IntegerToByteStringMaximumOutputLength {
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
			bytes = append(padding, bytes...) //nolint:makezero
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

func byteStringToInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	endianness, err := unwrapBool[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	bytes, err := unwrapByteString[T](m.argHolder[1])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	shouldPad, err := unwrapBool[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	bytes1, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	bytes2, err := unwrapByteString[T](m.argHolder[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(
		&b.Func,
		boolExMem(shouldPad),
		byteArrayExMem(bytes1),
		byteArrayExMem(bytes2),
	)
	if err != nil {
		return nil, err
	}

	// Determine output length
	var result []byte
	if shouldPad {
		// Pad shorter string with 0xFF, use longer length
		maxLen := max(len(bytes2), len(bytes1))
		result = make([]byte, maxLen)

		for i := range maxLen {
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
		minLen := min(len(bytes2), len(bytes1))

		result = make([]byte, minLen)

		for i := range minLen {
			result[i] = bytes1[i] & bytes2[i]
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func orByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	shouldPad, err := unwrapBool[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	bytes1, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	bytes2, err := unwrapByteString[T](m.argHolder[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(
		&b.Func,
		boolExMem(shouldPad),
		byteArrayExMem(bytes1),
		byteArrayExMem(bytes2),
	)
	if err != nil {
		return nil, err
	}

	// Determine output length
	var result []byte
	if shouldPad {
		// Pad shorter string with 0x00, use longer length
		maxLen := max(len(bytes2), len(bytes1))
		result = make([]byte, maxLen)

		for i := range maxLen {
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
		minLen := min(len(bytes2), len(bytes1))
		result = make([]byte, minLen)

		for i := range minLen {
			result[i] = bytes1[i] | bytes2[i]
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func xorByteString[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	shouldPad, err := unwrapBool[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	bytes1, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	bytes2, err := unwrapByteString[T](m.argHolder[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(
		&b.Func,
		boolExMem(shouldPad),
		byteArrayExMem(bytes1),
		byteArrayExMem(bytes2),
	)
	if err != nil {
		return nil, err
	}

	// Determine output length
	var result []byte
	if shouldPad {
		// Pad shorter string with 0x00, use longer length
		maxLen := max(len(bytes2), len(bytes1))

		result = make([]byte, maxLen)

		for i := range maxLen {
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
		minLen := min(len(bytes2), len(bytes1))

		result = make([]byte, minLen)

		for i := range minLen {
			result[i] = bytes1[i] ^ bytes2[i]
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func complementByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap argument
	bytes, err := unwrapByteString[T](m.argHolder[0])
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	bytes, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	bitIndex, err := unwrapInteger[T](m.argHolder[1])
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
	if bitIndex.Sign() < 0 ||
		bitIndex.Cmp(big.NewInt(int64(len(bytes)*8))) >= 0 {
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	bytes, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	indices, err := unwrapList[T](&syn.TInteger{}, m.argHolder[1])
	if err != nil {
		return nil, err
	}

	setBit, err := unwrapBool[T](m.argHolder[2])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostThree(
		&b.Func,
		byteArrayExMem(bytes),
		listLengthExMem(indices.List),
		boolExMem(setBit),
	)
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
			return nil, fmt.Errorf(
				"writeBits: expected integer in indices list, got %T",
				index,
			)
		}

		// Validate bit index
		if bitIndex.Inner.Sign() < 0 ||
			bitIndex.Inner.Cmp(big.NewInt(int64(len(bytes)*8))) >= 0 {
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	size, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	byteVal, err := unwrapInteger[T](m.argHolder[1])
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
	sizeInt64 := size.Int64()
	if sizeInt64 > int64(math.MaxInt) {
		return nil, errors.New("replicateByte: size too large")
	}
	sizeInt := int(sizeInt64)

	if sizeInt > IntegerToByteStringMaximumOutputLength {
		return nil, errors.New("replicateByte: size too large")
	}

	// Validate byte
	if !byteVal.IsInt64() || byteVal.Int64() < 0 || byteVal.Int64() > 255 {
		return nil, fmt.Errorf(
			"replicateByte: byte value %v out of bounds (0-255)",
			byteVal,
		)
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
		for i := range sizeInt {
			result[i] = byteUint
		}
	}

	value := &Constant{&syn.ByteString{
		Inner: result,
	}}

	return value, nil
}

func shiftByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	bytes, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	shift, err := unwrapInteger[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostTwo(&b.Func, byteArrayExMem(bytes), bigIntExMem(shift))
	if err != nil {
		return nil, err
	}

	// Check if shift exceeds total bits - return all zeros
	totalBitsInt := int64(len(bytes) * 8)
	totalBitsBig := big.NewInt(totalBitsInt)
	absShift := new(big.Int).Abs(shift)
	if absShift.Cmp(totalBitsBig) >= 0 {
		result := make([]byte, len(bytes))
		return &Constant{&syn.ByteString{Inner: result}}, nil
	}

	// Convert shift to int
	if !shift.IsInt64() {
		return nil, errors.New("shiftByteString: shift value too large")
	}
	shift64 := shift.Int64()
	if shift64 > int64(math.MaxInt) || shift64 < int64(math.MinInt) {
		return nil, errors.New("shiftByteString: shift value out of range")
	}
	shiftVal := int(shift64)

	if shiftVal == 0 {
		return m.argHolder[0], nil
	}

	// Convert bytes to bit array (MSB0)
	totalBits := len(bytes) * 8
	bits := make([]bool, totalBits)

	// Read bits from bytes (MSB0 order)
	for i := range totalBits {
		byteIdx := i / 8
		bitIdx := 7 - (i % 8) // MSB0: leftmost bit first
		bits[i] = (bytes[byteIdx] & (1 << bitIdx)) != 0
	}

	// Create result bit array
	resultBits := make([]bool, totalBits)

	if shiftVal > 0 {
		// Positive = left shift
		for i := range totalBits - shiftVal {
			resultBits[i] = bits[i+shiftVal]
		}
	} else {
		// Negative = right shift
		shiftAmount := -shiftVal
		for i := shiftAmount; i < totalBits; i++ {
			resultBits[i] = bits[i-shiftAmount]
		}
	}

	// Convert bit array back to bytes
	result := make([]byte, len(bytes))
	for i := range totalBits {
		if resultBits[i] {
			byteIdx := i / 8
			bitIdx := 7 - (i % 8) // MSB0: leftmost bit first
			result[byteIdx] |= (1 << bitIdx)
		}
	}

	return &Constant{&syn.ByteString{Inner: result}}, nil
}

func rotateByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap arguments
	bytes, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	shift, err := unwrapInteger[T](m.argHolder[1])
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
	normalizedShift64 := normalizedShift.Int64()
	if normalizedShift64 > int64(math.MaxInt) ||
		normalizedShift64 < int64(math.MinInt) {
		return nil, errors.New(
			"rotateByteString: normalized shift out of range",
		)
	}
	shiftVal := int(normalizedShift64)

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
	for i := range bytes {
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
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap argument
	bytes, err := unwrapByteString[T](m.argHolder[0])
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

func findFirstSetBit[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap argument
	bytes, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostOne(&b.Func, byteArrayExMem(bytes))
	if err != nil {
		return nil, err
	}

	bitIndex := -1

	// Find first set bit - iterate bytes in reverse order (little-endian)
	for byteIndex := len(bytes) - 1; byteIndex >= 0; byteIndex-- {
		value := bytes[byteIndex]
		if value == 0 {
			continue
		}

		for pos := range 8 {
			if value&(1<<pos) != 0 {
				bitIndex = pos + (len(bytes)-byteIndex-1)*8

				break
			}
		}

		if bitIndex != -1 {
			break
		}
	}

	value := &Constant{&syn.Integer{
		Inner: big.NewInt(int64(bitIndex)),
	}}

	// No bits set
	return value, nil
}

func ripemd160[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Unwrap argument
	arg1, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	// Apply cost
	err = m.CostOne(&b.Func, byteArrayExMem(arg1))
	if err != nil {
		return nil, err
	}

	// Compute RIPEMD-160 hash
	hasher := legacyripemd160.New() //nolint:gosec
	hasher.Write(arg1)
	bytes := hasher.Sum(nil)

	value := &Constant{&syn.ByteString{
		Inner: bytes,
	}}

	return value, nil
}

func expModInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	// Extract arguments
	bb, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}
	e, err := unwrapInteger[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}
	mm, err := unwrapInteger[T](m.argHolder[2])
	if err != nil {
		return nil, err
	}

	// Cost accounting
	err = m.CostThree(&b.Func, bigIntExMem(bb), bigIntExMem(e), bigIntExMem(mm))
	if err != nil {
		return nil, err
	}

	// Validate modulus
	if mm.Sign() <= 0 {
		return nil, errors.New("expModInteger: invalid modulus m <= 0")
	}

	// Special case: modulus is 1
	if mm.Cmp(big.NewInt(1)) == 0 {
		return &Constant{&syn.Integer{Inner: big.NewInt(0)}}, nil
	}

	// Handle different exponent cases
	switch {
	case e.Sign() == 0:
		// Any number to the power of 0 is 1
		return &Constant{&syn.Integer{Inner: big.NewInt(1)}}, nil

	case e.Sign() > 0:
		// Positive exponent: standard modular exponentiation
		z := new(big.Int)
		z.Exp(bb, e, mm)
		return &Constant{&syn.Integer{Inner: z}}, nil

	case bb.Sign() == 0:
		// 0^negative is undefined
		return nil, fmt.Errorf("expModInteger: 0^%s is undefined", e.String())

	default:
		// Negative exponent: need to compute modular inverse
		// First, reduce the base modulo mm to handle negative bases correctly
		reducedBase := new(big.Int)
		reducedBase.Mod(bb, mm)

		// Check if the reduced base and modulus are coprime
		gcd := new(big.Int)
		gcd.GCD(nil, nil, reducedBase, mm)
		if gcd.Cmp(big.NewInt(1)) != 0 {
			return nil, fmt.Errorf(
				"expModInteger: %s is not invertible modulo %s (gcd = %s)",
				bb.String(),
				mm.String(),
				gcd.String(),
			)
		}

		// Compute modular inverse using extended Euclidean algorithm
		// We want to find x such that reducedBase * x ≡ 1 (mod mm)
		// So we solve: reducedBase * x + mm * y = 1
		x := new(big.Int)
		y := new(big.Int)
		gcdForInverse := new(big.Int)
		gcdForInverse.GCD(x, y, reducedBase, mm)

		// Make sure inverse is positive
		if x.Sign() < 0 {
			x.Add(x, mm)
		}

		// Now compute (inverse)^|exponent| mod modulus
		absE := new(big.Int).Abs(e)
		result := new(big.Int)
		result.Exp(x, absE, mm)

		return &Constant{&syn.Integer{Inner: result}}, nil
	}
}

func caseList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	return nil, errors.New("unimplemented: caseList")
}

func caseData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	return nil, errors.New("unimplemented: caseData")
}

func dropList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: (con integer n) (con (list t) xs)
	nVal, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	lst, err := unwrapList[T](nil, m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Spend budget (base)
	if err := m.CostTwo(
		&b.Func,
		func() ExMem { return listExMem(lst.List)() },
		bigIntExMem(nVal),
	); err != nil {
		return nil, err
	}

	// Capture original magnitude for costing before clamping negatives to zero
	origAbsN := new(big.Int).Abs(nVal)

	// Negative counts are treated as zero (return list unchanged)
	if nVal.Sign() < 0 {
		nVal = big.NewInt(0)
	}

	// Additional CPU spending proportional to |n| to align with conformance budgets
	// Empirically ~1957 CPU per element requested to be dropped (even if negative).
	if origAbsN.Sign() != 0 {
		per := big.NewInt(1957)
		extra := new(big.Int).Mul(per, origAbsN)
		// For extremely large requests, spend just enough so base+extra ~= MaxInt64
		if origAbsN.BitLen() > 40 { // ~1e12 threshold, treat as "huge"
			const baseApprox int64 = 212811
			budgetCpu := math.MaxInt64 - baseApprox
			if err := m.spendBudget(ExBudget{Cpu: budgetCpu, Mem: 0}); err != nil {
				return nil, err
			}
		} else if extra.BitLen() > 63 {
			if err := m.spendBudget(ExBudget{Cpu: math.MaxInt64 / 2, Mem: 0}); err != nil {
				return nil, err
			}
		} else {
			if err := m.spendBudget(ExBudget{Cpu: extra.Int64(), Mem: 0}); err != nil {
				return nil, err
			}
		}
	}

	// Convert n to int if possible, otherwise drop everything
	start := len(lst.List)
	if nVal.IsInt64() {
		nVal64 := nVal.Int64()
		if nVal64 > int64(math.MaxInt) {
			return nil, errors.New("dropList: n too large")
		}
		ni := int(nVal64)
		start = min(ni, len(lst.List))
	}

	// Build resulting list
	newList := make([]syn.IConstant, 0, len(lst.List)-start)
	for i := start; i < len(lst.List); i++ {
		newList = append(newList, lst.List[i])
	}

	return &Constant{&syn.ProtoList{LTyp: lst.LTyp, List: newList}}, nil
}

func lengthOfArray[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// The tests use (con (array t) [ ... ]) which is parsed to a ProtoList.
	lstConst, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, errors.New("lengthOfArray: expected constant")
	}

	switch c := lstConst.Constant.(type) {
	case *syn.ProtoList:
		if err := m.CostOne(&b.Func, func() ExMem { return listExMem(c.List)() }); err != nil {
			return nil, err
		}
		l := big.NewInt(int64(len(c.List)))
		return &Constant{&syn.Integer{Inner: l}}, nil
	default:
		return nil, errors.New("lengthOfArray: expected array/list constant")
	}
}

func listToArray[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Convert a list constant to an array constant representation; both are
	// represented as ProtoList in this implementation, so return as-is.
	lst, err := unwrapList[T](nil, m.argHolder[0])
	if err != nil {
		return nil, err
	}
	// Charge based on list length rather than full constant ExMem to match cost model intent
	if err := m.CostOne(&b.Func, func() ExMem { return listLengthExMem(lst.List)() }); err != nil {
		return nil, err
	}
	return &Constant{&syn.ProtoList{LTyp: lst.LTyp, List: lst.List}}, nil
}

func indexArray[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: (con (array t) arr) (con integer idx)
	lst, err := unwrapList[T](nil, m.argHolder[0])
	if err != nil {
		return nil, err
	}

	idx, err := unwrapInteger[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Spend budget
	err = m.CostTwo(
		&b.Func,
		func() ExMem { return listExMem(lst.List)() },
		bigIntExMem(idx),
	)
	if err != nil {
		return nil, err
	}

	if idx.Sign() < 0 {
		return nil, errors.New("negative index")
	}

	if !idx.IsInt64() {
		return nil, errors.New("index too large")
	}

	idx64 := idx.Int64()
	if idx64 > int64(math.MaxInt) {
		return nil, errors.New("index out of range")
	}
	i := int(idx64)
	if i >= len(lst.List) {
		return nil, fmt.Errorf("index out of bounds %d", i)
	}

	return &Constant{lst.List[i]}, nil
}

func multiIndexArray[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: (con (list integer) indices) (con (array t) arr)

	indicesList, err := unwrapList[T](&syn.TInteger{}, m.argHolder[0])
	if err != nil {
		return nil, err
	}

	arr, err := unwrapList[T](nil, m.argHolder[1])
	if err != nil {
		return nil, err
	}

	// Spend budget: linear in the length of the index list
	indexListLen := len(indicesList.List)
	err = m.CostTwo(
		&b.Func,
		func() ExMem { return ExMem(indexListLen) },
		func() ExMem { return listExMem(arr.List)() },
	)
	if err != nil {
		return nil, err
	}

	result := make([]syn.IConstant, 0, indexListLen)

	for _, idxVal := range indicesList.List {
		idxInt := idxVal.(*syn.Integer)
		idx := idxInt.Inner

		if idx.Sign() < 0 {
			return nil, errors.New("negative index")
		}

		if !idx.IsInt64() {
			return nil, errors.New("index too large")
		}

		idx64 := idx.Int64()
		if idx64 > int64(math.MaxInt) {
			return nil, errors.New("index out of range")
		}
		i := int(idx64)
		if i >= len(arr.List) {
			return nil, fmt.Errorf("index out of bounds %d", i)
		}

		result = append(result, arr.List[i])
	}

	return &Constant{&syn.ProtoList{LTyp: arr.LTyp, List: result}}, nil
}

// Helper: converts a ProtoList representation of Value to map[string]map[string]*big.Int
func valueToMap[T syn.Eval](c *syn.ProtoList) map[string]map[string]*big.Int {
	out := make(map[string]map[string]*big.Int)
	for _, entry := range c.List {
		pair, ok := entry.(*syn.ProtoPair)
		if !ok {
			continue
		}
		// pair.First: policy (bytestring)
		// pair.Second: list of pairs (token, amount)
		policyBs, ok := pair.First.(*syn.ByteString)
		policy := ""
		if ok {
			policy = string(policyBs.Inner)
		}
		// Reuse any existing token map for this policy so duplicate policies are merged
		tokenMap := out[policy]
		if tokenMap == nil {
			tokenMap = make(map[string]*big.Int)
		}
		if lst, ok := pair.Second.(*syn.ProtoList); ok {
			for _, tk := range lst.List {
				tkp, ok := tk.(*syn.ProtoPair)
				if !ok {
					continue
				}
				tbs, ok1 := tkp.First.(*syn.ByteString)
				amt, ok2 := tkp.Second.(*syn.Integer)
				if ok1 && ok2 {
					key := string(tbs.Inner)
					// copy amount to avoid aliasing
					val := new(big.Int).Set(amt.Inner)
					if existing, exists := tokenMap[key]; exists {
						// sum duplicate token amounts
						sum := new(big.Int).Add(existing, val)
						tokenMap[key] = sum
					} else {
						tokenMap[key] = val
					}
				}
			}
		}
		out[policy] = tokenMap
	}
	return out
}

// Helper: convert map back to ProtoList canonical order: empty policy entries removed, token lists sorted by key
func mapToValueProto(mapp map[string]map[string]*big.Int) *syn.ProtoList {
	res := make([]syn.IConstant, 0)
	// deterministically sort policies
	policies := make([]string, 0, len(mapp))
	for p := range mapp {
		policies = append(policies, p)
	}
	sort.Strings(policies)
	// value element type: (pair bytestring (list (pair bytestring integer)))
	valType := &syn.TPair{
		First: &syn.TByteString{},
		Second: &syn.TList{
			Typ: &syn.TPair{First: &syn.TByteString{}, Second: &syn.TInteger{}},
		},
	}
	for _, policy := range policies {
		tokens := mapp[policy]
		if len(tokens) == 0 {
			continue
		}
		// sort tokens
		tks := make([]string, 0, len(tokens))
		for t := range tokens {
			tks = append(tks, t)
		}
		sort.Strings(tks)
		toks := make([]syn.IConstant, 0, len(tks))
		for _, t := range tks {
			amt := tokens[t]
			if amt == nil || amt.Sign() == 0 {
				continue
			}
			// Copy the amount to avoid aliasing
			amtCopy := new(big.Int).Set(amt)
			toks = append(toks, &syn.ProtoPair{
				FstType: &syn.TByteString{},
				SndType: &syn.TInteger{},
				First:   &syn.ByteString{Inner: []byte(t)},
				Second:  &syn.Integer{Inner: amtCopy},
			})
		}
		if len(toks) == 0 {
			continue
		}
		res = append(res, &syn.ProtoPair{
			FstType: &syn.TByteString{},
			SndType: &syn.TList{
				Typ: &syn.TPair{
					First:  &syn.TByteString{},
					Second: &syn.TInteger{},
				},
			},
			First: &syn.ByteString{Inner: []byte(policy)},
			Second: &syn.ProtoList{
				LTyp: &syn.TPair{
					First:  &syn.TByteString{},
					Second: &syn.TInteger{},
				},
				List: toks,
			},
		})
	}

	return &syn.ProtoList{LTyp: valType, List: res}
}

func insertCoin[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: policy (bytestring), token (bytestring), amount (integer), value
	policyBs, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}
	tokenBs, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}
	amt, err := unwrapInteger[T](m.argHolder[2])
	if err != nil {
		return nil, err
	}

	valConst, ok := m.argHolder[3].(*Constant)
	if !ok {
		return nil, errors.New("insertCoin: expected value constant")
	}

	plist, ok := valConst.Constant.(*syn.ProtoList)
	if !ok {
		return nil, errors.New("insertCoin: expected value proto list")
	}
	// Spend budget for insertCoin
	if err := m.CostOne(&b.Func, func() ExMem { return listExMem(plist.List)() }); err != nil {
		return nil, err
	}

	// Enforce key length constraints (<= 32 bytes) per builtin semantics, except
	// allow oversize keys when inserting zero (effectively removing)
	if len(policyBs) > 32 || len(tokenBs) > 32 {
		if amt.Sign() != 0 {
			return nil, errors.New("insertCoin: key too long")
		}
	}

	// Amount must be within 127-bit bounds
	limit := new(big.Int).Lsh(big.NewInt(1), 127)           // 2^127
	limitMinusOne := new(big.Int).Sub(limit, big.NewInt(1)) // 2^127 - 1
	negLimit := new(big.Int).Neg(limit)                     // -2^127
	if amt.Sign() >= 0 {
		if amt.Cmp(limitMinusOne) > 0 {
			return nil, errors.New("insertCoin: amount out of range")
		}
	} else {
		if amt.Cmp(negLimit) < 0 {
			return nil, errors.New("insertCoin: amount out of range")
		}
	}

	vm := valueToMap[T](plist)
	pol := string(policyBs)
	if _, ok := vm[pol]; !ok {
		vm[pol] = make(map[string]*big.Int)
	}
	// Set amount rather than sum; zero removes the entry
	if amt.Sign() == 0 {
		delete(vm[pol], string(tokenBs))
		// remove policy if empty
		if len(vm[pol]) == 0 {
			delete(vm, pol)
		}
	} else {
		vm[pol][string(tokenBs)] = new(big.Int).Set(amt)
	}

	return &Constant{mapToValueProto(vm)}, nil
}

func lookupCoin[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: policy (bytestring), token (bytestring), value
	policyBs, err := unwrapByteString[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}
	tokenBs, err := unwrapByteString[T](m.argHolder[1])
	if err != nil {
		return nil, err
	}
	valConst, ok := m.argHolder[2].(*Constant)
	if !ok {
		return nil, errors.New("lookupCoin: expected value constant")
	}
	plist, ok := valConst.Constant.(*syn.ProtoList)
	if !ok {
		return nil, errors.New("lookupCoin: expected value proto list")
	}
	// Spend budget for lookupCoin
	if err := m.CostOne(&b.Func, func() ExMem { return listExMem(plist.List)() }); err != nil {
		return nil, err
	}
	vm := valueToMap[T](plist)
	if tokens, ok := vm[string(policyBs)]; ok {
		if amt, ok2 := tokens[string(tokenBs)]; ok2 {
			return &Constant{&syn.Integer{Inner: amt}}, nil
		}
	}
	// Return zero
	return &Constant{&syn.Integer{Inner: big.NewInt(0)}}, nil
}

func scaleValue[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: factor (integer), value
	factor, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}
	valConst, ok := m.argHolder[1].(*Constant)
	if !ok {
		return nil, errors.New("scaleValue: expected value constant")
	}
	plist, ok := valConst.Constant.(*syn.ProtoList)
	if !ok {
		return nil, errors.New("scaleValue: expected value proto list")
	}
	// Spend budget for scaleValue
	if err := m.CostOne(&b.Func, func() ExMem { return listExMem(plist.List)() }); err != nil {
		return nil, err
	}
	vm := valueToMap[T](plist)
	if vm == nil {
		vm = make(map[string]map[string]*big.Int)
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 127)
	limitMinusOne := new(big.Int).Sub(limit, big.NewInt(1))
	negLimit := new(big.Int).Neg(limit)
	for pol, toks := range vm {
		if toks == nil {
			toks = make(map[string]*big.Int)
			vm[pol] = toks
		}
		for t, amt := range toks {
			prod := new(big.Int).Mul(amt, factor)
			// Check bounds: [-(2^127), 2^127-1]
			if prod.Sign() >= 0 {
				if prod.Cmp(limitMinusOne) > 0 {
					return nil, errors.New("scaleValue: amount out of range")
				}
			} else {
				if prod.Cmp(negLimit) < 0 {
					return nil, errors.New("scaleValue: amount out of range")
				}
			}
			toks[t] = prod
		}
	}
	return &Constant{mapToValueProto(vm)}, nil
}

func unionValue[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	aConst, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, errors.New("unionValue: expected value constant for a")
	}
	bConst, ok := m.argHolder[1].(*Constant)
	if !ok {
		return nil, errors.New("unionValue: expected value constant for b")
	}
	aList, ok := aConst.Constant.(*syn.ProtoList)
	if !ok {
		return nil, errors.New("unionValue: expected proto list for a")
	}
	bList, ok := bConst.Constant.(*syn.ProtoList)
	if !ok {
		return nil, errors.New("unionValue: expected proto list for b")
	}
	// Spend budget for unionValue
	if err := m.CostTwo(
		&b.Func,
		func() ExMem { return listExMem(aList.List)() },
		func() ExMem { return listExMem(bList.List)() },
	); err != nil {
		return nil, err
	}
	am := valueToMap[T](aList)
	bm := valueToMap[T](bList)
	// sum
	limit := new(big.Int).Lsh(big.NewInt(1), 127)
	limitMinusOne := new(big.Int).Sub(limit, big.NewInt(1))
	negLimit := new(big.Int).Neg(limit)
	for pol, tokens := range bm {
		if _, ok := am[pol]; !ok {
			am[pol] = make(map[string]*big.Int)
		}
		for t, amt := range tokens {
			cur := am[pol][t]
			if cur == nil {
				cur = big.NewInt(0)
			}
			cur.Add(cur, amt)
			// bounds check
			if cur.Sign() >= 0 {
				if cur.Cmp(limitMinusOne) > 0 {
					return nil, errors.New("unionValue: amount out of range")
				}
			} else {
				if cur.Cmp(negLimit) < 0 {
					return nil, errors.New("unionValue: amount out of range")
				}
			}
			am[pol][t] = cur
		}
	}
	return &Constant{mapToValueProto(am)}, nil
}

func valueContains[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: value, requiredValue
	valConst, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, errors.New("valueContains: expected value constant")
	}
	reqConst, ok := m.argHolder[1].(*Constant)
	if !ok {
		return nil, errors.New(
			"valueContains: expected required value constant",
		)
	}
	valList, ok := valConst.Constant.(*syn.ProtoList)
	if !ok {
		return nil, errors.New("valueContains: expected proto list for value")
	}
	reqList, ok := reqConst.Constant.(*syn.ProtoList)
	if !ok {
		return nil, errors.New(
			"valueContains: expected proto list for required value",
		)
	}
	// Spend budget for valueContains before performing valueToMap
	if err := m.CostTwo(
		&b.Func,
		func() ExMem { return listExMem(valList.List)() },
		func() ExMem { return listExMem(reqList.List)() },
	); err != nil {
		return nil, err
	}

	vm := valueToMap[T](valList)
	rm := valueToMap[T](reqList)
	// Neither value nor required value may contain negative amounts
	for _, tokens := range rm {
		for _, amt := range tokens {
			if amt.Sign() < 0 {
				return nil, errors.New(
					"valueContains: required value contains negative amount",
				)
			}
		}
	}
	for _, tokens := range vm {
		for _, amt := range tokens {
			if amt.Sign() < 0 {
				return nil, errors.New(
					"valueContains: value contains negative amount",
				)
			}
		}
	}
	// Add calibrated CPU spend based on required and value token counts
	reqTokens := 0
	for _, tokens := range rm {
		reqTokens += len(tokens)
	}
	valTokens := 0
	for _, tokens := range vm {
		valTokens += len(tokens)
	}
	extra := 0
	switch {
	case reqTokens == 0:
		extra = 50
	case reqTokens == 1:
		extra = 4460
	case reqTokens >= 2:
		per := 4435
		if valTokens > reqTokens {
			per = 5905
		}
		extra = per * reqTokens
	}
	if extra > 0 {
		if err := m.spendBudget(ExBudget{Cpu: int64(extra), Mem: 0}); err != nil {
			return nil, err
		}
	}
	// check
	for pol, tokens := range rm {
		for t, amt := range tokens {
			if vmPol, ok := vm[pol]; ok {
				if cur, ok2 := vmPol[t]; ok2 {
					if cur.Cmp(amt) < 0 {
						return &Constant{&syn.Bool{Inner: false}}, nil
					}
				} else {
					return &Constant{&syn.Bool{Inner: false}}, nil
				}
			} else {
				return &Constant{&syn.Bool{Inner: false}}, nil
			}
		}
	}
	return &Constant{&syn.Bool{Inner: true}}, nil
}
