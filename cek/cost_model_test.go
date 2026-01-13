package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestMachineVersion(t *testing.T) {
	version := LanguageVersionV2
	machine := NewMachine[syn.DeBruijn](version, 100)

	if machine.version != version {
		t.Errorf("Expected version %v, got %v", version, machine.version)
	}
}

func TestGetCostModel(t *testing.T) {
	v1 := GetCostModel(LanguageVersionV1)
	v2 := GetCostModel(LanguageVersionV2)
	defaultCM := GetCostModel(LanguageVersionV3)

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

func TestExBudgetSub(t *testing.T) {
	a := ExBudget{Mem: 100, Cpu: 200}
	b := ExBudget{Mem: 30, Cpu: 50}
	result := a.Sub(&b)

	expected := ExBudget{Mem: 70, Cpu: 150}
	if result != expected {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestFrameAwaitArgString(t *testing.T) {
	val := Constant{Constant: &syn.Integer{Inner: big.NewInt(42)}}
	ctx := &FrameAwaitFunValue[syn.DeBruijn]{Value: val}
	frame := FrameAwaitArg[syn.DeBruijn]{Value: val, Ctx: ctx}
	str := frame.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	// Just check it contains the expected parts
	if !contains(str, "FrameAwaitArg") || !contains(str, "42") {
		t.Errorf("String does not contain expected content: %s", str)
	}
}

func TestFrameAwaitFunTermString(t *testing.T) {
	env := &Env[syn.DeBruijn]{}
	// Create a simple term
	term := &syn.Var[syn.DeBruijn]{Name: 0}
	ctx := &FrameAwaitFunValue[syn.DeBruijn]{
		Value: Constant{Constant: &syn.Unit{}},
	}
	frame := FrameAwaitFunTerm[syn.DeBruijn]{Env: env, Term: term, Ctx: ctx}
	str := frame.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	if !contains(str, "FrameAwaitFunTerm") {
		t.Errorf("String does not contain expected content: %s", str)
	}
}

func TestFrameAwaitFunValueString(t *testing.T) {
	val := Constant{Constant: &syn.Integer{Inner: big.NewInt(123)}}
	ctx := &FrameAwaitFunValue[syn.DeBruijn]{Value: val}
	frame := FrameAwaitFunValue[syn.DeBruijn]{Value: val, Ctx: ctx}
	str := frame.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	if !contains(str, "FrameAwaitFunValue") || !contains(str, "123") {
		t.Errorf("String does not contain expected content: %s", str)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
func TestValueString(t *testing.T) {
	// Test Constant String
	constant := Constant{Constant: &syn.Integer{Inner: big.NewInt(99)}}
	str := constant.String()
	if !contains(str, "99") {
		t.Errorf("Constant String should contain 99: %s", str)
	}

	// Test Delay String
	env := &Env[syn.DeBruijn]{}
	term := &syn.Var[syn.DeBruijn]{Name: 1}
	delay := Delay[syn.DeBruijn]{Body: term, Env: env}
	str = delay.String()
	if !contains(str, "Delay") {
		t.Errorf("Delay String should contain Delay: %s", str)
	}
}

func TestMachineStateInterface(t *testing.T) {
	env := &Env[syn.DeBruijn]{}
	term := &syn.Var[syn.DeBruijn]{Name: 0}
	val := Constant{Constant: &syn.Unit{}}

	// Test Return implements MachineState
	var ret MachineState[syn.DeBruijn] = Return[syn.DeBruijn]{Ctx: nil, Value: val}
	_ = ret

	// Test Compute implements MachineState
	var comp MachineState[syn.DeBruijn] = Compute[syn.DeBruijn]{Ctx: nil, Env: env, Term: term}
	_ = comp

	// Test Done implements MachineState
	var done MachineState[syn.DeBruijn] = Done[syn.DeBruijn]{term: term}
	_ = done
}
