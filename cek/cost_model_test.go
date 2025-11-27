package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestMachineVersion(t *testing.T) {
	version := [3]uint32{1, 1, 0}
	machine := NewMachine[syn.DeBruijn](version, 100)

	if machine.version != version {
		t.Errorf("Expected version %v, got %v", version, machine.version)
	}
}

func TestGetCostModel(t *testing.T) {
	v1 := GetCostModel([3]uint32{1, 0, 0})
	v2 := GetCostModel([3]uint32{1, 1, 0})
	defaultCM := GetCostModel([3]uint32{1, 2, 0})

	// Just verify we get different cost models (they should have different builtin costs)
	if v1.builtinCosts == v2.builtinCosts {
		t.Error("V1 and V2 cost models should be different")
	}
	if v2.builtinCosts == defaultCM.builtinCosts {
		t.Error("V2 and Default cost models should be different")
	}
}

func TestBigIntExMem0(t *testing.T) {
	x := big.NewInt(0)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}
}

func TestBigIntExMemSmall(t *testing.T) {
	x := big.NewInt(1600000000000)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}
}

func TestBigIntExMemBig(t *testing.T) {
	x := big.NewInt(160000000000000)

	x.Mul(x, big.NewInt(1000000))

	y := bigIntExMem(x)()

	if y != ExMem(2) {
		t.Error("HOW???")
	}
}

func TestBigIntExMemHuge(t *testing.T) {
	x := big.NewInt(1600000000000000000)

	x.Mul(x, big.NewInt(1000000000000000000))

	x.Mul(x, big.NewInt(1000))

	y := bigIntExMem(x)()

	if y != ExMem(3) {
		t.Error("HOW???")
	}
}
