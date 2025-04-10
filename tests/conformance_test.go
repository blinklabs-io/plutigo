package conformance

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Debug configuration
const (
	debugEnabled = true
	debugLevel   = 2 // 1=basic, 2=verbose, 3=trace
)

func debugLog(level int, format string, args ...interface{}) {
	if debugEnabled && level <= debugLevel {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// ==================== UPLC Term Definitions ====================

type Term interface {
	uplcTerm()
	String() string
}

type Var struct{ DeBruijn int }

func (v Var) uplcTerm()      {}
func (v Var) String() string { return fmt.Sprintf("Var(%d)", v.DeBruijn) }

type Lam struct{ Body Term }

func (l Lam) uplcTerm()      {}
func (l Lam) String() string { return fmt.Sprintf("Lam(%s)", l.Body) }

type App struct{ Fun, Arg Term }

func (a App) uplcTerm()      {}
func (a App) String() string { return fmt.Sprintf("App(%s, %s)", a.Fun, a.Arg) }

type Con struct {
	Type  string
	Value interface{}
}

func (c Con) uplcTerm()      {}
func (c Con) String() string { return fmt.Sprintf("Con(%s, %v)", c.Type, c.Value) }

type Builtin struct {
	Name string
	Cost CpuMem
}

func (b Builtin) uplcTerm()      {}
func (b Builtin) String() string { return fmt.Sprintf("Builtin(%s)", b.Name) }

type Error string

func (e Error) uplcTerm()      {}
func (e Error) String() string { return fmt.Sprintf("Error(%s)", string(e)) }

type CpuMem struct{ Cpu, Mem int64 }

func (c CpuMem) String() string {
	return fmt.Sprintf("(%d, %d)", c.Cpu, c.Mem)
}

type Program struct{ Term Term }

func (p Program) String() string {
	return fmt.Sprintf("Program(%s)", p.Term)
}

// ==================== Built-in Functions ====================

var builtins = map[string]CpuMem{
	"addInteger":         {100, 10},
	"subtractInteger":    {100, 10},
	"multiplyInteger":    {150, 15},
	"divideInteger":      {200, 20},
	"equalsInteger":      {50, 5},
	"lessThanInteger":    {50, 5},
	"ifThenElse":         {20, 2},
	"chooseUnit":         {20, 2},
	"unit":               {10, 1},
	"trace":              {10, 1},
	"succInteger":        {50, 5},
	"appendByteString":   {100, 10},
	"consByteString":     {100, 10},
	"sliceByteString":    {100, 10},
	"lengthOfByteString": {50, 5},
	"force":              {50, 5},
}

// ==================== Improved Parser Implementation ====================

func ParseUPLC(input string) (*Program, error) {
	input = normalizeInput(input)
	debugLog(1, "Normalized input:\n%s", input)

	if !strings.HasPrefix(input, "(program") {
		return nil, fmt.Errorf("program must start with (program")
	}

	// Extract version and body
	versionEnd := strings.Index(input, ")")
	if versionEnd == -1 {
		return nil, fmt.Errorf("missing closing parenthesis for program")
	}

	body := strings.TrimSpace(input[versionEnd+1:])
	if len(body) == 0 {
		return nil, fmt.Errorf("empty program body")
	}

	term, err := parseTerm(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse program body: %w", err)
	}

	return &Program{Term: term}, nil
}

func parseTerm(input string) (Term, error) {
	input = strings.TrimSpace(input)
	debugLog(3, "parseTerm input: %q", input)

	// Handle parenthesized terms
	if strings.HasPrefix(input, "(") && strings.HasSuffix(input, ")") {
		inner := strings.TrimSpace(input[1 : len(input)-1])
		return parseParenthesizedTerm(inner)
	}

	// Handle applications
	if strings.HasPrefix(input, "[") {
		return parseApplication(input)
	}

	// Handle simple terms (variables, etc.)
	return parseSimpleTerm(input)
}

func parseParenthesizedTerm(input string) (Term, error) {
	parts := strings.SplitN(input, " ", 2)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty parenthesized term")
	}

	keyword := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = strings.TrimSpace(parts[1])
	}

	switch keyword {
	case "builtin":
		return parseBuiltin(rest)
	case "force":
		return parseForce(rest)
	case "delay":
		return parseDelay(rest)
	case "lam":
		return parseLambda(rest)
	case "con":
		return parseConstant(rest)
	default:
		return nil, fmt.Errorf("unknown keyword in parenthesized term: %s", keyword)
	}
}

