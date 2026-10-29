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

package cek

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestRunFlatMatchesRun(t *testing.T) {
	term := syn.Term[syn.DeBruijn](&syn.Apply[syn.DeBruijn]{
		Function: &syn.Lambda[syn.DeBruijn]{
			Body: &syn.Var[syn.DeBruijn]{Name: 1},
		},
		Argument: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(42)}},
	})
	flat, err := syn.Encode(&syn.Program[syn.DeBruijn]{
		Version: lang.LanguageVersionV3,
		Term:    term,
	})
	if err != nil {
		t.Fatal(err)
	}

	regularMachine := newTestMachineFlow()
	regularResult, regularErr := regularMachine.Run(term)
	if regularErr != nil {
		t.Fatal(regularErr)
	}

	flatMachine := newTestMachineFlow()
	flatResult, flatErr := RunFlat(
		flatMachine,
		syn.NewDeBruijnDecoder(),
		flat,
	)
	if flatErr != nil {
		t.Fatal(flatErr)
	}
	if got, want := syn.PrettyTerm[syn.DeBruijn](flatResult), syn.PrettyTerm[syn.DeBruijn](regularResult); got != want {
		t.Fatalf("RunFlat result = %q, Run result = %q", got, want)
	}
	if flatMachine.ExBudget != regularMachine.ExBudget {
		t.Fatalf("RunFlat remaining budget = %+v, Run remaining budget = %+v", flatMachine.ExBudget, regularMachine.ExBudget)
	}
}

func TestRunFlatUsesFullValidationForLargePrograms(t *testing.T) {
	value := bytes.Repeat([]byte{0x5a}, maxFlatRunSkipValidationBytes+1)
	term := &syn.Constant{Con: &syn.ByteString{Inner: value}}
	flat, err := syn.Encode(&syn.Program[syn.DeBruijn]{
		Version: lang.LanguageVersionV3,
		Term:    term,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(flat) <= maxFlatRunSkipValidationBytes {
		t.Fatalf("encoded program length = %d, want > %d", len(flat), maxFlatRunSkipValidationBytes)
	}

	regularMachine := newTestMachineFlow()
	regularResult, err := regularMachine.Run(term)
	if err != nil {
		t.Fatal(err)
	}
	flatMachine := newTestMachineFlow()
	flatResult, err := RunFlat(flatMachine, syn.NewDeBruijnDecoder(), flat)
	if err != nil {
		t.Fatal(err)
	}
	regularBytes := regularResult.(*syn.Constant).Con.(*syn.ByteString).Inner
	flatBytes := flatResult.(*syn.Constant).Con.(*syn.ByteString).Inner
	if !bytes.Equal(flatBytes, regularBytes) {
		t.Fatal("RunFlat returned different bytes")
	}
	if flatMachine.ExBudget != regularMachine.ExBudget {
		t.Fatalf("RunFlat remaining budget = %+v, Run remaining budget = %+v", flatMachine.ExBudget, regularMachine.ExBudget)
	}
}

func TestRunFlatMatchesRunScopeRules(t *testing.T) {
	tests := []struct {
		name       string
		term       syn.Term[syn.DeBruijn]
		protoMajor uint
	}{
		{
			name: "full scope rejects open variable",
			term: &syn.Var[syn.DeBruijn]{Name: 1},
		},
		{
			name: "pre-PV12 scope checks constr fields at evaluation",
			term: &syn.Constr[syn.DeBruijn]{
				Fields: []syn.Term[syn.DeBruijn]{&syn.Var[syn.DeBruijn]{Name: 1}},
			},
			protoMajor: 11,
		},
		{
			name: "PV12 full scope rejects open constr field",
			term: &syn.Constr[syn.DeBruijn]{
				Fields: []syn.Term[syn.DeBruijn]{&syn.Var[syn.DeBruijn]{Name: 1}},
			},
			protoMajor: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flat, err := syn.Encode(&syn.Program[syn.DeBruijn]{
				Version: lang.LanguageVersionV3,
				Term:    tt.term,
			})
			if err != nil {
				t.Fatal(err)
			}
			evalContext := testEvalContext()
			evalContext.ProtoMajor = tt.protoMajor
			regularMachine := NewMachine[syn.DeBruijn](
				lang.LanguageVersionV3,
				0,
				evalContext,
			)
			_, regularErr := regularMachine.Run(tt.term)

			flatMachine := NewMachine[syn.DeBruijn](
				lang.LanguageVersionV3,
				0,
				evalContext,
			)
			_, flatErr := RunFlat(flatMachine, syn.NewDeBruijnDecoder(), flat)
			if regularErr == nil || flatErr == nil {
				t.Fatalf("Run error = %v, RunFlat error = %v; both must fail", regularErr, flatErr)
			}
			regularCode, regularCodeOK := GetErrorCode(regularErr)
			flatCode, flatCodeOK := GetErrorCode(flatErr)
			if !regularCodeOK || !flatCodeOK || regularCode != flatCode {
				t.Fatalf("Run error code = %v (%t), RunFlat error code = %v (%t)", regularCode, regularCodeOK, flatCode, flatCodeOK)
			}
			if regularErr.Error() != flatErr.Error() {
				t.Fatalf("Run error = %q, RunFlat error = %q", regularErr, flatErr)
			}
			if regularMachine.ExBudget != flatMachine.ExBudget {
				t.Fatalf("Run remaining budget = %+v, RunFlat remaining budget = %+v", regularMachine.ExBudget, flatMachine.ExBudget)
			}
		})
	}
}
