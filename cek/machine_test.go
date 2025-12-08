package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestSmokeBuild(t *testing.T) {
	// Ensure the package builds and a CEK machine can be allocated.
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)
	if m == nil {
		t.Fatal("NewMachineWithVersionCosts returned nil")
	}
}

func TestNewMachineWithVersionCosts(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)
	if m == nil {
		t.Fatal("expected machine, got nil")
	}
	// check default budget
	if m.ExBudget != DefaultExBudget {
		t.Fatalf("expected default budget, got %+v", m.ExBudget)
	}
}

func TestRunConstant(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	// construct a simple constant term (integer)
	term := &syn.Constant{
		Con: &syn.Integer{Inner: big.NewInt(42)},
	}

	out, err := m.Run(term)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if out == nil {
		t.Fatal("Run returned nil term")
	}
}
