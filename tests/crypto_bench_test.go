package tests

import (
	"crypto/ed25519"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	circlbls "github.com/cloudflare/circl/ecc/bls12381"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/ethereum/go-ethereum/crypto"
	sha256 "github.com/minio/sha256-simd"
	"golang.org/x/crypto/blake2b"
)

var benchmarkBLSScalarBytes = []byte{
	1, 2, 3, 4, 5, 6, 7, 8,
	9, 10, 11, 12, 13, 14, 15, 16,
}

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

	benchmarkBLSLibraryComparisons(b, message, dst)
}

func benchmarkBLSLibraryComparisons(b *testing.B, message, dst []byte) {
	otherMessage := []byte("another message")

	b.Run("BLS_HashToG1", func(b *testing.B) {
		b.Run("gnark", func(b *testing.B) {
			for b.Loop() {
				_, _ = bls.HashToG1(message, dst)
			}
		})

		b.Run("circl", func(b *testing.B) {
			for b.Loop() {
				var point circlbls.G1
				point.Hash(message, dst)
			}
		})
	})

	b.Run("BLS_G1_Add", func(b *testing.B) {
		gnarkP1, _ := bls.HashToG1(message, dst)
		gnarkP2, _ := bls.HashToG1(otherMessage, dst)
		var gnarkJac1, gnarkJac2 bls.G1Jac
		gnarkJac1.FromAffine(&gnarkP1)
		gnarkJac2.FromAffine(&gnarkP2)

		var circlP1, circlP2 circlbls.G1
		circlP1.Hash(message, dst)
		circlP2.Hash(otherMessage, dst)

		b.Run("gnark", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				var result bls.G1Jac
				result.Set(&gnarkJac1).AddAssign(&gnarkJac2)
			}
		})

		b.Run("circl", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				var result circlbls.G1
				result.Add(&circlP1, &circlP2)
			}
		})
	})

	b.Run("BLS_G1_ScalarMul", func(b *testing.B) {
		gnarkPoint, _ := bls.HashToG1(message, dst)
		var gnarkJac bls.G1Jac
		gnarkJac.FromAffine(&gnarkPoint)
		gnarkScalar := new(big.Int).SetBytes(benchmarkBLSScalarBytes)

		var circlPoint circlbls.G1
		circlPoint.Hash(message, dst)
		var circlScalar circlbls.Scalar
		circlScalar.SetBytes(benchmarkBLSScalarBytes)

		b.Run("gnark", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				var result bls.G1Jac
				result.ScalarMultiplication(&gnarkJac, gnarkScalar)
			}
		})

		b.Run("circl", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				var result circlbls.G1
				result.ScalarMult(&circlScalar, &circlPoint)
			}
		})
	})

	b.Run("BLS_G2_Add", func(b *testing.B) {
		gnarkP1, _ := bls.HashToG2(message, dst)
		gnarkP2, _ := bls.HashToG2(otherMessage, dst)
		var gnarkJac1, gnarkJac2 bls.G2Jac
		gnarkJac1.FromAffine(&gnarkP1)
		gnarkJac2.FromAffine(&gnarkP2)

		var circlP1, circlP2 circlbls.G2
		circlP1.Hash(message, dst)
		circlP2.Hash(otherMessage, dst)

		b.Run("gnark", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				var result bls.G2Jac
				result.Set(&gnarkJac1).AddAssign(&gnarkJac2)
			}
		})

		b.Run("circl", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				var result circlbls.G2
				result.Add(&circlP1, &circlP2)
			}
		})
	})

	b.Run("BLS_Pairing", func(b *testing.B) {
		gnarkG1, _ := bls.HashToG1(message, dst)
		gnarkG2, _ := bls.HashToG2(otherMessage, dst)
		gnarkG1s := []bls.G1Affine{gnarkG1}
		gnarkG2s := []bls.G2Affine{gnarkG2}

		var circlG1 circlbls.G1
		var circlG2 circlbls.G2
		circlG1.Hash(message, dst)
		circlG2.Hash(otherMessage, dst)

		b.Run("gnark", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				_, _ = bls.Pair(gnarkG1s, gnarkG2s)
			}
		})

		b.Run("circl", func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				_ = circlbls.Pair(&circlG1, &circlG2)
			}
		})
	})
}
