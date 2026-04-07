package data

import (
	"bytes"
	"errors"
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

var (
	sharedUseIndefFalse = false
	sharedUseIndefTrue  = true
)

func useIndefPtr(useIndef bool) *bool {
	if useIndef {
		return &sharedUseIndefTrue
	}
	return &sharedUseIndefFalse
}

// Constr

type Constr struct {
	Tag      uint
	Fields   []PlutusData
	useIndef *bool
}

func (Constr) isPlutusData() {}

func (c *Constr) UnmarshalCBOR(data []byte) error {
	tmpConstr, rest, err := decodeConstrFromData(data)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	c.Tag = tmpConstr.Tag
	c.Fields = tmpConstr.Fields
	c.useIndef = tmpConstr.useIndef
	return nil
}

func (c Constr) MarshalCBOR() ([]byte, error) {
	// Determine whether to use indefinite-length encoding for fields.
	// If useIndef is explicitly set, honor it; otherwise default to
	// Haskell's cborg behavior: indefinite for non-empty, definite for empty.
	useIndef := len(c.Fields) > 0
	if c.useIndef != nil {
		useIndef = *c.useIndef
	}

	fields, err := encodeCBORArray(c.Fields, useIndef, "Constr field")
	if err != nil {
		return nil, err
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
		// Tag 102 uses a definite-length 2-element outer list
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
	tmpIndef := cloneUseIndef(c.useIndef)
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
	return &Constr{Tag: tag, Fields: tmpFields, useIndef: useIndefPtr(useIndef)}
}

// encodeCBORArray encodes a slice of PlutusData items as a CBOR array.
// When useIndef is true and the slice is non-empty, indefinite-length encoding
// is used. Empty slices always use definite-length encoding regardless of
// useIndef, matching Haskell's cborg behavior.
func encodeCBORArray(
	items []PlutusData,
	useIndef bool,
	desc string,
) (any, error) {
	if len(items) == 0 {
		return [0]any{}, nil
	}
	if useIndef {
		var buf bytes.Buffer
		buf.WriteByte(0x9F) // Start indefinite-length array
		for i, item := range items {
			encoded, err := cborMarshal(item)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to encode %s %d: %w",
					desc,
					i,
					err,
				)
			}
			buf.Write(encoded)
		}
		buf.WriteByte(0xff) // End indefinite-length array
		return cbor.RawMessage(buf.Bytes()), nil
	}
	encoded := make([]cbor.RawMessage, len(items))
	for i, item := range items {
		raw, err := cborMarshal(item)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to encode %s %d: %w",
				desc,
				i,
				err,
			)
		}
		encoded[i] = raw
	}
	return encoded, nil
}

// Map

type Map struct {
	Pairs    [][2]PlutusData // Each pair is [key, value]
	useIndef *bool
}

func (Map) isPlutusData() {}

func (m *Map) UnmarshalCBOR(data []byte) error {
	tmpMap, rest, err := decodeMapNext(data)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	m.Pairs = tmpMap.Pairs
	m.useIndef = tmpMap.useIndef
	return nil
}

func (m Map) MarshalCBOR() ([]byte, error) {
	// The below is a hack to work around our CBOR library not supporting encoding a map
	// with a specific key order. We pre-encode each key/value pair, build a dummy list to
	// steal and modify its header, and build our own output from pieces. This avoids
	// needing to support 6 different possible encodings of a map's header byte depending
	// on length
	// Default to definite-length encoding to match Haskell's canonical CBOR.
	// If useIndef is explicitly set, honor it.
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
	tmpIndef := cloneUseIndef(m.useIndef)
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
	return &Map{Pairs: tmpPairs, useIndef: useIndefPtr(useIndef)}
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
	// Haskell's Plutus encodes ByteStrings <= 64 bytes as definite-length,
	// and ByteStrings > 64 bytes as indefinite-length with 64-byte chunks.
	if len(b.Inner) <= 64 {
		return cborMarshal(b.Inner)
	}
	// Indefinite-length byte string with 64-byte chunks
	var buf bytes.Buffer
	buf.WriteByte(0x5f) // Start indefinite-length byte string
	for i := 0; i < len(b.Inner); i += 64 {
		end := i + 64
		if end > len(b.Inner) {
			end = len(b.Inner)
		}
		chunk, err := cborMarshal(b.Inner[i:end])
		if err != nil {
			return nil, fmt.Errorf("failed to encode byte string chunk: %w", err)
		}
		buf.Write(chunk)
	}
	buf.WriteByte(0xff) // End indefinite-length byte string
	return buf.Bytes(), nil
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
	tmpList, rest, err := decodeListNext(data)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	l.Items = tmpList.Items
	l.useIndef = tmpList.useIndef
	return nil
}

func decodeConstrFromData(data []byte) (*Constr, []byte, error) {
	tagNumber, tagContent, err := decodeCBORTag(data)
	if err != nil {
		return nil, nil, err
	}
	return decodeConstrNext(tagNumber, tagContent)
}

