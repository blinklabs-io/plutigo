package data

import (
	"math/big"
	"strings"
	"testing"
)

// These tests characterize the exact observable behavior of the PlutusData
// JSON decoder so the decoding implementation can be changed without
// changing semantics. They intentionally pin corner cases such as duplicate
// keys, ignored unknown keys, exact error messages, and limit boundaries.

func bigFromString(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("invalid big.Int literal %q", s)
	}
	return n
}

func TestDecodeJSONParityAccepted(t *testing.T) {
	tests := []struct {
		name string
		json string
		want PlutusData
	}{
		{
			name: "duplicate int key keeps last occurrence",
			json: `{"int":1,"int":2}`,
			want: NewInteger(big.NewInt(2)),
		},
		{
			name: "duplicate list key keeps last occurrence",
			json: `{"list":[{"int":1}],"list":[{"int":2}]}`,
			want: NewList(NewInteger(big.NewInt(2))),
		},
		{
			name: "duplicate list key ignores invalid first occurrence",
			json: `{"list":[{"bogus":1}],"list":[{"int":2}]}`,
			want: NewList(NewInteger(big.NewInt(2))),
		},
		{
			name: "duplicate bytes key keeps last occurrence",
			json: `{"bytes":"ff","bytes":"abcd"}`,
			want: NewByteString([]byte{0xab, 0xcd}),
		},
		{
			name: "duplicate constructor key keeps last occurrence",
			json: `{"constructor":1,"constructor":2,"fields":[]}`,
			want: NewConstr(2),
		},
		{
			name: "duplicate fields key keeps last occurrence",
			json: `{"constructor":0,"fields":[{"int":1}],"fields":[{"int":2}]}`,
			want: NewConstr(0, NewInteger(big.NewInt(2))),
		},
		{
			name: "unknown sibling key is ignored",
			json: `{"int":1,"extra":"x"}`,
			want: NewInteger(big.NewInt(1)),
		},
		{
			name: "unknown sibling key with nested junk is ignored",
			json: `{"int":1,"extra":{"deep":[1,2,{"a":null},true]}}`,
			want: NewInteger(big.NewInt(1)),
		},
		{
			name: "unknown sibling key on constr is ignored",
			json: `{"constructor":0,"fields":[],"junk":3}`,
			want: NewConstr(0),
		},
		{
			name: "fields without constructor is ignored when int present",
			json: `{"int":7,"fields":[{"bogus":true}]}`,
			want: NewInteger(big.NewInt(7)),
		},
		{
			name: "null bytes value decodes to empty bytestring",
			json: `{"bytes":null}`,
			want: NewByteString(nil),
		},
		{
			name: "null constructor tag decodes as zero",
			json: `{"constructor":null,"fields":[]}`,
			want: NewConstr(0),
		},
		{
			name: "uppercase hex accepted",
			json: `{"bytes":"AbCd"}`,
			want: NewByteString([]byte{0xab, 0xcd}),
		},
		{
			name: "escaped hex string accepted",
			json: `{"bytes":"ab"}`,
			want: NewByteString([]byte{0xab}),
		},
		{
			name: "escaped discriminator key recognized",
			json: `{"int":42}`,
			want: NewInteger(big.NewInt(42)),
		},
		{
			name: "big integer fidelity positive",
			json: `{"int":123456789012345678901234567890}`,
			want: NewInteger(bigFromString(t, "123456789012345678901234567890")),
		},
		{
			name: "big integer fidelity negative",
			json: `{"int":-987654321098765432109876543210}`,
			want: NewInteger(bigFromString(t, "-987654321098765432109876543210")),
		},
		{
			name: "surrounding whitespace tolerated",
			json: "  \n\t{ \"int\" : 42 }\r\n ",
			want: NewInteger(big.NewInt(42)),
		},
		{
			name: "map pair duplicate k keeps last occurrence",
			json: `{"map":[{"k":{"int":1},"k":{"int":2},"v":{"int":3}}]}`,
			want: NewMap([][2]PlutusData{
				{NewInteger(big.NewInt(2)), NewInteger(big.NewInt(3))},
			}),
		},
		{
			name: "map pair duplicate k ignores invalid first occurrence",
			json: `{"map":[{"k":{"bogus":1},"k":{"int":2},"v":{"int":3}}]}`,
			want: NewMap([][2]PlutusData{
				{NewInteger(big.NewInt(2)), NewInteger(big.NewInt(3))},
			}),
		},
		{
			name: "map pair duplicate k after v keeps last occurrence",
			json: `{"map":[{"k":{"int":1},"v":{"int":2},"k":{"int":9}}]}`,
			want: NewMap([][2]PlutusData{
				{NewInteger(big.NewInt(9)), NewInteger(big.NewInt(2))},
			}),
		},
		{
			name: "map pair v before k accepted",
			json: `{"map":[{"v":{"int":2},"k":{"int":1}}]}`,
			want: NewMap([][2]PlutusData{
				{NewInteger(big.NewInt(1)), NewInteger(big.NewInt(2))},
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeJSON([]byte(tt.json))
			if err != nil {
				t.Fatalf("DecodeJSON(%s) error: %v", tt.json, err)
			}
			if !tt.want.Equal(got) {
				t.Errorf("DecodeJSON(%s):\n  got:  %v\n  want: %v", tt.json, got, tt.want)
			}
		})
	}
}

func TestDecodeJSONParityErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "top-level number",
			json: `42`,
			want: `PlutusData JSON must be an object: json: cannot unmarshal number into Go value of type map[string]json.RawMessage`,
		},
		{
			name: "top-level string",
			json: `"x"`,
			want: `PlutusData JSON must be an object: json: cannot unmarshal string into Go value of type map[string]json.RawMessage`,
		},
		{
			name: "top-level array",
			json: `[1]`,
			want: `PlutusData JSON must be an object: json: cannot unmarshal array into Go value of type map[string]json.RawMessage`,
		},
		{
			name: "top-level bool",
			json: `true`,
			want: `PlutusData JSON must be an object: json: cannot unmarshal bool into Go value of type map[string]json.RawMessage`,
		},
		{
			name: "top-level null",
			json: `null`,
			want: `unrecognized PlutusData JSON keys: []`,
		},
		{
			name: "empty input",
			json: ``,
			want: `PlutusData JSON must be an object: unexpected end of JSON input`,
		},
		{
			name: "truncated object",
			json: `{"int":`,
			want: `PlutusData JSON must be an object: unexpected end of JSON input`,
		},
		{
			name: "truncated after value",
			json: `{"int":1,`,
			want: `PlutusData JSON must be an object: unexpected end of JSON input`,
		},
		{
			name: "trailing garbage",
			json: `{"int":1}x`,
			want: `PlutusData JSON must be an object: invalid character 'x' after top-level value`,
		},
		{
			name: "trailing second document",
			json: `{"int":1}{"int":2}`,
			want: `PlutusData JSON must be an object: invalid character '{' after top-level value`,
		},
		{
			name: "unknown key listed",
			json: `{"unknown":1}`,
			want: `unrecognized PlutusData JSON keys: [unknown]`,
		},
		{
			name: "unknown keys sorted",
			json: `{"b":1,"a":2}`,
			want: `unrecognized PlutusData JSON keys: [a b]`,
		},
		{
			name: "fields without constructor",
			json: `{"fields":[]}`,
			want: `unrecognized PlutusData JSON keys: [fields]`,
		},
		{
			name: "ambiguous int and bytes",
			json: `{"int":1,"bytes":"ab"}`,
			want: `ambiguous PlutusData JSON: multiple discriminator keys [int bytes]`,
		},
		{
			name: "ambiguity beats invalid scalar values",
			json: `{"int":"garbage","bytes":"zz"}`,
			want: `ambiguous PlutusData JSON: multiple discriminator keys [int bytes]`,
		},
		{
			name: "ambiguity beats invalid list content",
			json: `{"list":[{"bogus":1}],"int":1}`,
			want: `ambiguous PlutusData JSON: multiple discriminator keys [int list]`,
		},
		{
			name: "ambiguous constructor and int",
			json: `{"constructor":0,"fields":[],"int":1}`,
			want: `ambiguous PlutusData JSON: multiple discriminator keys [int constructor]`,
		},
		{
			name: "list value not an array",
			json: `{"list":42}`,
			want: `failed to unmarshal List value: value must be an array`,
		},
		{
			name: "list value is an object",
			json: `{"list":{"a":1}}`,
			want: `failed to unmarshal List value: value must be an array`,
		},
		{
			name: "map value not an array",
			json: `{"map":"x"}`,
			want: `failed to unmarshal Map value: value must be an array`,
		},
		{
			name: "fields value not an array",
			json: `{"constructor":0,"fields":42}`,
			want: `failed to unmarshal Constr fields: value must be an array`,
		},
		{
			name: "invalid list item wrapped with index",
			json: `{"list":[{"int":1},{"bad":1}]}`,
			want: `failed to unmarshal List value: failed to unmarshal List item 1: unrecognized PlutusData JSON keys: [bad]`,
		},
		{
			name: "invalid constr field wrapped with index",
			json: `{"constructor":0,"fields":[{"bad":1}]}`,
			want: `failed to unmarshal Constr fields: failed to unmarshal Constr field 0: unrecognized PlutusData JSON keys: [bad]`,
		},
		{
			name: "invalid map key wrapped",
			json: `{"map":[{"k":{"bad":1},"v":{"int":1}}]}`,
			want: `failed to unmarshal Map value: failed to unmarshal Map key 0: unrecognized PlutusData JSON keys: [bad]`,
		},
		{
			name: "invalid map value wrapped",
			json: `{"map":[{"k":{"int":1},"v":{"bad":1}}]}`,
			want: `failed to unmarshal Map value: failed to unmarshal Map value 0: unrecognized PlutusData JSON keys: [bad]`,
		},
		{
			name: "map key error reported before map value error",
			json: `{"map":[{"v":{"bad":1},"k":{"bad2":1}}]}`,
			want: `failed to unmarshal Map value: failed to unmarshal Map key 0: unrecognized PlutusData JSON keys: [bad2]`,
		},
		{
			name: "map pair not an object",
			json: `{"map":[42]}`,
			want: `failed to unmarshal Map value: Map pair 0 must be an object`,
		},
		{
			name: "map pair null",
			json: `{"map":[null]}`,
			want: `failed to unmarshal Map value: Map pair 0 must be an object`,
		},
		{
			name: "map pair unknown field",
			json: `{"map":[{"k":{"int":1},"v":{"int":2},"x":3}]}`,
			want: `failed to unmarshal Map value: unexpected Map pair 0 field "x"`,
		},
		{
			name: "map pair missing k",
			json: `{"map":[{"v":{"int":1}}]}`,
			want: `failed to unmarshal Map value: missing "k" key in Map pair 0`,
		},
		{
			name: "map pair missing v",
			json: `{"map":[{"k":{"int":1}}]}`,
			want: `failed to unmarshal Map value: missing "v" key in Map pair 0`,
		},
		{
			name: "constr tag error beats fields content error",
			json: `{"constructor":"x","fields":[{"bad":1}]}`,
			want: `failed to unmarshal Constr constructor tag: json: cannot unmarshal string into Go value of type uint`,
		},
		{
			name: "constr tag error when fields come first",
			json: `{"fields":[{"bad":1}],"constructor":"x"}`,
			want: `failed to unmarshal Constr constructor tag: json: cannot unmarshal string into Go value of type uint`,
		},
		{
			name: "negative constructor tag",
			json: `{"constructor":-1,"fields":[]}`,
			want: `failed to unmarshal Constr constructor tag: json: cannot unmarshal number -1 into Go value of type uint`,
		},
		{
			name: "fractional constructor tag",
			json: `{"constructor":1.5,"fields":[]}`,
			want: `failed to unmarshal Constr constructor tag: json: cannot unmarshal number 1.5 into Go value of type uint`,
		},
		{
			name: "overflowing constructor tag",
			json: `{"constructor":18446744073709551616,"fields":[]}`,
			want: `failed to unmarshal Constr constructor tag: json: cannot unmarshal number 18446744073709551616 into Go value of type uint`,
		},
		{
			name: "null int value",
			json: `{"int":null}`,
			want: `null "int" value in Integer JSON`,
		},
		{
			name: "null list value",
			json: `{"list":null}`,
			want: `null "list" value in List JSON`,
		},
		{
			name: "null map value",
			json: `{"map":null}`,
			want: `null "map" value in Map JSON`,
		},
		{
			name: "null fields value",
			json: `{"constructor":0,"fields":null}`,
			want: `null "fields" value in Constr JSON`,
		},
		{
			name: "missing fields key",
			json: `{"constructor":0}`,
			want: `missing "fields" key in Constr JSON`,
		},
		{
			name: "bytes value not a string",
			json: `{"bytes":5}`,
			want: `failed to unmarshal ByteString value: json: cannot unmarshal number into Go value of type string`,
		},
		{
			name: "odd-length hex",
			json: `{"bytes":"abc"}`,
			want: `invalid hex in ByteString JSON: encoding/hex: odd length hex string`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tt.json))
			if err == nil {
				t.Fatalf("DecodeJSON(%s): expected error, got nil", tt.json)
			}
			if err.Error() != tt.want {
				t.Errorf("DecodeJSON(%s):\n  got:  %s\n  want: %s", tt.json, err, tt.want)
			}
		})
	}
}

