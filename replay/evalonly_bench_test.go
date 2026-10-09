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
	"context"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

// BenchmarkMainnetEvalOnly measures one ledger-style evaluation per
// iteration: a fresh machine and Run over a pre-decoded program and
// arguments, with the evaluation context built once, as a node does.
func BenchmarkMainnetEvalOnly(b *testing.B) {
	corpus, err := LoadFile(context.Background(), mainnetCorpusPath)
	if err != nil {
		b.Fatalf("LoadFile() failed: %v", err)
	}
	decodedCases, err := corpus.validateCases(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	for i := range corpus.Cases {
		replayCase := &corpus.Cases[i]
		decoded := decodedCases[i]
		prepared, err := prepareEvaluation(replayCase, decoded)
		if err != nil {
			b.Fatal(err)
		}
		name := string(replayCase.Language) + "/" + strings.ReplaceAll(replayCase.ID, ":", "_")
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			startCPU := processCPUNanos()
			for b.Loop() {
				machine := cek.NewMachine[syn.DeBruijn](prepared.languageVersion, 200, prepared.evalContext)
				machine.ExBudget = prepared.budget
				if _, err := machine.Run(prepared.term); err != nil {
					b.Fatal(err)
				}
				used := prepared.budget.Sub(&machine.ExBudget)
				if used.Cpu != replayCase.Expected.ExUnits.Steps || used.Mem != replayCase.Expected.ExUnits.Memory {
					b.Fatalf("budget mismatch: %+v", used)
				}
			}
			if startCPU != 0 {
				b.ReportMetric(float64(processCPUNanos()-startCPU)/float64(b.N), "cpu-ns/op")
			}
		})
	}
}
