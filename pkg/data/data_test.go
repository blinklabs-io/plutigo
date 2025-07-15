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
			[]PlutusData{
				NewInteger(big.NewInt(123)),
				NewInteger(big.NewInt(456)),
			},
		),
		CborHex: "82187b1901c8",
	},
	{
		Data: NewConstr(
			1,
			[]PlutusData{
				NewByteString([]byte{0xab, 0xcd}),
			},
		),
		CborHex: "d87a8142abcd",
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
