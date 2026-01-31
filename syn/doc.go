// Package syn provides the syntax layer for Untyped Plutus Core (UPLC),
// including parsing, AST definitions, and serialization.
//
// # Key Types
//
//   - [Term] - Interface for all AST node types
//   - [Program] - A complete UPLC program with version
//   - [Name], [DeBruijn], [NamedDeBruijn] - Variable representations
//
// # Term Types
//
// The AST is generic over variable representation T:
//
//   - [Var] - Variable reference
//   - [Lambda] - Lambda abstraction
//   - [Apply] - Function application
//   - [Delay] - Delayed computation
//   - [Force] - Force a delayed computation
//   - [Constant] - Constant values (integers, bytestrings, etc.)
//   - [Builtin] - Builtin function reference
//   - [Constr] - Constructor (sums-of-products)
//   - [Case] - Pattern matching
//   - [Error] - Error term
//
// # Variable Representations
//
// Terms can use different variable representations:
//
//   - [Name] - String names, used immediately after parsing
//   - [DeBruijn] - De Bruijn indices, required for evaluation
//   - [NamedDeBruijn] - Both name and index, useful for debugging
//
// # Basic Usage
//
//	// Parse UPLC text
//	program, err := syn.Parse[syn.Name](input)
//	if err != nil {
//	    // Handle parse error
//	}
//
//	// Convert to De Bruijn indices for evaluation
//	dbProgram, err := syn.NameToDeBruijn(program)
//	if err != nil {
//	    // Handle conversion error
//	}
//
//	// Pretty-print a term
//	output := syn.PrettyTerm[syn.DeBruijn](term)
//
// # Serialization
//
// The package supports two serialization formats:
//
//   - Text format via [Parse] and [PrettyTerm]
//   - FLAT binary format via [Decode] (used on-chain)
//
// # Binder Interface
//
// Types that can bind variables implement the [Binder] interface.
// Types that can be evaluated implement [Eval] (subset of Binder).
package syn
