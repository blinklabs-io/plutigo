package data

import (
	"fmt"
	"math/big"
)

type PlutusDataWrapper struct {
	Data PlutusData
}

func (p *PlutusDataWrapper) UnmarshalCBOR(data []byte) error {
	tmpData, err := Decode(data)
	if err != nil {
		return err
	}
	p.Data = tmpData
	return nil
}

func (p *PlutusDataWrapper) MarshalCBOR() ([]byte, error) {
	tmpCbor, err := Encode(p.Data)
	if err != nil {
		return nil, err
	}
	return tmpCbor, nil
}

type PlutusData interface {
	isPlutusData()

	fmt.Stringer
}

// Constr

type Constr struct {
	Tag      uint
	Fields   []PlutusData
	useIndef *bool
}

func (Constr) isPlutusData() {}

func (c Constr) String() string {
	return fmt.Sprintf("Constr{tag: %d, fields: %v}", c.Tag, c.Fields)
}

// NewConstr creates a new Constr variant.
func NewConstr(tag uint, fields ...PlutusData) PlutusData {
	if fields == nil {
		fields = make([]PlutusData, 0)
	}
	return &Constr{Tag: tag, Fields: fields}
}

// NewConstrDefIndef creates a Constr with the ability to specify whether it should use definite- or indefinite-length encoding
func NewConstrDefIndef(useIndef bool, tag uint, fields ...PlutusData) PlutusData {
	if fields == nil {
		fields = make([]PlutusData, 0)
	}
	return &Constr{Tag: tag, Fields: fields, useIndef: &useIndef}
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
	return &Map{Pairs: pairs}
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
	if value == nil {
		value = make([]byte, 0)
	}
	return &ByteString{value}
}

// List

type List struct {
	Items    []PlutusData
	useIndef *bool
}

func (List) isPlutusData() {}

func (l List) String() string {
	return fmt.Sprintf("List%v", l.Items)
}

// NewList creates a new List variant.
func NewList(items ...PlutusData) PlutusData {
	if items == nil {
		items = make([]PlutusData, 0)
	}
	return &List{Items: items}
}

// NewListDefIndef creates a list with the ability to specify whether it should use definite- or indefinite-length encoding
func NewListDefIndef(useIndef bool, items ...PlutusData) PlutusData {
	if items == nil {
		items = make([]PlutusData, 0)
	}
	return &List{Items: items, useIndef: &useIndef}
}
