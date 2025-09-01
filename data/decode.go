package data

import (
	"fmt"
	"math/big"

	"github.com/fxamacker/cbor/v2"
)

const (
	CborTypeByteString uint8 = 0x40
	CborTypeArray      uint8 = 0x80
	CborTypeMap        uint8 = 0xa0
	CborTypeTag        uint8 = 0xc0

	// Only the top 3 bytes are used to specify the type
	CborTypeMask uint8 = 0xe0

	CborIndefFlag uint8 = 0x1f
)

// Decode decodes a CBOR-encoded byte slice into a PlutusData value.
// It returns an error if the input is invalid or not a valid PlutusData encoding.
func Decode(b []byte) (PlutusData, error) {
	v, err := decodeCborRaw(b)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CBOR: %w", err)
	}

	return decodeRaw(v)
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

// decodeCborRaw is an alternative to cbor.Unmarshal() that converts cbor.Tag to Constr
// This is needed because cbor.Tag with a slice as the content (such as in a Constr) is
// not hashable and cannot be used as a map key
func decodeCborRaw(data []byte) (any, error) {
	cborType := data[0] & CborTypeMask
	switch cborType {
	case CborTypeByteString:
		var tmpData cbor.ByteString
		if err := cborUnmarshal(data, &tmpData); err != nil {
			return nil, err
		}
		return tmpData, nil
	case CborTypeArray:
		return decodeCborRawList(data)
	case CborTypeMap:
		return decodeCborRawMap(data)
	case CborTypeTag:
		var tmpTag cbor.RawTag
		if err := cborUnmarshal(data, &tmpTag); err != nil {
			return nil, err
		}
		return decodeRawTag(tmpTag)
	default:
		// Decode using default representation
		var tmpData any
		if err := cborUnmarshal(data, &tmpData); err != nil {
			return nil, err
		}
		return tmpData, nil
	}
}

func decodeCborRawList(data []byte) (any, error) {
	useIndef := (data[0] & CborIndefFlag) == CborIndefFlag
	var tmpData []cbor.RawMessage
	if err := cborUnmarshal(data, &tmpData); err != nil {
		return nil, err
	}
	tmpItems := make([]PlutusData, len(tmpData))
	for i, item := range tmpData {
		tmp, err := decodeCborRaw(item)
		if err != nil {
			return nil, err
		}
		tmpPd, err := decodeRaw(tmp)
		if err != nil {
			return nil, err
		}
		tmpItems[i] = tmpPd
	}
	ret := NewList(tmpItems...)
	ret.(*List).useIndef = &useIndef
	return ret, nil
}

func decodeCborRawMap(data []byte) (any, error) {
	// The below is a hack to work around our CBOR library not supporting preserving key
	// order when decoding a map. We decode our map to determine its length, create a dummy
	// list the same length as our map to determine the header size, and then decode each
	// key/value pair individually
	var tmpData map[RawMessageStr]RawMessageStr
	if err := cborUnmarshal(data, &tmpData); err != nil {
		return nil, err
	}
	// Create dummy list of same length to determine map header length
	tmpList := make([]bool, len(tmpData))
	tmpListRaw, err := cborMarshal(tmpList)
	if err != nil {
		return nil, err
	}
	tmpListHeader := tmpListRaw[0 : len(tmpListRaw)-len(tmpData)]
	// Strip off map header bytes
	data = data[len(tmpListHeader):]
	pairs := make([][2]PlutusData, 0, len(tmpData))
	var rawKey, rawVal cbor.RawMessage
	// Read key/value pairs until we have no data left
	for len(data) > 0 {
		// Read raw key/value bytes
		data, err = cbor.UnmarshalFirst(data, &rawKey)
		if err != nil {
			return nil, err
		}
		data, err = cbor.UnmarshalFirst(data, &rawVal)
		if err != nil {
			return nil, err
		}
		// Decode key/value
		tmpKey, err := decodeCborRaw(rawKey)
		if err != nil {
			return nil, err
		}
		tmpKeyPd, err := decodeRaw(tmpKey)
		if err != nil {
			return nil, err
		}
		tmpVal, err := decodeCborRaw(rawVal)
		if err != nil {
			return nil, err
		}
		tmpValPd, err := decodeRaw(tmpVal)
		if err != nil {
			return nil, err
		}
		pairs = append(
			pairs,
			[2]PlutusData{
				tmpKeyPd,
				tmpValPd,
			},
		)
	}
	ret := NewMap(pairs)
	return ret, nil
}

