# Agent Guide for Plutigo

This document provides comprehensive guidance for AI agents working on the plutigo codebase.

## Quick Start

```bash
# Verify environment
go version          # Must be 1.24+
make test           # Run all tests
golangci-lint run   # Lint check
```

## Project Overview

**plutigo** is a pure Go implementation of Untyped Plutus Core (UPLC), the smart contract VM for Cardano blockchain.

### Key Concepts

| Term | Definition |
| ------ | ---------- |
| UPLC | Untyped Plutus Core - the low-level language scripts compile to |
| CEK Machine | The evaluation engine (Control-Environment-Continuation) |
| Plutus Version | V1, V2, V3, V4 - different eras with different features |
| ExBudget | Execution budget (CPU + memory units) |
| PlutusData | The data format for script inputs/outputs |
| Builtin | Built-in functions like `addInteger`, `sha2_256` |

### Architecture

```text
Input UPLC → Parse (syn/) → De Bruijn → CEK Machine (cek/) → Result
                                              ↓
                                    Cost Model (budget tracking)
```

---

## Package Guide

### `cek/` - Evaluation Engine (Primary)

The CEK machine that executes UPLC programs.

| File | Purpose |
| ------ | ------- |
| `machine.go` | Core CEK machine loop, state management |
| `builtins.go` | 104 builtin implementations |
| `cost_model.go` | Budget tracking, cost model types |
| `cost_model_builtins.go` | Per-builtin cost functions |
| `env.go` | Environment (variable bindings) |
| `value.go` | Runtime value types |

**Standard evaluation flow:**
```go
program, _ := syn.Parse[syn.Name](input)
dbProgram, _ := syn.NameToDeBruijn(program)
machine := cek.NewMachine[syn.DeBruijn](dbProgram.Version, 0, nil)
result, _ := machine.Run(dbProgram.Term)
```

### `syn/` - Syntax and Parsing

| File | Purpose |
| ------ | ------- |
| `parser.go` | UPLC text parser |
| `ast.go` | Abstract syntax tree types |
| `debruijn.go` | Name to De Bruijn index conversion |
| `flat/` | FLAT binary serialization |

### `builtin/` - Builtin Definitions

| File | Purpose |
| ------ | ------- |
| `builtins.go` | DefaultFunction enum (104 builtins) |
| `availability.go` | Which builtins available in which version |
| `arity.go` | Argument counts |
| `force_count.go` | Type instantiation counts |

### `data/` - PlutusData Codec

CBOR encoding/decoding for Plutus data types.

### `lang/` - Language Constants

| File | Purpose |
| ------ | ------- |
| `version.go` | Language version constants |
| `v1.go`, `v2.go`, `v3.go` | Cost model parameter names |
| `v4.go` | **MISSING** - needs creation (Plan 11) |

---

## Development Workflow

### Before Starting Any Task

1. **Pull latest:** `git pull origin main`
2. **Understand the task:** Review related code and tests
3. **Understand acceptance criteria:** Know what "done" means

### While Working

1. **Write tests first** when feasible
2. **Run tests incrementally:** `make test-match TEST=YourTest`
3. **Keep changes focused:** One logical change per commit

### Before Completing

```bash
# Required checks
make test                    # Must pass
golangci-lint run ./...      # Must have 0 issues
nilaway ./...                # Nil safety check

# Recommended
make bench                   # If touching hot paths
```

### Commit Convention

Format: `type: description`

| Type | Use For |
| ------ | ------- |
| `feat` | New feature |
| `fix` | Bug fix |
| `refactor` | Code restructuring (no behavior change) |
| `test` | Test additions/changes |
| `docs` | Documentation |
| `chore` | Build, deps, CI |

**Always sign commits:** `git commit -s -m "type: message"`

---

## Code Patterns

