package data

import (
	"encoding/binary"
	"errors"
	"testing"
)

// valueNodeCount is the node count of Value {h'01': {h'02': 1}}: the tag, the
// policy map, its key, the token map, its key and the quantity.
const valueNodeCount = 6

// TestValueNodeLimitAgreesAcrossDecoders pins the node accounting of tag 1401
// at the limit boundary: both decoders must accept at exactly the node count
// and reject one below it.
func TestValueNodeLimitAgreesAcrossDecoders(t *testing.T) {
	t.Parallel()
	encoded := []byte{
		0xd9, 0x05, 0x79, // tag 1401
		0xa1, 0x41, 0x01, // {h'01':
		0xa1, 0x41, 0x02, 0x01, // {h'02': 1}}
	}
	for _, limit := range []int{valueNodeCount - 1, valueNodeCount, valueNodeCount + 1} {
		wantOK := limit >= valueNodeCount
		limits := decodeLimits{maxDepth: MaxDecodeNestingDepth(), maxNodes: limit}

		_, pkgErr := decodeWithState(encoded, newDecodeStateWithLimits(limits))
		_, arenaErr := NewDecoder().decode(encoded, newDecodeStateWithLimits(limits))

		for name, err := range map[string]error{"package": pkgErr, "arena": arenaErr} {
			if wantOK {
				if err != nil {
					t.Errorf("limit %d: %s decoder rejected: %v", limit, name, err)
				}
				continue
			}
			var limitErr *DecodeLimitError
			if !errors.As(err, &limitErr) || limitErr.Limit != "node count" {
				t.Errorf("limit %d: %s decoder error = %v, want node count limit", limit, name, err)
			}
		}
	}
}

// bigValueCBOR returns a Value with the given number of single-token
// policies, encoded with 4-byte ascending keys.
func bigValueCBOR(policies int) []byte {
	out := []byte{0xd9, 0x05, 0x79, 0xba}
	out = binary.BigEndian.AppendUint32(out, uint32(policies))
	for i := range policies {
		out = append(out, 0x44)
		out = binary.BigEndian.AppendUint32(out, uint32(i))
		out = append(out, 0xa1, 0x41, 0x01, 0x01)
	}
	return out
}

// TestValueAcceptedAtScaleByBothDecoders uses a Value whose single count is
// under MaxDecodeNodes but whose doubled count is over it.
func TestValueAcceptedAtScaleByBothDecoders(t *testing.T) {
	t.Parallel()
	encoded := bigValueCBOR(240_000)
	if nodes := 2 + 4*240_000; nodes > MaxDecodeNodes || 2*nodes <= MaxDecodeNodes {
		t.Fatalf("fixture node count %d does not straddle the limit", nodes)
	}
	if _, err := Decode(encoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, err := NewDecoder().Decode(encoded); err != nil {
		t.Fatalf("arena Decode: %v", err)
	}
}

// valueCBORWithNodes returns a Value of exactly MaxDecodeNodes+1 nodes when
// odd is set, or MaxDecodeNodes nodes otherwise. Policy 0 has two tokens; its
// first quantity is a bignum (tag 2 and its byte string) when odd is set.
func valueCBORWithNodes(odd bool) []byte {
	const policies = (MaxDecodeNodes - 4) / 4
	out := []byte{0xd9, 0x05, 0x79, 0xba}
	out = binary.BigEndian.AppendUint32(out, uint32(policies))
	out = append(out, 0x44, 0, 0, 0, 0, 0xa2, 0x41, 0x01)
	if odd {
		out = append(out, 0xc2, 0x41, 0x01)
	} else {
		out = append(out, 0x01)
	}
	out = append(out, 0x41, 0x02, 0x01)
	for i := 1; i < policies; i++ {
		out = append(out, 0x44)
		out = binary.BigEndian.AppendUint32(out, uint32(i))
		out = append(out, 0xa1, 0x41, 0x01, 0x01)
	}
	return out
}

// TestValueUnmarshalCBORCountsTag pins Value.UnmarshalCBOR to the node
// accounting of Decode: the tag is a node, so a Value one node over the limit
// is rejected by both.
func TestValueUnmarshalCBORCountsTag(t *testing.T) {
	t.Parallel()
	atLimit := valueCBORWithNodes(false)
	if _, err := Decode(atLimit); err != nil {
		t.Fatalf("Decode at limit: %v", err)
	}
	var v Value
	if err := v.UnmarshalCBOR(atLimit); err != nil {
		t.Fatalf("UnmarshalCBOR at limit: %v", err)
	}

	over := valueCBORWithNodes(true)
	var limitErr *DecodeLimitError
	if _, err := Decode(over); !errors.As(err, &limitErr) || limitErr.Limit != "node count" {
		t.Fatalf("Decode over limit: %v, want node count limit", err)
	}
	if err := v.UnmarshalCBOR(over); !errors.As(err, &limitErr) || limitErr.Limit != "node count" {
		t.Fatalf("UnmarshalCBOR over limit: %v, want node count limit", err)
	}
}
