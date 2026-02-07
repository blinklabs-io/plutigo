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
//	// Parse and convert to De Bruijn indices first (see syn package)
//	// slippage is the step-interval threshold for batch budget checking
//	machine := cek.NewMachine[syn.DeBruijn](program.Version, slippage, nil)
//	result, err := machine.Run(program.Term)
//	if err != nil {
//	    // Handle evaluation error or budget exhaustion
//	}
//	// result is a Value[syn.DeBruijn]
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
// an error rather than allowing unbounded computation. Pass an [EvalContext]
// to [NewMachine] to configure custom cost model parameters.
package cek
