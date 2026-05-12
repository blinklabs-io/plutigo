package syn

import (
	"testing"
)

func TestParsePrettyRoundTrip(t *testing.T) {
	// Test that parsing a pretty-printed program gives the same AST
	programs := []string{
		`(program 1.2.0 (con integer 42))`,
		`(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])`,
		`(program 1.2.0 (lam x x))`,
	}

	for _, input := range programs {
		parsed, err := Parse(input)
		if err != nil {
			t.Fatalf("Failed to parse input %q: %v", input, err)
		}

		pretty := Pretty(parsed)

		parsedAgain, err := Parse(pretty)
		if err != nil {
			t.Errorf("Failed to parse pretty output %q: %v", pretty, err)
			continue
		}

		// Check that pretty printing again gives the same result
		prettyAgain := Pretty(parsedAgain)
		if pretty != prettyAgain {
			t.Errorf(
				"Round-trip failed for input %q: first pretty %q, second %q",
				input,
				pretty,
				prettyAgain,
			)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, input := range fuzzProgramSeeds() {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}

		program, err := Parse(input)
		if err != nil {
			return
		}

		_, _ = NameToDeBruijn(program)
	})
}

func FuzzPretty(f *testing.F) {
	for _, prog := range fuzzProgramSeeds() {
		parsed, err := Parse(prog)
		if err != nil {
			f.Fatal("Failed to parse canonical program:", err)
		}
		f.Add(Pretty(parsed))
	}
	f.Fuzz(func(t *testing.T, pretty string) {
		if len(pretty) > 4096 {
			t.Skip()
		}

		parsed, err := Parse(pretty)
		if err != nil {
			return
		}

		prettyAgain := Pretty(parsed)
		parsedAgain, err := Parse(prettyAgain)
		if err != nil {
			t.Fatalf("failed to parse pretty output %q: %v", prettyAgain, err)
		}

		if stable := Pretty(parsedAgain); stable != prettyAgain {
			t.Fatalf("pretty output was not stable:\nfirst:\n%s\nsecond:\n%s", prettyAgain, stable)
		}
	})
}

func fuzzProgramSeeds() []string {
	return []string{
		"(program 1.0.0 (con integer 42))",
		"(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])",
		"(program 1.2.0 (lam x x))",
		"(program 1.2.0 [(lam x x) (con integer 7)])",
		"(program 1.2.0 (force (delay (con integer 1))))",
		"(program 1.2.0 (con bytestring #deadbeef))",
		"(program 1.2.0 (con string \"hello\\nworld\"))",
		"(program 1.2.0 (con string \"nul\\0del\\DEL\"))",
		"(program 1.2.0 (con bool True))",
		"(program 1.2.0 (con unit ()))",
		"(program 1.3.0 (constr 0 (con integer 1) (con bytestring #00)))",
		"(program 1.3.0 (case (constr 0 (con integer 1)) (lam x x)))",
	}
}
