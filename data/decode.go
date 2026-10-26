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
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync/atomic"
)

const (
	CborTypeUnsignedInt uint8 = 0
	CborTypeNegativeInt uint8 = 0x20
	CborTypeByteString  uint8 = 0x40
	CborTypeTextString  uint8 = 0x60
	CborTypeArray       uint8 = 0x80
	CborTypeMap         uint8 = 0xa0
	CborTypeTag         uint8 = 0xc0
	CborTypeSimple      uint8 = 0xe0

	// Only the top 3 bytes are used to specify the type
	CborTypeMask uint8 = 0xe0

	CborIndefFlag uint8 = 0x1f

	MaxByteStringLeafSize = 64

	// CBOR bignum tags, used for any integer outside the Word64 range and its
	// negative mirror.
	cborTagPositiveBignum = 0xc2
	cborTagNegativeBignum = 0xc3

	// MaxDecodeNodes bounds the number of PlutusData values in one JSON
	// document (see DecodeJSON).
	//
	// It no longer applies to CBOR: the reference Data decoder
	// (PlutusCore.Data.decodeData, used for datums, redeemers and Data
	// constants) has no node limit, and a Cardano node accepts any number of
	// nodes that fits in a transaction. The CBOR decoders accept the same;
	// every node occupies at least one input byte, so their work and memory
	// stay linear in the input size.
	MaxDecodeNodes = 1_000_000

	minDecodeNestingDepth      = 4
	maxDecodeNestingDepthLimit = 65535
	defaultDecodeNestingDepth  = 16384
)

var maxDecodeNestingDepth atomic.Int64

func init() {
	maxDecodeNestingDepth.Store(defaultDecodeNestingDepth)
}

// MaxDecodeNestingDepth returns the nesting depth limit of the PlutusData
// JSON decoder (see SetMaxDecodeNestingDepth).
//
// Deprecated: the CBOR decoders no longer have a nesting limit. The
// reference Data decoder has none, and a Cardano node accepts datums,
// redeemers and Data constants nested as deep as a transaction allows (one
// byte per level), so the CBOR decoders accept any depth; they decode
// iteratively, so depth cannot exhaust the goroutine stack. The value still
// bounds the recursive JSON decoder, which is not used by the ledger.
func MaxDecodeNestingDepth() int {
	return int(maxDecodeNestingDepth.Load())
}

// SetMaxDecodeNestingDepth changes the nesting depth limit of the PlutusData
// JSON decoder. depth must be between 4 and 65535.
//
// Deprecated: it has no effect on CBOR decoding; see MaxDecodeNestingDepth.
func SetMaxDecodeNestingDepth(depth int) error {
	if depth < minDecodeNestingDepth || depth > maxDecodeNestingDepthLimit {
		return fmt.Errorf(
			"decode nesting depth must be between %d and %d: %d",
			minDecodeNestingDepth,
			maxDecodeNestingDepthLimit,
			depth,
		)
	}
	maxDecodeNestingDepth.Store(int64(depth))
	return nil
}

// DecodeLimitError reports that PlutusData exceeded a structural limit: a
// CBOR byte string chunk longer than MaxByteStringLeafSize bytes (a rule of
// the reference decoder), or a limit passed to CloneContext.
type DecodeLimitError struct {
	Limit  string
	Max    int
	Actual int
}

func (e *DecodeLimitError) Error() string {
	return fmt.Sprintf(
		"PlutusData CBOR %s limit exceeded: %d > %d",
		e.Limit,
		e.Actual,
		e.Max,
	)
}

// Decode decodes a CBOR-encoded byte slice into a PlutusData value.
// It returns an error if the input is invalid or not a valid PlutusData encoding.
//
// Decode accepts what the reference decoder (PlutusCore.Data.decodeData)
// accepts, at any nesting depth and node count. It runs in time and memory
// linear in len(b) and does not recurse, so no input can exhaust the
// goroutine stack.
func Decode(b []byte) (PlutusData, error) {
	return wrapDecodeError(decodeAll(nil, b, false))
}

