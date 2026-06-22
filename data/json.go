package data

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
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
	return json.Marshal(map[string]*big.Int{"int": i.Inner})
}

// UnmarshalJSON implements json.Unmarshaler for Integer.
func (i *Integer) UnmarshalJSON(data []byte) error {
	var raw map[string]*big.Int
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal Integer JSON: %w", err)
	}
	v, ok := raw["int"]
	if !ok {
		return errors.New("missing \"int\" key in Integer JSON")
	}
	if v == nil {
		return errors.New("null \"int\" value in Integer JSON")
	}
	i.Inner = v
	return nil
}

// MarshalJSON implements json.Marshaler for ByteString.
func (b ByteString) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"bytes": hex.EncodeToString(b.Inner)})
}

// UnmarshalJSON implements json.Unmarshaler for ByteString.
func (b *ByteString) UnmarshalJSON(data []byte) error {
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal ByteString JSON: %w", err)
	}
	v, ok := raw["bytes"]
	if !ok {
		return errors.New("missing \"bytes\" key in ByteString JSON")
	}
	decoded, err := hex.DecodeString(v)
	if err != nil {
		return fmt.Errorf("invalid hex in ByteString JSON: %w", err)
	}
	b.Inner = decoded
	return nil
}

// MarshalJSON implements json.Marshaler for List.
func (l List) MarshalJSON() ([]byte, error) {
	items := make([]json.RawMessage, len(l.Items))
	for i, item := range l.Items {
		raw, err := marshalPlutusDataJSON(item)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal List item %d: %w", i, err)
		}
		items[i] = raw
	}
	return json.Marshal(map[string][]json.RawMessage{"list": items})
}

// UnmarshalJSON implements json.Unmarshaler for List.
func (l *List) UnmarshalJSON(data []byte) error {
	var raw map[string][]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal List JSON: %w", err)
	}
	items, ok := raw["list"]
	if !ok {
		return errors.New("missing \"list\" key in List JSON")
	}
	if items == nil {
		return errors.New("null \"list\" value in List JSON")
	}
	l.Items = make([]PlutusData, len(items))
	for i, item := range items {
		pd, err := unmarshalPlutusDataJSON(item)
		if err != nil {
			return fmt.Errorf("failed to unmarshal List item %d: %w", i, err)
		}
		l.Items[i] = pd
	}
	return nil
}

// mapPairJSON is the JSON representation of a single key-value pair in a Map.
type mapPairJSON struct {
	K json.RawMessage `json:"k"`
	V json.RawMessage `json:"v"`
}

// MarshalJSON implements json.Marshaler for Map.
func (m Map) MarshalJSON() ([]byte, error) {
	pairs := make([]mapPairJSON, len(m.Pairs))
	for i, pair := range m.Pairs {
		k, err := marshalPlutusDataJSON(pair[0])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Map key %d: %w", i, err)
		}
		v, err := marshalPlutusDataJSON(pair[1])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Map value %d: %w", i, err)
		}
		pairs[i] = mapPairJSON{K: k, V: v}
	}
	return json.Marshal(map[string][]mapPairJSON{"map": pairs})
}

// UnmarshalJSON implements json.Unmarshaler for Map.
func (m *Map) UnmarshalJSON(data []byte) error {
	var raw map[string][]mapPairJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal Map JSON: %w", err)
	}
	pairs, ok := raw["map"]
	if !ok {
		return errors.New("missing \"map\" key in Map JSON")
	}
	if pairs == nil {
		return errors.New("null \"map\" value in Map JSON")
	}
	m.Pairs = make([][2]PlutusData, len(pairs))
	for i, pair := range pairs {
		k, err := unmarshalPlutusDataJSON(pair.K)
		if err != nil {
			return fmt.Errorf("failed to unmarshal Map key %d: %w", i, err)
		}
		v, err := unmarshalPlutusDataJSON(pair.V)
		if err != nil {
			return fmt.Errorf("failed to unmarshal Map value %d: %w", i, err)
		}
		m.Pairs[i] = [2]PlutusData{k, v}
	}
	return nil
}

// constrJSON is the JSON representation of a Constr.
type constrJSON struct {
	Constructor uint              `json:"constructor"`
	Fields      []json.RawMessage `json:"fields"`
}

// MarshalJSON implements json.Marshaler for Constr.
func (c Constr) MarshalJSON() ([]byte, error) {
	fields := make([]json.RawMessage, len(c.Fields))
	for i, field := range c.Fields {
		raw, err := marshalPlutusDataJSON(field)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal Constr field %d: %w", i, err)
		}
		fields[i] = raw
	}
	return json.Marshal(constrJSON{Constructor: c.Tag, Fields: fields})
}

