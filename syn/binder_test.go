package syn

import (
	"testing"
)

func TestNameBinder(t *testing.T) {
	name := Name{
		Text:   "testVar",
		Unique: Unique(42),
	}

	// Test TextName
	if name.TextName() != "testVar" {
		t.Errorf("Expected TextName 'testVar', got %q", name.TextName())
	}

	// Test String
	expectedString := "Name: testVar 42"
	if name.String() != expectedString {
		t.Errorf("Expected String %q, got %q", expectedString, name.String())
	}
}

func TestNamedDeBruijnBinder(t *testing.T) {
	nd := NamedDeBruijn{
		Text:  "param",
		Index: DeBruijn(5),
	}

	// Test TextName
	expectedTextName := "param_5"
	if nd.TextName() != expectedTextName {
		t.Errorf("Expected TextName %q, got %q", expectedTextName, nd.TextName())
	}

	// Test String
	expectedString := "NamedDeBruijn: param 5"
	if nd.String() != expectedString {
		t.Errorf("Expected String %q, got %q", expectedString, nd.String())
	}

	// Test LookupIndex
	if nd.LookupIndex() != 5 {
		t.Errorf("Expected LookupIndex 5, got %d", nd.LookupIndex())
	}
}

func TestDeBruijnBinder(t *testing.T) {
	db := DeBruijn(7)

	// Test TextName
	expectedTextName := "i_7"
	if db.TextName() != expectedTextName {
		t.Errorf("Expected TextName %q, got %q", expectedTextName, db.TextName())
	}

	// Test String
	expectedString := "DeBruijn: 7"
	if db.String() != expectedString {
		t.Errorf("Expected String %q, got %q", expectedString, db.String())
	}

	// Test LookupIndex
	if db.LookupIndex() != 7 {
		t.Errorf("Expected LookupIndex 7, got %d", db.LookupIndex())
	}
}

// TestBinderInterfaceCompliance tests that all binder types implement the Binder interface
func TestBinderInterfaceCompliance(t *testing.T) {
	var _ Binder = Name{}
	var _ Binder = NamedDeBruijn{}
	var _ Binder = DeBruijn(0)

	var _ Eval = NamedDeBruijn{}
	var _ Eval = DeBruijn(0)
}

// TestUniqueType tests the Unique type
func TestUniqueType(t *testing.T) {
	u1 := Unique(42)
	u2 := Unique(42)
	u3 := Unique(43)

	if u1 != u2 {
		t.Error("Expected equal Unique values to be equal")
	}

	if u1 == u3 {
		t.Error("Expected different Unique values to not be equal")
	}
}

// TestDeBruijnType tests the DeBruijn type
func TestDeBruijnType(t *testing.T) {
	db1 := DeBruijn(5)
	db2 := DeBruijn(5)
	db3 := DeBruijn(6)

	if db1 != db2 {
		t.Error("Expected equal DeBruijn values to be equal")
	}

	if db1 == db3 {
		t.Error("Expected different DeBruijn values to not be equal")
	}

	// Test negative values are allowed (they represent invalid states)
	dbNeg := DeBruijn(-1)
	if dbNeg.LookupIndex() != -1 {
		t.Errorf("Expected LookupIndex -1, got %d", dbNeg.LookupIndex())
	}
}
