# Consumer Guide

How to run Plutus scripts with plutigo from another Go program. Each section
points at a runnable example in the package's `example_test.go`; `go test`
compiles and runs them, so they stay in step with the supported Go version.

## Evaluating a script

A script is evaluated for a **ledger language** (Plutus V1 to V4) at a
**protocol version**, both chosen by the caller. The `(program 1.x.y ...)` header
is the UPLC term version, not the ledger language, and must never be used to
select one.

For a textual program, `cek.EvaluateText` does the whole job: phase-1
validation for the language and protocol version, the execution-time term
version gate, then evaluation within a budget. A program that fails either
check is returned as an error and never reaches the machine. See
`ExampleEvaluateText` and `ExampleEvaluateText_rejected` in `cek/example_test.go`.

For a FLAT-encoded on-chain script, decode with
`syn.DecodeDeBruijnForExecution`, which applies the same two checks, then run
the term on a machine built with `cek.NewMachine`. See `ExampleNewMachine`.

`cek.NewMachine` requires an `EvalContext`; it panics on nil or on a context
whose machine costs would not deplete the budget. Use
`cek.NewDefaultEvalContext` where the default cost model is intended.

## Protocol versions and EvalContext

`EvalContext` carries the cost model, the semantics variant and the protocol
major version. Build one per distinct (language, protocol major, cost model
parameters) and reuse it: it is immutable and safe to share across goroutines.

- `cek.NewEvalContext(language, protoVersion, params)` builds the context from
  the ledger's cost model parameters. It rejects a model whose recurring step
  costs are not positive or whose startup cost is negative. A parameter list
  shorter than the language's name list has the missing tail costed at the
  maximum, as the reference implementation does. See `ExampleNewEvalContext`.
- `cek.NewDefaultEvalContext(language, protoVersion)` uses the default cost
  model and is meant for tests and tooling, not for ledger validation.

The protocol major version selects the semantics variant and which builtins
exist: a builtin introduced at a later protocol version fails validation, and
fails at run time on a raw machine, before that version. `DefaultFunction.IsAvailableInWithProto`
in the `builtin` package reports availability for a language and protocol version.

## Budgets and errors

The caller sets `machine.ExBudget` (or passes a budget to `EvaluateText`).
Every step and builtin charges before it runs, and the arithmetic saturates
rather than wraps, so an overflowing cost model exhausts the budget instead of
undercharging. After a run, `initial.Sub(&machine.ExBudget)` is the budget
consumed.

Evaluation errors are typed. `cek.IsBudgetError`, `IsScriptError`, `IsTypeError`,
`IsBuiltinError` and `IsInternalError` classify an error, and `cek.GetErrorCode`
returns its numeric code. Parsing, validation and cancellation errors may not
match these categories. A budget error is recoverable by raising the budget;
the others are failures of the script. See `Example_budgetAndErrors`.

`machine.RunContext(ctx, term)` stops cooperatively when `ctx` is canceled; it
starts no goroutine, so a builtin already executing finishes first.

### Metrics

`machine.EnableMetrics()` opts in to step counts by kind, the maximum frame
stack depth and per-builtin call counts and budget totals, read with
`machine.Metrics()` after a run. A metered machine runs the generic evaluation
loop; the machine without metrics is unchanged. See `ExampleMachine_Metrics`
and `BenchmarkMachineMetrics`.

## PlutusData and JSON

`data.EncodeJSON` and `data.DecodeJSON` convert `PlutusData` to and from the
Cardano detailed JSON schema (`{"constructor":0,"fields":[{"int":42}]}`), and
`data.Encode` and `data.Decode` convert to and from CBOR. See
`ExampleEncodeJSON` in `data/example_test.go`.
