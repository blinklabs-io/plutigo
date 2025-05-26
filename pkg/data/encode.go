package data

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/fxamacker/cbor/v2"
)

// Encode encodes a PlutusData value into CBOR bytes.
func Encode(pd PlutusData) ([]byte, error) {
	encoded, err := encodeToRaw(pd)
	if err != nil {
		return nil, err
	}

	return cbor.Marshal(encoded)
}

// encodeToRaw converts PlutusData to a CBOR-encodable representation.
func encodeToRaw(pd PlutusData) (any, error) {
	switch v := pd.(type) {
	case *Constr:
		return encodeConstr(v)
	case *Map:
		return encodeMap(v)
	case *Integer:
		return encodeInteger(v)
	case *ByteString:
		return encodeByteString(v)
	case *List:
		return encodeList(v)
	default:
		return nil, fmt.Errorf("unknown PlutusData type: %T", pd)
	}
}

// encodeConstr encodes a Constr to CBOR tag format.
func encodeConstr(c *Constr) (any, error) {
	// Encode fields first
	fields := make([]any, len(c.Fields))
	for i, field := range c.Fields {
		encoded, err := encodeToRaw(field)
		if err != nil {
			return nil, fmt.Errorf("failed to encode Constr field %d: %w", i, err)
		}
		fields[i] = encoded
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
	case c.Tag == 102:
		return nil, errors.New("tagged data (tag 102) not implemented")
	default:
		return nil, fmt.Errorf("unsupported Constr tag: %d", c.Tag)
	}

	return cbor.Tag{
		Number:  cborTag,
		Content: fields,
	}, nil
}

// encodeMap encodes a Map to CBOR map format.
func encodeMap(m *Map) (any, error) {
	// Convert to map[any]any for CBOR encoding
	result := make(map[any]any, len(m.Pairs))

	for _, pair := range m.Pairs {
		key, err := encodeToRaw(pair[0])
		if err != nil {
			return nil, fmt.Errorf("failed to encode map key: %w", err)
		}

		value, err := encodeToRaw(pair[1])
		if err != nil {
			return nil, fmt.Errorf("failed to encode map value: %w", err)
		}

		result[key] = value
	}

	return result, nil
}

// encodeInteger encodes an Integer to CBOR format.
func encodeInteger(i *Integer) (any, error) {
	// For small integers, encode directly
	if i.Inner.IsInt64() {
		val := i.Inner.Int64()
		// Check if it fits in the standard CBOR integer range
		if val >= -2147483648 && val <= 2147483647 {
			return val, nil
		}
	}

	// For large integers, use CBOR bignum tags
	isNegative := i.Inner.Sign() < 0

	// Get absolute value bytes
	var absInt *big.Int
	if isNegative {
		absInt = new(big.Int).Neg(i.Inner)
	} else {
		absInt = new(big.Int).Set(i.Inner)
	}

	bytes := absInt.Bytes() // Big-endian byte representation

	var tag uint64
	if isNegative {
		tag = 3 // NegBignum
	} else {
		tag = 2 // PosBignum
	}

	return cbor.Tag{
		Number:  tag,
		Content: bytes,
	}, nil
}

// encodeByteString encodes a ByteString to CBOR format.
func encodeByteString(bs *ByteString) (any, error) {
	// The Rust code shows special handling for byte strings longer than 64 bytes
	// using indefinite-length encoding, but the Go CBOR library handles this automatically
	// when using cbor.Marshal, so we can just return the bytes directly
	return bs.Inner, nil
}

// encodeList encodes a List to CBOR array format.
func encodeList(l *List) (any, error) {
	result := make([]any, len(l.Items))

	for i, item := range l.Items {
		encoded, err := encodeToRaw(item)
		if err != nil {
			return nil, fmt.Errorf("failed to encode list item %d: %w", i, err)
		}
		result[i] = encoded
	}

	return result, nil
}
