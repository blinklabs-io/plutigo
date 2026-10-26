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
	return runTextProgramIn(t, lang.LanguageVersionV3, src, protoMajor)
}

func runTextProgramIn(
	t *testing.T,
	language lang.LanguageVersion,
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
		language,
		200,
		NewDefaultEvalContext(language, ProtoVersion{Major: protoMajor}),
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
// fails the script even when evaluation would never reach it. Before protocol
// version 12 its scope check (NoCaseConstr mode) does not look inside case
// and constr terms: a free variable there fails only when evaluated.
func TestRunRejectsOpenTermsEvenWhenUnreached(t *testing.T) {
	t.Parallel()

	untakenBranch := &syn.Case[syn.DeBruijn]{
		Constr: &syn.Constant{Con: &syn.Bool{Inner: false}},
		Branches: []syn.Term[syn.DeBruijn]{
			&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}},
			&syn.Var[syn.DeBruijn]{Name: 1},
		},
	}
	unevaluatedField := &syn.Delay[syn.DeBruijn]{Term: &syn.Constr[syn.DeBruijn]{
		Fields: []syn.Term[syn.DeBruijn]{&syn.Var[syn.DeBruijn]{Name: 7}},
	}}
	takenBranch := &syn.Case[syn.DeBruijn]{
		Constr: &syn.Constant{Con: &syn.Bool{Inner: true}},
		Branches: []syn.Term[syn.DeBruijn]{
			&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}},
			&syn.Var[syn.DeBruijn]{Name: 1},
		},
	}
	tests := []struct {
		name     string
		term     syn.Term[syn.DeBruijn]
		openAt11 bool
		openAt12 bool
		// failsAt11 is true when the free variable is evaluated.
		failsAt11 bool
	}{
		{"unused free variable", &syn.Lambda[syn.DeBruijn]{Body: &syn.Var[syn.DeBruijn]{Name: 2}}, true, true, true},
		{"index zero", &syn.Lambda[syn.DeBruijn]{Body: &syn.Var[syn.DeBruijn]{Name: 0}}, true, true, true},
		{"free variable in an untaken branch", untakenBranch, false, true, false},
		{"free variable in an unevaluated constr field", unevaluatedField, false, true, false},
		{"free variable in a taken branch", takenBranch, false, true, true},
	}
	for _, tc := range tests {
		for _, proto := range []uint{11, 12} {
			machine := NewMachine[syn.DeBruijn](
				lang.LanguageVersionV3,
				200,
				NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: proto}),
			)
			machine.ExBudget = ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
			_, err := machine.Run(tc.term)
			wantOpen, wantFail := tc.openAt12, tc.openAt12
			if proto == 11 {
				wantOpen, wantFail = tc.openAt11, tc.failsAt11
			}
			var typeErr *TypeError
			isOpen := errors.As(err, &typeErr) && typeErr != nil &&
				typeErr.Code == ErrCodeOpenTerm
			switch {
			case wantFail && !isOpen:
				t.Fatalf("%s at protocol %d: error = %v, want an open term error", tc.name, proto, err)
			case !wantFail && err != nil:
				t.Fatalf("%s at protocol %d: error = %v, want success", tc.name, proto, err)
			case wantOpen && err == nil:
				t.Fatalf("%s at protocol %d: succeeded, want rejection", tc.name, proto)
			}
		}
	}

	machine := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		200,
		NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: 11}),
	)
	machine.ExBudget = ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
	closed := &syn.Lambda[syn.DeBruijn]{Body: &syn.Var[syn.DeBruijn]{Name: 1}}
	if _, err := machine.Run(closed); err != nil {
		t.Fatalf("closed term: %v", err)
	}
}
