package lex

import (
	"fmt"
	"math/big"
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
	for {
		// Skip standard whitespace
		for unicode.IsSpace(l.ch) {
			l.readChar()
		}

		// Check for comment start
		if l.ch == '-' && l.peekChar() == '-' {
			// Skip the '--'
			l.readChar() // Consume first '-'
			l.readChar() // Consume second '-'

			// Skip until newline or EOF
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}

			// If we hit a newline, continue to check for more whitespace or comments
			if l.ch == '\n' {
				l.readChar()

				continue
			}

			// If we hit EOF, break
			if l.ch == 0 {
				break
			}
		} else {
			// No comment, exit loop
			break
		}
	}
}

func (l *Lexer) readIdentifier() string {
	start := l.pos

	for unicode.IsLetter(l.ch) || unicode.IsDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}

	return l.input[start:l.pos]
}

func (l *Lexer) readNumber() (string, error) {
	start := l.pos

	if l.ch == '-' || l.ch == '+' {
		l.readChar() // Consume the minus sign
	}

	if !unicode.IsDigit(l.ch) {
		return "", fmt.Errorf("expected digit after sign at position %d", l.pos)
	}

	for unicode.IsDigit(l.ch) {
		l.readChar()
	}

	return l.input[start:l.pos], nil
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

		switch {
		case l.ch == 0,
			unicode.IsSpace(l.ch),
			l.ch == ')',
			l.ch == ']',
			l.ch == ',':
			literal := l.input[start:l.pos]

			if len(literal)%2 != 0 {
				return "", fmt.Errorf(
					"bytestring #%s has odd length at position %d",
					literal,
					start-1,
				)
			}

			return literal, nil
		case (l.ch >= '0' && l.ch <= '9'),
			(l.ch >= 'a' && l.ch <= 'f'),
			(l.ch >= 'A' && l.ch <= 'F'):
			// All good, continue
			continue
		default:
			return "", fmt.Errorf(
				"invalid bytestring character %c at position %d",
				l.ch,
				l.pos,
			)
		}
	}
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	tok := Token{Position: l.pos}

	switch l.ch {
	case '(':
		if l.peekChar() == ')' {
			// Handle () as TokenUnit
			tok.Type = TokenUnit

			tok.Literal = "()"

			l.readChar() // Consume (

			l.readChar() // Consume )
		} else {
			tok.Type = TokenLParen

			tok.Literal = string(l.ch)

			l.readChar()
		}
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
	case ',':
		tok.Type = TokenComma

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
			_, err = fmt.Sscanf(literal[i:i+2], "%x", &val)
			if err != nil {
				tok.Type = TokenError
				tok.Literal = err.Error()

				return tok
			}
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

			switch literal {
			case "lam":
				tok.Type = TokenLam
			case "delay":
				tok.Type = TokenDelay
			case "force":
				tok.Type = TokenForce
			case "builtin":
				tok.Type = TokenBuiltin
			case "case":
				tok.Type = TokenCase
			case "con":
				tok.Type = TokenCon
			case "error":
				tok.Type = TokenErrorTerm
			case "program":
				tok.Type = TokenProgram
			case "True":
				tok.Type = TokenTrue
				tok.Value = true
			case "False":
				tok.Type = TokenFalse
				tok.Value = false
			case "pair":
				tok.Type = TokenPair
			case "I":
				tok.Type = TokenI
			case "B":
				tok.Type = TokenB
			case "list":
				tok.Type = TokenList
			case "List":
				tok.Type = TokenPlutusList
			case "Map":
				tok.Type = TokenMap
			case "constr":
				tok.Type = TokenConstr
			case "Constr":
				tok.Type = TokenPlutusConstr
			default:
				tok.Type = TokenIdentifier
			}

			return tok
		} else if unicode.IsDigit(l.ch) || l.ch == '-' || l.ch == '+' {
			literal, err := l.readNumber()
			if err != nil {
				tok.Type = TokenError
				tok.Literal = err.Error()

				return tok
			}

			tok.Type = TokenNumber

			tok.Literal = literal

			n := new(big.Int)

			if _, ok := n.SetString(literal, 10); !ok {
				tok.Type = TokenError

				tok.Literal = fmt.Sprintf("invalid number %s at position %d", literal, l.pos)

				return tok
			}

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
