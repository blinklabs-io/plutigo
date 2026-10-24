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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"sync/atomic"
)

// JSON (de)serialization for PlutusData types.
// Uses the standard Cardano "detailed schema" JSON format:
//   Integer:    {"int": <number>}
//   ByteString: {"bytes": "<hex>"}
//   List:       {"list": [...]}
//   Map:        {"map": [{"k": <key>, "v": <value>}, ...]}
//   Constr:     {"constructor": <tag>, "fields": [...]}

// MarshalJSON implements json.Marshaler for Integer.
func (i Integer) MarshalJSON() ([]byte, error) {
	if i.Inner == nil {
		return nil, ErrNilInteger
	}
	return json.Marshal(map[string]*big.Int{"int": i.Inner})
}

// UnmarshalJSON implements json.Unmarshaler for Integer.
func (i *Integer) UnmarshalJSON(data []byte) error {
	decoded, err := decodeJSONAs[Integer](data, "Integer", "int")
	if err != nil {
		return err
	}
	i.Inner = decoded.Inner
	return nil
}

// MarshalJSON implements json.Marshaler for ByteString.
func (b ByteString) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"bytes": hex.EncodeToString(b.Inner)})
}

// UnmarshalJSON implements json.Unmarshaler for ByteString.
func (b *ByteString) UnmarshalJSON(data []byte) error {
	decoded, err := decodeJSONAs[ByteString](data, "ByteString", "bytes")
	if err != nil {
		return err
	}
	b.Inner = decoded.Inner
	return nil
}

// MarshalJSON implements json.Marshaler for List.
func (l List) MarshalJSON() ([]byte, error) {
	return marshalPlutusDataJSON(l)
}

// UnmarshalJSON implements json.Unmarshaler for List.
func (l *List) UnmarshalJSON(data []byte) error {
	decoded, err := decodeJSONAs[List](data, "List", "list")
	if err != nil {
		return err
	}
	l.Items = decoded.Items
	return nil
}

// MarshalJSON implements json.Marshaler for Map.
func (m Map) MarshalJSON() ([]byte, error) {
	return marshalPlutusDataJSON(m)
}

// UnmarshalJSON implements json.Unmarshaler for Map.
func (m *Map) UnmarshalJSON(data []byte) error {
	decoded, err := decodeJSONAs[Map](data, "Map", "map")
	if err != nil {
		return err
	}
	m.Pairs = decoded.Pairs
	return nil
}

// MarshalJSON implements json.Marshaler for Constr.
func (c Constr) MarshalJSON() ([]byte, error) {
	return marshalPlutusDataJSON(c)
}

// UnmarshalJSON implements json.Unmarshaler for Constr.
func (c *Constr) UnmarshalJSON(data []byte) error {
	decoded, err := decodeJSONAs[Constr](data, "Constr", "constructor")
	if err != nil {
		return err
	}
	c.Tag = decoded.Tag
	c.Fields = decoded.Fields
	return nil
}

// decodeJSONAs decodes data with the bounded whole-document decoder, so the
// exported per-type decoders share its depth and node limits, and requires the
// document to be a T. key is T's discriminator key.
func decodeJSONAs[T any, P interface {
	*T
	PlutusData
}](data []byte, name, key string) (P, error) {
	pd, err := unmarshalPlutusDataJSON(data)
	if err != nil {
		return nil, err
	}
	decoded, ok := pd.(P)
	if !ok {
		return nil, fmt.Errorf("missing %q key in %s JSON", key, name)
	}
	return decoded, nil
}

// MarshalJSON implements json.Marshaler for PlutusDataWrapper.
func (p PlutusDataWrapper) MarshalJSON() ([]byte, error) {
	return marshalPlutusDataJSON(p.Data)
}

// UnmarshalJSON implements json.Unmarshaler for PlutusDataWrapper.
func (p *PlutusDataWrapper) UnmarshalJSON(data []byte) error {
	pd, err := unmarshalPlutusDataJSON(data)
	if err != nil {
		return err
	}
	p.Data = pd
	return nil
}

// EncodeJSON encodes a PlutusData value into JSON bytes using the Cardano detailed schema.
func EncodeJSON(pd PlutusData) ([]byte, error) {
	return marshalPlutusDataJSON(pd)
}

// DecodeJSON decodes JSON bytes into a PlutusData value by inspecting the discriminator key.
func DecodeJSON(data []byte) (PlutusData, error) {
	return unmarshalPlutusDataJSON(data)
}

