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

package cek

import (
	"errors"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestBoolConstantReturnsDistinctValues(t *testing.T) {
	first := boolConstant(true)
	second := boolConstant(true)

	firstBool, ok := first.Constant.(*syn.Bool)
	if !ok || !firstBool.Inner {
		t.Fatalf("boolConstant(true) = %#v, want true bool constant", first)
	}
	secondBool, ok := second.Constant.(*syn.Bool)
	if !ok || !secondBool.Inner {
		t.Fatalf("boolConstant(true) second call = %#v, want true bool constant", second)
	}
	if first != second {
		t.Fatal("boolConstant(true) should reuse the cached constant")
	}

	falseFirst := boolConstant(false)
	falseSecond := boolConstant(false)
	falseBool, ok := falseFirst.Constant.(*syn.Bool)
	if !ok || falseBool.Inner {
		t.Fatalf("boolConstant(false) = %#v, want false bool constant", falseFirst)
	}
	if falseFirst != falseSecond {
		t.Fatal("boolConstant(false) should reuse the cached constant")
	}
}

func TestCloneConstantCopiesIntegerValue(t *testing.T) {
	original := &syn.Integer{}
	original.SetInner(big.NewInt(7))

	constant, err := cloneConstant(original)
	if err != nil {
		t.Fatalf("cloneConstant returned error: %v", err)
	}
	cloned, ok := constant.(*syn.Integer)
	if !ok {
		t.Fatalf("cloneConstant returned %T, want *syn.Integer", cloned)
	}
	if cloned.Inner == nil || cloned.Inner.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("cloned integer Inner = %v, want 7", cloned.Inner)
	}
	if cloned.Inner == original.Inner {
		t.Fatal("cloneConstant reused the original big.Int pointer")
	}
}

func TestCloneConstantRejectsNilInteger(t *testing.T) {
	_, err := cloneConstant(&syn.Integer{})
	var typeErr *TypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("cloneConstant error = %T %v, want TypeError", err, err)
	}
	if typeErr.Code != ErrCodeTypeMismatch {
		t.Fatalf("error code = %d, want %d", typeErr.Code, ErrCodeTypeMismatch)
	}
}

func TestCloneConstantRejectsMalformedConstant(t *testing.T) {
	_, err := cloneConstant((*syn.String)(nil))
	var typeErr *TypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("cloneConstant error = %T %v, want TypeError", err, err)
	}
	if typeErr.Code != ErrCodeTypeMismatch {
		t.Fatalf("error code = %d, want %d", typeErr.Code, ErrCodeTypeMismatch)
	}
}

func TestCloneConstantSnapshotsDecodedValue(t *testing.T) {
	encodeValue := func(quantity int64) []byte {
		t.Helper()
		entries := []syn.IConstant{
			&syn.ProtoPair{
				FstType: &syn.TByteString{},
				SndType: &syn.TList{Typ: &syn.TPair{
					First:  &syn.TByteString{},
					Second: &syn.TInteger{},
				}},
				First: &syn.ByteString{Inner: []byte{1}},
				Second: &syn.ProtoList{
					LTyp: &syn.TPair{
						First:  &syn.TByteString{},
						Second: &syn.TInteger{},
					},
					List: []syn.IConstant{&syn.ProtoPair{
						FstType: &syn.TByteString{},
						SndType: &syn.TInteger{},
						First:   &syn.ByteString{Inner: []byte{2}},
						Second:  &syn.Integer{Inner: big.NewInt(quantity)},
					}},
				},
			},
		}
		value, err := syn.NewValue(entries)
		if err != nil {
			t.Fatalf("NewValue: %v", err)
		}
		encoded, err := syn.Encode(&syn.Program[syn.DeBruijn]{
			Version: lang.LanguageVersionV4,
			Term:    &syn.Constant{Con: value},
		})
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		return encoded
	}

	decoder := syn.NewDeBruijnDecoder()
	program, err := decoder.Decode(encodeValue(1))
	if err != nil {
		t.Fatalf("decode initial program: %v", err)
	}
	original := program.Term.(*syn.Constant).Con.(*syn.Value)
	clonedConstant, err := cloneConstant(original)
	if err != nil {
		t.Fatalf("cloneConstant: %v", err)
	}
	cloned := clonedConstant.(*syn.Value)

	if _, err := decoder.Decode(encodeValue(9)); err != nil {
		t.Fatalf("reuse decoder: %v", err)
	}
	got, ok := cloned.LookupCoin([]byte{1}, []byte{2})
	if !ok || got.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("cloned Value quantity = %v, %v; want 1, true", got, ok)
	}
}
