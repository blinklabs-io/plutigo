package data

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
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
	Tag      *big.Int
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
	return marshalWith(c.appendCBOR)
}

func (c Constr) appendCBOR(buf *bytes.Buffer) error {
	constrTag := c.Tag
	if constrTag == nil {
		constrTag = new(big.Int)
	}
	if !constrTag.IsUint64() {
		return fmt.Errorf(
			"constructor tag %s is outside the Word64 CBOR range",
			constrTag,
		)
	}

	// Determine whether to use indefinite-length encoding for fields.
	// If useIndef is explicitly set, honor it; otherwise default to
	// Haskell's cborg behavior: indefinite for non-empty, definite for empty.
	useIndef := len(c.Fields) > 0
	if c.useIndef != nil {
		useIndef = *c.useIndef
	}

	switch {
	case constrTag.Uint64() <= 6:
		// Tags 0-6 map to CBOR tags 121-127
		appendCBORHead(buf, CborTypeTag, 121+constrTag.Uint64())
	case constrTag.Uint64() <= 127:
		// Tags 7-127 map to CBOR tags 1280-1400
		appendCBORHead(buf, CborTypeTag, 1280+constrTag.Uint64()-7)
	default:
		// Tag 102 uses a definite-length 2-element outer list
		appendCBORHead(buf, CborTypeTag, 102)
		appendCBORHead(buf, CborTypeArray, 2)
		encodedTag, err := cborMarshal(constrTag)
		if err != nil {
			return err
		}
		buf.Write(encodedTag)
	}
	return appendCBORArray(buf, c.Fields, useIndef, "Constr field")
}

func (c Constr) Clone() PlutusData {
	tmpFields := make([]PlutusData, len(c.Fields))
	for i, field := range c.Fields {
		tmpFields[i] = field.Clone()
	}
	tmpIndef := cloneUseIndef(c.useIndef)
	tmpTag := new(big.Int)
	if c.Tag != nil {
		tmpTag.Set(c.Tag)
	}
	return &Constr{Tag: tmpTag, Fields: tmpFields, useIndef: tmpIndef}
}

func (c Constr) Equal(pd PlutusData) bool {
	pdConstr, ok := pd.(*Constr)
	if !ok {
		return false
	}
	if constrTagCmp(c.Tag, pdConstr.Tag) != 0 {
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
	return fmt.Sprintf("Constr{tag: %s, fields: %v}", constrTagString(c.Tag), c.Fields)
}

// NewConstr creates a new Constr variant.
func NewConstr(tag uint64, fields ...PlutusData) PlutusData {
	tmpFields := make([]PlutusData, len(fields))
	copy(tmpFields, fields)
	return &Constr{
		Tag:    new(big.Int).SetUint64(tag),
		Fields: tmpFields,
	}
}

// NewConstrFromBigInt creates a Constr variant with an arbitrary integer tag.
// The tag is copied so later caller mutations don't change the constructor.
func NewConstrFromBigInt(tag *big.Int, fields ...PlutusData) PlutusData {
	tmpFields := make([]PlutusData, len(fields))
	copy(tmpFields, fields)
	tmpTag := new(big.Int)
	if tag != nil {
		tmpTag.Set(tag)
	}
	return &Constr{Tag: tmpTag, Fields: tmpFields}
}

// NewConstrDefIndef creates a Constr with the ability to specify whether it should use definite- or indefinite-length encoding
func NewConstrDefIndef(
	useIndef bool,
	tag uint64,
	fields ...PlutusData,
) PlutusData {
	tmpFields := make([]PlutusData, len(fields))
	copy(tmpFields, fields)
	return &Constr{
		Tag:      new(big.Int).SetUint64(tag),
		Fields:   tmpFields,
		useIndef: useIndefPtr(useIndef),
	}
}

func constrTagCmp(a, b *big.Int) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -b.Sign()
	case b == nil:
		return a.Sign()
	default:
		return a.Cmp(b)
	}
}

func constrTagString(tag *big.Int) string {
	if tag == nil {
		return "0"
	}
	return tag.String()
}