// marshalPlutusDataJSON marshals any PlutusData value to JSON.
func marshalPlutusDataJSON(pd PlutusData) (json.RawMessage, error) {
	var buf bytes.Buffer
	if err := appendPlutusDataJSON(&buf, pd); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// jsonFrame writes the children of one container: array items separated by
// commas, or map pairs as {"k":key,"v":value} objects, then "]}".
type jsonFrame struct {
	children childFrame
	isMap    bool
	label    string
}

// appendPlutusDataJSON writes pd to buf. Containers write their children
// into the same buffer, so the work is linear in the size of the output;
// returning each child as a json.RawMessage would make json.Marshal rescan
// the whole encoded subtree at every nesting level. The walk uses an explicit
// stack, so any nesting depth is safe.
func appendPlutusDataJSON(buf *bytes.Buffer, pd PlutusData) error {
	var stackBuf [16]jsonFrame
	stack := stackBuf[:0]
	for {
		frame, err := appendJSONShallow(buf, pd)
		if err != nil {
			if n := len(stack); n > 0 {
				top := &stack[n-1]
				index := top.children.next - 1
				if top.isMap {
					part := "key"
					if index%2 == 1 {
						part = "value"
					}
					return fmt.Errorf("failed to marshal Map %s %d: %w", part, index/2, err)
				}
				return fmt.Errorf("failed to marshal %s %d: %w", top.label, index, err)
			}
			return err
		}
		if frame != nil {
			stack = append(stack, *frame)
		}
		for {
			if len(stack) == 0 {
				return nil
			}
			top := &stack[len(stack)-1]
			index := top.children.next
			var more bool
			if pd, more = top.children.nextChild(); more {
				switch {
				case !top.isMap:
					if index > 0 {
						buf.WriteByte(',')
					}
				case index%2 == 0:
					if index > 0 {
						buf.WriteString(`},`)
					}
					buf.WriteString(`{"k":`)
				default:
					buf.WriteString(`,"v":`)
				}
				break
			}
			if top.isMap && index > 0 {
				buf.WriteByte('}')
			}
			buf.WriteString(`]}`)
			stack = stack[:len(stack)-1]
		}
	}
}

// appendJSONShallow writes pd, or for a container its opening text, and
// returns the frame that writes the rest of the container.
func appendJSONShallow(buf *bytes.Buffer, pd PlutusData) (*jsonFrame, error) {
	switch v := pd.(type) {
	case *List:
		if v != nil {
			return openListJSON(buf, v), nil
		}
	case List:
		return openListJSON(buf, &v), nil
	case *Map:
		if v != nil {
			return openMapJSON(buf, v), nil
		}
	case Map:
		return openMapJSON(buf, &v), nil
	case *Constr:
		if v != nil {
			return openConstrJSON(buf, v), nil
		}
	case Constr:
		return openConstrJSON(buf, &v), nil
	case *Integer:
		if v != nil && v.Inner != nil {
			appendIntegerJSON(buf, v.Inner)
			return nil, nil
		}
	case Integer:
		if v.Inner != nil {
			appendIntegerJSON(buf, v.Inner)
			return nil, nil
		}
	case *ByteString:
		if v != nil {
			appendByteStringJSON(buf, v.Inner)
			return nil, nil
		}
	case ByteString:
		appendByteStringJSON(buf, v.Inner)
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown PlutusData type: %T", pd)
	}
	raw, err := json.Marshal(pd)
	if err != nil {
		return nil, err
	}
	buf.Write(raw)
	return nil, nil
}

// appendIntegerJSON writes {"int":n}, as Integer.MarshalJSON does.
func appendIntegerJSON(buf *bytes.Buffer, n *big.Int) {
	buf.WriteString(`{"int":`)
	buf.Write(n.Append(buf.AvailableBuffer(), 10))
	buf.WriteByte('}')
}

// appendByteStringJSON writes {"bytes":"<hex>"}, as ByteString.MarshalJSON
// does.
func appendByteStringJSON(buf *bytes.Buffer, b []byte) {
	buf.WriteString(`{"bytes":"`)
	buf.Write(hex.AppendEncode(buf.AvailableBuffer(), b))
	buf.WriteString(`"}`)
}

func openListJSON(buf *bytes.Buffer, l *List) *jsonFrame {
	buf.WriteString(`{"list":[`)
	return &jsonFrame{children: childFrame{items: l.Items}, label: "List item"}
}

func openMapJSON(buf *bytes.Buffer, m *Map) *jsonFrame {
	buf.WriteString(`{"map":[`)
	return &jsonFrame{children: childFrame{pairs: m.Pairs}, isMap: true}
}

func openConstrJSON(buf *bytes.Buffer, c *Constr) *jsonFrame {
	buf.WriteString(`{"constructor":`)
	buf.WriteString(constrTagString(c.Tag))
	buf.WriteString(`,"fields":[`)
	return &jsonFrame{children: childFrame{items: c.Fields}, label: "Constr field"}
}

const (
	// DefaultMaxJSONInputSize is the default maximum size in bytes of a JSON
	// document accepted by DecodeJSON and the JSON unmarshalers. The node
	// count limit alone does not bound memory: a document within the node
	// limit can still be arbitrarily large (long hex strings, whitespace). A
	// MaxDecodeNodes-node document in canonical form is at most ~40 MB (for
	// example one million `{"constructor":0,"fields":[]}` values), so 64 MiB
	// accepts every unpadded document within the node limit, while keeping
	// the parse tree (about 64 bytes per node) bounded.
	DefaultMaxJSONInputSize = 64 << 20

	// maxJSONInputSizeLimit is the largest value SetMaxJSONInputSize accepts;
	// the parse tree stores 32-bit offsets into the input.
	maxJSONInputSizeLimit = math.MaxInt32

	// MaxJSONIntegerDigits is the maximum number of decimal digits (excluding
	// a leading minus sign) accepted for a single "int" or "constructor"
	// value. big.Int decimal parsing is quadratic: 400k digits takes about a
	// second. 100,000 digits is about 41 KB of magnitude, more than twice the
	// 16 KiB Cardano maximum transaction size (about 39,500 digits) that any
	// on-chain PlutusData integer must fit in, and parses in well under
	// 100 ms.
	MaxJSONIntegerDigits = 100_000

	// MaxJSONTotalIntegerDigits bounds the sum of digits over all integers
	// in one document, so many large integers cannot multiply the
	// per-integer cost.
	MaxJSONTotalIntegerDigits = 1_000_000
)

var maxJSONInputSize atomic.Int64

func init() {
	maxJSONInputSize.Store(DefaultMaxJSONInputSize)
}

// MaxJSONInputSize returns the maximum JSON document size in bytes accepted
// by the JSON decoders.
func MaxJSONInputSize() int {
	return int(maxJSONInputSize.Load())
}

// SetMaxJSONInputSize changes the maximum JSON document size in bytes
// accepted by the JSON decoders. size must be between 1 and math.MaxInt32.
func SetMaxJSONInputSize(size int) error {
	if size < 1 || size > maxJSONInputSizeLimit {
		return fmt.Errorf(
			"JSON input size limit must be between 1 and %d: %d",
			maxJSONInputSizeLimit,
			size,
		)
	}
	maxJSONInputSize.Store(int64(size))
	return nil
}

// JSONLimitError reports that a PlutusData JSON document exceeded a size
// limit: its input size or the digit count of an integer.
type JSONLimitError struct {
	Limit  string
	Max    int
	Actual int
}

func (e *JSONLimitError) Error() string {
	return fmt.Sprintf(
		"PlutusData JSON %s limit exceeded: %d > %d",
		e.Limit,
		e.Actual,
		e.Max,
	)
}

// jsonDecodeState bounds recursion depth and total node count while decoding
// untrusted PlutusData JSON.
type jsonDecodeState struct {
	depth    int
	nodes    int
	maxDepth int
	digits   int
	slab     jsonSlab
}

type jsonDepthLimitError struct {
	maxDepth int
}

func (e *jsonDepthLimitError) Error() string {
	return fmt.Sprintf(
		"PlutusData JSON nesting exceeds max depth %d",
		e.maxDepth,
	)
}

func isJSONDepthLimitError(err error) bool {
	_, ok := err.(*jsonDepthLimitError)
	return ok
}

func (st *jsonDecodeState) enterNode() error {
	st.nodes++
	if st.nodes > MaxDecodeNodes {
		return fmt.Errorf(
			"PlutusData JSON exceeds max node count %d",
			MaxDecodeNodes,
		)
	}
	return nil
}

// jsonParseState bounds the jsonValue tree while it is being built. A
// detailed-schema PlutusData value occupies at most three JSON nesting levels:
// a data object, an enclosing array, and (for Maps) a key/value-pair object.
// The additional value preserves the existing PlutusData-depth error for
// one-level-over inputs; arbitrary JSON nesting still cannot grow the parser
// stack beyond this bound.
type jsonParseState struct {
	depth    int
	nodes    int
	maxDepth int
}

// A canonical detailed-schema Map pair with constructor values occupies seven
// jsonValue nodes while representing two PlutusData nodes. Keep the parser cap
// above that worst-case structural multiplier.
const maxJSONParseNodes = MaxDecodeNodes * 4

// maxJSONParseNestingDepth derives the JSON parser's structural cap from the
// configured PlutusData depth limit.
func maxJSONParseNestingDepth(maxDepth int) int { return (maxDepth + 1) * 3 }

func (st *jsonParseState) enterValue() error {
	if st.depth >= maxJSONParseNestingDepth(st.maxDepth) {
		return fmt.Errorf(
			"PlutusData JSON tree nesting exceeds max depth %d",
			maxJSONParseNestingDepth(st.maxDepth),
		)
	}
	st.depth++

	st.nodes++
	if st.nodes > maxJSONParseNodes {
		st.depth--
		return fmt.Errorf(
			"PlutusData JSON tree exceeds max node count %d",
			maxJSONParseNodes,
		)
	}
	return nil
}

func (st *jsonParseState) leaveValue() {
	st.depth--
}

// jsonValueKind identifies the syntactic kind of a parsed JSON value.
type jsonValueKind uint8

const (
	jsonKindNull jsonValueKind = iota
	jsonKindBool
	jsonKindNumber
	jsonKindString
	jsonKindArray
	jsonKindObject
)

const (
	jsonChunkShift = 12
	jsonChunkSize  = 1 << jsonChunkShift
)

// jsonTree is a JSON syntax tree stored in a single flat arena. Every value,
// including each object member, is one fixed-size pointer-free jsonValue
// that links to its first child and next sibling by arena index, so a
// document costs about 28 bytes per value with no per-value slice headers or
// key strings, and the garbage collector never scans it.
type jsonTree struct {
	chunks [][]jsonValue // node arena; node 0 is an unused sentinel meaning "none"
	size   int           // number of nodes in use, including the sentinel
	keys   []string      // key table; ids 0..jsonKeyV are well-known
	keyIDs map[string]uint32
	root   uint32
}

// Well-known object key ids. jsonKeyNone is the key of array items and the
// root value; real keys always have a non-zero id.
const (
	jsonKeyNone uint32 = iota
	jsonKeyInt
	jsonKeyBytes
	jsonKeyList
	jsonKeyMap
	jsonKeyConstructor
	jsonKeyFields
	jsonKeyK
	jsonKeyV
	jsonKeyFirstDynamic
)

var jsonWellKnownKeys = [...]string{
	jsonKeyNone:        "",
	jsonKeyInt:         "int",
	jsonKeyBytes:       "bytes",
	jsonKeyList:        "list",
	jsonKeyMap:         "map",
	jsonKeyConstructor: "constructor",
	jsonKeyFields:      "fields",
	jsonKeyK:           "k",
	jsonKeyV:           "v",
}

// jsonValue is a node of a jsonTree. start and end are byte offsets of the
// raw value within the original input, so scalar leaves can be handed to
// encoding/json verbatim without rescanning any enclosing structure. Object
// members are the children of an object node, in order of appearance
// including duplicate keys, so the decoder can reproduce encoding/json's
// last-occurrence-wins map semantics as well as the Map pair node accounting
// for repeated "k"/"v" keys. A member's key is the key table id in key.
type jsonValue struct {
	start uint32
	end   uint32
	key   uint32 // key table id of this value as an object member
	first uint32 // first child index (arrays and objects)
	next  uint32 // next sibling index
	count uint32 // number of children
	kind  jsonValueKind
}

func newJSONTree() *jsonTree {
	t := &jsonTree{
		keys: make([]string, len(jsonWellKnownKeys), 16),
	}
	t.add(jsonValue{}) // sentinel
	copy(t.keys, jsonWellKnownKeys[:])
	return t
}

// add appends a node and returns its index.
func (t *jsonTree) add(v jsonValue) uint32 {
	if t.size>>jsonChunkShift == len(t.chunks) {
		// Chunks are fixed-size and never reallocated, so growing the tree
		// neither copies nodes nor leaves garbage behind, and pointers
		// returned by node stay valid.
		t.chunks = append(t.chunks, make([]jsonValue, jsonChunkSize))
	}
	idx := t.size
	t.size++
	t.chunks[idx>>jsonChunkShift][idx&(jsonChunkSize-1)] = v
	return uint32(idx)
}

// node returns a pointer to node i.
func (t *jsonTree) node(i uint32) *jsonValue {
	return &t.chunks[i>>jsonChunkShift][i&(jsonChunkSize-1)]
}

// appendChild links child to the end of parent's child list; last is the
// index of parent's current last child, or 0 when it has none.
func (t *jsonTree) appendChild(parent, last, child uint32) {
	if last == 0 {
		t.node(parent).first = child
	} else {
		t.node(last).next = child
	}
	t.node(parent).count++
}

// keyIDBytes returns the key table id for key without allocating when the
// key is well known or has been seen before.
func (t *jsonTree) keyIDBytes(key []byte) uint32 {
	switch string(key) {
	case "int":
		return jsonKeyInt
	case "bytes":
		return jsonKeyBytes
	case "list":
		return jsonKeyList
	case "map":
		return jsonKeyMap
	case "constructor":
		return jsonKeyConstructor
	case "fields":
		return jsonKeyFields
	case "k":
		return jsonKeyK
	case "v":
		return jsonKeyV
	}
	if id, ok := t.keyIDs[string(key)]; ok {
		return id
	}
	return t.addKey(string(key))
}

// keyIDString is keyIDBytes for a string.
func (t *jsonTree) keyIDString(key string) uint32 {
	return t.keyIDBytes([]byte(key))
}

func (t *jsonTree) addKey(key string) uint32 {
	if t.keyIDs == nil {
		t.keyIDs = make(map[string]uint32)
	}
	id := uint32(len(t.keys))
	t.keys = append(t.keys, key)
	t.keyIDs[key] = id
	return id
}

// keyName returns the key string of the member v.
func (t *jsonTree) keyName(v jsonValue) string { return t.keys[v.key] }

// unmarshalPlutusDataJSON unmarshals JSON into the appropriate PlutusData
// type by inspecting which key is present in the JSON object. The input is
// scanned exactly once into a lightweight syntax tree and PlutusData values
// are built from that tree, so decoding does O(size) work instead of
// re-parsing the remaining subtree at every nesting level.
func unmarshalPlutusDataJSON(data json.RawMessage) (PlutusData, error) {
	if limit := MaxJSONInputSize(); len(data) > limit {
		return nil, &JSONLimitError{
			Limit:  "input size",
			Max:    limit,
			Actual: len(data),
		}
	}
	maxDepth := MaxDecodeNestingDepth()
	// The hand-written parser is used for every input: it allocates no
	// per-token strings or numbers, unlike encoding/json's token stream, and
	// its errors are kept identical to the stream parser's (see
	// json_deep_parser_test.go).
	tree, err := parseDeepPlutusDataJSONTree(data, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("PlutusData JSON must be an object: %w", err)
	}
	return decodePlutusDataJSONValue(
		data,
		tree,
		*tree.node(tree.root),
		&jsonDecodeState{maxDepth: maxDepth},
		false,
	)
}

type deepJSONParser struct {
	data  []byte
	pos   int
	state jsonParseState
	tree  *jsonTree
}

func parseDeepPlutusDataJSONTree(data []byte, maxDepth int) (*jsonTree, error) {
	p := deepJSONParser{
		data:  data,
		state: jsonParseState{maxDepth: maxDepth},
		tree:  newJSONTree(),
	}
	root, err := p.parseValue(jsonKeyNone)
	if err != nil {
		return nil, normalizeJSONStreamError(err)
	}
	p.skipSpace()
	if p.pos != len(data) {
		return nil, fmt.Errorf(
			"invalid character %s after top-level value",
			jsonQuoteChar(data[p.pos]),
		)
	}
	p.tree.root = root
	return p.tree, nil
}

// parseValue parses one value whose object-member key id is key and returns
// its arena index.
func (p *deepJSONParser) parseValue(key uint32) (uint32, error) {
	if err := p.state.enterValue(); err != nil {
		return 0, err
	}
	defer p.state.leaveValue()
	p.skipSpace()
	start := uint32(p.pos)
	if int(start) == len(p.data) {
		return 0, io.ErrUnexpectedEOF
	}
	switch p.data[p.pos] {
	case '{':
		p.pos++
		p.skipSpace()
		idx := p.tree.add(jsonValue{kind: jsonKindObject, start: start, key: key})
		if p.consume('}') {
			p.tree.node(idx).end = uint32(p.pos)
			return idx, nil
		}
		var last uint32
		for {
			memberKey, err := p.parseKey()
			if err != nil {
				return 0, err
			}
			p.skipSpace()
			if !p.consume(':') {
				if p.pos == len(p.data) {
					return 0, io.ErrUnexpectedEOF
				}
				return 0, errors.New("invalid character after object key")
			}
			child, err := p.parseValue(memberKey)
			if err != nil {
				return 0, err
			}
			p.tree.appendChild(idx, last, child)
			last = child
			p.skipSpace()
			if p.consume('}') {
				break
			}
			if !p.consume(',') {
				if p.pos == len(p.data) {
					return 0, io.ErrUnexpectedEOF
				}
				return 0, errors.New("invalid character after object value")
			}
			p.skipSpace()
		}
		p.tree.node(idx).end = uint32(p.pos)
		return idx, nil
	case '[':
		p.pos++
		p.skipSpace()
		idx := p.tree.add(jsonValue{kind: jsonKindArray, start: start, key: key})
		if p.consume(']') {
			p.tree.node(idx).end = uint32(p.pos)
			return idx, nil
		}
		var last uint32
		for {
			child, err := p.parseValue(jsonKeyNone)
			if err != nil {
				return 0, err
			}
			p.tree.appendChild(idx, last, child)
			last = child
			p.skipSpace()
			if p.consume(']') {
				break
			}
			if !p.consume(',') {
				if p.pos == len(p.data) {
					return 0, io.ErrUnexpectedEOF
				}
				return 0, errors.New("invalid character after array value")
			}
		}
		p.tree.node(idx).end = uint32(p.pos)
		return idx, nil
	case '"':
		if _, _, err := p.scanString(); err != nil {
			return 0, err
		}
		return p.tree.add(jsonValue{kind: jsonKindString, start: start, end: uint32(p.pos), key: key}), nil
	case 't':
		return p.parseLiteral("true", jsonKindBool, start, key)
	case 'f':
		return p.parseLiteral("false", jsonKindBool, start, key)
	case 'n':
		return p.parseLiteral("null", jsonKindNull, start, key)
	default:
		if p.data[p.pos] == '-' || p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
			if err := p.parseNumber(); err != nil {
				return 0, err
			}
			return p.tree.add(jsonValue{kind: jsonKindNumber, start: start, end: uint32(p.pos), key: key}), nil
		}
		return 0, fmt.Errorf("invalid character %s looking for beginning of value", jsonQuoteChar(p.data[p.pos]))
	}
}

