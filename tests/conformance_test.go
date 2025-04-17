package conformance

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/pkg/machine"
	"github.com/blinklabs-io/plutigo/pkg/syn"
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

// ==================== Test Cases ====================

func TestParse(t *testing.T) {
	t.Run("program", func(t *testing.T) {
		input := "(program 1.1.0 (con integer 1))"

		program, err := syn.Parse(input)

		if err != nil {
			t.Fatalf("syn.Parse(%q) failed: %v", input, err)
		}

		want := syn.NewProgram(
			[3]uint32{1, 1, 0},
			syn.NewSimpleInteger(1),
		)

		if !reflect.DeepEqual(program, want) {
			t.Errorf("got %+v, want %+v", program, want)
		}
	})

}

func TestParseTerm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  syn.Term[syn.Name]
	}{
		{
			name:  "builtin",
			input: "(builtin addInteger)",
			want:  syn.AddInteger(),
		},
		{
			name:  "delay",
			input: "(delay (builtin addInteger))",
			want:  syn.NewDelay(syn.AddInteger()),
		},
		{
			name:  "constant integer",
			input: "(con integer 42)",
			want:  syn.NewSimpleInteger(42),
		},
		{
			name:  "constant bool",
			input: "(con bool True)",
			want:  syn.NewBool(true),
		},
		{
			name:  "simple application",
			input: "[[(builtin addInteger) (con integer 1)] (con integer 2)]",
			want: syn.Intern(
				syn.NewApply(
					syn.NewApply(syn.AddInteger(), syn.NewSimpleInteger(1)),
					syn.NewSimpleInteger(2),
				),
			),
		},
		{
			name:  "force builtin",
			input: "(force (builtin ifThenElse))",
			want: syn.Intern(
				syn.NewForce(syn.IfThenElse()),
			),
		},
		{
			name:  "nested application",
			input: "[ [(builtin addInteger) (con integer 1)] [ (builtin subtractInteger) (con integer 2) (con integer 1) ] ]",
			want: syn.Intern(
				syn.NewApply(
					syn.NewApply(syn.AddInteger(), syn.NewSimpleInteger(1)),
					syn.NewApply(
						syn.NewApply(syn.SubtractInteger(), syn.NewSimpleInteger(2)),
						syn.NewSimpleInteger(1),
					),
				),
			),
		},
		{
			name:  "lambda application",
			input: "[ (lam x [ (builtin addInteger) x (con integer 1) ]) (con integer 5) ]",
			want: syn.Intern(
				syn.NewApply(
					syn.NewLambda(
						syn.NewRawName("x"),
						syn.NewApply(
							syn.NewApply(syn.AddInteger(), syn.NewRawVar("x")),
							syn.NewSimpleInteger(1),
						),
					),
					syn.NewSimpleInteger(5),
				),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := syn.NewParser(tc.input)

			got, err := parser.ParseTerm()

			if err != nil {
				t.Fatalf("parseApplication(%q) failed: %v", tc.input, err)
			}

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
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
				program, err := syn.Parse(string(programText))
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

				mach := machine.NewMachine(200)

				dProgram, err := program.NamedDeBruijn()
				if err != nil {
					t.Fatalf("Failed to convert program to DeBruijn: %v", err)
				}

				result, budget := mach.Run(&dProgram.Term)

				debugLog(2, "Evaluation result: %s", result)
				debugLog(2, "Remaining budget: %s", budget)

				if errTerm, isErr := result.(syn.Error); isErr {
					if string(expectedText) == "evaluation failure" {
						debugLog(1, "Evaluation failure expected and encountered")
						return
					}
					t.Fatalf("Evaluation failed: %v\nProgram: %s", errTerm, program.Term.String())
				}

				// Parse expected result
				debugLog(1, "Parsing expected result...")
				expected, err := syn.Parse(string(expectedText))
				if err != nil {
					t.Fatalf("Failed to parse expected result: %v\nExpected: %s", err, expectedText)
				}
				debugLog(2, "Parsed expected: %s", expected)

				// Compare results
				debugLog(1, "Comparing results...")
				if !alphaEquivalent(result, expected.Term) {
					t.Errorf("Result mismatch\nGot:  %s\nWant: %s\nProgram: %s",
						result, expected.Term, program.Term)
				} else {
					debugLog(1, "Results match")
				}

				// Compare budgets if budget file was found
				if expectedBudget != "" {
					actualBudget := budget.Error()
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

// ==================== Test Helpers ====================

func alphaEquivalent(a syn.Term[syn.NamedDeBruijn], b syn.Term[syn.NamedDeBruijn]) bool {
	switch aTerm := a.(type) {
	case syn.Var[syn.NamedDeBruijn]:
		if bTerm, ok := b.(syn.Var[syn.NamedDeBruijn]); ok {
			return aTerm.Name.Index == bTerm.Name.Index
		}
	case syn.Lambda[syn.NamedDeBruijn]:
		if bTerm, ok := b.(syn.Lambda[syn.NamedDeBruijn]); ok {
			return alphaEquivalent(aTerm.Body, bTerm.Body)
		}
	case syn.Apply[syn.NamedDeBruijn]:
		if bTerm, ok := b.(syn.Apply[syn.NamedDeBruijn]); ok {
			return alphaEquivalent(aTerm.Function, bTerm.Function) &&
				alphaEquivalent(aTerm.Argument, bTerm.Argument)
		}
	case syn.Constant:
		if bTerm, ok := b.(syn.Constant); ok {
			switch aConstant := aTerm.Con.(type) {
			case syn.Integer:
				if bConstant, ok := bTerm.Con.(syn.Integer); ok {
					return aConstant.Inner == bConstant.Inner
				}
			case syn.String:
				if bConstant, ok := bTerm.Con.(syn.String); ok {
					return aConstant.Inner == bConstant.Inner
				}
			case syn.Bool:
				if bConstant, ok := bTerm.Con.(syn.Bool); ok {
					return aConstant.Inner == bConstant.Inner
				}
			}
		}
	case syn.Builtin:
		if bTerm, ok := b.(syn.Builtin); ok {
			return aTerm == bTerm
		}
	case syn.Error:
		if bTerm, ok := b.(syn.Error); ok {
			return aTerm == bTerm
		}
	}

	return false
}
