package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestUnwrapStringPreservesEscapes(t *testing.T) {
	value := &Constant{&syn.String{Inner: `\u00A9\n\8712`}}

	out, err := unwrapString[syn.DeBruijn](value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `\u00A9\n\8712` {
		t.Fatalf("got %q want %q", out, `\u00A9\n\8712`)
	}
}