// The deepJSONParser methods report io.ErrUnexpectedEOF only when the input
// ends where a valid continuation could still follow, so the fallback
// classifies truncated and malformed input the same way json.Decoder does.

// parseKey scans an object key string and returns its key table id.
func (p *deepJSONParser) parseKey() (uint32, error) {
	start := p.pos
	if p.pos < len(p.data) && p.data[p.pos] == '"' {
		rawStart, plain, err := p.scanString()
		if err != nil {
			return 0, err
		}
		if plain {
			return p.tree.keyIDBytes(p.data[rawStart : p.pos-1]), nil
		}
		var value string
		if err := json.Unmarshal(p.data[start:p.pos], &value); err != nil {
			return 0, err
		}
		return p.tree.keyIDString(value), nil
	}
	if p.pos == len(p.data) {
		return 0, io.ErrUnexpectedEOF
	}
	return 0, fmt.Errorf(
		"invalid character %s looking for beginning of object key string",
		jsonQuoteChar(p.data[p.pos]),
	)
}

// scanString validates the string literal at the current position, which
// must be a '"', and advances past it. rawStart is the offset of the first
// byte after the opening quote and plain reports whether the string is
// plain ASCII without escapes, in which case its value is exactly
// data[rawStart:p.pos-1].
func (p *deepJSONParser) scanString() (rawStart int, plain bool, err error) {
	p.pos++
	rawStart = p.pos
	// Fast path: a plain ASCII string without escapes needs no unquoting.
	for i := p.pos; i < len(p.data); i++ {
		c := p.data[i]
		if c == '"' {
			p.pos = i + 1
			return rawStart, true, nil
		}
		if c == '\\' || c < 0x20 || c >= 0x80 {
			break
		}
	}
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		p.pos++
		switch {
		case c == '"':
			return rawStart, false, nil
		case c == '\\':
			if err := p.skipEscape(); err != nil {
				return 0, false, err
			}
		case c < 0x20:
			return 0, false, errors.New("invalid character in string")
		}
	}
	return 0, false, io.ErrUnexpectedEOF
}