// appendCBORArray writes items as a CBOR array. When useIndef is true and the
// slice is non-empty, indefinite-length encoding is used. Empty slices always
// use definite-length encoding regardless of useIndef, matching Haskell's
// cborg behavior.
func appendCBORArray(
	buf *bytes.Buffer,
	items []PlutusData,
	useIndef bool,
	desc string,
) error {
	useIndef = useIndef && len(items) > 0
	if useIndef {
		buf.WriteByte(CborTypeArray | CborIndefFlag)
	} else {
		appendCBORHead(buf, CborTypeArray, uint64(len(items)))
	}
	for i, item := range items {
		if err := appendPlutusDataCBOR(buf, item); err != nil {
			return fmt.Errorf("failed to encode %s %d: %w", desc, i, err)
		}
	}
	if useIndef {
		buf.WriteByte(0xff) // End indefinite-length array
	}
	return nil
}

// appendPlutusDataCBOR writes pd to buf. Containers write their children
// into the same buffer, so the work is linear in the size of the encoding;
// marshaling each child separately would copy the whole encoded subtree again
// at every nesting level.
func appendPlutusDataCBOR(buf *bytes.Buffer, pd PlutusData) error {
	switch v := pd.(type) {
	case *List:
		if v != nil {
			return v.appendCBOR(buf)
		}
	case List:
		return v.appendCBOR(buf)
	case *Map:
		if v != nil {
			return v.appendCBOR(buf)
		}
	case Map:
		return v.appendCBOR(buf)
	case *Constr:
		if v != nil {
			return v.appendCBOR(buf)
		}
	case Constr:
		return v.appendCBOR(buf)
	}
	encoded, err := cborMarshal(pd)
	if err != nil {
		return err
	}
	buf.Write(encoded)
	return nil
}

// appendCBORHead writes a CBOR initial byte and argument for the given major
// type using the shortest form.
func appendCBORHead(buf *bytes.Buffer, majorType uint8, n uint64) {
	switch {
	case n < 24:
		buf.WriteByte(majorType | uint8(n)) //nolint:gosec // n < 24
	case n <= math.MaxUint8:
		buf.WriteByte(majorType | 24)
		buf.WriteByte(uint8(n)) //nolint:gosec // n <= MaxUint8
	case n <= math.MaxUint16:
		buf.WriteByte(majorType | 25)
		buf.Write(binary.BigEndian.AppendUint16(nil, uint16(n))) //nolint:gosec // n <= MaxUint16
	case n <= math.MaxUint32:
		buf.WriteByte(majorType | 26)
		buf.Write(binary.BigEndian.AppendUint32(nil, uint32(n))) //nolint:gosec // n <= MaxUint32
	default:
		buf.WriteByte(majorType | 27)
		buf.Write(binary.BigEndian.AppendUint64(nil, n))
	}
}

// marshalWith runs appendTo against a fresh buffer and returns its contents.
func marshalWith(appendTo func(*bytes.Buffer) error) ([]byte, error) {
	var buf bytes.Buffer
	if err := appendTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	return marshalWith(m.appendCBOR)
}

