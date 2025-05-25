package syn

import (
	"math/big"

	"github.com/blinklabs-io/plutigo/pkg/data"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type IConstant interface {
	isConstant()

	Typ() Typ
}

// (con integer 1)
type Integer struct {
	Inner *big.Int
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
	return &TBls12_381G1Element{}
}

type Bls12_381MlResult struct {
	Inner *bls.GT
}

func (Bls12_381MlResult) isConstant() {}

func (Bls12_381MlResult) Typ() Typ {
	return &TBls12_381MlResult{}
}
