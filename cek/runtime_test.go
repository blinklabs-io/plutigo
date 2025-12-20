package cek

import (
	"testing"
)

func TestProcessEscapeSequences_DEL(t *testing.T) {
	out, err := processEscapeSequences("Hello\\DELWorld")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Hello" + string(rune(127)) + "World"
	if out != expected {
		t.Fatalf("got %q want %q", out, expected)
	}
}

func TestProcessEscapeSequences_Numeric(t *testing.T) {
	out, err := processEscapeSequences("\\8712")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != string(rune(8712)) {
		t.Fatalf("got %q want %q", out, string(rune(8712)))
	}
}

func TestProcessEscapeSequences_Unicode(t *testing.T) {
	out, err := processEscapeSequences("\\u00A9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "©" {
		t.Fatalf("got %q want %q", out, "©")
	}
}

func TestProcessEscapeSequences_LiteralBackslashPreserved(t *testing.T) {
	out, err := processEscapeSequences("\\\\8712")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "\\8712" {
		t.Fatalf("got %q want %q", out, "\\8712")
	}
}
