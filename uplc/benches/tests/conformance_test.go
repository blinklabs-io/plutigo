package uplc_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ==================== UPLC Core Implementation ====================

type Term interface{ uplcTerm() }

type Var struct{ DeBruijn int }

func (Var) uplcTerm() {}

type Lam struct{ Body Term }

func (Lam) uplcTerm() {}

type App struct{ Fun, Arg Term }

func (App) uplcTerm() {}

type Con struct {
	Type  string
	Value interface{}
}

func (Con) uplcTerm() {}

type Builtin struct {
	Name string
	Cost CpuMem
}

func (Builtin) uplcTerm() {}

type Error string

func (Error) uplcTerm() {}

type CpuMem struct{ Cpu, Mem int64 }

type Program struct{ Term Term }

// ==================== Evaluation Engine ====================

type EvalContext struct {
	Env    []Term
	Budget CpuMem
	Steps  int64
}

func (ctx *EvalContext) consume(c CpuMem) {
	ctx.Budget.Cpu += c.Cpu
	ctx.Budget.Mem += c.Mem
}

func Eval(program *Program) (Term, CpuMem) {
	ctx := &EvalContext{
		Env:    make([]Term, 0, 10),
		Budget: CpuMem{1, 10}, // Base costs
	}
	return eval(program.Term, ctx), ctx.Budget
}

func eval(term Term, ctx *EvalContext) Term {
	ctx.Steps++
	if ctx.Steps > 1_000_000 {
		return Error("evaluation budget exceeded")
	}

	switch t := term.(type) {
	case Var:
		if t.DeBruijn >= len(ctx.Env) {
			return Error("unbound variable")
		}
		return ctx.Env[t.DeBruijn]

	case Lam:
		return t // Lambdas are values

	case App:
		// Strict evaluation - evaluate function first
		fun := eval(t.Fun, ctx)
		if isError(fun) {
			return fun
		}

		// Then evaluate argument
		arg := eval(t.Arg, ctx)
		if isError(arg) {
			return arg
		}

		// Apply
		switch f := fun.(type) {
		case Lam:
			newCtx := &EvalContext{
				Env:    append([]Term{arg}, ctx.Env...),
				Budget: ctx.Budget,
				Steps:  ctx.Steps,
			}
			return eval(f.Body, newCtx)

		case Builtin:
			ctx.consume(f.Cost)
			return evalBuiltin(f.Name, arg, ctx)

		default:
			return Error("non-function application")
		}

	case Con, Builtin:
		return t // Constants and builtins are values

	case Error:
		return t

	default:
		return Error("unknown term type")
	}
}

// ==================== Built-in Functions ====================

var builtins = map[string]CpuMem{
	"addInteger":      {100, 10},
	"subtractInteger": {100, 10},
	"equalsInteger":   {50, 5},
	"ifThenElse":      {20, 2},
}

func evalBuiltin(name string, arg Term, ctx *EvalContext) Term {
	cost, exists := builtins[name]
	if !exists {
		return Error("unknown builtin: " + name)
	}
	ctx.consume(cost)

	switch name {
	case "addInteger":
		x, y := extractTwoInts(arg)
		return Con{Type: "int", Value: x + y}

	case "subtractInteger":
		x, y := extractTwoInts(arg)
		return Con{Type: "int", Value: x - y}

	case "equalsInteger":
		x, y := extractTwoInts(arg)
		return Con{Type: "bool", Value: x == y}

	case "ifThenElse":
		args := extractThreeArgs(arg)
		if b, ok := args[0].(Con); ok && b.Type == "bool" {
			if b.Value.(bool) {
				return args[1]
			}
			return args[2]
		}
		return Error("invalid if condition")

	default:
		return Error("unimplemented builtin: " + name)
	}
}

// ==================== Test Infrastructure ====================

func TestConformance(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(filename))
	os.Chdir(rootDir)

	testDir := filepath.Join("testdata", "conformance")
	filepath.Walk(testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".uplc") {
			return nil
		}

		testName := strings.TrimSuffix(filepath.Base(path), ".uplc")
		t.Run(testName, func(t *testing.T) {
			// Read test files
			programText := mustReadFile(path)
			expectedText := mustReadFile(path + ".expected")
			budgetText := mustReadFile(path + ".budget")

			// Parse and evaluate
			program, err := ParseUPLC(programText)
			if err != nil {
				if expectedText != "parse error" {
					t.Fatalf("Parse failed: %v", err)
				}
				return
			}

			result, budget := Eval(program)
			if err, isErr := result.(Error); isErr {
				if expectedText != "evaluation failure" {
					t.Fatalf("Evaluation failed: %v", err)
				}
				return
			}

			// Verify results
			expected, _ := ParseUPLC(expectedText)
			if !AlphaEqual(result, expected.Term) {
				t.Errorf("Result mismatch\nGot:  %v\nWant: %v", result, expected.Term)
			}

			actualBudget := fmt.Sprintf("(%d, %d)", budget.Cpu, budget.Mem)
			if actualBudget != strings.TrimSpace(budgetText) {
				t.Errorf("Budget mismatch\nGot:  %s\nWant: %s", actualBudget, budgetText)
			}
		})
		return nil
	})
}

// ==================== Helper Functions ====================

func mustReadFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Failed to read %s: %v", path, err))
	}
	return string(data)
}

func extractTwoInts(arg Term) (int64, int64) {
	app, ok := arg.(App)
	if !ok {
		return 0, 0
	}
	x, ok1 := app.Fun.(Con)
	y, ok2 := app.Arg.(Con)
	if !ok1 || !ok2 || x.Type != "int" || y.Type != "int" {
		return 0, 0
	}
	return x.Value.(int64), y.Value.(int64)
}

func extractThreeArgs(arg Term) []Term {
	app1, ok := arg.(App)
	if !ok {
		return nil
	}
	app2, ok := app1.Fun.(App)
	if !ok {
		return nil
	}
	return []Term{app2.Fun, app2.Arg, app1.Arg}
}

// ==================== Placeholder Implementations ====================

func ParseUPLC(input string) (*Program, error) {
	// TODO: Implement actual parser
	if input == "parse error" {
		return nil, fmt.Errorf("parse error")
	}
	return &Program{Term: Con{Type: "int", Value: int64(42)}}, nil
}

func AlphaEqual(a, b Term) bool {
	// TODO: Implement alpha-equivalence
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func isError(t Term) bool {
	_, ok := t.(Error)
	return ok
}
