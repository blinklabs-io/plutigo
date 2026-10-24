// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cek

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha3"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"sync"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
	"github.com/btcsuite/btcd/btcec/v2"
	ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	schnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	sha256 "github.com/minio/sha256-simd"
	"golang.org/x/crypto/blake2b"
	legacyripemd160 "golang.org/x/crypto/ripemd160" //nolint:staticcheck,gosec
	legacykeccak "golang.org/x/crypto/sha3"
)

var (
	sharedIntegerType  = &syn.TInteger{}
	sharedDataType     = &syn.TData{}
	sharedListDataType = &syn.TList{Typ: sharedDataType}
	sharedPairDataType = &syn.TPair{First: sharedDataType, Second: sharedDataType}
	blsScalarModulus   = blsfr.Modulus()
	blsMSMScalarLimit  = new(big.Int).Lsh(big.NewInt(1), 4095)
)

// normalizeBLSScalar maps an arbitrary UPLC integer into the BLS12-381 scalar
// field. gnark-crypto's GLV implementation expects a field-sized scalar and
// can panic when handed a larger integer directly.
func normalizeBLSScalar(scalar *big.Int) *big.Int {
	return new(big.Int).Mod(scalar, blsScalarModulus)
}

func validBLSMSMScalar(scalar *big.Int) bool {
	if scalar.Sign() < 0 {
		return scalar.Cmp(new(big.Int).Neg(blsMSMScalarLimit)) >= 0
	}
	return scalar.Cmp(blsMSMScalarLimit) < 0
}

// checkBLSScalarMulBound fails a G1 or G2 scalarMul whose scalar is outside
// the signed 4096-bit range: from van Rossem the reference bounds it like
// multiScalarMul (BLS12_381.G1/G2.scalarMulE), failing after costing.
func (m *Machine[T]) checkBLSScalarMulBound(fn builtin.DefaultFunction, scalar *big.Int) error {
	if (m.semantics == SemanticsVariantD || m.semantics == SemanticsVariantE) &&
		!validBLSMSMScalar(scalar) {
		return &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: fn.String(),
			Message: "scalar is outside the signed 4096-bit range",
		}
	}
	return nil
}

func validateBLSMSMScalars[T syn.Eval](
	m *Machine[T],
	scalars []syn.IConstant,
	builtinName string,
) error {
	for _, scalar := range scalars {
		if err := m.checkBuiltinCancellation(); err != nil {
			return err
		}
		integer, ok := scalar.(*syn.Integer)
		if !ok || integer == nil || integer.Inner == nil {
			return &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: builtinName,
				Message: "expected integer list",
			}
		}
		if !validBLSMSMScalar(integer.Inner) {
			return &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: builtinName,
				Message: "scalar is outside the signed 4096-bit range",
			}
		}
	}
	return nil
}

const (
	ed25519VerifyCacheLimit  = 4096
	ed25519VerifyMaxCacheMsg = 64
	ed25519VerifyCacheShards = 64
	ed25519VerifyShardLimit  = ed25519VerifyCacheLimit / ed25519VerifyCacheShards
	valueDataMaxSize         = 40_000
)

type ed25519VerifyCacheKey struct {
	publicKey [ed25519.PublicKeySize]byte
	signature [ed25519.SignatureSize]byte
	message   [ed25519VerifyMaxCacheMsg]byte
	msgLen    uint8
}

type ed25519VerifyCacheShard struct {
	sync.Mutex
	values map[ed25519VerifyCacheKey]bool
}

type ed25519VerifyCacheStore struct {
	shards [ed25519VerifyCacheShards]ed25519VerifyCacheShard
}

var ed25519VerifyCache = newEd25519VerifyCacheStore()

func newEd25519VerifyCacheStore() *ed25519VerifyCacheStore {
	cache := &ed25519VerifyCacheStore{}
	for i := range cache.shards {
		cache.shards[i].values = make(map[ed25519VerifyCacheKey]bool, ed25519VerifyShardLimit)
	}
	return cache
}

func newEd25519VerifyCacheKey(
	publicKey []byte,
	message []byte,
	signature []byte,
) ed25519VerifyCacheKey {
	var key ed25519VerifyCacheKey
	copy(key.publicKey[:], publicKey)
	copy(key.signature[:], signature)
	copy(key.message[:], message)
	key.msgLen = uint8(len(message))
	return key
}

func (c *ed25519VerifyCacheStore) get(key *ed25519VerifyCacheKey) (bool, bool) {
	shard := c.shard(key)
	shard.Lock()
	value, ok := shard.values[*key]
	shard.Unlock()
	return value, ok
}

func (c *ed25519VerifyCacheStore) store(key *ed25519VerifyCacheKey, value bool) {
	shard := c.shard(key)
	shard.Lock()
	if _, ok := shard.values[*key]; !ok && len(shard.values) >= ed25519VerifyShardLimit {
		// Reset-on-full: clear the shard rather than permanently refusing new
		// admissions once it reaches capacity. This mirrors the cache-wide
		// reset-on-full behavior introduced in #327, applied per shard.
		shard.values = make(map[ed25519VerifyCacheKey]bool, ed25519VerifyShardLimit)
	}
	shard.values[*key] = value
	shard.Unlock()
}

func (c *ed25519VerifyCacheStore) shard(key *ed25519VerifyCacheKey) *ed25519VerifyCacheShard {
	hash := binary.LittleEndian.Uint64(key.publicKey[:8])
	hash ^= bits.RotateLeft64(binary.LittleEndian.Uint64(key.signature[:8]), 21)
	hash ^= bits.RotateLeft64(
		binary.LittleEndian.Uint64(key.signature[ed25519.SignatureSize-8:]),
		42,
	)
	if key.msgLen > 0 {
		lastMsgByte := key.message[int(key.msgLen)-1]
		hash ^= uint64(key.message[0]) << 8
		hash ^= uint64(lastMsgByte) << 16
	}
	hash ^= uint64(key.msgLen) * 0x9e3779b97f4a7c15
	return &c.shards[hash&(ed25519VerifyCacheShards-1)]
}

func absUint64(v int64) uint64 {
	if v < 0 {
		return uint64(^v) + 1
	}
	return uint64(v)
}

func mulInt64Exact(left, right int64) (int64, bool) {
	if left == 0 || right == 0 {
		return 0, true
	}

	negative := (left < 0) != (right < 0)
	hi, lo := bits.Mul64(absUint64(left), absUint64(right))
	if hi != 0 {
		return 0, false
	}

	if negative {
		if lo > 1<<63 {
			return 0, false
		}
		if lo == 1<<63 {
			return math.MinInt64, true
		}
		return -int64(lo), true
	}

	if lo > math.MaxInt64 {
		return 0, false
	}
	return int64(lo), true
}

func quoInt64Exact(left, right int64) (int64, bool) {
	if right == 0 {
		return 0, false
	}
	if left == math.MinInt64 && right == -1 {
		return 0, false
	}
	return left / right, true
}

func bigIntMod256Byte(i *big.Int) byte {
	if i == nil {
		return 0
	}

	words := i.Bits()
	if len(words) == 0 {
		return 0
	}

	lowByte := byte(words[0])
	if i.Sign() >= 0 || lowByte == 0 {
		return lowByte
	}

	return byte(256 - int(lowByte))
}

// ============================================================================
// INTEGER OPERATIONS
// Functions: addInteger, subtractInteger, multiplyInteger, divideInteger,
//            quotientInteger, remainderInteger, modInteger, equalsInteger,
//            lessThanInteger, lessThanEqualsInteger
// ============================================================================

func integerBuiltinName(fn builtin.DefaultFunction) string {
	switch fn {
	case builtin.AddInteger:
		return "addInteger"
	case builtin.SubtractInteger:
		return "subtractInteger"
	case builtin.MultiplyInteger:
		return "multiplyInteger"
	case builtin.DivideInteger:
		return "divideInteger"
	case builtin.QuotientInteger:
		return "quotientInteger"
	case builtin.RemainderInteger:
		return "remainderInteger"
	case builtin.ModInteger:
		return "modInteger"
	case builtin.EqualsInteger:
		return "equalsInteger"
	case builtin.LessThanInteger:
		return "lessThanInteger"
	case builtin.LessThanEqualsInteger:
		return "lessThanEqualsInteger"
	default:
		return fn.String()
	}
}

func integerMetadata(i *big.Int) string {
	return fmt.Sprintf("sign=%d, bit length=%d", i.Sign(), i.BitLen())
}