func decodeConstrNext(tagNumber uint64, data []byte) (*Constr, []byte, error) {
	switch {
	case tagNumber >= 121 && tagNumber <= 127:
		tmpFields, tmpUseIndef, rest, err := decodeListItemsNext(data)
		if err != nil {
			return nil, nil, err
		}
		return &Constr{
			Tag:      uint(tagNumber) - 121,
			Fields:   tmpFields,
			useIndef: tmpUseIndef,
		}, rest, nil
	case tagNumber >= 1280 && tagNumber <= 1400:
		tmpFields, tmpUseIndef, rest, err := decodeListItemsNext(data)
		if err != nil {
			return nil, nil, err
		}
		return &Constr{
			Tag:      uint(tagNumber) - 1280 + 7,
			Fields:   tmpFields,
			useIndef: tmpUseIndef,
		}, rest, nil
	case tagNumber == 102:
		fieldCount, rest, useIndef, err := decodeCBORArray(data)
		if err != nil {
			return nil, nil, err
		}
		if !useIndef && fieldCount != 2 {
			return nil, nil, fmt.Errorf("constructor 102 outer array has %d items, want 2", fieldCount)
		}

		alternative, next, err := decodeCBORUint(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next

		tmpFields, tmpUseIndef, next, err := decodeListItemsNext(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next

		if useIndef {
			if len(rest) == 0 || rest[0] != 0xff {
				return nil, nil, errors.New("unterminated indefinite-length CBOR array")
			}
			rest = rest[1:]
		}

		return &Constr{
			Tag:      uint(alternative),
			Fields:   tmpFields,
			useIndef: tmpUseIndef,
		}, rest, nil
	default:
		return nil, nil, fmt.Errorf(
			"unknown CBOR tag for PlutusData constructor: %d",
			tagNumber,
		)
	}
}

func decodeMapNext(data []byte) (*Map, []byte, error) {
	pairCount, rest, useIndef, err := decodeCBORMap(data)
	if err != nil {
		return nil, nil, err
	}

	var pairs [][2]PlutusData
	if useIndef {
		pairs = make([][2]PlutusData, 0, 4)
	} else {
		pairs = make([][2]PlutusData, pairCount)
	}

	for i := 0; useIndef || i < pairCount; i++ {
		if useIndef {
			if len(rest) == 0 {
				return nil, nil, errors.New("unterminated indefinite-length CBOR map")
			}
			if rest[0] == 0xff {
				rest = rest[1:]
				break
			}
		}

		tmpKey, next, err := decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next

		tmpVal, next, err := decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next

		if useIndef {
			pairs = append(pairs, [2]PlutusData{tmpKey, tmpVal})
		} else {
			pairs[i] = [2]PlutusData{tmpKey, tmpVal}
		}
	}

	return &Map{Pairs: pairs, useIndef: useIndefPtr(useIndef)}, rest, nil
}

func decodeListNext(data []byte) (*List, []byte, error) {
	tmpItems, tmpUseIndef, rest, err := decodeListItemsNext(data)
	if err != nil {
		return nil, nil, err
	}
	return &List{Items: tmpItems, useIndef: tmpUseIndef}, rest, nil
}

func decodeListItemsNext(data []byte) ([]PlutusData, *bool, []byte, error) {
	itemCount, rest, useIndef, err := decodeCBORArray(data)
	if err != nil {
		return nil, nil, nil, err
	}

	if !useIndef {
		tmpItems, rest, err := decodeListItemsDefinite(itemCount, rest)
		if err != nil {
			return nil, nil, nil, err
		}
		return tmpItems, useIndefPtr(false), rest, nil
	}

	var smallItems [4]PlutusData
	var tmpItems []PlutusData
	tmpLen := 0
	for {
		if len(rest) == 0 {
			return nil, nil, nil, errors.New("unterminated indefinite-length CBOR array")
		}
		if rest[0] == 0xff {
			rest = rest[1:]
			break
		}

		tmp, next, err := decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, nil, err
		}
		rest = next

		if tmpItems != nil {
			if tmpLen == len(tmpItems) {
				tmpItems = append(tmpItems, tmp)
				tmpLen++
				continue
			}
			tmpItems[tmpLen] = tmp
			tmpLen++
			continue
		}
		if tmpLen < len(smallItems) {
			smallItems[tmpLen] = tmp
			tmpLen++
			continue
		}
		tmpItems = make([]PlutusData, tmpLen+1, tmpLen*2)
		copy(tmpItems, smallItems[:tmpLen])
		tmpItems[tmpLen] = tmp
		tmpLen++
	}

	if tmpItems == nil {
		tmpItems = make([]PlutusData, tmpLen)
		copy(tmpItems, smallItems[:tmpLen])
	} else {
		tmpItems = tmpItems[:tmpLen]
	}
	return tmpItems, useIndefPtr(true), rest, nil
}

func decodeListItemsDefinite(itemCount int, rest []byte) ([]PlutusData, []byte, error) {
	tmpItems := make([]PlutusData, itemCount)
	for i := range itemCount {
		tmp, next, err := decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next
		tmpItems[i] = tmp
	}
	return tmpItems, rest, nil
}

func (l List) MarshalCBOR() ([]byte, error) {
	// Determine whether to use indefinite-length encoding.
	// If useIndef is explicitly set, honor it; otherwise default to
	// Haskell's cborg behavior: indefinite for non-empty, definite for empty.
	useIndef := len(l.Items) > 0
	if l.useIndef != nil {
		useIndef = *l.useIndef
	}

	result, err := encodeCBORArray(l.Items, useIndef, "list item")
	if err != nil {
		return nil, err
	}
	return cborMarshal(result)
}

func (l List) Clone() PlutusData {
	tmpItems := make([]PlutusData, len(l.Items))
	for i, item := range l.Items {
		tmpItems[i] = item.Clone()
	}
	tmpIndef := cloneUseIndef(l.useIndef)
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
	return &List{Items: tmpItems, useIndef: useIndefPtr(useIndef)}
}

func cloneUseIndef(useIndef *bool) *bool {
	if useIndef == nil {
		return nil
	}
	return useIndefPtr(*useIndef)
}
