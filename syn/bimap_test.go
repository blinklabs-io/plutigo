package syn

import (
	"testing"
)

func TestBiMap(t *testing.T) {
	bm := &biMap{
		left:  make(map[Unique]uint),
		right: make(map[uint]Unique),
	}

	// Test insert and getByUnique
	unique1 := Unique(42)
	level1 := uint(10)
	bm.insert(unique1, level1)

	if level, ok := bm.getByUnique(unique1); !ok || level != level1 {
		t.Errorf(
			"Expected to find level %d for unique %d, got %d, ok=%v",
			level1,
			unique1,
			level,
			ok,
		)
	}

	// Test getByLevel
	if unique, ok := bm.getByLevel(level1); !ok || unique != unique1 {
		t.Errorf(
			"Expected to find unique %d for level %d, got %d, ok=%v",
			unique1,
			level1,
			unique,
			ok,
		)
	}

	// Test non-existent unique
	if _, ok := bm.getByUnique(Unique(999)); ok {
		t.Error("Expected not to find non-existent unique")
	}

	// Test non-existent level
	if _, ok := bm.getByLevel(999); ok {
		t.Error("Expected not to find non-existent level")
	}

	// Test insert another pair
	unique2 := Unique(100)
	level2 := uint(20)
	bm.insert(unique2, level2)

	if level, ok := bm.getByUnique(unique2); !ok || level != level2 {
		t.Errorf(
			"Expected to find level %d for unique %d, got %d, ok=%v",
			level2,
			unique2,
			level,
			ok,
		)
	}

	if unique, ok := bm.getByLevel(level2); !ok || unique != unique2 {
		t.Errorf(
			"Expected to find unique %d for level %d, got %d, ok=%v",
			unique2,
			level2,
			unique,
			ok,
		)
	}

	// Test remove
	bm.remove(unique1, level1)

	if _, ok := bm.getByUnique(unique1); ok {
		t.Error("Expected unique1 to be removed")
	}

	if _, ok := bm.getByLevel(level1); ok {
		t.Error("Expected level1 to be removed")
	}

	// Test that unique2 is still there
	if level, ok := bm.getByUnique(unique2); !ok || level != level2 {
		t.Errorf(
			"Expected unique2 to still exist with level %d, got %d, ok=%v",
			level2,
			level,
			ok,
		)
	}
}

func TestBiMapOverwrite(t *testing.T) {
	bm := &biMap{
		left:  make(map[Unique]uint),
		right: make(map[uint]Unique),
	}

	// Insert initial pair
	unique := Unique(1)
	level1 := uint(10)
	bm.insert(unique, level1)

	// Insert same unique with different level (should overwrite)
	level2 := uint(20)
	bm.insert(unique, level2)

	// Should find new level
	if level, ok := bm.getByUnique(unique); !ok || level != level2 {
		t.Errorf(
			"Expected level %d after overwrite, got %d, ok=%v",
			level2,
			level,
			ok,
		)
	}

	// Old level should not exist
	if _, ok := bm.getByLevel(level1); ok {
		t.Error("Expected old level to be removed after overwrite")
	}

	// New level should point to unique
	if foundUnique, ok := bm.getByLevel(level2); !ok || foundUnique != unique {
		t.Errorf(
			"Expected level %d to point to unique %d, got %d, ok=%v",
			level2,
			unique,
			foundUnique,
			ok,
		)
	}
}
