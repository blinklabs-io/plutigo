package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

// BenchmarkNewMachineLedgerContext measures per-evaluation machine setup the
// way a ledger drives it: one shared EvalContext carrying a real protocol
// major, and a fresh machine per script.
func BenchmarkNewMachineLedgerContext(b *testing.B) {
	evalContext := NewDefaultEvalContext(
		LanguageVersionV2,
		ProtoVersion{Major: 8, Minor: 0},
	)
	b.ReportAllocs()
	for b.Loop() {
		m := NewMachine[syn.DeBruijn](LanguageVersionV2, 200, evalContext)
		if m == nil {
			b.Fatal("nil machine")
		}
	}
}
