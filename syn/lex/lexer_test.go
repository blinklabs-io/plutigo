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
	"math/big"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUnknownNamedEscapeMessageContainsFullName(t *testing.T) {
	// Input is a quoted string containing an unknown named escape \INVALID
	input := `"\INVALID"`
	lexer := NewLexer(input)
	tok := lexer.NextToken()
	switch tok.Type {
	case TokenError:
		if !strings.Contains(tok.Literal, "\\INVALID") {
			t.Fatalf(
				"token literal %q does not contain full escape sequence \\INVALID",
				tok.Literal,
			)
		}
	case TokenString:
		if tok.Literal != "\\INVALID" {
			t.Fatalf(
				"TokenString literal %q not equal to \\INVALID",
				tok.Literal,
			)
		}
	default:
		t.Fatalf("unexpected token type %v (literal %q)", tok.Type, tok.Literal)
	}
}

func TestLexerNextToken(t *testing.T) {
	input := `(program 1.0.0 (con integer 42))`

	expectedTokens := []Token{
		{Type: TokenLParen, Literal: "(", Position: 0},
		{Type: TokenProgram, Literal: "program", Position: 1},
		{Type: TokenNumber, Literal: "1", Position: 9, Value: big.NewInt(1)},
		{Type: TokenDot, Literal: ".", Position: 10},
		{Type: TokenNumber, Literal: "0", Position: 11, Value: big.NewInt(0)},
		{Type: TokenDot, Literal: ".", Position: 12},
		{Type: TokenNumber, Literal: "0", Position: 13, Value: big.NewInt(0)},
		{Type: TokenLParen, Literal: "(", Position: 15},
		{Type: TokenCon, Literal: "con", Position: 16},
		{Type: TokenIdentifier, Literal: "integer", Position: 20},
		{Type: TokenNumber, Literal: "42", Position: 28, Value: big.NewInt(42)},
		{Type: TokenRParen, Literal: ")", Position: 30},
		{Type: TokenRParen, Literal: ")", Position: 31},
		{Type: TokenEOF, Literal: "", Position: 32},
	}

	lexer := NewLexer(input)

	for i, expected := range expectedTokens {
		token := lexer.NextToken()
		if token.Type != expected.Type {
			t.Errorf(
				"Token %d: expected type %v, got %v",
				i,
				expected.Type,
				token.Type,
			)
		}
		if token.Literal != expected.Literal {
			t.Errorf(
				"Token %d: expected literal %q, got %q",
				i,
				expected.Literal,
				token.Literal,
			)
		}
		if token.Position != expected.Position {
			t.Errorf(
				"Token %d: expected position %d, got %d",
				i,
				expected.Position,
				token.Position,
			)
		}
		if expected.Value != nil {
			if !reflect.DeepEqual(token.Value, expected.Value) {
				t.Errorf(
					"Token %d: expected value %v, got %v",
					i,
					expected.Value,
					token.Value,
				)
			}
		}
	}
}

func TestLexerStrings(t *testing.T) {
	input := `"hello world" "with \"quotes\""`

	expectedTokens := []Token{
		{
			Type:     TokenString,
			Literal:  "hello world",
			Position: 0,
			Value:    "hello world",
		},
		{
			Type:     TokenString,
			Literal:  `with "quotes"`,
			Position: 14,
			Value:    `with "quotes"`,
		},
		{Type: TokenEOF, Literal: "", Position: 31},
	}

	lexer := NewLexer(input)

	for i, expected := range expectedTokens {
		token := lexer.NextToken()
		if token.Type != expected.Type {
			t.Errorf(
				"Token %d: expected type %v, got %v",
				i,
				expected.Type,
				token.Type,
			)
		}
		if token.Literal != expected.Literal {
			t.Errorf(
				"Token %d: expected literal %q, got %q",
				i,
				expected.Literal,
				token.Literal,
			)
		}
		if token.Position != expected.Position {
			t.Errorf(
				"Token %d: expected position %d, got %d",
				i,
				expected.Position,
				token.Position,
			)
		}
		if expected.Value != nil {
			if token.Value != expected.Value {
				t.Errorf(
					"Token %d: expected value %v, got %v",
					i,
					expected.Value,
					token.Value,
				)
			}
		}
	}
}

