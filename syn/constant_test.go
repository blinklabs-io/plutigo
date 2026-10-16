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
	"bytes"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

func testValueEntries() []IConstant {
	return []IConstant{&ProtoPair{
		FstType: &TByteString{},
		SndType: &TList{Typ: &TPair{
			First:  &TByteString{},
			Second: &TInteger{},
		}},
		First: &ByteString{Inner: []byte{1}},
		Second: &ProtoList{
			LTyp: &TPair{First: &TByteString{}, Second: &TInteger{}},
			List: []IConstant{&ProtoPair{
				FstType: &TByteString{},
				SndType: &TInteger{},
				First:   &ByteString{Inner: []byte{2}},
				Second:  &Integer{Inner: big.NewInt(3)},
			}},
		},
	}}
}

func TestValueSnapshotPreservesCachedShape(t *testing.T) {
	input := testValueEntries()
	value, err := NewValue(input)
	if err != nil {
		t.Fatalf("NewValue: %v", err)
	}
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersionV4,
		Term:    &Constant{Con: value},
	}
	wantPretty := Pretty(program)
	wantFlat, err := Encode(program)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	inputPolicy := input[0].(*ProtoPair)
	inputPolicy.First.(*ByteString).Inner[0] = 9
	inputTokens := inputPolicy.Second.(*ProtoList)
	inputTokens.List[0].(*ProtoPair).Second.(*Integer).Inner.SetInt64(99)
	inputTokens.List = append(inputTokens.List, inputTokens.List[0])

	returned := value.Entries()
	if len(returned) != 1 {
		t.Fatalf("Entries() returned %d policies, want 1", len(returned))
	}
	returnedPolicy := returned[0].(*ProtoPair)
	returnedPolicy.First.(*ByteString).Inner[0] = 8
	returnedTokens := returnedPolicy.Second.(*ProtoList)
	returnedTokens.List[0].(*ProtoPair).Second.(*Integer).Inner.SetInt64(88)
	returnedTokens.List = nil

	total, outer, maxInner, ok := value.Sizes()
	if !ok || total != 1 || outer != 1 || maxInner != 1 {
		t.Fatalf(
			"Sizes() = (%d, %d, %d, %t), want (1, 1, 1, true)",
			total,
			outer,
			maxInner,
			ok,
		)
	}
	gotEntries := value.Entries()
	if len(gotEntries) != 1 {
		t.Fatalf("Entries() after mutation returned %d policies, want 1", len(gotEntries))
	}
	gotPolicy := gotEntries[0].(*ProtoPair)
	if got := gotPolicy.First.(*ByteString).Inner; !bytes.Equal(got, []byte{1}) {
		t.Fatalf("policy ID = %x, want 01", got)
	}
	gotTokens := gotPolicy.Second.(*ProtoList).List
	if len(gotTokens) != 1 {
		t.Fatalf("token count = %d, want 1", len(gotTokens))
	}
	if got := gotTokens[0].(*ProtoPair).Second.(*Integer).Inner; got.Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("quantity = %s, want 3", got)
	}
	if got := Pretty(program); got != wantPretty {
		t.Fatalf("Pretty changed after external mutation: %q != %q", got, wantPretty)
	}
	gotFlat, err := Encode(program)
	if err != nil {
		t.Fatalf("Encode after external mutation: %v", err)
	}
	if !bytes.Equal(gotFlat, wantFlat) {
		t.Fatalf("FLAT encoding changed after external mutation: %x != %x", gotFlat, wantFlat)
	}
}

func TestNewValueRejectsTypedNilEntries(t *testing.T) {
	var nilPolicy *ProtoPair
	var nilTokens *ProtoList
	var nilToken *ProtoPair
	tests := map[string][]IConstant{
		"policy": {nilPolicy},
		"token list": {&ProtoPair{
			First:  &ByteString{Inner: []byte{1}},
			Second: nilTokens,
		}},
		"token": {&ProtoPair{
			First: &ByteString{Inner: []byte{1}},
			Second: &ProtoList{
				List: []IConstant{nilToken},
			},
		}},
	}
	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("NewValue panicked for typed-nil %s: %v", name, recovered)
				}
			}()
			if _, err := NewValue(entries); err == nil {
				t.Fatalf("NewValue accepted typed-nil %s", name)
			}
		})
	}
}