func (m *Machine[T]) evalBinaryIntegerBuiltin(
	fn builtin.DefaultFunction,
	leftVal Value[T],
	rightVal Value[T],
) (Value[T], bool, error) {
	switch fn {
	case builtin.AddInteger,
		builtin.SubtractInteger,
		builtin.MultiplyInteger,
		builtin.DivideInteger,
		builtin.QuotientInteger,
		builtin.RemainderInteger,
		builtin.ModInteger,
		builtin.EqualsInteger,
		builtin.LessThanInteger,
		builtin.LessThanEqualsInteger:
	default:
		return nil, false, nil
	}

	arg1, err := unwrapIntegerInfo[T](leftVal)
	if err != nil {
		return nil, true, err
	}

	arg2, err := unwrapIntegerInfo[T](rightVal)
	if err != nil {
		return nil, true, err
	}

	if err := m.CostTwoExMem(&fn, arg1.exMem, arg2.exMem); err != nil {
		return nil, true, err
	}

	switch fn {
	case builtin.AddInteger:
		if arg1.isInt64 && arg2.isInt64 {
			left := arg1.int64Val
			right := arg2.int64Val
			if (right > 0 && left <= math.MaxInt64-right) ||
				(right < 0 && left >= math.MinInt64-right) ||
				right == 0 {
				return m.int64Constant(left + right), true, nil
			}
		}

		var newInt big.Int
		newInt.Add(arg1.value, arg2.value)
		return m.allocConstant(m.allocIntegerConstant(&newInt)), true, nil
	case builtin.SubtractInteger:
		if arg1.isInt64 && arg2.isInt64 {
			left := arg1.int64Val
			right := arg2.int64Val
			if (right > 0 && left >= math.MinInt64+right) ||
				(right < 0 && left <= math.MaxInt64+right) ||
				right == 0 {
				return m.int64Constant(left - right), true, nil
			}
		}

		var newInt big.Int
		newInt.Sub(arg1.value, arg2.value)
		return m.allocConstant(m.allocIntegerConstant(&newInt)), true, nil
	case builtin.MultiplyInteger:
		if arg1.isInt64 && arg2.isInt64 {
			if result, ok := mulInt64Exact(arg1.int64Val, arg2.int64Val); ok {
				return m.int64Constant(result), true, nil
			}
		}

		var newInt big.Int
		newInt.Mul(arg1.value, arg2.value)
		return m.allocConstant(m.allocIntegerConstant(&newInt)), true, nil
	case builtin.DivideInteger:
		if arg2.value.Sign() == 0 {
			return nil, true, &BuiltinError{
				Code:    ErrCodeDivisionByZero,
				Builtin: integerBuiltinName(fn),
				Message: "division by zero",
			}
		}

		if arg1.isInt64 && arg2.isInt64 {
			left := arg1.int64Val
			right := arg2.int64Val
			if q, ok := quoInt64Exact(left, right); ok {
				r := left % right
				if r != 0 && ((left < 0) != (right < 0)) {
					q--
				}
				return m.int64Constant(q), true, nil
			}
		}

		var quotient big.Int
		var remainder big.Int
		quotient.Quo(arg1.value, arg2.value)
		remainder.Rem(arg1.value, arg2.value)
		if remainder.Sign() != 0 && arg1.value.Sign() != arg2.value.Sign() {
			quotient.Sub(&quotient, big.NewInt(1))
		}
		return m.allocConstant(m.allocIntegerConstant(&quotient)), true, nil
	case builtin.QuotientInteger:
		if arg2.value.Sign() == 0 {
			return nil, true, &BuiltinError{
				Code:    ErrCodeDivisionByZero,
				Builtin: integerBuiltinName(fn),
				Message: "division by zero",
			}
		}

		if arg1.isInt64 && arg2.isInt64 {
			if result, ok := quoInt64Exact(arg1.int64Val, arg2.int64Val); ok {
				return m.int64Constant(result), true, nil
			}
		}

		var quotient big.Int
		quotient.Quo(arg1.value, arg2.value)
		return m.allocConstant(m.allocIntegerConstant(&quotient)), true, nil
	case builtin.RemainderInteger:
		if arg2.value.Sign() == 0 {
			return nil, true, &BuiltinError{
				Code:    ErrCodeDivisionByZero,
				Builtin: integerBuiltinName(fn),
				Message: "division by zero",
			}
		}

		if arg1.isInt64 && arg2.isInt64 {
			left := arg1.int64Val
			right := arg2.int64Val
			if left == math.MinInt64 && right == -1 {
				return m.int64Constant(0), true, nil
			}
			return m.int64Constant(left % right), true, nil
		}

		var remainder big.Int
		remainder.Rem(arg1.value, arg2.value)
		return m.allocConstant(m.allocIntegerConstant(&remainder)), true, nil
	case builtin.ModInteger:
		if arg2.value.Sign() == 0 {
			return nil, true, &BuiltinError{
				Code:    ErrCodeDivisionByZero,
				Builtin: integerBuiltinName(fn),
				Message: "division by zero",
			}
		}

		if arg1.isInt64 && arg2.isInt64 {
			left := arg1.int64Val
			right := arg2.int64Val
			var remainder int64
			if left == math.MinInt64 && right == -1 {
				remainder = 0
			} else {
				remainder = left % right
			}
			if remainder != 0 && ((left < 0) != (right < 0)) {
				remainder += right
			}
			return m.int64Constant(remainder), true, nil
		}

		var remainder big.Int
		remainder.Rem(arg1.value, arg2.value)
		if remainder.Sign() != 0 && arg1.value.Sign() != arg2.value.Sign() {
			remainder.Add(&remainder, arg2.value)
		}
		return m.allocConstant(m.allocIntegerConstant(&remainder)), true, nil
	case builtin.EqualsInteger:
		return boolConstant(arg1.value.Cmp(arg2.value) == 0), true, nil
	case builtin.LessThanInteger:
		return boolConstant(arg1.value.Cmp(arg2.value) == -1), true, nil
	case builtin.LessThanEqualsInteger:
		res := arg1.value.Cmp(arg2.value)
		return boolConstant(res == -1 || res == 0), true, nil
	default:
		return nil, false, nil
	}
}

func (m *Machine[T]) evalUnaryBuiltinFast(
	fn builtin.DefaultFunction,
	arg Value[T],
) (Value[T], bool, error) {
	if arg == nil {
		return nil, true, internalError("unary builtin evaluated nil argument")
	}
	if err := m.checkBoundedArgs1(fn, arg); err != nil {
		return nil, true, err
	}

	switch fn {
	case builtin.FstPair:
		if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
			return nil, true, err
		}
		first, _, err := splitPairValue(m, arg)
		if err != nil {
			return nil, true, err
		}
		if first == nil {
			return nil, true, &InternalError{
				Code:    ErrCodeInternalError,
				Message: "fstPair returned nil constant",
			}
		}
		return first, true, nil
	case builtin.SndPair:
		if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
			return nil, true, err
		}
		_, second, err := splitPairValue(m, arg)
		if err != nil {
			return nil, true, err
		}
		if second == nil {
			return nil, true, &InternalError{
				Code:    ErrCodeInternalError,
				Message: "sndPair returned nil constant",
			}
		}
		return second, true, nil
	case builtin.HeadList:
		if dataList, ok := arg.(*dataListValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
				return nil, true, err
			}
			if len(dataList.items) == 0 {
				return nil, true, &BuiltinError{
					Code:    ErrCodeOutOfBounds,
					Builtin: "headList",
					Message: "headList on an empty list",
				}
			}
			return m.allocDataValue(dataList.items[0]), true, nil
		}
		if dataMap, ok := arg.(*dataMapValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
				return nil, true, err
			}
			if len(dataMap.items) == 0 {
				return nil, true, &BuiltinError{
					Code:    ErrCodeOutOfBounds,
					Builtin: "headList",
					Message: "headList on an empty list",
				}
			}
			return m.allocDataPairValue(dataMap.items[0][0], dataMap.items[0][1]), true, nil
		}
		if lazy, ok := arg.(lazyListValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
				return nil, true, err
			}
			return lazy.lazyHead(m), true, nil
		}
		list, err := unwrapList[T](nil, arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, listExMem(list.List)); err != nil {
			return nil, true, err
		}
		if len(list.List) == 0 {
			return nil, true, &BuiltinError{
				Code:    ErrCodeOutOfBounds,
				Builtin: "headList",
				Message: "headList on an empty list",
			}
		}
		if list.List[0] == nil {
			return nil, true, &InternalError{
				Code:    ErrCodeInternalError,
				Message: "headList returned nil constant",
			}
		}
		return dataAwareConstantValue(m, list.List[0]), true, nil
	case builtin.TailList:
		if dataList, ok := arg.(*dataListValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
				return nil, true, err
			}
			if len(dataList.items) == 0 {
				return nil, true, &BuiltinError{
					Code:    ErrCodeOutOfBounds,
					Builtin: "tailList",
					Message: "tailList on an empty list",
				}
			}
			return m.allocDataListValue(dataList.items[1:]), true, nil
		}
		if dataMap, ok := arg.(*dataMapValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
				return nil, true, err
			}
			if len(dataMap.items) == 0 {
				return nil, true, &BuiltinError{
					Code:    ErrCodeOutOfBounds,
					Builtin: "tailList",
					Message: "tailList on an empty list",
				}
			}
			return m.allocDataMapValue(dataMap.items[1:]), true, nil
		}
		if lazy, ok := arg.(lazyListValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
				return nil, true, err
			}
			return lazy.lazyTail(m), true, nil
		}
		list, err := unwrapList[T](nil, arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, listExMem(list.List)); err != nil {
			return nil, true, err
		}
		if len(list.List) == 0 {
			return nil, true, &BuiltinError{
				Code:    ErrCodeOutOfBounds,
				Builtin: "tailList",
				Message: "tailList on an empty list",
			}
		}
		return m.allocConstant(m.allocProtoListConstant(list.LTyp, list.List[1:])), true, nil
	case builtin.NullList:
		if v, ok := arg.(*dataListValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](v)); err != nil {
				return nil, true, err
			}
			return boolConstant(len(v.items) == 0), true, nil
		}
		if w, ok := arg.(*dataMapValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](w)); err != nil {
				return nil, true, err
			}
			return boolConstant(len(w.items) == 0), true, nil
		}
		if lazy, ok := arg.(lazyListValue[T]); ok {
			if err := m.CostOne(&fn, valueExMem[T](arg)); err != nil {
				return nil, true, err
			}
			return boolConstant(lazy.lazyLen() == 0), true, nil
		}
		list, err := unwrapList[T](nil, arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, listExMem(list.List)); err != nil {
			return nil, true, err
		}
		return boolConstant(len(list.List) == 0), true, nil
	case builtin.UnConstrData:
		datum, err := unwrapData[T](arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, dataExMem(datum)); err != nil {
			return nil, true, err
		}
		constr, ok := datum.(*data.Constr)
		if !ok {
			return nil, true, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "unConstrData",
				Message: "data is not a constr",
			}
		}
		tag := constr.Tag
		if tag == nil {
			tag = new(big.Int)
		}
		return m.allocPairValue(
			m.allocConstant(m.allocIntegerConstant(tag)),
			m.allocDataListValue(constr.Fields),
		), true, nil
	case builtin.UnMapData:
		datum, err := unwrapData[T](arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, dataExMem(datum)); err != nil {
			return nil, true, err
		}
		dataMap, ok := datum.(*data.Map)
		if !ok {
			return nil, true, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "unMapData",
				Message: "data is not a map",
			}
		}
		return m.allocDataMapValue(dataMap.Pairs), true, nil
	case builtin.UnListData:
		datum, err := unwrapData[T](arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, dataExMem(datum)); err != nil {
			return nil, true, err
		}
		dataList, ok := datum.(*data.List)
		if !ok {
			return nil, true, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "unListData",
				Message: "data is not a list",
			}
		}
		return m.allocDataListValue(dataList.Items), true, nil
	case builtin.UnIData:
		datum, err := unwrapData[T](arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, dataExMem(datum)); err != nil {
			return nil, true, err
		}
		integer, ok := datum.(*data.Integer)
		if !ok {
			return nil, true, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "unIData",
				Message: "data is not a integer",
			}
		}
		if integer.Inner.IsInt64() {
			return m.int64Constant(integer.Inner.Int64()), true, nil
		}
		return m.allocConstant(m.allocIntegerConstant(integer.Inner)), true, nil
	case builtin.UnBData:
		datum, err := unwrapData[T](arg)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostOne(&fn, dataExMem(datum)); err != nil {
			return nil, true, err
		}
		bytesData, ok := datum.(*data.ByteString)
		if !ok {
			return nil, true, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "unBData",
				Message: "data is not a bytearray",
			}
		}
		return m.allocConstant(m.allocByteStringConstant(bytesData.Inner)), true, nil
	default:
		return nil, false, nil
	}
}

