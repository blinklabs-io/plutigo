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
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

func TestDecodeBorrowedWithScopeCheck(t *testing.T) {
	freeVar := func(index DeBruijn) Term[DeBruijn] {
		return &Var[DeBruijn]{Name: index}
	}
	programs := []struct {
		name       string
		term       Term[DeBruijn]
		full       bool
		wantIndex  DeBruijn
		wantScope  int
		wantScoped bool
	}{
		{
			name: "closed lambda",
			term: &Lambda[DeBruijn]{Body: freeVar(1)},
		},
		{
			name:       "open variable",
			term:       freeVar(1),
			wantIndex:  1,
			wantScope:  0,
			wantScoped: true,
		},
		{
			name: "open variable in constr under NoCaseConstr",
			term: &Constr[DeBruijn]{Fields: []Term[DeBruijn]{freeVar(1)}},
		},
		{
			name:       "open variable in constr under Full",
			term:       &Constr[DeBruijn]{Fields: []Term[DeBruijn]{freeVar(1)}},
			full:       true,
			wantIndex:  1,
			wantScope:  0,
			wantScoped: true,
		},
		{
			name: "open variable in case under NoCaseConstr",
			term: &Case[DeBruijn]{Constr: freeVar(1), Branches: []Term[DeBruijn]{freeVar(2)}},
		},
		{
			name:       "open variable in case under Full",
			term:       &Case[DeBruijn]{Constr: freeVar(1), Branches: []Term[DeBruijn]{freeVar(2)}},
			full:       true,
			wantIndex:  2,
			wantScope:  0,
			wantScoped: true,
		},
		{
			name:       "last decoded free variable matches validator order",
			term:       &Apply[DeBruijn]{Function: freeVar(2), Argument: freeVar(1)},
			wantIndex:  1,
			wantScope:  0,
			wantScoped: true,
		},
	}

	for _, tt := range programs {
		t.Run(tt.name, func(t *testing.T) {
			flat, err := Encode(&Program[DeBruijn]{
				Version: lang.LanguageVersionV2,
				Term:    tt.term,
			})
			if err != nil {
				t.Fatal(err)
			}
			decoder := NewDeBruijnDecoder()
			_, err = decoder.DecodeBorrowedWithScopeCheck(flat, tt.full)
			if !tt.wantScoped {
				if err != nil {
					t.Fatalf("DecodeBorrowedWithScopeCheck() error = %v, want nil", err)
				}
				return
			}
			var scopeErr *DeBruijnScopeError
			if !errors.As(err, &scopeErr) {
				t.Fatalf("DecodeBorrowedWithScopeCheck() error = %v, want DeBruijnScopeError", err)
			}
			if scopeErr.Index != tt.wantIndex || scopeErr.Scope != tt.wantScope {
				t.Fatalf("scope error = %+v, want index %d at scope %d", scopeErr, tt.wantIndex, tt.wantScope)
			}
		})
	}
}

func TestDecodeBorrowedWithScopeCheckKeepsDecodeErrorPrecedence(t *testing.T) {
	flat, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV2,
		Term:    &Var[DeBruijn]{Name: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	flat = append(flat, 0)
	_, err = NewDeBruijnDecoder().DecodeBorrowedWithScopeCheck(flat, true)
	if err == nil || err.Error() != "trailing bytes after program: 1 byte(s) not consumed" {
		t.Fatalf("DecodeBorrowedWithScopeCheck() error = %v, want trailing-byte error", err)
	}
}

func TestDecodeBorrowedFailureClearsPreviousDecode(t *testing.T) {
	fields := make([]Term[DeBruijn], 4)
	for i := range fields {
		fields[i] = &Apply[DeBruijn]{
			Function: &Constant{Con: &ByteString{Inner: []byte{byte(i)}}},
			Argument: &Error{},
		}
	}
	previous, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV2,
		Term:    &Constr[DeBruijn]{Fields: fields},
	})
	if err != nil {
		t.Fatal(err)
	}

	decoder := NewDeBruijnDecoder()
	if _, err := decoder.DecodeBorrowed(previous); err != nil {
		t.Fatal(err)
	}

	failed, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV2,
		Term:    &Error{},
	})
	if err != nil {
		t.Fatal(err)
	}
	failed = append(failed, 0)
	if _, err := decoder.DecodeBorrowed(failed); err == nil || err.Error() !=
		"trailing bytes after program: 1 byte(s) not consumed" {
		t.Fatalf("DecodeBorrowed() error = %v, want trailing-byte error", err)
	}

	for _, chunk := range decoder.arena.applies.chunks {
		for _, apply := range chunk {
			if apply.Function != nil || apply.Argument != nil {
				t.Fatal("failed decode retained an Apply from the previous decode")
			}
		}
	}
	for _, chunk := range decoder.arena.termLists.chunks {
		for _, term := range chunk {
			if term != nil {
				t.Fatal("failed decode retained a term list from the previous decode")
			}
		}
	}
	for _, chunk := range decoder.consts.byteStrings.chunks {
		for _, byteString := range chunk {
			if byteString.Inner != nil {
				t.Fatal("failed decode retained a byte string from the previous decode")
			}
		}
	}
}