// DecodeWithValue is Decode that also admits the tag 1401 Value
// constructor (the Plutus V4 Value data encoding of this package), which
// Decode rejects as the reference Data decoder does. Use it only where Value
// data is meant to be accepted.
func DecodeWithValue(b []byte) (PlutusData, error) {
	return wrapDecodeError(decodeAll(nil, b, true))
}

// isBignumHead reports whether head, the first byte of a tagged CBOR item, is
// a bignum: tag 2 or 3 in its one-byte form. The reference decoder
// (cborg's tokenType, used by PlutusCore.Data.decodeData) only classifies
// the heads 0xc2 and 0xc3 as integers; tag 2 or 3 written with a longer head
// is a generic tag there, which decodeData rejects as an unknown constructor.
func isBignumHead(head byte) bool {
	return head == cborTagPositiveBignum || head == cborTagNegativeBignum
}

// DecodePrefix decodes the PlutusData item at the start of b and ignores any
// bytes that follow it, which need not be valid CBOR. It is the decoding the
// reference applies to a Data constant inside a FLAT program
// (FlatViaSerialise, through serialise's deserialiseOrFail, which discards
// the input left after the first item). Use Decode everywhere else.
//
// Like DecodeWithValue, it admits the tag 1401 Value constructor: callers
// gate Value data by ledger language (syn.ValidateProgram).
func DecodePrefix(b []byte) (PlutusData, error) {
	return wrapDecodeError(decodePrefix(nil, b))
}

// wrapDecodeError adds the context shared by the exported CBOR decoders.
func wrapDecodeError(v PlutusData, err error) (PlutusData, error) {
	if err != nil {
		return nil, fmt.Errorf("failed to decode CBOR: %w", err)
	}
	return v, nil
}

// decodePrefix decodes the item at the start of data, admitting Value, and
// drops the bytes that follow it.
func decodePrefix(d *Decoder, data []byte) (PlutusData, error) {
	v, _, err := decodeItem(d, data, true)
	return v, err
}

// decodeAll decodes one item and requires it to span all of data.
func decodeAll(d *Decoder, data []byte, allowValue bool) (PlutusData, error) {
	v, rest, err := decodeItem(d, data, allowValue)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	return v, nil
}

func isConstrTag(tag uint64) bool {
	return tag == 102 || (tag >= 121 && tag <= 127) || (tag >= 1280 && tag <= 1400)
}

// Container kinds of a decodeFrame.
const (
	frameList uint8 = iota
	frameMap
	frameConstr
	// frameValue is the map inside a tag 1401 Value.
	frameValue
)

// decodeFrame is an open container on the decoder's explicit stack. Its
// decoded children sit on the shared value stack from index start.
type decodeFrame struct {
	start int
	// remaining is the number of child items still expected by a
	// definite-length container (two per map pair), or -1 for an
	// indefinite-length one, which ends at a break byte.
	remaining int
	tag       uint64
	kind      uint8
	indef     bool
	// outerIndef marks a tag 102 constructor whose [alternative, fields]
	// array is indefinite-length and so needs a break after the fields.
	outerIndef bool
}