func TestValueNarrowReadsReturnCopies(t *testing.T) {
	entries := testValueEntries()
	second := testValueEntries()[0].(*ProtoPair)
	second.First.(*ByteString).Inner[0] = 4
	secondToken := second.Second.(*ProtoList).List[0].(*ProtoPair)
	secondToken.First.(*ByteString).Inner[0] = 5
	secondToken.Second.(*Integer).Inner.SetInt64(6)
	entries = append(entries, second)

	value, err := NewValue(entries)
	if err != nil {
		t.Fatalf("NewValue: %v", err)
	}
	policyIDs, ok := value.PolicyIDs()
	if !ok || len(policyIDs) != 2 {
		t.Fatalf("PolicyIDs() returned %d IDs with ok=%t, want 2 and true", len(policyIDs), ok)
	}
	policyIDs[0][0] = 9
	policyIDs, ok = value.PolicyIDs()
	if !ok || !bytes.Equal(policyIDs[0], []byte{1}) {
		t.Fatalf("PolicyIDs()[0] = %x with ok=%t, want 01 and true", policyIDs[0], ok)
	}

	quantity, ok := value.LookupCoin([]byte{4}, []byte{5})
	if !ok || quantity.Cmp(big.NewInt(6)) != 0 {
		t.Fatalf("LookupCoin() = (%v, %t), want (6, true)", quantity, ok)
	}
	quantity.SetInt64(99)
	quantity, ok = value.LookupCoin([]byte{4}, []byte{5})
	if !ok || quantity.Cmp(big.NewInt(6)) != 0 {
		t.Fatalf("LookupCoin() after mutation = (%v, %t), want (6, true)", quantity, ok)
	}
	if quantity, ok := value.LookupCoin([]byte{4}, []byte{6}); ok || quantity != nil {
		t.Fatalf("LookupCoin() missing token = (%v, %t), want (nil, false)", quantity, ok)
	}
}

func TestIntegerTyp(t *testing.T) {
	integer := Integer{Inner: big.NewInt(42)}

	typ := integer.Typ()

	if _, ok := typ.(*TInteger); !ok {
		t.Errorf("Expected TInteger, got %T", typ)
	}
}

func TestIntegerCachedMetadata(t *testing.T) {
	integer := newInteger(big.NewInt(42))

	got, ok := integer.CachedInt64()
	if !ok || got != 42 {
		t.Fatalf("CachedInt64() = (%d, %t), want (42, true)", got, ok)
	}
	if got := integer.ExMemWords(); got != 1 {
		t.Fatalf("ExMemWords() = %d, want 1", got)
	}
}

func TestIntegerSetInnerRefreshesMetadata(t *testing.T) {
	integer := &Integer{}
	integer.SetInner(new(big.Int).Lsh(big.NewInt(1), 130))

	if got := integer.ExMemWords(); got != 3 {
		t.Fatalf("ExMemWords() = %d, want 3", got)
	}
	if _, ok := integer.CachedInt64(); ok {
		t.Fatal("CachedInt64() unexpectedly reported a large integer as int64")
	}

	integer.SetInner(big.NewInt(-7))
	got, ok := integer.CachedInt64()
	if !ok || got != -7 {
		t.Fatalf("CachedInt64() after reset = (%d, %t), want (-7, true)", got, ok)
	}
}

func TestIntegerExMemWordsNilReceiver(t *testing.T) {
	var integer *Integer

	if got := integer.ExMemWords(); got != 1 {
		t.Fatalf("ExMemWords() on nil receiver = %d, want 1", got)
	}
}

func TestByteStringTyp(t *testing.T) {
	bs := ByteString{Inner: []byte{1, 2, 3}}

	typ := bs.Typ()

	if _, ok := typ.(*TByteString); !ok {
		t.Errorf("Expected TByteString, got %T", typ)
	}
}

