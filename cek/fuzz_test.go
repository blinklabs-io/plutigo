package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func FuzzMachineRun(f *testing.F) {
	for _, input := range fuzzMachineProgramSeeds() {
		f.Add(input)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 2048 {
			t.Skip()
		}

		program, err := syn.Parse(input)
		if err != nil {
			return
		}

		dbProgram, err := syn.NameToDeBruijn(program)
		if err != nil {
			return
		}

		machine := NewMachine[syn.DeBruijn](dbProgram.Version, 0, nil)
		machine.ExBudget = ExBudget{Mem: 500_000, Cpu: 5_000_000}
		_, _ = machine.Run(dbProgram.Term)
	})
}

func fuzzMachineProgramSeeds() []string {
	return []string{
		"(program 1.0.0 (con integer 42))",
		"(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])",
		"(program 1.2.0 [(lam x x) (con integer 7)])",
		"(program 1.2.0 (force (delay (con integer 1))))",
		"(program 1.2.0 [(builtin equalsInteger) (con integer 1) (con integer 1)])",
		"(program 1.2.0 (constr 0 (con integer 1)))",
		"(program 1.2.0 (case (constr 0 (con integer 1)) (lam x x)))",
		"(program 1.2.0 (error))",
	}
}
