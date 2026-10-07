package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// cancelAfterErrCalls reports cancellation from its limit-th Err call on, so a
// test cancels at a deterministic point inside the code that polls it.
type cancelAfterErrCalls struct {
	calls atomic.Int64
	limit int64
	done  chan struct{}
	once  sync.Once
}

func (*cancelAfterErrCalls) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelAfterErrCalls) Done() <-chan struct{}     { return c.done }
func (*cancelAfterErrCalls) Value(any) any               { return nil }
func (c *cancelAfterErrCalls) Err() error {
	if c.calls.Add(1) >= c.limit {
		c.once.Do(func() { close(c.done) })
		return context.Canceled
	}
	return nil
}

func divergentCase(t *testing.T) Case {
	t.Helper()
	omega := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body: &syn.Apply[syn.DeBruijn]{
			Function: &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
			Argument: &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
		},
	}
	return baseCase(t, &syn.Apply[syn.DeBruijn]{
		Function: omega,
		Argument: omega,
	}, nil)
}

func paramsWithZeroedStepCost(t *testing.T) []int64 {
	t.Helper()
	names := lang.GetParamNamesForVersion(lang.LanguageVersionV1)
	params := make([]int64, len(names))
	for i, name := range names {
		params[i] = 1_000 + int64(i)
		if name == "cekVarCost-exBudgetCPU" {
			params[i] = 0
		}
	}
	return params
}

func TestLoadRejectsInvalidCostModelParameters(t *testing.T) {
	t.Parallel()

	replayCase := successfulCase(t)
	replayCase.CostModel = CostModel{Parameters: paramsWithZeroedStepCost(t)}
	encoded, err := json.Marshal(validCorpus(replayCase))
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	_, err = Load(context.Background(), bytes.NewReader(encoded))
	if err == nil || !strings.Contains(err.Error(), "cekVarCost-exBudgetCPU") {
		t.Fatalf("Load() error = %v, want the invalid cekVarCost parameter", err)
	}
}

func TestLoadStopsOnCanceledContext(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(validCorpus(successfulCase(t)))
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = Load(ctx, bytes.NewReader(encoded))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Load() error = %v, want context cancellation", err)
	}
}

func TestRunStopsActiveEvaluationOnCancellation(t *testing.T) {
	t.Parallel()

	corpus := validCorpus(divergentCase(t))
	ctx := &cancelAfterErrCalls{limit: 20, done: make(chan struct{})}

	report, err := Run(ctx, corpus, DefaultLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() = %+v, %v; want context cancellation", report, err)
	}
}

func TestRunEnforcesLimits(t *testing.T) {
	t.Parallel()

	replayCase := successfulCase(t)
	second := replayCase
	second.ID = "tx-2#spend:0"
	second.Transaction.ID = "tx-2"
	corpus := &Corpus{
		SchemaVersion: SchemaVersion,
		Network:       "mainnet",
		Reference:     testReference(),
		Cases:         []Case{replayCase, second},
	}
	budget := replayCase.BudgetLimit

	tests := []struct {
		name      string
		limits    Limits
		wantError string
	}{
		{
			name: "limits met exactly",
			limits: Limits{
				Case:   budget,
				Corpus: ExUnits{Steps: 2 * budget.Steps, Memory: 2 * budget.Memory},
			},
		},
		{
			name: "case steps over limit",
			limits: Limits{
				Case:   ExUnits{Steps: budget.Steps - 1, Memory: budget.Memory},
				Corpus: ExUnits{Steps: 2 * budget.Steps, Memory: 2 * budget.Memory},
			},
			wantError: "per-case limit",
		},
		{
			name: "case memory over limit",
			limits: Limits{
				Case:   ExUnits{Steps: budget.Steps, Memory: budget.Memory - 1},
				Corpus: ExUnits{Steps: 2 * budget.Steps, Memory: 2 * budget.Memory},
			},
			wantError: "per-case limit",
		},
		{
			name: "corpus steps over limit",
			limits: Limits{
				Case:   budget,
				Corpus: ExUnits{Steps: 2*budget.Steps - 1, Memory: 2 * budget.Memory},
			},
			wantError: "corpus limit",
		},
		{
			name: "corpus memory over limit",
			limits: Limits{
				Case:   budget,
				Corpus: ExUnits{Steps: 2 * budget.Steps, Memory: 2*budget.Memory - 1},
			},
			wantError: "corpus limit",
		},
		{
			name:      "zero limits",
			limits:    Limits{},
			wantError: "must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report, err := Run(context.Background(), corpus, tt.limits)
			if tt.wantError == "" {
				if err != nil || report.Summary.Total != 2 {
					t.Fatalf("Run() = %+v, %v; want both cases run", report, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Run() error = %v, want %q", err, tt.wantError)
			}
			if report != nil {
				t.Fatalf("Run() report = %+v, want none when limits are exceeded", report)
			}
		})
	}
}

func TestRunCaseNeverPassesCanceledEvaluation(t *testing.T) {
	t.Parallel()

	replayCase := successfulCase(t)
	// A canceled evaluation consumes nothing and carries no error code, so
	// comparison alone matches it against this expected result.
	replayCase.Expected = Expected{Success: false}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := RunCase(ctx, &replayCase)
	if result.Passed {
		t.Fatalf("RunCase() = %+v, want a canceled evaluation to fail", result)
	}
}

func TestRunChecksLimitsBeforeValidatingCases(t *testing.T) {
	t.Parallel()

	valid := successfulCase(t)
	invalid := valid
	invalid.ID = "tx-2#spend:0"
	invalid.Transaction.ID = "tx-2"
	invalid.FlatProgramHex = "zz"
	budget := valid.BudgetLimit
	corpusLimit := ExUnits{Steps: 2 * budget.Steps, Memory: 2 * budget.Memory}

	tests := []struct {
		name      string
		limits    Limits
		wantError string
	}{
		{
			name:      "case count",
			limits:    Limits{Case: budget, Corpus: corpusLimit, Cases: 1},
			wantError: "case-count limit",
		},
		{
			name: "case budget",
			limits: Limits{
				Case:   ExUnits{Steps: budget.Steps - 1, Memory: budget.Memory},
				Corpus: corpusLimit,
			},
			wantError: "per-case limit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			corpus := validCorpus(valid)
			corpus.Cases = append(corpus.Cases, invalid)
			_, err := Run(context.Background(), corpus, tt.limits)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Run() error = %v, want %q before case validation", err, tt.wantError)
			}
		})
	}
}

func TestRunRejectsNilCorpus(t *testing.T) {
	t.Parallel()

	_, err := Run(context.Background(), nil, DefaultLimits())
	if err == nil || !strings.Contains(err.Error(), "corpus is required") {
		t.Fatalf("Run(nil) error = %v, want corpus is required", err)
	}
}

// cancelOnFirstRead serves its input one byte per Read and cancels on the
// first Read, counting every Read it serves.
type cancelOnFirstRead struct {
	input  *bytes.Reader
	cancel context.CancelFunc
	reads  int
}

func (r *cancelOnFirstRead) Read(p []byte) (int, error) {
	r.reads++
	r.cancel()
	if len(p) > 1 {
		p = p[:1]
	}
	return r.input.Read(p)
}

func TestLoadStopsReadingOnceCanceled(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(validCorpus(successfulCase(t)))
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelOnFirstRead{input: bytes.NewReader(encoded), cancel: cancel}

	_, err = Load(ctx, reader)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Load() error = %v, want context.Canceled", err)
	}
	if reader.reads != 1 {
		t.Fatalf("Load() made %d reads, want none after cancellation", reader.reads)
	}
}