// skipEscape advances past the escape sequence following a backslash.
func (p *deepJSONParser) skipEscape() error {
	if p.pos == len(p.data) {
		return io.ErrUnexpectedEOF
	}
	c := p.data[p.pos]
	p.pos++
	switch c {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return nil
	case 'u':
		for range 4 {
			if p.pos == len(p.data) {
				return io.ErrUnexpectedEOF
			}
			if !isJSONHexDigit(p.data[p.pos]) {
				return fmt.Errorf(
					"invalid character %s in \\u hexadecimal character escape",
					jsonQuoteChar(p.data[p.pos]),
				)
			}
			p.pos++
		}
		return nil
	default:
		return fmt.Errorf("invalid character %s in string escape code", jsonQuoteChar(c))
	}
}

func isJSONHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func (p *deepJSONParser) parseNumber() error {
	p.consume('-')
	switch {
	case p.consume('0'):
		if p.pos < len(p.data) && isJSONDigit(p.data[p.pos]) {
			return errors.New("invalid leading zero in number")
		}
	default:
		if err := p.requireDigits("in numeric literal"); err != nil {
			return err
		}
	}
	if p.consume('.') {
		if err := p.requireDigits("after decimal point in numeric literal"); err != nil {
			return err
		}
	}
	if p.consume('e') || p.consume('E') {
		if !p.consume('+') {
			p.consume('-')
		}
		if err := p.requireDigits("in exponent of numeric literal"); err != nil {
			return err
		}
	}
	return nil
}

