package data

import (
	"encoding/hex"
	"math/big"
	"testing"
)

// arenaCapFixtures cover every arena slice allocation path: definite and
// indefinite lists and maps (small, spilled past the stack buffer, and
// empty), constr fields in each tag form, and definite and chunked byte
// strings.
var arenaCapFixtures = map[string]string{
	"definite list":    "830102" + "03",
	"indefinite small": "9f0102ff",
	"indefinite spill": "9f010203040506070809ff",
	"definite map":     "a2" + "0102" + "0304",
	"indefinite map":   "bf0102" + "0304" + "0506" + "0708" + "090a" + "0b0cff",
	"constr compact":   "d8799f0102ff",
	"constr definite":  "d87982" + "0102",
	"constr general":   "d9050082" + "0102",
	"bytes definite":   "4401020304",
	"bytes chunked":    "5f4201024203" + "04ff",
	"value":            "d90579bf4101bf410201ffff",
	"nested":           "9f" + "9f0102ff" + "a1" + "0102" + "4401020304" + "ff",
}

func arenaWalk(t *testing.T, pd PlutusData, visit func(kind string, length, capacity int)) {
	t.Helper()
	switch v := pd.(type) {
	case *List:
		visit("list items", len(v.Items), cap(v.Items))
		for _, item := range v.Items {
			arenaWalk(t, item, visit)
		}
	case *Map:
		visit("map pairs", len(v.Pairs), cap(v.Pairs))
		for _, p := range v.Pairs {
			arenaWalk(t, p[0], visit)
			arenaWalk(t, p[1], visit)
		}
	case *Constr:
		visit("constr fields", len(v.Fields), cap(v.Fields))
		for _, f := range v.Fields {
			arenaWalk(t, f, visit)
		}
	case *Value:
		arenaWalk(t, v.Inner, visit)
	case *ByteString:
		visit("bytes", len(v.Inner), cap(v.Inner))
	}
}

func TestArenaDecodedSlicesCapEqualsLen(t *testing.T) {
	t.Parallel()
	for name, h := range arenaCapFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw, err := hex.DecodeString(h)
			if err != nil {
				t.Fatal(err)
			}
			pd, err := NewDecoder().Decode(raw)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			arenaWalk(t, pd, func(kind string, length, capacity int) {
				if capacity != length {
					t.Errorf("%s: cap %d != len %d", kind, capacity, length)
				}
			})
		})
	}
}

// TestArenaAppendDoesNotCorruptNeighbours appends to the first decoded
// field of every kind and checks that an adjacent live value is unchanged.
func TestArenaAppendDoesNotCorruptNeighbours(t *testing.T) {
	t.Parallel()
	// Two byte strings and two lists decoded back to back share arena chunks.
	raw, err := hex.DecodeString("9f" + "42aabb" + "42ccdd" + "820102" + "820304" + "ff")
	if err != nil {
		t.Fatal(err)
	}
	pd, err := NewDecoder().Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	list := pd.(*List)
	first := list.Items[0].(*ByteString)
	second := list.Items[1].(*ByteString)
	l1 := list.Items[2].(*List)
	l2 := list.Items[3].(*List)

	first.Inner = append(first.Inner, 0xee, 0xee)
	l1.Items = append(l1.Items, NewInteger(big.NewInt(7)), NewInteger(big.NewInt(8)))

	if got := hex.EncodeToString(second.Inner); got != "ccdd" {
		t.Errorf("second byte string overwritten: %s", got)
	}
	if got := mustEncodeHex(t, l2); got != "820304" {
		t.Errorf("second list overwritten: %s", got)
	}
}
