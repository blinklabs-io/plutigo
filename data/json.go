package data

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
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

// unmarshalPlutusDataJSON unmarshals JSON into the appropriate PlutusData type
// by inspecting which key is present in the JSON object. It decodes directly
// from the already-extracted raw messages to avoid double-parsing.
func unmarshalPlutusDataJSON(data json.RawMessage) (PlutusData, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
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
		if err := json.Unmarshal(keys["int"], &n); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Integer value: %w", err)
		}
		if n == nil {
			return nil, errors.New("null \"int\" value in Integer JSON")
		}
		return &Integer{Inner: n}, nil
	case hasKey(keys, "bytes"):
		var s string
		if err := json.Unmarshal(keys["bytes"], &s); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ByteString value: %w", err)
		}
		decoded, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid hex in ByteString JSON: %w", err)
		}
		return &ByteString{Inner: decoded}, nil
	case hasKey(keys, "list"):
		var items []json.RawMessage
		if err := json.Unmarshal(keys["list"], &items); err != nil {
			return nil, fmt.Errorf("failed to unmarshal List value: %w", err)
		}
		if items == nil {
			return nil, errors.New("null \"list\" value in List JSON")
		}
		l := &List{Items: make([]PlutusData, len(items))}
		for i, item := range items {
			pd, err := unmarshalPlutusDataJSON(item)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal List item %d: %w", i, err)
			}
			l.Items[i] = pd
		}
		return l, nil
	case hasKey(keys, "map"):
		var pairs []mapPairJSON
		if err := json.Unmarshal(keys["map"], &pairs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Map value: %w", err)
		}
		if pairs == nil {
			return nil, errors.New("null \"map\" value in Map JSON")
		}
		m := &Map{Pairs: make([][2]PlutusData, len(pairs))}
		for i, pair := range pairs {
			k, err := unmarshalPlutusDataJSON(pair.K)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Map key %d: %w", i, err)
			}
			v, err := unmarshalPlutusDataJSON(pair.V)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Map value %d: %w", i, err)
			}
			m.Pairs[i] = [2]PlutusData{k, v}
		}
		return m, nil
	case hasKey(keys, "constructor"):
		var tag uint
		if err := json.Unmarshal(keys["constructor"], &tag); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Constr constructor tag: %w", err)
		}
		fieldsRaw, ok := keys["fields"]
		if !ok {
			return nil, errors.New("missing \"fields\" key in Constr JSON")
		}
		if string(fieldsRaw) == "null" {
			return nil, errors.New("null \"fields\" value in Constr JSON")
		}
		var fields []json.RawMessage
		if err := json.Unmarshal(fieldsRaw, &fields); err != nil {
			return nil, fmt.Errorf("failed to unmarshal Constr fields: %w", err)
		}
		c := &Constr{Tag: tag, Fields: make([]PlutusData, len(fields))}
		for i, field := range fields {
			pd, err := unmarshalPlutusDataJSON(field)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Constr field %d: %w", i, err)
			}
			c.Fields[i] = pd
		}
		return c, nil
	default:
		return nil, fmt.Errorf("unrecognized PlutusData JSON keys: %v", keysOf(keys))
	}
}

func hasKey(m map[string]json.RawMessage, key string) bool {
	_, ok := m[key]
	return ok
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