// requireDigits consumes one or more digits, reporting end of input when
// the data ends before the first one.
func (p *deepJSONParser) requireDigits(context string) error {
	if p.pos == len(p.data) {
		return io.ErrUnexpectedEOF
	}
	if !isJSONDigit(p.data[p.pos]) {
		return fmt.Errorf("invalid character %s %s", jsonQuoteChar(p.data[p.pos]), context)
	}
	for p.pos < len(p.data) && isJSONDigit(p.data[p.pos]) {
		p.pos++
	}
	return nil
}

func isJSONDigit(c byte) bool { return c >= '0' && c <= '9' }

func (p *deepJSONParser) parseLiteral(literal string, kind jsonValueKind, start, key uint32) (uint32, error) {
	for i := range len(literal) {
		if p.pos == len(p.data) {
			return 0, io.ErrUnexpectedEOF
		}
		if p.data[p.pos] != literal[i] {
			return 0, fmt.Errorf(
				"invalid character %s in literal %s (expecting %s)",
				jsonQuoteChar(p.data[p.pos]),
				literal,
				jsonQuoteChar(literal[i]),
			)
		}
		p.pos++
	}
	return p.tree.add(jsonValue{kind: kind, start: start, end: uint32(p.pos), key: key}), nil
}

func (p *deepJSONParser) consume(expected byte) bool {
	if p.pos < len(p.data) && p.data[p.pos] == expected {
		p.pos++
		return true
	}
	return false
}