func parseBuiltin(input string) (Builtin, error) {
	name := strings.TrimSpace(input)
	if cost, ok := builtins[name]; ok {
		return Builtin{Name: name, Cost: cost}, nil
	}
	return Builtin{}, fmt.Errorf("unknown builtin: %s", name)
}

func parseForce(input string) (Term, error) {
	term, err := parseTerm(input)
	if err != nil {
		return nil, fmt.Errorf("force parse failed: %w", err)
	}
	return App{Fun: Builtin{Name: "force"}, Arg: term}, nil
}

func parseDelay(input string) (Term, error) {
	term, err := parseTerm(input)
	if err != nil {
		return nil, fmt.Errorf("delay parse failed: %w", err)
	}
	return Lam{Body: term}, nil
}

func parseLambda(input string) (Term, error) {
	// Skip variable name (not used in De Bruijn indexing)
	bodyStart := strings.Index(input, " ")
	if bodyStart == -1 {
		return nil, fmt.Errorf("invalid lambda format")
	}
	body := strings.TrimSpace(input[bodyStart:])

	term, err := parseTerm(body)
	if err != nil {
		return nil, fmt.Errorf("lambda body parse failed: %w", err)
	}
	return Lam{Body: term}, nil
}

func parseConstant(input string) (Term, error) {
	parts := strings.SplitN(input, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid constant format")
	}

	typ := parts[0]
	value := strings.TrimSpace(parts[1])

	switch typ {
	case "integer":
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer: %w", err)
		}
		return Con{Type: "integer", Value: val}, nil
	case "bool":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid bool: %w", err)
		}
		return Con{Type: "bool", Value: val}, nil
	case "unit":
		return Con{Type: "unit", Value: nil}, nil
	default:
		return Con{Type: typ, Value: value}, nil
	}
}

