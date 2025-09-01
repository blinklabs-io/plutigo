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
		Data: NewListDefIndef(
			true,
			NewInteger(big.NewInt(123)),
			NewInteger(big.NewInt(456)),
		),
		CborHex: "9f187b1901c8ff",
	},
	{
		Data:    NewListDefIndef(false),
		CborHex: "80",
	},
	{
		Data: NewConstrDefIndef(
			true,
			1,
			NewByteString([]byte{0xab, 0xcd}),
		),
		CborHex: "d87a9f42abcdff",
	},
	{
		Data:    NewConstrDefIndef(false, 0),
		CborHex: "d87980",
	},
	{
		Data: NewListDefIndef(
			true,
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
					NewConstrDefIndef(
						true,
						0,
						NewInteger(big.NewInt(0)),
						NewInteger(big.NewInt(406)),
					),
					NewConstrDefIndef(
						true,
						0,
						NewInteger(big.NewInt(1725522262478821201)),
					),
				},
			},
		),
		CborHex: "a1d8799f00190196ffd8799f1b17f2495b03141751ff",
	},
	{
		Data: NewConstrDefIndef(
			true,
			0,
			NewMap(
				[][2]PlutusData{
					{
						NewConstrDefIndef(
							true,
							0,
							NewInteger(big.NewInt(0)),
							NewInteger(big.NewInt(406)),
						),
						NewConstrDefIndef(
							true,
							0,
							NewInteger(big.NewInt(1725522262478821201)),
						),
					},
				},
			),
		),
		CborHex: "d8799fa1d8799f00190196ffd8799f1b17f2495b03141751ffff",
	},
	{
		Data: NewConstrDefIndef(
			true,
			999,
			NewInteger(big.NewInt(6)),
			NewInteger(big.NewInt(7)),
		),
		// 102([999, [_ 6, 7]])
		CborHex: "d866821903e79f0607ff",
	},
	{
		Data: NewMap(
			[][2]PlutusData{
				{
					NewListDefIndef(
						true,
						NewInteger(big.NewInt(1)),
						NewInteger(big.NewInt(2)),
					),
					NewMap(
						[][2]PlutusData{
							{
								NewListDefIndef(
									true,
									NewByteString(nil),
								),
								NewConstrDefIndef(
									true,
									0,
									NewInteger(big.NewInt(2)),
									NewInteger(big.NewInt(1)),
								),
							},
						},
					),
				},
			},
		),
		// {[_ 1, 2]: {[_ h'']: 121_0([_ 2, 1])}}
		CborHex: "a19f0102ffa19f40ffd8799f0201ff",
	},
	{
		Data: NewMap(
			[][2]PlutusData{
				{
					NewMap(
						[][2]PlutusData{
							{
								NewInteger(big.NewInt(1)),
								NewInteger(big.NewInt(2)),
							},
						},
					),
					NewInteger(big.NewInt(3)),
				},
			},
		),
		// {{1: 2}: 3}
		CborHex: "a1a1010203",
	},
	{
		Data: NewConstrDefIndef(
			false,
			1,
			NewConstrDefIndef(
				false,
				0,
			),
		),
		CborHex: "d87a81d87980",
	},
	{
		Data: NewMap(
			[][2]PlutusData{
				{
					NewInteger(big.NewInt(2)),
					NewInteger(big.NewInt(2)),
				},
				{
					NewInteger(big.NewInt(3)),
					NewInteger(big.NewInt(3)),
				},
				{
					NewInteger(big.NewInt(1)),
					NewInteger(big.NewInt(1)),
				},
			},
		),
		// {2:2,3:3,1:1}
		CborHex: "a3020203030101",
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