func (m *Machine[T]) evalBinaryBuiltinFast(
	fn builtin.DefaultFunction,
	leftVal Value[T],
	rightVal Value[T],
) (Value[T], bool, error) {
	if err := m.checkBoundedArgs2(fn, leftVal, rightVal); err != nil {
		return nil, true, err
	}
	if resolved, handled, err := m.evalBinaryIntegerBuiltin(fn, leftVal, rightVal); handled {
		return resolved, true, err
	}

	switch fn {
	case builtin.EqualsByteString:
		arg1, err := unwrapByteString[T](leftVal)
		if err != nil {
			return nil, true, err
		}
		arg2, err := unwrapByteString[T](rightVal)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostTwo(&fn, byteArrayExMem(arg1), byteArrayExMem(arg2)); err != nil {
			return nil, true, err
		}
		return boolConstant(bytes.Equal(arg1, arg2)), true, nil
	case builtin.LessThanByteString:
		arg1, err := unwrapByteString[T](leftVal)
		if err != nil {
			return nil, true, err
		}
		arg2, err := unwrapByteString[T](rightVal)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostTwo(&fn, byteArrayExMem(arg1), byteArrayExMem(arg2)); err != nil {
			return nil, true, err
		}
		return boolConstant(bytes.Compare(arg1, arg2) == -1), true, nil
	case builtin.LessThanEqualsByteString:
		arg1, err := unwrapByteString[T](leftVal)
		if err != nil {
			return nil, true, err
		}
		arg2, err := unwrapByteString[T](rightVal)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostTwo(&fn, byteArrayExMem(arg1), byteArrayExMem(arg2)); err != nil {
			return nil, true, err
		}
		res := bytes.Compare(arg1, arg2)
		return boolConstant(res == -1 || res == 0), true, nil
	case builtin.ChooseUnit:
		if err := unwrapUnit[T](leftVal); err != nil {
			return nil, true, err
		}
		if err := m.CostTwo(&fn, unitExMem(), valueExMem[T](rightVal)); err != nil {
			return nil, true, err
		}
		return rightVal, true, nil
	case builtin.Trace:
		msg, err := unwrapString[T](leftVal)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostTwo(&fn, stringExMem(msg), valueExMem[T](rightVal)); err != nil {
			return nil, true, err
		}
		m.Logs = append(m.Logs, msg)
		return rightVal, true, nil
	case builtin.MkCons:
		return m.mkConsFast(fn, leftVal, rightVal)
	default:
		return nil, false, nil
	}
}

// mkConsFast prepends leftVal to the list rightVal in O(1) amortised: the
// result is always a cons cell sharing the tail (see list_share.go and
// list_lazy.go). The charged budget and every error are those of the
// contiguous implementation: a cell-backed tail takes the same path as the
// contiguous value it stands for.
func (m *Machine[T]) mkConsFast(
	fn builtin.DefaultFunction,
	leftVal Value[T],
	rightVal Value[T],
) (Value[T], bool, error) {
	var mapTail [][2]data.PlutusData
	var mapCell *consCell[[2]data.PlutusData]
	var dataTail []data.PlutusData
	var dataCell *consCell[data.PlutusData]
	isMap, isData := false, false
	switch tail := rightVal.(type) {
	case *dataMapValue[T]:
		isMap, mapTail = true, tail.items
	case *consDataMap[T]:
		isMap, mapCell = true, tail.cell
	case *dataListValue[T]:
		isData, dataTail = true, tail.items
	case *consDataList[T]:
		isData, dataCell = true, tail.cell
	}
	if isMap {
		if err := m.CostTwo(&fn, valueExMem[T](leftVal), valueExMem[T](rightVal)); err != nil {
			return nil, true, err
		}
		first, second, err := splitPairValue(m, leftVal)
		if err != nil {
			return nil, true, err
		}
		dataFirst, err := unwrapData[T](first)
		if err != nil {
			return nil, true, err
		}
		dataSecond, err := unwrapData[T](second)
		if err != nil {
			return nil, true, err
		}
		head := [2]data.PlutusData{dataFirst, dataSecond}
		a := newDataMapAlloc[T]()
		if mapCell != nil {
			consOntoCell(&a.c, head, mapCell)
		} else {
			consOntoSlice(&a.c, head, mapTail)
		}
		return &a.v, true, nil
	}
	if isData {
		if err := m.CostTwo(&fn, valueExMem[T](leftVal), valueExMem[T](rightVal)); err != nil {
			return nil, true, err
		}
		dataHead, err := unwrapData[T](leftVal)
		if err != nil {
			return nil, true, err
		}
		a := newDataListAlloc[T]()
		if dataCell != nil {
			consOntoCell(&a.c, dataHead, dataCell)
		} else {
			consOntoSlice(&a.c, dataHead, dataTail)
		}
		return &a.v, true, nil
	}
	head, err := m.unwrapConsHead(leftVal)
	if err != nil {
		return nil, true, err
	}
	typ := head.Constant.Typ()
	if lazy, ok := rightVal.(*consConstList[T]); ok {
		// Same check and error as unwrapList on the flattened list.
		if typ != nil && !syn.EqualType(typ, lazy.typ) {
			return nil, true, fmt.Errorf("Value not a List of type %T", typ)
		}
		if err := m.CostTwo(&fn, valueExMem[T](head), valueExMem[T](rightVal)); err != nil {
			return nil, true, err
		}
		a := newConstListAlloc[T](typ)
		consOntoCell(&a.c, head.Constant, lazy.cell)
		return &a.v, true, nil
	}
	tail, err := unwrapList[T](typ, rightVal)
	if err != nil {
		return nil, true, err
	}
	if err := m.CostTwo(&fn, valueExMem[T](head), listExMem(tail.List)); err != nil {
		return nil, true, err
	}
	a := newConstListAlloc[T](typ)
	consOntoSlice(&a.c, head.Constant, tail.List)
	return &a.v, true, nil
}

// unwrapConsHead is unwrapConstant for the mkCons head, converting data
// lists through the machine's conversion cache so that consing the same
// (or an extended) data list repeatedly does not re-convert it.
func (m *Machine[T]) unwrapConsHead(value Value[T]) (*Constant, error) {
	if v, ok := value.(*Constant); ok {
		return v, nil
	}
	budget := dischargeWorkBudget{remaining: maxDischargeWork}
	c, ok, err := materializeConstantValueShared[T](value, 0, maxDischargeDepth, &budget, &m.listShare)
	if err != nil {
		return nil, err
	}
	if ok {
		return &Constant{Constant: c}, nil
	}
	return nil, &TypeError{Code: ErrCodeTypeMismatch, Expected: "Constant", Got: fmt.Sprintf("%T", value), Message: "type mismatch"}
}

func (m *Machine[T]) evalTernaryBuiltinFast(
	fn builtin.DefaultFunction,
	firstVal Value[T],
	secondVal Value[T],
	thirdVal Value[T],
) (Value[T], bool, error) {
	if err := m.checkBoundedArgs3(fn, firstVal, secondVal, thirdVal); err != nil {
		return nil, true, err
	}
	switch fn {
	case builtin.IfThenElse:
		cond, err := unwrapBool[T](firstVal)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostThree(&fn, boolExMem(cond), valueExMem[T](secondVal), valueExMem[T](thirdVal)); err != nil {
			return nil, true, err
		}
		if cond {
			return secondVal, true, nil
		}
		return thirdVal, true, nil
	case builtin.ChooseList:
		if dataList, ok := firstVal.(*dataListValue[T]); ok {
			if err := m.CostThree(&fn, valueExMem[T](firstVal), valueExMem[T](secondVal), valueExMem[T](thirdVal)); err != nil {
				return nil, true, err
			}
			if len(dataList.items) == 0 {
				return secondVal, true, nil
			}
			return thirdVal, true, nil
		}
		if dataMap, ok := firstVal.(*dataMapValue[T]); ok {
			if err := m.CostThree(&fn, valueExMem[T](firstVal), valueExMem[T](secondVal), valueExMem[T](thirdVal)); err != nil {
				return nil, true, err
			}
			if len(dataMap.items) == 0 {
				return secondVal, true, nil
			}
			return thirdVal, true, nil
		}
		if lazy, ok := firstVal.(lazyListValue[T]); ok {
			if err := m.CostThree(&fn, valueExMem[T](firstVal), valueExMem[T](secondVal), valueExMem[T](thirdVal)); err != nil {
				return nil, true, err
			}
			if lazy.lazyLen() == 0 {
				return secondVal, true, nil
			}
			return thirdVal, true, nil
		}
		list, err := unwrapList[T](nil, firstVal)
		if err != nil {
			return nil, true, err
		}
		if err := m.CostThree(&fn, listExMem(list.List), valueExMem[T](secondVal), valueExMem[T](thirdVal)); err != nil {
			return nil, true, err
		}
		if len(list.List) == 0 {
			return secondVal, true, nil
		}
		return thirdVal, true, nil
	default:
		return nil, false, nil
	}
}

func addInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func subtractInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func multiplyInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func divideInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func quotientInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func remainderInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func modInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func equalsInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func lessThanInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func lessThanEqualsInteger[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryIntegerBuiltin(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

// ============================================================================
// BYTESTRING OPERATIONS
// Functions: appendByteString, consByteString, sliceByteString,
//            lengthOfByteString, indexByteString, equalsByteString,
//            lessThanByteString, lessThanEqualsByteString
// ============================================================================

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

	var firstByte byte

	switch m.semantics {
	case SemanticsVariantA, SemanticsVariantB, SemanticsVariantD:
		// V1/V2 semantics reduce the first argument modulo 256 (floored) for
		// any integer, including after the Van Rossem protocol upgrade. Use the
		// low byte directly so the post-costing work remains constant even for
		// very large integer inputs.
		firstByte = bigIntMod256Byte(arg1)
	default:
		// consByteString requires the integer to be in the range 0-255 in V3+
		if !arg1.IsInt64() {
			return nil, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "consByteString",
				Message: "int does not fit into a byte",
			}
		}
		intVal := arg1.Int64()
		if intVal < 0 || intVal > 255 {
			return nil, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "consByteString",
				Message: "int does not fit into a byte",
			}
		}
		firstByte = byte(intVal)
	}

	res := append([]byte{firstByte}, arg2...)

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

	// The reference unlifts skip and take as Int in every semantics variant
	// (PlutusCore.Default.Universe readKnownAsInteger): an Integer outside
	// the Int64 range fails the application before it is costed.
	if !arg1.IsInt64() || !arg2.IsInt64() {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "sliceByteString",
			Message: "argument is not within the bounds of Int",
		}
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

	// Convert skip to int and clamp it before deriving take. The fallback for
	// a positive take that does not fit in int64 depends on the remaining
	// bytes after skip, so using an unclamped skip can produce a negative take.
	skip := 0
	if arg1.Sign() > 0 {
		if skip64 := arg1.Int64(); skip64 > int64(len(arg3)) {
			skip = len(arg3)
		} else {
			skip = int(skip64)
		}
	}
	remaining := len(arg3) - skip

	// Convert take to int and clamp it into [0, remaining]. This keeps the
	// final slice bounds in range and avoids skip+take overflowing.
	take := 0
	if arg2.Sign() > 0 {
		if take64 := arg2.Int64(); take64 > int64(remaining) {
			take = remaining
		} else {
			take = int(take64)
		}
	}

	// Slice. res aliases arg3's backing array; this is intentional and matches
	// the reference's zero-copy slicing. Bytestring Constant.Inner must be
	// treated as immutable by all builtins (the bitwise/cons/append builtins
	// already allocate fresh buffers), so the aliasing is safe.
	res := arg3[skip : skip+take]

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
	return m.int64Constant(int64(res)), nil
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
		return nil, &BuiltinError{
			Code:    ErrCodeOutOfBounds,
			Builtin: "indexByteString",
			Message: "byte string out of bounds",
		}
	}

	// Check bounds: index must be in range [0, len(arg1))
	if index < 0 || index > int64(math.MaxInt) || int(index) >= len(arg1) {
		return nil, &BuiltinError{
			Code:    ErrCodeOutOfBounds,
			Builtin: "indexByteString",
			Message: "byte string out of bounds",
		}
	}

	res := int64(arg1[index])
	return m.int64Constant(res), nil
}

func equalsByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryBuiltinFast(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func lessThanByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryBuiltinFast(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func lessThanEqualsByteString[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryBuiltinFast(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

// ============================================================================
// CRYPTOGRAPHIC OPERATIONS - HASHING
// Functions: sha2256, sha3256, blake2B256, blake2B224, keccak256
// ============================================================================

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

// ============================================================================
// CRYPTOGRAPHIC OPERATIONS - SIGNATURE VERIFICATION
// Functions: verifyEd25519Signature, verifyEcdsaSecp256K1Signature,
//            verifySchnorrSecp256K1Signature
// ============================================================================

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

	err = m.CostThreeExMem(
		&b.Func,
		byteArrayExMemValue(publicKey),
		byteArrayExMemValue(message),
		byteArrayExMemValue(signature),
	)
	if err != nil {
		return nil, err
	}

	if len(publicKey) != ed25519.PublicKeySize { // 32 bytes
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEd25519Signature",
			Message: fmt.Sprintf(
				"invalid public key length: got %d, expected 32",
				len(publicKey),
			),
		}
	}

	if len(signature) != ed25519.SignatureSize { // 64 bytes
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEd25519Signature",
			Message: fmt.Sprintf(
				"invalid signature length: got %d, expected 64",
				len(signature),
			),
		}
	}

	var res bool
	if len(message) <= ed25519VerifyMaxCacheMsg {
		key := newEd25519VerifyCacheKey(publicKey, message, signature)
		cached, ok := ed25519VerifyCache.get(&key)
		if ok {
			res = cached
		} else {
			res = ed25519VerifyStrict(publicKey, message, signature)
			ed25519VerifyCache.store(&key, res)
		}
	} else {
		res = ed25519VerifyStrict(publicKey, message, signature)
	}

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

	// Plutus' keccak_256 is legacy Keccak (0x01 padding), not NIST SHA3
	// (0x06 padding), so stdlib crypto/sha3 cannot serve this builtin --
	// it exposes only the NIST variant.
	res := legacykeccak.NewLegacyKeccak256()
	// hash.Hash.Write never returns an error.
	_, _ = res.Write(arg1)

	con := &syn.ByteString{
		Inner: res.Sum(nil),
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEcdsaSecp256k1Signature",
			Message: "invalid public key length",
		}
	}

	if len(signature) != 64 {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEcdsaSecp256k1Signature",
			Message: "invalid signature length",
		}
	}

	if len(message) != 32 {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEcdsaSecp256k1Signature",
			Message: "invalid message length",
		}
	}

	key, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEcdsaSecp256k1Signature",
			Message: fmt.Sprintf("invalid public key: %v", err),
		}
	}

	r := new(btcec.ModNScalar)

	overflow := r.SetByteSlice(signature[0:32])
	if overflow {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEcdsaSecp256k1Signature",
			Message: "invalid signature (r)",
		}
	}

	s := new(btcec.ModNScalar)
	overflow = s.SetByteSlice(signature[32:])
	if overflow {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifyEcdsaSecp256k1Signature",
			Message: "invalid signature (s)",
		}
	}
	if s.IsOverHalfOrder() {
		return boolConstant(false), nil
	}

	sig := ecdsa.NewSignature(r, s)

	res := sig.Verify(message, key)

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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifySchnorrSecp256k1Signature",
			Message: "invalid signature length",
		}
	}

	key, err := schnorr.ParsePubKey(publicKey)
	if err != nil {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "verifySchnorrSecp256k1Signature",
			Message: fmt.Sprintf("invalid public key: %v", err),
		}
	}

	// BIP-340 verification fails, rather than errors, when r is not a field
	// element or s is not a scalar, so both return False. Without these
	// checks r would be carried unnormalized into schnorrVerify's step 9
	// comparison and s would be silently reduced modulo the group order.
	r := new(btcec.FieldVal)
	if overflow := r.SetByteSlice(signature[0:32]); overflow {
		return boolConstant(false), nil
	}

	s := new(btcec.ModNScalar)
	if overflow := s.SetByteSlice(signature[32:]); overflow {
		return boolConstant(false), nil
	}

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

	// Steps 3 and 4.
	//
	// Fail if r >= p or s >= n.
	//
	// Both are rejected by the caller, which is where the signature bytes are
	// parsed. btcec reduces s modulo the group order and leaves r
	// unnormalized on overflow, so neither value can be range-checked here.

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
		return &BuiltinError{
			Code:    ErrCodeValidationFailed,
			Builtin: "schnorrVerify",
			Message: "calculated R point is the point at infinity",
		}
	}

	// Step 8.
	//
	// Fail if R.y is odd
	//
	// Note that R must be in affine coordinates for this check.
	R.ToAffine()
	if R.Y.IsOdd() {
		return &BuiltinError{
			Code:    ErrCodeValidationFailed,
			Builtin: "schnorrVerify",
			Message: "calculated R y-value is odd",
		}
	}

	// Step 9.
	//
	// Verified if R.x == r
	//
	// Note that R must be in affine coordinates for this check.
	if !r.Equals(&R.X) {
		return &BuiltinError{
			Code:    ErrCodeValidationFailed,
			Builtin: "schnorrVerify",
			Message: "calculated R point was not given R",
		}
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

// ============================================================================
// STRING OPERATIONS
// Functions: appendString, equalsString, encodeUtf8, decodeUtf8
// ============================================================================

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

	arg1ExMem, arg2ExMem := stringExMem(arg1), stringExMem(arg2)
	if m.semantics == SemanticsVariantD || m.semantics == SemanticsVariantE {
		arg1ExMem, arg2ExMem = textByteLengthExMem(arg1), textByteLengthExMem(arg2)
	}
	err = m.CostTwo(&b.Func, arg1ExMem, arg2ExMem)
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
	arg1ExMem, arg2ExMem := stringExMem(arg1), stringExMem(arg2)
	if m.semantics == SemanticsVariantD || m.semantics == SemanticsVariantE {
		arg1ExMem, arg2ExMem = textByteLengthExMem(arg1), textByteLengthExMem(arg2)
	}
	err = m.CostTwo(&b.Func, arg1ExMem, arg2ExMem)
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

	arg1ExMem := stringExMem(arg1)
	if m.semantics == SemanticsVariantD || m.semantics == SemanticsVariantE {
		arg1ExMem = textByteLengthExMem(arg1)
	}
	err = m.CostOne(&b.Func, arg1ExMem)
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
		return nil, &BuiltinError{
			Code:    ErrCodeDecodeFailure,
			Builtin: "decodeUtf8",
			Message: "error decoding utf8 bytes",
		}
	}

	res := string(arg1)

	con := &syn.String{
		Inner: res,
	}

	value := &Constant{con}

	return value, nil
}

// ============================================================================
// CONTROL FLOW
// Functions: ifThenElse, chooseUnit, trace
// ============================================================================

func ifThenElse[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalTernaryBuiltinFast(
		b.Func,
		m.argHolder[0],
		m.argHolder[1],
		m.argHolder[2],
	)
	return value, err
}

func chooseUnit[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryBuiltinFast(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func trace[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryBuiltinFast(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

// ============================================================================
// PAIR AND LIST OPERATIONS
// Functions: fstPair, sndPair, chooseList, mkCons, headList, tailList, nullList
// ============================================================================

func fstPair[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func sndPair[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func chooseList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalTernaryBuiltinFast(
		b.Func,
		m.argHolder[0],
		m.argHolder[1],
		m.argHolder[2],
	)
	return value, err
}

func mkCons[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalBinaryBuiltinFast(b.Func, m.argHolder[0], m.argHolder[1])
	return value, err
}

func headList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func tailList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func nullList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

// ============================================================================
// DATA OPERATIONS
// Functions: chooseData, constrData, mapData, listData, iData, bData,
//            unConstrData, unMapData, unListData, unIData, unBData,
//            equalsData, serialiseData, mkPairData, mkNilData, mkNilPairData
// ============================================================================

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
	var valueBranch Value[T]
	if m.version == lang.LanguageVersionV4 {
		valueBranch = m.argHolder[6]
	}

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

	case *data.Value:
		if valueBranch == nil {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "chooseData",
				Message: "the Value Data constructor is unavailable before Plutus V4",
			}
		}
		return valueBranch, nil

	default:
		return nil, &BuiltinError{Code: ErrCodeInvalidArgument, Builtin: "chooseData", Message: "unexpected data variant"}
	}
}

func validateConstrTag(
	semantics SemanticsVariant,
	arg *big.Int,
) (*big.Int, error) {
	if (semantics == SemanticsVariantD || semantics == SemanticsVariantE) &&
		arg.Sign() < 0 {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "constrData",
			Message: "constructor tag must be non-negative",
		}
	}
	if (semantics == SemanticsVariantD || semantics == SemanticsVariantE) &&
		!arg.IsUint64() {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "constrData",
			Message: "constructor tag does not fit in Word64",
		}
	}
	return new(big.Int).Set(arg), nil
}

