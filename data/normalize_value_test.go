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
	"math/big"
	"strings"
	"testing"
)

const (
	// Value {h'01': {h'02': 1}} with indefinite-length outer and token maps.
	indefValueHex = "d90579bf4101bf410201ffff"
	// The same Value in canonical definite form.
	defValueHex = "d90579a14101a1410201"
)

func TestNormalizeValueContainers(t *testing.T) {
	t.Parallel()

	decoded := mustDecodeHex(t, indefValueHex)
	if got := mustEncodeHex(t, decoded); got != indefValueHex {
		t.Fatalf("fixture does not round-trip: got %s", got)
	}
	ptr := decoded.(*Value)

	t.Run("pointer", func(t *testing.T) {
		t.Parallel()
		before := mustEncodeHex(t, ptr)
		out := Normalize(ptr)
		if _, ok := out.(*Value); !ok {
			t.Fatalf("Normalize returned %T, want *Value", out)
		}
		if got := mustEncodeHex(t, out); got != defValueHex {
			t.Fatalf("encoded %s, want %s", got, defValueHex)
		}
		if got := mustEncodeHex(t, ptr); got != before {
			t.Fatalf("input mutated: %s -> %s", before, got)
		}
	})
	t.Run("value", func(t *testing.T) {
		t.Parallel()
		out := Normalize(*ptr)
		if _, ok := out.(*Value); !ok {
			t.Fatalf("Normalize returned %T, want *Value", out)
		}
		if got := mustEncodeHex(t, out); got != defValueHex {
			t.Fatalf("encoded %s, want %s", got, defValueHex)
		}
	})
	t.Run("nested", func(t *testing.T) {
		t.Parallel()
		in := NewList(
			NewConstr(0, ptr),
			NewMap([][2]PlutusData{{NewInteger(big.NewInt(1)), ptr}}),
		)
		before := mustEncodeHex(t, in)
		enc := mustEncodeHex(t, Normalize(in))
		if strings.Contains(enc, indefValueHex) {
			t.Fatalf("nested Value kept indefinite encoding: %s", enc)
		}
		if strings.Count(enc, defValueHex) != 2 {
			t.Fatalf("want 2 canonical Values in %s", enc)
		}
		if got := mustEncodeHex(t, in); got != before {
			t.Fatalf("input mutated: %s -> %s", before, got)
		}
	})
}

func TestNormalizeValueNil(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]PlutusData{
		"nil inner pointer": &Value{},
		"nil inner value":   Value{},
		"typed nil inner":   &Value{Inner: (*Map)(nil)},
	} {
		v, ok := Normalize(in).(*Value)
		if !ok || v == nil || v.Inner != nil {
			t.Errorf("%s: want *Value with nil Inner", name)
		}
	}
	if out := Normalize((*Value)(nil)); out != PlutusData((*Value)(nil)) {
		t.Errorf("typed nil *Value: got %#v", out)
	}
}
