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

package lex

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHexLiteralDecoding(t *testing.T) {
	tests := []struct {
		name  string
		input string
		typ   TokenType
		want  []byte
	}{
		{"empty", "#", TokenByteString, []byte{}},
		{"mixed case", "#aBcD01", TokenByteString, []byte{0xab, 0xcd, 0x01}},
		{"point", "0xFF00", TokenPoint, []byte{0xff, 0x00}},
		{"large", "#" + strings.Repeat("ab", 4<<20), TokenByteString, bytes.Repeat([]byte{0xab}, 4<<20)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := NewLexer(tt.input).NextToken()
			if tok.Type != tt.typ {
				t.Fatalf("type = %v, want %v (%s)", tok.Type, tt.typ, tok.Literal)
			}
			got, ok := tok.Value.([]byte)
			if !ok || !bytes.Equal(got, tt.want) {
				t.Fatalf("value of %d bytes differs from want of %d bytes", len(got), len(tt.want))
			}
		})
	}
	for _, in := range []string{"#abc", "#zz", "0xabc", "0xzz"} {
		if tok := NewLexer(in).NextToken(); tok.Type != TokenError {
			t.Errorf("%q: type = %v, want TokenError", in, tok.Type)
		}
	}
}

func TestIntegerDigitCap(t *testing.T) {
	atLimit := strings.Repeat("9", MaxIntegerDigits)
	for _, prefix := range []string{"", "-", "+"} {
		if tok := NewLexer(prefix + atLimit).NextToken(); tok.Type != TokenNumber {
			t.Fatalf("%q: at-limit literal rejected: %v", prefix, tok.Literal)
		}
		tok := NewLexer(prefix + atLimit + "9").NextToken()
		if tok.Type != TokenError {
			t.Fatalf("%q: type = %v, want TokenError", prefix, tok.Type)
		}
		err, _ := tok.Value.(error)
		if !errors.Is(err, ErrIntegerTooLong) {
			t.Fatalf("%q: value = %v, want ErrIntegerTooLong", prefix, tok.Value)
		}
	}
}