// flattenListArg returns the contiguous equivalent of a list argument and,
// for a list built by mkCons with constant elements, the buffer holding its
// elements, on which conversions are memoised.
func flattenListArg[T syn.Eval](value Value[T]) (Value[T], *frontBuf[syn.IConstant]) {
	if list, ok := value.(*consConstList[T]); ok {
		flat, buf := list.cell.flatten()
		return &Constant{Constant: &syn.ProtoList{LTyp: list.typ, List: flat}}, buf
	}
	return flattenLazyList[T](value), nil
}

func dataElem(item syn.IConstant, builtinName string) (data.PlutusData, error) {
	itemData, ok := item.(*syn.Data)
	if !ok {
		return nil, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Data",
			Got:      fmt.Sprintf("%T", item),
			Message:  builtinName + ": list element is not Data",
		}
	}
	return itemData.Inner, nil
}

func listDataElem(item syn.IConstant) (data.PlutusData, error) {
	return dataElem(item, "listData")
}

func constrDataElem(item syn.IConstant) (data.PlutusData, error) {
	return dataElem(item, "constrData")
}

func mapDataElem(item syn.IConstant) ([2]data.PlutusData, error) {
	pair, ok := item.(*syn.ProtoPair)
	if !ok {
		return [2]data.PlutusData{}, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Pair",
			Got:      fmt.Sprintf("%T", item),
			Message:  "mapData: list element is not a pair",
		}
	}
	fst, ok := pair.First.(*syn.Data)
	if !ok {
		return [2]data.PlutusData{}, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Data",
			Got:      fmt.Sprintf("%T", pair.First),
			Message:  "mapData: pair key is not Data",
		}
	}
	snd, ok := pair.Second.(*syn.Data)
	if !ok {
		return [2]data.PlutusData{}, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Data",
			Got:      fmt.Sprintf("%T", pair.Second),
			Message:  "mapData: pair value is not Data",
		}
	}
	return [2]data.PlutusData{fst.Inner, snd.Inner}, nil
}

func constrData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	// A cons-cell list must become one contiguous slice; flattening is
	// memoised per cell and O(1) amortised for lists built by chained mkCons.
	listArg, owner := flattenListArg[T](m.argHolder[1])
	if arg2, ok := listArg.(*dataListValue[T]); ok {
		err = m.CostTwo(&b.Func, bigIntExMem(arg1), valueExMem[T](arg2))
		if err != nil {
			return nil, err
		}

		tag, tagErr := validateConstrTag(m.semantics, arg1)
		if tagErr != nil {
			return nil, tagErr
		}

		return m.allocDataValue(&data.Constr{
			Tag:    tag,
			Fields: arg2.items,
		}), nil
	}

	arg2, err := unwrapList[T](&syn.TData{}, listArg)
	if err != nil {
		return nil, err
	}

	err = m.CostTwo(&b.Func, bigIntExMem(arg1), listExMem(arg2.List))
	if err != nil {
		return nil, err
	}

	dataList, err := convertList(&m.listShare.toData, owner, arg2.List, constrDataElem)
	if err != nil {
		return nil, err
	}

	tag, tagErr := validateConstrTag(m.semantics, arg1)
	if tagErr != nil {
		return nil, tagErr
	}

	return m.allocDataValue(&data.Constr{
		Tag:    tag,
		Fields: dataList,
	}), nil
}

func mapData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	listArg, owner := flattenListArg[T](m.argHolder[0])
	if arg1, ok := listArg.(*dataMapValue[T]); ok {
		err := m.CostOne(&b.Func, valueExMem[T](arg1))
		if err != nil {
			return nil, err
		}
		return m.allocDataValue(&data.Map{
			Pairs: arg1.items,
		}), nil
	}

	pairType := &syn.TPair{
		First:  &syn.TData{},
		Second: &syn.TData{},
	}

	arg1, err := unwrapList[T](pairType, listArg)
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, listExMem(arg1.List))
	if err != nil {
		return nil, err
	}

	dataList, err := convertList(&m.listShare.toPairs, owner, arg1.List, mapDataElem)
	if err != nil {
		return nil, err
	}

	return m.allocDataValue(&data.Map{
		Pairs: dataList,
	}), nil
}

func listData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	listArg, owner := flattenListArg[T](m.argHolder[0])
	if arg1, ok := listArg.(*dataListValue[T]); ok {
		err := m.CostOne(&b.Func, valueExMem[T](arg1))
		if err != nil {
			return nil, err
		}
		return m.allocDataValue(&data.List{
			Items: arg1.items,
		}), nil
	}

	arg1, err := unwrapList[T](&syn.TData{}, listArg)
	if err != nil {
		return nil, err
	}

	err = m.CostOne(&b.Func, listExMem(arg1.List))
	if err != nil {
		return nil, err
	}

	dataList, err := convertList(&m.listShare.toData, owner, arg1.List, listDataElem)
	if err != nil {
		return nil, err
	}

	return m.allocDataValue(&data.List{
		Items: dataList,
	}), nil
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

	return m.allocDataValue(&data.Integer{Inner: arg1}), nil
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

	return m.allocDataValue(&data.ByteString{Inner: arg1}), nil
}

func unConstrData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func unMapData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func unListData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func unIData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
}

func unBData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, _, err := m.evalUnaryBuiltinFast(b.Func, m.argHolder[0])
	return value, err
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

	err = m.costEqualsData(&b.Func, arg1, arg2)
	if err != nil {
		return nil, err
	}

	return boolConstant(arg1.Equal(arg2)), nil
}

func serialiseData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	arg1, err := unwrapData[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	err = m.costDataOne(&b.Func, arg1, false)
	if err != nil {
		return nil, err
	}

	// The reference (PlutusCore.Data.encodeData) always writes the
	// canonical encoding: definite maps, and lists and constructor fields
	// definite when empty and indefinite otherwise. Data decoded from a
	// script constant or a ledger argument remembers its original
	// definite/indefinite choices for re-encoding fidelity, so serialise
	// the normalized form.
	encoded, err := data.Encode(data.Normalize(arg1))
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

	return m.allocPairValue(
		m.allocDataValue(arg1),
		m.allocDataValue(arg2),
	), nil
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

	return m.allocDataListValue(nil), nil
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

	return m.allocDataMapValue(nil), nil
}

// ============================================================================
// BLS12-381 CRYPTOGRAPHIC OPERATIONS
// Functions: bls12381G1Add, bls12381G1Neg, bls12381G1ScalarMul, bls12381G1Equal,
//            bls12381G1Compress, bls12381G1Uncompress, bls12381G1HashToGroup,
//            bls12381G2Add, bls12381G2Neg, bls12381G2ScalarMul, bls12381G2Equal,
//            bls12381G2Compress, bls12381G2Uncompress, bls12381G2HashToGroup,
//            bls12381G1MultiScalarMul, bls12381G2MultiScalarMul,
//            bls12381MillerLoop, bls12381MulMlResult, bls12381FinalVerify
// ============================================================================

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

	if err := m.checkBLSScalarMulBound(b.Func, arg1); err != nil {
		return nil, err
	}

	newG1 := new(bls.G1Jac).ScalarMultiplication(
		arg2,
		normalizeBLSScalar(arg1),
	)

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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "bls12_381_G1_uncompress",
			Message: "bytestring is too long",
		}
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "bls12_381_G1_hashToGroup",
			Message: "hash to curve dst too big",
		}
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

	if err := m.checkBLSScalarMulBound(b.Func, arg1); err != nil {
		return nil, err
	}

	newG2 := new(bls.G2Jac).ScalarMultiplication(
		arg2,
		normalizeBLSScalar(arg1),
	)

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
		return nil, m.overdrawTwo(
			&b.Func,
			listChunksOf(len(ints.List)),
			listChunksOf(len(elems.List)),
			err,
		)
	}
	if err := validateBLSMSMScalars(
		m,
		ints.List,
		"bls12_381_G1_multiScalarMul",
	); err != nil {
		return nil, err
	}

	// Compute multi-scalar multiplication: sum_i (n_i * p_i)
	res := new(bls.G1Jac)

	n := min(len(elems.List), len(ints.List))

	for i := range n {
		if err := m.checkBuiltinCancellation(); err != nil {
			return nil, err
		}
		ni, ok := ints.List[i].(*syn.Integer)
		if !ok {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "bls12_381_G1_multiScalarMul",
				Message: "expected integer list",
			}
		}
		pi, ok := elems.List[i].(*syn.Bls12_381G1Element)
		if !ok {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "bls12_381_G1_multiScalarMul",
				Message: "expected g1 element list",
			}
		}

		tmp := new(bls.G1Jac).ScalarMultiplication(
			pi.Inner,
			normalizeBLSScalar(ni.Inner),
		)
		res.AddAssign(tmp)
		if err := m.checkBuiltinCancellation(); err != nil {
			return nil, err
		}
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
		return nil, m.overdrawTwo(
			&b.Func,
			listChunksOf(len(ints.List)),
			listChunksOf(len(elems.List)),
			err,
		)
	}
	if err := validateBLSMSMScalars(
		m,
		ints.List,
		"bls12_381_G2_multiScalarMul",
	); err != nil {
		return nil, err
	}

	// Compute multi-scalar multiplication: sum_i (n_i * p_i)
	res := new(bls.G2Jac)

	n := min(len(elems.List), len(ints.List))

	for i := range n {
		if err := m.checkBuiltinCancellation(); err != nil {
			return nil, err
		}
		ni, ok := ints.List[i].(*syn.Integer)
		if !ok {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "bls12_381_G2_multiScalarMul",
				Message: "expected integer list",
			}
		}
		pi, ok := elems.List[i].(*syn.Bls12_381G2Element)
		if !ok {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "bls12_381_G2_multiScalarMul",
				Message: "expected g2 element list",
			}
		}

		tmp := new(bls.G2Jac).ScalarMultiplication(
			pi.Inner,
			normalizeBLSScalar(ni.Inner),
		)
		res.AddAssign(tmp)
		if err := m.checkBuiltinCancellation(); err != nil {
			return nil, err
		}
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "bls12_381_G2_uncompress",
			Message: "bytestring is too long",
		}
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "bls12_381_G2_hashToGroup",
			Message: "hash to curve dst too big",
		}
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

