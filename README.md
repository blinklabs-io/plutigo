# plutigo

An implement of [plutus](https://github.com/IntersectMBO/plutus) in pure Go.

This package aims to only support Untyped Plutus Core because that is all that is needed
for a full node. The other stuff like Typed Plutus Core and Plutus IR is for Plinth.

## Features

- Complete Plutus Support: Implements Untyped Plutus Core (UPLC) evaluation
- Multi-Version Support: Compatible with Plutus V1, V2, and V3
- Cost Model Integration: Automatic cost model selection based on Plutus version
- High Performance: Optimized CEK machine implementation in pure Go

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

The library automatically selects appropriate cost models and builtin behavior based on the program version.
