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

package syn

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

// fusedDecodeCorpus returns FLAT programs to compare decoders on: the
// benchmark scripts, the node ground-truth scripts, the conformance programs
// and deterministic bit-flip mutations of them. It keeps every stride-th
// program of each source, so a caller can sample the corpus without paying
// to build all of it.
func fusedDecodeCorpus(t *testing.T, stride int) [][]byte {
	t.Helper()
	keep := func(i int) bool { return i%stride == 0 }
	var corpus [][]byte
	paths, err := filepath.Glob("../tests/bench/*.flat")
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range paths {
		if !keep(i) {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		corpus = append(corpus, b)
	}
	f, err := os.Open("../cek/testdata/node_ground_truth_pv11.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var gt struct {
		Scripts []struct {
			Flat string `json:"flat"`
		} `json:"scripts"`
	}
	if err := json.NewDecoder(zr).Decode(&gt); err != nil {
		t.Fatal(err)
	}
	for i, sc := range gt.Scripts {
		if !keep(i) {
			continue
		}
		b, err := hex.DecodeString(sc.Flat)
		if err != nil {
			t.Fatal(err)
		}
		corpus = append(corpus, b)
	}
	uplcs, err := filepath.Glob("../tests/conformance/*/*/*.uplc")
	if err != nil {
		t.Fatal(err)
	}
	more, _ := filepath.Glob("../tests/conformance/*/*/*/*.uplc")
	uplcs = append(uplcs, more...)
	for i, p := range uplcs {
		if !keep(i) {
			continue
		}
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		program, err := Parse(string(src))
		if err != nil {
			continue
		}
		db, err := NameToDeBruijn(program)
		if err != nil {
			continue
		}
		b, err := Encode(db)
		if err != nil {
			continue
		}
		corpus = append(corpus, b)
	}
	rng := rand.New(rand.NewSource(1))
	originals := append([][]byte(nil), corpus...)
	for _, original := range originals {
		if len(original) == 0 {
			continue
		}
		for range 8 {
			b := append([]byte(nil), original...)
			bit := rng.Intn(len(b) * 8)
			b[bit/8] ^= 1 << (bit % 8)
			corpus = append(corpus, b)
		}
	}
	return corpus
}

// legacyDecodeWithContext is DecodeDeBruijnWithContext as it was before the
// decoder applied the context checks: a decode that only takes the constr
// field limit from the context, followed by a ValidateProgram walk.
func legacyDecodeWithContext(b []byte, context ProgramContext) (*Program[DeBruijn], error) {
	d := newDecoder(b)
	d.strictValues = true
	d.maxConstrFields = constrFieldLimit(context.ProtocolMajor)
	program, err := decodeDeBruijnProgram(d, newTermArena[DeBruijn](), &constantArena{}, nil)
	if err != nil {
		return nil, err
	}
	if err := ValidateProgram(program, context); err != nil {
		return nil, err
	}
	return program, nil
}

func TestDecodeWithContextMatchesDecodeThenValidate(t *testing.T) {
	if testing.Short() {
		t.Skip("compares about 170k decodes")
	}
	// The race detector, with coverage, slows decoding a hundredfold; it
	// samples the corpus and compares two contexts: PlutusV1 at protocol 9,
	// where most builtins are unavailable, and PlutusV3 at protocol 11, with
	// its constant limits.
	stride, minPrograms := 1, 1000
	contexts := []ProgramContext{}
	for _, ledger := range []lang.LanguageVersion{
		lang.LanguageVersionV1, lang.LanguageVersionV2,
		lang.LanguageVersionV3, lang.LanguageVersionV4,
	} {
		for _, proto := range []uint{9, 10, 11, 12} {
			context := ProgramContext{LedgerLanguage: ledger, ProtocolMajor: proto}
			if validateLedgerLanguageAvailability(context) == nil {
				contexts = append(contexts, context)
			}
		}
	}
	if raceEnabled {
		stride, minPrograms = 64, 25
		contexts = []ProgramContext{
			{LedgerLanguage: lang.LanguageVersionV1, ProtocolMajor: 9},
			{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: 11},
		}
	}
	corpus := fusedDecodeCorpus(t, stride)
	if len(corpus) < minPrograms {
		t.Fatalf("corpus has %d programs", len(corpus))
	}
	compared, rejected := 0, 0
	for _, context := range contexts {
		ledger, proto := context.LedgerLanguage, context.ProtocolMajor
		for i, b := range corpus {
			_, gotErr := DecodeDeBruijnWithContext(b, context)
			_, wantErr := legacyDecodeWithContext(b, context)
			compared++
			if (gotErr == nil) != (wantErr == nil) {
				t.Fatalf("program %d, %v PV%d: fused error %v, legacy error %v", i, ledger, proto, gotErr, wantErr)
			}
			if gotErr != nil {
				rejected++
				if gotErr.Error() != wantErr.Error() {
					t.Fatalf("program %d, %v PV%d: fused error %q, legacy error %q", i, ledger, proto, gotErr, wantErr)
				}
			}
		}
	}
	t.Logf("%d decodes compared, %d rejected", compared, rejected)
}
