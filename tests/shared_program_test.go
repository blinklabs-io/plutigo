// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/syn"
)

type sharedEvalOutcome struct {
	budget cek.ExBudget
	result string
	err    string
}

func evalSharedProgram(program *syn.Program[syn.DeBruijn]) sharedEvalOutcome {
	machine := cek.NewMachine[syn.DeBruijn](
		program.Version,
		0,
		benchmarkEvalContext(program.Version),
	)
	result, err := machine.Run(program.Term)
	outcome := sharedEvalOutcome{budget: machine.ExBudget}
	if err != nil {
		outcome.err = err.Error()
	} else {
		outcome.result = syn.PrettyTerm[syn.DeBruijn](result)
	}
	return outcome
}

// TestDecodedProgramIsReusableAcrossEvaluations pins the property a
// decode-once cache relies on: evaluation never mutates the decoded term, so
// one decoded program can be run repeatedly and from concurrent machines with
// the same result and budget as a fresh decode.
func TestDecodedProgramIsReusableAcrossEvaluations(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("bench", "*.flat"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no benchmark scripts found: %v", err)
	}
	if testing.Short() {
		files = files[:4]
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			shared, err := syn.Decode[syn.DeBruijn](content)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			before, err := syn.Encode(shared)
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}
			fresh, err := syn.Decode[syn.DeBruijn](content)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			want := evalSharedProgram(fresh)
			if want.err != "" {
				t.Fatalf("fresh evaluation failed: %s", want.err)
			}

			for i := range 2 {
				if got := evalSharedProgram(shared); got != want {
					t.Fatalf("sequential run %d = %+v, want %+v", i, got, want)
				}
			}
			const workers = 4
			outcomes := make([]sharedEvalOutcome, workers)
			var wg sync.WaitGroup
			for i := range workers {
				wg.Go(func() { outcomes[i] = evalSharedProgram(shared) })
			}
			wg.Wait()
			for i, got := range outcomes {
				if got != want {
					t.Fatalf("concurrent run %d = %+v, want %+v", i, got, want)
				}
			}

			after, err := syn.Encode(shared)
			if err != nil {
				t.Fatalf("encode after evaluation failed: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("evaluation modified the shared decoded program")
			}
		})
	}
}
