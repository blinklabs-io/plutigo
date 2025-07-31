package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

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
						syn.NewApply(
							syn.SubtractInteger(),
							syn.NewSimpleInteger(2),
						),
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

	if _, err := os.Stat(testRoot); os.IsNotExist(err) {
		t.Fatalf("Test directory not found: %s", testRoot)
	}

	categories := []string{"builtin", "example", "term"}
	for _, category := range categories {
		categoryDir := filepath.Join(testRoot, category)

		err := filepath.Walk(
			categoryDir,
			func(path string, info os.FileInfo, err error) error {
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
					// Read program file
					programText, err := os.ReadFile(path)
					if err != nil {
						t.Fatalf("Failed to read program file: %v", err)
					}

					// Read expected file
					expectedPath := path + ".expected"
					expectedText, err := os.ReadFile(expectedPath)
					if err != nil {
						t.Fatalf("Failed to read expected file: %v", err)
					}

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

					program, err := syn.Parse(string(programText))
					if err != nil {
						if string(expectedText) == "parse error" {
							return
						}

						t.Fatalf(
							"Parse failed: %v\nInput: %s",
							err,
							programText,
						)
					}

					// Convert to NamedDeBruijn

					dProgram, err := syn.NameToDeBruijn(program)
					if err != nil {
						if string(expectedText) == "evaluation failure" {
							return
						}

						t.Fatalf(
							"Failed to convert program to DeBruijn: %v",
							err,
						)
					}

					// Evaluate program

					machine := cek.NewMachine[syn.DeBruijn](200)

					result, err := machine.Run(dProgram.Term)
					if err != nil {
						if string(expectedText) == "evaluation failure" {
							return
						}

						t.Fatalf("Failed to evaluate program: %v", err)
					}

					// Parse expected result

					expected, err := syn.Parse(string(expectedText))
					if err != nil {
						t.Fatalf(
							"Failed to parse expected result: %v\nExpected: %s",
							err,
							expectedText,
						)
					}

					dExpected, err := syn.NameToDeBruijn(expected)
					if err != nil {
						t.Fatalf(
							"Failed to convert program to DeBruijn: %v",
							err,
						)
					}

					// Compare results
					prettyResult := syn.PrettyTerm[syn.DeBruijn](result)
					prettyExpected := syn.PrettyTerm[syn.DeBruijn](
						dExpected.Term,
					)

					original := syn.Pretty[syn.Name](program)

					if prettyResult != prettyExpected {
						t.Errorf(
							"Result mismatch\nGot:\n%s\nWant:\n%s\nProgram:\n%s",
							prettyResult,
							prettyExpected,
							original,
						)
					}

					// Compare budgets if budget file was found
					if expectedBudget != "" {
						consumedBudget := cek.DefaultExBudget.Sub(
							&machine.ExBudget,
						)
						fmtBudget := fmt.Sprintf(
							"({cpu: %d\n| mem: %d})",
							consumedBudget.Cpu,
							consumedBudget.Mem,
						)

						if fmtBudget != expectedBudget {
							t.Errorf(
								"Budget mismatch\nGot:\n%s\nWant:\n%s",
								fmtBudget,
								expectedBudget,
							)
						}
					}
				})

				return nil
			},
		)
		if err != nil {
			t.Errorf("Error walking %s directory: %v", category, err)
		}
	}
}
