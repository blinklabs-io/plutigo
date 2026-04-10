package data

import (
	"errors"
	"fmt"
	"math"
	"math/big"
)

const (
	dataDecodeChunkSize = 256
	dataDecodeRetainCap = 2
	dataSliceRetainCap  = 4
)

type arenaChunks[S any] struct {
	chunks   [][]S
	chunkIdx int
	offset   int
}

func (a *arenaChunks[S]) used() int {
	if len(a.chunks) == 0 {
		return 0
	}
	return a.chunkIdx*dataDecodeChunkSize + a.offset
}

func (a *arenaChunks[S]) alloc() *S {
	if a.chunkIdx < len(a.chunks) {
		chunk := a.chunks[a.chunkIdx]
		if chunk != nil && a.offset < len(chunk) {
			slot := &chunk[a.offset]
			a.offset++
			return slot
		}
	}

	if nextIdx := a.chunkIdx + 1; nextIdx < len(a.chunks) {
		chunk := a.chunks[nextIdx]
		if chunk == nil {
			goto allocNewChunk
		}
		a.chunkIdx = nextIdx
		a.offset = 1
		return &chunk[0]
	}

allocNewChunk:
	chunk := make([]S, dataDecodeChunkSize)
	a.chunks = append(a.chunks, chunk)
	a.chunkIdx = len(a.chunks) - 1
	a.offset = 1
	return &chunk[0]
}

func (a *arenaChunks[S]) reset(retainCap int) {
	used := a.used()
	retained := len(a.chunks)
	if retained > retainCap {
		retained = retainCap
	}
	if used > 0 && retained > 0 {
		remaining := used
		maxRetained := retained * dataDecodeChunkSize
		if remaining > maxRetained {
			remaining = maxRetained
		}
		for i := 0; i < retained && remaining > 0; i++ {
			chunk := a.chunks[i]
			if chunk == nil {
				continue
			}
			clearCount := len(chunk)
			if remaining < clearCount {
				clearCount = remaining
			}
			clear(chunk[:clearCount])
			remaining -= clearCount
		}
	}
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
}

type arenaSlices[S any] struct {
	chunks [][]S
	pos    int
}

func (a *arenaSlices[S]) alloc(n int) []S {
	if n == 0 {
		return nil
	}

	remaining := a.pos
	for i := range a.chunks {
		chunk := a.chunks[i]
		if remaining < len(chunk) {
			if remaining+n <= len(chunk) {
				start := remaining
				a.pos += n
				return chunk[start : start+n]
			}
			a.pos += len(chunk) - remaining
			remaining = 0
			continue
		}
		remaining -= len(chunk)
	}

	size := dataDecodeChunkSize
	if n > size {
		size = n
	}
	chunk := make([]S, size)
	a.chunks = append(a.chunks, chunk)
	a.pos += n
	return chunk[:n]
}

func (a *arenaSlices[S]) reset(retainCap int) {
	retainedUsed := a.pos
	if len(a.chunks) > retainCap {
		maxRetained := 0
		for i := range retainCap {
			maxRetained += len(a.chunks[i])
		}
		if retainedUsed > maxRetained {
			retainedUsed = maxRetained
		}
	}
	remaining := retainedUsed
	for i := range a.chunks {
		if remaining <= 0 {
			break
		}
		chunk := a.chunks[i]
		if chunk == nil {
			continue
		}
		clearCount := len(chunk)
		if remaining < clearCount {
			clearCount = remaining
		}
		clear(chunk[:clearCount])
		remaining -= clearCount
	}
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.pos = 0
}

// Decoder reuses arena-backed storage for decoded PlutusData values.
// Returned values remain valid until the next Reset on the same decoder.
type Decoder struct {
	bigInts     arenaChunks[big.Int]
	integers    arenaChunks[Integer]
	byteStrings arenaChunks[ByteString]
	constrs     arenaChunks[Constr]
	lists       arenaChunks[List]
	maps        arenaChunks[Map]
	items       arenaSlices[PlutusData]
	pairs       arenaSlices[[2]PlutusData]
	bytes       arenaSlices[byte]
}

