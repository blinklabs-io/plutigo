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

package cek

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// preparedUsageCorpus returns the benchmark and node ground-truth scripts
// and deterministic bit-flip mutations of them.
func preparedUsageCorpus(t *testing.T) [][]byte {
	t.Helper()
	var corpus [][]byte
	paths, err := filepath.Glob("../tests/bench/*.flat")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		corpus = append(corpus, b)
	}
	f, err := os.Open("testdata/node_ground_truth_pv11.json.gz")
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
	for _, sc := range gt.Scripts {
		b, err := hex.DecodeString(sc.Flat)
		if err != nil {
			t.Fatal(err)
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

// TestDecodedStructureUsageMatchesWalk checks that what PrepareProgram
// derives from the decoder, whether the structure walk passes and the limits
// it uses, equals the walk itself in both scope modes.
func TestDecodedStructureUsageMatchesWalk(t *testing.T) {
	// The race detector slows the walk about tenfold; it samples the corpus.
	stride, minCompared := 1, 2000
	if raceEnabled {
		stride, minCompared = 8, 250
	}
	compared, open := 0, 0
	corpus := preparedUsageCorpus(t)
	for _, proto := range []uint{11, 12} {
		context := syn.ProgramContext{
			LedgerLanguage: lang.LanguageVersionV3,
			ProtocolMajor:  proto,
		}
		fullScope := fullScopeCheckFor(proto)
		for i := 0; i < len(corpus); i += stride {
			b := corpus[i]
			program, info, err := syn.DecodeDeBruijnWithContextInfo(
				b,
				context,
				fullScope,
			)
			if err != nil {
				continue
			}
			got, gotOK := decodedStructureUsage(info)
			var want termStructureUsage
			wantOK := validateTermStructureUsage[syn.DeBruijn](
				contextBackground, false, program.Term, nil, fullScope, &want,
			) == nil
			compared++
			if !wantOK {
				open++
			}
			if gotOK != wantOK || (wantOK && got != want) {
				t.Fatalf(
					"program %d PV%d: decoder usage %+v ok=%v, walk usage %+v ok=%v",
					i,
					proto,
					got,
					gotOK,
					want,
					wantOK,
				)
			}
		}
	}
	if compared < minCompared || open == 0 {
		t.Fatalf("compared %d programs, %d failing the walk", compared, open)
	}
	t.Logf("%d programs compared, %d failing the walk", compared, open)
}