// decodeRaw converts a raw CBOR-decoded value into PlutusData.
func decodeRaw(v any) (PlutusData, error) {
	switch x := v.(type) {
	// Handle List (untagged array).
	case []any:
		items := make([]PlutusData, len(x))

		for i, item := range x {
			pd, err := decodeRaw(item)
			if err != nil {
				return nil, fmt.Errorf("failed to decode list item %d: %w", i, err)
			}
			items[i] = pd
		}

		return NewList(items...), nil

	// Handle Map.
	case map[any]any:
		pairs := make([][2]PlutusData, 0, len(x))

		for k, v := range x {
			key, err := decodeRaw(k)
			if err != nil {
				return nil, fmt.Errorf("failed to decode map key: %w", err)
			}

			val, err := decodeRaw(v)
			if err != nil {
				return nil, fmt.Errorf("failed to decode map value: %w", err)
			}

			pairs = append(pairs, [2]PlutusData{key, val})
		}

		return NewMap(pairs), nil

	case []byte:
		return NewByteString(x), nil

	case cbor.ByteString:
		return NewByteString(x.Bytes()), nil

	case big.Int:
		return NewInteger(&x), nil

	case int64:
		return NewInteger(big.NewInt(x)), nil

	case uint64:
		return NewInteger(new(big.Int).SetUint64(x)), nil

	case *Constr:
		return x, nil

	case *Integer:
		return x, nil

	case *List:
		return x, nil

	case *Map:
		return x, nil

	default:
		return nil, fmt.Errorf("unsupported CBOR type for PlutusData: %T", x)
	}
}

func decodeRawTag(tag cbor.RawTag) (PlutusData, error) {
	var ret PlutusData
	var retErr error
	// Handle tagged data (Constr, Bignum).
	switch tag.Number {
	// Constr with tag 0..6.
	case 121, 122, 123, 124, 125, 126, 127:
		ret, retErr = decodeConstr(tag.Number-121, tag.Content)

	// Constr with tag 7..127.
	case 1280, 1281, 1282, 1283, 1284, 1285, 1286, 1287,
		1288, 1289, 1290, 1291, 1292, 1293, 1294, 1295,
		1296, 1297, 1298, 1299, 1300, 1301, 1302, 1303,
		1304, 1305, 1306, 1307, 1308, 1309, 1310, 1311,
		1312, 1313, 1314, 1315, 1316, 1317, 1318, 1319,
		1320, 1321, 1322, 1323, 1324, 1325, 1326, 1327,
		1328, 1329, 1330, 1331, 1332, 1333, 1334, 1335,
		1336, 1337, 1338, 1339, 1340, 1341, 1342, 1343,
		1344, 1345, 1346, 1347, 1348, 1349, 1350, 1351,
		1352, 1353, 1354, 1355, 1356, 1357, 1358, 1359,
		1360, 1361, 1362, 1363, 1364, 1365, 1366, 1367,
		1368, 1369, 1370, 1371, 1372, 1373, 1374, 1375,
		1376, 1377, 1378, 1379, 1380, 1381, 1382, 1383,
		1384, 1385, 1386, 1387, 1388, 1389, 1390, 1391,
		1392, 1393, 1394, 1395, 1396, 1397, 1398, 1399, 1400:

		ret, retErr = decodeConstr((tag.Number-1280)+7, tag.Content)

	// PosBignum
	case 2:
		ret, retErr = decodeBignum(tag.Content, false)

	// NegBignum
	case 3:
		ret, retErr = decodeBignum(tag.Content, true)

	case 102:
		var tmpData struct {
			_           struct{} `cbor:",toarray"`
			Alternative uint64
			FieldsRaw   cbor.RawMessage
		}
		if err := cborUnmarshal(tag.Content, &tmpData); err != nil {
			return nil, err
		}
		ret, retErr = decodeConstr(tmpData.Alternative, tmpData.FieldsRaw)
	default:
		return nil, fmt.Errorf("unknown CBOR tag for PlutusData: %d", tag.Number)
	}
	return ret, retErr
}

// decodeConstr decodes a Constr from a CBOR tag content (expected to be an array).
func decodeConstr(tag uint64, content cbor.RawMessage) (PlutusData, error) {
	tmpData, err := decodeCborRaw(content)
	if err != nil {
		return nil, err
	}
	tmpList, ok := tmpData.(*List)
	if !ok {
		return nil, fmt.Errorf(
			"expected array for Constr tag %d, got %T",
			tag,
			tmpData,
		)
	}

	fields := tmpList.Items

	ret := NewConstr(uint(tag), fields...)
	ret.(*Constr).useIndef = tmpList.useIndef
	return ret, nil
}

// decodeBignum decodes a big integer from CBOR tag content (expected to be bytes).
func decodeBignum(content any, negative bool) (PlutusData, error) {
	bytes, ok := content.(cbor.RawMessage)
	if !ok {
		return nil, fmt.Errorf("expected bytes for Bignum, got %T", content)
	}

	// Convert bytes to big.Int (assuming big-endian, as in Rust's rug::Integer::from_digits).
	n := new(big.Int).SetBytes([]byte(bytes))

	if negative {
		n.Neg(n)
	}

	return NewInteger(n), nil
}
