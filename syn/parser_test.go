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
	f.Add("(program 1.2.0 (con integer 42))")
	f.Add(
		"(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])",
	)
	f.Fuzz(func(t *testing.T, input string) {
		// Just try to parse, don't care about result
		Parse(input)
	})
}

func FuzzPretty(f *testing.F) {
	testPrograms := []string{
		"(program 1.2.0 (con integer 42))",
		"(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])",
	}
	for _, prog := range testPrograms {
		parsed, err := Parse(prog)
		if err != nil {
			f.Fatal("Failed to parse canonical program:", err)
		}
		f.Add(Pretty(parsed))
	}
	f.Fuzz(func(t *testing.T, pretty string) {
		// Try to parse the pretty output
		Parse(pretty)
	})
}
