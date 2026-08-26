package tests

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// ==================== Helper Functions ====================

// isV4BuiltinTest returns true if the test path contains a V4 builtin name
func isV4BuiltinTest(path string) bool {
	v4Builtins := []string{
		"insertCoin",
		"lookupCoin",
		"scaleValue",
		"unionValue",
		"valueContains",
		"valueData",
		"unValueData",
		"lengthOfArray",
		"listToArray",
		"indexArray",
		"bls12_381_G1_multiScalarMul",
		"bls12_381_G2_multiScalarMul",
	}
	for _, b := range v4Builtins {
		if strings.Contains(path, b) {
			return true
		}
	}
	return false
}

// isUnreleasedBuiltinTest returns true if the test path contains an unreleased builtin name
// These builtins are defined but not yet available on mainnet
func isUnreleasedBuiltinTest(path string) bool {
	unreleasedBuiltins := []string{
		"dropList",
	}
	for _, b := range unreleasedBuiltins {
		if strings.Contains(path, b) {
			return true
		}
	}
	return false
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

type conformanceFormat uint8

const (
	conformanceText conformanceFormat = iota
	conformanceFlat
)

type conformanceFailure uint8

const (
	conformanceSuccess conformanceFailure = iota
	conformanceParseFailure
	conformanceEvaluationFailure
)

type conformanceCase struct {
	inputPath    string
	expectedPath string
	budgetPath   string
	format       conformanceFormat
}

func discoverConformanceCases(root string) ([]conformanceCase, error) {
	cases := make([]conformanceCase, 0)
	err := filepath.WalkDir(
		root,
		func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("access %q: %w", path, err)
			}
			if entry.IsDir() {
				return nil
			}

			format, ok := conformanceFormatForPath(path)
			if !ok {
				return nil
			}

			budgetPath, err := conformanceBudgetPath(path)
			if err != nil {
				return err
			}
			expectedPath := path + ".expected"
			if _, err := os.Stat(expectedPath); err != nil {
				return fmt.Errorf(
					"expected result for %q: %w",
					path,
					err,
				)
			}

			cases = append(cases, conformanceCase{
				inputPath:    path,
				expectedPath: expectedPath,
				budgetPath:   budgetPath,
				format:       format,
			})
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("no conformance inputs found under %q", root)
	}
	return cases, nil
}

func conformanceFormatForPath(path string) (conformanceFormat, bool) {
	switch filepath.Ext(path) {
	case ".uplc":
		return conformanceText, true
	case ".flat":
		return conformanceFlat, true
	default:
		return conformanceText, false
	}
}

func conformanceBudgetPath(inputPath string) (string, error) {
	ext := filepath.Ext(inputPath)
	canonicalPath := strings.TrimSuffix(inputPath, ext) + ".budget.expected"
	if _, err := os.Stat(canonicalPath); err == nil {
		return canonicalPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat budget %q: %w", canonicalPath, err)
	}

	legacyPath := inputPath + ".budget.expected"
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat budget %q: %w", legacyPath, err)
	}

	return "", fmt.Errorf(
		"budget result for %q not found at %q or %q",
		inputPath,
		canonicalPath,
		legacyPath,
	)
}

func conformanceExpectedFailure(contents []byte) conformanceFailure {
	switch strings.TrimSpace(string(contents)) {
	case "parse error", "parse/decode error":
		return conformanceParseFailure
	case "evaluation failure":
		return conformanceEvaluationFailure
	default:
		return conformanceSuccess
	}
}

func decodeConformanceProgram(
	format conformanceFormat,
	contents []byte,
) (*syn.Program[syn.DeBruijn], conformanceFailure, error) {
	if format == conformanceFlat {
		program, err := syn.Decode[syn.DeBruijn](contents)
		return program, conformanceParseFailure, err
	}

	program, err := syn.Parse(string(contents))
	if err != nil {
		return nil, conformanceParseFailure, err
	}
	dProgram, err := syn.NameToDeBruijn(program)
	return dProgram, conformanceEvaluationFailure, err
}

func conformanceLanguageForPath(
	path string,
) (lang.LanguageVersion, uint) {
	if isV4BuiltinTest(path) {
		return lang.LanguageVersionV4, 12
	}
	return lang.LanguageVersionV3, 11
}

func conformanceFirstByteMismatch(got, want []byte) int {
	sharedLen := min(len(got), len(want))
	for i := range sharedLen {
		if got[i] != want[i] {
			return i
		}
	}
	return sharedLen
}

