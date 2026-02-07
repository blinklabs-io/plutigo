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
2. **Read relevant plan:** Check `docs/plans/*_PLAN.md`
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

See [ROADMAP.md](ROADMAP.md) for current plans. Priority order:

1. **Plan 2:** Typed Error System - enables programmatic error classification
2. **Plan 5:** Execution Metrics - enables script profiling
3. **Plan 4:** Script Context Builders - simplifies consumer integration

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
- [Project Roadmap](ROADMAP.md)
