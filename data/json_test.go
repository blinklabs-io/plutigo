package data

import (
	"encoding/json"
	"math/big"
	"testing"
)

func TestPlutusDataJSONInteger(t *testing.T) {
	tests := []struct {
		name string
		data PlutusData
		json string
	}{
		{
			name: "small positive",
			data: NewInteger(big.NewInt(42)),
			json: `{"int":42}`,
		},
		{
			name: "zero",
			data: NewInteger(big.NewInt(0)),
			json: `{"int":0}`,
		},
		{
			name: "negative",
			data: NewInteger(big.NewInt(-123)),
			json: `{"int":-123}`,
		},
		{
			name: "large",
			data: NewInteger(big.NewInt(9223372036854775807)),
			json: `{"int":9223372036854775807}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.data)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tt.json {
				t.Errorf("Marshal:\n  got:  %s\n  want: %s", got, tt.json)
			}
		})
	}
}

func TestPlutusDataJSONByteString(t *testing.T) {
	tests := []struct {
		name string
		data PlutusData
		json string
	}{
		{
			name: "non-empty",
			data: NewByteString([]byte{0xab, 0xcd}),
			json: `{"bytes":"abcd"}`,
		},
		{
			name: "empty",
			data: NewByteString(nil),
			json: `{"bytes":""}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.data)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tt.json {
				t.Errorf("Marshal:\n  got:  %s\n  want: %s", got, tt.json)
			}
		})
	}
}

func TestPlutusDataJSONList(t *testing.T) {
	tests := []struct {
		name string
		data PlutusData
		json string
	}{
		{
			name: "integers",
			data: NewList(NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))),
			json: `{"list":[{"int":1},{"int":2}]}`,
		},
		{
			name: "empty",
			data: NewList(),
			json: `{"list":[]}`,
		},
		{
			name: "nested",
			data: NewList(
				NewList(NewInteger(big.NewInt(1))),
				NewByteString([]byte{0xff}),
			),
			json: `{"list":[{"list":[{"int":1}]},{"bytes":"ff"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.data)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tt.json {
				t.Errorf("Marshal:\n  got:  %s\n  want: %s", got, tt.json)
			}
		})
	}
}

func TestPlutusDataJSONMap(t *testing.T) {
	tests := []struct {
		name string
		data PlutusData
		json string
	}{
		{
			name: "single pair",
			data: NewMap([][2]PlutusData{
				{NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))},
			}),
			json: `{"map":[{"k":{"int":1},"v":{"int":2}}]}`,
		},
		{
			name: "empty",
			data: NewMap(nil),
			json: `{"map":[]}`,
		},
		{
			name: "multiple pairs",
			data: NewMap([][2]PlutusData{
				{NewByteString([]byte("key")), NewInteger(big.NewInt(1))},
				{NewByteString([]byte("foo")), NewInteger(big.NewInt(2))},
			}),
			json: `{"map":[{"k":{"bytes":"6b6579"},"v":{"int":1}},{"k":{"bytes":"666f6f"},"v":{"int":2}}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.data)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tt.json {
				t.Errorf("Marshal:\n  got:  %s\n  want: %s", got, tt.json)
			}
		})
	}
}