// TestDecodeJSONParityErrorsRejected covers inputs whose error text comes
// from the Go runtime in version-dependent form; only rejection is pinned.
func TestDecodeJSONParityErrorsRejected(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"string int value", `{"int":"42"}`},
		{"float int value", `{"int":1.5}`},
		{"exponent int value", `{"int":1e3}`},
		{"bool int value", `{"int":true}`},
		{"object int value", `{"int":{"a":1}}`},
		{"string constructor tag", `{"constructor":"0","fields":[]}`},
		{"number with trailing junk", `{"int":12,}`},
		{"unterminated string", `{"bytes":"ab`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeJSON([]byte(tt.json)); err == nil {
				t.Errorf("DecodeJSON(%s): expected error, got nil", tt.json)
			}
		})
	}
}

func TestDecodeJSONParityDepthBoundary(t *testing.T) {
	// nestedListJSON(d) produces d nested Lists plus one Integer, so the
	// deepest PlutusData node sits at depth d+1.
	t.Run("exactly at limit decodes", func(t *testing.T) {
		input := nestedListJSON(MaxDecodeNestingDepth - 1)
		if _, err := DecodeJSON([]byte(input)); err != nil {
			t.Fatalf("expected success at depth limit, got: %v", err)
		}
	})
	t.Run("one past limit rejected", func(t *testing.T) {
		input := nestedListJSON(MaxDecodeNestingDepth)
		_, err := DecodeJSON([]byte(input))
		if err == nil {
			t.Fatal("expected depth error, got nil")
		}
		want := "PlutusData JSON nesting exceeds max depth 256"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error containing %q, got: %v", want, err)
		}
	})
}

func TestDecodeJSONParityBigIntRoundTrip(t *testing.T) {
	in := `{"int":123456789012345678901234567890}`
	pd, err := DecodeJSON([]byte(in))
	if err != nil {
		t.Fatalf("DecodeJSON error: %v", err)
	}
	out, err := EncodeJSON(pd)
	if err != nil {
		t.Fatalf("EncodeJSON error: %v", err)
	}
	if string(out) != in {
		t.Errorf("round-trip:\n  got:  %s\n  want: %s", out, in)
	}
}
