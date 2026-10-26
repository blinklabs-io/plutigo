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
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func setJSONInputLimitForTest(t *testing.T, size int) {
	t.Helper()
	prev := MaxJSONInputSize()
	if err := SetMaxJSONInputSize(size); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetMaxJSONInputSize(prev) })
}

func TestSetMaxJSONInputSizeValidation(t *testing.T) {
	prev := MaxJSONInputSize()
	t.Cleanup(func() { _ = SetMaxJSONInputSize(prev) })
	if prev != DefaultMaxJSONInputSize {
		t.Fatalf("default = %d, want %d", prev, DefaultMaxJSONInputSize)
	}
	for _, size := range []int{0, -1, maxJSONInputSizeLimit + 1} {
		if err := SetMaxJSONInputSize(size); err == nil {
			t.Errorf("SetMaxJSONInputSize(%d) succeeded", size)
		}
	}
	if err := SetMaxJSONInputSize(1234); err != nil {
		t.Fatal(err)
	}
	if got := MaxJSONInputSize(); got != 1234 {
		t.Fatalf("MaxJSONInputSize() = %d", got)
	}
}

func TestDecodeJSONRejectsOversizedInput(t *testing.T) {
	doc := `{"int": 1}`
	setJSONInputLimitForTest(t, len(doc))
	if _, err := DecodeJSON([]byte(doc)); err != nil {
		t.Fatalf("input at the limit rejected: %v", err)
	}
	over := doc + " "
	_, err := DecodeJSON([]byte(over))
	var limitErr *JSONLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("want *JSONLimitError, got %v", err)
	}
	if limitErr.Limit != "input size" || limitErr.Actual != len(over) ||
		limitErr.Max != len(doc) {
		t.Fatalf("unexpected error contents: %+v", limitErr)
	}
	// Every exported unmarshaler goes through the same check.
	var i Integer
	if err := i.UnmarshalJSON([]byte(over)); !errors.As(err, &limitErr) {
		t.Fatalf("Integer.UnmarshalJSON: want limit error, got %v", err)
	}
	var w PlutusDataWrapper
	if err := w.UnmarshalJSON([]byte(over)); !errors.As(err, &limitErr) {
		t.Fatalf("wrapper UnmarshalJSON: want limit error, got %v", err)
	}
}

func TestDecodeJSONIntegerDigitCap(t *testing.T) {
	digits := func(n int) string { return "1" + strings.Repeat("0", n-1) }
	tests := []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{"int at cap", `{"int":` + digits(MaxJSONIntegerDigits) + `}`, false},
		{"negative int at cap", `{"int":-` + digits(MaxJSONIntegerDigits) + `}`, false},
		{"int over cap", `{"int":` + digits(MaxJSONIntegerDigits+1) + `}`, true},
		{"negative int over cap", `{"int":-` + digits(MaxJSONIntegerDigits+1) + `}`, true},
		{"tag at cap", `{"constructor":` + digits(MaxJSONIntegerDigits) + `,"fields":[]}`, false},
		{"tag over cap", `{"constructor":` + digits(MaxJSONIntegerDigits+1) + `,"fields":[]}`, true},
		{
			"nested int over cap",
			`{"list":[{"int":1},{"int":` + digits(MaxJSONIntegerDigits+1) + `}]}`,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tt.doc))
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var limitErr *JSONLimitError
			if !errors.As(err, &limitErr) {
				t.Fatalf("want *JSONLimitError, got %v", err)
			}
			if limitErr.Limit != "integer digits" ||
				limitErr.Max != MaxJSONIntegerDigits {
				t.Fatalf("unexpected error contents: %+v", limitErr)
			}
		})
	}
}

func TestDecodeJSONTotalIntegerDigitCap(t *testing.T) {
	one := `{"int":1` + strings.Repeat("0", MaxJSONIntegerDigits-1) + `}`
	n := MaxJSONTotalIntegerDigits / MaxJSONIntegerDigits
	mk := func(count int) string {
		items := make([]string, count)
		for i := range items {
			items[i] = one
		}
		return `{"list":[` + strings.Join(items, ",") + `]}`
	}
	if _, err := DecodeJSON([]byte(mk(n))); err != nil {
		t.Fatalf("at total cap: %v", err)
	}
	_, err := DecodeJSON([]byte(mk(n + 1)))
	var limitErr *JSONLimitError
	if !errors.As(err, &limitErr) || limitErr.Limit != "total integer digits" {
		t.Fatalf("want total digits limit error, got %v", err)
	}
}

