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

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	input := string(content)

	program, err := syn.Parse(input)

	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)

		os.Exit(1)
	}

	dProgram, err := program.ToEval()

	if err != nil {
		fmt.Printf("Error converting program: %v\n", err)

		os.Exit(1)
	}

	mach := machine.NewMachine(200)

	term, err := mach.Run(dProgram.Term)

	prettyTerm := syn.PrettyTerm[syn.Eval](term)

	fmt.Println(prettyTerm)
}
