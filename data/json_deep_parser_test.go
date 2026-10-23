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

package data

import (
	"fmt"
	"strings"
	"testing"
)

const jsonUnexpectedEOF = "unexpected end of JSON input"

// jsonErrorClass reduces a parse result to accepted, truncated, or syntax
// error. The fallback parser's messages differ from the json.Decoder path,
// but the class must not: truncated input is reported as end of input and
// nothing else is. json.Unmarshal is not the reference here because it
// reports some truncations, such as a lone "-", as syntax errors.
func jsonErrorClass(err error) string {
	switch {
	case err == nil:
		return "ok"
	case err.Error() == jsonUnexpectedEOF:
		return "eof"
	default:
		return "syntax"
	}
}

var deepJSONParserClassInputs = []string{
	``,
	` `,
	`{`,
	`{ `,
	`{1`,
	`{1}`,
	`{a`,
	`{"a"`,
	`{"a" `,
	`{"a" 1`,
	`{"a"1}`,
	`{"a":`,
	`{"a":1`,
	`{"a":1,`,
	`{"a":1, `,
	`{"a":1,}`,
	`{"int":0,}`,
	`{"a":1,2}`,
	`{"a":1 2}`,
	`{"a":1}`,
	`{"a":1}x`,
	`{"a":1} `,
	`[`,
	`[1`,
	`[1,`,
	`[1,]`,
	`[1 2]`,
	`[]`,
	`[1,2]`,
	`"`,
	`"abc`,
	`"\`,
	`"\u`,
	`"\u12`,
	`"\u12"`,
	`"\u12g4"`,
	`"ሴ"`,
	`"\x"`,
	`"\x`,
	`"\n"`,
	`"\"`,
	`"\""`,
	"\"a\nb\"",
	`-`,
	`-x`,
	`-0`,
	`01`,
	`1.`,
	`1.x`,
	`1.5`,
	`1e`,
	`1e+`,
	`1e+x`,
	`1E-5`,
	`1ex`,
	`t`,
	`tr`,
	`tru`,
	`true`,
	`trux`,
	`tx`,
	`f`,
	`fals`,
	`falsy`,
	`n`,
	`nul`,
	`null`,
	`nulx`,
	`x`,
	`[tr`,
	`{"a":tr`,
	`{"a":1.`,
	`{"a":-`,
	`{"a":"\u12`,
}

// deepJSONParserClassMismatch compares the fallback parser with the
// json.Decoder path that handles inputs below the standard library's own
// nesting cap.
func deepJSONParserClassMismatch(input string) string {
	_, wantErr := parsePlutusDataJSONTree([]byte(input), MaxDecodeNestingDepth())
	_, gotErr := parseDeepPlutusDataJSONTree([]byte(input), MaxDecodeNestingDepth())
	want, got := jsonErrorClass(wantErr), jsonErrorClass(gotErr)
	if got == want {
		return ""
	}
	return fmt.Sprintf("input %q: fallback class %s (%v), decoder class %s (%v)",
		input, got, gotErr, want, wantErr)
}

func TestDeepJSONParserErrorClassMatchesDecoder(t *testing.T) {
	for _, input := range deepJSONParserClassInputs {
		if msg := deepJSONParserClassMismatch(input); msg != "" {
			t.Error(msg)
		}
	}
}

func FuzzDeepJSONParserErrorClass(f *testing.F) {
	for _, input := range deepJSONParserClassInputs {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if msg := deepJSONParserClassMismatch(input); msg != "" {
			t.Fatal(msg)
		}
	})
}

func TestParseDeepPlutusDataJSONTreeTruncatedDeepInput(t *testing.T) {
	const depth = 5_100
	input := nestedListJSON(depth)
	for _, cut := range []int{1, 2, len(input) / 2} {
		truncated := input[:len(input)-cut]
		_, err := parseDeepPlutusDataJSONTree([]byte(truncated), MaxDecodeNestingDepth())
		if err == nil || err.Error() != jsonUnexpectedEOF {
			t.Errorf("cut %d: error = %v, want %q", cut, err, jsonUnexpectedEOF)
		}
	}
	trailing := []byte(input + ` x`)
	if _, err := parseDeepPlutusDataJSONTree(trailing, MaxDecodeNestingDepth()); err == nil ||
		!strings.Contains(err.Error(), "after top-level value") {
		t.Errorf("trailing data error = %v, want after top-level value", err)
	}
}

func TestParseDeepPlutusDataJSONTreeDepthLimit(t *testing.T) {
	// A PlutusData depth of 0 permits three JSON container levels.
	for depth := 2; depth <= 4; depth++ {
		input := []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
		_, wantErr := parsePlutusDataJSONTree(input, 0)
		_, gotErr := parseDeepPlutusDataJSONTree(input, 0)
		if (gotErr == nil) != (wantErr == nil) || (depth == 4) != (gotErr != nil) {
			t.Errorf("depth %d: fallback error %v, decoder error %v", depth, gotErr, wantErr)
		}
	}
}
