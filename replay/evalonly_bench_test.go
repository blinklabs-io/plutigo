package replay

import (
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

// BenchmarkMainnetEvalOnly measures one ledger-style evaluation per
// iteration: a fresh machine and Run over a pre-decoded program and
// arguments, with the evaluation context built once, as a node does.
func BenchmarkMainnetEvalOnly(b *testing.B) {
	corpus, err := LoadFile(mainnetCorpusPath)
	if err != nil {
		b.Fatalf("LoadFile() failed: %v", err)
	}
	decodedCases, err := corpus.validateCases()
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
