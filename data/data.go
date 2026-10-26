// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package data

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

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
	tagNumber, _, err := decodeCBORTag(data)
	if err != nil {
		return err
	}
	if !isConstrTag(tagNumber) {
		return fmt.Errorf(
			"unknown CBOR tag for PlutusData constructor: %d",
			tagNumber,
		)
	}
	tmpConstr, err := decodeAs[*Constr](data, false)
	if err != nil {
		return err
	}
	*c = *tmpConstr
	return nil
}

func (c Constr) MarshalCBOR() ([]byte, error) {
	return marshalPlutusDataCBOR(c)
}

// appendCBORHead writes the constructor's tag and the head of its fields
// array, and returns the frame that writes the fields.
func (c Constr) appendCBORHead(buf *bytes.Buffer) (*encodeFrame, error) {
	constrTag := c.Tag
	if constrTag == nil {
		constrTag = new(big.Int)
	}
	if !constrTag.IsUint64() {
		return nil, fmt.Errorf(
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
			return nil, err
		}
		buf.Write(encodedTag)
	}
	return arrayFrame(buf, c.Fields, useIndef, "Constr field"), nil
}

func (c Constr) Clone() PlutusData {
	return copyData(c, false)
}

func (c Constr) Equal(pd PlutusData) bool {
	return equalData(c, pd)
}