func (p *deepJSONParser) skipSpace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\r', '\n':
			p.pos++
		default:
			return
		}
	}
}

// normalizeJSONStreamError maps json.Decoder end-of-input errors to the
// message json.Unmarshal reports for truncated documents.
func normalizeJSONStreamError(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return errors.New("unexpected end of JSON input")
	}
	return err
}

// jsonQuoteChar formats c as a quoted character literal the same way
// encoding/json does in its syntax errors.
func jsonQuoteChar(c byte) string {
	if c == '\'' {
		return `'\''`
	}
	if c == '"' {
		return `'"'`
	}
	s := strconv.Quote(string(rune(c)))
	return "'" + s[1:len(s)-1] + "'"
}

// jsonRawSpan returns the raw bytes of v within data.
func jsonRawSpan(data []byte, v jsonValue) json.RawMessage {
	return json.RawMessage(data[v.start:v.end])
}

// jsonSlabSize is the number of values allocated at once by jsonSlab.
const jsonSlabSize = 256

// jsonSlab hands out Integer, big.Int, big.Word and ByteString values from
// chunks, replacing several tiny allocations per decoded integer with one
// per jsonSlabSize values. Chunks are retained as long as any value from
// them is, which is bounded by the size of the decoded document.
type jsonSlab struct {
	integers []Integer
	bigs     []big.Int
	words    []big.Word
	bytes    []ByteString
}

// slabNext returns the next free element of *chunk, starting a new chunk
// when it is used up.
func slabNext[T any](chunk *[]T) *T {
	if len(*chunk) == 0 {
		*chunk = make([]T, jsonSlabSize)
	}
	v := &(*chunk)[0]
	*chunk = (*chunk)[1:]
	return v
}

func (s *jsonSlab) newInteger(n *big.Int) *Integer {
	i := slabNext(&s.integers)
	i.Inner = n
	return i
}

func (s *jsonSlab) newByteString(b []byte) *ByteString {
	bs := slabNext(&s.bytes)
	bs.Inner = b
	return bs
}

// newBigFromWord returns a big.Int holding w, negated if neg, backed by slab
// memory. The magnitude slice is capacity-limited, so any later growth of
// the integer reallocates instead of writing into a neighbour.
func (s *jsonSlab) newBigFromWord(w big.Word, neg bool) *big.Int {
	n := slabNext(&s.bigs)
	if w != 0 {
		if len(s.words) == 0 {
			s.words = make([]big.Word, jsonSlabSize)
		}
		mag := s.words[0:1:1]
		s.words = s.words[1:]
		mag[0] = w
		n.SetBits(mag)
		if neg {
			n.Neg(n)
		}
	}
	return n
}

// jsonObjectMembers summarizes an object node: the last member for each
// discriminator key (jsonKeyInt through jsonKeyConstructor).
type jsonObjectMembers struct {
	last  [jsonKeyConstructor - jsonKeyInt + 1]uint32 // node index of the last occurrence, 0 if absent
	count int                                         // number of discriminator keys present
	key   uint32                                      // key id of the last one found
}

func (m *jsonObjectMembers) scan(t *jsonTree, v jsonValue) {
	for c := v.first; c != 0; c = t.node(c).next {
		if key := t.node(c).key; key >= jsonKeyInt && key <= jsonKeyConstructor {
			m.last[key-jsonKeyInt] = c
		}
	}
	for i, c := range m.last {
		if c != 0 {
			m.count++
			m.key = jsonKeyInt + uint32(i)
		}
	}
}

// names returns the discriminator keys present, in key id order.
func (m *jsonObjectMembers) names() []string {
	var found []string
	for i, c := range m.last {
		if c != 0 {
			found = append(found, jsonWellKnownKeys[int(jsonKeyInt)+i])
		}
	}
	return found
}

// lastMember returns the value of the last member of v with key id key.
func (t *jsonTree) lastMember(v jsonValue, key uint32) (jsonValue, bool) {
	var found uint32
	for c := v.first; c != 0; c = t.node(c).next {
		if t.node(c).key == key {
			found = c
		}
	}
	if found == 0 {
		return jsonValue{}, false
	}
	return *t.node(found), true
}

