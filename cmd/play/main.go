package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/blinklabs-io/plutigo/pkg/cek"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type Output struct {
	Result string   `json:"result"`
	Cpu    int64    `json:"cpu"`
	Mem    int64    `json:"mem"`
	Logs   []string `json:"logs"`
}

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
			log.Fatalf("conversion error: %v\n\n", err)
		}

		machine := cek.NewMachine[syn.NamedDeBruijn](200)

		term, err := machine.Run(dProgram.Term)
		if err != nil {
			log.Fatalf("eval error: %v\n\n", err)
		}

		prettyTerm := syn.PrettyTerm[syn.NamedDeBruijn](term)

		consumedBudget := cek.DefaultExBudget.Sub(&machine.ExBudget)

		output, err := json.MarshalIndent(Output{
			Result: prettyTerm,
			Cpu:    consumedBudget.Cpu,
			Mem:    consumedBudget.Mem,
			Logs:   machine.Logs,
		}, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling JSON: %v", err)
		}

		fmt.Println(string(output))
	} else {
		prettyProgram := syn.Pretty[syn.Name](program)

		_ = os.WriteFile(filename, []byte(prettyProgram), 0644)

		fmt.Println("done.")
	}
}
