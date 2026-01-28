# Development Guide

This document provides comprehensive guidance for human developers working on plutigo.

> **AI agents**: See [AGENTS.md](AGENTS.md) for structured guidance optimized for automated development.

For organization-wide contributing guidelines (conventional commits, DCO sign-off, licensing, and general PR processes), see the [Blink Labs Contributing Guide](https://github.com/blinklabs-io/.github/blob/main/CONTRIBUTING.md).

---

## TL;DR - Common Tasks

| I want to... | Command |
|--------------|---------|
| Run all tests | `make test` |
| Run one test | `make test-match TEST=TestName` |
| Check my code | `golangci-lint run ./... && nilaway ./...` |
| Format code | `make format` |
| Run benchmarks | `make bench` |
| See coverage | `make test-cover && open coverage.html` |

---

## Getting Started

### Prerequisites

- **Go 1.24+** - Required for generics and toolchain features
- **make** - Build automation
- **git** - Version control

Optional but recommended:
- **golangci-lint** - Linting (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- **nilaway** - Nil safety analysis (`go install go.uber.org/nilaway/cmd/nilaway@latest`)

### Setup

```sh
# Clone the repository
git clone https://github.com/blinklabs-io/plutigo.git
cd plutigo

# Install dependencies
go mod tidy

# Verify setup
make test
```

### IDE Configuration

**VS Code** - Recommended extensions:
- `golang.go` - Go language support
- Settings: Enable `go.lintTool: golangci-lint`

**GoLand** - Enable:
- File Watchers for `gofmt` and `goimports`
- External Tools for `golangci-lint`

---

## Development Workflow

### Running Tests

```sh
# Run all tests with race detection
make test

# Run a specific test by name
make test-match TEST=TestMachineRun

# Run tests with HTML coverage report
make test-cover
# Generates coverage.html (open manually in browser)
```

### Running Benchmarks

```sh
# Run all benchmarks (5s per test)
make bench

# Benchmark comparison workflow:
make bench-baseline    # Save current performance as baseline
# ... make changes ...
make bench-compare     # Compare against baseline with benchstat
```

### Fuzz Testing

```sh
# Run all fuzz tests (10s each by default)
make fuzz

# Run specific fuzz test longer
go test -fuzz=FuzzParse -fuzztime=60s ./syn/
```

Available fuzz targets:
- `FuzzParse` - Parser robustness
- `FuzzPretty` - Pretty printer robustness
- `FuzzMachineRun` - Evaluation safety
- `FuzzDecodeCBOR` - CBOR decoding
- `FuzzFromByte` - Builtin byte parsing
- `FuzzLexerNextToken` - Lexer tokenization

### Code Formatting

```sh
# Format all code
make format
```

This runs `go fmt` and `gofmt -s` to ensure consistent formatting.

---

## Code Quality

### Linting Requirements

Both linters must pass with **zero issues** before committing:

```sh
# Run golangci-lint (style, performance, security)
golangci-lint run ./...

# Run nilaway (nil pointer safety)
nilaway ./...
```

The CI pipeline enforces these checks on all pull requests.

### Test Coverage

Current coverage: ~50%. When adding new functionality:
- Add unit tests for new functions
- Add table-driven tests for multiple cases
- Add benchmarks for performance-critical code

```sh
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Pre-Commit Checklist

Before committing:
1. `make format` - Format code
2. `golangci-lint run ./...` - Pass linting
3. `nilaway ./...` - Pass nil safety
4. `make test` - All tests pass
5. `make bench` - No performance regressions (for performance-related changes)

---

## Project-Specific Guidelines

### Package Organization

| Package | Responsibility |
| --------- | -------------- |
| `syn/` | Parsing, AST, conversions, serialization |
| `cek/` | CEK machine, evaluation, builtins, costs |
| `builtin/` | Builtin function definitions |
| `data/` | PlutusData CBOR codec |
| `lang/` | Version definitions, cost parameters |
| `tests/` | Integration tests, benchmarks |

### Naming Conventions

- **Files**: lowercase with underscores (`cost_model.go`)
- **Types**: PascalCase (`MachineState`, `ExBudget`)
- **Functions**: camelCase for private, PascalCase for public
- **Constants**: PascalCase (`DebugBudget`)
- **Test files**: `*_test.go` alongside source

### Performance Considerations

This is a VM implementation - performance matters. Follow these patterns:

**Allocation reduction:**
```go
// Pre-allocate slices with known capacity
result := make([]T, 0, expectedLen)

// Use bytes.Buffer for building byte slices
var buf bytes.Buffer

// Use strings.Builder for string concatenation
var sb strings.Builder
```

**Object pooling:**
```go
// Use sync.Pool for frequently allocated objects
obj := pool.Get().(*MyType)
defer pool.Put(obj)
```

**Avoid in hot paths:**
- Interface conversions
- Reflection
- Unnecessary allocations
- Map lookups (prefer direct field access)

### Adding Builtins

When adding a new builtin function:

1. Add to `builtin/default_function.go`:
   - Add constant with next available number
   - Update `String()` method
   - Add to `Parse()` function

2. Implement in `cek/builtins.go`:
   - Follow existing patterns
   - Charge cost before computation
   - Add to builtins map

3. Add cost model in `cek/cost_model_builtins.go`

4. Add parameters in `lang/v*.go` for appropriate version

5. Add comprehensive tests in `cek/builtins_test.go`

6. Add conformance tests if official tests exist

### Cost Model Changes

Cost models are version-specific. When modifying:

1. Identify the correct version file (`lang/v1.go`, `v2.go`, `v3.go`)
2. Update parameter names array
3. Update `cek/cost_model_builtins.go` for calculation logic
4. Run conformance tests to verify accuracy

---

## Contributing

### Finding Work

- Check [GitHub Issues](https://github.com/blinklabs-io/plutigo/issues) for open tasks
- Look for `good first issue` labels for beginner-friendly tasks
- Check `docs/plans/` for detailed implementation plans

### Creating a Branch

```sh
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/) with DCO sign-off:

```sh
git commit -s -m "feat: add new builtin function support

- Implement xyz builtin
- Add comprehensive tests
- Update documentation"
```

Common prefixes:
- `feat:` - New feature
- `fix:` - Bug fix
- `refactor:` - Code restructuring
- `perf:` - Performance improvement
- `test:` - Adding tests
- `docs:` - Documentation changes
- `chore:` - Maintenance tasks

### Pull Request Process

1. Ensure all checks pass locally
2. Push your branch to GitHub
3. Create a PR against `main`
4. Provide clear description of changes
5. Link to related issues
6. Respond to review feedback

### What Needs Tests

- All new builtins require comprehensive tests
- Bug fixes should include regression tests
- Performance changes need benchmark comparisons
- Parser changes need fuzz test verification

### What Needs Benchmarks

- Changes to `cek/machine.go`
- Changes to `cek/builtins.go`
- Changes to `cek/env.go`
- Any optimization work

---

## Debugging & Profiling

### Debugging the CEK Machine

Enable debug mode in `cek/machine.go`:
```go
const debug = true      // Enable runtime checks
const DebugBudget = true // Enable budget logging
```

### Using Delve

```sh
# Debug a specific test
dlv test ./cek/ -- -test.run TestMachineRun

# Common commands
(dlv) break cek.(*Machine).step
(dlv) continue
(dlv) print m.state
(dlv) next
```

### CPU Profiling

```sh
# Profile benchmarks
go test -bench=BenchmarkUniswap -cpuprofile=cpu.prof ./tests/

# Analyze profile
go tool pprof -http=:8080 cpu.prof
```

### Memory Profiling

```sh
# Profile memory allocations
go test -bench=BenchmarkUniswap -memprofile=mem.prof ./tests/

# Analyze allocations
go tool pprof -http=:8080 mem.prof
```

### Benchmark Comparison

```sh
# Before changes
make bench > old.txt

# After changes
make bench > new.txt

# Compare
benchstat old.txt new.txt
```

---

## Architecture Deep Dive

### Understanding UPLC

Untyped Plutus Core (UPLC) is a minimal lambda calculus with these core constructs:

| Term | Example | Description |
|------|---------|-------------|
| Variable | `x` | Reference to a bound name |
| Lambda | `(lam x body)` | Anonymous function |
| Application | `[f arg]` | Function application |
| Builtin | `addInteger` | Primitive operation |
| Constant | `(con integer 42)` | Literal value |
| Force | `(force term)` | Type instantiation |
| Delay | `(delay term)` | Deferred computation |
| Constr | `(constr 0 args...)` | Data constructor (V3+) |
| Case | `(case scrut alts...)` | Pattern matching (V3+) |

### CEK Machine

The CEK (Control, Environment, Kontinuation) machine evaluates UPLC programs:

- **Control**: The current term being evaluated
- **Environment**: Variable bindings (De Bruijn indices)
- **Kontinuation**: Stack of evaluation contexts (frames)

The machine transitions between three states:
1. `Compute` - Evaluating a term
2. `Return` - Returning a value to a waiting frame
3. `Done` - Evaluation complete

**Basic usage:**

```go
import (
    "github.com/blinklabs-io/plutigo/cek"
    "github.com/blinklabs-io/plutigo/syn"
)

// Parse UPLC text
program, err := syn.Parse[syn.Name](uplcText)
if err != nil {
    return err
}

// Convert to De Bruijn indices
dbProgram, err := syn.NameToDeBruijn(program)
if err != nil {
    return err
}

// Create machine and run
machine := cek.NewMachine[syn.DeBruijn](dbProgram.Version, 0, nil)
machine.ExBudget = cek.ExBudget{Cpu: 10_000_000, Mem: 1_000_000}

result, err := machine.Run(dbProgram.Term)
if err != nil {
    return err // Budget exhausted, evaluation error, etc.
}

// result is a cek.Value (can be *cek.Con, *cek.VLamAbs, etc.)
```

### Object Pooling

To reduce GC pressure, the machine uses `sync.Pool` for state objects:

```text
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ computePool │     │ returnPool  │     │   donePool  │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
   Compute[T]          Return[T]           Done[T]
```

This reduces allocations during evaluation.

### Cost Model

Every operation has CPU and memory costs:

```text
┌─────────────────────────────────────────────┐
│                  ExBudget                    │
├─────────────────────┬───────────────────────┤
│     CPU (int64)     │   Memory (int64)      │
└─────────────────────┴───────────────────────┘
```

Costs are charged **before** computation to prevent unbounded work.

### Parser Architecture

The parser is a recursive descent parser with:
- Lexer (`syn/lex/`) - Tokenization
- Parser (`syn/parser.go`) - AST construction
- Name interning for memory efficiency

---

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `budget exhausted` | Script ran out of CPU/memory | Increase budget or optimize script |
| `builtin not available` | Using V4 builtin in V3 script | Check version compatibility |
| `open term evaluated` | Variable not in scope | Check De Bruijn conversion |
| `type mismatch` | Wrong argument type to builtin | Verify argument types |

### Build Issues

**`nilaway` not found:**
```sh
go install go.uber.org/nilaway/cmd/nilaway@latest
```

**`golangci-lint` not found:**
```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

**Test timeout:**
```sh
# Run with increased timeout
go test -timeout 10m ./...
```

---

## Getting Help

- **Issues**: Open a [GitHub issue](https://github.com/blinklabs-io/plutigo/issues) for bugs or questions
- **Discord**: Join [Blink Labs Discord](https://discord.gg/5fPRZnX4qW)
- **Documentation**: Check existing issues/PRs for similar work
- **Cardano**: [Plutus Core Specification](https://github.com/IntersectMBO/plutus) for language semantics

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
