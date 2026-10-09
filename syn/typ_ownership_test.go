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
	"slices"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

// TList and TPair expose writable fields, so a composite type handed to one
// caller must not be reachable from another constant's type.

func TestProtoListTypIsPrivateToCaller(t *testing.T) {
	a := ProtoList{LTyp: &TInteger{}}
	b := ProtoList{LTyp: &TInteger{}}
	a.Typ().(*TList).Typ = &TBool{}
	if !EqualType(b.Typ(), &TList{Typ: &TInteger{}}) {
		t.Fatalf("independent list type changed to %#v", b.Typ())
	}
	if !EqualType(a.Typ(), &TList{Typ: &TInteger{}}) {
		t.Fatalf("list type changed to %#v", a.Typ())
	}
}

func TestProtoPairTypIsPrivateToCaller(t *testing.T) {
	a := ProtoPair{FstType: &TData{}, SndType: &TData{}}
	b := ProtoPair{FstType: &TData{}, SndType: &TData{}}
	a.Typ().(*TPair).First = &TBool{}
	want := &TPair{First: &TData{}, Second: &TData{}}
	if !EqualType(b.Typ(), want) {
		t.Fatalf("independent pair type changed to %#v", b.Typ())
	}
	if !EqualType(a.Typ(), want) {
		t.Fatalf("pair type changed to %#v", a.Typ())
	}
}

func encodeConstantsProgram(
	t *testing.T,
	first IConstant,
	rest ...IConstant,
) []byte {
	t.Helper()
	var term Term[DeBruijn] = &Constant{Con: first}
	for _, c := range rest {
		term = &Apply[DeBruijn]{Function: term, Argument: &Constant{Con: c}}
	}
	encoded, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV1,
		Term:    term,
	})
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}
	return encoded
}

func decodedConstants(program *Program[DeBruijn]) []IConstant {
	var cons []IConstant
	term := program.Term
	for {
		switch tm := term.(type) {
		case *Apply[DeBruijn]:
			cons = append(cons, tm.Argument.(*Constant).Con)
			term = tm.Function
		case *Constant:
			cons = append(cons, tm.Con)
			slices.Reverse(cons)
			return cons
		}
	}
}

func TestDecodedCompositeConstantTypesAreNotShared(t *testing.T) {
	listOfIntLists := func() IConstant {
		return &ProtoList{LTyp: &TList{Typ: &TInteger{}}, List: []IConstant{}}
	}
	listOfDataPairs := func() IConstant {
		return &ProtoList{
			LTyp: &TPair{First: &TData{}, Second: &TData{}},
			List: []IConstant{},
		}
	}
	encoded := encodeConstantsProgram(
		t,
		listOfIntLists(), listOfIntLists(),
		listOfDataPairs(), listOfDataPairs(),
	)
	for _, decode := range []struct {
		name string
		fn   func([]byte) (*Program[DeBruijn], error)
	}{
		{"fresh", Decode[DeBruijn]},
		{"reused", NewDeBruijnDecoder().Decode},
	} {
		t.Run(decode.name, func(t *testing.T) {
			program, err := decode.fn(encoded)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			cons := decodedConstants(program)
			if len(cons) != 4 {
				t.Fatalf("decoded %d constants, want 4", len(cons))
			}
			cons[0].(*ProtoList).LTyp.(*TList).Typ = &TBool{}
			cons[2].(*ProtoList).LTyp.(*TPair).First = &TBool{}
			if !EqualType(cons[1].Typ(), listOfIntLists().Typ()) {
				t.Fatalf("sibling list type changed to %#v", cons[1].Typ())
			}
			if !EqualType(cons[3].Typ(), listOfDataPairs().Typ()) {
				t.Fatalf("sibling pair list type changed to %#v", cons[3].Typ())
			}

			again, err := Decode[DeBruijn](encoded)
			if err != nil {
				t.Fatalf("second decode failed: %v", err)
			}
			for i, c := range decodedConstants(again) {
				want := listOfIntLists().Typ()
				if i >= 2 {
					want = listOfDataPairs().Typ()
				}
				if !EqualType(c.Typ(), want) {
					t.Fatalf("later decode constant %d has type %#v", i, c.Typ())
				}
			}
		})
	}
}