// decodeItem decodes the PlutusData item at the start of data and returns
// it with the bytes that follow it. It is the single decoder behind every
// CBOR entry point. d selects arena allocation; nil allocates each value on
// the heap.
//
// The reference decoder imposes no depth or size limit beyond the input
// itself, so neither does this one. It never recurses: open containers live
// on an explicit stack (one small frame per level) and decoded children on a
// shared value stack until their container completes, so stack use is
// constant and memory linear in len(data): every container and every leaf
// occupies at least one input byte, and a definite-length header's count is
// checked against the bytes left before anything is sized from it.
func decodeItem(
	d *Decoder,
	data []byte,
	allowValue bool,
) (PlutusData, []byte, error) {
	frames := make([]decodeFrame, 0, 16)
	values := make([]PlutusData, 0, 32)
	rest := data
	for {
		var (
			value PlutusData
			err   error
		)
		if n := len(frames); n > 0 && frames[n-1].remaining <= 0 {
			top := &frames[n-1]
			done := top.remaining == 0
			if !done {
				// Indefinite length: the container ends at a break byte
				// where its next item would start.
				if len(rest) == 0 {
					if top.kind == frameMap || top.kind == frameValue {
						return nil, nil, errors.New("unterminated indefinite-length CBOR map")
					}
					return nil, nil, errors.New("unterminated indefinite-length CBOR array")
				}
				if rest[0] == 0xff {
					if (top.kind == frameMap || top.kind == frameValue) &&
						(len(values)-top.start)%2 != 0 {
						return nil, nil, errors.New("unexpected CBOR break")
					}
					rest = rest[1:]
					done = true
				}
			}
			if done {
				value, rest, err = d.closeFrame(top, values[top.start:], rest)
				if err != nil {
					return nil, nil, err
				}
				clear(values[top.start:])
				values = values[:top.start]
				frames = frames[:n-1]
			}
		}
		if value == nil {
			var frame decodeFrame
			var opened bool
			value, frame, opened, rest, err = d.decodeHead(rest, allowValue)
			if err != nil {
				return nil, nil, err
			}
			if opened {
				frame.start = len(values)
				frames = append(frames, frame)
				continue
			}
		}
		if len(frames) == 0 {
			return value, rest, nil
		}
		if top := &frames[len(frames)-1]; top.remaining > 0 {
			top.remaining--
		}
		values = append(values, value)
	}
}

// decodeHead decodes the item at the start of data. A leaf is returned as
// value; a container head is returned as an open frame for the caller to
// push, with opened set.
func (d *Decoder) decodeHead(
	data []byte,
	allowValue bool,
) (PlutusData, decodeFrame, bool, []byte, error) {
	var frame decodeFrame
	if len(data) == 0 {
		return nil, frame, false, nil, errors.New("empty data")
	}
	switch data[0] & CborTypeMask {
	case CborTypeArray:
		count, rest, indef, err := decodeCBORArray(data)
		if err != nil {
			return nil, frame, false, nil, err
		}
		frame, err = openSequence(frameList, count, indef, rest)
		return nil, frame, err == nil, rest, err
	case CborTypeMap:
		frame, rest, err := openMap(frameMap, data)
		return nil, frame, err == nil, rest, err
	case CborTypeTag:
		tag, content, err := decodeCBORTag(data)
		if err != nil {
			return nil, frame, false, nil, err
		}
		switch {
		case isConstrTag(tag):
			frame, rest, err := openConstr(tag, content)
			return nil, frame, err == nil, rest, err
		case tag == valueCBORTag && allowValue:
			frame, rest, err := openMap(frameValue, content)
			return nil, frame, err == nil, rest, err
		case isBignumHead(data[0]):
			value, rest, err := d.decodeBignum(tag, content)
			return value, frame, false, rest, err
		default:
			return nil, frame, false, nil, fmt.Errorf(
				"unknown CBOR tag for PlutusData: %d",
				tag,
			)
		}
	case CborTypeUnsignedInt, CborTypeNegativeInt:
		value, rest, err := d.decodeSmallInteger(data)
		return value, frame, false, rest, err
	case CborTypeByteString:
		inner, rest, err := d.decodeByteStringContent(data)
		if err != nil {
			return nil, frame, false, nil, err
		}
		return d.allocByteString(inner), frame, false, rest, nil
	default:
		if data[0] == 0xff {
			return nil, frame, false, nil, errors.New("unexpected CBOR break")
		}
		return nil, frame, false, nil, fmt.Errorf(
			"unsupported CBOR major type 0x%02x for PlutusData",
			data[0]&CborTypeMask,
		)
	}
}