// NewDecoder returns a ready-to-use Decoder with empty internal pools.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Reset clears all internal arena pools so the Decoder can be reused; previously returned values become invalid.
func (d *Decoder) Reset() {
	d.bigInts.reset(dataDecodeRetainCap)
	d.integers.reset(dataDecodeRetainCap)
	d.byteStrings.reset(dataDecodeRetainCap)
	d.constrs.reset(dataDecodeRetainCap)
	d.lists.reset(dataDecodeRetainCap)
	d.maps.reset(dataDecodeRetainCap)
	d.items.reset(dataSliceRetainCap)
	d.pairs.reset(dataSliceRetainCap)
	d.bytes.reset(dataSliceRetainCap)
}

// Decode decodes CBOR bytes into PlutusData; the Decoder is not safe for concurrent use.
func (d *Decoder) Decode(data []byte) (PlutusData, error) {
	v, err := d.decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CBOR: %w", err)
	}
	return v, nil
}

func (d *Decoder) decode(data []byte) (PlutusData, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	cborType := data[0] & CborTypeMask
	switch cborType {
	case CborTypeUnsignedInt, CborTypeNegativeInt:
		return d.decodeInteger(data)
	case CborTypeByteString:
		return d.decodeByteString(data)
	case CborTypeArray:
		tmpList, rest, err := d.decodeListNext(data)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
		}
		return tmpList, nil
	case CborTypeMap:
		tmpMap, rest, err := d.decodeMapNext(data)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
		}
		return tmpMap, nil
	case CborTypeTag:
		tagNumber, tagContent, err := decodeCBORTag(data)
		if err != nil {
			return nil, err
		}
		switch {
		case tagNumber == 102 || (tagNumber >= 121 && tagNumber <= 127) || (tagNumber >= 1280 && tagNumber <= 1400):
			tmpConstr, rest, err := d.decodeConstrNext(tagNumber, tagContent)
			if err != nil {
				return nil, err
			}
			if len(rest) > 0 {
				return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
			}
			return tmpConstr, nil
		case tagNumber == 2 || tagNumber == 3:
			return d.decodeInteger(data)
		default:
			return nil, fmt.Errorf("unknown CBOR tag for PlutusData: %d", tagNumber)
		}
	}
	return nil, fmt.Errorf("unsupported CBOR major type 0x%02x for PlutusData", cborType)
}

func (d *Decoder) decodeNextPlutusData(data []byte) (PlutusData, []byte, error) {
	if len(data) == 0 {
		return nil, nil, errors.New("empty data")
	}

	switch data[0] & CborTypeMask {
	case CborTypeArray:
		tmpList, rest, err := d.decodeListNext(data)
		if err != nil {
			return nil, nil, err
		}
		return tmpList, rest, nil
	case CborTypeMap:
		tmpMap, rest, err := d.decodeMapNext(data)
		if err != nil {
			return nil, nil, err
		}
		return tmpMap, rest, nil
	case CborTypeTag:
		tagNumber, tagContent, err := decodeCBORTag(data)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case tagNumber == 102 || (tagNumber >= 121 && tagNumber <= 127) || (tagNumber >= 1280 && tagNumber <= 1400):
			tmpConstr, rest, err := d.decodeConstrNext(tagNumber, tagContent)
			if err != nil {
				return nil, nil, err
			}
			return tmpConstr, rest, nil
		}
	}

	item, rest, err := splitCBORItem(data)
	if err != nil {
		return nil, nil, err
	}
	tmp, err := d.decode(item)
	if err != nil {
		return nil, nil, err
	}
	return tmp, rest, nil
}

