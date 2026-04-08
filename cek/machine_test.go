package cek

import (
	"math/big"
	"strings"
	"testing"
	"unsafe"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

type unsupportedDischargeValue struct{}

func (unsupportedDischargeValue) String() string { return "unsupportedDischargeValue" }

func (unsupportedDischargeValue) toExMem() ExMem { return 0 }

func (unsupportedDischargeValue) isValue() {}

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

func TestRunReturnsIndependentConstantTerms(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
	term := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(42)}}

	first, err := m.Run(term)
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}

	firstConst, ok := first.(*syn.Constant)
	if !ok {
		t.Fatalf("first Run returned %T, want *syn.Constant", first)
	}
	firstInt, ok := firstConst.Con.(*syn.Integer)
	if !ok {
		t.Fatalf("first Run returned %T constant, want *syn.Integer", firstConst.Con)
	}
	firstInt.Inner.SetInt64(99)

	second, err := m.Run(term)
	if err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}

	secondConst, ok := second.(*syn.Constant)
	if !ok {
		t.Fatalf("second Run returned %T, want *syn.Constant", second)
	}
	secondInt, ok := secondConst.Con.(*syn.Integer)
	if !ok {
		t.Fatalf("second Run returned %T constant, want *syn.Integer", secondConst.Con)
	}
	if got := secondInt.Inner.Int64(); got != 42 {
		t.Fatalf("second Run integer = %d, want 42", got)
	}
	if firstConst == secondConst {
		t.Fatal("Run reused the same *syn.Constant across invocations")
	}
	if firstConst.Con == secondConst.Con {
		t.Fatal("Run reused the same syn.IConstant across invocations")
	}
}

func TestRunResetsEnvArena(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)

	dbProgram, err := syn.NameToDeBruijn(buildLambdaChainProgram(envChunkSize + 1))
	if err != nil {
		t.Fatalf("NameToDeBruijn returned error: %v", err)
	}
	term := dbProgram.Term

	for _, run := range []string{"first", "second"} {
		if _, err := m.Run(term); err != nil {
			t.Fatalf("%s Run returned error: %v", run, err)
		}
		if got := len(m.envChunks); got > envRetainChunkCap {
			t.Fatalf("len(envChunks) after %s Run = %d, want <= %d", run, got, envRetainChunkCap)
		}
		if m.envChunkPos != 0 {
			t.Fatalf("envChunkPos after %s Run = %d, want 0", run, m.envChunkPos)
		}
	}
}

func TestResetEnvArenaRetainsOnlyTrackedChunkHeaders(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)

	usedChunks := envRetainChunkCap + 2
	for i := 0; i < envChunkSize*usedChunks; i++ {
		m.extendEnv(nil, &Constant{&syn.Integer{Inner: big.NewInt(int64(i))}})
	}
	if len(m.envChunks) < usedChunks {
		t.Fatalf("expected multiple env chunks, got %d", len(m.envChunks))
	}

	m.resetEnvArena()

	if len(m.envChunks) != envRetainChunkCap {
		t.Fatalf("len(envChunks) after reset = %d, want %d", len(m.envChunks), envRetainChunkCap)
	}
	if cap(m.envChunks) != envRetainChunkCap {
		t.Fatalf("cap(envChunks) after reset = %d, want %d", cap(m.envChunks), envRetainChunkCap)
	}
	if m.envChunkPos != 0 {
		t.Fatalf("envChunkPos after reset = %d, want 0", m.envChunkPos)
	}
}

func TestAllocArenaSliceReusesLaterChunk(t *testing.T) {
	chunks := [][]int{
		make([]int, 4),
		make([]int, 8),
	}
	pos := 3

	allocated := allocArenaSlice(&chunks, &pos, 2, 4)

	if len(chunks) != 2 {
		t.Fatalf("allocArenaSlice appended a chunk, len(chunks) = %d, want 2", len(chunks))
	}
	if pos != 6 {
		t.Fatalf("allocArenaSlice pos = %d, want 6", pos)
	}
	if unsafe.SliceData(allocated) != unsafe.SliceData(chunks[1]) {
		t.Fatal("allocArenaSlice did not reuse the next existing chunk from its start")
	}
}

