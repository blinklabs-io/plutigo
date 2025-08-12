# plutigo

An implement of [plutus](https://github.com/IntersectMBO/plutus) in pure Go.

This package aims to only support Untyped Plutus Core because that is all that is needed
for a full node. The other stuff like Typed Plutus Core and Plutus IR is for Plinth.

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
	(program 1.0.0
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

	machine := cek.NewMachine[syn.DeBruijn](200)

	term, _ := machine.Run(program.Term)

	prettyTerm := syn.PrettyTerm[syn.DeBruijn](term)

	fmt.Println(prettyTerm)
}
```