func (m Map) appendCBOR(buf *bytes.Buffer) error {
	// Default to definite-length encoding to match Haskell's canonical CBOR.
	// If useIndef is explicitly set, honor it.
	useIndef := m.useIndef != nil && *m.useIndef
	if useIndef {
		buf.WriteByte(CborTypeMap | CborIndefFlag)
	} else {
		appendCBORHead(buf, CborTypeMap, uint64(len(m.Pairs)))
	}
	for _, pair := range m.Pairs {
		if err := appendPlutusDataCBOR(buf, pair[0]); err != nil {
			return fmt.Errorf("encode map key: %w", err)
		}
		if err := appendPlutusDataCBOR(buf, pair[1]); err != nil {
			return fmt.Errorf("encode map value: %w", err)
		}
	}
	if useIndef {
		buf.WriteByte(0xff) // Indef-length "break" byte
	}
	return nil
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

// Value is the Plutus V4 Data constructor for the built-in Value type.
// Its payload uses the same nested map representation as V1-V3 valueData.
type Value struct {
	Inner *Map
}

func (Value) isPlutusData() {}

const valueCBORTag uint64 = 1401

func (v *Value) UnmarshalCBOR(encoded []byte) error {
	state := newDecodeState()
	if err := state.enterValue(); err != nil {
		return err
	}
	defer state.leaveValue()
	tag, content, err := decodeCBORTag(encoded)
	if err != nil {
		return err
	}
	if tag != valueCBORTag {
		return fmt.Errorf("unexpected CBOR tag for PlutusData Value: %d", tag)
	}
	value, rest, err := decodeValueNextEntered(content, state)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	v.Inner = value.Inner
	return nil
}

// decodeValueNextEntered decodes the content of a tag 1401 item. The tag has
// already been counted by the caller; the inner map is counted here, so each
// node of the tagged subtree is charged exactly once.
func decodeValueNextEntered(data []byte, state *decodeState) (*Value, []byte, error) {
	if err := state.enterValue(); err != nil {
		return nil, nil, err
	}
	defer state.leaveValue()
	inner, rest, err := decodeMapNextEntered(data, state)
	if err != nil {
		return nil, nil, err
	}
	value := &Value{Inner: inner}
	if err := value.Validate(); err != nil {
		return nil, nil, err
	}
	return value, rest, nil
}

func (v Value) MarshalCBOR() ([]byte, error) {
	if v.Inner == nil {
		return nil, errors.New("cannot encode a nil PlutusData Value")
	}
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return cborMarshal(cbor.Tag{Number: valueCBORTag, Content: v.Inner})
}

func (v Value) Clone() PlutusData {
	if v.Inner == nil {
		return &Value{}
	}
	return &Value{Inner: v.Inner.Clone().(*Map)}
}

func (v Value) Equal(pd PlutusData) bool {
	other, ok := pd.(*Value)
	if !ok || (v.Inner == nil) != (other.Inner == nil) {
		return false
	}
	return v.Inner == nil || v.Inner.Equal(other.Inner)
}

func (v Value) String() string {
	return fmt.Sprintf("Value{%v}", v.Inner)
}

// MaxValueKeyLength is the longest policy or token key a Value may carry.
const MaxValueKeyLength = 32

// ErrValueQuantityRange is wrapped by the error Validate returns for a token
// quantity outside the signed 128-bit range.
var ErrValueQuantityRange = errors.New("quantity out of signed 128-bit range")

var (
	valueQuantityMax = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
	valueQuantityMin = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))
)

// Validate checks that a Value is canonical: policy and token keys are
// bytestrings of at most MaxValueKeyLength bytes, both levels are strictly
// ascending, every token map is non-empty, and every quantity is a non-zero
// integer within the signed 128-bit range.
func (v Value) Validate() error {
	if v.Inner == nil {
		return errors.New("Value must contain a map")
	}
	var prevPolicy []byte
	for i, policy := range v.Inner.Pairs {
		policyKey, ok := policy[0].(*ByteString)
		if !ok || policyKey == nil {
			return fmt.Errorf("Value policy key must be a bytestring, got %T", policy[0])
		}
		if err := CheckValueKey("policy", i, policyKey.Inner, prevPolicy); err != nil {
			return err
		}
		prevPolicy = policyKey.Inner
		tokens, ok := policy[1].(*Map)
		if !ok || tokens == nil {
			return fmt.Errorf("Value policy value must be a map, got %T", policy[1])
		}
		if err := CheckValueTokenCount(i, len(tokens.Pairs)); err != nil {
			return err
		}
		var prevToken []byte
		for j, token := range tokens.Pairs {
			tokenKey, ok := token[0].(*ByteString)
			if !ok || tokenKey == nil {
				return fmt.Errorf("Value token key must be a bytestring, got %T", token[0])
			}
			if err := CheckValueKey("token", j, tokenKey.Inner, prevToken); err != nil {
				return err
			}
			prevToken = tokenKey.Inner
			quantity, ok := token[1].(*Integer)
			if !ok || quantity == nil || quantity.Inner == nil {
				return fmt.Errorf("Value token quantity must be an integer, got %T", token[1])
			}
			if err := CheckValueQuantity(i, j, quantity.Inner); err != nil {
				return err
			}
		}
	}
	return nil
}

// CheckValueKey validates a policy or token key at index i against its
// predecessor. The first key at a level has no predecessor, which is tracked by
// index because an empty key is a valid predecessor.
func CheckValueKey(kind string, i int, key, prev []byte) error {
	if len(key) > MaxValueKeyLength {
		return fmt.Errorf("Value %s %d key exceeds %d bytes", kind, i, MaxValueKeyLength)
	}
	if i > 0 && bytes.Compare(prev, key) >= 0 {
		return fmt.Errorf("Value %s keys must be strictly ascending (index %d)", kind, i)
	}
	return nil
}

