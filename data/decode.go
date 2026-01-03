package data

import (
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

const (
	CborTypeUnsignedInt uint8 = 0
	CborTypeNegativeInt uint8 = 0x20
	CborTypeByteString  uint8 = 0x40
	CborTypeArray       uint8 = 0x80
	CborTypeMap         uint8 = 0xa0
	CborTypeTag         uint8 = 0xc0

	// Only the top 3 bytes are used to specify the type
	CborTypeMask uint8 = 0xe0

	CborIndefFlag uint8 = 0x1f
)

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
	decOptions := cbor.DecOptions{
		// This defaults to 32, but there are blocks in the wild using >64 nested levels
		MaxNestedLevels: 256,
	}
	decMode, err := decOptions.DecMode()
	if err != nil {
		return err
	}
	return decMode.Unmarshal(dataBytes, dest)
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
		if err := cborUnmarshal(data, &tmpData); err != nil {
			return nil, err
		}
		return &tmpData, nil
	case CborTypeMap:
		var tmpData Map
		if err := cborUnmarshal(data, &tmpData); err != nil {
			return nil, err
		}
		return &tmpData, nil
	case CborTypeTag:
		var tmpTag cbor.RawTag
		if err := cborUnmarshal(data, &tmpTag); err != nil {
			return nil, err
		}
		switch {
		// Constr
		case tmpTag.Number == 102 || (tmpTag.Number >= 121 && tmpTag.Number <= 127) || (tmpTag.Number >= 1280 && tmpTag.Number <= 1400):
			var tmpConstr Constr
			if err := cborUnmarshal(data, &tmpConstr); err != nil {
				return nil, err
			}
			return &tmpConstr, nil

		case tmpTag.Number == 2 || tmpTag.Number == 3:
			var tmpData Integer
			if err := cborUnmarshal(data, &tmpData); err != nil {
				return nil, err
			}
			return &tmpData, nil

		default:
			return nil, fmt.Errorf(
				"unknown CBOR tag for PlutusData: %d",
				tmpTag.Number,
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
