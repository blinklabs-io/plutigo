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
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer scans the original input string in place. It never materializes a
// []rune copy of the input. pos and readPos are rune indices (token positions
// are reported in runes); bytePos and byteNext are the matching byte offsets
// into input. Invalid UTF-8 decodes one U+FFFD per bad byte, exactly like a
// []rune(string) conversion would.
type Lexer struct {
	input    string
	pos      int // rune index of ch (may run past the end after EOF)
	readPos  int // rune index of the next rune
	bytePos  int // byte offset of ch (clamped to len(input))
	byteNext int // byte offset of the next rune
	ch       rune
	// runeLen is the total number of runes in input.
	runeLen int
}

func NewLexer(input string) *Lexer {
	l := &Lexer{input: input, runeLen: utf8.RuneCountInString(input)}

	l.readChar()

	return l
}

// decodeAt decodes the rune at byte offset i, which must be < len(l.input).
func (l *Lexer) decodeAt(i int) (rune, int) {
	if c := l.input[i]; c < utf8.RuneSelf {
		return rune(c), 1
	}

	return utf8.DecodeRuneInString(l.input[i:])
}

func (l *Lexer) readChar() {
	if l.byteNext >= len(l.input) {
		l.ch = 0
		l.bytePos = len(l.input)
	} else {
		r, w := l.decodeAt(l.byteNext)
		l.ch = r
		l.bytePos = l.byteNext
		l.byteNext += w
	}

	l.pos = l.readPos

	l.readPos++
}

// atEOF reports whether the lexer has consumed all input. EOF is tracked by
// position rather than by l.ch, so an embedded U+0000 is not mistaken for it.
func (l *Lexer) atEOF() bool {
	return l.pos >= l.runeLen
}

func (l *Lexer) peekChar() rune {
	if l.byteNext >= len(l.input) {
		return 0
	}

	r, _ := l.decodeAt(l.byteNext)

	return r
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
			for l.ch != '\n' && !l.atEOF() {
				l.readChar()
			}

			// If we hit a newline, continue to check for more whitespace or comments
			if l.ch == '\n' {
				l.readChar()

				continue
			}

			// If we hit EOF, break
			if l.atEOF() {
				break
			}
		} else {
			// No comment, exit loop
			break
		}
	}
}

func (l *Lexer) readIdentifier() string {
	start := l.bytePos
	for unicode.IsLetter(l.ch) || unicode.IsDigit(l.ch) || l.ch == '_' || l.ch == '\'' || l.ch == '-' {
		l.readChar()
	}

	return l.input[start:l.bytePos]
}

// MaxIntegerDigits is the largest number of decimal digits accepted in a
// textual integer literal. Decimal to big integer conversion is quadratic in
// the digit count, so unbounded literals are a DoS vector. 100,000 digits is
// roughly a 332,000 bit integer, far beyond anything real tooling emits;
// on-chain scripts are FLAT encoded and are not affected by this limit.
const MaxIntegerDigits = 100_000

// ErrIntegerTooLong is reported when an integer literal exceeds
// MaxIntegerDigits. It is carried in Token.Value of the resulting TokenError.
var ErrIntegerTooLong = errors.New("integer literal too long")

func (l *Lexer) readNumber() (string, error) {
	start := l.pos
	byteStart := l.bytePos

	if l.ch == '-' || l.ch == '+' {
		l.readChar() // Consume the minus sign
	}

	if !unicode.IsDigit(l.ch) {
		return "", fmt.Errorf("expected digit after sign at position %d", l.pos)
	}

	digitsStart := l.pos
	for unicode.IsDigit(l.ch) {
		l.readChar()
		// Bound the scan so decimal conversion (quadratic in the digit
		// count) cannot be abused with a huge literal.
		if l.pos-digitsStart > MaxIntegerDigits {
			return "", fmt.Errorf(
				"%w: more than %d digits at position %d",
				ErrIntegerTooLong,
				MaxIntegerDigits,
				start,
			)
		}
	}

	return l.input[byteStart:l.bytePos], nil
}

func isHexDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9') ||
		(ch >= 'a' && ch <= 'f') ||
		(ch >= 'A' && ch <= 'F')
}