func (d *Decoder) decodeConstrNext(tagNumber uint64, data []byte) (*Constr, []byte, error) {
	constr := d.constrs.alloc()
	switch {
	case tagNumber >= 121 && tagNumber <= 127:
		tmpFields, tmpUseIndef, rest, err := d.decodeListItemsNext(data)
		if err != nil {
			return nil, nil, err
		}
		constr.Tag = uint(tagNumber) - 121
		constr.Fields = tmpFields
		constr.useIndef = tmpUseIndef
		return constr, rest, nil
	case tagNumber >= 1280 && tagNumber <= 1400:
		tmpFields, tmpUseIndef, rest, err := d.decodeListItemsNext(data)
		if err != nil {
			return nil, nil, err
		}
		constr.Tag = uint(tagNumber) - 1280 + 7
		constr.Fields = tmpFields
		constr.useIndef = tmpUseIndef
		return constr, rest, nil
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
		if alternative > math.MaxUint {
			return nil, nil, fmt.Errorf("constructor alternative too large: %d", alternative)
		}
		rest = next

		tmpFields, tmpUseIndef, next, err := d.decodeListItemsNext(rest)
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

		constr.Tag = uint(alternative)
		constr.Fields = tmpFields
		constr.useIndef = tmpUseIndef
		return constr, rest, nil
	default:
		return nil, nil, fmt.Errorf("unknown CBOR tag for PlutusData constructor: %d", tagNumber)
	}
}

func (d *Decoder) decodeMapNext(data []byte) (*Map, []byte, error) {
	pairCount, rest, useIndef, err := decodeCBORMap(data)
	if err != nil {
		return nil, nil, err
	}

	var (
		smallPairs [4][2]PlutusData
		pairs      [][2]PlutusData
		pairLen    int
	)
	if !useIndef {
		if pairCount > len(rest)/2 {
			return nil, nil, fmt.Errorf("CBOR map claims %d pairs but only %d bytes remain", pairCount, len(rest))
		}
		pairs = d.pairs.alloc(pairCount)
		pairLen = pairCount
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

		tmpKey, next, err := d.decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next

		tmpVal, next, err := d.decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next

		pair := [2]PlutusData{tmpKey, tmpVal}
		if !useIndef {
			pairs[i] = pair
			continue
		}
		if pairs != nil {
			if pairLen == len(pairs) {
				grown := d.pairs.alloc(max(pairLen*2, pairLen+1))
				copy(grown, pairs[:pairLen])
				pairs = grown
			}
			pairs[pairLen] = pair
			pairLen++
			continue
		}
		if pairLen < len(smallPairs) {
			smallPairs[pairLen] = pair
			pairLen++
			continue
		}
		pairs = d.pairs.alloc(pairLen * 2)
		copy(pairs, smallPairs[:pairLen])
		pairs[pairLen] = pair
		pairLen++
	}

	if useIndef {
		if pairs == nil {
			pairs = d.pairs.alloc(pairLen)
			copy(pairs, smallPairs[:pairLen])
		} else {
			pairs = pairs[:pairLen]
		}
	}

	decoded := d.maps.alloc()
	decoded.Pairs = pairs
	decoded.useIndef = useIndefPtr(useIndef)
	return decoded, rest, nil
}

func (d *Decoder) decodeListNext(data []byte) (*List, []byte, error) {
	tmpItems, tmpUseIndef, rest, err := d.decodeListItemsNext(data)
	if err != nil {
		return nil, nil, err
	}
	decoded := d.lists.alloc()
	decoded.Items = tmpItems
	decoded.useIndef = tmpUseIndef
	return decoded, rest, nil
}

