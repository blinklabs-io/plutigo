package data

import (
	"bytes"
	"fmt"
	"math/big"
	"slices"

	"github.com/fxamacker/cbor/v2"
)

// Encode encodes a PlutusData value into CBOR bytes.
func Encode(pd PlutusData) ([]byte, error) {
	encoded, err := encodeToRaw(pd)
	if err != nil {
		return nil, err
	}

	return cborMarshal(encoded)
}

// cborMarshal acts like cbor.Marshal but allows us to set our own encoder options
func cborMarshal(data any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	opts := cbor.EncOptions{
		// NOTE: set any additional encoder options here
	}
	em, err := opts.EncMode()
	if err != nil {
		return nil, err
	}
	enc := em.NewEncoder(buf)
	err = enc.Encode(data)
	return buf.Bytes(), err
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
			encoded, err := encodeToRaw(item)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to encode Constr field %d: %w",
					i,
					err,
				)
			}
			encodedCbor, err := cborMarshal(encoded)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to encode Constr field item %d: %w",
					i,
					err,
				)
			}
			tmpFields[i] = cbor.RawMessage(encodedCbor)
		}
		fields = tmpFields
	} else {
		// Encode as indefinite-length array
		tmpData := []byte{
			// Start an indefinite-length list
			0x9F,
		}
		for i, item := range c.Fields {
			encoded, err := encodeToRaw(item)
			if err != nil {
				return nil, fmt.Errorf("failed to encode Constr field %d: %w", i, err)
			}
			encodedCbor, err := cborMarshal(encoded)
			if err != nil {
				return nil, fmt.Errorf("failed to encode Constr field item %d: %w", i, err)
			}
			tmpData = slices.Concat(tmpData, encodedCbor)
		}
		tmpData = append(
			tmpData,
			// End indefinite-length list
			0xff,
		)
		fields = RawMessageStr(string(tmpData))
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
	case c.Tag >= 128:
		cborTag = 102
		fields = []any{c.Tag, fields}
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
	// The below is a hack to work around our CBOR library not supporting encoding a map
	// with a specific key order. We pre-encode each key/value pair, build a dummy list to
	// steal and modify its header, and build our own output from pieces. This avoids
	// needing to support 6 different possible encodings of a map's header byte depending
	// on length
	useIndef := false
	if m.useIndef != nil {
		useIndef = *m.useIndef
	}
	// Build encoded pairs
	tmpPairs := make([][]byte, 0, len(m.Pairs))
	for _, pair := range m.Pairs {
		key, err := encodeToRaw(pair[0])
		if err != nil {
			return nil, fmt.Errorf("failed to encode map key: %w", err)
		}
		keyRaw, err := cborMarshal(key)
		if err != nil {
			return nil, fmt.Errorf("encode map key: %w", err)
		}
		value, err := encodeToRaw(pair[1])
		if err != nil {
			return nil, fmt.Errorf("failed to encode map value: %w", err)
		}
		valueRaw, err := cborMarshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode map value: %w", err)
		}
		tmpPairs = append(
			tmpPairs,
			slices.Concat(keyRaw, valueRaw),
		)
	}
	// Build return value
	ret := bytes.NewBuffer(nil)
	if useIndef {
		ret.WriteByte(CborTypeMap | CborIndefFlag)
	} else {
		// Create dummy list with simple (one-byte) values so we can easily extract the header
		tmpList := make([]bool, len(tmpPairs))
		tmpListRaw, err := cborMarshal(tmpList)
		if err != nil {
			return nil, err
		}
		tmpListHeader := tmpListRaw[0 : len(tmpListRaw)-len(tmpPairs)]
		// Modify header byte to switch type from array to map
		tmpListHeader[0] |= 0x20
		_, _ = ret.Write(tmpListHeader)
	}
	_, _ = ret.Write(slices.Concat(tmpPairs...))
	if useIndef {
		// Indef-length "break" byte
		ret.WriteByte(0xff)
	}
	return cbor.RawMessage(ret.Bytes()), nil
}

// encodeInteger encodes an Integer to CBOR format.
func encodeInteger(i *Integer) (any, error) {
	// Encode directly for values that fit in int64
	if i.Inner.IsInt64() {
		val := i.Inner.Int64()
		return val, nil
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
	// NOTE: we use cbor.ByteString to allow it to be used as a map key
	return cbor.ByteString(string(bs.Inner)), nil
}

// encodeList encodes a List to CBOR array format.
func encodeList(l *List) (any, error) {
	useIndef := len(l.Items) > 0
	if l.useIndef != nil {
		useIndef = *l.useIndef
	}
	if !useIndef {
		ret := make([]any, len(l.Items))
		for i, item := range l.Items {
			ret[i] = item
		}
		return ret, nil
	}
	tmpData := []byte{
		// Start an indefinite-length list
		0x9F,
	}
	for i, item := range l.Items {
		encoded, err := encodeToRaw(item)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to encode indef-length list item %d: %w",
				i,
				err,
			)
		}
		encodedCbor, err := cborMarshal(encoded)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to encode indef-length list item %d: %w",
				i,
				err,
			)
		}
		tmpData = slices.Concat(tmpData, encodedCbor)
	}
	tmpData = append(
		tmpData,
		// End indefinite-length list
		0xff,
	)
	return RawMessageStr(string(tmpData)), nil
}

// RawMessageStr is a hashable variant of cbor.RawMessage
type RawMessageStr string

func (r *RawMessageStr) UnmarshalCBOR(data []byte) error {
	*r = RawMessageStr(string(data))
	return nil
}

func (r RawMessageStr) MarshalCBOR() ([]byte, error) {
	return []byte(r), nil
}

func (r RawMessageStr) Bytes() []byte {
	return []byte(r)
}
