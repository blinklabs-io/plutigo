package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestSmokeBuild(t *testing.T) {
	// Ensure the package builds and a CEK machine can be allocated.
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
	if m == nil {
		t.Fatal("NewMachine returned nil")
	}
}

func TestNewMachine(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
	if m == nil {
		t.Fatal("expected machine, got nil")
	}
	// check default budget
	if m.ExBudget != DefaultExBudget {
		t.Fatalf("expected default budget, got %+v", m.ExBudget)
	}
}

func TestRunConstant(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)

	// construct a simple constant term (integer)
	term := &syn.Constant{
		Con: &syn.Integer{Inner: big.NewInt(42)},
	}

	out, err := m.Run(term)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if out == nil {
		t.Fatal("Run returned nil term")
	}
}

func TestRunResetsEnvArena(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)

	dbProgram, err := syn.NameToDeBruijn(buildLambdaChainProgram(envChunkSize + 1))
	if err != nil {
		t.Fatalf("NameToDeBruijn returned error: %v", err)
	}
	term := dbProgram.Term

	if _, err := m.Run(term); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if got := len(m.envChunks); got > 1 {
		t.Fatalf("len(envChunks) after first Run = %d, want <= 1", got)
	}
	if m.envChunkPos != 0 {
		t.Fatalf("envChunkPos after first Run = %d, want 0", m.envChunkPos)
	}

	if _, err := m.Run(term); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if got := len(m.envChunks); got > 1 {
		t.Fatalf("len(envChunks) after second Run = %d, want <= 1", got)
	}
	if m.envChunkPos != 0 {
		t.Fatalf("envChunkPos after second Run = %d, want 0", m.envChunkPos)
	}
}

func TestResetEnvArenaClearsDroppedChunkHeaders(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)

	for i := 0; i < envChunkSize+1; i++ {
		m.extendEnv(nil, &Constant{&syn.Integer{Inner: big.NewInt(int64(i))}})
	}
	if len(m.envChunks) < 2 {
		t.Fatalf("expected multiple env chunks, got %d", len(m.envChunks))
	}

	m.resetEnvArena()

	if len(m.envChunks) != 1 {
		t.Fatalf("len(envChunks) after reset = %d, want 1", len(m.envChunks))
	}
	if m.envChunkPos != 0 {
		t.Fatalf("envChunkPos after reset = %d, want 0", m.envChunkPos)
	}

	hidden := m.envChunks[:cap(m.envChunks)]
	for i := 1; i < len(hidden); i++ {
		if hidden[i] != nil {
			t.Fatalf("envChunks[%d] still references a dropped chunk", i)
		}
	}
}

func TestNewMachinePanicsForNonDeBruijnEval(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected panic for non-DeBruijn machine type")
		}
	}()

	_ = NewMachine[syn.NamedDeBruijn](lang.LanguageVersionV3, 0, nil)
}

func TestRunResetsTransientStateAcrossInvocations(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
	initialBudget := ExBudget{Mem: 100_000, Cpu: 1_000_000}
	m.ExBudget = initialBudget

	constTerm := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(42)}}
	if _, err := m.Run(constTerm); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	firstRemaining := m.ExBudget
	m.Logs = append(m.Logs, "stale log")
	m.unbudgetedSteps = [10]uint32{1, 1, 1, 1, 1, 1, 1, 1, 1, 9}

	if _, err := m.Run(constTerm); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if m.ExBudget != firstRemaining {
		t.Fatalf("ExBudget after second Run = %+v, want %+v", m.ExBudget, firstRemaining)
	}
	if len(m.Logs) != 0 {
		t.Fatalf("Logs after second Run = %v, want empty", m.Logs)
	}
	if got := m.unbudgetedSteps; got != [10]uint32{} {
		t.Fatalf("unbudgetedSteps after second Run = %v, want zeroed", got)
	}
}

func TestRunUsesUpdatedBudgetOverrideOnReuse(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)

	term := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(7)}}
	firstBudget := ExBudget{Mem: 100_000, Cpu: 1_000_000}
	secondBudget := ExBudget{Mem: 200_000, Cpu: 2_000_000}

	m.ExBudget = firstBudget
	if _, err := m.Run(term); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}

	m.ExBudget = secondBudget
	if _, err := m.Run(term); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if !(m.ExBudget.Mem < secondBudget.Mem && m.ExBudget.Cpu < secondBudget.Cpu) {
		t.Fatalf("ExBudget after override run = %+v, want remaining budget below %+v", m.ExBudget, secondBudget)
	}
}
