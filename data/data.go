package data

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/fxamacker/cbor/v2"
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

func (c *Constr) UnmarshalCBOR(data []byte) error {
	var tmpTag cbor.RawTag
	if err := cborUnmarshal(data, &tmpTag); err != nil {
		return err
	}
	switch {
	// Constr with tag 0..6.
	case tmpTag.Number >= 121 && tmpTag.Number <= 127:
		c.Tag = uint(tmpTag.Number) - 121
		var tmpList List
		if err := cborUnmarshal(tmpTag.Content, &tmpList); err != nil {
			return err
		}
		c.Fields = tmpList.Items
		c.useIndef = tmpList.useIndef
	case tmpTag.Number >= 1280 && tmpTag.Number <= 1400:
		c.Tag = uint(tmpTag.Number) - 1280 + 7
		var tmpList List
		if err := cborUnmarshal(tmpTag.Content, &tmpList); err != nil {
			return err
		}
		c.Fields = tmpList.Items
		c.useIndef = tmpList.useIndef
	case tmpTag.Number == 102:
		var tmpData struct {
			_           struct{} `cbor:",toarray"`
			Alternative uint64
			FieldsRaw   cbor.RawMessage
		}
		if err := cborUnmarshal(tmpTag.Content, &tmpData); err != nil {
			return err
		}
		c.Tag = uint(tmpData.Alternative)
		var tmpList List
		if err := cborUnmarshal(tmpData.FieldsRaw, &tmpList); err != nil {
			return err
		}
		c.Fields = tmpList.Items
		c.useIndef = tmpList.useIndef
	default:
		return fmt.Errorf(
			"unknown CBOR tag for PlutusData constructor: %d",
			tmpTag.Number,
		)
	}
	return nil
}

func (c Constr) MarshalCBOR() ([]byte, error) {
	useIndef := len(c.Fields) > 0
	if c.useIndef != nil {
		useIndef = *c.useIndef
	}
	// Encode fields first
	var fields any
	if !useIndef {
		// Encode empty fields as simple array
		tmpFields := make([]any, len(c.Fields))
		for i, item := range c.Fields {
			encoded, err := cborMarshal(item)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to encode Constr field %d: %w",
					i,
					err,
				)
			}
			tmpFields[i] = cbor.RawMessage(encoded)
		}
		fields = tmpFields
	} else {
		// Encode as indefinite-length array using buffer to avoid repeated allocations
		var buf bytes.Buffer
		buf.WriteByte(0x9F) // Start indefinite-length list
		for i, item := range c.Fields {
			encoded, err := cborMarshal(item)
			if err != nil {
				return nil, fmt.Errorf("failed to encode Constr field %d: %w", i, err)
			}
			buf.Write(encoded)
		}
		buf.WriteByte(0xff) // End indefinite-length list
		fields = cbor.RawMessage(buf.Bytes())
	}

	// Determine CBOR tag based on Constr tag value
	var cborTag uint64
	switch {
	case c.Tag <= 6:
		// Tags 0-6 map to CBOR tags 121-127
		cborTag = 121 + uint64(c.Tag)
	case c.Tag >= 7 && c.Tag <= 127:
		// Tags 7-127 map to CBOR tags 1280-1400
		cborTag = 1280 + uint64(c.Tag-7)
	default:
		cborTag = 102
		fields = []any{c.Tag, fields}
	}

	tmpTag := cbor.Tag{
		Number:  cborTag,
		Content: fields,
	}
	return cborMarshal(tmpTag)
}

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

func (m *Map) UnmarshalCBOR(data []byte) error {
	// The below is a hack to work around our CBOR library not supporting preserving key
	// order when decoding a map. We decode our map to determine its length, create a dummy
	// list the same length as our map to determine the header size, and then decode each
	// key/value pair individually. We use a pointer for the key to keep duplicates to get
	// an accurate count for decoding
	useIndef := (data[0] & CborIndefFlag) == CborIndefFlag
	var tmpData map[*cbor.RawMessage]cbor.RawMessage
	if err := cborUnmarshal(data, &tmpData); err != nil {
		return err
	}
	if useIndef {
		// Strip off indef-length map header byte
		data = data[1:]
	} else {
		// Create dummy list of same length to determine map header length
		tmpList := make([]bool, len(tmpData))
		tmpListRaw, err := cborMarshal(tmpList)
		if err != nil {
			return err
		}
		tmpListHeader := tmpListRaw[0 : len(tmpListRaw)-len(tmpData)]
		// Strip off map header bytes
		data = data[len(tmpListHeader):]
	}
	pairs := make([][2]PlutusData, 0, len(tmpData))
	var rawKey, rawVal cbor.RawMessage
	// Read key/value pairs until we have no data left
	var err error
	for len(data) > 0 {
		// Check for "break" at end of indefinite-length map
		if data[0] == 0xFF {
			break
		}
		// Read raw key/value bytes
		data, err = cbor.UnmarshalFirst(data, &rawKey)
		if err != nil {
			return err
		}
		data, err = cbor.UnmarshalFirst(data, &rawVal)
		if err != nil {
			return err
		}
		// Decode key/value
		tmpKey, err := decode(rawKey)
		if err != nil {
			return err
		}
		tmpVal, err := decode(rawVal)
		if err != nil {
			return err
		}
		pairs = append(
			pairs,
			[2]PlutusData{
				tmpKey,
				tmpVal,
			},
		)
	}
	m.Pairs = pairs
	m.useIndef = &useIndef
	return nil
}

