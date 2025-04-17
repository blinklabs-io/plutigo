package lex

import (
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

type Lexer struct {
	input   string
	pos     int
	readPos int
	ch      rune
}

func NewLexer(input string) *Lexer {
	l := &Lexer{input: input}

	l.readChar()

	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = rune(l.input[l.readPos])
	}

	l.pos = l.readPos

	l.readPos++
}

func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.input) {
		return 0
	}

	return rune(l.input[l.readPos])
}

func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(l.ch) {
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() string {
	start := l.pos

	for unicode.IsLetter(l.ch) || unicode.IsDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}

	return l.input[start:l.pos]
}

func (l *Lexer) readNumber() string {
	start := l.pos

	for unicode.IsDigit(l.ch) {
		l.readChar()
	}

	return l.input[start:l.pos]
}

func (l *Lexer) readString() (string, error) {
	start := l.pos + 1 // Skip opening quote

	for {
		l.readChar()

		if l.ch == 0 {
			return "", fmt.Errorf("unterminated string at position %d", l.pos)
		}

		if l.ch == '"' {
			l.readChar() // Consume closing quote
			return l.input[start : l.pos-1], nil
		}
	}
}

func (l *Lexer) readByteString() (string, error) {
	start := l.pos + 1 // Skip #

	for {
		l.readChar()

		if l.ch == 0 || unicode.IsSpace(l.ch) || l.ch == ')' || l.ch == ']' {
			return l.input[start:l.pos], nil
		}

		if !((l.ch >= '0' && l.ch <= '9') || (l.ch >= 'a' && l.ch <= 'f') || (l.ch >= 'A' && l.ch <= 'F')) {
			return "", fmt.Errorf("invalid bytestring character %c at position %d", l.ch, l.pos)
		}
	}
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	tok := Token{Position: l.pos}

	switch l.ch {
	case '(':
		tok.Type = TokenLParen

		tok.Literal = string(l.ch)

		l.readChar()
	case ')':
		tok.Type = TokenRParen

		tok.Literal = string(l.ch)

		l.readChar()
	case '[':
		tok.Type = TokenLBracket

		tok.Literal = string(l.ch)

		l.readChar()
	case ']':
		tok.Type = TokenRBracket

		tok.Literal = string(l.ch)

		l.readChar()
	case '.':
		tok.Type = TokenDot

		tok.Literal = string(l.ch)

		l.readChar()
	case '#':
		literal, err := l.readByteString()

		if err != nil {
			tok.Type = TokenError
			tok.Literal = err.Error()
			return tok
		}

		tok.Type = TokenByteString

		tok.Literal = literal

		// Convert hex to bytes
		bytes := make([]byte, len(literal)/2)

		for i := 0; i < len(literal); i += 2 {
			var val uint8
			fmt.Sscanf(literal[i:i+2], "%x", &val)
			bytes[i/2] = val
		}

		tok.Value = bytes
	case '"':
		literal, err := l.readString()

		if err != nil {
			tok.Type = TokenError

			tok.Literal = err.Error()

			return tok
		}

		tok.Type = TokenString

		tok.Literal = literal

		tok.Value = literal
	case 0:
		tok.Type = TokenEOF

		tok.Literal = ""
	default:
		if unicode.IsLetter(l.ch) {
			literal := l.readIdentifier()

			tok.Literal = literal

			switch strings.ToLower(literal) {
			case "lam":
				tok.Type = TokenLam
			case "delay":
				tok.Type = TokenDelay
			case "force":
				tok.Type = TokenForce
			case "builtin":
				tok.Type = TokenBuiltin
			case "constr":
				tok.Type = TokenConstr
			case "case":
				tok.Type = TokenCase
			case "con":
				tok.Type = TokenCon
			case "error":
				tok.Type = TokenErrorTerm
			case "program":
				tok.Type = TokenProgram
			case "true":
				tok.Type = TokenTrue
				tok.Value = true
			case "false":
				tok.Type = TokenFalse
				tok.Value = false
			default:
				tok.Type = TokenIdentifier
			}

			return tok
		} else if unicode.IsDigit(l.ch) {
			literal := l.readNumber()

			tok.Type = TokenNumber

			tok.Literal = literal

			n := new(big.Int)

			n.SetString(literal, 10)

			tok.Value = n

			return tok
		} else {
			tok.Type = TokenError

			tok.Literal = fmt.Sprintf("unexpected character %c at position %d", l.ch, l.pos)

			l.readChar()
		}
	}

	return tok
}