func runConformanceCase(testCase conformanceCase) error {
	input, err := os.ReadFile(testCase.inputPath)
	if err != nil {
		return fmt.Errorf("read program: %w", err)
	}
	expected, err := os.ReadFile(testCase.expectedPath)
	if err != nil {
		return fmt.Errorf("read expected result: %w", err)
	}
	budget, err := os.ReadFile(testCase.budgetPath)
	if err != nil {
		return fmt.Errorf("read expected budget: %w", err)
	}

	expectedFailure := conformanceExpectedFailure(expected)
	budgetFailure := conformanceExpectedFailure(budget)
	program, failureStage, err := decodeConformanceProgram(
		testCase.format,
		input,
	)
	if err != nil {
		if expectedFailure == failureStage && budgetFailure == failureStage {
			return nil
		}
		return fmt.Errorf("load program: %w", err)
	}
	if expectedFailure == conformanceParseFailure {
		return fmt.Errorf("expected parse/decode failure, program loaded")
	}
	if budgetFailure == conformanceParseFailure {
		return fmt.Errorf("budget expects parse/decode failure, program loaded")
	}

	plutusVersion, protocolVersion := conformanceLanguageForPath(
		testCase.inputPath,
	)
	initialBudget := cek.ExBudget{
		Mem: math.MaxInt64,
		Cpu: math.MaxInt64,
	}
	machine := cek.NewMachine[syn.DeBruijn](
		plutusVersion,
		200,
		cek.NewDefaultEvalContext(
			plutusVersion,
			cek.ProtoVersion{Major: protocolVersion},
		),
	)
	machine.ExBudget = initialBudget

	result, err := machine.Run(program.Term)
	if err != nil {
		if expectedFailure == conformanceEvaluationFailure &&
			budgetFailure == conformanceEvaluationFailure {
			return nil
		}
		return fmt.Errorf("evaluate program: %w", err)
	}
	if expectedFailure == conformanceEvaluationFailure {
		return fmt.Errorf("expected evaluation failure, evaluation succeeded")
	}
	if budgetFailure == conformanceEvaluationFailure {
		return fmt.Errorf("budget expects evaluation failure, evaluation succeeded")
	}

	if testCase.format == conformanceFlat {
		encodedResult, err := syn.Encode(&syn.Program[syn.DeBruijn]{
			Version: program.Version,
			Term:    result,
		})
		if err != nil {
			return fmt.Errorf("encode result: %w", err)
		}
		if !bytes.Equal(encodedResult, expected) {
			return fmt.Errorf(
				"result mismatch at byte %d: got %d bytes, want %d",
				conformanceFirstByteMismatch(encodedResult, expected),
				len(encodedResult),
				len(expected),
			)
		}
	} else {
		expectedProgram, _, err := decodeConformanceProgram(
			conformanceText,
			expected,
		)
		if err != nil {
			return fmt.Errorf("load expected result: %w", err)
		}
		prettyResult := syn.PrettyTerm[syn.DeBruijn](result)
		prettyExpected := syn.PrettyTerm[syn.DeBruijn](
			expectedProgram.Term,
		)
		if prettyResult != prettyExpected {
			return fmt.Errorf(
				"result mismatch: got %s, want %s",
				prettyResult,
				prettyExpected,
			)
		}
	}

	consumedBudget := initialBudget.Sub(&machine.ExBudget)
	formattedBudget := fmt.Sprintf(
		"({cpu: %d\n| mem: %d})",
		consumedBudget.Cpu,
		consumedBudget.Mem,
	)
	expectedBudget := strings.TrimSpace(string(budget))
	if formattedBudget != expectedBudget {
		return fmt.Errorf(
			"budget mismatch: got %s, want %s",
			formattedBudget,
			expectedBudget,
		)
	}
	return nil
}

func TestConformance(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Could not get caller information")
	}

	testRoot := filepath.Join(filepath.Dir(filename), "conformance")
	testRoot = filepath.Clean(testRoot)
	cases, err := discoverConformanceCases(testRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range cases {
		relPath, err := filepath.Rel(testRoot, testCase.inputPath)
		if err != nil {
			t.Fatalf("could not get relative path: %v", err)
		}
		t.Run(filepath.ToSlash(relPath), func(t *testing.T) {
			if isUnreleasedBuiltinTest(testCase.inputPath) {
				t.Skip("Skipping unreleased builtin test")
			}
			if err := runConformanceCase(testCase); err != nil {
				t.Fatal(err)
			}
		})
	}
}
