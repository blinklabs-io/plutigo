package data

import (
	"bytes"
	"encoding/hex"
	"errors"
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
		Data: func() PlutusData {
			v := big.NewInt(1)
			v.Lsh(v, 64)
			return NewInteger(v)
		}(),
		CborHex: "c249010000000000000000",
	},
	{
		Data: func() PlutusData {
			v := big.NewInt(1)
			v.Lsh(v, 64)
			v.Add(v, big.NewInt(1))
			v.Neg(v)
			return NewInteger(v)
		}(),
		CborHex: "c349010000000000000000",
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
		name     string
		data     PlutusData
		byteIdx  int
		wantByte byte
		minLen   int
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

func TestDecoderReuseMatchesDecode(t *testing.T) {
	decoder := NewDecoder()
	decoded := make([]PlutusData, 0, len(testDefs))

	t.Run("before-reset", func(t *testing.T) {
		for _, testDef := range testDefs {
			testDef := testDef
			t.Run(testDef.CborHex, func(t *testing.T) {
				tmpData, err := hex.DecodeString(testDef.CborHex)
				if err != nil {
					t.Fatalf("failed to decode hex %s: %v", testDef.CborHex, err)
				}

				got, err := decoder.Decode(tmpData)
				if err != nil {
					t.Fatalf("decoder.Decode() failed for %s: %v", testDef.CborHex, err)
				}
				if !got.Equal(testDef.Data) {
					t.Fatalf("decoder.Decode() mismatch for %s: got %s, want %s", testDef.CborHex, got, testDef.Data)
				}
				decoded = append(decoded, got)
			})
		}
	})

	for i, got := range decoded {
		if !got.Equal(testDefs[i].Data) {
			t.Fatalf("decoded value %d changed before reset: got %s, want %s", i, got, testDefs[i].Data)
		}
	}

	decoder.Reset()
	t.Run("after-reset", func(t *testing.T) {
		for _, testDef := range testDefs {
			testDef := testDef
			t.Run(testDef.CborHex, func(t *testing.T) {
				tmpData, err := hex.DecodeString(testDef.CborHex)
				if err != nil {
					t.Fatalf("failed to decode hex %s after reset: %v", testDef.CborHex, err)
				}
				got, err := decoder.Decode(tmpData)
				if err != nil {
					t.Fatalf("decoder.Decode() after reset failed for %s: %v", testDef.CborHex, err)
				}
				if !got.Equal(testDef.Data) {
					t.Fatalf("decoder.Decode() after reset mismatch for %s: got %s, want %s", testDef.CborHex, got, testDef.Data)
				}
			})
		}
	})
}

func TestDecodeRejectsExcessiveNesting(t *testing.T) {
	encoded := nestedListCBOR(MaxDecodeNestingDepth + 1)

	_, err := Decode(encoded)
	assertDecodeLimitError(t, err, "nesting depth")

	decoder := NewDecoder()
	_, err = decoder.Decode(encoded)
	assertDecodeLimitError(t, err, "nesting depth")

	var list List
	err = list.UnmarshalCBOR(encoded)
	assertDecodeLimitError(t, err, "nesting depth")
}

func TestDecodeRejectsExcessiveNodeCount(t *testing.T) {
	encoded := []byte{0x84, 0x00, 0x00, 0x00, 0x00}
	limits := decodeLimits{maxDepth: MaxDecodeNestingDepth, maxNodes: 4}

	_, err := decodeWithState(encoded, newDecodeStateWithLimits(limits))
	assertDecodeLimitError(t, err, "node count")

	decoder := NewDecoder()
	_, err = decoder.decode(encoded, newDecodeStateWithLimits(limits))
	assertDecodeLimitError(t, err, "node count")
}

func nestedListCBOR(depth int) []byte {
	encoded := make([]byte, depth+1)
	for i := 0; i < depth; i++ {
		encoded[i] = 0x81
	}
	encoded[depth] = 0x00
	return encoded
}

func assertDecodeLimitError(t *testing.T, err error, limit string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s limit error, got nil", limit)
	}
	var limitErr *DecodeLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected DecodeLimitError, got %T: %v", err, err)
	}
	if limitErr.Limit != limit {
		t.Fatalf("DecodeLimitError limit = %q, want %q", limitErr.Limit, limit)
	}
}

func TestDecoderResetOverwritesRetainedBigInts(t *testing.T) {
	decoder := NewDecoder()

	large := new(big.Int).Lsh(big.NewInt(1), 80)
	largeEncoded, err := Encode(NewInteger(large))
	if err != nil {
		t.Fatalf("Encode large failed: %v", err)
	}
	smallEncoded, err := Encode(NewInteger(big.NewInt(-3)))
	if err != nil {
		t.Fatalf("Encode small failed: %v", err)
	}

	if _, err := decoder.Decode(largeEncoded); err != nil {
		t.Fatalf("Decode large failed: %v", err)
	}
	decoder.Reset()
	decoded, err := decoder.Decode(smallEncoded)
	if err != nil {
		t.Fatalf("Decode small failed: %v", err)
	}

	integer, ok := decoded.(*Integer)
	if !ok {
		t.Fatalf("decoded value = %T, want *Integer", decoded)
	}
	if got, want := integer.Inner.Int64(), int64(-3); got != want {
		t.Fatalf("decoded integer = %d, want %d", got, want)
	}
	if !decoded.Equal(NewInteger(big.NewInt(-3))) {
		t.Fatalf("decoded value changed after overwrite: got %s", decoded)
	}
}

