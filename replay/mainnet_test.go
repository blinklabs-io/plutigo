package replay

import (
	"context"
	"testing"
)

const mainnetCorpusPath = "testdata/mainnet.json"

func TestMainnetCorpusParity(t *testing.T) {
	corpus, err := LoadFile(mainnetCorpusPath)
	if err != nil {
		t.Fatalf("LoadFile() failed: %v", err)
	}

	report, err := Run(context.Background(), corpus)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if report.Summary.Total != 8 {
		t.Fatalf("mainnet corpus has %d cases, want 8", report.Summary.Total)
	}
	for _, result := range report.Cases {
		t.Run(result.ID, func(t *testing.T) {
			if !result.Passed {
				t.Errorf("mismatches: %v", result.Mismatches)
			}
		})
	}
}

func BenchmarkMainnetCorpus(b *testing.B) {
	corpus, err := LoadFile(mainnetCorpusPath)
	if err != nil {
		b.Fatalf("LoadFile() failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		report, err := Run(context.Background(), corpus)
		if err != nil {
			b.Fatalf("Run() failed: %v", err)
		}
		if report.Summary.Passed != report.Summary.Total {
			b.Fatalf(
				"mainnet parity passed %d/%d cases",
				report.Summary.Passed,
				report.Summary.Total,
			)
		}
	}
	b.ReportMetric(float64(len(corpus.Cases)), "validators/op")
	b.ReportMetric(
		float64(b.Elapsed().Nanoseconds())/
			float64(b.N*len(corpus.Cases)),
		"ns/validator",
	)
}