func (c Constr) String() string {
	var sb strings.Builder
	writePlutusDataString(&sb, &c)
	return sb.String()
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

// marshalPlutusDataCBOR returns the CBOR encoding of pd.
func marshalPlutusDataCBOR(pd PlutusData) ([]byte, error) {
	var buf bytes.Buffer
	if err := appendPlutusDataCBOR(&buf, pd); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeAs decodes data, which must hold exactly one item, and requires the
// result to be a P. Callers check the item's head first, so a mismatch is
// not expected.
func decodeAs[P PlutusData](data []byte, allowValue bool) (P, error) {
	var zero P
	decoded, err := decodeAll(nil, data, allowValue)
	if err != nil {
		return zero, err
	}
	typed, ok := decoded.(P)
	if !ok {
		return zero, fmt.Errorf("unexpected PlutusData type %T", decoded)
	}
	return typed, nil
}

// Map

type Map struct {
	Pairs    [][2]PlutusData // Each pair is [key, value]
	useIndef *bool
}

func (Map) isPlutusData() {}

func (m *Map) UnmarshalCBOR(data []byte) error {
	if _, _, _, err := decodeCBORMap(data); err != nil {
		return err
	}
	tmpMap, err := decodeAs[*Map](data, false)
	if err != nil {
		return err
	}
	*m = *tmpMap
	return nil
}

func (m Map) MarshalCBOR() ([]byte, error) {
	return marshalPlutusDataCBOR(m)
}

// appendCBORHead writes the map's head and returns the frame that writes
// its pairs.
func (m Map) appendCBORHead(buf *bytes.Buffer) *encodeFrame {
	// Default to definite-length encoding to match Haskell's canonical CBOR.
	// If useIndef is explicitly set, honor it.
	useIndef := m.useIndef != nil && *m.useIndef
	if useIndef {
		buf.WriteByte(CborTypeMap | CborIndefFlag)
	} else {
		appendCBORHead(buf, CborTypeMap, uint64(len(m.Pairs)))
	}
	return &encodeFrame{
		children: childFrame{pairs: m.Pairs},
		indef:    useIndef,
		desc:     "map",
	}
}

func (m Map) Clone() PlutusData {
	return copyData(m, false)
}

func (m Map) Equal(pd PlutusData) bool {
	return equalData(m, pd)
}

func (m Map) String() string {
	var sb strings.Builder
	writePlutusDataString(&sb, &m)
	return sb.String()
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
	tag, _, err := decodeCBORTag(encoded)
	if err != nil {
		return err
	}
	if tag != valueCBORTag {
		return fmt.Errorf("unexpected CBOR tag for PlutusData Value: %d", tag)
	}
	value, err := decodeAs[*Value](encoded, true)
	if err != nil {
		return err
	}
	*v = *value
	return nil
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
	return copyData(v, false)
}

func (v Value) Equal(pd PlutusData) bool {
	return equalData(v, pd)
}

func (v Value) String() string {
	var sb strings.Builder
	writePlutusDataString(&sb, &v)
	return sb.String()
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
	var (
		decoded *Integer
		rest    []byte
		err     error
	)
	switch {
	case len(data) == 0:
		return errors.New("empty data")
	case isBignumHead(data[0]):
		decoded, rest, err = (*Decoder)(nil).decodeBignum(uint64(data[0]-CborTypeTag), data[1:])
	case data[0]&CborTypeMask == CborTypeUnsignedInt, data[0]&CborTypeMask == CborTypeNegativeInt:
		decoded, rest, err = (*Decoder)(nil).decodeSmallInteger(data)
	default:
		return fmt.Errorf("unsupported CBOR head 0x%02x for PlutusData Integer", data[0])
	}
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	i.Inner = decoded.Inner
	return nil
}

func (i Integer) MarshalCBOR() ([]byte, error) {
	// Plutus encodes an integer inside the Word64 range, or its negative
	// mirror, as a plain CBOR integer, and any other integer as a CBOR bignum
	// whose magnitude byte string obeys the same MaxByteStringLeafSize
	// chunking rule as a Plutus byte string. See PlutusCore.Data.encodeInteger.
	if i.Inner == nil {
		return nil, ErrNilInteger
	}
	var buf bytes.Buffer
	appendIntegerCBOR(&buf, i.Inner)
	return buf.Bytes(), nil
}

// appendIntegerCBOR writes n as Plutus encodes an integer (see
// Integer.MarshalCBOR).
func appendIntegerCBOR(buf *bytes.Buffer, n *big.Int) {
	if n.IsUint64() {
		appendCBORHead(buf, CborTypeUnsignedInt, n.Uint64())
		return
	}
	tag := byte(cborTagPositiveBignum)
	major := CborTypeUnsignedInt
	magnitude := n
	if n.Sign() < 0 {
		// A negative integer encodes -1 - n, which is big.Int.Not for a
		// negative operand.
		tag = byte(cborTagNegativeBignum)
		major = CborTypeNegativeInt
		magnitude = new(big.Int).Not(n)
	}
	if magnitude.IsUint64() {
		appendCBORHead(buf, major, magnitude.Uint64())
		return
	}
	buf.WriteByte(tag)
	appendDataByteString(buf, magnitude.Bytes())
}

// ErrNilInteger is returned when encoding an Integer whose Inner is nil.
var ErrNilInteger = errors.New("PlutusData Integer has nil value")

// value returns Inner, treating a nil Inner as zero so that the non-encoding
// methods (Clone, Equal, String) never panic on a zero-value Integer.
func (i Integer) value() *big.Int {
	if i.Inner == nil {
		return new(big.Int)
	}
	return i.Inner
}

func (i Integer) Clone() PlutusData {
	tmpVal := new(big.Int).Set(i.value())
	return &Integer{tmpVal}
}

func (i Integer) Equal(pd PlutusData) bool {
	return i.equalLeaf(pd)
}

// String renders the integer in decimal. A nil Inner renders as zero.
//
// The cost of decimal conversion is inherent to math/big and is not
// improved on here: Go's big.Int.String is already a divide-and-conquer
// conversion (precomputed powers of the radix, recursive division), so a
// hand-rolled version using Quo/DivMod would be no faster; its cost is
// dominated by Karatsuba multiplication/division, roughly O(n^1.6) in the
// size of the magnitude. Measured on arm64 with Go 1.26: 128 KiB takes about
// 0.13s, 512 KiB about 1s, 2 MiB about 9s, and a 4 MiB bignum (the size of a
// CBOR bignum in a 16 MiB document) tens of seconds. Nothing inside this
// module renders attacker-sized integers: evaluator errors report only sign
// and bit length (cek integerMetadata), the parser reports the (digit
// capped) source literal, and the JSON decoder caps decimal digit counts
// (see MaxJSONIntegerDigits). Callers that print or JSON-encode untrusted,
// unbounded integers must bound the size first, for example by checking
// Inner.BitLen().
func (i Integer) String() string {
	var sb strings.Builder
	writeIntegerString(&sb, &i)
	return sb.String()
}

// NewInteger creates a new Integer variant.
func NewInteger(value *big.Int) PlutusData {
	tmpVal := new(big.Int)
	if value != nil {
		tmpVal.Set(value)
	}
	return &Integer{tmpVal}
}

// ByteString

type ByteString struct {
	Inner []byte
}

func (ByteString) isPlutusData() {}

func (b *ByteString) UnmarshalCBOR(data []byte) error {
	inner, rest, err := (*Decoder)(nil).decodeByteStringContent(data)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	b.Inner = inner
	return nil
}

func (b ByteString) MarshalCBOR() ([]byte, error) {
	var buf bytes.Buffer
	appendDataByteString(&buf, b.Inner)
	return buf.Bytes(), nil
}

// appendDataByteString writes b the way Plutus' Data encoding does:
// definite-length up to MaxByteStringLeafSize bytes, and otherwise an
// indefinite-length byte string split into chunks of that size. This applies
// both to a ByteString and to the magnitude of a bignum-encoded Integer. See
// PlutusCore.Data.encodeBs.
func appendDataByteString(buf *bytes.Buffer, b []byte) {
	if len(b) <= MaxByteStringLeafSize {
		appendCBORHead(buf, CborTypeByteString, uint64(len(b)))
		buf.Write(b)
		return
	}
	// Indefinite-length byte string with MaxByteStringLeafSize chunks
	buf.WriteByte(0x5f) // Start indefinite-length byte string
	for i := 0; i < len(b); i += MaxByteStringLeafSize {
		end := min(i+MaxByteStringLeafSize, len(b))
		appendCBORHead(buf, CborTypeByteString, uint64(end-i))
		buf.Write(b[i:end])
	}
	buf.WriteByte(0xff) // End indefinite-length byte string
}

func (b ByteString) Clone() PlutusData {
	tmpVal := make([]byte, len(b.Inner))
	copy(tmpVal, b.Inner)
	return &ByteString{tmpVal}
}

func (b ByteString) Equal(pd PlutusData) bool {
	return b.equalLeaf(pd)
}

func (b ByteString) String() string {
	var sb strings.Builder
	writeByteStringString(&sb, &b)
	return sb.String()
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
	if _, _, _, err := decodeCBORArray(data); err != nil {
		return err
	}
	tmpList, err := decodeAs[*List](data, false)
	if err != nil {
		return err
	}
	*l = *tmpList
	return nil
}

func (l List) MarshalCBOR() ([]byte, error) {
	return marshalPlutusDataCBOR(l)
}

// appendCBORHead writes the list's head and returns the frame that writes
// its items.
func (l List) appendCBORHead(buf *bytes.Buffer) *encodeFrame {
	// Determine whether to use indefinite-length encoding.
	// If useIndef is explicitly set, honor it; otherwise default to
	// Haskell's cborg behavior: indefinite for non-empty, definite for empty.
	useIndef := len(l.Items) > 0
	if l.useIndef != nil {
		useIndef = *l.useIndef
	}
	return arrayFrame(buf, l.Items, useIndef, "list item")
}

func (l List) Clone() PlutusData {
	return copyData(l, false)
}

func (l List) Equal(pd PlutusData) bool {
	return equalData(l, pd)
}

func (l List) String() string {
	var sb strings.Builder
	writePlutusDataString(&sb, &l)
	return sb.String()
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