### Table-Driven Tests

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case1", "input1", "expected1"},
        {"case2", "input2", "expected2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Foo(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Error Handling

The codebase uses typed errors for machine failures:
```go
return nil, &BudgetError{Code: ErrCodeBudgetExhausted, Requested: cost, Available: remaining, Message: "out of budget"}
return nil, &BuiltinError{Code: ErrCodeUnimplemented, Builtin: "caseList", Message: "unimplemented"}
return nil, &InternalError{Code: ErrCodeInternalError, Message: "compute returned nil state"}
```

### Builtin Implementation Pattern

```go
func (m *Machine[T]) builtinAddInteger(args []Value) (Value, error) {
    a := args[0].(*Int)
    b := args[1].(*Int)
    return &Int{Value: new(big.Int).Add(a.Value, b.Value)}, nil
}
```

---

## Common Tasks

### Adding a New Builtin

1. Add constant to `builtin/builtins.go`
2. Update `MaxDefaultFunction`
3. Add to `builtinNames` map
4. Add availability in `builtin/availability.go`
5. Add arity in `builtin/arity.go`
6. Add force count in `builtin/force_count.go`
7. Implement in `cek/builtins.go`
8. Add cost model in `cek/cost_model_builtins.go`
9. Add conformance tests to `tests/`

### Adding a Cost Model

Cost models map input sizes to CPU/memory costs:

```go
// Constant cost
ConstantCost{Cost: 100}

// Linear in one argument
LinearCost{Intercept: 100, Slope: 10}

// Based on argument sizes
AddedSizesModel{Intercept: 100, Slope: 5}
```

### Adding Conformance Tests

1. Create `tests/builtin/<category>/<name>.uplc`
2. Create `tests/builtin/<category>/<name>.uplc.expected`
3. Create `tests/builtin/<category>/<name>.uplc.budget.expected`

---

## Debugging Tips

### Understanding CEK Machine State

The machine has three components:
- **Control:** Current term being evaluated
- **Environment:** Variable bindings
- **Continuation:** Stack of pending operations

### Common Failure Modes

| Symptom | Likely Cause |
| --------- | ------------ |
| Budget exhausted | Cost model too expensive, or infinite loop |
| Open term evaluated | De Bruijn conversion issue |
| NonConstrScrutinized | Case on non-constructor value |
| Builtin not available | Using V4 builtin in V3 script |

### Trace Output

Scripts can emit log messages using the Plutus `trace` builtin. These are captured in `machine.Logs`:

```go
machine := cek.NewMachine[syn.DeBruijn](version, 0, nil)
result, _ := machine.Run(term)
for _, log := range machine.Logs {
    fmt.Println(log)  // Logs emitted by trace builtin
}
```

---

## Active Work

Current priorities:

1. **PV11 Unified Builtins** - Protocol-version-aware builtin availability (CRITICAL)
2. **Execution Metrics** - Enables script profiling
3. **Script Context Builders** - Simplifies consumer integration

**Completed:** Typed Error System (PR #209) - see Error Recovery Playbook below

---

## File Locations by Task

| If working on... | Look at... |
| ------------------ | ---------- |
| Parsing | `syn/parser.go`, `syn/lexer.go` |
| Evaluation | `cek/machine.go` |
| Builtins | `cek/builtins.go`, `builtin/` |
| Cost models | `cek/cost_model.go`, `cek/cost_model_builtins.go` |
| FLAT encoding | `syn/flat/` |
| PlutusData | `data/` |
| Language versions | `lang/` |
| Tests | `tests/`, `*_test.go` files |

---

## Consumer Integration Example

```go
import (
    "github.com/blinklabs-io/plutigo/cek"
    "github.com/blinklabs-io/plutigo/syn"
)

func EvaluateScript(uplcHex string, budget cek.ExBudget) (cek.Value, error) {
    // Parse FLAT-encoded script
    program, err := syn.ParseFlat[syn.DeBruijn](uplcHex)
    if err != nil {
        return nil, fmt.Errorf("parse: %w", err)
    }

    // Create machine with budget
    machine := cek.NewMachine[syn.DeBruijn](program.Version, 0, nil)
    machine.ExBudget = budget

    // Evaluate
    return machine.Run(program.Term)
}
```

---

## Resources

- [Plutus Core Specification](https://github.com/IntersectMBO/plutus)
- [Cardano Improvement Proposals](https://cips.cardano.org/)
- [Effective Go](https://golang.org/doc/effective_go.html)

---

## Critical Invariants

These invariants MUST be maintained. Violating them will cause test failures or incorrect behavior.

| Invariant | Location | What Breaks If Violated |
|-----------|----------|-------------------------|
| Cost charged BEFORE computation | `cek/builtins.go` | Unbounded work possible; budget exhaustion won't stop expensive ops |
| Builtin enum order must match Haskell spec | `builtin/default_function.go` | FLAT decoding produces wrong builtins |
| De Bruijn indices are 1-indexed | `syn/conversions.go`, `cek/env.go` | Variable lookup returns wrong values |
| V1/V2/V3 param name arrays must align with cost model | `lang/v*.go` | Cost model loading panics or uses wrong costs |
| `MaxDefaultFunction` must equal highest builtin constant | `builtin/default_function.go` | Builtin parsing/iteration fails |
| Arity, ForceCount, Availability must cover all builtins | `builtin/*.go` | Panics or incorrect evaluation |
| Conformance test `.expected` files are canonical | `tests/conformance/` | False test failures |
| Object pools must reset state before Put | `cek/machine.go` | Stale state leaks between evaluations |

### Cost-Before-Computation Pattern

Every builtin MUST charge costs before performing work:

```go
func addInteger[T syn.Eval](m *Machine[T], b *Builtin[T]) (Value[T], error) {
    // 1. Extract arguments
    arg1, err := unwrapInteger[T](m.argHolder[0])
    // ...

    // 2. CHARGE COST FIRST (before computation!)
    err = m.CostTwo(&b.Func, bigIntExMem(arg1), bigIntExMem(arg2))
    if err != nil {
        return nil, err  // Budget exhausted - stop before work
    }

    // 3. Only now perform the actual computation
    newInt.Add(arg1, arg2)
    // ...
}
```

---

## Error Recovery Playbook

Use this table to diagnose and fix errors encountered during development.

### Error Classification

| Error Type | Code Range | Recoverable | When It Occurs |
|------------|------------|-------------|----------------|
| `BudgetError` | 100-199 | Yes | Script ran out of CPU/memory budget |
| `ScriptError` | 200-299 | No | Script logic failure (explicit error, missing case) |
| `TypeError` | 300-399 | No | Malformed script structure (open term, type mismatch) |
| `BuiltinError` | 400-499 | No | Builtin function failure (div by zero, decode error) |
| `InternalError` | 500-599 | No | VM implementation bug |

### Common Errors and Fixes

| Error Message | Error Type | Cause | Investigation | Fix |
|---------------|------------|-------|---------------|-----|
| `out of budget` | BudgetError | Script exhausted CPU/memory | Check `machine.ExBudget` before/after | Increase budget or optimize script |
| `open term evaluated` | TypeError | Variable not in scope | Check De Bruijn conversion in `syn/conversions.go` | Ensure all variables are bound |
| `NonConstrScrutinized` | TypeError | `case` on non-constructor value | Print scrutinee with `syn.PrettyTerm` | Ensure case scrutinee is a Constr |
| `expected Integer, got ByteString` | TypeError | Wrong argument type to builtin | Check argument order and types | Fix argument types at call site |
| `division by zero` | BuiltinError | `divideInteger` with zero divisor | Check divisor value | Add zero check before division |
| `index out of bounds` | BuiltinError | Array/bytestring index invalid | Check index vs length | Validate index before access |
| `builtin not available` | BuiltinError | Using V4 builtin in V3 script | Check `builtin.IsAvailableIn()` | Use correct Plutus version |
| `decode failure` | BuiltinError | Invalid CBOR/data format | Check input data encoding | Validate input format |
| `missing case branch` | ScriptError | Case expression has no matching branch | Check number of case alternatives | Add missing case branches |

### Error Handling Patterns

```go
// Check error type and handle appropriately
result, err := machine.Run(term)
if err != nil {
    // Get numeric error code for metrics/logging
    if code, ok := cek.GetErrorCode(err); ok {
        metrics.RecordError(code)
    }

    switch {
    case cek.IsBudgetError(err):
        // Recoverable - could retry with more budget
        log.Warn("budget exhausted", "remaining", machine.ExBudget)
    case cek.IsScriptError(err):
        // Script logic failure - contract rejected
        log.Info("script failed", "error", err)
    case cek.IsTypeError(err):
        // Malformed script - should not happen with valid UPLC
        log.Error("type error", "error", err)
    case cek.IsBuiltinError(err):
        // Builtin failure - check inputs
        log.Error("builtin error", "error", err)
    case cek.IsInternalError(err):
        // VM bug - report issue
        log.Error("internal error", "error", err)
    }
    return err
}
```

---

## Code Location by Concept

Use this table to find where to make changes for specific tasks.

### Feature Implementation

| Task | Primary File | Supporting Files |
|------|--------------|------------------|
| Add a new builtin | `cek/builtins.go` | `builtin/default_function.go`, `builtin/availability.go`, `builtin/arity.go`, `builtin/force_count.go`, `cek/cost_model_builtins.go` |
| Change evaluation semantics | `cek/machine.go` | `cek/state.go`, `cek/value.go`, `cek/env.go` |
| Add a new Plutus version | `lang/version.go` | `lang/v*.go`, `builtin/availability.go`, `cek/cost_model.go` |
| Modify CBOR encoding | `data/encode.go` | `data/decode.go`, `data/data.go` |
| Change parsing behavior | `syn/parser.go` | `syn/lex/lexer.go`, `syn/lex/token.go` |
| Modify pretty printing | `syn/pretty.go` | `syn/term.go` |
| Change FLAT serialization | `syn/flat_encode.go` | `syn/flat_decode.go` |

### Bug Investigation

| Symptom | Start Here | Related Files |
|---------|------------|---------------|
| Wrong evaluation result | `cek/machine.go:step()` | `cek/builtins.go`, `cek/value.go` |
| Budget calculation wrong | `cek/cost_model_builtins.go` | `cek/cost_model.go`, `lang/v*.go` |
| Parsing fails on valid UPLC | `syn/parser.go` | `syn/lex/lexer.go` |
| FLAT decode fails | `syn/flat_decode.go` | `builtin/default_function.go` |
| Conformance test fails | `tests/conformance/` | Check `.expected` file matches spec |

### Test Locations

| Testing | Location | Command |
|---------|----------|---------|
| Unit tests | `*_test.go` in each package | `go test ./...` |
| Conformance tests | `tests/conformance/` | `go test ./tests/...` |
| Benchmarks | `*_test.go` with `Benchmark*` funcs | `make bench` |
| Fuzz tests | `*_test.go` with `Fuzz*` funcs | `make fuzz` |

---

## Pre-Commit Validation

Run validation before committing to catch CI failures early:

```bash
# Full validation (recommended)
./scripts/validate.sh

# Quick validation (skip benchmarks)
./scripts/validate.sh --quick

# Auto-fix formatting issues
./scripts/validate.sh --fix
```

The script runs:
1. Code formatting check
2. All tests with race detection
3. golangci-lint (must have 0 issues)
4. nilaway (nil safety analysis)
5. Quick benchmark sanity check
