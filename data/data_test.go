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
		Data:    NewInteger(big.NewInt(-123)),
		CborHex: "387a",
	},
	{
		Data:    NewInteger(big.NewInt(9223372036854775807)),
		CborHex: "1b7fffffffffffffff",
	},
	{
		Data:    NewInteger(big.NewInt(-9223372036854775808)),
		CborHex: "3b7fffffffffffffff",
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
		Data: NewMapDefIndef(
			false,
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
		Data: NewMapDefIndef(
			false,
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
			NewMapDefIndef(
				false,
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
		Data: NewMapDefIndef(
			false,
			[][2]PlutusData{
				{
					NewListDefIndef(
						true,
						NewInteger(big.NewInt(1)),
						NewInteger(big.NewInt(2)),
					),
					NewMapDefIndef(
						false,
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
		Data: NewMapDefIndef(
			false,
			[][2]PlutusData{
				{
					NewMapDefIndef(
						false,
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
	// Map with specific-order keys
	{
		Data: NewMapDefIndef(
			false,
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
	// Indef-length map
	{
		Data: NewMapDefIndef(
			true,
			[][2]PlutusData{
				{
					NewByteString([]byte{0x01}),
					NewInteger(big.NewInt(1)),
				},
				{
					NewByteString([]byte{0x02}),
					NewInteger(big.NewInt(2)),
				},
			},
		),
		// {_ h'01': 1, h'02': 2}
		CborHex: "bf410101410202ff",
	},
	// Map with duplicate keys
	{
		Data: NewMapDefIndef(
			false,
			[][2]PlutusData{
				{
					NewByteString([]byte("6Key")),
					NewByteString([]byte("1")),
				},
				{
					NewByteString([]byte("5Key")),
					NewByteString([]byte("7")),
				},
				{
					NewByteString([]byte("6Key")),
					NewByteString(nil),
				},
				{
					NewByteString([]byte("5Key")),
					NewByteString(nil),
				},
			},
		),
		CborHex: "a444364b6579413144354b6579413744364b65794044354b657940",
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
			t.Errorf(
				"did not get expected CBOR\n     got: %x\n  wanted: %s",
				tmpCbor,
				testDef.CborHex,
			)
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
			t.Errorf(
				"did not get expected data\n     got: %#v\n  wanted: %#v",
				tmpData.Data,
				testDef.Data,
			)
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	for _, testDef := range testDefs {
		// Encode
		wrapper := PlutusDataWrapper{Data: testDef.Data}
		cborData, err := wrapper.MarshalCBOR()
		if err != nil {
			t.Fatalf("encode error: %v", err)
		}

		// Decode
		var decodedWrapper PlutusDataWrapper
		err = decodedWrapper.UnmarshalCBOR(cborData)
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}

		// Check equal
		if !reflect.DeepEqual(decodedWrapper.Data, testDef.Data) {
			t.Errorf("round-trip failed for %v", testDef.Data)
		}
	}
}

func FuzzDecodeCBOR(f *testing.F) {
	for _, testDef := range testDefs {
		cborData, err := hex.DecodeString(testDef.CborHex)
		if err != nil {
			f.Fatalf("Failed to decode CborHex %q: %v", testDef.CborHex, err)
		}
		f.Add(cborData)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var wrapper PlutusDataWrapper
		_ = wrapper.UnmarshalCBOR(data) // Ignore errors, just check no panic
	})
}

func TestPlutusDataClone(t *testing.T) {
	original := NewInteger(big.NewInt(42))
	cloned := original.Clone()
	if !original.Equal(cloned) {
		t.Error("Cloned data should be equal to original")
	}
	// Check they are different instances
	if reflect.ValueOf(original).
		Pointer() ==
		reflect.ValueOf(cloned).
			Pointer() {
		t.Error("Cloned data should be a different instance")
	}
}

func TestPlutusDataEqual(t *testing.T) {
	a := NewInteger(big.NewInt(100))
	b := NewInteger(big.NewInt(100))
	c := NewInteger(big.NewInt(200))
	if !a.Equal(b) {
		t.Error("Equal integers should be equal")
	}
	if a.Equal(c) {
		t.Error("Different integers should not be equal")
	}
}

func TestPlutusDataString(t *testing.T) {
	data := NewInteger(big.NewInt(123))
	str := data.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	if !contains(str, "123") {
		t.Errorf("String should contain the value: %s", str)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
