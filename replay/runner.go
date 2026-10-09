// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// Limits bounds the work a replay may start. Case caps the budget limit of any
// single case and Corpus caps the sum of the budget limits of all cases, so
// that a corpus cannot ask for more evaluation than the caller allows.
type Limits struct {
	Case   ExUnits
	Corpus ExUnits
	Cases  int
}

const (
	// Mainnet's maxBlockExUnits: no single script evaluation can exceed it.
	defaultCaseSteps  = 20_000_000_000
	defaultCaseMemory = 62_000_000
	// defaultCorpusCases is the number of maximum-budget cases DefaultLimits
	// admits in one corpus.
	defaultCorpusCases = 1000
)

// DefaultLimits caps a case at mainnet's block execution budget and a corpus
// at defaultCorpusCases such cases.
func DefaultLimits() Limits {
	return Limits{
		Case: ExUnits{Steps: defaultCaseSteps, Memory: defaultCaseMemory},
		Corpus: ExUnits{
			Steps:  defaultCaseSteps * defaultCorpusCases,
			Memory: defaultCaseMemory * defaultCorpusCases,
		},
		Cases: defaultCorpusCases,
	}
}

func (l Limits) check(ctx context.Context, corpus *Corpus) error {
	if l.Case.Steps <= 0 || l.Case.Memory <= 0 ||
		l.Corpus.Steps <= 0 || l.Corpus.Memory <= 0 {
		return errors.New("replay limits must be positive")
	}
	if l.Cases > 0 && len(corpus.Cases) > l.Cases {
		return fmt.Errorf("replay corpus has %d cases, exceeds the case-count limit %d", len(corpus.Cases), l.Cases)
	}
	var total ExUnits
	for i := range corpus.Cases {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check replay limits: %w", err)
		}
		limit := corpus.Cases[i].BudgetLimit
		if limit.Steps > l.Case.Steps || limit.Memory > l.Case.Memory {
			return fmt.Errorf(
				"replay case %d: budget limit steps=%d memory=%d exceeds the per-case limit steps=%d memory=%d",
				i,
				limit.Steps,
				limit.Memory,
				l.Case.Steps,
				l.Case.Memory,
			)
		}
		if limit.Steps > l.Corpus.Steps-total.Steps ||
			limit.Memory > l.Corpus.Memory-total.Memory {
			return fmt.Errorf(
				"replay case %d: budget limits exceed the corpus limit steps=%d memory=%d",
				i,
				l.Corpus.Steps,
				l.Corpus.Memory,
			)
		}
		total.Steps += limit.Steps
		total.Memory += limit.Memory
	}
	return nil
}

// Run replays every case in the corpus within limits. It stops with the
// context's error once ctx is canceled, including while a case is evaluating.
// Limits are checked before any case is decoded.
func Run(ctx context.Context, corpus *Corpus, limits Limits) (*Report, error) {
	if corpus == nil {
		return nil, errors.New("replay corpus is required")
	}
	if err := limits.check(ctx, corpus); err != nil {
		return nil, err
	}
	decodedCases, err := corpus.validateCases(ctx)
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

		replayCase := &corpus.Cases[i]
		result := runDecodedCase(ctx, replayCase, decodedCases[i])
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("run replay corpus: %w", err)
		}
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

func RunCase(ctx context.Context, replayCase *Case) CaseResult {
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
	return runDecodedCase(ctx, replayCase, decoded)
}

func runDecodedCase(
	ctx context.Context,
	replayCase *Case,
	decoded decodedCase,
) CaseResult {
	result := CaseResult{
		ID:          replayCase.ID,
		Transaction: replayCase.Transaction,
	}
	start := time.Now()
	result.Actual = evaluate(ctx, replayCase, decoded)
	result.DurationNS = time.Since(start).Nanoseconds()
	result.Mismatches = compare(replayCase.Expected, result.Actual)
	// A canceled evaluation consumes no budget and carries no error code, so
	// comparison alone would match it against an expected zero-cost failure.
	if err := ctx.Err(); err != nil {
		result.Mismatches = append(
			result.Mismatches,
			"evaluation interrupted: "+err.Error(),
		)
	}
	result.Passed = len(result.Mismatches) == 0
	return result
}

// evaluation is a replay case prepared for the machine: the program applied
// to its arguments, the evaluation context and the initial budget.
type evaluation struct {
	languageVersion cek.LanguageVersion
	term            syn.Term[syn.DeBruijn]
	evalContext     *cek.EvalContext
	budget          cek.ExBudget
}

func prepareEvaluation(replayCase *Case, decoded decodedCase) (evaluation, error) {
	languageVersion, err := replayCase.Language.Version()
	if err != nil {
		return evaluation{}, err
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

	return evaluation{
		languageVersion: languageVersion,
		term:            term,
		evalContext:     decoded.evalContext,
		budget: cek.ExBudget{
			Cpu: replayCase.BudgetLimit.Steps,
			Mem: replayCase.BudgetLimit.Memory,
		},
	}, nil
}

func evaluate(ctx context.Context, replayCase *Case, decoded decodedCase) Actual {
	prepared, err := prepareEvaluation(replayCase, decoded)
	if err != nil {
		return setupFailure(err)
	}

	initialBudget := prepared.budget
	machine := cek.NewMachine[syn.DeBruijn](
		prepared.languageVersion,
		0,
		prepared.evalContext,
	)
	machine.ExBudget = initialBudget
	evalErr := runMachine(ctx, machine, prepared.term)
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
	ctx context.Context,
	machine *cek.Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic during script evaluation: %v", recovered)
		}
	}()
	_, err = machine.RunContext(ctx, term)
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