// CheckValueTokenCount rejects an empty token map for a Value policy.
func CheckValueTokenCount(policy, count int) error {
	if count == 0 {
		return fmt.Errorf("Value policy %d has an empty token map", policy)
	}
	return nil
}

// CheckValueQuantity checks that a Value token quantity is a non-zero signed
// 128-bit integer. Policy and token are its indices for error reporting.
func CheckValueQuantity(policy, token int, quantity *big.Int) error {
	switch {
	case quantity == nil:
		return errors.New("Value token quantity must be an integer")
	case quantity.Sign() == 0:
		return fmt.Errorf("Value policy %d token %d has a zero quantity", policy, token)
	case quantity.Cmp(valueQuantityMin) < 0, quantity.Cmp(valueQuantityMax) > 0:
		return fmt.Errorf("Value policy %d token %d: %w", policy, token, ErrValueQuantityRange)
	}
	return nil
}

// NewValue creates a validated Plutus V4 Data Value constructor.
func NewValue(inner *Map) (*Value, error) {
	value := &Value{Inner: inner}
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return value, nil
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
	// Plutus encodes an integer inside the Word64 range, or its negative
	// mirror, as a plain CBOR integer, and any other integer as a CBOR bignum
	// whose magnitude byte string obeys the same MaxByteStringLeafSize
	// chunking rule as a Plutus byte string. See PlutusCore.Data.encodeInteger.
	if i.Inner == nil {
		return cborMarshal(i.Inner)
	}
	tag := byte(cborTagPositiveBignum)
	magnitude := i.Inner
	if i.Inner.Sign() < 0 {
		// A negative bignum encodes -1 - i, which is big.Int.Not for a
		// negative operand.
		tag = byte(cborTagNegativeBignum)
		magnitude = new(big.Int).Not(i.Inner)
	}
	if magnitude.BitLen() <= 64 {
		return cborMarshal(i.Inner)
	}
	body, err := marshalDataByteString(magnitude.Bytes())
	if err != nil {
		return nil, err
	}
	return append([]byte{tag}, body...), nil
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
	return marshalDataByteString(b.Inner)
}

