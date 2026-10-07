// Package lang defines Plutus language versions and their associated
// cost model parameters.
//
// # Language Versions
//
// Plutus has evolved through several versions:
//
//   - V1 (1.0.0) - Alonzo era: Basic functionality
//   - V2 (1.1.0) - Vasil era: Additional crypto builtins
//   - V3 (1.2.0) - Chang+ era: Sums-of-products, bitwise operations
//   - V4 (1.3.0) - Leios era: Array operations, Value builtins
//
// Use [LanguageVersionV1], [LanguageVersionV2], [LanguageVersionV3], [LanguageVersionV4] constants.
//
// # Cost Model Parameters
//
// Each version has specific cost model parameter names used to configure
// the CEK machine's budget tracking. Use [GetParamNamesForVersion] to
// obtain the parameter names for a specific version.
//
// # Parameter Arrays
//
// The cost model parameter arrays ([CostModelParamNamesV1], etc.) list
// all parameters in the order expected by protocol parameter updates.
// These match the official Plutus cost model specification.
//
// # Script Contexts
//
// [ScriptContextV4], [TxInfoV4], [TopTxInfoV4], and the related V4 purpose,
// script info, address, and output types serialize Plutus V4 script context
// values to data.PlutusData. Callers supply already-derived values; ledger
// integrations remain responsible for deriving them from transaction and
// ledger state.
//
// # Version Detection
//
// Programs declare a UPLC term version in the header: (program 1.0.0 ...)
// It is not the Plutus ledger language. Callers supply the ledger language
// and protocol version, which the cek package uses to select builtins and
// costs; the header is only checked for being a legal term version.
package lang
