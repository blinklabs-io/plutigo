// Package builtin defines the builtin function identifiers for Untyped Plutus Core.
//
// # DefaultFunction
//
// [DefaultFunction] is an enumeration of all builtin functions available in UPLC.
// The numeric values correspond to the official Plutus specification and are used
// in FLAT binary encoding.
//
// # Function Categories
//
// Builtins are organized into categories:
//
// Integer operations:
//   - AddInteger, SubtractInteger, MultiplyInteger, DivideInteger
//   - QuotientInteger, RemainderInteger, ModInteger
//   - EqualsInteger, LessThanInteger, LessThanEqualsInteger
//
// ByteString operations:
//   - AppendByteString, ConsByteString, SliceByteString
//   - LengthOfByteString, IndexByteString
//   - EqualsByteString, LessThanByteString, LessThanEqualsByteString
//
// Cryptographic operations:
//   - Sha2_256, Sha3_256, Blake2b_256, Blake2b_224, Keccak_256, Ripemd_160
//   - VerifyEd25519Signature, VerifyEcdsaSecp256k1Signature, VerifySchnorrSecp256k1Signature
//   - BLS12-381 operations (G1, G2, pairing)
//
// String operations:
//   - AppendString, EqualsString, EncodeUtf8, DecodeUtf8
//
// Data operations:
//   - ChooseData, ConstrData, MapData, ListData, IData, BData
//   - UnConstrData, UnMapData, UnListData, UnIData, UnBData
//   - EqualsData, SerialiseData
//
// List operations:
//   - ChooseList, MkCons, HeadList, TailList, NullList
//
// Control flow:
//   - IfThenElse, ChooseUnit, Trace
//
// # Version Availability
//
// Not all builtins are available in all Plutus versions. Check the language
// version before using newer builtins. The cek package handles version
// checking during evaluation.
package builtin
