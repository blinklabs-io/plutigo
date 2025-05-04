package main

import (
	"fmt"
	"os"

	"github.com/blinklabs-io/plutigo/pkg/cek"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Error: Please provide a file name as argument")

		os.Exit(1)
	}

	filename := os.Args[1]

	content, _ := os.ReadFile(filename)

	input := string(content)

	program, err := syn.Parse(input)
	if err != nil {
		fmt.Printf("parse error: %v", err)

		os.Exit(1)
	}

	dProgram, err := syn.NameToNamedDeBruijn(program)
	if err != nil {
		fmt.Printf("conversion error: %v", err)

		os.Exit(1)
	}

	machine := cek.NewMachine[syn.NamedDeBruijn](200)

	term, err := machine.Run(dProgram.Term)
	if err != nil {
		fmt.Printf("eval error: %v", err)

		os.Exit(1)
	}

	prettyTerm := syn.PrettyTerm[syn.NamedDeBruijn](term)

	fmt.Println(prettyTerm)
}