// openSequence opens a list-like container of count items (indef: ended by a
// break) whose items start at rest.
func openSequence(kind uint8, count int, indef bool, rest []byte) (decodeFrame, error) {
	if indef {
		return decodeFrame{kind: kind, indef: true, remaining: -1}, nil
	}
	// Each item needs at least 1 byte for a CBOR head.
	if count > len(rest) {
		return decodeFrame{}, fmt.Errorf(
			"CBOR array claims %d items but only %d bytes remain",
			count,
			len(rest),
		)
	}
	return decodeFrame{kind: kind, remaining: count}, nil
}

// openMap opens the map at the start of data as a frame of the given kind.
func openMap(kind uint8, data []byte) (decodeFrame, []byte, error) {
	pairCount, rest, indef, err := decodeCBORMap(data)
	if err != nil {
		return decodeFrame{}, nil, err
	}
	if indef {
		return decodeFrame{kind: kind, indef: true, remaining: -1}, rest, nil
	}
	// Each pair needs at least 2 bytes (one per key/value CBOR head).
	if pairCount > len(rest)/2 {
		return decodeFrame{}, nil, fmt.Errorf(
			"CBOR map claims %d pairs but only %d bytes remain",
			pairCount,
			len(rest),
		)
	}
	return decodeFrame{kind: kind, remaining: pairCount * 2}, rest, nil
}

// openConstr opens the constructor whose tag content starts at data.
func openConstr(tag uint64, data []byte) (decodeFrame, []byte, error) {
	var (
		alternative uint64
		outerIndef  bool
	)
	switch {
	case tag >= 121 && tag <= 127:
		alternative = tag - 121
	case tag >= 1280 && tag <= 1400:
		alternative = tag - 1280 + 7
	default: // 102: [alternative, fields]
		fieldCount, rest, indef, err := decodeCBORArray(data)
		if err != nil {
			return decodeFrame{}, nil, err
		}
		if !indef && fieldCount != 2 {
			return decodeFrame{}, nil, fmt.Errorf(
				"constructor 102 outer array has %d items, want 2",
				fieldCount,
			)
		}
		alternative, data, err = decodeCBORUint(rest)
		if err != nil {
			return decodeFrame{}, nil, err
		}
		outerIndef = indef
	}
	count, rest, indef, err := decodeCBORArray(data)
	if err != nil {
		return decodeFrame{}, nil, err
	}
	frame, err := openSequence(frameConstr, count, indef, rest)
	if err != nil {
		return decodeFrame{}, nil, err
	}
	frame.tag = alternative
	frame.outerIndef = outerIndef
	return frame, rest, nil
}

// closeFrame builds the container of a completed frame from its decoded
// children. rest follows the container's last byte.
func (d *Decoder) closeFrame(
	frame *decodeFrame,
	children []PlutusData,
	rest []byte,
) (PlutusData, []byte, error) {
	switch frame.kind {
	case frameList:
		list := d.allocList()
		list.Items = d.copyItems(children)
		list.useIndef = useIndefPtr(frame.indef)
		return list, rest, nil
	case frameConstr:
		if frame.outerIndef {
			if len(rest) == 0 || rest[0] != 0xff {
				return nil, nil, errors.New("unterminated indefinite-length CBOR array")
			}
			rest = rest[1:]
		}
		constr := d.allocConstr()
		constr.Tag = d.allocBigInt().SetUint64(frame.tag)
		constr.Fields = d.copyItems(children)
		constr.useIndef = useIndefPtr(frame.indef)
		return constr, rest, nil
	default: // frameMap, frameValue
		pairs := d.allocPairs(len(children) / 2)
		for i := range pairs {
			pairs[i] = [2]PlutusData{children[2*i], children[2*i+1]}
		}
		decoded := d.allocMap()
		decoded.Pairs = pairs
		decoded.useIndef = useIndefPtr(frame.indef)
		if frame.kind == frameMap {
			return decoded, rest, nil
		}
		if err := (&Value{Inner: decoded}).Validate(); err != nil {
			return nil, nil, err
		}
		value := d.allocValue()
		value.Inner = decoded
		return value, rest, nil
	}
}

