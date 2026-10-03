package cek

import (
	"context"
	"errors"
	"fmt"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// EvaluateText parses a textual UPLC program, validates it for execution and
// evaluates it within budget.
//
// language is the ledger's Plutus language, never the program header's UPLC
// version, and the protocol version comes from evalCtx. Before anything is
// evaluated the program must pass phase-1 validation for that language and
// protocol version ([syn.ParseWithContext]) and the execution-time term
// version gate ([syn.ValidateTermVersionForExecution]); a program that fails
// either is returned as an error and is never given to the machine.
//
// evalCtx must have been built for the same language and protocol version.
// The returned budget is the amount consumed, which is zero when the program
// was rejected before evaluation.
func EvaluateText(
	ctx context.Context,
	input string,
	language lang.LanguageVersion,
	evalCtx *EvalContext,
	budget ExBudget,
) (syn.Term[syn.DeBruijn], ExBudget, error) {
	if evalCtx == nil {
		return nil, ExBudget{}, errors.New("evaluation context is required")
	}
	if err := evalCtx.CostModel.machineCosts.validate(); err != nil {
		return nil, ExBudget{}, fmt.Errorf("invalid cost model: %w", err)
	}
	if evalCtx.SemanticsVariant != GetSemantics(
		language,
		ProtoVersion{Major: evalCtx.ProtoMajor},
	) {
		return nil, ExBudget{}, errors.New(
			"evaluation context was not built for this ledger language and protocol version",
		)
	}

	programContext := syn.ProgramContext{
		LedgerLanguage: language,
		ProtocolMajor:  evalCtx.ProtoMajor,
	}
	program, err := syn.ParseWithContext(input, programContext)
	if err != nil {
		return nil, ExBudget{}, err
	}
	if err := syn.ValidateTermVersionForExecution(
		program.Version,
		programContext,
	); err != nil {
		return nil, ExBudget{}, err
	}
	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		return nil, ExBudget{}, err
	}

	machine := NewMachine[syn.DeBruijn](language, 0, evalCtx)
	machine.ExBudget = budget
	term, err := machine.RunContext(ctx, dbProgram.Term)
	return term, budget.Sub(&machine.ExBudget), err
}
