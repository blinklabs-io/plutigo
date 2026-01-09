package lex

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

type Lexer struct {
	input   []rune
	pos     int
	readPos int
	ch      rune
}

func NewLexer(input string) *Lexer {
	l := &Lexer{input: []rune(input)}

	l.readChar()

	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
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
	for unicode.IsLetter(l.ch) || unicode.IsDigit(l.ch) || l.ch == '_' || l.ch == '\'' || l.ch == '-' {
		l.readChar()
	}

	return string(l.input[start:l.pos])
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

	return string(l.input[start:l.pos]), nil
}

func (l *Lexer) readString() (string, error) {
	var sb strings.Builder
	leftoverChar := false
	for {
		if !leftoverChar {
			l.readChar()
		}
		leftoverChar = false

		if l.ch == 0 {
			return "", fmt.Errorf("unterminated string at position %d", l.pos)
		}

		if l.ch == '\\' {
			// Read next character to determine escape sequence type
			l.readChar()
			// Check for simple one-char escapes (like \" and \\\)
			tmpEscape := `\` + string(l.ch)
			if val, ok := escapeMap[tmpEscape]; ok {
				sb.WriteString(val)
				continue
			}
			// Unicode escape
			if tmpEscape == `\u` {
				var unicodeHex string
				for {
					l.readChar()
					unicodeHex += string(l.ch)
					tmpHex := fmt.Sprintf("%04s", unicodeHex)
					if _, err := hex.DecodeString(tmpHex); err != nil {
						// Strip off last hex character
						unicodeHex = unicodeHex[:len(unicodeHex)-1]
						leftoverChar = true
						break
					}
					if len(unicodeHex) >= 4 {
						break
					}
				}
				if len(unicodeHex) < 2 {
					return "", fmt.Errorf(
						"unicode escape sequence too short: \\u%s",
						unicodeHex,
					)
				}
				// Pad hex string for parsing
				tmpHex := fmt.Sprintf("%04s", unicodeHex)
				r, err := hex.DecodeString(tmpHex)
				if err != nil {
					return "", fmt.Errorf(
						"invalid unicode escape sequence: \\u%s",
						unicodeHex,
					)
				}
				sb.Write(r)
				continue
			}
			// Hex escape
			if tmpEscape == `\x` {
				var hexStr string
				for {
					l.readChar()
					hexStr += string(l.ch)
					tmpHex := fmt.Sprintf("%04s", hexStr)
					if _, err := hex.DecodeString(tmpHex); err != nil {
						// Strip off last hex character
						hexStr = hexStr[:len(hexStr)-1]
						leftoverChar = true
						break
					}
					if len(hexStr) >= 4 {
						break
					}
				}
				if len(hexStr) < 2 {
					return "", fmt.Errorf(
						"hex escape sequence too short: \\x%s",
						hexStr,
					)
				}
				// Pad hex string for parsing
				tmpHex := fmt.Sprintf("%04s", hexStr)
				// Strip out any leading zero bytes
				tmpHex = strings.TrimPrefix(tmpHex, "00")
				r, err := hex.DecodeString(tmpHex)
				if err != nil {
					return "", fmt.Errorf(
						"invalid hex escape sequence: \\x%s",
						hexStr,
					)
				}
				sb.Write(r)
				continue
			}
			// Octal escape
			if tmpEscape == `\o` {
				var octalStr string
				for {
					l.readChar()
					if l.ch > unicode.MaxASCII || !unicode.IsDigit(l.ch) {
						leftoverChar = true
						break
					}
					octalStr += string(l.ch)
					if len(octalStr) >= 3 {
						break
					}
				}
				tmpOctal, err := strconv.ParseUint(octalStr, 8, 16)
				if err != nil {
					return "", fmt.Errorf(
						"invalid octal escape sequence: \\o%s",
						octalStr,
					)
				}
				sb.WriteRune(rune(tmpOctal))
				continue
			}
			// Handle named escapes (e.g., \DEL) after specific sequences
			if unicode.IsLetter(l.ch) {
				// Read consecutive letters to form the full name (e.g., DEL)
				name := string(l.ch)
				var nameSb232 strings.Builder
				for {
					l.readChar()
					if !unicode.IsLetter(l.ch) {
						// we've read one character too far; mark leftover and stop
						leftoverChar = true
						break
					}
					nameSb232.WriteString(string(l.ch))
				}
				name += nameSb232.String()

				key := `\` + name
				if val, ok := escapeMap[key]; ok {
					sb.WriteString(val)
					continue
				}

				// no valid named escape found; return an error that includes the
				// full name (e.g., \INVALID) so error messages are clear.
				return "", fmt.Errorf("unknown escape sequence: %s", key)
			}
			// Check for unknown non-numeric escape
			if l.ch > unicode.MaxASCII || !unicode.IsDigit(l.ch) {
				return "", fmt.Errorf("unknown escape sequence: %s", tmpEscape)
			}
			// Decimal escape
			decStr := string(l.ch)
			for {
				l.readChar()
				if l.ch > unicode.MaxASCII || !unicode.IsDigit(l.ch) {
					leftoverChar = true
					break
				}
				decStr += string(l.ch)
				if len(decStr) >= 4 {
					break
				}
			}
			tmpDec, err := strconv.ParseUint(decStr, 10, 16)
			if err != nil {
				return "", fmt.Errorf(
					"invalid decimal escape sequence: \\%s",
					decStr,
				)
			}
			sb.WriteRune(rune(tmpDec))
			continue
		}

		if l.ch == '"' {
			l.readChar() // Consume closing quote

			return sb.String(), nil
		}

		sb.WriteRune(l.ch)
	}
}

func (l *Lexer) readByteString() (string, error) {
	start := l.pos + 1 // Skip prefix (# or x)

	for {
		l.readChar()

		switch {
		case l.ch == 0,
			unicode.IsSpace(l.ch),
			l.ch == ')',
			l.ch == ']',
			l.ch == ',':
			literal := string(l.input[start:l.pos])

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
	case '0':
		if l.peekChar() == 'x' {
			l.readChar() // Consume '0'

			literal, err := l.readByteString() // 0x prefix
			if err != nil {
				tok.Type = TokenError
				tok.Literal = err.Error()
				return tok
			}

			tok.Type = TokenPoint
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
		} else {
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
		}
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