// UnmarshalJSON implements json.Unmarshaler for Constr.
func (c *Constr) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal Constr JSON: %w", err)
	}
	tagRaw, ok := raw["constructor"]
	if !ok {
		return errors.New("missing \"constructor\" key in Constr JSON")
	}
	if err := json.Unmarshal(tagRaw, &c.Tag); err != nil {
		return fmt.Errorf("failed to unmarshal Constr constructor tag: %w", err)
	}
	fieldsRaw, ok := raw["fields"]
	if !ok {
		return errors.New("missing \"fields\" key in Constr JSON")
	}
	if string(fieldsRaw) == "null" {
		return errors.New("null \"fields\" value in Constr JSON")
	}
	var fields []json.RawMessage
	if err := json.Unmarshal(fieldsRaw, &fields); err != nil {
		return fmt.Errorf("failed to unmarshal Constr fields: %w", err)
	}
	c.Fields = make([]PlutusData, len(fields))
	for i, field := range fields {
		pd, err := unmarshalPlutusDataJSON(field)
		if err != nil {
			return fmt.Errorf("failed to unmarshal Constr field %d: %w", i, err)
		}
		c.Fields[i] = pd
	}
	return nil
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
	switch v := pd.(type) {
	case *Integer:
		return json.Marshal(v)
	case Integer:
		return json.Marshal(v)
	case *ByteString:
		return json.Marshal(v)
	case ByteString:
		return json.Marshal(v)
	case *List:
		return json.Marshal(v)
	case List:
		return json.Marshal(v)
	case *Map:
		return json.Marshal(v)
	case Map:
		return json.Marshal(v)
	case *Constr:
		return json.Marshal(v)
	case Constr:
		return json.Marshal(v)
	default:
		return nil, fmt.Errorf("unknown PlutusData type: %T", pd)
	}
}

// discriminatorKeys are the top-level JSON keys that identify each PlutusData variant.
var discriminatorKeys = []string{"int", "bytes", "list", "map", "constructor"}

// jsonDecodeState bounds recursion depth and total node count while decoding
// untrusted PlutusData JSON, mirroring the CBOR decoder's limits.
type jsonDecodeState struct {
	depth int
	nodes int
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

// jsonEntry is a single key/value member of a JSON object. Object members
// are kept in order of appearance, including duplicate keys, so the decoder
// can reproduce encoding/json's last-occurrence-wins map semantics as well
// as the Map pair node accounting for repeated "k"/"v" keys.
type jsonEntry struct {
	key string
	val jsonValue
}

// jsonValue is a JSON value parsed in a single pass over the input. start
// and end are byte offsets of the raw value within the original input, so
// scalar leaves can be handed to encoding/json verbatim without rescanning
// any enclosing structure.
type jsonValue struct {
	kind    jsonValueKind
	start   int
	end     int
	items   []jsonValue // kind == jsonKindArray
	entries []jsonEntry // kind == jsonKindObject
}

// unmarshalPlutusDataJSON unmarshals JSON into the appropriate PlutusData
// type by inspecting which key is present in the JSON object. The input is
// scanned exactly once into a lightweight syntax tree and PlutusData values
// are built from that tree, so decoding does O(size) work instead of
// re-parsing the remaining subtree at every nesting level.
func unmarshalPlutusDataJSON(data json.RawMessage) (PlutusData, error) {
	root, err := parsePlutusDataJSONTree(data)
	if err != nil {
		return nil, fmt.Errorf("PlutusData JSON must be an object: %w", err)
	}
	return decodePlutusDataJSONValue(data, root, &jsonDecodeState{}, false)
}

// parsePlutusDataJSONTree parses data into a jsonValue tree in a single
// pass, rejecting malformed JSON and trailing non-whitespace data exactly
// like json.Unmarshal does.
func parsePlutusDataJSONTree(data []byte) (jsonValue, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	root, err := parsePlutusDataJSONValue(dec, data)
	if err != nil {
		return jsonValue{}, normalizeJSONStreamError(err)
	}
	if err := checkJSONTrailingData(data, dec.InputOffset()); err != nil {
		return jsonValue{}, err
	}
	return root, nil
}

func parsePlutusDataJSONValue(
	dec *json.Decoder,
	data []byte,
) (jsonValue, error) {
	start := jsonValueStart(data, dec.InputOffset())
	tok, err := dec.Token()
	if err != nil {
		return jsonValue{}, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '[':
			var items []jsonValue
			for dec.More() {
				item, err := parsePlutusDataJSONValue(dec, data)
				if err != nil {
					return jsonValue{}, err
				}
				items = append(items, item)
			}
			// Consume the closing ']'.
			if _, err := dec.Token(); err != nil {
				return jsonValue{}, err
			}
			return jsonValue{
				kind:  jsonKindArray,
				start: start,
				end:   int(dec.InputOffset()),
				items: items,
			}, nil
		case '{':
			var entries []jsonEntry
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return jsonValue{}, err
				}
				key, ok := keyTok.(string)
				if !ok {
					// Object keys are always strings in well-formed JSON.
					return jsonValue{}, fmt.Errorf(
						"unexpected JSON object key token %v",
						keyTok,
					)
				}
				val, err := parsePlutusDataJSONValue(dec, data)
				if err != nil {
					return jsonValue{}, err
				}
				entries = append(entries, jsonEntry{
					key: internJSONKey(key),
					val: val,
				})
			}
			// Consume the closing '}'.
			if _, err := dec.Token(); err != nil {
				return jsonValue{}, err
			}
			return jsonValue{
				kind:    jsonKindObject,
				start:   start,
				end:     int(dec.InputOffset()),
				entries: entries,
			}, nil
		default:
			// Unreachable: closing delimiters are consumed above.
			return jsonValue{}, fmt.Errorf("unexpected JSON delimiter %v", t)
		}
	case nil:
		return jsonValue{kind: jsonKindNull, start: start, end: int(dec.InputOffset())}, nil
	case bool:
		return jsonValue{kind: jsonKindBool, start: start, end: int(dec.InputOffset())}, nil
	case json.Number:
		return jsonValue{kind: jsonKindNumber, start: start, end: int(dec.InputOffset())}, nil
	case string:
		return jsonValue{kind: jsonKindString, start: start, end: int(dec.InputOffset())}, nil
	default:
		// Unreachable: the cases above cover every token type.
		return jsonValue{}, fmt.Errorf("unexpected JSON token %v", tok)
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

