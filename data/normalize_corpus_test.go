package data

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// corpusPath is the real mainnet script-argument capture that the replay
// tests run against. It is read directly rather than through the replay
// package because replay imports this one, so importing it back would be a
// cycle.
const corpusPath = "../replay/testdata/mainnet.json"

type corpusFile struct {
	Network   string `json:"network"`
	Reference struct {
		Implementation string `json:"implementation"`
		Version        string `json:"version"`
	} `json:"reference"`
	Cases []struct {
		ID           string   `json:"id"`
		Language     string   `json:"language"`
		ArgumentsHex []string `json:"arguments_cbor_hex"`
	} `json:"cases"`
}

// TestNormalizeIsIdentityOnReferenceEncodedArguments is the empirical proof
// that this package's default encoding is the one cardano-ledger produces,
// which is the entire premise Normalize rests on.
//
// The arguments in replay/testdata/mainnet.json were captured from
// cardano-node (via Ogmios) for real mainnet script evaluations. That
// matters because Haskell's Data type carries no definite/indefinite
// fidelity of its own -- it is a plain algebraic data type -- so anything
// the node emits for a Data value is necessarily its canonical encoding,
// not a replay of some original wire bytes. These bytes are therefore a
// reference-implementation oracle for the canonical form.
//
// Normalize resets every container to this package's default. So on input
// that is already canonical, Normalize must be a byte-exact no-op. If any
// default were wrong in either direction -- a non-empty Constr or List
// defaulting to definite, an empty one defaulting to indefinite, or a Map
// defaulting to indefinite -- re-encoding would differ from the captured
// bytes and this test would fail.
//
// This is the test that would have caught a Normalize that "worked" while
// normalizing toward the wrong encoding, which no round-trip test against
// hand-written fixtures can rule out.
func TestNormalizeIsIdentityOnReferenceEncodedArguments(t *testing.T) {
	buf, err := os.ReadFile(filepath.Clean(corpusPath))
	if err != nil {
		t.Fatalf("read corpus %s: %v", corpusPath, err)
	}
	var corpus corpusFile
	if err := json.Unmarshal(buf, &corpus); err != nil {
		t.Fatalf("parse corpus: %v", err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("corpus has no cases; the oracle would be vacuous")
	}
	t.Logf(
		"oracle: %s capture from %s (%s), %d cases",
		corpus.Network,
		corpus.Reference.Implementation,
		corpus.Reference.Version,
		len(corpus.Cases),
	)

	// decoded counts arguments that reached the comparison at all, and is
	// what proves the oracle is non-vacuous. identity counts the ones that
	// round-tripped unchanged. They must be tracked separately: if every
	// argument decoded but every comparison failed, a single counter would
	// still read zero and the vacuity guard below would report that nothing
	// decoded -- false, and it would bury the real failure this test exists
	// to surface.
	decoded := 0
	identity := 0
	for _, tc := range corpus.Cases {
		for i, argHex := range tc.ArgumentsHex {
			raw, err := hex.DecodeString(argHex)
			if err != nil {
				t.Fatalf("%s arg %d: bad hex: %v", tc.ID, i, err)
			}
			pd, err := Decode(raw)
			if err != nil {
				// Not every script argument is required to be
				// PlutusData this package can decode; skip rather than
				// fail. The vacuity guard below catches a corpus that
				// stopped decoding entirely.
				continue
			}
			decoded++
			got, err := Encode(Normalize(pd))
			if err != nil {
				t.Fatalf("%s arg %d: encode normalized: %v", tc.ID, i, err)
			}
			if hex.EncodeToString(got) != argHex {
				t.Errorf(
					"%s arg %d (%s): Normalize changed reference-canonical bytes\n"+
						" reference: %s\n normalized: %s",
					tc.ID, i, tc.Language,
					argHex, hex.EncodeToString(got),
				)
				continue
			}
			identity++
		}
	}

	if decoded == 0 {
		t.Fatal(
			"no corpus argument decoded as PlutusData;" +
				" the oracle proved nothing",
		)
	}
	t.Logf(
		"Normalize is byte-identity on %d of %d reference-encoded arguments",
		identity, decoded,
	)
}
