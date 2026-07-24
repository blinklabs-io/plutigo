package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/replay"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestRunRequiresCorpus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("run() stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(
		got,
		"plutigo-replay: -corpus is required",
	) {
		t.Fatalf("run() stderr = %q, want missing corpus error", got)
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if code := run([]string{arg}, &stdout, &stderr); code != 0 {
				t.Fatalf("run() code = %d, want 0", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf(
					"run() stdout = %q, want empty",
					stdout.String(),
				)
			}
			if got := stderr.String(); !strings.Contains(
				got,
				"Usage of plutigo-replay:",
			) {
				t.Fatalf("run() stderr = %q, want usage", got)
			}
		})
	}
}

func TestRunCanceledContext(t *testing.T) {
	corpusPath := writeCorpus(t, replay.Corpus{
		SchemaVersion: replay.SchemaVersion,
		Network:       "mainnet",
		Reference:     commandReference(),
		Cases:         []replay.Case{commandReplayCase(t)},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runContext(
		ctx,
		[]string{"-corpus", corpusPath},
		&stdout,
		&stderr,
	); code != 2 {
		t.Fatalf("runContext() code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf(
			"runContext() stdout = %q, want empty",
			stdout.String(),
		)
	}
	if got := stderr.String(); !strings.Contains(got, "context canceled") {
		t.Fatalf(
			"runContext() stderr = %q, want cancellation error",
			got,
		)
	}
}

func TestRunReportsMatchingCorpus(t *testing.T) {
	replayCase := commandReplayCase(t)
	first := replay.RunCase(&replayCase)
	if !first.Actual.Success {
		t.Fatalf("replay setup failed: %s", first.Actual.Error)
	}
	replayCase.Expected.ExUnits = first.Actual.ExUnits

	corpusPath := writeCorpus(t, replay.Corpus{
		SchemaVersion: replay.SchemaVersion,
		Network:       "mainnet",
		Reference:     commandReference(),
		Cases:         []replay.Case{replayCase},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run(
		[]string{"-corpus", corpusPath, "-pretty"},
		&stdout,
		&stderr,
	); code != 0 {
		t.Fatalf(
			"run() code = %d, want 0; stderr=%s",
			code,
			stderr.String(),
		)
	}

	var report replay.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Summary.Passed != 1 || report.Summary.Failed != 0 {
		t.Fatalf("report summary = %+v", report.Summary)
	}
}

func TestRunReturnsMismatchExitCode(t *testing.T) {
	replayCase := commandReplayCase(t)
	replayCase.Expected.ExUnits = replay.ExUnits{}
	corpusPath := writeCorpus(t, replay.Corpus{
		SchemaVersion: replay.SchemaVersion,
		Network:       "mainnet",
		Reference:     commandReference(),
		Cases:         []replay.Case{replayCase},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"-corpus", corpusPath}, &stdout, &stderr); code != 1 {
		t.Fatalf(
			"run() code = %d, want 1; stderr=%s",
			code,
			stderr.String(),
		)
	}
}

func commandReference() replay.Reference {
	return replay.Reference{
		Implementation: "cardano-node",
		Version:        "10.14.0.0",
	}
}

func commandReplayCase(t *testing.T) replay.Case {
	t.Helper()
	program := &syn.Program[syn.DeBruijn]{
		Version: lang.LanguageVersionV1,
		Term: &syn.Lambda[syn.DeBruijn]{
			Body: &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
		},
	}
	encodedProgram, err := syn.Encode(program)
	if err != nil {
		t.Fatalf("encode program: %v", err)
	}
	encodedArg, err := data.Encode(data.NewInteger(big.NewInt(42)))
	if err != nil {
		t.Fatalf("encode argument: %v", err)
	}

	return replay.Case{
		ID: "tx-command#spend:0",
		Transaction: replay.TransactionRef{
			ID:       "tx-command",
			Redeemer: "spend:0",
		},
		Language:         replay.PlutusV1,
		ProtocolVersion:  replay.ProtocolVersion{Major: 10},
		FlatProgramHex:   hex.EncodeToString(encodedProgram),
		ArgumentsCBORHex: []string{hex.EncodeToString(encodedArg)},
		CostModel:        replay.CostModel{UseDefault: true},
		BudgetLimit: replay.ExUnits{
			Steps:  10_000_000_000,
			Memory: 14_000_000,
		},
		Expected: replay.Expected{Success: true},
	}
}

func writeCorpus(t *testing.T, corpus replay.Corpus) string {
	t.Helper()
	encoded, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("encode corpus: %v", err)
	}
	path := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatalf("write corpus: %v", err)
	}
	return path
}