func parseApplication(input string) (Term, error) {
	if !strings.HasPrefix(input, "[") || !strings.HasSuffix(input, "]") {
		return nil, fmt.Errorf("invalid application format")
	}

	inner := strings.TrimSpace(input[1 : len(input)-1])
	if inner == "" {
		return nil, fmt.Errorf("empty application")
	}

	terms, err := splitTerms(inner)
	if err != nil {
		return nil, fmt.Errorf("failed to split application terms: %w", err)
	}

	if len(terms) < 2 {
		return nil, fmt.Errorf("application requires at least 2 terms")
	}

	fun, err := parseTerm(terms[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse function: %w", err)
	}

	var result Term = fun
	for _, argStr := range terms[1:] {
		arg, err := parseTerm(argStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument: %w", err)
		}
		result = App{Fun: result, Arg: arg}
	}

	return result, nil
}

func splitTerms(input string) ([]string, error) {
	var terms []string
	var current strings.Builder
	depth := 0

	for _, r := range input {
		switch r {
		case '(', '[':
			depth++
			current.WriteRune(r)
		case ')', ']':
			depth--
			current.WriteRune(r)
		case ' ':
			if depth == 0 {
				if current.Len() > 0 {
					terms = append(terms, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		terms = append(terms, current.String())
	}

	return terms, nil
}

func parseSimpleTerm(input string) (Term, error) {
	input = strings.TrimSpace(input)
	debugLog(3, "parseSimpleTerm input: %q", input)

	if _, err := strconv.Atoi(input); err == nil {
		index, err := strconv.Atoi(input)
		if err != nil {
			return nil, fmt.Errorf("invalid variable index: %w", err)
		}
		return Var{DeBruijn: index}, nil
	}

	return nil, fmt.Errorf("unsupported term format: %q", input)
}

func normalizeInput(input string) string {
	// Remove comments
	input = regexp.MustCompile(`--.*`).ReplaceAllString(input, "")

	// Normalize whitespace
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	// Trim leading/trailing whitespace
	return strings.TrimSpace(input)
}

// ==================== Test Cases ====================

func TestParseApplication(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple application",
			input: "[ (builtin addInteger) (con integer 1) (con integer 2) ]",
			want:  "App(App(Builtin(addInteger), Con(integer, 1)), Con(integer, 2))",
		},
		{
			name:  "force application",
			input: "[ (builtin force) (builtin f) ]",
			want:  "App(Builtin(force), Builtin(f))",
		},
		{
			name:  "nested application",
			input: "[ [ (builtin addInteger) (con integer 1) ] [ (builtin subtractInteger) (con integer 2) ] ]",
			want:  "App(App(Builtin(addInteger), Con(integer, 1)), App(Builtin(subtractInteger), Con(integer, 2)))",
		},
		{
			name:  "lambda application",
			input: "[ (lam x [ (builtin addInteger) x (con integer 1) ]) (con integer 5) ]",
			want:  "App(Lam(App(App(Builtin(addInteger), Var(0)), Con(integer, 1))), Con(integer, 5))",
		},
		{
			name:  "complex force expression",
			input: "[ (force [ (builtin f) (con integer 1) ]) (con integer 2) ]",
			want:  "App(App(Builtin(force), App(Builtin(f), Con(integer, 1))), Con(integer, 2))",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseApplication(tc.input)
			if err != nil {
				t.Fatalf("parseApplication(%q) failed: %v", tc.input, err)
			}
			if got.String() != tc.want {
				t.Errorf("got %v, want %v", got.String(), tc.want)
			}
		})
	}
}

func TestParseTerm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "builtin",
			input: "(builtin addInteger)",
			want:  "Builtin(addInteger)",
		},
		{
			name:  "force",
			input: "(force (builtin f))",
			want:  "App(Builtin(force), Builtin(f))",
		},
		{
			name:  "delay",
			input: "(delay (builtin f))",
			want:  "Lam(Builtin(f))",
		},
		{
			name:  "lambda",
			input: "(lam x (builtin f))",
			want:  "Lam(Builtin(f))",
		},
		{
			name:  "constant integer",
			input: "(con integer 42)",
			want:  "Con(integer, 42)",
		},
		{
			name:  "constant bool",
			input: "(con bool True)",
			want:  "Con(bool, true)",
		},
		{
			name:  "nested force",
			input: "(force (force (builtin f)))",
			want:  "App(Builtin(force), App(Builtin(force), Builtin(f)))",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTerm(tc.input)
			if err != nil {
				t.Fatalf("parseTerm(%q) failed: %v", tc.input, err)
			}
			if got.String() != tc.want {
				t.Errorf("got %v, want %v", got.String(), tc.want)
			}
		})
	}
}

// ==================== Conformance Test ====================