// ============================================================================
// INTEGER/BYTESTRING CONVERSIONS
// Functions: integerToByteString, byteStringToInteger
// ============================================================================

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

	// The reference charges before it checks the size (the size is costed
	// as NumBytesCostedAsNumWords), so an oversized request fails for lack
	// of budget when its cost overdraws.
	err = m.CostThree(
		&b.Func,
		boolExMem(endianness),
		numBytesAsWordsExMem(size),
		bigIntExMem(input),
	)
	if err != nil {
		return nil, err
	}

	// Convert size to int64
	if !size.IsInt64() {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "integerToByteString",
			Message: "size too large",
		}
	}

	sizeInt64 := size.Int64()

	if sizeInt64 < 0 {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "integerToByteString",
			Message: fmt.Sprintf("negative size %v", sizeInt64),
		}
	}

	sizeUnwrapped := int(sizeInt64)

	if sizeUnwrapped > IntegerToByteStringMaximumOutputLength {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "integerToByteString",
			Message: "size too large",
		}
	}

	if input.Sign() < 0 {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "integerToByteString",
			Message: "negative input (" + integerMetadata(input) + ")",
		}
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

		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "integerToByteString",
			Message: fmt.Sprintf(
				"input requires %d bytes, exceeds max %d",
				required, IntegerToByteStringMaximumOutputLength,
			),
		}
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "integerToByteString",
			Message: fmt.Sprintf(
				"size %d too small, need %d",
				sizeUnwrapped, len(bytes),
			),
		}
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

// ============================================================================
// BITWISE OPERATIONS
// Functions: andByteString, orByteString, xorByteString, complementByteString,
//            readBit, writeBits, replicateByte, shiftByteString,
//            rotateByteString, countSetBits, findFirstSetBit
// ============================================================================

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
	for i, v := range bytes {
		result[i] = v ^ 0xFF
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
		return nil, &BuiltinError{
			Code:    ErrCodeOutOfBounds,
			Builtin: "readBit",
			Message: "empty byte array",
		}
	}

	// Validate bit index
	if bitIndex.Sign() < 0 ||
		bitIndex.Cmp(big.NewInt(int64(len(bytes)*8))) >= 0 {
		return nil, &BuiltinError{
			Code:    ErrCodeOutOfBounds,
			Builtin: "readBit",
			Message: "bit index out of bounds",
		}
	}

	// Convert bit index to int64
	bitIndexInt, ok := bitIndex.Int64(), bitIndex.IsInt64()
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "readBit",
			Message: "bit index too large",
		}
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

// writeBitsMaximumInputLength is the longest input writeBits accepts from
// van Rossem on.
const writeBitsMaximumInputLength = 4096

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
		// The reference streams the index list's size.
		return nil, m.overdrawThree(
			&b.Func,
			singleChunkOf(byteArrayExMem(bytes)()),
			listChunksOf(len(indices.List)),
			singleChunkOf(boolExMem(setBit)()),
			err,
		)
	}

	if (m.semantics == SemanticsVariantD || m.semantics == SemanticsVariantE) &&
		len(bytes) > writeBitsMaximumInputLength {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "writeBits",
			Message: fmt.Sprintf(
				"input too long (maximum is %d bytes)",
				writeBitsMaximumInputLength,
			),
		}
	}

	// Clone bytes to avoid modifying the input
	result := make([]byte, len(bytes))
	copy(result, bytes)

	// Process each bit index
	for _, index := range indices.List {
		// Unwrap integer from list element
		bitIndex, ok := index.(*syn.Integer)
		if !ok {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "writeBits",
				Message: fmt.Sprintf(
					"expected integer in indices list, got %T",
					index,
				),
			}
		}

		// Validate bit index
		if bitIndex.Inner.Sign() < 0 ||
			bitIndex.Inner.Cmp(big.NewInt(int64(len(bytes)*8))) >= 0 {
			return nil, &BuiltinError{
				Code:    ErrCodeOutOfBounds,
				Builtin: "writeBits",
				Message: "bit index out of bounds",
			}
		}

		// Convert bit index to int64
		bitIndexInt, ok := bitIndex.Inner.Int64(), bitIndex.Inner.IsInt64()
		if !ok {
			return nil, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "writeBits",
				Message: "bit index too large",
			}
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

	// The reference charges before it checks the size (costed as
	// NumBytesCostedAsNumWords), so an oversized request fails for lack of
	// budget when its cost overdraws.
	err = m.CostTwo(&b.Func, numBytesAsWordsExMem(size), bigIntExMem(byteVal))
	if err != nil {
		return nil, err
	}

	// Validate size
	if size.Sign() < 0 {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "replicateByte",
			Message: "negative size",
		}
	}
	if !size.IsInt64() {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "replicateByte",
			Message: "size too large",
		}
	}
	sizeInt64 := size.Int64()
	if sizeInt64 > int64(math.MaxInt) {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "replicateByte",
			Message: "size too large",
		}
	}
	sizeInt := int(sizeInt64)

	if sizeInt > IntegerToByteStringMaximumOutputLength {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "replicateByte",
			Message: "size too large",
		}
	}

	if !byteVal.IsInt64() || byteVal.Int64() < 0 || byteVal.Int64() > 255 {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "replicateByte",
			Message: "byte value out of bounds (" + integerMetadata(byteVal) + "; want 0-255)",
		}
	}
	byteUint := byte(byteVal.Int64())

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
	if (m.semantics == SemanticsVariantD || m.semantics == SemanticsVariantE) &&
		!shift.IsInt64() {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "shiftByteString",
			Message: "shift value is outside the Int64 range",
		}
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
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "shiftByteString",
			Message: "shift value too large",
		}
	}
	shift64 := shift.Int64()
	if shift64 > int64(math.MaxInt) || shift64 < int64(math.MinInt) {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "shiftByteString",
			Message: "shift value out of range",
		}
	}
	shiftVal := int(shift64)

	if shiftVal == 0 {
		return m.argHolder[0], nil
	}

	result := shiftBytes(bytes, shiftVal)

	return &Constant{&syn.ByteString{Inner: result}}, nil
}

// shiftBytes shifts bytes by shift bits (MSB0 order), positive = left,
// negative = right, filling with zeros. |shift| must be < 8*len(bytes).
func shiftBytes(in []byte, shift int) []byte {
	out := make([]byte, len(in))
	n := len(in)
	if shift > 0 {
		byteShift := shift / 8
		bitShift := uint(shift % 8)
		for i := 0; i < n-byteShift; i++ {
			v := in[i+byteShift] << bitShift
			if bitShift != 0 && i+byteShift+1 < n {
				v |= in[i+byteShift+1] >> (8 - bitShift)
			}
			out[i] = v
		}
	} else if shift < 0 {
		s := -shift
		byteShift := s / 8
		bitShift := uint(s % 8)
		for i := byteShift; i < n; i++ {
			v := in[i-byteShift] >> bitShift
			if bitShift != 0 && i-byteShift-1 >= 0 {
				v |= in[i-byteShift-1] << (8 - bitShift)
			}
			out[i] = v
		}
	} else {
		copy(out, in)
	}
	return out
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
	if (m.semantics == SemanticsVariantD || m.semantics == SemanticsVariantE) &&
		!shift.IsInt64() {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "rotateByteString",
			Message: "rotation value is outside the Int64 range",
		}
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
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "rotateByteString",
			Message: "normalized shift too large",
		}
	}
	normalizedShift64 := normalizedShift.Int64()
	if normalizedShift64 > int64(math.MaxInt) ||
		normalizedShift64 < int64(math.MinInt) {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "rotateByteString",
			Message: "normalized shift out of range",
		}
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

	return m.int64Constant(int64(count)), nil
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

	// No bits set
	return m.int64Constant(int64(bitIndex)), nil
}

// ============================================================================
// ADDITIONAL CRYPTOGRAPHIC OPERATIONS
// Functions: ripemd160, expModInteger
// ============================================================================

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

// expModBoundBits bounds the expModInteger arguments: the modulus must be
// at most 2^8191-1 and base and exponent within [-(2^8191), 2^8191-1].
const expModBoundBits = 8191

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

	// The checks follow PlutusCore.Crypto.ExpMod.expMod in order: the
	// modulus must lie in [1, 2^8191-1], a modulus of 1 gives 0 whatever the
	// other arguments, zero to a negative power fails, and only then must
	// base and exponent lie in [-(2^8191), 2^8191-1].
	if mm.Sign() <= 0 || mm.BitLen() > expModBoundBits {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "expModInteger",
			Message: "invalid modulus",
		}
	}

	// Special case: modulus is 1
	if mm.Cmp(big.NewInt(1)) == 0 {
		return &Constant{&syn.Integer{Inner: big.NewInt(0)}}, nil
	}

	if bb.Sign() == 0 && e.Sign() < 0 {
		// 0^negative is undefined
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "expModInteger",
			Message: fmt.Sprintf("0^negative exponent (%s) is undefined", integerMetadata(e)),
		}
	}

	if !inSignedBitRange(bb, expModBoundBits) || !inSignedBitRange(e, expModBoundBits) {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "expModInteger",
			Message: "base or exponent out of bounds",
		}
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

	default:
		// Negative exponent: need to compute modular inverse
		// First, reduce the base modulo mm to handle negative bases correctly
		reducedBase := new(big.Int)
		reducedBase.Mod(bb, mm)

		// Check if the reduced base and modulus are coprime
		gcd := new(big.Int)
		gcd.GCD(nil, nil, reducedBase, mm)
		if gcd.Cmp(big.NewInt(1)) != 0 {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "expModInteger",
				Message: fmt.Sprintf(
					"base (%s) is not invertible modulo modulus (%s) (gcd = %s)",
					integerMetadata(bb),
					integerMetadata(mm),
					integerMetadata(gcd),
				),
			}
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

// ============================================================================
// LIST EXTENSION OPERATIONS
// Functions: dropList
// ============================================================================

// costDropList charges dropList in one amount, as the reference does: its
// CPU cost is linear in |n| (IntegerCostedLiterally, saturated to the int64
// range like the reference's SatInt costing arithmetic), its memory cost is
// the memory model applied to the arguments.
func (m *Machine[T]) costDropList(
	b *Builtin[T],
	nVal *big.Int,
	listMem func() ExMem,
) error {
	model := m.costs.builtinCosts[b.Func]
	cpuModel, ok := model.cpu.(*DropListCost)
	if !ok {
		return m.CostTwo(&b.Func, listMem, bigIntExMem(nVal))
	}
	memModel, ok := model.mem.(TwoArgument)
	if !ok {
		return internalError("dropList memory costing is not a two-argument model")
	}
	absN := int64(math.MaxInt64)
	if nVal.IsInt64() && nVal.Int64() != math.MinInt64 {
		absN = nVal.Int64()
		if absN < 0 {
			absN = -absN
		}
	}
	nMem := bigIntExMem(nVal)()
	listSize := ExMem(0)
	if !memModel.HasConstants()[0] {
		listSize = listMem()
	}
	return m.spendBudget(ExBudget{
		Cpu: cpuModel.Cost(ExMem(absN)),
		Mem: memModel.CostTwo(listSize, nMem),
	})
}