func TestStringTyp(t *testing.T) {
	str := String{Inner: "hello"}

	typ := str.Typ()

	if _, ok := typ.(*TString); !ok {
		t.Errorf("Expected TString, got %T", typ)
	}
}

func TestUnitTyp(t *testing.T) {
	unit := Unit{}

	typ := unit.Typ()

	if _, ok := typ.(*TUnit); !ok {
		t.Errorf("Expected TUnit, got %T", typ)
	}
}

func TestBoolTyp(t *testing.T) {
	boolConst := Bool{Inner: true}

	typ := boolConst.Typ()

	if _, ok := typ.(*TBool); !ok {
		t.Errorf("Expected TBool, got %T", typ)
	}
}

func TestProtoListTyp(t *testing.T) {
	elementType := &TInteger{}
	protoList := ProtoList{
		LTyp: elementType,
		List: []IConstant{
			&Integer{Inner: big.NewInt(1)},
			&Integer{Inner: big.NewInt(2)},
		},
	}

	typ := protoList.Typ()

	listTyp, ok := typ.(*TList)
	if !ok {
		t.Errorf("Expected TList, got %T", typ)
		return
	}

	if listTyp.Typ != elementType {
		t.Errorf("Expected element type %T, got %T", elementType, listTyp.Typ)
	}
}

func TestProtoPairTyp(t *testing.T) {
	fstType := &TInteger{}
	sndType := &TBool{}
	protoPair := ProtoPair{
		FstType: fstType,
		SndType: sndType,
		First:   &Integer{Inner: big.NewInt(42)},
		Second:  &Bool{Inner: true},
	}

	typ := protoPair.Typ()

	pairTyp, ok := typ.(*TPair)
	if !ok {
		t.Errorf("Expected TPair, got %T", typ)
		return
	}

	if pairTyp.First != fstType {
		t.Errorf("Expected first type %T, got %T", fstType, pairTyp.First)
	}

	if pairTyp.Second != sndType {
		t.Errorf("Expected second type %T, got %T", sndType, pairTyp.Second)
	}
}

func TestDataTyp(t *testing.T) {
	dataConst := Data{
		Inner: data.NewInteger(big.NewInt(42)),
	} // Simple integer data for testing

	typ := dataConst.Typ()

	if _, ok := typ.(*TData); !ok {
		t.Errorf("Expected TData, got %T", typ)
	}
}

func TestBls12_381G1ElementTyp(t *testing.T) {
	g1 := Bls12_381G1Element{
		Inner: &bls.G1Jac{},
	} // Empty G1 element for testing

	typ := g1.Typ()

	if _, ok := typ.(*TBls12_381G1Element); !ok {
		t.Errorf("Expected TBls12_381G1Element, got %T", typ)
	}
}

func TestBls12_381G2ElementTyp(t *testing.T) {
	g2 := Bls12_381G2Element{
		Inner: &bls.G2Jac{},
	} // Empty G2 element for testing

	typ := g2.Typ()

	if _, ok := typ.(*TBls12_381G2Element); !ok {
		t.Errorf("Expected TBls12_381G2Element, got %T", typ)
	}
}

func TestBls12_381MlResultTyp(t *testing.T) {
	ml := Bls12_381MlResult{Inner: &bls.GT{}} // Empty GT element for testing

	typ := ml.Typ()

	if _, ok := typ.(*TBls12_381MlResult); !ok {
		t.Errorf("Expected TBls12_381MlResult, got %T", typ)
	}
}

// Test that all constant types implement IConstant interface
func TestIConstantInterfaceCompliance(t *testing.T) {
	var _ IConstant = Integer{}
	var _ IConstant = ByteString{}
	var _ IConstant = String{}
	var _ IConstant = Unit{}
	var _ IConstant = Bool{}
	var _ IConstant = ProtoList{}
	var _ IConstant = ProtoPair{}
	var _ IConstant = Data{}
	var _ IConstant = Bls12_381G1Element{}
	var _ IConstant = Bls12_381G2Element{}
	var _ IConstant = Bls12_381MlResult{}
}
