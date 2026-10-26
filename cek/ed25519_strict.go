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
	"crypto/ed25519"
)

// The Plutus reference implementation verifies Ed25519 signatures through
// cardano-crypto-class, which calls libsodium's
// crypto_sign_ed25519_verify_detached. That routine is stricter than
// crypto/ed25519.Verify (for example, the latter accepts the identity public
// key with an all-zero signature for any message). The checks below mirror
// libsodium's, in order, and need no curve arithmetic. They match the strict
// criteria used by gouroboros' internal/ed25519strict package.

// ed25519SmallOrderBlocklist is libsodium's list of encodings of small-order
// points (and non-canonical aliases). Comparison ignores the sign bit of the
// final byte, which covers the sign-bit variants.
var ed25519SmallOrderBlocklist = [7][32]byte{
	// y = 0 (order 4)
	{},
	// y = 1 (identity)
	{0x01},
	// order 8
	{
		0x26, 0xe8, 0x95, 0x8f, 0xc2, 0xb2, 0x27, 0xb0,
		0x45, 0xc3, 0xf4, 0x89, 0xf2, 0xef, 0x98, 0xf0,
		0xd5, 0xdf, 0xac, 0x05, 0xd3, 0xc6, 0x33, 0x39,
		0xb1, 0x38, 0x02, 0x88, 0x6d, 0x53, 0xfc, 0x05,
	},
	// order 8
	{
		0xc7, 0x17, 0x6a, 0x70, 0x3d, 0x4d, 0xd8, 0x4f,
		0xba, 0x3c, 0x0b, 0x76, 0x0d, 0x10, 0x67, 0x0f,
		0x2a, 0x20, 0x53, 0xfa, 0x2c, 0x39, 0xcc, 0xc6,
		0x4e, 0xc7, 0xfd, 0x77, 0x92, 0xac, 0x03, 0x7a,
	},
	// y = p-1 (order 2)
	{
		0xec, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f,
	},
	// y = p (non-canonical y = 0)
	{
		0xed, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f,
	},
	// y = p+1 (non-canonical identity)
	{
		0xee, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f,
	},
}

// ed25519GroupOrder is the prime order L of the base point subgroup,
// little-endian.
var ed25519GroupOrder = [32]byte{
	0xed, 0xd3, 0xf5, 0x5c, 0x1a, 0x63, 0x12, 0x58,
	0xd6, 0x9c, 0xf7, 0xa2, 0xde, 0xf9, 0xde, 0x14,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10,
}

// ed25519HasSmallOrder reports whether the 32-byte point encoding matches
// the libsodium small-order blocklist, ignoring the sign bit.
func ed25519HasSmallOrder(p []byte) bool {
	for i := range ed25519SmallOrderBlocklist {
		entry := &ed25519SmallOrderBlocklist[i]
		var diff byte
		for j := 0; j < 31; j++ {
			diff |= p[j] ^ entry[j]
		}
		diff |= (p[31] & 0x7f) ^ entry[31]
		if diff == 0 {
			return true
		}
	}
	return false
}

// ed25519IsCanonicalY reports whether the encoded y coordinate (sign bit
// ignored) is strictly less than p = 2^255 - 19.
func ed25519IsCanonicalY(p []byte) bool {
	if p[31]&0x7f != 0x7f {
		return true
	}
	for j := 1; j < 31; j++ {
		if p[j] != 0xff {
			return true
		}
	}
	return p[0] < 0xed
}

// ed25519ScalarIsCanonical reports whether the little-endian scalar s < L.
func ed25519ScalarIsCanonical(s []byte) bool {
	for i := 31; i >= 0; i-- {
		if s[i] < ed25519GroupOrder[i] {
			return true
		}
		if s[i] > ed25519GroupOrder[i] {
			return false
		}
	}
	return false // s == L
}

// ed25519VerifyStrict verifies sig over msg for publicKey using
// libsodium's criteria. Lengths must already be validated (32 and 64).
func ed25519VerifyStrict(publicKey, msg, sig []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize ||
		len(sig) != ed25519.SignatureSize {
		return false
	}
	if !ed25519ScalarIsCanonical(sig[32:]) ||
		ed25519HasSmallOrder(sig[:32]) ||
		!ed25519IsCanonicalY(publicKey) ||
		ed25519HasSmallOrder(publicKey) {
		return false
	}
	// crypto/ed25519 rejects undecodable keys and compares the re-encoded
	// R' byte-wise (cofactorless), matching libsodium's final check.
	return ed25519.Verify(publicKey, msg, sig)
}