func TestPlutusDataJSONConstr(t *testing.T) {
	tests := []struct {
		name string
		data PlutusData
		json string
	}{
		{
			name: "with fields",
			data: NewConstr(1, NewByteString([]byte{0xab, 0xcd})),
			json: `{"constructor":1,"fields":[{"bytes":"abcd"}]}`,
		},
		{
			name: "no fields",
			data: NewConstr(0),
			json: `{"constructor":0,"fields":[]}`,
		},
		{
			name: "large tag",
			data: NewConstr(999, NewInteger(big.NewInt(6)), NewInteger(big.NewInt(7))),
			json: `{"constructor":999,"fields":[{"int":6},{"int":7}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.data)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(got) != tt.json {
				t.Errorf("Marshal:\n  got:  %s\n  want: %s", got, tt.json)
			}
		})
	}
}

func TestPlutusDataJSONWrapper(t *testing.T) {
	w := PlutusDataWrapper{Data: NewInteger(big.NewInt(42))}
	got, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if string(got) != `{"int":42}` {
		t.Errorf("Marshal: got %s, want {\"int\":42}", got)
	}

	var w2 PlutusDataWrapper
	if err := json.Unmarshal(got, &w2); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !w.Data.Equal(w2.Data) {
		t.Errorf("Unmarshal: got %v, want %v", w2.Data, w.Data)
	}
}

func TestPlutusDataJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data PlutusData
	}{
		{"integer", NewInteger(big.NewInt(42))},
		{"negative integer", NewInteger(big.NewInt(-999))},
		{"bytestring", NewByteString([]byte{0xde, 0xad, 0xbe, 0xef})},
		{"empty bytestring", NewByteString(nil)},
		{"list", NewList(NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2)))},
		{"empty list", NewList()},
		{
			"map",
			NewMap([][2]PlutusData{
				{NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))},
				{NewInteger(big.NewInt(3)), NewInteger(big.NewInt(4))},
			}),
		},
		{"empty map", NewMap(nil)},
		{"constr no fields", NewConstr(0)},
		{"constr with fields", NewConstr(1, NewInteger(big.NewInt(99)))},
		{
			"deeply nested",
			NewConstr(0,
				NewMap([][2]PlutusData{
					{
						NewConstr(0, NewInteger(big.NewInt(0)), NewInteger(big.NewInt(406))),
						NewConstr(0, NewInteger(big.NewInt(1725522262478821201))),
					},
				}),
			),
		},
		{
			"map with nested list keys",
			NewMap([][2]PlutusData{
				{
					NewList(NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))),
					NewMap([][2]PlutusData{
						{
							NewList(NewByteString(nil)),
							NewConstr(0, NewInteger(big.NewInt(2)), NewInteger(big.NewInt(1))),
						},
					}),
				},
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := PlutusDataWrapper{Data: tt.data}
			jsonBytes, err := json.Marshal(w)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var w2 PlutusDataWrapper
			if err := json.Unmarshal(jsonBytes, &w2); err != nil {
				t.Fatalf("Unmarshal error for JSON %s: %v", jsonBytes, err)
			}

			if !tt.data.Equal(w2.Data) {
				t.Errorf("round-trip failed:\n  original: %v\n  got:      %v\n  json:     %s",
					tt.data, w2.Data, jsonBytes)
			}
		})
	}
}

func TestEncodeDecodeJSON(t *testing.T) {
	original := NewConstr(0,
		NewInteger(big.NewInt(42)),
		NewByteString([]byte{0xde, 0xad}),
	)

	encoded, err := EncodeJSON(original)
	if err != nil {
		t.Fatalf("EncodeJSON error: %v", err)
	}

	decoded, err := DecodeJSON(encoded)
	if err != nil {
		t.Fatalf("DecodeJSON error: %v", err)
	}

	if !original.Equal(decoded) {
		t.Errorf("round-trip failed:\n  original: %v\n  decoded:  %v\n  json:     %s",
			original, decoded, encoded)
	}
}

func TestPlutusDataJSONUnmarshalErrors(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"not an object", `42`},
		{"unknown key", `{"unknown": 1}`},
		{"invalid int", `{"int": "not_a_number"}`},
		{"null int", `{"int":null}`},
		{"invalid bytes hex", `{"bytes": "xyz"}`},
		{"missing bytes key", `{"bytes_wrong": "abcd"}`},
		{"constr missing constructor", `{"fields":[]}`},
		{"constr missing fields", `{"constructor":0}`},
		{"ambiguous int and bytes", `{"int":1,"bytes":"ab"}`},
		{"ambiguous int and list", `{"int":1,"list":[]}`},
		{"ambiguous int and constructor", `{"int":1,"constructor":0,"fields":[]}`},
		{"ambiguous bytes and map", `{"bytes":"ab","map":[]}`},
		{"null list", `{"list":null}`},
		{"null map", `{"map":null}`},
		{"null fields", `{"constructor":0,"fields":null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w PlutusDataWrapper
			if err := json.Unmarshal([]byte(tt.json), &w); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