func TestLookupEnvUsesOneIndexedDepth(t *testing.T) {
	var env *Env[syn.DeBruijn]
	env = env.Extend(&Constant{&syn.Integer{Inner: big.NewInt(1)}})
	env = env.Extend(&Constant{&syn.Integer{Inner: big.NewInt(2)}})
	env = env.Extend(&Constant{&syn.Integer{Inner: big.NewInt(3)}})

	// Deep env for testing the general-loop fallback path (idx >= 5)
	var deepEnv *Env[syn.DeBruijn]
	for i := int64(1); i <= 7; i++ {
		deepEnv = deepEnv.Extend(&Constant{&syn.Integer{Inner: big.NewInt(i)}})
	}

	tests := []struct {
		name         string
		env          *Env[syn.DeBruijn]
		idx          int
		wantIntValue int64 // -1 means expect !ok
		wantOk       bool
	}{
		{"idx=1 returns most recent (3)", env, 1, 3, true},
		{"idx=3 returns oldest (1)", env, 3, 1, true},
		{"idx=0 out of bounds", env, 0, -1, false},
		{"idx=4 out of bounds", env, 4, -1, false},
		{"nil env", nil, 1, -1, false},
		// Deep-env fallback loop (idx >= 5)
		{"deep idx=5 traverses loop", deepEnv, 5, 3, true},
		{"deep idx=6 traverses loop", deepEnv, 6, 2, true},
		{"deep idx=7 returns oldest", deepEnv, 7, 1, true},
		{"deep idx=8 out of bounds", deepEnv, 8, -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := lookupEnv(tt.env, tt.idx)
			if ok != tt.wantOk {
				t.Fatalf("lookupEnv(%d) ok = %v, want %v", tt.idx, ok, tt.wantOk)
			}
			if !tt.wantOk {
				return
			}
			intValue, ok := value.(*Constant)
			if !ok {
				t.Fatalf("lookupEnv(%d) returned %T, want *Constant", tt.idx, value)
			}
			got, ok := intValue.Constant.(*syn.Integer)
			if !ok {
				t.Fatalf("lookupEnv(%d) constant = %T, want *syn.Integer", tt.idx, intValue.Constant)
			}
			if got.Inner.Int64() != tt.wantIntValue {
				t.Fatalf("lookupEnv(%d) = %d, want %d", tt.idx, got.Inner.Int64(), tt.wantIntValue)
			}
		})
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
	m.unbudgetedSteps = [9]uint32{1, 1, 1, 1, 1, 1, 1, 1, 1}
	m.unbudgetedTotal = 9

	if _, err := m.Run(constTerm); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if m.ExBudget != firstRemaining {
		t.Fatalf("ExBudget after second Run = %+v, want %+v", m.ExBudget, firstRemaining)
	}
	if len(m.Logs) != 0 {
		t.Fatalf("Logs after second Run = %v, want empty", m.Logs)
	}
	if got := m.unbudgetedSteps; got != [9]uint32{} {
		t.Fatalf("unbudgetedSteps after second Run = %v, want zeroed", got)
	}
	if m.unbudgetedTotal != 0 {
		t.Fatalf("unbudgetedTotal after second Run = %d, want 0", m.unbudgetedTotal)
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

func TestRunResetsFrameStackAcrossInvocations(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)

	identity := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body:          &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
	}
	nestedApply := &syn.Apply[syn.DeBruijn]{
		Function: identity,
		Argument: &syn.Apply[syn.DeBruijn]{
			Function: identity,
			Argument: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(7)}},
		},
	}

	if _, err := m.Run(nestedApply); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if len(m.frameStack) != 0 {
		t.Fatalf("frameStack len after first Run = %d, want 0", len(m.frameStack))
	}
	if m.frameStackUsed != 0 {
		t.Fatalf("frameStackUsed after first Run = %d, want 0", m.frameStackUsed)
	}
	hiddenAfterFirst := m.frameStack[:cap(m.frameStack)]
	for i := range hiddenAfterFirst {
		if hiddenAfterFirst[i].value != nil || hiddenAfterFirst[i].env != nil || hiddenAfterFirst[i].term != nil {
			t.Fatalf("frameStack[%d] retained stale data after first Run: %+v", i, hiddenAfterFirst[i])
		}
	}

	staleValue := &Constant{Constant: &syn.Integer{Inner: big.NewInt(99)}}
	m.frameStack = append(m.frameStack, stackFrame[syn.DeBruijn]{
		kind:  frameAwaitArg,
		value: staleValue,
		env:   &Env[syn.DeBruijn]{data: staleValue},
		term:  &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(99)}},
	})
	m.frameStackUsed = len(m.frameStack)

	if _, err := m.Run(&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(42)}}); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if len(m.frameStack) != 0 {
		t.Fatalf("frameStack len after second Run = %d, want 0", len(m.frameStack))
	}
	if m.frameStackUsed != 0 {
		t.Fatalf("frameStackUsed after second Run = %d, want 0", m.frameStackUsed)
	}

	hidden := m.frameStack[:cap(m.frameStack)]
	if len(hidden) == 0 {
		t.Fatal("expected frameStack storage to be retained")
	}
	if hidden[0].value != nil || hidden[0].env != nil || hidden[0].term != nil {
		t.Fatalf("frameStack[0] retained stale data: %+v", hidden[0])
	}
}

func TestDischargeValueUnsupportedValueReturnsInternalError(t *testing.T) {
	_, err := dischargeValue[syn.DeBruijn](unsupportedDischargeValue{})
	if err == nil {
		t.Fatal("expected internal error for unsupported value kind")
	}
	if !IsInternalError(err) {
		t.Fatalf("expected InternalError, got %T", err)
	}
	if !strings.Contains(err.Error(), "unsupportedDischargeValue") {
		t.Fatalf("expected error to mention unsupported value type, got %q", err.Error())
	}
}
