package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
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

func TestDataExMemWithNilValues(t *testing.T) {
	// Test that dataExMem does not panic when encountering nil values in nested structures

	t.Run("constr with nil fields", func(t *testing.T) {
		constrWithNil := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				nil,
				&data.Integer{Inner: big.NewInt(42)},
				nil,
			},
		}

		// Should not panic and return correct cost:
		// 1 Constr (4) + 3 fields traversed (nil=4, Integer=4+1, nil=4) = 17
		cost := dataExMem(constrWithNil)()
		expectedCost := ExMem(
			4 + 4 + 4 + 1 + 4,
		) // DataCost for each item + bigIntExMem(42)=1
		if cost != expectedCost {
			t.Errorf("Expected cost %d, got %d", expectedCost, cost)
		}
	})

	t.Run("list with nil items", func(t *testing.T) {
		listWithNil := &data.List{
			Items: []data.PlutusData{
				nil,
				&data.ByteString{Inner: []byte("test")},
				nil,
			},
		}

		// Should not panic and return correct cost:
		// 1 List (4) + nil (4) + ByteString (4 + 1 for 4 bytes) + nil (4) = 17
		cost := dataExMem(listWithNil)()
		expectedCost := ExMem(4 + 4 + 4 + 1 + 4)
		if cost != expectedCost {
			t.Errorf("Expected cost %d, got %d", expectedCost, cost)
		}
	})

	t.Run("map with nil values in pairs", func(t *testing.T) {
		mapWithNil := &data.Map{
			Pairs: [][2]data.PlutusData{
				{nil, &data.Integer{Inner: big.NewInt(1)}},
				{&data.Integer{Inner: big.NewInt(2)}, nil},
			},
		}

		// Should not panic and return correct cost:
		// 1 Map (4) + 4 pair elements (nil=4, Int=4+1, Int=4+1, nil=4) = 22
		cost := dataExMem(mapWithNil)()
		expectedCost := ExMem(4 + 4 + 4 + 1 + 4 + 1 + 4)
		if cost != expectedCost {
			t.Errorf("Expected cost %d, got %d", expectedCost, cost)
		}
	})

	t.Run("all-nil constr fields", func(t *testing.T) {
		allNilConstr := &data.Constr{
			Tag:    0,
			Fields: []data.PlutusData{nil, nil, nil},
		}

		// 1 Constr (4) + 3 nil fields (4 each) = 16
		cost := dataExMem(allNilConstr)()
		expectedCost := ExMem(4 + 4 + 4 + 4)
		if cost != expectedCost {
			t.Errorf("Expected cost %d, got %d", expectedCost, cost)
		}
	})

	t.Run("deeply nested structure with nil", func(t *testing.T) {
		// Constr containing a List containing a Constr with nil
		deeplyNested := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				&data.List{
					Items: []data.PlutusData{
						&data.Constr{
							Tag:    1,
							Fields: []data.PlutusData{nil, nil},
						},
						nil,
					},
				},
				nil,
			},
		}

		// Outer Constr (4) + List (4) + nil (4) + Inner Constr (4) + nil (4) + nil (4) + nil (4) = 28
		cost := dataExMem(deeplyNested)()
		expectedCost := ExMem(4 + 4 + 4 + 4 + 4 + 4 + 4)
		if cost != expectedCost {
			t.Errorf("Expected cost %d, got %d", expectedCost, cost)
		}
	})

	t.Run("nil at different nesting levels", func(t *testing.T) {
		// Test nil appearing at multiple different levels
		mixedNesting := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				nil, // nil at level 1
				&data.Constr{
					Tag: 1,
					Fields: []data.PlutusData{
						nil, // nil at level 2
						&data.Integer{Inner: big.NewInt(100)},
					},
				},
			},
		}

		// Outer Constr (4) + nil (4) + Inner Constr (4) + nil (4) + Integer (4+1) = 21
		cost := dataExMem(mixedNesting)()
		expectedCost := ExMem(4 + 4 + 4 + 4 + 4 + 1)
		if cost != expectedCost {
			t.Errorf("Expected cost %d, got %d", expectedCost, cost)
		}
	})
}

