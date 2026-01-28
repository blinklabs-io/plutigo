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
