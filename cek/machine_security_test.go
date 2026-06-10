package cek

import (
	"errors"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

// TestDischargeValueDepthLimit verifies that discharging a pathologically deep
// result value graph returns an error instead of recursing until the Go stack
// overflows (a fatal, unrecoverable crash). Discharge happens after the
// budgeted evaluation loop, so it must self-limit.
func TestDischargeValueDepthLimit(t *testing.T) {
	var v Value[syn.DeBruijn] = &Constant{&syn.Integer{Inner: big.NewInt(0)}}
	for i := 0; i < 150000; i++ {
		v = &Constr[syn.DeBruijn]{Tag: 0, Fields: []Value[syn.DeBruijn]{v}}
	}
	_, err := dischargeValue[syn.DeBruijn](v)
	if err == nil {
		t.Fatal("expected depth-limit error discharging deep value, got nil")
	}
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError discharging deep value, got %T: %v", err, err)
	}
}

func TestUnwrapConstantPreservesMaterializationDepthLimit(t *testing.T) {
	leaf := &Constant{&syn.Integer{Inner: big.NewInt(0)}}
	var v Value[syn.DeBruijn] = leaf
	for i := 0; i < maxDischargeDepth+2; i++ {
		v = &pairValue[syn.DeBruijn]{first: v, second: leaf}
	}

	_, err := unwrapConstant[syn.DeBruijn](v)
	if err == nil {
		t.Fatal("expected depth-limit error unwrapping deep pair value, got nil")
	}
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected BudgetError unwrapping deep pair value, got %T: %v", err, err)
	}
}

// TestAllocArenaSliceCapBounded verifies that arena-backed slices are returned
// with capacity equal to their length, so a future over-append reallocates
// instead of silently clobbering neighboring arena cells owned by other live
// values.
func TestAllocArenaSliceCapBounded(t *testing.T) {
	var chunks [][]int
	pos := 0

	// First allocation comes from a freshly made chunk (chunkSize 16).
	s1 := allocArenaSlice(&chunks, &pos, 3, 16)
	if cap(s1) != len(s1) {
		t.Fatalf("first alloc: expected cap == len == %d, got cap %d", len(s1), cap(s1))
	}

	// Second allocation reuses the remaining space in the same chunk.
	s2 := allocArenaSlice(&chunks, &pos, 4, 16)
	if cap(s2) != len(s2) {
		t.Fatalf("second alloc: expected cap == len == %d, got cap %d", len(s2), cap(s2))
	}
}