func (m Map) MarshalCBOR() ([]byte, error) {
	// The below is a hack to work around our CBOR library not supporting encoding a map
	// with a specific key order. We pre-encode each key/value pair, build a dummy list to
	// steal and modify its header, and build our own output from pieces. This avoids
	// needing to support 6 different possible encodings of a map's header byte depending
	// on length
	useIndef := false
	if m.useIndef != nil {
		useIndef = *m.useIndef
	}
	// Build encoded pairs into buffer directly to avoid allocations
	var pairsBuf bytes.Buffer
	for _, pair := range m.Pairs {
		keyRaw, err := cborMarshal(pair[0])
		if err != nil {
			return nil, fmt.Errorf("encode map key: %w", err)
		}
		valueRaw, err := cborMarshal(pair[1])
		if err != nil {
			return nil, fmt.Errorf("encode map value: %w", err)
		}
		pairsBuf.Write(keyRaw)
		pairsBuf.Write(valueRaw)
	}
	// Build return value
	var ret bytes.Buffer
	if useIndef {
		ret.WriteByte(CborTypeMap | CborIndefFlag)
	} else {
		// Create dummy list with simple (one-byte) values so we can easily extract the header
		tmpList := make([]bool, len(m.Pairs))
		tmpListRaw, err := cborMarshal(tmpList)
		if err != nil {
			return nil, err
		}
		tmpListHeader := tmpListRaw[0 : len(tmpListRaw)-len(m.Pairs)]
		// Modify header byte to switch type from array to map
		tmpListHeader[0] |= 0x20
		ret.Write(tmpListHeader)
	}
	ret.Write(pairsBuf.Bytes())
	if useIndef {
		// Indef-length "break" byte
		ret.WriteByte(0xff)
	}
	return ret.Bytes(), nil
}

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

func (i *Integer) UnmarshalCBOR(data []byte) error {
	return cborUnmarshal(data, &i.Inner)
}

func (i Integer) MarshalCBOR() ([]byte, error) {
	return cborMarshal(i.Inner)
}

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

func (b *ByteString) UnmarshalCBOR(data []byte) error {
	if err := cborUnmarshal(data, &b.Inner); err != nil {
		return err
	}
	return nil
}

func (b ByteString) MarshalCBOR() ([]byte, error) {
	// The Rust code shows special handling for byte strings longer than 64 bytes
	// using indefinite-length encoding, but the Go CBOR library handles this automatically
	// when using cbor.Marshal, so we can just return the bytes directly
	return cborMarshal(b.Inner)
}

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

func (l *List) UnmarshalCBOR(data []byte) error {
	useIndef := (data[0] & CborIndefFlag) == CborIndefFlag
	var tmpData []cbor.RawMessage
	if err := cborUnmarshal(data, &tmpData); err != nil {
		return err
	}
	tmpItems := make([]PlutusData, len(tmpData))
	for i, item := range tmpData {
		tmp, err := decode(item)
		if err != nil {
			return err
		}
		tmpItems[i] = tmp
	}
	l.Items = tmpItems
	l.useIndef = &useIndef
	return nil
}

func (l List) MarshalCBOR() ([]byte, error) {
	useIndef := len(l.Items) > 0
	if l.useIndef != nil {
		useIndef = *l.useIndef
	}
	if !useIndef {
		ret := make([]any, len(l.Items))
		for i, item := range l.Items {
			ret[i] = item
		}
		return cborMarshal(ret)
	}
	// Use buffer to avoid repeated allocations from slices.Concat
	var buf bytes.Buffer
	buf.WriteByte(0x9F) // Start indefinite-length list
	for i, item := range l.Items {
		encoded, err := cborMarshal(item)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to encode indef-length list item %d: %w",
				i,
				err,
			)
		}
		buf.Write(encoded)
	}
	buf.WriteByte(0xff) // End indefinite-length list
	return buf.Bytes(), nil
}

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
