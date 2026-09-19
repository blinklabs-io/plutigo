package cek

import (
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// firstParamIndex returns the index of the first cost-model parameter whose
// name begins with prefix.
func firstParamIndex(t *testing.T, names []string, prefix string) int {
	t.Helper()
	for i, name := range names {
		if strings.HasPrefix(name, prefix) {
			return i
		}
	}
	t.Fatalf("no cost model parameter with prefix %q", prefix)
	return -1
}

func runWithCostModelParams(
	t *testing.T,
	src string,
	protoMajor uint,
	params []int64,
) error {
	t.Helper()
	program, err := syn.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn: %v", err)
	}
	evalCtx, err := NewEvalContext(
		lang.LanguageVersionV3,
		ProtoVersion{Major: protoMajor},
		params,
	)
	if err != nil {
		t.Fatalf("NewEvalContext: %v", err)
	}
	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		0,
		evalCtx,
	)
	_, err = machine.Run(dbProgram.Term)
	return err
}

// TestShortCostModelListCostsMissingParamsAtMaxBound pins the reference
// behavior for a parameter list shorter than the version's parameter name
// list: plutus-ledger-api's tagWithParamNames fills the missing tail with
// maxBound :: Int64 so that a builtin the ledger has not yet costed cannot be
// executed within any budget. Retaining plutigo's compiled-in defaults instead
// under-costs those builtins, which is a phase-2 divergence.
func TestShortCostModelListCostsMissingParamsAtMaxBound(t *testing.T) {
	t.Parallel()

	names := lang.GetParamNamesForVersion(lang.LanguageVersionV3)
	// andByteString is available in PlutusV3 from protocol version 10, and its
	// parameters sit past the 251 entries of the PlutusV3 cost model that
	// Conway shipped, so it is a builtin a short list can leave uncosted.
	cut := firstParamIndex(t, names, "andByteString-")

	const src = `(program 1.1.0
		[ [ [ (builtin andByteString) (con bool False) ]
		    (con bytestring #ff) ]
		  (con bytestring #ff) ])`

	// Every supplied parameter is zero, so nothing but the uncosted tail can
	// exhaust the budget.
	full := make([]int64, len(names))
	if err := runWithCostModelParams(t, src, 10, full); err != nil {
		t.Fatalf("complete parameter list: Run returned error: %v", err)
	}

	short := make([]int64, cut)
	err := runWithCostModelParams(t, src, 10, short)
	if err == nil {
		t.Fatalf(
			"short parameter list (%d of %d values): Run succeeded; "+
				"andByteString was costed from compiled-in defaults instead of maxBound",
			cut,
			len(names),
		)
	}
	if !IsBudgetError(err) {
		t.Fatalf(
			"short parameter list: got %T (%v), want a budget error",
			err,
			err,
		)
	}
}

// TestEmptyCostModelListExhaustsBudgetImmediately covers the boundary case of
// the same rule: with no parameters at all, even the CEK machine's startup
// cost is maxBound, so nothing is executable.
func TestEmptyCostModelListExhaustsBudgetImmediately(t *testing.T) {
	t.Parallel()

	err := runWithCostModelParams(t, `(program 1.1.0 (con integer 1))`, 10, nil)
	if err == nil {
		t.Fatal("empty parameter list: Run succeeded, want a budget error")
	}
	if !IsBudgetError(err) {
		t.Fatalf("empty parameter list: got %T (%v), want a budget error", err, err)
	}
}
