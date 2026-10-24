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
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// parsePlutusDataJSONTree parses data into a jsonTree in a single
// pass using encoding/json's token stream, rejecting malformed JSON and
// trailing non-whitespace data exactly like json.Unmarshal does. It is the
// reference implementation the hand-written parser is tested against.
func parsePlutusDataJSONTree(data []byte, maxDepth int) (*jsonTree, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tree := newJSONTree()
	root, err := parsePlutusDataJSONValue(
		dec,
		data,
		&jsonParseState{maxDepth: maxDepth},
		tree,
	)
	if err != nil {
		// encoding/json limits its own token nesting independently of the
		// PlutusData limit. Keep accepting structurally valid detailed-schema
		// values up to our own bound when that implementation limit is reached.
		if strings.Contains(err.Error(), "exceeded max depth") {
			return parseDeepPlutusDataJSONTree(data, maxDepth)
		}
		return nil, normalizeJSONStreamError(err)
	}
	if err := checkJSONTrailingData(data, dec.InputOffset()); err != nil {
		return nil, err
	}
	tree.root = root
	return tree, nil
}

// parsePlutusDataJSONValue parses one value from the token stream and
// returns its arena index. The member key id is filled in by the caller.
func parsePlutusDataJSONValue(
	dec *json.Decoder,
	data []byte,
	st *jsonParseState,
	tree *jsonTree,
) (uint32, error) {
	if err := st.enterValue(); err != nil {
		return 0, err
	}
	defer st.leaveValue()

	start := uint32(jsonValueStart(data, dec.InputOffset()))
	tok, err := dec.Token()
	if err != nil {
		return 0, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '[':
			idx := tree.add(jsonValue{kind: jsonKindArray, start: start})
			var last uint32
			for dec.More() {
				child, err := parsePlutusDataJSONValue(dec, data, st, tree)
				if err != nil {
					return 0, err
				}
				tree.appendChild(idx, last, child)
				last = child
			}
			// Consume the closing ']'.
			if _, err := dec.Token(); err != nil {
				return 0, err
			}
			tree.node(idx).end = uint32(dec.InputOffset())
			return idx, nil
		case '{':
			idx := tree.add(jsonValue{kind: jsonKindObject, start: start})
			var last uint32
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return 0, err
				}
				key, ok := keyTok.(string)
				if !ok {
					// Object keys are always strings in well-formed JSON.
					return 0, fmt.Errorf(
						"unexpected JSON object key token %v",
						keyTok,
					)
				}
				child, err := parsePlutusDataJSONValue(dec, data, st, tree)
				if err != nil {
					return 0, err
				}
				tree.node(child).key = tree.keyIDString(key)
				tree.appendChild(idx, last, child)
				last = child
			}
			// Consume the closing '}'.
			if _, err := dec.Token(); err != nil {
				return 0, err
			}
			tree.node(idx).end = uint32(dec.InputOffset())
			return idx, nil
		default:
			// Unreachable: closing delimiters are consumed above.
			return 0, fmt.Errorf("unexpected JSON delimiter %v", t)
		}
	case nil:
		return tree.add(jsonValue{kind: jsonKindNull, start: start, end: uint32(dec.InputOffset())}), nil
	case bool:
		return tree.add(jsonValue{kind: jsonKindBool, start: start, end: uint32(dec.InputOffset())}), nil
	case json.Number:
		return tree.add(jsonValue{kind: jsonKindNumber, start: start, end: uint32(dec.InputOffset())}), nil
	case string:
		return tree.add(jsonValue{kind: jsonKindString, start: start, end: uint32(dec.InputOffset())}), nil
	default:
		// Unreachable: the cases above cover every token type.
		return 0, fmt.Errorf("unexpected JSON token %v", tok)
	}
}

// jsonValueStart returns the offset of the first byte of the next JSON
// value at or after from, skipping insignificant whitespace and the ':' or
// ',' separators that json.Decoder consumes silently between tokens.
func jsonValueStart(data []byte, from int64) int {
	i := int(from)
	for i < len(data) {
		switch data[i] {
		case ' ', '\t', '\r', '\n', ',', ':':
			i++
		default:
			return i
		}
	}
	return i
}

// checkJSONTrailingData rejects non-whitespace bytes after the top-level
// value, matching json.Unmarshal's behavior and error message.
func checkJSONTrailingData(data []byte, from int64) error {
	for i := int(from); i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return fmt.Errorf(
				"invalid character %s after top-level value",
				jsonQuoteChar(data[i]),
			)
		}
	}
	return nil
}

// children returns the children of v in order.
func (t *jsonTree) children(v jsonValue) []jsonValue {
	out := make([]jsonValue, 0, v.count)
	for c := v.first; c != 0; c = t.node(c).next {
		out = append(out, *t.node(c))
	}
	return out
}