// dropListStart charges the dropList budget for a list with the given
// length and ExMem size, then computes the start index after dropping n
// elements. The costing is shared by the materialized (ProtoList) path and
// the data-backed fast paths so that both charge byte-for-byte identical
// budgets.
func dropListStart[T syn.Eval](
	m *Machine[T],
	b *Builtin[T],
	nVal *big.Int,
	listLen int,
	listMem func() ExMem,
) (int, error) {
	if err := m.costDropList(b, nVal, listMem); err != nil {
		return 0, err
	}

	// Negative counts are treated as zero (return list unchanged)
	if nVal.Sign() < 0 {
		nVal = big.NewInt(0)
	}

	// Convert n to int if possible, otherwise drop everything
	start := listLen
	if nVal.IsInt64() {
		nVal64 := nVal.Int64()
		if nVal64 > int64(math.MaxInt) {
			return 0, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "dropList",
				Message: "n too large",
			}
		}
		ni := int(nVal64)
		start = min(ni, listLen)
	}

	return start, nil
}

func dropList[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: (con integer n) (con (list t) xs)
	nVal, err := unwrapInteger[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	// Fast paths: data-backed lists/maps are re-sliced without materializing
	// the whole list into a ProtoList. valueExMem (toExMem) of these values
	// is defined to equal listExMem of their materialized equivalent, so the
	// charged budget is identical to the materialized path (the same
	// mechanism headList/tailList/nullList rely on).
	switch v := m.argHolder[1].(type) {
	case *dataListValue[T]:
		start, err := dropListStart(m, b, nVal, len(v.items), valueExMem[T](v))
		if err != nil {
			return nil, err
		}
		return m.allocDataListValue(v.items[start:]), nil
	case *dataMapValue[T]:
		start, err := dropListStart(m, b, nVal, len(v.items), valueExMem[T](v))
		if err != nil {
			return nil, err
		}
		return m.allocDataMapValue(v.items[start:]), nil
	case lazyListValue[T]:
		// Walks at most start <= |n| cells, which the per-element charge
		// covers.
		start, err := dropListStart(m, b, nVal, v.lazyLen(), valueExMem[T](v))
		if err != nil {
			return nil, err
		}
		return v.lazyDrop(m, start), nil
	}

	lst, err := unwrapList[T](nil, m.argHolder[1])
	if err != nil {
		return nil, err
	}

	start, err := dropListStart(
		m,
		b,
		nVal,
		len(lst.List),
		func() ExMem { return listExMem(lst.List)() },
	)
	if err != nil {
		return nil, err
	}

	// Build resulting list
	// Lists are immutable, so share the retained tail instead of copying it.
	newList := lst.List[start:len(lst.List):len(lst.List)]

	return &Constant{&syn.ProtoList{LTyp: lst.LTyp, List: newList}}, nil
}

// ============================================================================
// ARRAY OPERATIONS (V4)
// Functions: lengthOfArray, listToArray, indexArray, multiIndexArray
// ============================================================================

func unwrapArray[T syn.Eval](value Value[T]) (syn.Typ, []syn.IConstant, error) {
	constant, ok := value.(*Constant)
	if !ok {
		return nil, nil, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Array",
			Got:      fmt.Sprintf("%T", value),
			Message:  "type mismatch",
		}
	}

	switch array := constant.Constant.(type) {
	case *syn.ProtoArray:
		return array.ATyp, array.Array, nil
	default:
		return nil, nil, &TypeError{
			Code:     ErrCodeTypeMismatch,
			Expected: "Array",
			Got:      fmt.Sprintf("%T", constant.Constant),
			Message:  "type mismatch",
		}
	}
}

func lengthOfArray[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	_, items, err := unwrapArray[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}
	if err := m.CostOne(&b.Func, func() ExMem { return listExMem(items)() }); err != nil {
		return nil, err
	}
	return &Constant{&syn.Integer{Inner: big.NewInt(int64(len(items)))}}, nil
}

func listToArray[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	lst, err := unwrapList[T](nil, m.argHolder[0])
	if err != nil {
		return nil, err
	}
	// Charge based on list length rather than full constant ExMem to match cost model intent
	if err := m.CostOne(&b.Func, func() ExMem { return listLengthExMem(lst.List)() }); err != nil {
		return nil, m.overdrawOne(&b.Func, listChunksOf(len(lst.List)), err)
	}
	return &Constant{&syn.ProtoArray{ATyp: lst.LTyp, Array: lst.List}}, nil
}

func indexArray[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: (con (array t) arr) (con integer idx)
	_, items, err := unwrapArray[T](m.argHolder[0])
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
		func() ExMem { return listExMem(items)() },
		bigIntExMem(idx),
	)
	if err != nil {
		return nil, err
	}

	if idx.Sign() < 0 {
		return nil, &BuiltinError{
			Code:    ErrCodeOutOfBounds,
			Builtin: "indexArray",
			Message: "negative index",
		}
	}

	if !idx.IsInt64() {
		return nil, &BuiltinError{
			Code:    ErrCodeOverflow,
			Builtin: "indexArray",
			Message: "index too large",
		}
	}

	idx64 := idx.Int64()
	if idx64 > int64(math.MaxInt) {
		return nil, &BuiltinError{
			Code:    ErrCodeOutOfBounds,
			Builtin: "indexArray",
			Message: "index out of range",
		}
	}
	i := int(idx64)
	if i >= len(items) {
		return nil, &BuiltinError{
			Code:    ErrCodeOutOfBounds,
			Builtin: "indexArray",
			Message: fmt.Sprintf("index %d out of bounds for array of length %d", i, len(items)),
		}
	}

	return &Constant{items[i]}, nil
}

const multiIndexArrayMaximumIndexCount = 1024

func multiIndexArray[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: (con (array t) arr) (con (list integer) indices)
	arrayType, items, err := unwrapArray[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}
	indices, err := unwrapList[T](&syn.TInteger{}, m.argHolder[1])
	if err != nil {
		return nil, err
	}

	if err := m.CostTwo(
		&b.Func,
		func() ExMem { return listExMem(items)() },
		func() ExMem { return listLengthExMem(indices.List)() },
	); err != nil {
		return nil, err
	}

	result := make([]syn.IConstant, 0, len(indices.List))
	for i, indexValue := range indices.List {
		if i >= multiIndexArrayMaximumIndexCount {
			return nil, &BuiltinError{
				Code:    ErrCodeOutOfBounds,
				Builtin: "multiIndexArray",
				Message: fmt.Sprintf("too many indices (maximum is %d)", multiIndexArrayMaximumIndexCount),
			}
		}

		index, ok := indexValue.(*syn.Integer)
		if !ok {
			return nil, &TypeError{
				Code:     ErrCodeTypeMismatch,
				Expected: "Integer",
				Got:      fmt.Sprintf("%T", indexValue),
				Message:  "type mismatch",
			}
		}
		if index.Inner.Sign() < 0 {
			return nil, &BuiltinError{
				Code:    ErrCodeOutOfBounds,
				Builtin: "multiIndexArray",
				Message: "negative index",
			}
		}
		if !index.Inner.IsInt64() {
			return nil, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "multiIndexArray",
				Message: "index too large",
			}
		}

		index64 := index.Inner.Int64()
		if index64 > int64(math.MaxInt) || int(index64) >= len(items) {
			return nil, &BuiltinError{
				Code:    ErrCodeOutOfBounds,
				Builtin: "multiIndexArray",
				Message: fmt.Sprintf("index %d out of bounds for array of length %d", index64, len(items)),
			}
		}
		result = append(result, items[int(index64)])
	}

	return &Constant{&syn.ProtoList{LTyp: arrayType, List: result}}, nil
}

// ============================================================================
// VALUE OPERATIONS (V4)
// Functions: insertCoin, lookupCoin, scaleValue, unionValue, valueContains
// Helpers: valueShape, valueConstructionError
// ============================================================================

func valueShape(
	constant syn.IConstant,
) (*syn.Value, int, int, int, bool) {
	value, ok := constant.(*syn.Value)
	if !ok {
		return nil, 0, 0, 0, false
	}
	total, outer, maxInner, ok := value.Sizes()
	return value, total, outer, maxInner, ok
}

// policies returns the policy IDs from a canonical Value. Value constants are
// validated in ascending policy order during decoding, matching the upstream
// ordered-map representation.
func policies[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "policies",
			Message: "expected value constant",
		}
	}
	valueConstant, _, outer, _, ok := valueShape(value.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "policies",
			Message: "expected value constant",
		}
	}
	if err := m.CostOne(&b.Func, func() ExMem { return ExMem(outer) }); err != nil {
		return nil, err
	}
	policyIDs, ok := valueConstant.PolicyIDs()
	if !ok {
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "invalid value policy entries",
		}
	}
	result := make([]syn.IConstant, len(policyIDs))
	for i, policyID := range policyIDs {
		result[i] = &syn.ByteString{Inner: policyID}
	}
	return &Constant{&syn.ProtoList{LTyp: &syn.TByteString{}, List: result}}, nil
}

func assetCount[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	value, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "assetCount",
			Message: "expected value constant",
		}
	}
	_, count, _, _, ok := valueShape(value.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "assetCount",
			Message: "expected value constant",
		}
	}
	if err := m.CostOne(&b.Func, func() ExMem {
		return ExMem(count)
	}); err != nil {
		return nil, err
	}
	return &Constant{&syn.Integer{Inner: big.NewInt(int64(count))}}, nil
}