func TestConformance(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Could not get caller information")
	}

	testRoot := filepath.Join(filepath.Dir(filename), "conformance")
	testRoot = filepath.Clean(testRoot)
	debugLog(1, "Test root directory: %s", testRoot)

	if _, err := os.Stat(testRoot); os.IsNotExist(err) {
		t.Fatalf("Test directory not found: %s", testRoot)
	}

	categories := []string{"builtin", "example", "term"}
	for _, category := range categories {
		categoryDir := filepath.Join(testRoot, category)
		debugLog(1, "Checking category: %s", categoryDir)

		err := filepath.Walk(categoryDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("error accessing path %q: %w", path, err)
			}

			if info.IsDir() || !strings.HasSuffix(path, ".uplc") {
				return nil
			}

			relPath, err := filepath.Rel(testRoot, path)
			if err != nil {
				return fmt.Errorf("could not get relative path: %w", err)
			}
			testName := strings.TrimSuffix(relPath, ".uplc")

			t.Run(testName, func(t *testing.T) {
				debugLog(1, "Running test: %s", testName)

				// Read program file
				programText, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("Failed to read program file: %v", err)
				}
				debugLog(2, "Program:\n%s", programText)

				// Read expected file
				expectedPath := path + ".expected"
				expectedText, err := os.ReadFile(expectedPath)
				if err != nil {
					t.Fatalf("Failed to read expected file: %v", err)
				}
				debugLog(2, "Expected result:\n%s", expectedText)

				// Handle budget file
				budgetPath := path + ".budget.expected"
				var expectedBudget string
				if _, err := os.Stat(budgetPath); err == nil {
					budgetText, err := os.ReadFile(budgetPath)
					if err != nil {
						t.Logf("Failed to read budget file: %v", err)
					} else {
						expectedBudget = strings.TrimSpace(string(budgetText))
					}
				}

				// Parse program
				debugLog(1, "Parsing program...")
				program, err := ParseUPLC(string(programText))
				if err != nil {
					if string(expectedText) == "parse error" {
						debugLog(1, "Parse error expected and encountered")
						return
					}
					t.Fatalf("Parse failed: %v\nInput: %s", err, programText)
				}
				debugLog(2, "Parsed program: %#v", program)

				// Evaluate program
				debugLog(1, "Evaluating program...")
				result, budget := Eval(program, t)
				debugLog(2, "Evaluation result: %s", result)
				debugLog(2, "Remaining budget: %s", budget)

				if errTerm, isErr := result.(Error); isErr {
					if string(expectedText) == "evaluation failure" {
						debugLog(1, "Evaluation failure expected and encountered")
						return
					}
					t.Fatalf("Evaluation failed: %v\nProgram: %s", errTerm, program)
				}

				// Parse expected result
				debugLog(1, "Parsing expected result...")
				expected, err := ParseUPLC(string(expectedText))
				if err != nil {
					t.Fatalf("Failed to parse expected result: %v\nExpected: %s", err, expectedText)
				}
				debugLog(2, "Parsed expected: %s", expected)

				// Compare results
				debugLog(1, "Comparing results...")
				if !alphaEquivalent(result, expected.Term) {
					t.Errorf("Result mismatch\nGot:  %s\nWant: %s\nProgram: %s",
						result, expected.Term, program)
				} else {
					debugLog(1, "Results match")
				}

				// Compare budgets if budget file was found
				if expectedBudget != "" {
					actualBudget := budget.String()
					debugLog(2, "Comparing budgets - Expected: %s, Actual: %s", expectedBudget, actualBudget)
					if actualBudget != expectedBudget {
						t.Errorf("Budget mismatch\nGot:  %s\nWant: %s", actualBudget, expectedBudget)
					} else {
						debugLog(1, "Budgets match")
					}
				}
			})
			return nil
		})

		if err != nil {
			t.Errorf("Error walking %s directory: %v", category, err)
		}
	}
}

// ==================== Evaluation Context ====================

type EvalContext struct {
	Env    []Term
	Budget CpuMem
	Steps  int64
	t      *testing.T
}

func (ctx *EvalContext) log(format string, args ...interface{}) {
	if ctx.t != nil && testing.Verbose() {
		ctx.t.Logf(format, args...)
	}
}

func (ctx *EvalContext) consume(c CpuMem) {
	ctx.log("Consuming budget: %s (remaining: %s)", c, ctx.Budget)
	ctx.Budget.Cpu -= c.Cpu
	ctx.Budget.Mem -= c.Mem
	if ctx.Budget.Cpu < 0 || ctx.Budget.Mem < 0 {
		panic("budget exhausted")
	}
}

func Eval(program *Program, t *testing.T) (Term, CpuMem) {
	ctx := &EvalContext{
		Env:    make([]Term, 0, 10),
		Budget: CpuMem{10_000_000, 10_000_000},
		t:      t,
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Evaluation panic: %v", r)
		}
	}()
	t.Logf("Starting evaluation of program: %s", program)
	result := eval(program.Term, ctx)
	t.Logf("Evaluation completed. Result: %s, Remaining budget: %s", result, ctx.Budget)
	return result, ctx.Budget
}

