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
	return fmt.Sprintf("Integer: %v", v.Inner)
}

// (con bytestring #aaBB)
type ByteString struct {
	Inner []uint8
}

func (ByteString) isConstant() {}
func (v ByteString) String() string {
	return fmt.Sprintf("ByteString: %v", v.Inner)
}

// (con string "hello world")
type String struct {
	Inner string
}

func (String) isConstant() {}
func (v String) String() string {
	return fmt.Sprintf("String: %v", v.Inner)
}

// (con unit ())
type Unit struct{}

func (Unit) isConstant() {}
func (v Unit) String() string {
	return fmt.Sprintf("Unit")
}

// (con bool True)
type Bool struct {
	Inner bool
}

func (Bool) isConstant() {}
func (v Bool) String() string {
	return fmt.Sprintf("Bool: %v", v.Inner)
}

type ProtoList struct {
}
