package cek

import "testing"

func TestBoolConstantReturnsDistinctValues(t *testing.T) {
	first := boolConstant(true)
	second := boolConstant(true)

	if first == second {
		t.Fatal("boolConstant(true) returned shared pointer")
	}

	falseFirst := boolConstant(false)
	falseSecond := boolConstant(false)

	if falseFirst == falseSecond {
		t.Fatal("boolConstant(false) returned shared pointer")
	}
}
