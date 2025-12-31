# plutigo

An implementation of [Plutus](https://github.com/IntersectMBO/plutus) in pure Go.

This package aims to only support Untyped Plutus Core because that is all that is needed
for a full node. The other stuff like Typed Plutus Core and Plutus IR is for Plinth.

## Features

- Complete Plutus Support: Implements Untyped Plutus Core (UPLC) evaluation
- Multi-Version Support: Compatible with Plutus V1, V2, V3, and initial Plutus V4 support
- Cost Model Integration: Automatic cost model selection based on Plutus version (Plutus V4 cost models are placeholders)
- High Performance: Optimized CEK machine implementation in pure Go
- Cryptographic Operations: Full BLS12-381 support using gnark-crypto
- Comprehensive Testing: 52%+ test coverage with fuzz testing and property-based testing

## Supported CIPs

plutigo implements the following Cardano Improvement Proposals (CIPs) related to Plutus Core:

- [CIP-0042](https://cips.cardano.org/cips/cip42/): `serialiseData` builtin for CBOR serialization of Plutus Data
- [CIP-0049](https://cips.cardano.org/cips/cip49/): ECDSA and Schnorr signature verification builtins
- [CIP-0058](https://cips.cardano.org/cips/cip58/): Bitwise primitives for integers
- [CIP-0085](https://cips.cardano.org/cips/cip85/): Sums-of-products (constructor and case expressions)
- [CIP-0091](https://cips.cardano.org/cips/cip91/): Optimized builtin evaluation (no forced evaluation for saturated calls)
- [CIP-0101](https://cips.cardano.org/cips/cip101/): `keccak_256` hash function
- [CIP-0109](https://cips.cardano.org/cips/cip109/): `expModInteger` for modular exponentiation
- [CIP-0121](https://cips.cardano.org/cips/cip121/): Integer to ByteString conversions
- [CIP-0122](https://cips.cardano.org/cips/cip122/): Logical operations on ByteString
- [CIP-0123](https://cips.cardano.org/cips/cip123/): Bitwise operations on ByteString
- [CIP-0127](https://cips.cardano.org/cips/cip127/): `ripemd_160` hash function
- [CIP-0132](https://cips.cardano.org/cips/cip132/): `dropList` builtin
- [CIP-0133](https://cips.cardano.org/cips/cip133/): BLS12-381 multi-scalar multiplication
- [CIP-0138](https://cips.cardano.org/cips/cip138/): Array type and operations (`lengthOfArray`, `listToArray`, `indexArray`)
- [CIP-0153](https://cips.cardano.org/cips/cip153/): Mary-era Value builtins (`insertCoin`, `lookupCoin`, `scaleValue`, `unionValue`, `valueContains`)
- [CIP-0381](https://cips.cardano.org/cips/cip381/): BLS12-381 pairing operations

### Testing and Conformance
All implemented CIPs include comprehensive conformance tests ensuring correct behavior and cost modeling.

## Performance

plutigo is optimized for high-performance Plutus script evaluation:

### Cryptographic Operations (Go 1.24, ARM64)
- SHA256: 93 ns/op (~10.75M ops/sec)
- Blake2b-256: 326 ns/op (3M ops/sec)
- Ed25519 Verify: 242 μs/op (4K ops/sec)
- ECDSA Verify: 405 μs/op (2.5K ops/sec)
- BLS12-381 G1 Add: 3.4 μs/op (294K ops/sec)
- BLS12-381 Pairing: 3.4 ms/op (294 ops/sec)

### Plutus Script Evaluation
Evaluates complex smart contracts (Uniswap, vesting, etc.) in milliseconds with accurate cost modeling.

## Architecture

### Core Components

- CEK Machine (`cek/`): Optimized evaluation engine with object pooling and memory-efficient state management
- Syntax Layer (`syn/`): Parser, pretty-printer, and AST transformations with De Bruijn conversion
- Builtin Functions (`builtin/`): Complete Plutus builtin function implementations
- Data Layer (`data/`): CBOR encoding/decoding for Plutus data types

### Design Decisions

- Pure Go: No CGO dependencies for better portability and security
- Memory Safety: Comprehensive nil-pointer analysis and bounds checking
- Version Compatibility: Automatic cost model and builtin selection by Plutus version
- Testing First: Property-based testing and fuzzing ensure correctness

## Usage

### Install

```sh
go get github.com/blinklabs-io/plutigo
```

### Example

```go
package main

import (
	"fmt"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

func main() {
	input := `
	(program 1.2.0
	  [
	    [
	      (builtin addInteger)
	      (con integer 1)
	    ]
	    (con integer 1)
	  ]
	)
	`

	pprogram, _ := syn.Parse(input)

	program, _ := syn.NameToDeBruijn(pprogram)

	// Create machine with Plutus V3 support
	machine := cek.NewMachine[syn.DeBruijn](program.Version, 200)

	term, _ := machine.Run(program.Term)

	prettyTerm := syn.PrettyTerm[syn.DeBruijn](term)

	fmt.Println(prettyTerm) // Output: (con integer 2)
}
```

## Plutus Version Support

plutigo supports all major Plutus protocol versions:

- Plutus V1 (1.0.0): Alonzo era - Basic builtin functions
- Plutus V2 (1.1.0): Vasil era - Additional crypto builtins
- Plutus V3 (1.2.0+): Chang+ era - Latest features and optimizations
- Plutus V4 (1.3.0+): Initial support with placeholder cost models

The library automatically selects appropriate cost models and builtin behavior based on the program version.

## Development

### Prerequisites

- Go 1.24+
- make

### Setup

```sh
git clone https://github.com/blinklabs-io/plutigo.git
cd plutigo
go mod tidy
```

### Testing

```sh
# Run all tests
make test

# Run benchmarks
make bench

# Run fuzz tests
make fuzz
```

### Code Quality

The project maintains high code quality standards:

- Linting: Passes golangci-lint with zero issues
- Nil Safety: Passes nilaway static analysis
- Test Coverage: 52%+ coverage across all packages
- Fuzz Testing: Continuous fuzzing for parsing and evaluation

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `make test` and `make bench`
5. Submit a pull request

### Code Style

- Follow standard Go formatting (`go fmt`)
- Add tests for new functionality
- Update documentation as needed
- Ensure benchmarks don't regress
