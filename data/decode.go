package data

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/fxamacker/cbor/v2"
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
)

// decMode is cached at package level to avoid recreation on every decode call
var decMode cbor.DecMode

func init() {
	decOptions := cbor.DecOptions{
		// This defaults to 32, but there are blocks in the wild using >64 nested levels
		MaxNestedLevels: 256,
	}
	var err error
	decMode, err = decOptions.DecMode()
	if err != nil {
		panic("failed to initialize CBOR decoder: " + err.Error())
	}
}

// Decode decodes a CBOR-encoded byte slice into a PlutusData value.
// It returns an error if the input is invalid or not a valid PlutusData encoding.
func Decode(b []byte) (PlutusData, error) {
	v, err := decode(b)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CBOR: %w", err)
	}
	return v, nil
}

// cborUnmarshal acts like cbor.Unmarshal but allows us to set our own decoder options
func cborUnmarshal(dataBytes []byte, dest any) error {
	dm := decMode
	if dm == nil {
		panic("CBOR decoder not initialized")
	}
	return dm.Unmarshal(dataBytes, dest)
}

// decode is a low-level decode function that detects the CBOR type and uses the correct
// PlutusData type to decode it
func decode(data []byte) (PlutusData, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	cborType := data[0] & CborTypeMask
	switch cborType {
	case CborTypeUnsignedInt, CborTypeNegativeInt:
		var tmpData Integer
		if err := cborUnmarshal(data, &tmpData); err != nil {
			return nil, err
		}
		return &tmpData, nil
	case CborTypeByteString:
		var tmpData ByteString
		if err := cborUnmarshal(data, &tmpData); err != nil {
			return nil, err
		}
		return &tmpData, nil
	case CborTypeArray:
		var tmpData List
		if err := tmpData.UnmarshalCBOR(data); err != nil {
			return nil, err
		}
		return &tmpData, nil
	case CborTypeMap:
		var tmpData Map
		if err := tmpData.UnmarshalCBOR(data); err != nil {
			return nil, err
		}
		return &tmpData, nil
	case CborTypeTag:
		tagNumber, _, err := decodeCBORTag(data)
		if err != nil {
			return nil, err
		}
		switch {
		// Constr
		case tagNumber == 102 || (tagNumber >= 121 && tagNumber <= 127) || (tagNumber >= 1280 && tagNumber <= 1400):
			var tmpConstr Constr
			if err := tmpConstr.UnmarshalCBOR(data); err != nil {
				return nil, err
			}
			return &tmpConstr, nil

		case tagNumber == 2 || tagNumber == 3:
			var tmpData Integer
			if err := cborUnmarshal(data, &tmpData); err != nil {
				return nil, err
			}
			return &tmpData, nil

		default:
			return nil, fmt.Errorf(
				"unknown CBOR tag for PlutusData: %d",
				tagNumber,
			)
		}
	}
	var tmpData any
	if err := cborUnmarshal(data, &tmpData); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf(
		"unsupported CBOR type for PlutusData: %T",
		tmpData,
	)
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

	if value > math.MaxInt {
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
		return 0, nil, fmt.Errorf("expected CBOR type 0x%02x", CborTypeUnsignedInt)
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
		return cborType, uint64(binary.BigEndian.Uint16(data[1:3])), data[3:], false, nil
	case additional == 26:
		if len(data) < 5 {
			return 0, 0, nil, false, errors.New("truncated CBOR header")
		}
		return cborType, uint64(binary.BigEndian.Uint32(data[1:5])), data[5:], false, nil
	case additional == 27:
		if len(data) < 9 {
			return 0, 0, nil, false, errors.New("truncated CBOR header")
		}
		return cborType, binary.BigEndian.Uint64(data[1:9]), data[9:], false, nil
	case additional == CborIndefFlag:
		return cborType, 0, data[1:], true, nil
	default:
		return 0, 0, nil, false, fmt.Errorf("invalid CBOR header: 0x%02x", data[0])
	}
}

