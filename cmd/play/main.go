package main

import (
	"fmt"
	"os"

	"github.com/blinklabs-io/plutigo/pkg/machine"
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

	program, _ := syn.Parse(input)

	dProgram, _ := syn.NameToNamedDeBruijn(program)

	mach := machine.NewMachine(200)

	term, _ := machine.Run[syn.NamedDeBruijn](mach, dProgram.Term)

	prettyTerm := syn.PrettyTerm[syn.NamedDeBruijn](term)

	fmt.Println(prettyTerm)
}