// internJSONKey returns a static copy of well-known object keys so large
// documents do not retain one allocation per repeated key.
func internJSONKey(key string) string {
	switch key {
	case "int":
		return "int"
	case "bytes":
		return "bytes"
	case "list":
		return "list"
	case "map":
		return "map"
	case "constructor":
		return "constructor"
	case "fields":
		return "fields"
	case "k":
		return "k"
	case "v":
		return "v"
	default:
		return key
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

func decodePlutusDataJSONValue(
	data []byte,
	v jsonValue,
	st *jsonDecodeState,
	counted bool,
) (PlutusData, error) {
	st.depth++
	if st.depth > MaxDecodeNestingDepth {
		return nil, fmt.Errorf(
			"PlutusData JSON nesting exceeds max depth %d",
			MaxDecodeNestingDepth,
		)
	}
	defer func() { st.depth-- }()

	if !counted {
		if err := st.enterNode(); err != nil {
			return nil, err
		}
	}

	var keys map[string]jsonValue
	switch v.kind {
	case jsonKindObject:
		keys = make(map[string]jsonValue, len(v.entries))
		for _, e := range v.entries {
			// Duplicate keys: the last occurrence wins, matching
			// encoding/json's map unmarshaling behavior.
			keys[e.key] = e.val
		}
	case jsonKindNull:
		// json.Unmarshal leaves the destination map nil for a JSON null,
		// which then reads as an object with no keys.
	default:
		// Reproduce json.Unmarshal's type error for non-object values by
		// running it on the raw value. This is reached at most once per
		// decode, so it cannot re-introduce quadratic scanning.
		var m map[string]json.RawMessage
		err := json.Unmarshal(jsonRawSpan(data, v), &m)
		if err == nil {
			// Unreachable: every non-object, non-null kind fails above.
			err = errors.New("value is not a JSON object")
		}
		return nil, fmt.Errorf("PlutusData JSON must be an object: %w", err)
	}

	// Reject objects with multiple discriminator keys (e.g. both "int" and "bytes").
	var found []string
	for _, dk := range discriminatorKeys {
		if _, ok := keys[dk]; ok {
			found = append(found, dk)
		}
	}
	if len(found) > 1 {
		return nil, fmt.Errorf(
			"ambiguous PlutusData JSON: multiple discriminator keys %v",
			found,
		)
	}
	switch {
	case hasKey(keys, "int"):
		var n *big.Int
		if err := json.Unmarshal(jsonRawSpan(data, keys["int"]), &n); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Integer value: %w", err)
		}
		if n == nil {
			return nil, errors.New("null \"int\" value in Integer JSON")
		}
		return &Integer{Inner: n}, nil
	case hasKey(keys, "bytes"):
		var s string
		if err := json.Unmarshal(jsonRawSpan(data, keys["bytes"]), &s); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ByteString value: %w", err)
		}
		decoded, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid hex in ByteString JSON: %w", err)
		}
		return &ByteString{Inner: decoded}, nil
	case hasKey(keys, "list"):
		itemsVal := keys["list"]
		if itemsVal.kind == jsonKindNull {
			return nil, errors.New("null \"list\" value in List JSON")
		}
		items, err := decodePlutusDataJSONArray(data, itemsVal, st, "List item")
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal List value: %w", err)
		}
		return &List{Items: items}, nil
	case hasKey(keys, "map"):
		pairsVal := keys["map"]
		if pairsVal.kind == jsonKindNull {
			return nil, errors.New("null \"map\" value in Map JSON")
		}
		pairs, err := decodePlutusDataJSONMap(data, pairsVal, st)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal Map value: %w", err)
		}
		return &Map{Pairs: pairs}, nil
	case hasKey(keys, "constructor"):
		var tag uint
		if err := json.Unmarshal(jsonRawSpan(data, keys["constructor"]), &tag); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Constr constructor tag: %w", err)
		}
		fieldsVal, ok := keys["fields"]
		if !ok {
			return nil, errors.New("missing \"fields\" key in Constr JSON")
		}
		if fieldsVal.kind == jsonKindNull {
			return nil, errors.New("null \"fields\" value in Constr JSON")
		}
		fields, err := decodePlutusDataJSONArray(data, fieldsVal, st, "Constr field")
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal Constr fields: %w", err)
		}
		return &Constr{Tag: tag, Fields: fields}, nil
	default:
		return nil, fmt.Errorf("unrecognized PlutusData JSON keys: %v", keysOf(keys))
	}
}

