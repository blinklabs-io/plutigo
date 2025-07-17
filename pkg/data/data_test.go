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
		CborHex: "82187b1901c8",
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
		CborHex: "820102",
	},
	// TODO: figure out how to not fail this
	// It works fine for encode, but we don't have a good way to capture an indef-length list on decode
	/*
		{
			Data: NewIndefList(
				NewInteger(big.NewInt(1)),
				NewInteger(big.NewInt(2)),
			),
			CborHex: "9f0102ff",
		},
	*/
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