func TestEqualsDataExMemWithNilValues(t *testing.T) {
	// Test that equalsDataExMem does not panic when encountering nil values

	t.Run("nil on left side only", func(t *testing.T) {
		constrWithNil := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				nil,
				&data.Integer{Inner: big.NewInt(42)},
			},
		}

		normalConstr := &data.Constr{
			Tag:    0,
			Fields: []data.PlutusData{&data.Integer{Inner: big.NewInt(42)}},
		}

		// Should not panic
		costX, costY := equalsDataExMem(constrWithNil, normalConstr)
		if costX() == 0 || costY() == 0 {
			t.Error("Expected non-zero costs")
		}
	})

	t.Run("asymmetric nil positions", func(t *testing.T) {
		// x has nil at position 0, y has nil at position 1
		xConstr := &data.Constr{
			Tag:    0,
			Fields: []data.PlutusData{nil, &data.Integer{Inner: big.NewInt(1)}},
		}

		yConstr := &data.Constr{
			Tag:    0,
			Fields: []data.PlutusData{&data.Integer{Inner: big.NewInt(2)}, nil},
		}

		costX, costY := equalsDataExMem(xConstr, yConstr)
		// Both should return the same min value
		if costX() != costY() {
			t.Errorf(
				"Expected costX and costY to return same min value, got %d and %d",
				costX(),
				costY(),
			)
		}
		if costX() == 0 {
			t.Error("Expected non-zero cost")
		}
	})

	t.Run("both sides with nil", func(t *testing.T) {
		constrWithNil1 := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				nil,
				&data.Integer{Inner: big.NewInt(42)},
				nil,
			},
		}

		constrWithNil2 := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				nil,
				nil,
				&data.Integer{Inner: big.NewInt(100)},
			},
		}

		costX, costY := equalsDataExMem(constrWithNil1, constrWithNil2)
		// Both return functions should return the same min value
		if costX() != costY() {
			t.Errorf(
				"Expected costX and costY to return same min value, got %d and %d",
				costX(),
				costY(),
			)
		}
		if costX() == 0 {
			t.Error("Expected non-zero cost")
		}
	})

	t.Run("min cost behavior with size disparity", func(t *testing.T) {
		// Small structure
		smallConstr := &data.Constr{
			Tag:    0,
			Fields: []data.PlutusData{nil},
		}

		// Large structure with many fields
		largeConstr := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				&data.Integer{Inner: big.NewInt(1)},
				&data.Integer{Inner: big.NewInt(2)},
				&data.Integer{Inner: big.NewInt(3)},
				&data.Integer{Inner: big.NewInt(4)},
				&data.Integer{Inner: big.NewInt(5)},
			},
		}

		costX, costY := equalsDataExMem(smallConstr, largeConstr)

		// Both functions should return the same value (min of the two)
		if costX() != costY() {
			t.Errorf(
				"Expected both cost functions to return same min value, got %d and %d",
				costX(),
				costY(),
			)
		}

		// The cost should be based on the smaller structure's traversal
		// Small: Constr (4) + nil (4) = 8
		smallCost := dataExMem(smallConstr)()
		// The min should be close to the small structure's cost
		// (may be slightly different due to interleaved traversal)
		if costX() > smallCost+ExMem(8) {
			t.Errorf(
				"Expected min cost to be close to small structure cost %d, got %d",
				smallCost,
				costX(),
			)
		}
	})

	t.Run("identical structures with nil", func(t *testing.T) {
		constrWithNil := &data.Constr{
			Tag: 0,
			Fields: []data.PlutusData{
				nil,
				&data.Integer{Inner: big.NewInt(42)},
				nil,
			},
		}

		// Same structure
		costX, costY := equalsDataExMem(constrWithNil, constrWithNil)
		if costX() != costY() {
			t.Errorf(
				"Expected identical cost for identical structures, got %d and %d",
				costX(),
				costY(),
			)
		}

		// Cost should equal dataExMem for the structure
		expectedCost := dataExMem(constrWithNil)()
		if costX() != expectedCost {
			t.Errorf(
				"Expected cost %d for identical structures, got %d",
				expectedCost,
				costX(),
			)
		}
	})
}