// TestLexerStringNumericEscapes pins string escapes to GHC's readLitChar as
// Text.Megaparsec.Char.Lexer.charLiteral applies it: numeric escapes take
// every digit within ten characters of the backslash.
func TestLexerStringNumericEscapes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"hex ascii", `"\x41"`, "A"},
		{"hex ascii leading zeros", `"\x0041"`, "A"},
		{"hex one digit", `"\x9"`, "\t"},
		{"hex latin1", `"\xe9"`, "é"},
		{"hex bmp", `"\x20AC"`, "€"},
		{"hex beyond the bmp", `"\x1F600"`, "\U0001F600"},
		{"hex nul", `"\x00"`, "\x00"},
		{"hex stops at non-hex", `"\x41!"`, "A!"},
		{"hex stops at the window", `"\x0000000041"`, "\x0041"},
		{"hex in text", `"caf\xe9"`, "café"},
		{"decimal", `"\233"`, "é"},
		{"decimal six digits", `"\128512"`, "\U0001F600"},
		{"decimal largest code point", `"\1114111"`, "\U0010FFFF"},
		{"decimal stops at the window", `"\0000000065"`, "\x065"},
		{"octal", `"\o351"`, "é"},
		{"octal seven digits", `"\o1750000"`, "\U0007D000"},
		{"octal stops at non-octal", `"\o18"`, "\x018"},
		// Surrogate code points cannot be encoded in UTF-8; they become
		// U+FFFD, matching Data.Text's replacement behavior
		{"decimal surrogate", `"\55296"`, "\uFFFD"},
		{"named", `"\NUL\BEL\DEL"`, "\x00\a\x7f"},
		{"named SOH before SO", `"\SOH"`, "\x01"},
		{"named SO", `"\SO"`, "\x0e"},
		{"named SO then a letter", `"\SOX"`, "\x0eX"},
		{"named followed by letters", `"\DELx\SOHH"`, "\x7fx\x01H"},
		{"named space", `"\SP"`, " "},
		{"control at", `"\^@"`, "\x00"},
		{"control letter", `"\^A"`, "\x01"},
		{"control escape", `"\^["`, "\x1b"},
		{"control underscore", `"\^_"`, "\x1f"},
		{"single quote", `"\'"`, "'"},
		{"empty escape after a character", `"a\&b"`, "ab"},
		{"empty escapes after a character", `"a\&\&b"`, "ab"},
		{"empty escape ends a numeric escape", `"\65\&5"`, "A5"},
		{"empty escape ends SO", `"\SO\&H"`, "\x0eH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			token := lexer.NextToken()

			if token.Type != TokenString {
				t.Fatalf(
					"expected TokenString, got %v (literal %q)",
					token.Type,
					token.Literal,
				)
			}
			if token.Literal != tt.expected {
				t.Errorf(
					"input %s: expected literal %q, got %q",
					tt.input,
					tt.expected,
					token.Literal,
				)
			}
			if !utf8.ValidString(token.Literal) {
				t.Errorf(
					"input %s: literal %q is not valid UTF-8",
					tt.input,
					token.Literal,
				)
			}
		})
	}
}

func TestLexerStringNumericEscapeErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unicode escape", `"\u0041"`},
		{"hex no digits", `"\xzz"`},
		{"hex out of range", `"\x110000"`},
		{"decimal out of range", `"\1114112"`},
		{"octal no digits", `"\o8"`},
		{"control lowercase", `"\^a"`},
		{"empty escape", `"\&"`},
		{"hex beyond 32 bits", `"\x80000000"`},
		{"hex largest window value", `"\xFFFFFFFF"`},
		{"unknown letter", `"\q"`},
		{"escape at end of input", `"\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			token := lexer.NextToken()

			if token.Type != TokenError {
				t.Fatalf(
					"input %s: expected TokenError, got %v (literal %q)",
					tt.input,
					token.Type,
					token.Literal,
				)
			}
		})
	}
}

// TestLexerStringEscapeErrorPosition checks that an escape error reports the
// escape's position in runes.
func TestLexerStringEscapeErrorPosition(t *testing.T) {
	token := NewLexer(`"éé\q"`).NextToken()
	if token.Type != TokenError {
		t.Fatalf("expected TokenError, got %v (literal %q)", token.Type, token.Literal)
	}
	if !strings.Contains(token.Literal, "at position 3") {
		t.Fatalf("error %q does not report position 3", token.Literal)
	}
}

func TestLexerByteString(t *testing.T) {
	input := `#aaBBcc`

	expected := Token{
		Type:     TokenByteString,
		Literal:  "aaBBcc",
		Position: 0,
		Value:    []byte{0xaa, 0xBB, 0xcc},
	}

	lexer := NewLexer(input)
	token := lexer.NextToken()

	if token.Type != expected.Type {
		t.Errorf("Expected type %v, got %v", expected.Type, token.Type)
	}
	if token.Literal != expected.Literal {
		t.Errorf("Expected literal %q, got %q", expected.Literal, token.Literal)
	}
	if token.Position != expected.Position {
		t.Errorf(
			"Expected position %d, got %d",
			expected.Position,
			token.Position,
		)
	}
	if !reflect.DeepEqual(token.Value, expected.Value) {
		t.Errorf("Expected value %v, got %v", expected.Value, token.Value)
	}
}

