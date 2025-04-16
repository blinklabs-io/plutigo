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
func (v Integer) String() string {
	return "TODO"
}

// (con bytestring #aaBB)
type ByteString struct {
	Inner []uint8
}

func (ByteString) isConstant() {}
func (v ByteString) String() string {
	return "TODO"
}

// (con string "hello world")
type String struct {
	Inner string
}

func (String) isConstant() {}
func (v String) String() string {
	return "TODO"
}

// (con unit ())
type Unit struct{}

func (Unit) isConstant() {}
func (v Unit) String() string {
	return "TODO"
}

// (con bool True)
type Bool struct {
	Inner bool
}

func (Bool) isConstant() {}
func (v Bool) String() string {
	return "TODO"
}

type ProtoList struct {
}
