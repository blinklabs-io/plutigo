package syn

import (
	"math/big"

	"github.com/blinklabs-io/plutigo/data"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type IConstant interface {
	isConstant()

	Typ() Typ
}

// (con integer 1)
type Integer struct {
	Inner       *big.Int
	cachedExMem int64
	cachedInt64 int64
	cachedMeta  uint8
}

const (
	integerMetaCached uint8 = 1 << iota
	integerMetaInt64
)

func newInteger(inner *big.Int) *Integer {
	ret := &Integer{}
	ret.SetInner(inner)
	return ret
}

// SetInner replaces the integer payload and refreshes the cached metadata used
// by the evaluator hot path.
func (i *Integer) SetInner(inner *big.Int) {
	i.Inner = inner
	i.cachedExMem = integerExMemWords(inner)
	i.cachedInt64 = 0
	i.cachedMeta = integerMetaCached
	if inner != nil && inner.IsInt64() {
		i.cachedMeta |= integerMetaInt64
		i.cachedInt64 = inner.Int64()
	}
}

func (i *Integer) CachedInt64() (int64, bool) {
	if i != nil && i.cachedMeta&integerMetaCached != 0 {
		return i.cachedInt64, i.cachedMeta&integerMetaInt64 != 0
	}
	if i == nil || i.Inner == nil || !i.Inner.IsInt64() {
		return 0, false
	}
	return i.Inner.Int64(), true
}

func (i *Integer) ExMemWords() int64 {
	if i != nil && i.cachedMeta&integerMetaCached != 0 {
		return i.cachedExMem
	}
	if i == nil {
		return integerExMemWords(nil)
	}
	return integerExMemWords(i.Inner)
}

func integerExMemWords(inner *big.Int) int64 {
	if inner == nil || inner.Sign() == 0 {
		return 1
	}
	if inner.IsInt64() || inner.IsUint64() {
		return 1
	}
	return int64((inner.BitLen()-1)/64 + 1)
}

func (Integer) isConstant() {}

func (i Integer) Typ() Typ {
	return &TInteger{}
}

// (con bytestring #aaBB)
type ByteString struct {
	Inner []byte
}

func (ByteString) isConstant() {}

func (bs ByteString) Typ() Typ {
	return &TByteString{}
}

// (con string "hello world")
type String struct {
	Inner string
}

func (String) isConstant() {}

func (s String) Typ() Typ {
	return &TString{}
}

// (con unit ())
type Unit struct{}

func (Unit) isConstant() {}

func (u Unit) Typ() Typ {
	return &TUnit{}
}

// (con bool True)
type Bool struct {
	Inner bool
}

func (Bool) isConstant() {}

func (b Bool) Typ() Typ {
	return &TBool{}
}

type ProtoList struct {
	LTyp Typ
	List []IConstant
}

func (ProtoList) isConstant() {}

func (pl ProtoList) Typ() Typ {
	return &TList{Typ: pl.LTyp}
}

type ProtoPair struct {
	FstType Typ
	SndType Typ
	First   IConstant
	Second  IConstant
}

func (ProtoPair) isConstant() {}

func (pp ProtoPair) Typ() Typ {
	return &TPair{First: pp.FstType, Second: pp.SndType}
}

type Data struct {
	Inner data.PlutusData
}

func (Data) isConstant() {}

func (Data) Typ() Typ {
	return &TData{}
}

type Bls12_381G1Element struct {
	Inner *bls.G1Jac
}

func (Bls12_381G1Element) isConstant() {}

func (Bls12_381G1Element) Typ() Typ {
	return &TBls12_381G1Element{}
}

type Bls12_381G2Element struct {
	Inner *bls.G2Jac
}

func (Bls12_381G2Element) isConstant() {}

func (Bls12_381G2Element) Typ() Typ {
	return &TBls12_381G2Element{}
}

type Bls12_381MlResult struct {
	Inner *bls.GT
}

func (Bls12_381MlResult) isConstant() {}

func (Bls12_381MlResult) Typ() Typ {
	return &TBls12_381MlResult{}
}