// wrapJSONError adds context to an error from a nested value. A depth-limit
// error is returned as is.
func wrapJSONError(err error, context string) error {
	if isJSONDepthLimitError(err) {
		return err
	}
	return fmt.Errorf("%s: %w", context, err)
}

// jsonKindName names a non-object JSON kind the way encoding/json's
// UnmarshalTypeError does.
func jsonKindName(kind jsonValueKind) string {
	switch kind {
	case jsonKindBool:
		return "bool"
	case jsonKindNumber:
		return "number"
	case jsonKindString:
		return "string"
	case jsonKindArray:
		return "array"
	default:
		return "value"
	}
}

func decodePlutusDataJSONValue(
	data []byte,
	t *jsonTree,
	v jsonValue,
	st *jsonDecodeState,
	counted bool,
) (PlutusData, error) {
	st.depth++
	if st.depth > st.maxDepth {
		return nil, &jsonDepthLimitError{maxDepth: st.maxDepth}
	}
	defer func() { st.depth-- }()

	if !counted {
		if err := st.enterNode(); err != nil {
			return nil, err
		}
	}

	// json.Unmarshal leaves the destination map nil for a JSON null, which
	// then reads as an object with no keys.
	if v.kind != jsonKindObject && v.kind != jsonKindNull {
		// Keep the public error stable across encoding/json implementations.
		return nil, fmt.Errorf(
			"PlutusData JSON must be an object: json: cannot unmarshal %s into Go value of type map[string]json.RawMessage",
			jsonKindName(v.kind),
		)
	}

	// Members are looked up by scanning the children (last occurrence wins,
	// matching encoding/json's map unmarshaling) instead of building a map
	// per object, which dominated allocation for large documents.
	var members jsonObjectMembers
	if v.kind == jsonKindObject {
		members.scan(t, v)
	}
	switch members.count {
	case 0:
		return nil, fmt.Errorf(
			"unrecognized PlutusData JSON keys: %v",
			t.keysOf(v),
		)
	case 1:
	default:
		return nil, fmt.Errorf(
			"ambiguous PlutusData JSON: multiple discriminator keys %v",
			members.names(),
		)
	}
	member := *t.node(members.last[members.key-jsonKeyInt])
	switch members.key {
	case jsonKeyInt:
		n, err := parseJSONBigInt(data, member, st, "failed to unmarshal Integer value")
		if err != nil {
			return nil, err
		}
		if n == nil {
			return nil, errors.New("null \"int\" value in Integer JSON")
		}
		return st.slab.newInteger(n), nil
	case jsonKeyBytes:
		decoded, err := decodeJSONHexString(data, member)
		if err != nil {
			return nil, err
		}
		return st.slab.newByteString(decoded), nil
	case jsonKeyList:
		if member.kind == jsonKindNull {
			return nil, errors.New("null \"list\" value in List JSON")
		}
		items, err := decodePlutusDataJSONArray(data, t, member, st, "List item")
		if err != nil {
			return nil, wrapJSONError(err, "failed to unmarshal List value")
		}
		return &List{Items: items}, nil
	case jsonKeyMap:
		if member.kind == jsonKindNull {
			return nil, errors.New("null \"map\" value in Map JSON")
		}
		pairs, err := decodePlutusDataJSONMap(data, t, member, st)
		if err != nil {
			return nil, wrapJSONError(err, "failed to unmarshal Map value")
		}
		return &Map{Pairs: pairs}, nil
	default: // jsonKeyConstructor
		return decodePlutusDataJSONConstr(data, t, v, member, st)
	}
}

// decodePlutusDataJSONConstr decodes the constructor object v whose
// "constructor" member is tagValue.
func decodePlutusDataJSONConstr(
	data []byte,
	t *jsonTree,
	v, tagValue jsonValue,
	st *jsonDecodeState,
) (PlutusData, error) {
	tag, err := parseJSONBigInt(data, tagValue, st, "failed to unmarshal Constr constructor tag")
	if err != nil {
		return nil, err
	}
	if tag == nil {
		// json.Unmarshal of null into a big.Int leaves it zero.
		tag = new(big.Int)
	}
	fieldsVal, ok := t.lastMember(v, jsonKeyFields)
	if !ok {
		return nil, errors.New("missing \"fields\" key in Constr JSON")
	}
	if fieldsVal.kind == jsonKindNull {
		return nil, errors.New("null \"fields\" value in Constr JSON")
	}
	fields, err := decodePlutusDataJSONArray(data, t, fieldsVal, st, "Constr field")
	if err != nil {
		return nil, wrapJSONError(err, "failed to unmarshal Constr fields")
	}
	return &Constr{Tag: tag, Fields: fields}, nil
}

// decodeJSONHexString decodes the JSON value v, which must be a string of
// hex digits, into bytes. The common case of a plain ASCII string without
// escapes is decoded straight from the input without building a string.
func decodeJSONHexString(data []byte, v jsonValue) ([]byte, error) {
	raw := jsonRawSpan(data, v)
	if v.kind == jsonKindString && len(raw) >= 2 {
		body := raw[1 : len(raw)-1]
		plain := true
		for _, c := range body {
			// The parser already rejected control characters and
			// malformed escapes.
			if c == '\\' || c >= 0x80 {
				plain = false
				break
			}
		}
		if plain {
			dst := make([]byte, hex.DecodedLen(len(body)))
			n, err := hex.Decode(dst, body)
			if err != nil {
				return nil, fmt.Errorf("invalid hex in ByteString JSON: %w", err)
			}
			return dst[:n], nil
		}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf(
			"failed to unmarshal ByteString value: %w",
			err,
		)
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex in ByteString JSON: %w", err)
	}
	return decoded, nil
}

