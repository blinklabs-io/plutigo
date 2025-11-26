package tests

import (
	"crypto/ed25519"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/ethereum/go-ethereum/crypto"
	sha256 "github.com/minio/sha256-simd"
	"golang.org/x/crypto/blake2b"
)

func BenchmarkDirectCrypto(b *testing.B) {
	message := []byte("test message for hashing and signing")
	hash := make([]byte, 32)
	dst := []byte("BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_")

	b.Run("SHA256_minio", func(b *testing.B) {
		for b.Loop() {
			sha256.Sum256(message)
		}
	})

	b.Run("Keccak_256", func(b *testing.B) {
		for b.Loop() {
			crypto.Keccak256(message)
		}
	})

	b.Run("Blake2b_256", func(b *testing.B) {
		for b.Loop() {
			blake2b.Sum256(message)
		}
	})

	b.Run("Keccak_256_Hash", func(b *testing.B) {
		for b.Loop() {
			crypto.Keccak256Hash(message)
		}
	})

	b.Run("Ed25519_Verify", func(b *testing.B) {
		// Use a valid key for testing
		pub, priv, _ := ed25519.GenerateKey(nil)
		sig := ed25519.Sign(priv, message)
		b.ResetTimer()
		for b.Loop() {
			ed25519.Verify(pub, message, sig)
		}
	})

	b.Run("ECDSA_Verify", func(b *testing.B) {
		priv, _ := btcec.NewPrivateKey()
		sig := ecdsa.SignCompact(priv, hash, false)
		b.ResetTimer()
		for b.Loop() {
			ecdsa.RecoverCompact(sig, hash)
		}
	})

	b.Run("BLS_HashToG1", func(b *testing.B) {
		for b.Loop() {
			bls.HashToG1(message, dst)
		}
	})
}
