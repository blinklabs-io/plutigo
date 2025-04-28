package main

import (
	"fmt"
	"os"

	"github.com/blinklabs-io/plutigo/pkg/machine"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

func main() {
	// Check if a file name was provided
	if len(os.Args) < 2 {
		fmt.Println("Error: Please provide a file name as an argument")
		fmt.Println("Usage: plutigo <filename>")
		os.Exit(1)
	}

	// Get the file name from command line arguments
	fileName := os.Args[1]

	// Read the file content
	content, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", fileName, err)
		os.Exit(1)
	}

	program, err := syn.Parse(string(content))
	if err != nil {
		fmt.Printf("Error %v\n", err)
		os.Exit(1)
	}

	dProgram, err := program.ToEval()
	if err != nil {
		fmt.Printf("Error %v\n", err)
		os.Exit(1)
	}

	fmt.Println(dProgram)

	mach := machine.NewMachine(200)

	eval, err := mach.Run(dProgram.Term)
	if err != nil {
		fmt.Printf("Error %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%#v\n", eval)

}