// valueConstructionError reports a failure to build a Value as an internal
// error, but passes a cancellation through so callers can still match it with
// errors.Is.
func valueConstructionError(what string, err error) error {
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &InternalError{
		Code:    ErrCodeInternalError,
		Message: what + ": " + err.Error(),
	}
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "insertCoin",
			Message: "expected value constant",
		}
	}

	valueConstant, _, outer, maxInner, ok := valueShape(valConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "insertCoin",
			Message: "expected value constant",
		}
	}
	// Spend budget for insertCoin (4 args: policy, token, amount, value)
	// The cost model is linear_in_u where u is the Value's max depth, since
	// inserting walks the outer map then the token map of one policy.
	if err := m.CostFour(&b.Func,
		byteArrayExMem(policyBs),
		byteArrayExMem(tokenBs),
		bigIntExMem(amt),
		func() ExMem {
			return ExMem(bits.Len(uint(outer)) + bits.Len(uint(maxInner)))
		},
	); err != nil {
		return nil, err
	}
	// Enforce key length constraints (<= 32 bytes) per builtin semantics, except
	// allow oversize keys when inserting zero (effectively removing)
	if len(policyBs) > 32 || len(tokenBs) > 32 {
		if amt.Sign() != 0 {
			return nil, &BuiltinError{
				Code:    ErrCodeInvalidArgument,
				Builtin: "insertCoin",
				Message: "key too long",
			}
		}
	}

	// Amount must be within 127-bit bounds
	limit := new(big.Int).Lsh(big.NewInt(1), 127)           // 2^127
	limitMinusOne := new(big.Int).Sub(limit, big.NewInt(1)) // 2^127 - 1
	negLimit := new(big.Int).Neg(limit)                     // -2^127
	if amt.Sign() >= 0 {
		if amt.Cmp(limitMinusOne) > 0 {
			return nil, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "insertCoin",
				Message: "amount out of range",
			}
		}
	} else {
		if amt.Cmp(negLimit) < 0 {
			return nil, &BuiltinError{Code: ErrCodeOverflow, Builtin: "insertCoin", Message: "amount out of range"}
		}
	}

	if err := m.checkBuiltinCancellation(); err != nil {
		return nil, err
	}
	// Binary search plus copy-on-write of the touched policy only; the input
	// Value (possibly a shared constant) is never modified.
	result, err := valueConstant.Insert(policyBs, tokenBs, amt)
	if err != nil {
		return nil, valueConstructionError("construct inserted value", err)
	}
	return &Constant{result}, nil
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "lookupCoin",
			Message: "expected value constant",
		}
	}
	valueConstant, _, outer, maxInner, ok := valueShape(valConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "lookupCoin",
			Message: "expected value constant",
		}
	}
	// Spend budget for lookupCoin (3 args: policy, token, value)
	// Cost model is linear_in_z where z is the Value's max depth, for the
	// same two-level map lookup insertCoin is costed on.
	if err := m.CostThree(&b.Func,
		byteArrayExMem(policyBs),
		byteArrayExMem(tokenBs),
		func() ExMem {
			return ExMem(bits.Len(uint(outer)) + bits.Len(uint(maxInner)))
		},
	); err != nil {
		return nil, err
	}
	if amount, ok := valueConstant.LookupCoin(policyBs, tokenBs); ok {
		return &Constant{&syn.Integer{Inner: amount}}, nil
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
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "scaleValue",
			Message: "expected value constant",
		}
	}
	valueConstant, total, _, _, ok := valueShape(valConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "scaleValue",
			Message: "expected value constant",
		}
	}
	// Spend budget for scaleValue (2 args: factor, value)
	// Cost model uses linear_in_y where y is the Value's total size, since
	// scaling touches every (policy, token) pair.
	if err := m.CostTwo(&b.Func,
		bigIntExMem(factor),
		func() ExMem { return ExMem(total) },
	); err != nil {
		return nil, err
	}
	if err := m.checkBuiltinCancellation(); err != nil {
		return nil, err
	}
	result, err := valueConstant.Scale(factor, m.checkBuiltinCancellation)
	if err != nil {
		if errors.Is(err, syn.ErrValueOverflow) {
			return nil, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "scaleValue",
				Message: "amount out of range",
			}
		}
		return nil, valueConstructionError("construct scaled value", err)
	}
	return &Constant{result}, nil
}

func unionValue[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	aConst, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "unionValue",
			Message: "expected value constant for a",
		}
	}
	bConst, ok := m.argHolder[1].(*Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "unionValue",
			Message: "expected value constant for b",
		}
	}
	aValue, aTotal, _, _, ok := valueShape(aConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "unionValue",
			Message: "expected value constant for a",
		}
	}
	bValue, bTotal, _, _, ok := valueShape(bConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "unionValue",
			Message: "expected value constant for b",
		}
	}
	// Spend budget for unionValue (2 args: value a, value b)
	// Cost model uses with_interaction_in_x_and_y, where x and y are each
	// argument's total size, with c11 charging the x*y interaction term.
	if err := m.CostTwo(&b.Func,
		func() ExMem { return ExMem(aTotal) },
		func() ExMem { return ExMem(bTotal) },
	); err != nil {
		return nil, err
	}
	if err := m.checkBuiltinCancellation(); err != nil {
		return nil, err
	}
	result, err := syn.UnionValue(aValue, bValue, m.checkBuiltinCancellation)
	if err != nil {
		if errors.Is(err, syn.ErrValueOverflow) {
			return nil, &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: "unionValue",
				Message: "amount out of range",
			}
		}
		return nil, valueConstructionError("construct union value", err)
	}
	return &Constant{result}, nil
}

func valueContains[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)
	// Args: value, requiredValue
	valConst, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueContains",
			Message: "expected value constant",
		}
	}
	reqConst, ok := m.argHolder[1].(*Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueContains",
			Message: "expected required value constant",
		}
	}
	valValue, valTotal, _, _, ok := valueShape(valConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueContains",
			Message: "expected value constant",
		}
	}
	reqValue, reqTotal, _, _, ok := valueShape(reqConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueContains",
			Message: "expected required value constant",
		}
	}
	// Spend budget for valueContains (2 args: value, required value)
	// Cost model uses const_above_diagonal with linear_in_x_and_y (token count)
	if err := m.CostTwo(&b.Func,
		func() ExMem { return ExMem(valTotal) },
		func() ExMem { return ExMem(reqTotal) },
	); err != nil {
		return nil, err
	}
	if err := m.checkBuiltinCancellation(); err != nil {
		return nil, err
	}
	// Neither value nor required value may contain negative amounts. The
	// check is answered from cached metadata.
	if reqValue.HasNegative() {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueContains",
			Message: "required value contains negative amount",
		}
	}
	if valValue.HasNegative() {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueContains",
			Message: "value contains negative amount",
		}
	}
	return &Constant{&syn.Bool{Inner: valValue.Contains(reqValue)}}, nil
}

// valueData converts a Value to a Data Map.
// The Value format is: [(policy_bs, [(token_bs, amount_int), ...]), ...]
// The Data format is: Map [(B policy, Map [(B token, I amount), ...]), ...]
func valueData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	valConst, ok := m.argHolder[0].(*Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueData",
			Message: "expected value constant",
		}
	}

	valueConstant, total, _, _, ok := valueShape(valConst.Constant)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueData",
			Message: "expected value constant",
		}
	}

	// Spend budget for valueData (1 arg: value). The reference measures the
	// argument as ValueTotalSize (Value.totalSize) for its linear_in_x model.
	if err := m.CostOne(&b.Func, func() ExMem {
		return ExMem(total)
	}); err != nil {
		return nil, err
	}
	if total > valueDataMaxSize {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "valueData",
			Message: "maximum input size (40000) exceeded",
		}
	}
	valueItems := valueConstant.Entries()

	// Convert value to data map
	entries := make([][2]data.PlutusData, 0, len(valueItems))
	for _, entry := range valueItems {
		pair, ok := entry.(*syn.ProtoPair)
		if !ok {
			continue
		}
		policyBs, ok := pair.First.(*syn.ByteString)
		if !ok {
			continue
		}

		tokenList, ok := pair.Second.(*syn.ProtoList)
		if !ok {
			continue
		}

		// Build inner map for tokens
		tokenEntries := make([][2]data.PlutusData, 0, len(tokenList.List))
		for _, tk := range tokenList.List {
			tkp, ok := tk.(*syn.ProtoPair)
			if !ok {
				continue
			}
			tbs, ok1 := tkp.First.(*syn.ByteString)
			amt, ok2 := tkp.Second.(*syn.Integer)
			if ok1 && ok2 {
				tokenEntries = append(tokenEntries, [2]data.PlutusData{
					&data.ByteString{Inner: tbs.Inner},
					&data.Integer{Inner: new(big.Int).Set(amt.Inner)},
				})
			}
		}

		entries = append(entries, [2]data.PlutusData{
			&data.ByteString{Inner: policyBs.Inner},
			&data.Map{Pairs: tokenEntries},
		})
	}

	return &Constant{&syn.Data{Inner: &data.Map{Pairs: entries}}}, nil
}

// unValueData converts a Data Map to a Value.
// The Data format is: Map [(B policy, Map [(B token, I amount), ...]), ...]
// The Value format is: [(policy_bs, [(token_bs, amount_int), ...]), ...]
// The input must be well-formed: keys in ascending order, no duplicates,
// quantities within [-2^127, 2^127-1], no zero quantities.
func unValueData[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
	b.Args.Extract(&m.argHolder, b.ArgCount)

	d, err := unwrapData[T](m.argHolder[0])
	if err != nil {
		return nil, err
	}

	// Spend budget for unValueData (1 arg: data)
	// Cost model uses quadratic_in_x with node count
	if err := m.costDataOne(&b.Func, d, true); err != nil {
		return nil, err
	}

	// Must be a Map
	dmap, ok := d.(*data.Map)
	if !ok {
		return nil, &BuiltinError{
			Code:    ErrCodeInvalidArgument,
			Builtin: "unValueData",
			Message: "expected data map",
		}
	}

	if err := (data.Value{Inner: dmap}).Validate(); err != nil {
		code := ErrCodeInvalidArgument
		if errors.Is(err, data.ErrValueQuantityRange) {
			code = ErrCodeOverflow
		}
		return nil, &BuiltinError{
			Code:    code,
			Builtin: "unValueData",
			Message: err.Error(),
		}
	}

	// Validate guarantees every key is a bytestring and every quantity an integer.
	result := make([]syn.IConstant, 0, len(dmap.Pairs))
	for _, pair := range dmap.Pairs {
		policyData, _ := pair[0].(*data.ByteString)
		tokensData, _ := pair[1].(*data.Map)
		tokenPairs := make([]syn.IConstant, 0, len(tokensData.Pairs))
		for _, tkPair := range tokensData.Pairs {
			tokenData, _ := tkPair[0].(*data.ByteString)
			amtData, _ := tkPair[1].(*data.Integer)
			tokenPairs = append(tokenPairs, &syn.ProtoPair{
				FstType: &syn.TByteString{},
				SndType: &syn.TInteger{},
				First:   &syn.ByteString{Inner: tokenData.Inner},
				Second:  &syn.Integer{Inner: new(big.Int).Set(amtData.Inner)},
			})
		}

		result = append(result, &syn.ProtoPair{
			FstType: &syn.TByteString{},
			SndType: &syn.TList{
				Typ: &syn.TPair{
					First:  &syn.TByteString{},
					Second: &syn.TInteger{},
				},
			},
			First: &syn.ByteString{Inner: policyData.Inner},
			Second: &syn.ProtoList{
				LTyp: &syn.TPair{
					First:  &syn.TByteString{},
					Second: &syn.TInteger{},
				},
				List: tokenPairs,
			},
		})
	}

	value, err := syn.NewValue(result)
	if err != nil {
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "construct decoded value: " + err.Error(),
		}
	}
	return &Constant{value}, nil
}