// marshalDataByteString encodes a byte string the way Plutus' Data encoding
// does: definite-length up to MaxByteStringLeafSize bytes, and otherwise an
// indefinite-length byte string split into chunks of that size. This applies
// both to a ByteString and to the magnitude of a bignum-encoded Integer. See
// PlutusCore.Data.encodeBs.
func marshalDataByteString(b []byte) ([]byte, error) {
	if len(b) <= MaxByteStringLeafSize {
		if b == nil {
			return cborMarshal([]byte{})
		}
		return cborMarshal(b)
	}
	// Indefinite-length byte string with MaxByteStringLeafSize chunks
	var buf bytes.Buffer
	buf.WriteByte(0x5f) // Start indefinite-length byte string
	for i := 0; i < len(b); i += MaxByteStringLeafSize {
		end := min(i+MaxByteStringLeafSize, len(b))
		chunk, err := cborMarshal(b[i:end])
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
	state := newDecodeState()
	if err := state.enterValue(); err != nil {
		return nil, nil, err
	}
	defer state.leaveValue()
	return decodeConstrFromDataEntered(data, state)
}

func decodeConstrFromDataEntered(
	data []byte,
	state *decodeState,
) (*Constr, []byte, error) {
	tagNumber, tagContent, err := decodeCBORTag(data)
	if err != nil {
		return nil, nil, err
	}
	return decodeConstrNextEntered(tagNumber, tagContent, state)
}

func decodeConstrNextEntered(
	tagNumber uint64,
	data []byte,
	state *decodeState,
) (*Constr, []byte, error) {
	switch {
	case tagNumber >= 121 && tagNumber <= 127:
		tmpFields, tmpUseIndef, rest, err := decodeListItemsNextEntered(
			data,
			state,
		)
		if err != nil {
			return nil, nil, err
		}
		return &Constr{
			Tag:      new(big.Int).SetUint64(tagNumber - 121),
			Fields:   tmpFields,
			useIndef: tmpUseIndef,
		}, rest, nil
	case tagNumber >= 1280 && tagNumber <= 1400:
		tmpFields, tmpUseIndef, rest, err := decodeListItemsNextEntered(
			data,
			state,
		)
		if err != nil {
			return nil, nil, err
		}
		return &Constr{
			Tag:      new(big.Int).SetUint64(tagNumber - 1280 + 7),
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

		tmpFields, tmpUseIndef, next, err := decodeListItemsNextEntered(
			rest,
			state,
		)
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
			Tag:      new(big.Int).SetUint64(alternative),
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
	state := newDecodeState()
	if err := state.enterValue(); err != nil {
		return nil, nil, err
	}
	defer state.leaveValue()
	return decodeMapNextEntered(data, state)
}

func decodeMapNextEntered(data []byte, state *decodeState) (*Map, []byte, error) {
	pairCount, rest, useIndef, err := decodeCBORMap(data)
	if err != nil {
		return nil, nil, err
	}

	var pairs [][2]PlutusData
	if useIndef {
		pairs = make([][2]PlutusData, 0, 4)
	} else {
		// Each pair needs at least 2 bytes (one per key/value CBOR head).
		if pairCount > len(rest)/2 {
			return nil, nil, fmt.Errorf("CBOR map claims %d pairs but only %d bytes remain", pairCount, len(rest))
		}
		if err := state.checkAdditionalNodes(pairCount * 2); err != nil {
			return nil, nil, err
		}
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

		tmpKey, next, err := decodeNextPlutusDataWithState(rest, state)
		if err != nil {
			return nil, nil, err
		}
		rest = next

		tmpVal, next, err := decodeNextPlutusDataWithState(rest, state)
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
	state := newDecodeState()
	if err := state.enterValue(); err != nil {
		return nil, nil, err
	}
	defer state.leaveValue()
	return decodeListNextEntered(data, state)
}

func decodeListNextEntered(data []byte, state *decodeState) (*List, []byte, error) {
	tmpItems, tmpUseIndef, rest, err := decodeListItemsNextEntered(data, state)
	if err != nil {
		return nil, nil, err
	}
	return &List{Items: tmpItems, useIndef: tmpUseIndef}, rest, nil
}

func decodeListItemsNext(data []byte) ([]PlutusData, *bool, []byte, error) {
	return decodeListItemsNextEntered(data, newDecodeState())
}

func decodeListItemsNextEntered(
	data []byte,
	state *decodeState,
) ([]PlutusData, *bool, []byte, error) {
	itemCount, rest, useIndef, err := decodeCBORArray(data)
	if err != nil {
		return nil, nil, nil, err
	}

	if !useIndef {
		tmpItems, rest, err := decodeListItemsDefiniteEntered(
			itemCount,
			rest,
			state,
		)
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

		tmp, next, err := decodeNextPlutusDataWithState(rest, state)
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
	return decodeListItemsDefiniteEntered(itemCount, rest, newDecodeState())
}

func decodeListItemsDefiniteEntered(
	itemCount int,
	rest []byte,
	state *decodeState,
) ([]PlutusData, []byte, error) {
	// Each item needs at least 1 byte for a CBOR head.
	if itemCount > len(rest) {
		return nil, nil, fmt.Errorf("CBOR array claims %d items but only %d bytes remain", itemCount, len(rest))
	}
	if err := state.checkAdditionalNodes(itemCount); err != nil {
		return nil, nil, err
	}
	tmpItems := make([]PlutusData, itemCount)
	for i := range itemCount {
		tmp, next, err := decodeNextPlutusDataWithState(rest, state)
		if err != nil {
			return nil, nil, err
		}
		rest = next
		tmpItems[i] = tmp
	}
	return tmpItems, rest, nil
}

func (l List) MarshalCBOR() ([]byte, error) {
	return marshalWith(l.appendCBOR)
}

func (l List) appendCBOR(buf *bytes.Buffer) error {
	// Determine whether to use indefinite-length encoding.
	// If useIndef is explicitly set, honor it; otherwise default to
	// Haskell's cborg behavior: indefinite for non-empty, definite for empty.
	useIndef := len(l.Items) > 0
	if l.useIndef != nil {
		useIndef = *l.useIndef
	}
	return appendCBORArray(buf, l.Items, useIndef, "list item")
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
