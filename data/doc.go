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

// Package data provides CBOR encoding and decoding for Plutus Data,
// the serialization format used by Plutus smart contracts on Cardano.
//
// # Key Types
//
// All types implement the [PlutusData] interface:
//
//   - [Constr] - Constructor with tag and fields (most common)
//   - [Map] - Key-value pairs
//   - [List] - Ordered list of data
//   - [Integer] - Arbitrary-precision integer
//   - [ByteString] - Raw bytes
//
// # Encoding and Decoding
//
//	// Decode CBOR bytes to PlutusData
//	plutusData, err := data.Decode(cborBytes)
//
//	// Encode PlutusData to CBOR bytes
//	cborBytes, err := data.Encode(plutusData)
//
// # Constructor Tags
//
// Constr uses special CBOR tags for efficient encoding:
//
//   - Tags 121-127 for constructors 0-6
//   - Tag 1280 + n for constructors 7-127
//   - Tag 102 with explicit index for larger tags
//
// # Usage in UPLC
//
// PlutusData values appear in UPLC programs as constants and are
// manipulated by builtin functions like constrData, unConstrData,
// mapData, listData, iData, and bData.
package data
