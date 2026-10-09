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

package replay

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

// BenchmarkMainnetCorpusDecode measures the ledger decode path
// (DecodeDeBruijnWithContext) on each mainnet validator in isolation, since
// a node decodes a script afresh for every redeemer that executes it.
func BenchmarkMainnetCorpusDecode(b *testing.B) {
	corpus, err := LoadFile(context.Background(), mainnetCorpusPath)
	if err != nil {
		b.Fatalf("LoadFile() failed: %v", err)
	}
	for i := range corpus.Cases {
		c := &corpus.Cases[i]
		languageVersion, err := c.Language.Version()
		if err != nil {
			b.Fatal(err)
		}
		flatProgram, err := decodeHex("flat program", c.FlatProgramHex)
		if err != nil {
			b.Fatal(err)
		}
		programContext := syn.ProgramContext{
			LedgerLanguage: languageVersion,
			ProtocolMajor:  c.ProtocolVersion.Major,
		}
		name := fmt.Sprintf("%d-%s-%dB", i, c.Language, len(flatProgram))
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := syn.DecodeDeBruijnWithContext(flatProgram, programContext); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestMainnetDecodedProgramsAreReusable runs each mainnet validator from one
// decoded program on concurrent machines, with the ledger arguments applied
// on top of the shared term, and requires the recorded cardano-node result
// from every run.
func TestMainnetDecodedProgramsAreReusable(t *testing.T) {
	ctx := context.Background()
	corpus, err := LoadFile(ctx, mainnetCorpusPath)
	if err != nil {
		t.Fatalf("LoadFile() failed: %v", err)
	}
	decoded, err := corpus.validateCases(ctx)
	if err != nil {
		t.Fatalf("validateCases() failed: %v", err)
	}
	for i := range corpus.Cases {
		replayCase := &corpus.Cases[i]
		t.Run(replayCase.ID, func(t *testing.T) {
			before, err := syn.Encode(decoded[i].program)
			if err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}
			const runs = 4
			results := make([]CaseResult, runs)
			var wg sync.WaitGroup
			for r := range runs {
				wg.Go(func() { results[r] = runDecodedCase(ctx, replayCase, decoded[i]) })
			}
			wg.Wait()
			for r, result := range results {
				if !result.Passed {
					t.Fatalf("run %d mismatches: %v", r, result.Mismatches)
				}
			}
			after, err := syn.Encode(decoded[i].program)
			if err != nil {
				t.Fatalf("Encode() after evaluation failed: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("evaluation modified the shared decoded program")
			}
		})
	}
}
