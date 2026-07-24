package replay

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

type Report struct {
	SchemaVersion int          `json:"schema_version"`
	Network       string       `json:"network"`
	Reference     Reference    `json:"reference"`
	Cases         []CaseResult `json:"cases"`
	Summary       Summary      `json:"summary"`
}

type CaseResult struct {
	ID          string         `json:"id"`
	Transaction TransactionRef `json:"transaction"`
	Passed      bool           `json:"passed"`
	Actual      Actual         `json:"actual"`
	DurationNS  int64          `json:"duration_ns"`
	Mismatches  []string       `json:"mismatches,omitempty"`
}

type Actual struct {
	Success    bool           `json:"success"`
	ExUnits    ExUnits        `json:"ex_units"`
	SetupError bool           `json:"setup_error,omitempty"`
	Error      string         `json:"error,omitempty"`
	ErrorCode  *cek.ErrorCode `json:"error_code,omitempty"`
}

type Summary struct {
	Total                 int     `json:"total"`
	Passed                int     `json:"passed"`
	Failed                int     `json:"failed"`
	TotalDurationNS       int64   `json:"total_duration_ns"`
	MedianDurationNS      int64   `json:"median_duration_ns"`
	P95DurationNS         int64   `json:"p95_duration_ns"`
	TransactionsPerSecond float64 `json:"transactions_per_second"`
}

func Run(ctx context.Context, corpus *Corpus) (*Report, error) {
	decodedCases, err := corpus.validate()
	if err != nil {
		return nil, err
	}

	report := &Report{
		SchemaVersion: SchemaVersion,
		Network:       corpus.Network,
		Reference:     corpus.Reference,
		Cases:         make([]CaseResult, 0, len(corpus.Cases)),
	}
	durations := make([]int64, 0, len(corpus.Cases))

	for i := range corpus.Cases {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("run replay corpus: %w", err)
		}

		result := runDecodedCase(&corpus.Cases[i], decodedCases[i])
		report.Cases = append(report.Cases, result)
		durations = append(durations, result.DurationNS)
		report.Summary.TotalDurationNS += result.DurationNS
		if result.Passed {
			report.Summary.Passed++
		}
	}

	report.Summary.Total = len(report.Cases)
	report.Summary.Failed = report.Summary.Total - report.Summary.Passed
	report.Summary.MedianDurationNS = percentile(durations, 50)
	report.Summary.P95DurationNS = percentile(durations, 95)
	if report.Summary.TotalDurationNS > 0 {
		duration := time.Duration(report.Summary.TotalDurationNS)
		report.Summary.TransactionsPerSecond = float64(report.Summary.Total) /
			duration.Seconds()
	}
	return report, nil
}

func RunCase(replayCase *Case) CaseResult {
	if replayCase == nil {
		actual := setupFailure(errors.New("replay case is required"))
		return CaseResult{
			Actual:     actual,
			Mismatches: compare(Expected{}, actual),
		}
	}
	decoded, err := replayCase.validate()
	if err != nil {
		actual := setupFailure(err)
		return CaseResult{
			ID:          replayCase.ID,
			Transaction: replayCase.Transaction,
			Actual:      actual,
			Mismatches:  compare(replayCase.Expected, actual),
		}
	}
	return runDecodedCase(replayCase, decoded)
}

func runDecodedCase(replayCase *Case, decoded decodedCase) CaseResult {
	result := CaseResult{
		ID:          replayCase.ID,
		Transaction: replayCase.Transaction,
	}
	start := time.Now()
	result.Actual = evaluate(replayCase, decoded)
	result.DurationNS = time.Since(start).Nanoseconds()
	result.Mismatches = compare(replayCase.Expected, result.Actual)
	result.Passed = len(result.Mismatches) == 0
	return result
}

func evaluate(replayCase *Case, decoded decodedCase) Actual {
	languageVersion, err := replayCase.Language.Version()
	if err != nil {
		return setupFailure(err)
	}

	term := decoded.program.Term
	for _, argument := range decoded.arguments {
		term = &syn.Apply[syn.DeBruijn]{
			Function: term,
			Argument: &syn.Constant{
				Con: &syn.Data{Inner: argument},
			},
		}
	}

	protoVersion := cek.ProtoVersion{
		Major: replayCase.ProtocolVersion.Major,
		Minor: replayCase.ProtocolVersion.Minor,
	}
	var evalContext *cek.EvalContext
	if replayCase.CostModel.UseDefault {
		evalContext = cek.NewDefaultEvalContext(languageVersion, protoVersion)
	} else {
		evalContext, err = cek.NewEvalContext(
			languageVersion,
			protoVersion,
			replayCase.CostModel.Parameters,
		)
		if err != nil {
			return setupFailure(fmt.Errorf("build evaluation context: %w", err))
		}
	}

	initialBudget := cek.ExBudget{
		Cpu: replayCase.BudgetLimit.Steps,
		Mem: replayCase.BudgetLimit.Memory,
	}
	machine := cek.NewMachine[syn.DeBruijn](
		languageVersion,
		0,
		evalContext,
	)
	machine.ExBudget = initialBudget
	evalErr := runMachine(machine, term)
	consumed := initialBudget.Sub(&machine.ExBudget)

	actual := Actual{
		Success: evalErr == nil,
		ExUnits: ExUnits{
			Steps:  consumed.Cpu,
			Memory: consumed.Mem,
		},
	}
	if evalErr != nil {
		actual.Error = evalErr.Error()
		if code, ok := cek.GetErrorCode(evalErr); ok {
			actual.ErrorCode = &code
		}
	}
	return actual
}

func runMachine(
	machine *cek.Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic during script evaluation: %v", recovered)
		}
	}()
	_, err = machine.Run(term)
	return err
}

func setupFailure(err error) Actual {
	return Actual{
		Success:    false,
		SetupError: true,
		Error:      "replay setup: " + err.Error(),
	}
}

func compare(expected Expected, actual Actual) []string {
	var mismatches []string
	if actual.SetupError {
		mismatches = append(mismatches, actual.Error)
	}
	if actual.Success != expected.Success {
		mismatches = append(
			mismatches,
			fmt.Sprintf(
				"success: got %t, want %t",
				actual.Success,
				expected.Success,
			),
		)
	}
	if actual.ExUnits != expected.ExUnits {
		mismatches = append(
			mismatches,
			fmt.Sprintf(
				"ex_units: got steps=%d memory=%d, want steps=%d memory=%d",
				actual.ExUnits.Steps,
				actual.ExUnits.Memory,
				expected.ExUnits.Steps,
				expected.ExUnits.Memory,
			),
		)
	}
	if expected.ErrorCode != nil {
		if actual.ErrorCode == nil {
			mismatches = append(
				mismatches,
				fmt.Sprintf("error_code: got none, want %d", *expected.ErrorCode),
			)
		} else if *actual.ErrorCode != *expected.ErrorCode {
			mismatches = append(
				mismatches,
				fmt.Sprintf(
					"error_code: got %d, want %d",
					*actual.ErrorCode,
					*expected.ErrorCode,
				),
			)
		}
	}
	return mismatches
}

func percentile(values []int64, percentage int) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	index := ((len(sorted) * percentage) + 99) / 100
	if index == 0 {
		return sorted[0]
	}
	return sorted[index-1]
}
