package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestBoolConstantReturnsDistinctValues(t *testing.T) {
	first := boolConstant(true)
	second := boolConstant(true)

	firstBool, ok := first.Constant.(*syn.Bool)
	if !ok || !firstBool.Inner {
		t.Fatalf("boolConstant(true) = %#v, want true bool constant", first)
	}
	secondBool, ok := second.Constant.(*syn.Bool)
	if !ok || !secondBool.Inner {
		t.Fatalf("boolConstant(true) second call = %#v, want true bool constant", second)
	}
	if first != second {
		t.Fatal("boolConstant(true) should reuse the cached constant")
	}

	falseFirst := boolConstant(false)
	falseSecond := boolConstant(false)
	falseBool, ok := falseFirst.Constant.(*syn.Bool)
	if !ok || falseBool.Inner {
		t.Fatalf("boolConstant(false) = %#v, want false bool constant", falseFirst)
	}
	if falseFirst != falseSecond {
		t.Fatal("boolConstant(false) should reuse the cached constant")
	}
}
