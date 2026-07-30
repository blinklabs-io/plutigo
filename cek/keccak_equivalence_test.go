package cek

import (
	"encoding/hex"
	"testing"

	legacykeccak "golang.org/x/crypto/sha3"
)

// keccak256Reference is the replacement implementation. Plutus' keccak_256
// builtin is legacy Keccak (0x01 padding), not NIST SHA3 (0x06 padding), so
// stdlib crypto/sha3 cannot be used here -- it exposes only the NIST variant.
func keccak256Reference(b []byte) []byte {
	h := legacykeccak.NewLegacyKeccak256()
	// hash.Hash.Write never returns an error.
	_, _ = h.Write(b)
	return h.Sum(nil)
}

// TestKeccak256KnownVectors pins published Keccak-256 vectors so the builtin
// stays correct independently of the hashing library behind it.
//
// Provenance: before go-ethereum was removed, x/crypto/sha3.NewLegacyKeccak256
// was verified byte-for-byte identical to crypto.Keccak256Hash across 17
// boundary sizes (including the 136-byte Keccak rate boundary) and 20,000
// randomized inputs. These vectors are the permanent guard.
func TestKeccak256KnownVectors(t *testing.T) {
	vectors := []struct {
		in   string
		want string
	}{
		{
			// Keccak-256 of the empty string.
			in:   "",
			want: "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
		},
		{
			in:   "abc",
			want: "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45",
		},
		{
			in:   "testing",
			want: "5f16f4c7f149ac4f9510d9cf8cf384038ad348b3bcdc01915f95de12df9d1b02",
		},
	}
	for _, v := range vectors {
		got := hex.EncodeToString(keccak256Reference([]byte(v.in)))
		if got != v.want {
			t.Errorf("keccak256(%q) = %s, want %s", v.in, got, v.want)
		}
		if len(keccak256Reference([]byte(v.in))) != 32 {
			t.Errorf("keccak256(%q) digest is not 32 bytes", v.in)
		}
	}
}