// readString reads a string literal after its opening quote, as the
// reference parser's conText does with Text.Megaparsec.Char.Lexer.charLiteral:
// an escape is decoded by readLitChar from at most escapeWindow characters
// starting at the backslash.
func (l *Lexer) readString() (string, error) {
	var sb strings.Builder
	var window [escapeWindow]rune
	for {
		l.readChar()
		if l.atEOF() {
			return "", fmt.Errorf("unterminated string at position %d", l.pos)
		}
		if l.ch == '"' {
			l.readChar() // Consume closing quote

			return sb.String(), nil
		}
		if l.ch != '\\' {
			sb.WriteRune(l.ch)

			continue
		}
		r, n, err := readEscape(l.lookahead(window[:0]))
		if err != nil {
			return "", fmt.Errorf("%w at position %d", err, l.pos)
		}
		// Data.Text replaces a surrogate code point with U+FFFD, as
		// WriteRune does.
		sb.WriteRune(r)
		for range n - 1 {
			l.readChar()
		}
	}
}

// lookahead appends to buf the current character and those after it, up to
// cap(buf) characters or the end of input, without consuming them.
func (l *Lexer) lookahead(buf []rune) []rune {
	buf = append(buf, l.ch)
	for i := l.byteNext; len(buf) < cap(buf) && i < len(l.input); {
		r, w := l.decodeAt(i)
		buf = append(buf, r)
		i += w
	}
	return buf
}

func (l *Lexer) readByteString() (string, error) {
	start := l.pos + 1         // Skip prefix (# or x), rune index
	byteStart := l.bytePos + 1 // prefix is a single ASCII byte

	for {
		l.readChar()

		switch {
		case l.atEOF(),
			unicode.IsSpace(l.ch),
			l.ch == ')',
			l.ch == ']',
			l.ch == ',':
			literal := l.input[byteStart:l.bytePos]

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

// readHexToken reads a byte string literal (after # or 0x) into tok as a
// token of type typ.
func (l *Lexer) readHexToken(tok Token, typ TokenType) Token {
	literal, err := l.readByteString()
	if err != nil {
		tok.Type = TokenError
		tok.Literal = err.Error()

		return tok
	}

	bytes, err := hex.DecodeString(literal)
	if err != nil {
		tok.Type = TokenError
		tok.Literal = err.Error()

		return tok
	}

	tok.Type = typ
	tok.Literal = literal
	tok.Value = bytes

	return tok
}

// readNumberToken reads a decimal integer literal into tok. A readNumber
// error is also carried in tok.Value so the parser can report typed errors
// such as ErrIntegerTooLong.
func (l *Lexer) readNumberToken(tok Token) Token {
	literal, err := l.readNumber()
	if err != nil {
		tok.Type = TokenError
		tok.Literal = err.Error()
		tok.Value = err

		return tok
	}

	n, ok := new(big.Int).SetString(literal, 10)
	if !ok {
		tok.Type = TokenError
		tok.Literal = fmt.Sprintf(
			"invalid number %s at position %d",
			literal,
			l.pos,
		)

		return tok
	}

	tok.Type = TokenNumber
	tok.Literal = literal
	tok.Value = n

	return tok
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	tok := Token{Position: l.pos}

	if l.atEOF() {
		tok.Type = TokenEOF

		return tok
	}

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
		return l.readHexToken(tok, TokenByteString)
	case '0':
		if l.peekChar() == 'x' {
			l.readChar() // Consume '0'

			return l.readHexToken(tok, TokenPoint)
		}

		return l.readNumberToken(tok)
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
			case "array":
				tok.Type = TokenArray
			case "List":
				tok.Type = TokenPlutusList
			case "Map":
				tok.Type = TokenMap
			case "constr":
				tok.Type = TokenConstr
			case "Constr":
				tok.Type = TokenPlutusConstr
			case "V":
				tok.Type = TokenPlutusValue
			default:
				tok.Type = TokenIdentifier
			}

			return tok
		} else if unicode.IsDigit(l.ch) || l.ch == '-' || l.ch == '+' {
			return l.readNumberToken(tok)
		} else {
			tok.Type = TokenError

			tok.Literal = fmt.Sprintf("unexpected character %c at position %d", l.ch, l.pos)

			l.readChar()
		}
	}

	return tok
}
