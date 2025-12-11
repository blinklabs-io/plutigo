package syn

import (
	"math/big"
	"strings"
	"testing"
)

func TestPrettyTerm(t *testing.T) {
	// Test with a simple integer constant
	term := &Constant{Con: &Integer{Inner: big.NewInt(42)}}

	result := PrettyTerm[DeBruijn](term)

	if result == "" {
		t.Errorf("PrettyTerm returned empty string")
	}

	// Should contain "42"
	if !strings.Contains(result, "42") {
		t.Errorf("PrettyTerm output does not contain '42': %s", result)
	}
}

func TestPrettyTermComplex(t *testing.T) {
	// Test with a lambda
	term := &Lambda[DeBruijn]{
		Body: &Constant{Con: &Integer{Inner: big.NewInt(1)}},
	}

	result := PrettyTerm[DeBruijn](term)

	if result == "" {
		t.Errorf("PrettyTerm returned empty string")
	}

	// Should contain lambda syntax
	if !strings.Contains(result, "lam") {
		t.Errorf("PrettyTerm output does not contain 'lam': %s", result)
	}
}
