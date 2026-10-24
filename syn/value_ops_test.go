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

package syn

import (
	"errors"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
)

// oracleValue is the map-based reference model the old Value builtins used
// (Entries -> maps -> sort -> NewValue).
type oracleValue map[string]map[string]*big.Int

func toOracle(v *Value) oracleValue {
	out := oracleValue{}
	for _, e := range v.Entries() {
		p := e.(*ProtoPair)
		pol := string(p.First.(*ByteString).Inner)
		out[pol] = map[string]*big.Int{}
		for _, t := range p.Second.(*ProtoList).List {
			tp := t.(*ProtoPair)
			out[pol][string(tp.First.(*ByteString).Inner)] = new(big.Int).Set(tp.Second.(*Integer).Inner)
		}
	}
	return out
}

func (o oracleValue) build(t testing.TB) *Value {
	var entries []IConstant
	for pol, toks := range o {
		var tl []IConstant
		for tk, amt := range toks {
			if amt.Sign() == 0 {
				continue
			}
			tl = append(tl, &ProtoPair{
				First:  &ByteString{Inner: []byte(tk)},
				Second: &Integer{Inner: new(big.Int).Set(amt)},
			})
		}
		if len(tl) == 0 {
			continue
		}
		entries = append(entries, &ProtoPair{
			First:  &ByteString{Inner: []byte(pol)},
			Second: &ProtoList{List: tl},
		})
	}
	v, err := NewValue(entries)
	if err != nil {
		t.Fatalf("oracle build: %v", err)
	}
	return v
}

func randValue(t testing.TB, r *rand.Rand, withNeg bool) *Value {
	o := oracleValue{}
	for range r.Intn(6) {
		pol := string([]byte{byte(r.Intn(5))})
		if o[pol] == nil {
			o[pol] = map[string]*big.Int{}
		}
		for range 1 + r.Intn(4) {
			amt := int64(1 + r.Intn(5))
			if withNeg && r.Intn(4) == 0 {
				amt = -amt
			}
			o[pol][string([]byte{byte(r.Intn(5))})] = big.NewInt(amt)
		}
	}
	return o.build(t)
}

func sameValue(t *testing.T, got, want *Value, what string) {
	t.Helper()
	if !reflect.DeepEqual(got.Entries(), want.Entries()) {
		t.Fatalf("%s: entries differ\n got %v\nwant %v", what, toOracle(got), toOracle(want))
	}
	g1, g2, g3, _ := got.Sizes()
	w1, w2, w3, _ := want.Sizes()
	if g1 != w1 || g2 != w2 || g3 != w3 {
		t.Fatalf("%s: sizes %d/%d/%d want %d/%d/%d", what, g1, g2, g3, w1, w2, w3)
	}
	if got.HasNegative() != want.HasNegative() {
		t.Fatalf("%s: HasNegative %v want %v", what, got.HasNegative(), want.HasNegative())
	}
}

func TestValueInsertMatchesOracle(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := range 3000 {
		v := randValue(t, r, true)
		before := v.Entries()
		pol := []byte{byte(r.Intn(6))}
		tok := []byte{byte(r.Intn(6))}
		amt := big.NewInt(int64(r.Intn(7) - 3))
		o := toOracle(v)
		if o[string(pol)] == nil {
			o[string(pol)] = map[string]*big.Int{}
		}
		o[string(pol)][string(tok)] = new(big.Int).Set(amt)
		got, err := v.Insert(pol, tok, amt)
		if err != nil {
			t.Fatal(err)
		}
		sameValue(t, got, o.build(t), "insert")
		if !reflect.DeepEqual(before, v.Entries()) {
			t.Fatalf("iteration %d: Insert mutated its receiver", i)
		}
	}
}

func TestValueScaleMatchesOracle(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for range 2000 {
		v := randValue(t, r, true)
		before := v.Entries()
		f := big.NewInt(int64(r.Intn(7) - 3))
		o := toOracle(v)
		for _, toks := range o {
			for k, a := range toks {
				toks[k] = new(big.Int).Mul(a, f)
			}
		}
		got, err := v.Scale(f, nil)
		if err != nil {
			t.Fatal(err)
		}
		sameValue(t, got, o.build(t), "scale")
		if !reflect.DeepEqual(before, v.Entries()) {
			t.Fatal("Scale mutated its receiver")
		}
	}
}

func TestValueUnionMatchesOracle(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for range 3000 {
		a, b := randValue(t, r, true), randValue(t, r, true)
		ba, bb := a.Entries(), b.Entries()
		o := toOracle(a)
		for pol, toks := range toOracle(b) {
			if o[pol] == nil {
				o[pol] = map[string]*big.Int{}
			}
			for k, amt := range toks {
				cur := o[pol][k]
				if cur == nil {
					cur = new(big.Int)
				}
				o[pol][k] = cur.Add(cur, amt)
			}
		}
		got, err := UnionValue(a, b, nil)
		if err != nil {
			t.Fatal(err)
		}
		sameValue(t, got, o.build(t), "union")
		if !reflect.DeepEqual(ba, a.Entries()) || !reflect.DeepEqual(bb, b.Entries()) {
			t.Fatal("UnionValue mutated an input")
		}
	}
}

func TestValueContainsMatchesOracle(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	hits := 0
	for range 5000 {
		a, b := randValue(t, r, false), randValue(t, r, false)
		if r.Intn(2) == 0 {
			sum, err := UnionValue(a, b, nil)
			if err != nil {
				t.Fatal(err)
			}
			a = sum
		}
		want := true
		oa := toOracle(a)
		for pol, toks := range toOracle(b) {
			for k, amt := range toks {
				cur, ok := oa[pol][k]
				if !ok || cur.Cmp(amt) < 0 {
					want = false
				}
			}
		}
		if got := a.Contains(b); got != want {
			t.Fatalf("Contains = %v, want %v\n a=%v\n b=%v", got, want, oa, toOracle(b))
		}
		if want {
			hits++
		}
	}
	if hits == 0 {
		t.Fatal("no containment cases generated")
	}
}

func TestValueOverflow(t *testing.T) {
	maxQ := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
	v := oracleValue{"p": {"t": maxQ}}.build(t)
	if _, err := v.Scale(big.NewInt(2), nil); !errors.Is(err, ErrValueOverflow) {
		t.Fatalf("Scale err = %v", err)
	}
	if _, err := UnionValue(v, v, nil); !errors.Is(err, ErrValueOverflow) {
		t.Fatalf("UnionValue err = %v", err)
	}
	neg := new(big.Int).Neg(maxQ)
	u, err := UnionValue(v, oracleValue{"p": {"t": neg}}.build(t), nil)
	if err != nil || len(u.Entries()) != 0 {
		t.Fatalf("cancelling union: %v", err)
	}
}
