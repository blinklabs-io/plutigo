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
	"errors"
	"fmt"
	"unicode"
)

// escapeWindow is the number of characters, starting at the backslash, that
// Text.Megaparsec.Char.Lexer.charLiteral hands to readLitChar. An escape
// never extends past it, so a numeric escape reads at most nine decimal or
// eight octal or hexadecimal digits.
const escapeWindow = 10

var simpleEscapes = map[rune]rune{
	'a':  '\a',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'v':  '\v',
	'\\': '\\',
	'"':  '"',
	'\'': '\'',
}

// asciiEscapes lists the named escapes of GHC.Read.Lex.lexAscii. SOH comes
// before SO, which is a prefix of it; no other name is a prefix of another.
var asciiEscapes = []struct {
	name string
	char rune
}{
	{"SOH", 0x01},
	{"SO", 0x0e},
	{"NUL", 0x00},
	{"STX", 0x02},
	{"ETX", 0x03},
	{"EOT", 0x04},
	{"ENQ", 0x05},
	{"ACK", 0x06},
	{"BEL", 0x07},
	{"BS", 0x08},
	{"HT", 0x09},
	{"LF", 0x0a},
	{"VT", 0x0b},
	{"FF", 0x0c},
	{"CR", 0x0d},
	{"SI", 0x0f},
	{"DLE", 0x10},
	{"DC1", 0x11},
	{"DC2", 0x12},
	{"DC3", 0x13},
	{"DC4", 0x14},
	{"NAK", 0x15},
	{"SYN", 0x16},
	{"ETB", 0x17},
	{"CAN", 0x18},
	{"EM", 0x19},
	{"SUB", 0x1a},
	{"ESC", 0x1b},
	{"FS", 0x1c},
	{"GS", 0x1d},
	{"RS", 0x1e},
	{"US", 0x1f},
	{"SP", 0x20},
	{"DEL", 0x7f},
}

// readEscape decodes the escape at the start of w, which begins with a
// backslash and holds at most escapeWindow characters, as GHC's readLitChar
// does. It returns the character and the number of characters the escape
// spans.
func readEscape(w []rune) (rune, int, error) {
	if len(w) < 2 {
		return 0, 0, errors.New("unterminated escape sequence")
	}
	c := w[1]
	if r, ok := simpleEscapes[c]; ok {
		return r, 2, nil
	}
	base, start := 10, 1
	switch c {
	case 'o':
		base, start = 8, 2
	case 'x':
		base, start = 16, 2
	}
	if start == 2 || isDigitIn(c, 10) {
		end := start
		var value int64
		for end < len(w) && isDigitIn(w[end], base) {
			value = value*int64(base) + int64(digitValue(w[end]))
			end++
		}
		if end > start {
			if value > unicode.MaxRune {
				return 0, 0, fmt.Errorf(
					"escape sequence out of range: %s",
					string(w[:end]),
				)
			}
			return rune(value), end, nil
		}
	}
	if c == '^' && len(w) >= 3 && w[2] >= '@' && w[2] <= '_' {
		return w[2] - '@', 3, nil
	}
	for _, e := range asciiEscapes {
		if hasRunePrefix(w[1:], e.name) {
			return e.char, 1 + len(e.name), nil
		}
	}
	end := 2
	for end < len(w) && unicode.IsLetter(w[end]) && unicode.IsLetter(c) {
		end++
	}
	return 0, 0, fmt.Errorf("unknown escape sequence: %s", string(w[:end]))
}

// skipEmptyEscapes returns the end of the empty escapes (\&) that start at
// w[n], as GHC's lexChar consumes them after a character.
func skipEmptyEscapes(w []rune, n int) int {
	for n+1 < len(w) && w[n] == '\\' && w[n+1] == '&' {
		n += 2
	}
	return n
}

func isDigitIn(r rune, base int) bool {
	switch base {
	case 8:
		return r >= '0' && r <= '7'
	case 16:
		return isHexDigit(r)
	default:
		return r >= '0' && r <= '9'
	}
}

func digitValue(r rune) int {
	switch {
	case r >= 'a':
		return int(r-'a') + 10
	case r >= 'A':
		return int(r-'A') + 10
	default:
		return int(r - '0')
	}
}

func hasRunePrefix(w []rune, prefix string) bool {
	i := 0
	for _, r := range prefix {
		if i >= len(w) || w[i] != r {
			return false
		}
		i++
	}
	return true
}