func (d *Decoder) decodeListItemsNext(data []byte) ([]PlutusData, *bool, []byte, error) {
	itemCount, rest, useIndef, err := decodeCBORArray(data)
	if err != nil {
		return nil, nil, nil, err
	}

	if !useIndef {
		tmpItems, rest, err := d.decodeListItemsDefinite(itemCount, rest)
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

		tmp, next, err := d.decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, nil, err
		}
		rest = next

		if tmpItems != nil {
			if tmpLen == len(tmpItems) {
				grown := d.items.alloc(max(tmpLen*2, tmpLen+1))
				copy(grown, tmpItems[:tmpLen])
				tmpItems = grown
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
		tmpItems = d.items.alloc(tmpLen * 2)
		copy(tmpItems, smallItems[:tmpLen])
		tmpItems[tmpLen] = tmp
		tmpLen++
	}

	if tmpItems == nil {
		tmpItems = d.items.alloc(tmpLen)
		copy(tmpItems, smallItems[:tmpLen])
	} else {
		tmpItems = tmpItems[:tmpLen]
	}
	return tmpItems, useIndefPtr(true), rest, nil
}

func (d *Decoder) decodeListItemsDefinite(itemCount int, rest []byte) ([]PlutusData, []byte, error) {
	if itemCount > len(rest) {
		return nil, nil, fmt.Errorf("CBOR array claims %d items but only %d bytes remain", itemCount, len(rest))
	}
	tmpItems := d.items.alloc(itemCount)
	for i := range itemCount {
		tmp, next, err := d.decodeNextPlutusData(rest)
		if err != nil {
			return nil, nil, err
		}
		rest = next
		tmpItems[i] = tmp
	}
	return tmpItems, rest, nil
}

var bigIntOne = big.NewInt(1)

func (d *Decoder) allocBigInt() *big.Int {
	return d.bigInts.alloc()
}

func (d *Decoder) allocInteger(inner *big.Int) *Integer {
	integer := d.integers.alloc()
	integer.Inner = inner
	return integer
}

func (d *Decoder) decodeInteger(data []byte) (*Integer, error) {
	cborType, value, rest, indefinite, err := decodeCBORHead(data)
	if err != nil {
		return nil, err
	}

	switch cborType {
	case CborTypeUnsignedInt:
		if indefinite {
			return nil, errors.New("indefinite CBOR integer is not supported")
		}
		if len(rest) > 0 {
			return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
		}
		inner := d.allocBigInt()
		inner.SetUint64(value)
		return d.allocInteger(inner), nil
	case CborTypeNegativeInt:
		if indefinite {
			return nil, errors.New("indefinite CBOR integer is not supported")
		}
		if len(rest) > 0 {
			return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
		}
		inner := d.allocBigInt()
		inner.SetUint64(value)
		inner.Add(inner, bigIntOne)
		inner.Neg(inner)
		return d.allocInteger(inner), nil
	case CborTypeTag:
		if indefinite {
			return nil, errors.New("indefinite CBOR tags are not supported")
		}
		if value != 2 && value != 3 {
			return nil, fmt.Errorf("unknown CBOR tag for PlutusData: %d", value)
		}
		bytes, rest, err := d.decodeByteStringContent(rest)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
		}
		inner := d.allocBigInt()
		inner.SetBytes(bytes)
		if value == 3 {
			inner.Add(inner, bigIntOne)
			inner.Neg(inner)
		}
		return d.allocInteger(inner), nil
	default:
		return nil, fmt.Errorf("unsupported CBOR type for PlutusData integer: 0x%02x", cborType)
	}
}

func (d *Decoder) decodeByteString(data []byte) (*ByteString, error) {
	inner, rest, err := d.decodeByteStringContent(data)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, fmt.Errorf("unexpected %d trailing bytes", len(rest))
	}
	value := d.byteStrings.alloc()
	value.Inner = inner
	return value, nil
}

func (d *Decoder) decodeByteStringContent(data []byte) ([]byte, []byte, error) {
	value, rest, indefinite, err := decodeCBORHeadValue(data, CborTypeByteString)
	if err != nil {
		return nil, nil, err
	}
	if !indefinite {
		if value > uint64(len(rest)) {
			return nil, nil, errors.New("truncated CBOR payload")
		}
		inner := d.bytes.alloc(int(value))
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
		if chunkValue > uint64(len(chunkRest)) {
			return nil, nil, errors.New("truncated CBOR payload")
		}
		if chunkValue > uint64(math.MaxInt-total) {
			return nil, nil, fmt.Errorf("CBOR byte string too large: %d", chunkValue)
		}
		total += int(chunkValue)
		scan = chunkRest[chunkValue:]
	}

	inner := d.bytes.alloc(total)
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
