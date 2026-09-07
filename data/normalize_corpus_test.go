package data

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// TestNormalizeIsIdentityOnReferenceEncodedArguments checks this package's
// default encoding against every script argument cardano-node passed to a
// real mainnet evaluation, as captured in replay/testdata/mainnet.json.
//
// Normalize resets every container to the package default, so on input
// already in that form it must be a byte-exact no-op. If a default were
// wrong in either direction -- a non-empty Constr or List defaulting to
// definite, an empty one defaulting to indefinite, or a Map defaulting to
// indefinite -- re-encoding would differ from the captured bytes and this
// test would fail. No round-trip test against hand-written fixtures can
// rule that out, because such a test only shows Normalize is
// self-consistent, not that it normalizes toward the right encoding.
//
// Be careful about what this proves on its own. Redeemers and datums
// reach the node as submitter-supplied bytes, so finding them already in
// canonical form is also consistent with the node having passed those
// bytes through untouched -- a submitter is free to encode canonically.
// This test therefore pins agreement with the reference for these values;
// it is not by itself evidence that the node re-encodes what it receives.
// TestNormalizeIsIdentityOnNodeConstructedScriptContexts below carries
// that stronger claim, on the arguments that can actually support it.
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

// TestNormalizeIsIdentityOnNodeConstructedScriptContexts pins the stronger
// half of the claim above, on the only corpus arguments that can support
// it.
//
// Redeemers and datums start life as submitter-supplied bytes, so finding
// them already in canonical form proves nothing about what the node does
// -- the submitter may simply have encoded them that way. The
// ScriptContext is different in kind: the node builds it from ledger
// state, so no submitter bytes influence it and there is no original
// encoding to preserve. Its encoding is therefore purely the reference
// implementation's canonical Data encoder.
//
// Normalize being a byte-identity on those contexts is a direct check
// that this package's defaults are that canonical encoder, which is the
// premise Normalize depends on and the reason it is safe to apply at the
// evaluator boundary.
//
// The ScriptContext is the last argument in every case: [datum, redeemer,
// context] for a PlutusV1/V2 spend, [redeemer, context] for other V1/V2
// purposes, and [context] alone for PlutusV3. That position is verified
// per case rather than assumed. A case with an unexpected argument count
// would otherwise slide the check onto a submitter-supplied datum or
// redeemer -- values a submitter is free to encode canonically -- and the
// test would report having verified node-constructed contexts while
// actually proving the strong claim on the wrong data.
func TestNormalizeIsIdentityOnNodeConstructedScriptContexts(t *testing.T) {
	buf, err := os.ReadFile(filepath.Clean(corpusPath))
	if err != nil {
		t.Fatalf("read corpus %s: %v", corpusPath, err)
	}
	var corpus corpusFile
	if err := json.Unmarshal(buf, &corpus); err != nil {
		t.Fatalf("parse corpus: %v", err)
	}

	// decoded counts contexts that reached the comparison, identity counts
	// the ones that matched. Kept separate so the vacuity guard can tell
	// "nothing decoded" apart from "everything failed the check" -- the
	// latter being precisely the failure this test exists to report.
	decoded := 0
	identity := 0
	for _, tc := range corpus.Cases {
		wantArgs, err := scriptContextArgCount(tc.ID, tc.Language)
		if err != nil {
			t.Errorf("%s: %v", tc.ID, err)
			continue
		}
		if len(tc.ArgumentsHex) != wantArgs {
			t.Errorf(
				"%s (%s): expected %d arguments, got %d;"+
					" the last argument may not be the ScriptContext,"+
					" so this case is not safe to check",
				tc.ID, tc.Language, wantArgs, len(tc.ArgumentsHex),
			)
			continue
		}
		ctxHex := tc.ArgumentsHex[len(tc.ArgumentsHex)-1]
		raw, err := hex.DecodeString(ctxHex)
		if err != nil {
			t.Fatalf("%s: bad context hex: %v", tc.ID, err)
		}
		pd, err := Decode(raw)
		if err != nil {
			t.Errorf("%s: ScriptContext did not decode: %v", tc.ID, err)
			continue
		}
		// Every ScriptContext is a Constr in all Plutus versions. A bare
		// leaf here would mean the position check above let something
		// through that is not a context at all.
		if _, ok := pd.(*Constr); !ok {
			t.Errorf(
				"%s (%s): last argument decoded as %T, not a Constr;"+
					" it is not a ScriptContext",
				tc.ID, tc.Language, pd,
			)
			continue
		}
		decoded++
		got, err := Encode(Normalize(pd))
		if err != nil {
			t.Fatalf("%s: encode normalized context: %v", tc.ID, err)
		}
		if hex.EncodeToString(got) != ctxHex {
			t.Errorf(
				"%s (%s): Normalize altered a node-constructed ScriptContext;"+
					" this package's defaults are not the reference encoder",
				tc.ID, tc.Language,
			)
			continue
		}
		identity++
	}

	if decoded == 0 {
		t.Fatal("no ScriptContext decoded; the check proved nothing")
	}
	t.Logf(
		"Normalize is byte-identity on %d of %d node-constructed ScriptContexts",
		identity, decoded,
	)
}

// scriptContextArgCount returns how many arguments a case must have for
// its last one to be the ScriptContext. PlutusV3 unified the arguments
// into a single ScriptContext value; before that a spend script also
// received its datum, and every other purpose received only the redeemer
// alongside the context.
//
// The purpose is the part of the corpus case ID between "#" and ":", as
// in "<txid>#spend:1".
func scriptContextArgCount(id, language string) (int, error) {
	if language == "PlutusV3" {
		return 1, nil
	}
	_, rest, ok := strings.Cut(id, "#")
	if !ok {
		return 0, fmt.Errorf("case ID %q has no purpose suffix", id)
	}
	purpose, _, ok := strings.Cut(rest, ":")
	if !ok {
		return 0, fmt.Errorf("case ID %q has no purpose index", id)
	}
	switch purpose {
	case "spend":
		return 3, nil
	case "mint", "reward", "cert", "vote", "propose":
		return 2, nil
	default:
		return 0, fmt.Errorf("unrecognized script purpose %q", purpose)
	}
}