func TestLexerKeywords(t *testing.T) {
	input := `lam delay force builtin constr case con error program list pair I B List Map Constr True False ()`

	keywords := []TokenType{
		TokenLam, TokenDelay, TokenForce, TokenBuiltin, TokenConstr, TokenCase, TokenCon, TokenErrorTerm,
		TokenProgram, TokenList, TokenPair, TokenI, TokenB, TokenPlutusList, TokenMap, TokenPlutusConstr,
		TokenTrue, TokenFalse, TokenUnit,
	}

	lexer := NewLexer(input)
	position := 0

	for _, expectedType := range keywords {
		token := lexer.NextToken()
		if token.Type != expectedType {
			t.Errorf(
				"Expected type %v at position %d, got %v",
				expectedType,
				position,
				token.Type,
			)
		}
		position = token.Position + len(token.Literal)
	}
}

func FuzzLexerNextToken(f *testing.F) {
	testInputs := []string{
		`(program 1.2.0 (con integer 42))`,
		`"hello"`,
		`#aaBBcc`,
		`lam delay force`,
	}
	for _, input := range testInputs {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}

		lexer := NewLexer(input)
		maxTokens := len([]rune(input)) + 1
		for i := 0; i < maxTokens; i++ {
			token := lexer.NextToken()
			if token.Type == TokenEOF {
				return
			}
		}
		t.Fatalf("lexer did not reach EOF after %d tokens", maxTokens)
	})
}

// TestLexerEmbeddedNulIsNotEOF checks that the lexer reports a NUL between
// tokens as an error token rather than end of input, and then keeps reading:
// the closing parenthesis after the NUL must still be returned, followed by
// TokenEOF only once the input is really exhausted.
func TestLexerEmbeddedNulIsNotEOF(t *testing.T) {
	lexer := NewLexer("(\x00)")
	expected := []TokenType{TokenLParen, TokenError, TokenRParen, TokenEOF}
	for i, want := range expected {
		token := lexer.NextToken()
		if token.Type != want {
			t.Fatalf(
				"token %d: expected type %v, got %v (literal %q)",
				i,
				want,
				token.Type,
				token.Literal,
			)
		}
	}
}

// TestLexerEmbeddedNulInStringAndByteString checks NUL handling inside
// literals: a NUL inside a string literal is kept as a character, a NUL
// inside a bytestring is rejected as an invalid hex character, and a NUL in an
// unclosed string does not hide the missing closing quote.
func TestLexerEmbeddedNulInStringAndByteString(t *testing.T) {
	token := NewLexer("\"a\x00b\"").NextToken()
	if token.Type != TokenString || token.Value != "a\x00b" {
		t.Fatalf(
			"expected string token with embedded NUL, got %v (literal %q)",
			token.Type,
			token.Literal,
		)
	}
	// A NUL is not a hex digit, so the bytestring is invalid.
	token = NewLexer("#00\x0000").NextToken()
	if token.Type != TokenError {
		t.Fatalf(
			"expected error for NUL in bytestring, got %v (literal %q)",
			token.Type,
			token.Literal,
		)
	}
	// The string never closes; the NUL must not be treated as its end.
	token = NewLexer("\"abc\x00").NextToken()
	if token.Type != TokenError ||
		!strings.Contains(token.Literal, "unterminated string") {
		t.Fatalf(
			"expected unterminated string error, got %v (literal %q)",
			token.Type,
			token.Literal,
		)
	}
}
