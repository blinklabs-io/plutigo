// Package cek implements the CEK (Control, Environment, Kontinuation) machine
// for evaluating Untyped Plutus Core (UPLC) programs.
//
// The CEK machine is the standard abstract machine for evaluating lambda calculus
// with explicit environments and continuations. This implementation supports
// Plutus V1 through V4 with version-aware cost modeling.
//
// # Key Types
//
//   - [Machine] - The evaluation engine, generic over variable representation
//   - [Value] - Runtime values (constants, closures, delays, builtins, constructors)
//   - [ExBudget] - CPU and memory budget tracking
//   - [CostModel] - Version-specific cost parameters
//
// # Machine States
//
// The machine uses continuation-passing style with three states:
//
//   - Compute - Evaluating a term with an environment and context
//   - Return - Returning a computed value to a waiting context
//   - Done - Evaluation complete
//
// # Basic Usage
//
//	// Validate for the ledger language and protocol version the caller
//	// supplies, then evaluate (see syn.ParseWithContext for the checks).
//	evalCtx := cek.NewDefaultEvalContext(language, protoVersion)
//	term, consumed, err := cek.EvaluateText(ctx, input, language, evalCtx, budget)
//	if err != nil {
//	    // Handle validation error, evaluation error or budget exhaustion
//	}
//
// To run a decoded program directly, build the machine with an explicit
// [EvalContext]; [NewMachine] panics on nil. slippage is the step-interval
// threshold for batch budget checking.
//
//	machine := cek.NewMachine[syn.DeBruijn](language, slippage, evalCtx)
//	result, err := machine.Run(program.Term)
//
// [Machine.RunContext] adds cooperative, synchronous cancellation. It does not
// start a goroutine; a builtin already executing must return before
// cancellation can be observed.
//
// # Performance
//
// The machine uses object pooling (sync.Pool) for state objects to reduce
// allocations. The pools use unsafe.Pointer casting and are only tested
// with [syn.DeBruijn] type parameter.
//
// # Cost Model
//
// Every operation charges costs before execution. Budget exhaustion returns
// an error rather than allowing unbounded computation. Build an [EvalContext]
// with [NewEvalContext] to use the ledger's cost model parameters, and pass it
// to [NewMachine].
package cek
