package cek

import (
	"encoding/hex"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// keccak256Builtin evaluates the keccak_256 builtin over input and returns the
// resulting byte string. Tests go through the builtin rather than calling the
// hash library directly, so a regression in the builtin's wiring -- wrong
// argument handling, wrong constant type, truncated result -- is caught and not
// just a change of hashing implementation.
func keccak256Builtin(t *testing.T, input []byte) []byte {
	t.Helper()

	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Keccak_256,
		ArgCount: 0,
		Forces:   0,
	}
	b = b.ApplyArg(&Constant{&syn.ByteString{Inner: input}})

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}
	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}
	bs, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}
	return bs.Inner
}

// TestKeccak256KnownVectors pins published Keccak-256 vectors so the builtin
// stays correct independently of the hashing library behind it.
//
// Plutus' keccak_256 is legacy Keccak (0x01 padding), not NIST SHA3 (0x06
// padding); the two produce entirely different digests for the same input, so
// these vectors also guard against the builtin being switched to stdlib
// crypto/sha3 by mistake.
//
// Provenance: before go-ethereum was removed, x/crypto/sha3.NewLegacyKeccak256
// was verified byte-for-byte identical to crypto.Keccak256Hash across 17
// boundary sizes (including the 136-byte Keccak rate boundary) and 20,000
// randomized inputs. These externally published vectors are the permanent
// guard.
func TestKeccak256KnownVectors(t *testing.T) {
	vectors := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
		},
		{
			name: "abc",
			in:   "abc",
			want: "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45",
		},
		{
			name: "testing",
			in:   "testing",
			want: "5f16f4c7f149ac4f9510d9cf8cf384038ad348b3bcdc01915f95de12df9d1b02",
		},
	}
	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			got := keccak256Builtin(t, []byte(v.in))
			if len(got) != 32 {
				t.Fatalf("digest is %d bytes, want 32", len(got))
			}
			if hex := hex.EncodeToString(got); hex != v.want {
				t.Errorf("keccak256(%q) = %s, want %s", v.in, hex, v.want)
			}
		})
	}
}

// TestKeccak256BuiltinSpansRateBoundary checks inputs either side of Keccak's
// 136-byte rate, where an implementation that mishandles block padding or
// buffering would go wrong, and confirms the digest is always 32 bytes and
// input-dependent.
func TestKeccak256BuiltinSpansRateBoundary(t *testing.T) {
	seen := make(map[string]int, 8)
	for _, n := range []int{0, 1, 135, 136, 137, 271, 272, 273} {
		in := make([]byte, n)
		for i := range in {
			in[i] = byte(i)
		}
		got := keccak256Builtin(t, in)
		if len(got) != 32 {
			t.Errorf("input %d bytes: digest is %d bytes, want 32", n, len(got))
			continue
		}
		if prev, dup := seen[string(got)]; dup {
			t.Errorf(
				"inputs of %d and %d bytes produced the same digest %s",
				prev, n, hex.EncodeToString(got),
			)
			continue
		}
		seen[string(got)] = n
	}
}