func decodePlutusDataJSONArray(
	data []byte,
	t *jsonTree,
	v jsonValue,
	st *jsonDecodeState,
	itemLabel string,
) ([]PlutusData, error) {
	if v.kind != jsonKindArray {
		return nil, errors.New("value must be an array")
	}

	items := make([]PlutusData, 0, v.count)
	i := 0
	for c := v.first; c != 0; c = t.node(c).next {
		if err := st.enterNode(); err != nil {
			return nil, err
		}
		pd, err := decodePlutusDataJSONValue(data, t, *t.node(c), st, true)
		if err != nil {
			return nil, wrapJSONError(err, fmt.Sprintf("failed to unmarshal %s %d", itemLabel, i))
		}
		items = append(items, pd)
		i++
	}
	return items, nil
}

func decodePlutusDataJSONMap(
	data []byte,
	t *jsonTree,
	v jsonValue,
	st *jsonDecodeState,
) ([][2]PlutusData, error) {
	if v.kind != jsonKindArray {
		return nil, errors.New("value must be an array")
	}

	pairs := make([][2]PlutusData, 0, v.count)
	i := 0
	for c := v.first; c != 0; c = t.node(c).next {
		pair, err := decodePlutusDataJSONMapPair(data, t, *t.node(c), st, i)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
		i++
	}
	return pairs, nil
}

func decodePlutusDataJSONMapPair(
	data []byte,
	t *jsonTree,
	v jsonValue,
	st *jsonDecodeState,
	index int,
) ([2]PlutusData, error) {
	var pair [2]PlutusData
	if v.kind != jsonKindObject {
		return pair, fmt.Errorf("Map pair %d must be an object", index)
	}

	var kVal, vVal jsonValue
	var kSeen, vSeen bool
	for c := v.first; c != 0; c = t.node(c).next {
		e := *t.node(c)
		switch e.key {
		case jsonKeyK:
			// Each occurrence counts as a node, and the last one wins,
			// mirroring the previous streaming decoder.
			if err := st.enterNode(); err != nil {
				return pair, err
			}
			kVal, kSeen = e, true
		case jsonKeyV:
			if err := st.enterNode(); err != nil {
				return pair, err
			}
			vVal, vSeen = e, true
		default:
			return pair, fmt.Errorf(
				"unexpected Map pair %d field %q",
				index,
				t.keyName(e),
			)
		}
	}
	if !kSeen {
		return pair, fmt.Errorf("missing \"k\" key in Map pair %d", index)
	}
	if !vSeen {
		return pair, fmt.Errorf("missing \"v\" key in Map pair %d", index)
	}

	k, err := decodePlutusDataJSONValue(data, t, kVal, st, true)
	if err != nil {
		return pair, wrapJSONError(err, fmt.Sprintf("failed to unmarshal Map key %d", index))
	}
	val, err := decodePlutusDataJSONValue(data, t, vVal, st, true)
	if err != nil {
		return pair, wrapJSONError(err, fmt.Sprintf("failed to unmarshal Map value %d", index))
	}
	return [2]PlutusData{k, val}, nil
}

// keysOf returns the sorted distinct member names of an object.
func (t *jsonTree) keysOf(v jsonValue) []string {
	seen := make(map[uint32]struct{}, v.count)
	keys := make([]string, 0, v.count)
	for c := v.first; c != 0; c = t.node(c).next {
		id := t.node(c).key
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		keys = append(keys, t.keys[id])
	}
	sort.Strings(keys)
	return keys
}

// parseJSONBigInt parses the JSON number v as a big.Int, enforcing the digit
// limits before the quadratic conversion runs. It returns a nil integer for
// JSON null, as json.Unmarshal does for a *big.Int. A conversion error is
// wrapped with context; a *JSONLimitError is returned as is.
func parseJSONBigInt(
	data []byte,
	v jsonValue,
	st *jsonDecodeState,
	context string,
) (*big.Int, error) {
	raw := jsonRawSpan(data, v)
	if v.kind == jsonKindNumber {
		digits := len(raw)
		if digits > 0 && raw[0] == '-' {
			digits--
		}
		if digits > MaxJSONIntegerDigits {
			return nil, &JSONLimitError{
				Limit:  "integer digits",
				Max:    MaxJSONIntegerDigits,
				Actual: digits,
			}
		}
		st.digits += digits
		if st.digits > MaxJSONTotalIntegerDigits {
			return nil, &JSONLimitError{
				Limit:  "total integer digits",
				Max:    MaxJSONTotalIntegerDigits,
				Actual: st.digits,
			}
		}
		if n := st.slab.parsePlainInteger(raw); n != nil {
			return n, nil
		}
	}
	var n *big.Int
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("%s: %w", context, err)
	}
	return n, nil
}

// parsePlainInteger converts a JSON number consisting only of an optional
// minus sign and digits that fits in one machine word. It returns nil for
// anything else (fraction, exponent, larger values), which the caller then
// converts with encoding/json so those cases behave exactly as before.
func (s *jsonSlab) parsePlainInteger(raw []byte) *big.Int {
	body := raw
	neg := false
	if len(body) > 0 && body[0] == '-' {
		neg = true
		body = body[1:]
	}
	// 18 digits always fit in a uint64; a 32-bit word is checked below.
	if len(body) == 0 || len(body) > 18 {
		return nil
	}
	var u uint64
	for _, c := range body {
		if c < '0' || c > '9' {
			return nil
		}
		u = u*10 + uint64(c-'0')
	}
	if uint64(big.Word(u)) != u {
		return nil
	}
	return s.newBigFromWord(big.Word(u), neg)
}
