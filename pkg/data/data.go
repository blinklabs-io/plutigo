package data

import (
	"fmt"
	"math/big"
)

type PlutusData interface {
	isPlutusData()

	fmt.Stringer
}

// Constr

type Constr struct {
	Tag    uint
	Fields []PlutusData
}

func (Constr) isPlutusData() {}

func (c Constr) String() string {
	return fmt.Sprintf("Constr{tag: %d, fields: %v}", c.Tag, c.Fields)
}

// NewConstr creates a new Constr variant.
func NewConstr(tag uint, fields []PlutusData) PlutusData {
	return &Constr{tag, fields}
}

// Map

type Map struct {
	Pairs [][2]PlutusData // Each pair is [key, value]
}

func (Map) isPlutusData() {}

func (m Map) String() string {
	return fmt.Sprintf("Map%v", m.Pairs)
}

// NewMap creates a new Map variant.
func NewMap(pairs [][2]PlutusData) PlutusData {
	return &Map{pairs}
}

// Integer

type Integer struct {
	Inner *big.Int
}

func (Integer) isPlutusData() {}

func (i Integer) String() string {
	return fmt.Sprintf("Integer(%s)", i.Inner.String())
}

// NewInteger creates a new Integer variant.
func NewInteger(value *big.Int) PlutusData {
	return &Integer{value}
}

// ByteString

type ByteString struct {
	Inner []byte
}

func (ByteString) isPlutusData() {}

func (b ByteString) String() string {
	return fmt.Sprintf("ByteString(%x)", b.Inner)
}

// NewByteString creates a new ByteString variant.
func NewByteString(value []byte) PlutusData {
	return &ByteString{value}
}

// List

type List struct {
	Items []PlutusData
}

func (List) isPlutusData() {}

func (l List) String() string {
	return fmt.Sprintf("List%v", l.Items)
}

// NewList creates a new List variant.
func NewList(items []PlutusData) PlutusData {
	return &List{items}
}
