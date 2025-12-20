package lex

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestUnknownNamedEscapeMessageContainsFullName(t *testing.T) {
	// Input is a quoted string containing an unknown named escape \INVALID
	input := `"\INVALID"`
	lexer := NewLexer(input)
	tok := lexer.NextToken()
	switch tok.Type {
	case TokenError:
		if !strings.Contains(tok.Literal, "\\INVALID") {
			t.Fatalf("token literal %q does not contain full escape sequence \\INVALID", tok.Literal)
		}
	case TokenString:
		if tok.Literal != "\\INVALID" {
			t.Fatalf("TokenString literal %q not equal to \\INVALID", tok.Literal)
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
		lexer := NewLexer(input)
		for {
			token := lexer.NextToken()
			if token.Type == TokenEOF {
				break
			}
			// Just ensure no panic
		}
	})
}