func TestDecodeJSONIntegerRoundTripLarge(t *testing.T) {
	// A 16 KiB magnitude (the Cardano maximum transaction size) round-trips.
	n := new(big.Int).Lsh(big.NewInt(1), 8*16384-1)
	enc, err := EncodeJSON(NewInteger(n))
	if err != nil {
		t.Fatal(err)
	}
	pd, err := DecodeJSON(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !pd.Equal(NewInteger(n)) {
		t.Fatal("large integer did not round-trip")
	}
}

func TestNilIntegerInner(t *testing.T) {
	var i Integer
	if _, err := i.MarshalCBOR(); !errors.Is(err, ErrNilInteger) {
		t.Fatalf("MarshalCBOR error = %v, want ErrNilInteger", err)
	}
	if _, err := Encode(&i); err == nil {
		t.Fatal("Encode of nil Integer succeeded")
	}
	if _, err := i.MarshalJSON(); !errors.Is(err, ErrNilInteger) {
		t.Fatalf("MarshalJSON error = %v, want ErrNilInteger", err)
	}
	if got := i.String(); got != "Integer(0)" {
		t.Fatalf("String() = %q", got)
	}
	clone, ok := i.Clone().(*Integer)
	if !ok || clone.Inner == nil || clone.Inner.Sign() != 0 {
		t.Fatalf("Clone() = %#v", i.Clone())
	}
	if !i.Equal(NewInteger(big.NewInt(0))) {
		t.Fatal("nil Integer should equal zero")
	}
	if i.Equal(NewInteger(big.NewInt(1))) {
		t.Fatal("nil Integer should not equal one")
	}
	if !NewInteger(big.NewInt(0)).Equal(&Integer{}) {
		t.Fatal("zero should equal nil Integer")
	}
	if NewInteger(nil).(*Integer).Inner == nil {
		t.Fatal("NewInteger(nil) left Inner nil")
	}
	// Nested in a container, encoding reports the error rather than emitting
	// CBOR null.
	if _, err := Encode(NewList(&Integer{})); err == nil {
		t.Fatal("encoding a list holding a nil Integer succeeded")
	}
}

// legacyString renders pd with the original nested fmt.Sprintf formulation of
// String(), as the reference for the strings.Builder implementation.
type legacyString struct{ pd PlutusData }

func (l legacyString) String() string {
	wrap := func(items []PlutusData) []legacyString {
		out := make([]legacyString, len(items))
		for i, it := range items {
			out[i] = legacyString{it}
		}
		return out
	}
	if l.pd == nil {
		return "<nil>"
	}
	if rv := reflect.ValueOf(l.pd); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return "<nil>"
	}
	switch v := l.pd.(type) {
	case *List:
		return fmt.Sprintf("List%v", wrap(v.Items))
	case *Map:
		pairs := make([][2]legacyString, len(v.Pairs))
		for i, p := range v.Pairs {
			pairs[i] = [2]legacyString{{p[0]}, {p[1]}}
		}
		return fmt.Sprintf("Map%v", pairs)
	case *Constr:
		return fmt.Sprintf(
			"Constr{tag: %s, fields: %v}",
			constrTagString(v.Tag),
			wrap(v.Fields),
		)
	case *Integer:
		return fmt.Sprintf("Integer(%s)", v.Inner.String())
	case *ByteString:
		return fmt.Sprintf("ByteString(%x)", v.Inner)
	case *Value:
		return fmt.Sprintf("Value{%v}", legacyString{v.Inner})
	}
	return fmt.Sprint(l.pd)
}

func stringCorpus() []PlutusData {
	big1, _ := new(big.Int).SetString("-123456789012345678901234567890", 10)
	inner := NewMap([][2]PlutusData{
		{NewByteString([]byte{0xca, 0xfe}), NewInteger(big.NewInt(7))},
	})
	return []PlutusData{
		NewInteger(big.NewInt(0)),
		NewInteger(big1),
		NewByteString(nil),
		NewByteString([]byte{0, 1, 0xff}),
		NewList(),
		NewList(NewList(), NewList(NewInteger(big.NewInt(1)))),
		NewMap(nil),
		NewMap([][2]PlutusData{
			{NewInteger(big.NewInt(1)), NewList(NewByteString([]byte("a")))},
			{NewList(), NewMap(nil)},
		}),
		NewConstr(0),
		NewConstr(5, NewInteger(big.NewInt(1)), NewConstr(1, inner)),
		NewConstrFromBigInt(big1, NewList(NewConstr(2))),
		&Constr{Fields: []PlutusData{nil}},
		&List{Items: []PlutusData{nil, (*Integer)(nil), NewList()}},
		&Value{Inner: inner.(*Map)},
		NewList(&Value{Inner: inner.(*Map)}),
		&Value{},
	}
}

func TestStringMatchesLegacyFormatting(t *testing.T) {
	for i, pd := range stringCorpus() {
		want := legacyString{pd}.String()
		if got := fmt.Sprint(pd); got != want {
			t.Errorf("corpus[%d]: got %q, want %q", i, got, want)
		}
		if s, ok := pd.(fmt.Stringer); ok && s.String() != want {
			t.Errorf("corpus[%d]: String() = %q, want %q", i, s.String(), want)
		}
	}
}

func TestStringDeepNesting(t *testing.T) {
	const depth = 16000
	var pd PlutusData = NewInteger(big.NewInt(1))
	for range depth {
		pd = NewList(pd)
	}
	got := pd.String()
	want := strings.Repeat("List[", depth) + "Integer(1)" +
		strings.Repeat("]", depth)
	if got != want {
		t.Fatal("deep String() output mismatch")
	}
}

func TestStringDeepConstrMapNesting(t *testing.T) {
	const depth = 1000
	var pd PlutusData = NewByteString([]byte{1})
	for i := range depth {
		if i%2 == 0 {
			pd = NewConstr(1, pd)
		} else {
			pd = NewMap([][2]PlutusData{{NewList(), pd}})
		}
	}
	got := fmt.Sprint(pd)
	if want := (legacyString{pd}).String(); got != want {
		t.Fatal("output differs from legacy formatting")
	}
}
