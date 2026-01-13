package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/syn"
)

// Helper functions for machine flow tests

func newTestMachineFlow() *Machine[syn.DeBruijn] {
	return NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)
}

// Helper to run a term and assert no error
func runTerm(
	t *testing.T,
	m *Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
) syn.Term[syn.DeBruijn] {
	out, err := m.Run(term)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	return out
}

func TestDelayForce(t *testing.T) {
	m := newTestMachineFlow()

	// (force (delay (con integer 7))) => 7
	delayed := &syn.Delay[syn.DeBruijn]{
		Term: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(7)}},
	}
	force := &syn.Force[syn.DeBruijn]{Term: delayed}

	out := runTerm(t, m, force)
	if out == nil {
		t.Fatal("expected non-nil output for delay/force")
	}
}

func TestLambdaApply(t *testing.T) {
	m := newTestMachineFlow()

	// ( (lam x x) (con integer 1) ) => 1
	// Use DeBruijn indices with 1-based lookup for parameter reference
	lam := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body:          &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
	}
	arg := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}}
	app := &syn.Apply[syn.DeBruijn]{Function: lam, Argument: arg}

	out := runTerm(t, m, app)
	if out == nil {
		t.Fatal("expected non-nil output for lambda apply")
	}
}

func TestBuiltinAddInteger(t *testing.T) {
	m := newTestMachineFlow()

	// Create built-in addInteger applied to two integers
	b := &syn.Builtin{DefaultFunction: builtin.AddInteger}
	left := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(2)}}
	right := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(3)}}

	// ((builtin addInteger) 2) 3
	app1 := &syn.Apply[syn.DeBruijn]{Function: b, Argument: left}
	app2 := &syn.Apply[syn.DeBruijn]{Function: app1, Argument: right}

	out := runTerm(t, m, app2)
	if out == nil {
		t.Fatal("expected non-nil output for builtin addInteger")
	}
}

func TestConstrCase(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

	// Build a simple constructor and a case that matches it
	constr := &syn.Constr[syn.DeBruijn]{
		Tag: 0,
		Fields: []syn.Term[syn.DeBruijn]{
			&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(9)}},
		},
	}

	// Simple branch that returns the field 0 (parameter referenced as DeBruijn(1))
	branch := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body:          &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
	}
	caseTerm := &syn.Case[syn.DeBruijn]{
		Constr:   constr,
		Branches: []syn.Term[syn.DeBruijn]{branch},
	}

	out := runTerm(t, m, caseTerm)
	if out == nil {
		t.Fatal("expected non-nil output for constr/case")
	}
}

func TestBudgetExhaustion(t *testing.T) {
	// Use a machine with very small budget to provoke exhaustion
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)
	m.ExBudget = ExBudget{Mem: 0, Cpu: 0}

	term := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}}
	_, err := m.Run(term)
	if err == nil {
		t.Fatal("expected error due to budget exhaustion, got nil")
	}
}

func TestNestedLambdasEnv(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

	// ( (lam x (lam y x)) (con integer 5) ) => (lam y 5)
	inner := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body:          &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(2)},
	}
	outer := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body:          inner,
	}
	app := &syn.Apply[syn.DeBruijn]{
		Function: outer,
		Argument: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(5)}},
	}

	out, err := m.Run(app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Discharge the value should be a lambda
	if _, ok := out.(*syn.Lambda[syn.DeBruijn]); !ok {
		t.Fatalf("expected lambda result, got %T", out)
	}
}

func TestMissingCaseBranch(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

	// constructor tag 1 but case has only branch 0
	constr := &syn.Constr[syn.DeBruijn]{
		Tag: 1,
		Fields: []syn.Term[syn.DeBruijn]{
			&syn.Constant{Con: &syn.Integer{Inner: big.NewInt(9)}},
		},
	}
	branch := &syn.Lambda[syn.DeBruijn]{
		ParameterName: syn.DeBruijn(0),
		Body:          &syn.Var[syn.DeBruijn]{Name: syn.DeBruijn(1)},
	}
	caseTerm := &syn.Case[syn.DeBruijn]{
		Constr:   constr,
		Branches: []syn.Term[syn.DeBruijn]{branch},
	}

	_, err := m.Run(caseTerm)
	if err == nil {
		t.Fatalf("expected MissingCaseBranch error, got nil")
	}
}

func TestDivisionByZeroBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

	b := &syn.Builtin{DefaultFunction: builtin.DivideInteger}
	v1 := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(10)}}
	v2 := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(0)}}

	app1 := &syn.Apply[syn.DeBruijn]{Function: b, Argument: v1}
	app2 := &syn.Apply[syn.DeBruijn]{Function: app1, Argument: v2}

	_, err := m.Run(app2)
	if err == nil {
		t.Fatalf("expected division by zero error, got nil")
	}
}

func TestNonFunctionalApplication(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

	// Attempt to apply a constant (non-function) to another constant
	fun := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}}
	arg := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(2)}}
	app := &syn.Apply[syn.DeBruijn]{Function: fun, Argument: arg}

	_, err := m.Run(app)
	if err == nil {
		t.Fatalf("expected NonFunctionalApplication error, got nil")
	}
}

func TestForceHeavyBuiltin(t *testing.T) {
	// IfThenElse needs forces (it expects a boolean condition forced)
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

	b := &syn.Builtin{DefaultFunction: builtin.IfThenElse}
	// Apply non-boolean constants to provoke errors when forcing
	app1 := &syn.Apply[syn.DeBruijn]{
		Function: b,
		Argument: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(1)}},
	}
	// Now call with two branches
	app2 := &syn.Apply[syn.DeBruijn]{
		Function: app1,
		Argument: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(2)}},
	}
	app3 := &syn.Apply[syn.DeBruijn]{
		Function: app2,
		Argument: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(3)}},
	}

	_, err := m.Run(app3)
	if err == nil {
		t.Fatalf(
			"expected error from IfThenElse when condition is not boolean, got nil",
		)
	}
}
