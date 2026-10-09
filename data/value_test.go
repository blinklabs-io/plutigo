// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package data

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func valueToken(key []byte, quantity *big.Int) [2]PlutusData {
	return [2]PlutusData{NewByteString(key), NewInteger(quantity)}
}

func valuePolicy(key []byte, tokens ...[2]PlutusData) [2]PlutusData {
	return [2]PlutusData{NewByteString(key), NewMap(tokens)}
}

func valueMap(policies ...[2]PlutusData) *Map {
	return &Map{Pairs: policies}
}

func pow2(n uint) *big.Int { return new(big.Int).Lsh(big.NewInt(1), n) }

func valueCases() (valid, invalid map[string]*Map) {
	one := big.NewInt(1)
	maxQty := new(big.Int).Sub(pow2(127), one)
	minQty := new(big.Int).Neg(pow2(127))
	long := bytes.Repeat([]byte{0x01}, 33)
	max := bytes.Repeat([]byte{0x01}, 32)

	valid = map[string]*Map{
		"empty":             valueMap(),
		"max quantity":      valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, maxQty))),
		"min quantity":      valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, minQty))),
		"32 byte keys":      valueMap(valuePolicy(max, valueToken(max, one))),
		"empty keys":        valueMap(valuePolicy(nil, valueToken(nil, one))),
		"ascending keys":    valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, one), valueToken([]byte{1, 0}, one)), valuePolicy([]byte{2}, valueToken([]byte{1}, one))),
		"negative quantity": valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, big.NewInt(-5)))),
	}

	invalid = map[string]*Map{
		"policy key too long":    valueMap(valuePolicy(long, valueToken([]byte{1}, one))),
		"token key too long":     valueMap(valuePolicy([]byte{1}, valueToken(long, one))),
		"policies out of order":  valueMap(valuePolicy([]byte{2}, valueToken([]byte{1}, one)), valuePolicy([]byte{1}, valueToken([]byte{1}, one))),
		"duplicate policy":       valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, one)), valuePolicy([]byte{1}, valueToken([]byte{2}, one))),
		"duplicate empty policy": valueMap(valuePolicy(nil, valueToken([]byte{1}, one)), valuePolicy([]byte{}, valueToken([]byte{1}, one))),
		"tokens out of order":    valueMap(valuePolicy([]byte{1}, valueToken([]byte{2}, one), valueToken([]byte{1}, one))),
		"duplicate token":        valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, one), valueToken([]byte{1}, one))),
		"duplicate empty token":  valueMap(valuePolicy([]byte{1}, valueToken(nil, one), valueToken([]byte{}, one))),
		"empty token map":        valueMap(valuePolicy([]byte{1})),
		"zero quantity":          valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, new(big.Int)))),
		"quantity above range":   valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, pow2(127)))),
		"quantity below range":   valueMap(valuePolicy([]byte{1}, valueToken([]byte{1}, new(big.Int).Sub(new(big.Int).Neg(pow2(127)), one)))),
	}
	return valid, invalid
}

func TestValueCanonicalFormAcrossPaths(t *testing.T) {
	t.Parallel()
	valid, invalid := valueCases()

	for name, inner := range valid {
		t.Run("accepts "+name, func(t *testing.T) {
			t.Parallel()
			value, err := NewValue(inner)
			if err != nil {
				t.Fatalf("NewValue: %v", err)
			}
			encoded, err := Encode(value)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if _, err := Decode(encoded); err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if _, err := NewDecoder().Decode(encoded); err != nil {
				t.Fatalf("arena Decode: %v", err)
			}
		})
	}

	for name, inner := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewValue(inner); err == nil {
				t.Error("NewValue accepted a non-canonical Value")
			}
			if _, err := Encode(&Value{Inner: inner}); err == nil {
				t.Error("Encode accepted a non-canonical Value")
			}
			encoded, err := cborMarshal(cbor.Tag{Number: valueCBORTag, Content: inner})
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			if _, err := Decode(encoded); err == nil {
				t.Error("Decode accepted a non-canonical Value")
			}
			var value Value
			if err := value.UnmarshalCBOR(encoded); err == nil {
				t.Error("UnmarshalCBOR accepted a non-canonical Value")
			}
			if _, err := NewDecoder().Decode(encoded); err == nil {
				t.Error("arena Decode accepted a non-canonical Value")
			}
		})
	}
}

func TestNewValueRejectsTypedNil(t *testing.T) {
	one := big.NewInt(1)
	invalid := map[string]*Map{
		"policy key": valueMap([2]PlutusData{(*ByteString)(nil), NewMap([][2]PlutusData{valueToken(nil, one)})}),
		"token map":  valueMap([2]PlutusData{NewByteString(nil), (*Map)(nil)}),
		"token key":  valueMap(valuePolicy(nil, [2]PlutusData{(*ByteString)(nil), NewInteger(one)})),
		"quantity":   valueMap(valuePolicy(nil, [2]PlutusData{NewByteString(nil), (*Integer)(nil)})),
	}
	for name, inner := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := NewValue(inner); err == nil {
				t.Fatal("NewValue accepted a typed-nil entry")
			}
		})
	}
}
