package data

import (
	"bytes"
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
	Clone() PlutusData
	Equal(PlutusData) bool
	fmt.Stringer
}

// Constr

type Constr struct {
	Tag      uint
	Fields   []PlutusData
	useIndef *bool
}

func (Constr) isPlutusData() {}

func (c Constr) Clone() PlutusData {
	tmpFields := make([]PlutusData, len(c.Fields))
	for i, field := range c.Fields {
		tmpFields[i] = field.Clone()
	}
	var tmpIndef *bool
	if c.useIndef != nil {
		tmpIndef = new(bool)
		*tmpIndef = *c.useIndef
	}
	return &Constr{Tag: c.Tag, Fields: tmpFields, useIndef: tmpIndef}
}

func (c Constr) Equal(pd PlutusData) bool {
	pdConstr, ok := pd.(*Constr)
	if !ok {
		return false
	}
	if c.Tag != pdConstr.Tag {
		return false
	}
	if len(c.Fields) != len(pdConstr.Fields) {
		return false
	}
	for i := range c.Fields {
		if !c.Fields[i].Equal(pdConstr.Fields[i]) {
			return false
		}
	}
	return true
}

func (c Constr) String() string {
	return fmt.Sprintf("Constr{tag: %d, fields: %v}", c.Tag, c.Fields)
}

// NewConstr creates a new Constr variant.
func NewConstr(tag uint, fields ...PlutusData) PlutusData {
	tmpFields := make([]PlutusData, len(fields))
	copy(tmpFields, fields)
	return &Constr{Tag: tag, Fields: tmpFields}
}

// NewConstrDefIndef creates a Constr with the ability to specify whether it should use definite- or indefinite-length encoding
func NewConstrDefIndef(
	useIndef bool,
	tag uint,
	fields ...PlutusData,
) PlutusData {
	tmpFields := make([]PlutusData, len(fields))
	copy(tmpFields, fields)
	return &Constr{Tag: tag, Fields: tmpFields, useIndef: &useIndef}
}

// Map

type Map struct {
	Pairs    [][2]PlutusData // Each pair is [key, value]
	useIndef *bool
}

func (Map) isPlutusData() {}

func (m Map) Clone() PlutusData {
	tmpPairs := make([][2]PlutusData, len(m.Pairs))
	for i, pair := range m.Pairs {
		tmpPairs[i] = [2]PlutusData{
			pair[0].Clone(),
			pair[1].Clone(),
		}
	}
	var tmpIndef *bool
	if m.useIndef != nil {
		tmpIndef = new(bool)
		*tmpIndef = *m.useIndef
	}
	return &Map{Pairs: tmpPairs, useIndef: tmpIndef}
}

func (m Map) Equal(pd PlutusData) bool {
	pdMap, ok := pd.(*Map)
	if !ok {
		return false
	}
	if len(m.Pairs) != len(pdMap.Pairs) {
		return false
	}
	for i, pair := range m.Pairs {
		if !pair[0].Equal(pdMap.Pairs[i][0]) {
			return false
		}
		if !pair[1].Equal(pdMap.Pairs[i][1]) {
			return false
		}
	}
	return true
}

func (m Map) String() string {
	return fmt.Sprintf("Map%v", m.Pairs)
}

// NewMap creates a new Map variant.
func NewMap(pairs [][2]PlutusData) PlutusData {
	tmpPairs := make([][2]PlutusData, len(pairs))
	copy(tmpPairs, pairs)
	return &Map{Pairs: tmpPairs}
}

// NewMapDefIndef creates a new Map with the ability to specify whether it should use definite- or indefinite-length encoding
func NewMapDefIndef(useIndef bool, pairs [][2]PlutusData) PlutusData {
	tmpPairs := make([][2]PlutusData, len(pairs))
	copy(tmpPairs, pairs)
	return &Map{Pairs: tmpPairs, useIndef: &useIndef}
}

// Integer

type Integer struct {
	Inner *big.Int
}

func (Integer) isPlutusData() {}

func (i Integer) Clone() PlutusData {
	tmpVal := new(big.Int).Set(i.Inner)
	return &Integer{tmpVal}
}

func (i Integer) Equal(pd PlutusData) bool {
	pdInt, ok := pd.(*Integer)
	if !ok {
		return false
	}
	if i.Inner.Cmp(pdInt.Inner) != 0 {
		return false
	}
	return true
}

func (i Integer) String() string {
	return fmt.Sprintf("Integer(%s)", i.Inner.String())
}

// NewInteger creates a new Integer variant.
func NewInteger(value *big.Int) PlutusData {
	tmpVal := new(big.Int).Set(value)
	return &Integer{tmpVal}
}

// ByteString

type ByteString struct {
	Inner []byte
}

func (ByteString) isPlutusData() {}

func (b ByteString) Clone() PlutusData {
	tmpVal := make([]byte, len(b.Inner))
	copy(tmpVal, b.Inner)
	return &ByteString{tmpVal}
}

func (b ByteString) Equal(pd PlutusData) bool {
	pdByteString, ok := pd.(*ByteString)
	if !ok {
		return false
	}
	if !bytes.Equal(b.Inner, pdByteString.Inner) {
		return false
	}
	return true
}

func (b ByteString) String() string {
	return fmt.Sprintf("ByteString(%x)", b.Inner)
}

// NewByteString creates a new ByteString variant.
func NewByteString(value []byte) PlutusData {
	tmpVal := make([]byte, len(value))
	copy(tmpVal, value)
	return &ByteString{tmpVal}
}

// List

type List struct {
	Items    []PlutusData
	useIndef *bool
}

func (List) isPlutusData() {}

func (l List) Clone() PlutusData {
	tmpItems := make([]PlutusData, len(l.Items))
	for i, item := range l.Items {
		tmpItems[i] = item.Clone()
	}
	var tmpIndef *bool
	if l.useIndef != nil {
		tmpIndef = new(bool)
		*tmpIndef = *l.useIndef
	}
	return &List{Items: tmpItems, useIndef: tmpIndef}
}

func (l List) Equal(pd PlutusData) bool {
	pdList, ok := pd.(*List)
	if !ok {
		return false
	}
	if len(l.Items) != len(pdList.Items) {
		return false
	}
	for i := range l.Items {
		if !l.Items[i].Equal(pdList.Items[i]) {
			return false
		}
	}
	return true
}

func (l List) String() string {
	return fmt.Sprintf("List%v", l.Items)
}

// NewList creates a new List variant.
func NewList(items ...PlutusData) PlutusData {
	tmpItems := make([]PlutusData, len(items))
	copy(tmpItems, items)
	return &List{Items: tmpItems}
}

// NewListDefIndef creates a list with the ability to specify whether it should use definite- or indefinite-length encoding
func NewListDefIndef(useIndef bool, items ...PlutusData) PlutusData {
	tmpItems := make([]PlutusData, len(items))
	copy(tmpItems, items)
	return &List{Items: tmpItems, useIndef: &useIndef}
}