func eval(term Term, ctx *EvalContext) Term {
	if ctx.Steps > 1_000_000 {
		ctx.log("Evaluation budget exceeded (steps: %d)", ctx.Steps)
		return Error("evaluation budget exceeded")
	}
	ctx.Steps++
	ctx.log("Evaluating term: %s (step %d)", term, ctx.Steps)

	switch t := term.(type) {
	case Var:
		if t.DeBruijn >= len(ctx.Env) {
			ctx.log("Unbound variable %d (env size: %d)", t.DeBruijn, len(ctx.Env))
			return Error(fmt.Sprintf("unbound variable %d", t.DeBruijn))
		}
		val := ctx.Env[t.DeBruijn]
		ctx.log("Variable %d resolved to: %s", t.DeBruijn, val)
		return val

	case Lam:
		ctx.log("Lambda abstraction: %s", t)
		return t

	case App:
		ctx.log("Application: evaluating function %s", t.Fun)
		fun := eval(t.Fun, ctx)
		if isError(fun) {
			return fun
		}

		if b, ok := fun.(Builtin); ok {
			ctx.log("Builtin function detected: %s", b.Name)
			arg := eval(t.Arg, ctx)
			if isError(arg) {
				return arg
			}
			return evalBuiltin(b.Name, arg, ctx)
		}

		arg := eval(t.Arg, ctx)
		if isError(arg) {
			return arg
		}

		switch f := fun.(type) {
		case Lam:
			newCtx := &EvalContext{
				Env:    append([]Term{arg}, ctx.Env...),
				Budget: ctx.Budget,
				Steps:  ctx.Steps,
				t:      ctx.t,
			}
			return eval(f.Body, newCtx)
		default:
			return Error(fmt.Sprintf("non-function application: %T", fun))
		}

	case Con:
		ctx.log("Constant: %s", t)
		return t

	case Builtin:
		ctx.log("Builtin: %s", t)
		return t

	case Error:
		ctx.log("Error: %s", t)
		return t

	default:
		return Error(fmt.Sprintf("unknown term type: %T", term))
	}
}

func evalBuiltin(name string, arg Term, ctx *EvalContext) Term {
	ctx.log("Evaluating builtin %s with argument %s", name, arg)

	cost, exists := builtins[name]
	if !exists {
		return Error(fmt.Sprintf("unknown builtin: %s", name))
	}
	ctx.consume(cost)

	switch name {
	case "addInteger":
		if c, ok := arg.(Con); ok && c.Type == "integer" {
			return Con{Type: "integer", Value: c.Value.(int64) + 1}
		}
	case "equalsInteger":
		if c, ok := arg.(Con); ok && c.Type == "integer" {
			return Con{Type: "bool", Value: c.Value.(int64) == 0}
		}
	case "force":
		// Special handling for force builtin
		if lam, ok := arg.(Lam); ok {
			return lam.Body
		}
		return arg
	default:
		return Error(fmt.Sprintf("unimplemented builtin: %s", name))
	}

	return arg
}

// ==================== Test Helpers ====================

func isError(t Term) bool {
	_, ok := t.(Error)
	return ok
}

func alphaEquivalent(a, b Term) bool {
	switch aTerm := a.(type) {
	case Var:
		if bTerm, ok := b.(Var); ok {
			return aTerm.DeBruijn == bTerm.DeBruijn
		}
	case Lam:
		if bTerm, ok := b.(Lam); ok {
			return alphaEquivalent(aTerm.Body, bTerm.Body)
		}
	case App:
		if bTerm, ok := b.(App); ok {
			return alphaEquivalent(aTerm.Fun, bTerm.Fun) &&
				alphaEquivalent(aTerm.Arg, bTerm.Arg)
		}
	case Con:
		if bTerm, ok := b.(Con); ok {
			if aTerm.Type != bTerm.Type {
				return false
			}
			return aTerm.Value == bTerm.Value
		}
	case Builtin:
		if bTerm, ok := b.(Builtin); ok {
			return aTerm.Name == bTerm.Name
		}
	case Error:
		if bTerm, ok := b.(Error); ok {
			return aTerm == bTerm
		}
	}
	return false
}
