package replay

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestLoadRejectsUnknownFields(t *testing.T) {
	input := `{
		"schema_version": 1,
		"network": "mainnet",
		"cases": [],
		"unknown": true
	}`
	_, err := Load(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Load() error = %v, want unknown-field error", err)
	}
}

func TestCorpusValidateRejectsDuplicateIDs(t *testing.T) {
	replayCase := successfulCase(t)
	corpus := &Corpus{
		SchemaVersion: SchemaVersion,
		Network:       "mainnet",
		Reference:     testReference(),
		Cases:         []Case{replayCase, replayCase},
	}
	err := corpus.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("Validate() error = %v, want duplicate-id error", err)
	}
}

func TestRunCaseMatchesSuccessAndBudget(t *testing.T) {
	replayCase := successfulCase(t)
	first := RunCase(&replayCase)
	if !first.Actual.Success {
		t.Fatalf("first RunCase() failed: %s", first.Actual.Error)
	}
	if first.Actual.ExUnits == (ExUnits{}) {
		t.Fatal("first RunCase() consumed zero execution units")
	}

	replayCase.Expected.ExUnits = first.Actual.ExUnits
	result := RunCase(&replayCase)
	if !result.Passed {
		t.Fatalf("RunCase() mismatches = %v", result.Mismatches)
	}
}

func TestRunCaseMatchesEvaluationFailure(t *testing.T) {
	errorCode := cek.ErrCodeExplicitError
	replayCase := baseCase(t, &syn.Error{}, nil)
	replayCase.ID = "tx-error#spend:0"
	replayCase.Transaction.ID = "tx-error"
	replayCase.Expected = Expected{
		Success:   false,
		ErrorCode: &errorCode,
	}

	first := RunCase(&replayCase)
	if first.Actual.Success {
		t.Fatal("first RunCase() unexpectedly succeeded")
	}
	if first.Actual.SetupError {
		t.Fatalf("first RunCase() setup failed: %s", first.Actual.Error)
	}
	replayCase.Expected.ExUnits = first.Actual.ExUnits

	result := RunCase(&replayCase)
	if !result.Passed {
		t.Fatalf("RunCase() mismatches = %v", result.Mismatches)
	}
}

func TestRunCaseUsesLedgerLanguageInsteadOfUPLCVersion(t *testing.T) {
	term := &syn.Apply[syn.DeBruijn]{
		Function: &syn.Builtin{
			DefaultFunction: builtin.SerialiseData,
		},
		Argument: &syn.Constant{
			Con: &syn.Data{Inner: data.NewInteger(big.NewInt(42))},
		},
	}
	replayCase := baseCaseWithVersion(t, lang.LanguageVersionV2, term, nil)
	// The UPLC header identifies this as a V2 program. The ledger language must
	// still control builtin availability, so the same program is rejected as V1.
	replayCase.Language = PlutusV2
	v2Result := RunCase(&replayCase)
	if !v2Result.Actual.Success {
		t.Fatalf("Plutus V2 evaluation failed: %s", v2Result.Actual.Error)
	}

	replayCase.Language = PlutusV1
	v1Result := RunCase(&replayCase)
	if v1Result.Actual.Success {
		t.Fatal("Plutus V1 evaluation unexpectedly accepted a V2 builtin")
	}
	if v1Result.Actual.SetupError {
		t.Fatalf("Plutus V1 evaluation setup failed: %s", v1Result.Actual.Error)
	}
}

func TestRunCaseNeverTreatsSetupFailureAsExpectedScriptFailure(t *testing.T) {
	replayCase := successfulCase(t)
	replayCase.FlatProgramHex = "00"
	replayCase.Expected.Success = false
	replayCase.Expected.ExUnits = ExUnits{}

	result := RunCase(&replayCase)
	if result.Passed {
		t.Fatal("RunCase() passed an invalid FLAT program as an expected failure")
	}
	if !result.Actual.SetupError {
		t.Fatalf("RunCase() actual = %+v, want setup error", result.Actual)
	}
}

func TestRunBuildsSummary(t *testing.T) {
	first := successfulCase(t)
	firstResult := RunCase(&first)
	first.Expected.ExUnits = firstResult.Actual.ExUnits

	second := first
	second.ID = "tx-2#spend:0"
	second.Transaction.ID = "tx-2"

	corpus := &Corpus{
		SchemaVersion: SchemaVersion,
		Network:       "mainnet",
		Reference:     testReference(),
		Cases:         []Case{first, second},
	}
	report, err := Run(context.Background(), corpus)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if report.Summary.Total != 2 ||
		report.Summary.Passed != 2 ||
		report.Summary.Failed != 0 {
		t.Fatalf("Run() summary = %+v", report.Summary)
	}
	if report.Summary.TotalDurationNS <= 0 ||
		report.Summary.TransactionsPerSecond <= 0 {
		t.Fatalf("Run() timing summary = %+v", report.Summary)
	}
}

func TestRunHonorsCanceledContext(t *testing.T) {
	replayCase := successfulCase(t)
	corpus := &Corpus{
		SchemaVersion: SchemaVersion,
		Network:       "mainnet",
		Reference:     testReference(),
		Cases:         []Case{replayCase},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, corpus)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("Run() error = %v, want context cancellation", err)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	replayCase := successfulCase(t)
	input := Corpus{
		SchemaVersion: SchemaVersion,
		Network:       "mainnet",
		Reference:     testReference(),
		Cases:         []Case{replayCase},
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	loaded, err := Load(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if loaded.Cases[0].ID != replayCase.ID {
		t.Fatalf("Load() case id = %q, want %q", loaded.Cases[0].ID, replayCase.ID)
	}
}

func testReference() Reference {
	return Reference{
		Implementation: "cardano-node",
		Version:        "10.14.0.0",
	}
}

func successfulCase(t *testing.T) Case {
	t.Helper()
	argument, err := data.Encode(data.NewInteger(big.NewInt(42)))
	if err != nil {
		t.Fatalf("data.Encode() failed: %v", err)
	}
	term := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body: &syn.Var[syn.DeBruijn]{
			Name: syn.DeBruijn(1),
		},
	}
	replayCase := baseCase(t, term, []string{hex.EncodeToString(argument)})
	replayCase.Expected.Success = true
	return replayCase
}

func baseCase(
	t *testing.T,
	term syn.Term[syn.DeBruijn],
	arguments []string,
) Case {
	t.Helper()
	return baseCaseWithVersion(
		t,
		lang.LanguageVersionV1,
		term,
		arguments,
	)
}

func baseCaseWithVersion(
	t *testing.T,
	version lang.LanguageVersion,
	term syn.Term[syn.DeBruijn],
	arguments []string,
) Case {
	t.Helper()
	program := &syn.Program[syn.DeBruijn]{
		Version: version,
		Term:    term,
	}
	encoded, err := syn.Encode(program)
	if err != nil {
		t.Fatalf("syn.Encode() failed: %v", err)
	}
	return Case{
		ID: "tx-1#spend:0",
		Transaction: TransactionRef{
			ID:       "tx-1",
			Redeemer: "spend:0",
		},
		Language:         PlutusV1,
		ProtocolVersion:  ProtocolVersion{Major: 10},
		FlatProgramHex:   hex.EncodeToString(encoded),
		ArgumentsCBORHex: arguments,
		CostModel: CostModel{
			UseDefault: true,
		},
		BudgetLimit: ExUnits{
			Steps:  10_000_000_000,
			Memory: 14_000_000,
		},
	}
}
