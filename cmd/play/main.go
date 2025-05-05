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

	var filename string
	var format bool

	if os.Args[1] == "-f" {
		filename = os.Args[2]
		format = true
	} else {
		filename = os.Args[1]
		format = false
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("loading file error: %v\n\n", err)

		os.Exit(1)
	}

	input := string(content)

	program, err := syn.Parse(input)
	if err != nil {
		fmt.Printf("parse error: %v\n\n", err)

		os.Exit(1)
	}

	if !format {
		dProgram, err := syn.NameToNamedDeBruijn(program)
		if err != nil {
			fmt.Printf("conversion error: %v\n\n", err)

			os.Exit(1)
		}

		machine := cek.NewMachine[syn.NamedDeBruijn](200)

		term, err := machine.Run(dProgram.Term)
		if err != nil {
			fmt.Printf("eval error: %v\n\n", err)

			os.Exit(1)
		}

		prettyTerm := syn.PrettyTerm[syn.NamedDeBruijn](term)

		fmt.Println(prettyTerm)
	} else {
		prettyProgram := syn.Pretty[syn.Name](program)

		os.WriteFile(filename, []byte(prettyProgram), 0644)

		fmt.Println("done.")
	}
}