var bigIntOne = big.NewInt(1)

// decodeSmallInteger decodes a CBOR major type 0 or 1 integer.
func (d *Decoder) decodeSmallInteger(data []byte) (*Integer, []byte, error) {
	cborType, value, rest, indefinite, err := decodeCBORHead(data)
	if err != nil {
		return nil, nil, err
	}
	if indefinite {
		return nil, nil, errors.New("indefinite CBOR integer is not supported")
	}
	inner := d.allocBigInt().SetUint64(value)
	if cborType == CborTypeNegativeInt {
		inner.Add(inner, bigIntOne)
		inner.Neg(inner)
	}
	return d.allocInteger(inner), rest, nil
}

// decodeBignum decodes the byte string content of a bignum tag (2 or 3).
func (d *Decoder) decodeBignum(tag uint64, content []byte) (*Integer, []byte, error) {
	magnitude, rest, err := d.decodeByteStringContent(content)
	if err != nil {
		return nil, nil, err
	}
	inner := d.allocBigInt().SetBytes(magnitude)
	if tag == 3 {
		inner.Add(inner, bigIntOne)
		inner.Neg(inner)
	}
	return d.allocInteger(inner), rest, nil
}

// decodeByteStringContent decodes the byte string at the start of data,
// definite-length or chunked, with every chunk at most
// MaxByteStringLeafSize bytes as the reference decoder requires.
func (d *Decoder) decodeByteStringContent(data []byte) ([]byte, []byte, error) {
	value, rest, indefinite, err := decodeCBORHeadValue(data, CborTypeByteString)
	if err != nil {
		return nil, nil, err
	}
	if !indefinite {
		if err := checkByteStringLeafSize(value); err != nil {
			return nil, nil, err
		}
		if value > uint64(len(rest)) {
			return nil, nil, errors.New("truncated CBOR payload")
		}
		inner := d.allocBytes(int(value))
		copy(inner, rest[:value])
		return inner, rest[value:], nil
	}

	total := 0
	scan := rest
	for {
		if len(scan) == 0 {
			return nil, nil, errors.New("unterminated indefinite-length CBOR string")
		}
		if scan[0] == 0xff {
			break
		}
		chunkType, chunkValue, chunkRest, chunkIndef, err := decodeCBORHead(scan)
		if err != nil {
			return nil, nil, err
		}
		if chunkIndef || chunkType != CborTypeByteString {
			return nil, nil, fmt.Errorf("invalid CBOR chunk type 0x%02x", chunkType)
		}
		if err := checkByteStringLeafSize(chunkValue); err != nil {
			return nil, nil, err
		}
		if chunkValue > uint64(len(chunkRest)) {
			return nil, nil, errors.New("truncated CBOR payload")
		}
		total += int(chunkValue)
		scan = chunkRest[chunkValue:]
	}

	inner := d.allocBytes(total)
	offset := 0
	for {
		if rest[0] == 0xff {
			return inner, rest[1:], nil
		}
		_, chunkValue, chunkRest, _, err := decodeCBORHead(rest)
		if err != nil {
			return nil, nil, err
		}
		copy(inner[offset:], chunkRest[:chunkValue])
		offset += int(chunkValue)
		rest = chunkRest[chunkValue:]
	}
}

func decodeCBORTag(data []byte) (uint64, []byte, error) {
	value, rest, indefinite, err := decodeCBORHeadValue(data, CborTypeTag)
	if err != nil {
		return 0, nil, err
	}
	if indefinite {
		return 0, nil, errors.New("indefinite CBOR tags are not supported")
	}
	return value, rest, nil
}