func TestDecodeCBORTag(t *testing.T) {
	tests := []struct {
		name        string
		hexInput    string
		wantTag     uint64
		wantContent string
	}{
		{
			name:        "small tag",
			hexInput:    "d87980",
			wantTag:     121,
			wantContent: "80",
		},
		{
			name:        "two-byte tag",
			hexInput:    "d9050080",
			wantTag:     1280,
			wantContent: "80",
		},
		{
			name:        "tag 102 array payload",
			hexInput:    "d866821903e780",
			wantTag:     102,
			wantContent: "821903e780",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := hex.DecodeString(tt.hexInput)
			if err != nil {
				t.Fatalf("decode hex: %v", err)
			}

			gotTag, gotContent, err := decodeCBORTag(input)
			if err != nil {
				t.Fatalf("decodeCBORTag() error = %v", err)
			}
			if gotTag != tt.wantTag {
				t.Fatalf("decodeCBORTag() tag = %d, want %d", gotTag, tt.wantTag)
			}
			if hex.EncodeToString(gotContent) != tt.wantContent {
				t.Fatalf(
					"decodeCBORTag() content = %x, want %s",
					gotContent,
					tt.wantContent,
				)
			}
		})
	}
}

func TestDecodeCBORArray(t *testing.T) {
	tests := []struct {
		name      string
		hexInput  string
		wantCount int
		wantRest  string
		wantIndef bool
	}{
		{
			name:      "definite array",
			hexInput:  "820102",
			wantCount: 2,
			wantRest:  "0102",
		},
		{
			name:      "indefinite array",
			hexInput:  "9f0102ff",
			wantRest:  "0102ff",
			wantIndef: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := hex.DecodeString(tt.hexInput)
			if err != nil {
				t.Fatalf("decode hex: %v", err)
			}

			gotCount, gotRest, gotIndef, err := decodeCBORArray(input)
			if err != nil {
				t.Fatalf("decodeCBORArray() error = %v", err)
			}
			if gotCount != tt.wantCount {
				t.Fatalf("decodeCBORArray() count = %d, want %d", gotCount, tt.wantCount)
			}
			if hex.EncodeToString(gotRest) != tt.wantRest {
				t.Fatalf("decodeCBORArray() rest = %x, want %s", gotRest, tt.wantRest)
			}
			if gotIndef != tt.wantIndef {
				t.Fatalf(
					"decodeCBORArray() indef = %v, want %v",
					gotIndef,
					tt.wantIndef,
				)
			}
		})
	}
}

func TestSplitCBORItem(t *testing.T) {
	tests := []struct {
		name     string
		hexInput string
		wantItem string
		wantRest string
	}{
		{
			name:     "small integer",
			hexInput: "0102",
			wantItem: "01",
			wantRest: "02",
		},
		{
			name:     "constructor tag with fields",
			hexInput: "d8799f0102ff03",
			wantItem: "d8799f0102ff",
			wantRest: "03",
		},
		{
			name:     "indefinite bytestring",
			hexInput: "5f4201024103ff04",
			wantItem: "5f4201024103ff",
			wantRest: "04",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := hex.DecodeString(tt.hexInput)
			if err != nil {
				t.Fatalf("decode hex: %v", err)
			}

			gotItem, gotRest, err := splitCBORItem(input)
			if err != nil {
				t.Fatalf("splitCBORItem() error = %v", err)
			}
			if hex.EncodeToString(gotItem) != tt.wantItem {
				t.Fatalf("splitCBORItem() item = %x, want %s", gotItem, tt.wantItem)
			}
			if hex.EncodeToString(gotRest) != tt.wantRest {
				t.Fatalf("splitCBORItem() rest = %x, want %s", gotRest, tt.wantRest)
			}
		})
	}
}

func TestByteStringUnmarshalCBORIndefinite(t *testing.T) {
	original := bytes.Repeat([]byte{0xab}, 65)
	encoded, err := ByteString{Inner: original}.MarshalCBOR()
	if err != nil {
		t.Fatalf("marshal bytestring: %v", err)
	}

	var decoded ByteString
	if err := decoded.UnmarshalCBOR(encoded); err != nil {
		t.Fatalf("unmarshal bytestring: %v", err)
	}

	if !bytes.Equal(decoded.Inner, original) {
		t.Fatalf("decoded bytes mismatch")
	}
}

func TestByteStringMarshalCBOREmptyDecoded(t *testing.T) {
	var decoded ByteString
	if err := decoded.UnmarshalCBOR([]byte{0x40}); err != nil {
		t.Fatalf("unmarshal empty bytestring: %v", err)
	}

	encoded, err := decoded.MarshalCBOR()
	if err != nil {
		t.Fatalf("marshal empty bytestring: %v", err)
	}
	if !bytes.Equal(encoded, []byte{0x40}) {
		t.Fatalf("encoded empty bytestring = %x, want 40", encoded)
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
		if len(data) > 4096 {
			t.Skip()
		}

		var wrapper PlutusDataWrapper
		if err := wrapper.UnmarshalCBOR(data); err != nil {
			return
		}
		if wrapper.Data == nil {
			t.Fatal("decoded nil PlutusData without error")
		}

		encoded, err := wrapper.MarshalCBOR()
		if err != nil {
			t.Fatalf("failed to marshal decoded PlutusData: %v", err)
		}

		var decodedAgain PlutusDataWrapper
		if err := decodedAgain.UnmarshalCBOR(encoded); err != nil {
			t.Fatalf("failed to decode marshaled PlutusData: %v", err)
		}
		if !wrapper.Data.Equal(decodedAgain.Data) {
			t.Fatalf("CBOR round-trip mismatch: got %v, want %v", decodedAgain.Data, wrapper.Data)
		}
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
