package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestNewMachine(t *testing.T) {
	version := [3]uint32{1, 2, 0}
	slippage := uint32(100)

	machine := NewMachine[syn.DeBruijn](version, slippage)

	if machine.slippage != slippage {
		t.Errorf("Expected slippage %d, got %d", slippage, machine.slippage)
	}
	if machine.version != version {
		t.Errorf("Expected version %v, got %v", version, machine.version)
	}
	if len(machine.Logs) != 0 {
		t.Errorf("Expected empty logs, got %v", machine.Logs)
	}
}

func TestNewMachineWithVersionCosts(t *testing.T) {
	version := [3]uint32{1, 2, 0}
	slippage := uint32(100)

	machine := NewMachineWithVersionCosts[syn.DeBruijn](version, slippage)

	expectedCostModel := GetCostModel(version)
	if machine.costs != expectedCostModel {
		t.Errorf(
			"Expected cost model %v, got %v",
			expectedCostModel,
			machine.costs,
		)
	}
}

func TestMachineRunSimple(t *testing.T) {
	// Simple program: (con integer 42)
	input := `(program 1.2.0 (con integer 42))`

	pprogram, err := syn.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	program, err := syn.NameToDeBruijn(pprogram)
	if err != nil {
		t.Fatalf("NameToDeBruijn error: %v", err)
	}

	machine := NewMachineWithVersionCosts[syn.DeBruijn](program.Version, 200)

	result, err := machine.Run(program.Term)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	pretty := syn.PrettyTerm[syn.DeBruijn](result)
	expected := "(con integer 42)"
	if pretty != expected {
		t.Errorf("Expected %q, got %q", expected, pretty)
	}
}

func TestMachineRunWithBuiltin(t *testing.T) {
	// Program: (program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])
	input := `(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])`

	pprogram, err := syn.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	program, err := syn.NameToDeBruijn(pprogram)
	if err != nil {
		t.Fatalf("NameToDeBruijn error: %v", err)
	}

	machine := NewMachineWithVersionCosts[syn.DeBruijn](program.Version, 200)

	result, err := machine.Run(program.Term)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	pretty := syn.PrettyTerm[syn.DeBruijn](result)
	expected := "(con integer 3)"
	if pretty != expected {
		t.Errorf("Expected %q, got %q", expected, pretty)
	}
}

func TestMachineExBudget(t *testing.T) {
	version := [3]uint32{1, 2, 0}
	machine := NewMachine[syn.DeBruijn](version, 100)

	initialBudget := machine.ExBudget

	// Run a simple program
	input := `(program 1.2.0 (con integer 42))`

	pprogram, err := syn.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	program, err := syn.NameToDeBruijn(pprogram)
	if err != nil {
		t.Fatalf("NameToDeBruijn error: %v", err)
	}

	_, err = machine.Run(program.Term)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Budget should have decreased
	if machine.ExBudget.Cpu >= initialBudget.Cpu {
		t.Error("CPU budget should have decreased")
	}
	if machine.ExBudget.Mem >= initialBudget.Mem {
		t.Error("Mem budget should have decreased")
	}
}

func TestMachineDeterministic(t *testing.T) {
	// Property: running the same program multiple times gives same result
	input := `(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])`

	pprogram, err := syn.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	program, err := syn.NameToDeBruijn(pprogram)
	if err != nil {
		t.Fatalf("NameToDeBruijn error: %v", err)
	}

	version := program.Version

	results := make([]string, 5)
	for i := 0; i < 5; i++ {
		machine := NewMachineWithVersionCosts[syn.DeBruijn](version, 200)
		result, err := machine.Run(program.Term)
		if err != nil {
			t.Fatalf("Run error on attempt %d: %v", i, err)
		}
		results[i] = syn.PrettyTerm[syn.DeBruijn](result)
	}

	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf(
				"Non-deterministic result: attempt 0: %s, attempt %d: %s",
				results[0],
				i,
				results[i],
			)
		}
	}
}

func FuzzMachineRun(f *testing.F) {
	testPrograms := []string{
		`(program 1.2.0 (con integer 42))`,
		`(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])`,
	}
	for _, prog := range testPrograms {
		f.Add(prog)
	}
	f.Fuzz(func(t *testing.T, input string) {
		pprogram, err := syn.Parse(input)
		if err != nil {
			return // Skip invalid
		}
		program, err := syn.NameToDeBruijn(pprogram)
		if err != nil {
			return
		}
		machine := NewMachineWithVersionCosts[syn.DeBruijn](
			program.Version,
			1000,
		)
		_, _ = machine.Run(program.Term) // Ignore errors, check no panic
	})
}