func decodeCBORArray(data []byte) (int, []byte, bool, error) {
	value, rest, indefinite, err := decodeCBORHeadValue(data, CborTypeArray)
	if err != nil {
		return 0, nil, false, err
	}
	if indefinite {
		return 0, rest, true, nil
	}

	if value > math.MaxInt {
		return 0, nil, false, fmt.Errorf("CBOR array too large: %d", value)
	}
	return int(value), rest, false, nil
}

func decodeCBORMap(data []byte) (int, []byte, bool, error) {
	value, rest, indefinite, err := decodeCBORHeadValue(data, CborTypeMap)
	if err != nil {
		return 0, nil, false, err
	}
	if indefinite {
		return 0, rest, true, nil
	}

	if value > math.MaxInt/2 {
		return 0, nil, false, fmt.Errorf("CBOR map too large: %d", value)
	}
	return int(value), rest, false, nil
}

func decodeCBORUint(data []byte) (uint64, []byte, error) {
	cborType, value, rest, indefinite, err := decodeCBORHead(data)
	if err != nil {
		return 0, nil, err
	}
	if cborType != CborTypeUnsignedInt {
		return 0, nil, fmt.Errorf(
			"expected CBOR type 0x%02x",
			CborTypeUnsignedInt,
		)
	}
	if indefinite {
		return 0, nil, errors.New("indefinite CBOR integer is not supported")
	}
	return value, rest, nil
}

func decodeCBORHeadValue(
	data []byte,
	wantType uint8,
) (uint64, []byte, bool, error) {
	cborType, value, rest, indefinite, err := decodeCBORHead(data)
	if err != nil {
		return 0, nil, false, err
	}
	if cborType != wantType {
		return 0, nil, false, fmt.Errorf("expected CBOR type 0x%02x", wantType)
	}
	return value, rest, indefinite, nil
}

func decodeCBORHead(data []byte) (uint8, uint64, []byte, bool, error) {
	if len(data) == 0 {
		return 0, 0, nil, false, errors.New("empty data")
	}
	cborType := data[0] & CborTypeMask

	additional := data[0] & 0x1f
	switch {
	case additional < 24:
		return cborType, uint64(additional), data[1:], false, nil
	case additional == 24:
		if len(data) < 2 {
			return 0, 0, nil, false, errors.New("truncated CBOR header")
		}
		return cborType, uint64(data[1]), data[2:], false, nil
	case additional == 25:
		if len(data) < 3 {
			return 0, 0, nil, false, errors.New("truncated CBOR header")
		}
		return cborType, uint64(
			binary.BigEndian.Uint16(data[1:3]),
		), data[3:], false, nil
	case additional == 26:
		if len(data) < 5 {
			return 0, 0, nil, false, errors.New("truncated CBOR header")
		}
		return cborType, uint64(
			binary.BigEndian.Uint32(data[1:5]),
		), data[5:], false, nil
	case additional == 27:
		if len(data) < 9 {
			return 0, 0, nil, false, errors.New("truncated CBOR header")
		}
		return cborType, binary.BigEndian.Uint64(
			data[1:9],
		), data[9:], false, nil
	case additional == CborIndefFlag:
		return cborType, 0, data[1:], true, nil
	default:
		return 0, 0, nil, false, fmt.Errorf(
			"invalid CBOR header: 0x%02x",
			data[0],
		)
	}
}

func checkByteStringLeafSize(size uint64) error {
	if size <= MaxByteStringLeafSize {
		return nil
	}
	actual := math.MaxInt
	if size <= uint64(math.MaxInt) {
		actual = int(size)
	}
	return &DecodeLimitError{
		Limit:  "bytestring leaf",
		Max:    MaxByteStringLeafSize,
		Actual: actual,
	}
}
