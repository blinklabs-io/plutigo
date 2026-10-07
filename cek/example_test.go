package cek_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// deBruijnProgram parses and converts a program known to be well formed.
func deBruijnProgram(src string) *syn.Program[syn.DeBruijn] {
	program, err := syn.Parse(src)
	if err != nil {
		panic(err)
	}
	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		panic(err)
	}
	return dbProgram
}

// EvaluateText validates a textual program for the ledger language and
// protocol version the caller supplies, then evaluates it.
func ExampleEvaluateText() {
	evalCtx := cek.NewDefaultEvalContext(
		lang.LanguageVersionV3,
		cek.ProtoVersion{Major: 11},
	)
	term, consumed, err := cek.EvaluateText(
		context.Background(),
		`(program 1.1.0 [ [ (builtin addInteger) (con integer 1) ] (con integer 1) ])`,
		lang.LanguageVersionV3,
		evalCtx,
		cek.DefaultExBudget,
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(syn.PrettyTerm[syn.DeBruijn](term))
	fmt.Println("budget consumed:", consumed.Cpu > 0 && consumed.Mem > 0)
	// Output:
	// (con integer 2)
	// budget consumed: true
}

// A program that fails validation for the caller's ledger language and
// protocol version is rejected before it reaches the machine.
func ExampleEvaluateText_rejected() {
	evalCtx := cek.NewDefaultEvalContext(
		lang.LanguageVersionV3,
		cek.ProtoVersion{Major: 10},
	)
	_, _, err := cek.EvaluateText(
		context.Background(),
		`(program 1.0.0 (builtin expModInteger))`,
		lang.LanguageVersionV3,
		evalCtx,
		cek.DefaultExBudget,
	)
	fmt.Println(err)
	// Output:
	// builtin expModInteger is not available in Plutus V3 at protocol version 10
}

// A FLAT-encoded script is decoded for execution under an explicit context:
// the ledger language and protocol version come from the caller, never from
// the program header.
func ExampleNewMachine() {
	flat, err := syn.Encode(deBruijnProgram(`(program 1.1.0 [ (lam x x) (con integer 7) ])`))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	language := lang.LanguageVersionV3
	const protocolMajor = 11
	script, err := syn.DecodeDeBruijnForExecution(flat, syn.ProgramContext{
		LedgerLanguage: language,
		ProtocolMajor:  protocolMajor,
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	evalCtx := cek.NewDefaultEvalContext(
		language,
		cek.ProtoVersion{Major: protocolMajor},
	)
	machine := cek.NewMachine[syn.DeBruijn](language, 0, evalCtx)
	machine.ExBudget = cek.DefaultExBudget
	term, err := machine.RunContext(context.Background(), script.Term)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(syn.PrettyTerm[syn.DeBruijn](term))
	// Output:
	// (con integer 7)
}

// NewEvalContext builds the context from the ledger's cost model parameters
// and rejects a model under which evaluation would not consume budget.
func ExampleNewEvalContext() {
	names := lang.GetParamNamesForVersion(lang.LanguageVersionV3)
	params := make([]int64, len(names))
	for i, name := range names {
		params[i] = 1_000
		if name == "cekVarCost-exBudgetCPU" {
			params[i] = 0
		}
	}

	_, err := cek.NewEvalContext(
		lang.LanguageVersionV3,
		cek.ProtoVersion{Major: 11},
		params,
	)
	fmt.Println(err)
	// Output:
	// build cost model: cekVarCost-exBudgetCPU is 0, must be at least 1
}

// Evaluation failures carry typed errors so a caller can tell an exhausted
// budget from a failing script.
func Example_budgetAndErrors() {
	evalCtx := cek.NewDefaultEvalContext(
		lang.LanguageVersionV3,
		cek.ProtoVersion{Major: 11},
	)
	run := func(src string, budget cek.ExBudget) error {
		_, _, err := cek.EvaluateText(
			context.Background(),
			src,
			lang.LanguageVersionV3,
			evalCtx,
			budget,
		)
		return err
	}

	err := run(`(program 1.1.0 (con integer 1))`, cek.ExBudget{Cpu: 1, Mem: 1})
	var budgetErr *cek.BudgetError
	fmt.Println(cek.IsBudgetError(err), errors.As(err, &budgetErr))

	err = run(`(program 1.1.0 (error))`, cek.DefaultExBudget)
	fmt.Println(cek.IsScriptError(err), cek.IsBudgetError(err))
	// Output:
	// true true
	// true false
}

// Metrics are opt-in and describe the most recent run.
func ExampleMachine_Metrics() {
	dbProgram := deBruijnProgram(`(program 1.1.0
		[ [ (builtin addInteger) (con integer 1) ] (con integer 1) ])`)

	machine := cek.NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		0,
		cek.NewDefaultEvalContext(
			lang.LanguageVersionV3,
			cek.ProtoVersion{Major: 11},
		),
	)
	machine.EnableMetrics()
	if _, err := machine.Run(dbProgram.Term); err != nil {
		fmt.Println("error:", err)
		return
	}

	metrics := machine.Metrics()
	fmt.Println("applications:", metrics.Steps[cek.ExApply])
	fmt.Println("addInteger calls:", metrics.Builtins[builtin.AddInteger].Calls)
	// Output:
	// applications: 2
	// addInteger calls: 1
}
