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
	v3 := GetCostModel(LanguageVersionV3)
	v4 := GetCostModel(LanguageVersionV4)

	// Just verify we get different cost models (they should have different builtin costs)
	if v1.builtinCosts == v2.builtinCosts {
		t.Error("V1 and V2 cost models should be different")
	}
	if v2.builtinCosts == v3.builtinCosts {
		t.Error("V2 and V3 cost models should be different")
	}
	// V4 currently uses the same builtin costs as V3 (pending calibration)
	// This is expected behavior until Plan 002 is implemented
	if v4.builtinCosts != v3.builtinCosts {
		t.Error("V4 should currently use same builtin costs as V3 (pending calibration)")
	}
}

func TestVersionLessThan(t *testing.T) {
	// V1 < V2 < V3 < V4
	if !VersionLessThan(LanguageVersionV1, LanguageVersionV2) {
		t.Error("V1 should be less than V2")
	}
	if !VersionLessThan(LanguageVersionV2, LanguageVersionV3) {
		t.Error("V2 should be less than V3")
	}
	if !VersionLessThan(LanguageVersionV3, LanguageVersionV4) {
		t.Error("V3 should be less than V4")
	}

	// Not less than self
	if VersionLessThan(LanguageVersionV3, LanguageVersionV3) {
		t.Error("V3 should not be less than V3")
	}

	// Greater than is not less than
	if VersionLessThan(LanguageVersionV4, LanguageVersionV3) {
		t.Error("V4 should not be less than V3")
	}
}

func TestLanguageVersionV4(t *testing.T) {
	// V4 should be 1.3.0
	expected := LanguageVersion{1, 3, 0}
	if LanguageVersionV4 != expected {
		t.Errorf("Expected V4 to be %v, got %v", expected, LanguageVersionV4)
	}

	// V4 should get V4CostModel
	cm := GetCostModel(LanguageVersionV4)
	if cm.machineCosts != V4CostModel.machineCosts {
		t.Error("V4 should get V4CostModel")
	}

	// V4 should use SemanticsVariantC (same as V3)
	sem := GetSemantics(LanguageVersionV4)
	if sem != SemanticsVariantC {
		t.Errorf("V4 should use SemanticsVariantC, got %v", sem)
	}
}

func TestMachineVersionV4(t *testing.T) {
	version := LanguageVersionV4
	machine := NewMachine[syn.DeBruijn](version, 100)

	if machine.version != version {
		t.Errorf("Expected version %v, got %v", version, machine.version)
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