func decodeNextPlutusData(data []byte) (PlutusData, []byte, error) {
	if len(data) == 0 {
		return nil, nil, errors.New("empty data")
	}

	switch data[0] & CborTypeMask {
	case CborTypeArray:
		tmpList, rest, err := decodeListNext(data)
		if err != nil {
			return nil, nil, err
		}
		return tmpList, rest, nil
	case CborTypeMap:
		tmpMap, rest, err := decodeMapNext(data)
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
			tmpConstr, rest, err := decodeConstrNext(tagNumber, tagContent)
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
	tmp, err := decode(item)
	if err != nil {
		return nil, nil, err
	}
	return tmp, rest, nil
}

func splitCBORItem(data []byte) ([]byte, []byte, error) {
	rest, err := skipCBORItem(data)
	if err != nil {
		return nil, nil, err
	}
	itemLen := len(data) - len(rest)
	return data[:itemLen], rest, nil
}

func skipCBORItem(data []byte) ([]byte, error) {
	cborType, value, rest, indefinite, err := decodeCBORHead(data)
	if err != nil {
		return nil, err
	}

	switch cborType {
	case CborTypeUnsignedInt, CborTypeNegativeInt:
		if indefinite {
			return nil, errors.New("indefinite CBOR integer is not supported")
		}
		return rest, nil
	case CborTypeByteString, CborTypeTextString:
		return skipCBORBytesLike(rest, value, indefinite, cborType)
	case CborTypeArray:
		if indefinite {
			return skipCBORSequence(rest, 1, true)
		}
		if value > math.MaxInt {
			return nil, fmt.Errorf("CBOR array too large: %d", value)
		}
		return skipCBORSequence(rest, int(value), false)
	case CborTypeMap:
		if indefinite {
			return skipCBORSequence(rest, 2, true)
		}
		if value > math.MaxInt/2 {
			return nil, fmt.Errorf("CBOR map too large: %d", value)
		}
		return skipCBORSequence(rest, int(value)*2, false)
	case CborTypeTag:
		if indefinite {
			return nil, errors.New("indefinite CBOR tags are not supported")
		}
		return skipCBORItem(rest)
	case CborTypeSimple:
		if indefinite {
			return nil, errors.New("unexpected CBOR break")
		}
		return rest, nil
	default:
		return nil, fmt.Errorf("unsupported CBOR type 0x%02x", cborType)
	}
}

func skipCBORBytesLike(
	rest []byte,
	value uint64,
	indefinite bool,
	cborType uint8,
) ([]byte, error) {
	if !indefinite {
		if value > uint64(len(rest)) {
			return nil, errors.New("truncated CBOR payload")
		}
		return rest[value:], nil
	}

	for {
		if len(rest) == 0 {
			return nil, errors.New("unterminated indefinite-length CBOR string")
		}
		if rest[0] == 0xff {
			return rest[1:], nil
		}

		chunkType, chunkValue, chunkRest, chunkIndef, err := decodeCBORHead(rest)
		if err != nil {
			return nil, err
		}
		if chunkIndef || chunkType != cborType {
			return nil, fmt.Errorf("invalid CBOR chunk type 0x%02x", chunkType)
		}
		if chunkValue > uint64(len(chunkRest)) {
			return nil, errors.New("truncated CBOR payload")
		}
		rest = chunkRest[chunkValue:]
	}
}

func skipCBORSequence(rest []byte, count int, indefinite bool) ([]byte, error) {
	if indefinite {
		for {
			if len(rest) == 0 {
				return nil, errors.New("unterminated indefinite-length CBOR container")
			}
			if rest[0] == 0xff {
				return rest[1:], nil
			}
			var err error
			for range count {
				rest, err = skipCBORItem(rest)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	for range count {
		var err error
		rest, err = skipCBORItem(rest)
		if err != nil {
			return nil, err
		}
	}
	return rest, nil
}
