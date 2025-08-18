package data

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"
)

var testDefs = []struct {
	Data    PlutusData
	CborHex string
}{
	{
		Data:    NewInteger(big.NewInt(7)),
		CborHex: "07",
	},
	{
		Data:    NewInteger(big.NewInt(999)),
		CborHex: "1903e7",
	},
	{
		Data: NewList(
			NewInteger(big.NewInt(123)),
			NewInteger(big.NewInt(456)),
		),
		CborHex: "9f187b1901c8ff",
	},
	{
		Data:    NewList(),
		CborHex: "80",
	},
	{
		Data: NewConstr(
			1,
			NewByteString([]byte{0xab, 0xcd}),
		),
		CborHex: "d87a9f42abcdff",
	},
	{
		Data:    NewConstr(0),
		CborHex: "d87980",
	},
	{
		Data: NewList(
			NewInteger(big.NewInt(1)),
			NewInteger(big.NewInt(2)),
		),
		CborHex: "9f0102ff",
	},
	{
		Data: NewMap(
			[][2]PlutusData{
				{
					NewInteger(big.NewInt(1)),
					NewInteger(big.NewInt(2)),
				},
			},
		),
		CborHex: "a10102",
	},
	{
		Data: NewMap(
			[][2]PlutusData{
				{
					NewConstr(
						0,
						NewInteger(big.NewInt(0)),
						NewInteger(big.NewInt(406)),
					),
					NewConstr(
						0,
						NewInteger(big.NewInt(1725522262478821201)),
					),
				},
			},
		),
		CborHex: "a1d8799f00190196ffd8799f1b17f2495b03141751ff",
	},
	{
		Data: NewConstr(
			0,
			NewMap(
				[][2]PlutusData{
					{
						NewConstr(
							0,
							NewInteger(big.NewInt(0)),
							NewInteger(big.NewInt(406)),
						),
						NewConstr(
							0,
							NewInteger(big.NewInt(1725522262478821201)),
						),
					},
				},
			),
		),
		CborHex: "d8799fa1d8799f00190196ffd8799f1b17f2495b03141751ffff",
	},
}

func TestPlutusDataEncode(t *testing.T) {
	for _, testDef := range testDefs {
		tmpData := PlutusDataWrapper{
			Data: testDef.Data,
		}
		tmpCbor, err := tmpData.MarshalCBOR()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if hex.EncodeToString(tmpCbor) != testDef.CborHex {
			t.Errorf("did not get expected CBOR\n     got: %x\n  wanted: %s", tmpCbor, testDef.CborHex)
		}
	}
}

func TestPlutusDataDecode(t *testing.T) {
	for _, testDef := range testDefs {
		tmpCbor, err := hex.DecodeString(testDef.CborHex)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		var tmpData PlutusDataWrapper
		if err := tmpData.UnmarshalCBOR(tmpCbor); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(tmpData.Data, testDef.Data) {
			t.Errorf("did not get expected data\n     got: %#v\n  wanted: %#v", tmpData.Data, testDef.Data)
		}
	}
}
