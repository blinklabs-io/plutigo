package replay

import (
	"strings"
	"syscall"
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
		languageVersion, err := replayCase.Language.Version()
		if err != nil {
			b.Fatal(err)
		}
		term := decoded.program.Term
		for _, argument := range decoded.arguments {
			term = &syn.Apply[syn.DeBruijn]{
				Function: term,
				Argument: &syn.Constant{Con: &syn.Data{Inner: argument}},
			}
		}
		protoVersion := cek.ProtoVersion{
			Major: replayCase.ProtocolVersion.Major,
			Minor: replayCase.ProtocolVersion.Minor,
		}
		var evalContext *cek.EvalContext
		if replayCase.CostModel.UseDefault {
			evalContext = cek.NewDefaultEvalContext(languageVersion, protoVersion)
		} else {
			evalContext, err = cek.NewEvalContext(languageVersion, protoVersion, replayCase.CostModel.Parameters)
			if err != nil {
				b.Fatal(err)
			}
		}
		budget := cek.ExBudget{Cpu: replayCase.BudgetLimit.Steps, Mem: replayCase.BudgetLimit.Memory}
		name := string(replayCase.Language) + "/" + strings.ReplaceAll(replayCase.ID, ":", "_")
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			startCPU := processCPUNanos()
			for b.Loop() {
				machine := cek.NewMachine[syn.DeBruijn](languageVersion, 200, evalContext)
				machine.ExBudget = budget
				if _, err := machine.Run(term); err != nil {
					b.Fatal(err)
				}
				used := budget.Sub(&machine.ExBudget)
				if used.Cpu != replayCase.Expected.ExUnits.Steps || used.Mem != replayCase.Expected.ExUnits.Memory {
					b.Fatalf("budget mismatch: %+v", used)
				}
			}
			b.ReportMetric(float64(processCPUNanos()-startCPU)/float64(b.N), "cpu-ns/op")
		})
	}
}

// processCPUNanos is the process's user+system CPU time, which unlike wall
// time is not inflated by other load on the host.
func processCPUNanos() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return ru.Utime.Nano() + ru.Stime.Nano()
}
