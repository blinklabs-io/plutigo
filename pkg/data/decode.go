package data

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/fxamacker/cbor/v2"
)

// Decode decodes a CBOR-encoded byte slice into a PlutusData value.
// It returns an error if the input is invalid or not a valid PlutusData encoding.
func Decode(b []byte) (PlutusData, error) {
	var raw cbor.RawMessage = b
	var v any

	if err := cbor.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("failed to decode CBOR: %w", err)
	}

	return decodeRaw(v)
}

// decodeRaw converts a raw CBOR-decoded value into PlutusData.
func decodeRaw(v any) (PlutusData, error) {
	switch x := v.(type) {
	case cbor.Tag:
		// Handle tagged data (Constr, Bignum).
		switch x.Number {
		// Constr with tag 0..6.
		case 121, 122, 123, 124, 125, 126, 127:
			return decodeConstr(x.Number-121, x.Content)

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

			return decodeConstr((x.Number-1280)+7, x.Content)

		// PosBignum
		case 2:
			return decodeBignum(x.Content, false)

		// NegBignum
		case 3:
			return decodeBignum(x.Content, true)

		case 102:
			return nil, errors.New("tagged data (tag 102) not implemented")

		default:
			return nil, fmt.Errorf("unknown CBOR tag for PlutusData: %d", x.Number)
		}
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

		return NewList(items), nil

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

	case int64:
		return NewInteger(big.NewInt(x)), nil

	case uint64:
		return NewInteger(new(big.Int).SetUint64(x)), nil

	default:
		return nil, fmt.Errorf("unsupported CBOR type for PlutusData: %T", x)
	}
}

// decodeConstr decodes a Constr from a CBOR tag content (expected to be an array).
func decodeConstr(tag uint64, content any) (PlutusData, error) {
	arr, ok := content.([]any)
	if !ok {
		return nil, fmt.Errorf(
			"expected array for Constr tag %d, got %T",
			tag,
			content,
		)
	}

	fields := make([]PlutusData, len(arr))

	for i, item := range arr {
		pd, err := decodeRaw(item)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to decode Constr field %d: %w",
				i,
				err,
			)
		}

		fields[i] = pd
	}

	return NewConstr(uint(tag), fields), nil
}

// decodeBignum decodes a big integer from CBOR tag content (expected to be bytes).
func decodeBignum(content any, negative bool) (PlutusData, error) {
	bytes, ok := content.([]byte)
	if !ok {
		return nil, fmt.Errorf("expected bytes for Bignum, got %T", content)
	}

	// Convert bytes to big.Int (assuming big-endian, as in Rust's rug::Integer::from_digits).
	n := new(big.Int).SetBytes(bytes)

	if negative {
		n.Neg(n)
	}

	return NewInteger(n), nil
}
