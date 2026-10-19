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
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func runTextProgram(
	t *testing.T,
	src string,
	protoMajor uint,
) (string, error) {
	t.Helper()
	named, err := syn.Parse(src)
	if err != nil {
		t.Fatalf("parse program: %v", err)
	}
	program, err := syn.NameToDeBruijn(named)
	if err != nil {
		t.Fatalf("convert program: %v", err)
	}
	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		200,
		NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: protoMajor}),
	)
	machine.ExBudget = ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
	result, err := machine.Run(program.Term)
	if err != nil {
		return "", err
	}
	return syn.PrettyTerm[syn.DeBruijn](result), nil
}

func TestCaseOnDataSelectsBranchByConstrTag(t *testing.T) {
	t.Parallel()

	const second = `(program 1.2.0
		(case (con data (Constr 1 [I 42, B #abcd]))
			(lam fields (con integer 0))
			(lam fields fields)))`
	got, err := runTextProgram(t, second, 12)
	if err != nil {
		t.Fatalf("case on Data: %v", err)
	}
	want, err := runTextProgram(t, `(program 1.2.0 (con (list data) [I 42, B #abcd]))`, 12)
	if err != nil {
		t.Fatalf("evaluate expected result: %v", err)
	}
	if got != want {
		t.Fatalf("result = %s, want %s", got, want)
	}

	for name, src := range map[string]string{
		"tag beyond the branches": `(program 1.2.0
			(case (con data (Constr 2 [])) (lam f (con integer 0)) (lam f (con integer 1))))`,
		"integer is not a constr": `(program 1.2.0
			(case (con data (I 1)) (lam f (con integer 0))))`,
		"list is not a constr": `(program 1.2.0
			(case (con data (List [])) (lam f (con integer 0))))`,
	} {
		if _, err := runTextProgram(t, src, 12); err == nil {
			t.Fatalf("%s: case on Data succeeded", name)
		}
	}
}

func TestCaseOnDataIsUnavailableBeforeDijkstra(t *testing.T) {
	t.Parallel()

	const src = `(program 1.2.0
		(case (con data (Constr 0 [])) (lam fields (con integer 42))))`
	_, err := runTextProgram(t, src, 11)
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) || scriptErr.Code != ErrCodeCaseOnBuiltinUnavailable {
		t.Fatalf("case on Data at protocol 11: error = %v, want unavailable", err)
	}
	if got, err := runTextProgram(t, src, 12); err != nil || got != "(con integer 42)" {
		t.Fatalf("case on Data at protocol 12 = %q, %v", got, err)
	}
}

// The reference rejects an open term before evaluating it, so a free variable
// fails the script even when evaluation would never reach it.
func TestRunRejectsOpenTermsEvenWhenUnreached(t *testing.T) {
	t.Parallel()

	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		200,
		NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: 11}),
	)
	machine.ExBudget = ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
	for name, term := range map[string]syn.Term[syn.DeBruijn]{
		"unused free variable": &syn.Lambda[syn.DeBruijn]{Body: &syn.Var[syn.DeBruijn]{Name: 2}},
		"free variable in an untaken branch": &syn.Case[syn.DeBruijn]{
			Constr: &syn.Constant{Con: &syn.Bool{Inner: false}},
			Branches: []syn.Term[syn.DeBruijn]{
				&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}},
				&syn.Var[syn.DeBruijn]{Name: 1},
			},
		},
		"index zero": &syn.Lambda[syn.DeBruijn]{Body: &syn.Var[syn.DeBruijn]{Name: 0}},
	} {
		_, err := machine.Run(term)
		var typeErr *TypeError
		if !errors.As(err, &typeErr) || typeErr.Code != ErrCodeOpenTerm {
			t.Fatalf("%s: error = %v, want an open term error", name, err)
		}
	}

	closed := &syn.Lambda[syn.DeBruijn]{Body: &syn.Var[syn.DeBruijn]{Name: 1}}
	if _, err := machine.Run(closed); err != nil {
		t.Fatalf("closed term: %v", err)
	}
}
