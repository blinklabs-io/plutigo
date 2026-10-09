package syn

import (
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
)

// TestParseRejectsEmbeddedNul checks that a NUL character (U+0000) in the
// program text is not mistaken for the end of the input (issue #455). Each
// input is a valid program with a NUL placed before it, after it, inside it,
// or inside a trailing comment, and both Parse and ParseWithContext must
// return an error instead of accepting the valid part and ignoring the rest.
func TestParseRejectsEmbeddedNul(t *testing.T) {
	context := ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  builtin.VanRossemProtoVersion,
	}
	inputs := map[string]string{
		"nul before program":                  "\x00(program 1.1.0 (con integer 1))",
		"nul after program":                   "(program 1.1.0 (con integer 1))\x00",
		"nul before trailing text":            "(program 1.1.0 (con integer 1))\x00 garbage ((((",
		"nul inside program":                  "(program 1.1.0 (con integer 1)\x00)",
		"nul in comment before trailing text": "(program 1.1.0 (con integer 1)) -- \x00\ngarbage",
	}
	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatalf("Parse accepted %q", input)
			}
			if _, err := ParseWithContext(input, context); err == nil {
				t.Fatalf("ParseWithContext accepted %q", input)
			}
		})
	}
}

// TestParseAcceptsNulInStringAndComment checks the places where a NUL is
// legal and must not cause an error: inside a string constant, where it is
// kept as part of the string value, and inside a comment, which the parser
// skips up to the end of the line.
func TestParseAcceptsNulInStringAndComment(t *testing.T) {
	program, err := Parse("(program 1.1.0 (con string \"a\x00b\"))")
	if err != nil {
		t.Fatalf("Parse rejected NUL in string literal: %v", err)
	}
	constant, ok := program.Term.(*Constant)
	if !ok {
		t.Fatalf("expected constant term, got %T", program.Term)
	}
	str, ok := constant.Con.(*String)
	if !ok || str.Inner != "a\x00b" {
		t.Fatalf("expected string constant \"a\\x00b\", got %#v", constant.Con)
	}
	// A NUL inside a comment is skipped together with the comment.
	if _, err := Parse(
		"(program 1.1.0 (con integer 1)) -- note\x00 here\n",
	); err != nil {
		t.Fatalf("Parse rejected NUL in comment: %v", err)
	}
}
