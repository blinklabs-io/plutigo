package syn

import (
	"fmt"
	"math/big"
)

type IConstant interface {
	fmt.Stringer
	isConstant()
}

// (con integer 1)
type Integer struct {
	Inner *big.Int
}

func (Integer) isConstant() {}

// (con bytestring #aaBB)
type ByteString struct {
	Inner []uint8
}

func (ByteString) isConstant() {}

// (con string "hello world")
type String struct {
	Inner string
}

func (String) isConstant() {}

// (con unit ())
type Unit struct{}

func (Unit) isConstant() {}

// (con bool True)
type Bool struct {
	Inner bool
}

func (Bool) isConstant() {}

type ProtoList struct{}
