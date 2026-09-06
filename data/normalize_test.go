package data

import (
	"encoding/hex"
	"math/big"
	"testing"
)

func mustDecodeHex(t *testing.T, s string) PlutusData {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad fixture hex %q: %v", s, err)
	}
	pd, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode(%s): %v", s, err)
	}
	return pd
}

func mustEncodeHex(t *testing.T, pd PlutusData) string {
	t.Helper()
	b, err := Encode(pd)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return hex.EncodeToString(b)
}

// TestNormalizeResetsDefiniteLengthEncoding is the regression test for the
// mainnet Plutus divergence. Decode preserves whichever definite/indefinite
// encoding each node carried on the wire, so re-encoding a decoded value
// reproduces the original bytes. cardano-ledger always rebuilds
// script-visible values fresh, i.e. at this package's default, so a script
// calling serialiseData on pass-through decoded data observed different
// bytes than the reference implementation and rejected canonical
// transactions.
//
// Each case decodes wire bytes whose encoding differs from the default,
// asserts the round-trip really is byte-identical to that input (proving the
// fidelity that causes the bug, so the test cannot silently pass for the
// wrong reason), then asserts Normalize re-encodes to the default. Without
// Normalize the final assertion fails: the value still carries the wire's
// useIndef from Decode.
//
// Note the direction is not the same for every type. Constr and List default
// to indefinite when non-empty, so their rows go definite -> indefinite; Map
// always defaults to definite, so its row goes indefinite -> definite. The
// field is named wire rather than definite for that reason.
func TestNormalizeResetsDefiniteLengthEncoding(t *testing.T) {
	tests := []struct {
		name       string
		wire       string
		wantNormal string
	}{
		{
			// Constr tag 0 (CBOR tag 121 = d879), one field: integer 1.
			// 81 = definite array(1); 9f..ff = indefinite.
			name:       "constr one field",
			wire:       "d8798101",
			wantNormal: "d8799f01ff",
		},
		{
			// 82 = definite array(2); 9f..ff = indefinite.
			name:       "list two items",
			wire:       "820102",
			wantNormal: "9f0102ff",
		},
		{
			// Map is the one container whose package default is
			// DEFINITE ("to match Haskell's canonical CBOR", see
			// Map.MarshalCBOR), so the meaningful direction here is the
			// reverse: indefinite (bf..ff) on the wire must normalize
			// back to definite (a1).
			name:       "map one pair, indefinite on the wire",
			wire:       "bf0102ff",
			wantNormal: "a10102",
		},
		{
			// Definite list holding a definite constr: Normalize must
			// recurse, not just reset the outermost node.
			name:       "nested list of constr",
			wire:       "81d8798101",
			wantNormal: "9fd8799f01ffff",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decoded := mustDecodeHex(t, tc.wire)

			// Guard: if this stops holding, Decode no longer preserves
			// wire encoding and the rest of this test proves nothing.
			if got := mustEncodeHex(t, decoded); got != tc.wire {
				t.Fatalf(
					"precondition failed: decode/encode round-trip = %s, want the original %s",
					got, tc.wire,
				)
			}

			if got := mustEncodeHex(t, Normalize(decoded)); got != tc.wantNormal {
				t.Errorf("Normalize round-trip = %s, want %s", got, tc.wantNormal)
			}
		})
	}
}

// TestNormalizeEmptyStaysDefinite pins the other half of the package
// default: empty containers encode definite-length, so Normalize must not
// turn them indefinite.
func TestNormalizeEmptyStaysDefinite(t *testing.T) {
	tests := []struct {
		name  string
		input PlutusData
		want  string
	}{
		{"empty constr", &Constr{Tag: big.NewInt(0)}, "d87980"},
		{"empty list", &List{}, "80"},
		{"empty map", &Map{}, "a0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustEncodeHex(t, Normalize(tc.input)); got != tc.want {
				t.Errorf("Normalize = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestNormalizeDoesNotMutateInput guards the deep copy. The callers added
// alongside Normalize hand it values that are still referenced elsewhere
// (witness datums reused across redeemers, for example), so mutating in
// place would corrupt the script-data hash computed from the original
// preserved bytes.
func TestNormalizeDoesNotMutateInput(t *testing.T) {
	const definite = "81d8798101"

	decoded := mustDecodeHex(t, definite)
	normalized := Normalize(decoded)

	if got := mustEncodeHex(t, decoded); got != definite {
		t.Errorf("input mutated by Normalize: re-encodes to %s, want %s", got, definite)
	}
	if normalized == decoded {
		t.Error("Normalize returned the same pointer; it must return a copy")
	}
}

// TestNormalizeCopiesConstrTag guards the one mutable field a container
// node owns. big.Int is mutable, so aliasing the tag would let a write
// through the normalized value reach the original.
func TestNormalizeCopiesConstrTag(t *testing.T) {
	original := &Constr{Tag: big.NewInt(3), Fields: []PlutusData{}}

	normalized, ok := Normalize(original).(*Constr)
	if !ok {
		t.Fatalf("Normalize returned %T, want *Constr", Normalize(original))
	}
	if normalized.Tag == original.Tag {
		t.Fatal("normalized Constr aliases the original's Tag pointer")
	}

	normalized.Tag.SetInt64(9)
	if original.Tag.Int64() != 3 {
		t.Errorf("mutating the normalized tag changed the original to %d, want 3", original.Tag.Int64())
	}
}

// TestNormalizeNilConstrTagStaysNil pins the nil case: MarshalCBOR treats a
// nil tag as zero, so Normalize must not turn nil into a non-nil zero and
// must not panic trying to copy it.
func TestNormalizeNilConstrTagStaysNil(t *testing.T) {
	normalized, ok := Normalize(&Constr{}).(*Constr)
	if !ok {
		t.Fatal("Normalize did not return a *Constr")
	}
	if normalized.Tag != nil {
		t.Errorf("nil Tag became %v, want nil", normalized.Tag)
	}
}

// TestNormalizeLeafPassthrough documents that Integer and ByteString carry
// no encoding-style state, so Normalize returns them untouched.
func TestNormalizeLeafPassthrough(t *testing.T) {
	tests := []struct {
		name  string
		input PlutusData
	}{
		{"integer", &Integer{Inner: big.NewInt(42)}},
		{"bytestring", &ByteString{Inner: []byte{0xde, 0xad}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.input); got != tc.input {
				t.Errorf("Normalize(%s) returned a copy; leaves should pass through", tc.name)
			}
		})
	}
}
