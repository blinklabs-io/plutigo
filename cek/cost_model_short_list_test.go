package cek

import (
	"math"
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

// TestShortCostModelListMatchesMaxBoundPaddedList states the fill rule as an
// equivalence: for every language version and semantics variant, a list of n
// values must build the same cost model as that list padded to the full name
// list with maxBound. It covers the boundary lengths, including the 251-value
// length of the PlutusV3 model currently on chain, and the over-length case
// that must ignore its surplus values.
func TestShortCostModelListMatchesMaxBoundPaddedList(t *testing.T) {
	t.Parallel()

	for _, tc := range costModelFieldMappingCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			names := lang.GetParamNamesForVersion(tc.version)
			full := synthCostModelParams(names)
			lengths := []int{
				0,
				1,
				len(names) / 2,
				len(realPreviewV3CostModelParams),
				len(names) - 1,
				len(names),
				len(names) + 1,
			}

			for _, n := range lengths {
				if n > len(full) {
					// Over-length: the surplus value must be ignored, so the
					// expected model is the one built from the full list.
					over := append(append([]int64{}, full...), 7)
					assertCostModelsEqual(t, tc.version, tc.semantics, over, full)
					continue
				}
				short := full[:n]
				padded := append([]int64{}, short...)
				for range len(names) - n {
					padded = append(padded, math.MaxInt64)
				}
				assertCostModelsEqual(t, tc.version, tc.semantics, short, padded)
			}
		})
	}
}

// assertCostModelsEqual fails unless data and want build identical cost
// models, reporting the first dumped line that differs.
func assertCostModelsEqual(
	t *testing.T,
	version lang.LanguageVersion,
	semantics SemanticsVariant,
	data, want []int64,
) {
	t.Helper()

	got, err := costModelFromList(version, semantics, data)
	if err != nil {
		t.Fatalf("costModelFromList(%d values): %v", len(data), err)
	}
	expected, err := costModelFromList(version, semantics, want)
	if err != nil {
		t.Fatalf("costModelFromList(%d reference values): %v", len(want), err)
	}

	gotLines := strings.Split(dumpCostModel(got), "\n")
	wantLines := strings.Split(dumpCostModel(expected), "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf(
			"%d values: dump has %d lines, reference has %d",
			len(data),
			len(gotLines),
			len(wantLines),
		)
	}
	for i := range gotLines {
		if gotLines[i] != wantLines[i] {
			t.Fatalf(
				"%d values: line %d\n got: %s\nwant: %s",
				len(data),
				i,
				gotLines[i],
				wantLines[i],
			)
		}
	}
}
