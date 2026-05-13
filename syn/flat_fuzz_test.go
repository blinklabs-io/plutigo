package syn

import (
	"bytes"
	"testing"
)

func FuzzDecodeFlatDeBruijn(f *testing.F) {
	for _, input := range fuzzFlatProgramSeeds() {
		program, err := Parse(input)
		if err != nil {
			f.Fatalf("failed to parse seed %q: %v", input, err)
		}
		dbProgram, err := NameToDeBruijn(program)
		if err != nil {
			f.Fatalf("failed to convert seed %q to De Bruijn: %v", input, err)
		}
		encoded, err := Encode(dbProgram)
		if err != nil {
			f.Fatalf("failed to encode seed %q: %v", input, err)
		}
		f.Add(encoded)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 4096 {
			t.Skip()
		}

		genericProgram, genericErr := Decode[DeBruijn](input)
		fastProgram, fastErr := DecodeDeBruijn(input)
		if (genericErr != nil) != (fastErr != nil) {
			t.Fatalf("Decode and DecodeDeBruijn error mismatch: generic=%v fast=%v", genericErr, fastErr)
		}
		if genericErr != nil {
			return
		}

		genericEncoded, err := Encode(genericProgram)
		if err != nil {
			t.Fatalf("failed to re-encode generic decoded program: %v", err)
		}
		fastEncoded, err := Encode(fastProgram)
		if err != nil {
			t.Fatalf("failed to re-encode fast decoded program: %v", err)
		}
		if !bytes.Equal(genericEncoded, fastEncoded) {
			t.Fatalf("Decode and DecodeDeBruijn re-encoded to different bytes")
		}

		decodedAgain, err := DecodeDeBruijn(fastEncoded)
		if err != nil {
			t.Fatalf("failed to decode re-encoded program: %v", err)
		}
		encodedAgain, err := Encode(decodedAgain)
		if err != nil {
			t.Fatalf("failed to encode decoded re-encoded program: %v", err)
		}
		if !bytes.Equal(fastEncoded, encodedAgain) {
			t.Fatalf("FLAT encode/decode round-trip was not stable")
		}
	})
}

func fuzzFlatProgramSeeds() []string {
	return []string{
		"(program 1.0.0 (con integer 42))",
		"(program 1.2.0 [(builtin addInteger) (con integer 1) (con integer 2)])",
		"(program 1.2.0 (force (delay (con integer 1))))",
		"(program 1.2.0 (con bytestring #deadbeef))",
		"(program 1.2.0 (con string \"hello\\nworld\"))",
		"(program 1.2.0 (con bool True))",
		"(program 1.2.0 (constr 0 (con integer 1) (con bytestring #00)))",
		"(program 1.2.0 (case (constr 0 (con integer 1)) (lam x x)))",
	}
}
