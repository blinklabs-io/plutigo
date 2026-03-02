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
		// Constr(1, [Constr(0, [])]) — non-empty fields use indefinite-length,
		// empty fields use definite-length (matches Haskell cborg)
		Data: NewConstrDefIndef(
			true,
			1,
			NewConstrDefIndef(
				false,
				0,
			),
		),
		CborHex: "d87a9fd87980ff",
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
	// Map — maps always use definite-length per Haskell's encodeMapLen
	{
		Data: NewMapDefIndef(
			false,
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
		// {h'01': 1, h'02': 2}
		CborHex: "a2410101410202",
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

func TestUseIndefHonored(t *testing.T) {
	tests := []struct {
		name      string
		data      PlutusData
		byteIdx   int
		wantByte  byte
		minLen    int
	}{
		{
			name:     "list definite non-empty",
			data:     NewListDefIndef(false, NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))),
			byteIdx:  0,
			wantByte: 0x82, // definite-length array header for 2 items
		},
		{
			name:     "list indefinite non-empty",
			data:     NewListDefIndef(true, NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))),
			byteIdx:  0,
			wantByte: 0x9f, // indefinite-length array marker
		},
		{
			name:     "list indefinite empty produces definite",
			data:     NewListDefIndef(true),
			byteIdx:  0,
			wantByte: 0x80, // empty arrays always use definite-length
		},
		{
			name: "map indefinite non-empty",
			data: NewMapDefIndef(true, [][2]PlutusData{
				{NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))},
			}),
			byteIdx:  0,
			wantByte: 0xbf, // indefinite-length map marker
		},
		{
			name:     "constr definite non-empty",
			data:     NewConstrDefIndef(false, 0, NewInteger(big.NewInt(1))),
			byteIdx:  2,    // tag bytes (d879) come first
			wantByte: 0x81, // definite 1-element array
			minLen:   3,
		},
		{
			name:     "constr indefinite empty produces definite",
			data:     NewConstrDefIndef(true, 0),
			byteIdx:  2,    // tag bytes (d879) come first
			wantByte: 0x80, // empty arrays always use definite-length
			minLen:   3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := Encode(tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.minLen > 0 && len(encoded) < tt.minLen {
				t.Fatalf("encoded too short: %x", encoded)
			}
			if encoded[tt.byteIdx] != tt.wantByte {
				t.Errorf(
					"expected byte 0x%02x at index %d, got 0x%02x (full: %x)",
					tt.wantByte, tt.byteIdx, encoded[tt.byteIdx], encoded,
				)
			}
		})
	}
}

func TestByteStringEncode64ByteBoundary(t *testing.T) {
	tests := []struct {
		name         string
		size         int
		wantDefinite bool // true: must NOT start with 0x5f; false: must start with 0x5f and end with 0xff
	}{
		{
			name:         "64 bytes uses definite-length",
			size:         64,
			wantDefinite: true,
		},
		{
			name:         "65 bytes uses indefinite-length chunks",
			size:         65,
			wantDefinite: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.size)
			for i := range data {
				data[i] = byte(i)
			}
			bs := ByteString{Inner: data}
			encoded, err := bs.MarshalCBOR()
			if err != nil {
				t.Fatalf("unexpected error encoding %d-byte ByteString: %v", tt.size, err)
			}
			if tt.wantDefinite {
				if encoded[0] == 0x5f {
					t.Errorf("%d-byte ByteString should use definite-length encoding, but got indefinite-length marker 0x5f", tt.size)
				}
			} else {
				if encoded[0] != 0x5f {
					t.Errorf("%d-byte ByteString should start with indefinite-length marker 0x5f, got 0x%02x", tt.size, encoded[0])
				}
				if encoded[len(encoded)-1] != 0xff {
					t.Errorf("%d-byte ByteString should end with break byte 0xff, got 0x%02x", tt.size, encoded[len(encoded)-1])
				}
			}
		})
	}
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