func decodePlutusDataJSONArray(
	data []byte,
	v jsonValue,
	st *jsonDecodeState,
	itemLabel string,
) ([]PlutusData, error) {
	if v.kind != jsonKindArray {
		return nil, errors.New("value must be an array")
	}

	items := make([]PlutusData, 0, len(v.items))
	for i, item := range v.items {
		if err := st.enterNode(); err != nil {
			return nil, err
		}
		pd, err := decodePlutusDataJSONValue(data, item, st, true)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s %d: %w", itemLabel, i, err)
		}
		items = append(items, pd)
	}
	return items, nil
}

func decodePlutusDataJSONMap(
	data []byte,
	v jsonValue,
	st *jsonDecodeState,
) ([][2]PlutusData, error) {
	if v.kind != jsonKindArray {
		return nil, errors.New("value must be an array")
	}

	pairs := make([][2]PlutusData, 0, len(v.items))
	for i, pairVal := range v.items {
		pair, err := decodePlutusDataJSONMapPair(data, pairVal, st, i)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

func decodePlutusDataJSONMapPair(
	data []byte,
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
	for _, e := range v.entries {
		switch e.key {
		case "k":
			// Each occurrence counts as a node, and the last one wins,
			// mirroring the previous streaming decoder.
			if err := st.enterNode(); err != nil {
				return pair, err
			}
			kVal, kSeen = e.val, true
		case "v":
			if err := st.enterNode(); err != nil {
				return pair, err
			}
			vVal, vSeen = e.val, true
		default:
			return pair, fmt.Errorf("unexpected Map pair %d field %q", index, e.key)
		}
	}
	if !kSeen {
		return pair, fmt.Errorf("missing \"k\" key in Map pair %d", index)
	}
	if !vSeen {
		return pair, fmt.Errorf("missing \"v\" key in Map pair %d", index)
	}

	k, err := decodePlutusDataJSONValue(data, kVal, st, true)
	if err != nil {
		return pair, fmt.Errorf("failed to unmarshal Map key %d: %w", index, err)
	}
	val, err := decodePlutusDataJSONValue(data, vVal, st, true)
	if err != nil {
		return pair, fmt.Errorf("failed to unmarshal Map value %d: %w", index, err)
	}
	pair[0] = k
	pair[1] = val
	return pair, nil
}

func hasKey(m map[string]jsonValue, key string) bool {
	_, ok := m[key]
	return ok
}

func keysOf(m map[string]jsonValue) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
