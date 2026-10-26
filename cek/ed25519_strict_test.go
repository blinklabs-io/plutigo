// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cek

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"testing"
)

func mustHex32(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		t.Fatalf("bad hex %q", s)
	}
	return b
}

func ed25519TestCat(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func TestEd25519VerifyStrictRejects(t *testing.T) {
	zero32 := make([]byte, 32)
	identity := append([]byte{0x01}, make([]byte, 31)...)
	nonCanonIdentity := mustHex32(t,
		"eeffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f")
	order8a := mustHex32(t,
		"26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05")
	order8b := mustHex32(t,
		"c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a")

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("abc")
	sig := ed25519.Sign(priv, msg)
	if !ed25519VerifyStrict(pub, msg, sig) {
		t.Fatal("valid signature rejected")
	}
	// S + L
	l := new(big.Int).SetBytes(reverseBytes(ed25519GroupOrder[:]))
	s := new(big.Int).SetBytes(reverseBytes(sig[32:]))
	s.Add(s, l)
	sPlusL := make([]byte, 32)
	sb := s.Bytes()
	for i := range sb {
		sPlusL[i] = sb[len(sb)-1-i]
	}
	sigSL := ed25519TestCat(sig[:32], sPlusL)

	type tc struct {
		name     string
		pk, sig  []byte
		messages []string
	}
	both := []string{"", "abc"}
	tests := []tc{
		{"identity pk and R", identity, ed25519TestCat(identity, zero32), both},
		{"non-canonical identity", nonCanonIdentity,
			ed25519TestCat(nonCanonIdentity, zero32), both},
		{"order-4 pk, identity R", zero32,
			ed25519TestCat(identity, zero32), both},
		{"S+L", pub, sigSL, []string{"abc"}},
		{"order8a pk", order8a, ed25519TestCat(sig[:32], sig[32:]), both},
		{"order8b pk", order8b, ed25519TestCat(sig[:32], sig[32:]), both},
		{"order8a R", pub, ed25519TestCat(order8a, zero32), both},
		{"order8b R", pub, ed25519TestCat(order8b, zero32), both},
		{"order8a pk sign bit", func() []byte {
			b := append([]byte{}, order8a...)
			b[31] ^= 0x80
			return b
		}(), ed25519TestCat(identity, zero32), both},
	}
	for _, tt := range tests {
		for _, m := range tt.messages {
			t.Run(tt.name+"/"+m, func(t *testing.T) {
				if ed25519VerifyStrict(tt.pk, []byte(m), tt.sig) {
					t.Fatal("expected rejection")
				}
			})
		}
	}
}

func reverseBytes(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}

func TestEd25519VerifyStrictAcceptsRFC8032(t *testing.T) {
	tests := []struct{ name, pk, msg, sig string }{
		{"test1",
			"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
			"",
			"e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e06522490155" +
				"5fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b"},
		{"test2",
			"3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c",
			"72",
			"92a009a9f0d4cab8720e820b5f642540a2b27b5416503f8fb3762223ebdb69da" +
				"085ac1e43e15996e458f3613d0f11d8c387b2eaeb4302aeeb00d291612bb0c00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pk, _ := hex.DecodeString(tt.pk)
			msg, _ := hex.DecodeString(tt.msg)
			sig, _ := hex.DecodeString(tt.sig)
			if !ed25519VerifyStrict(pk, msg, sig) {
				t.Fatal("expected acceptance")
			}
		})
	}
}

func TestEd25519VerifyStrictDifferential(t *testing.T) {
	for i := 0; i < 500; i++ {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		msg := make([]byte, i%97)
		_, _ = rand.Read(msg)
		sig := ed25519.Sign(priv, msg)
		if !ed25519VerifyStrict(pub, msg, sig) ||
			ed25519VerifyStrict(pub, msg, sig) != ed25519.Verify(pub, msg, sig) {
			t.Fatalf("mismatch on valid signature %d", i)
		}
		bad := append([]byte{}, sig...)
		bad[int(bad[0])%64] ^= 1
		if ed25519VerifyStrict(pub, msg, bad) {
			t.Fatalf("corrupted signature accepted %d", i)
		}
	}
}
