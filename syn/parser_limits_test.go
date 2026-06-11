package syn

import (
	"fmt"
	"strings"
	"testing"
)

// TestParseConstrVersionGate verifies that constr/case availability is gated by
// a correct lexicographic comparison against version 1.1.0, rather than a
// component-wise comparison that mis-handles versions like 0.9.0.
func TestParseConstrVersionGate(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{"v1.0.0 rejects constr", "(program 1.0.0 (constr 0))", true},
		{"v0.9.0 rejects constr", "(program 0.9.0 (constr 0))", true},
		{"v1.0.0 rejects case", "(program 1.0.0 (case (con integer 0)))", true},
		{"v1.1.0 allows constr", "(program 1.1.0 (constr 0))", false},
		{"v1.2.0 allows constr", "(program 1.2.0 (constr 0))", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.src)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tt.src)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.src, err)
			}
		})
	}
}

// TestParseDepthLimit verifies the recursive-descent parser rejects
// pathologically deep nesting with an error instead of overflowing the Go
// stack. A modestly deep program still parses.
func TestParseDepthLimit(t *testing.T) {
	t.Run("modest depth parses", func(t *testing.T) {
		src := "(program 1.0.0 " + strings.Repeat("(delay ", 500) +
			"(con unit ())" + strings.Repeat(")", 500) + ")"
		if _, err := Parse(src); err != nil {
			t.Fatalf("modest-depth program should parse, got: %v", err)
		}
	})

	t.Run("excessive depth is rejected", func(t *testing.T) {
		const depth = 200000
		src := "(program 1.0.0 " + strings.Repeat("(delay ", depth) +
			"(con unit ())" + strings.Repeat(")", depth) + ")"
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected depth-limit error, got nil")
		}
		if !strings.Contains(err.Error(), "too deep") {
			t.Fatalf("expected a depth error, got: %v", err)
		}
	})
}

// TestParseApplyWidth verifies that a single application with more arguments
// than maxParseDepth is rejected with a depth-limit error. Each argument in
// [f a1 a2 ... aN] produces one Apply node in a left-nested chain of depth N,
// so very wide applications defeat the recursive-descent depth guard on
// downstream passes (NameToDeBruijn, flat encoding, pretty printing) just as
// deeply nested terms do.
func TestParseApplyWidth(t *testing.T) {
	// Build argument strings: each argument is a simple variable "x".
	makeApply := func(n int) string {
		args := make([]string, n+1)
		args[0] = "(builtin addInteger)" // function head
		for i := 1; i <= n; i++ {
			args[i] = fmt.Sprintf("x%d", i)
		}
		return "(program 1.0.0 [ " + strings.Join(args, " ") + " ])"
	}

	t.Run("healthy width parses", func(t *testing.T) {
		// 100 args is well within maxParseDepth (100_000); must succeed.
		if _, err := Parse(makeApply(100)); err != nil {
			t.Fatalf("100-argument application should parse, got: %v", err)
		}
	})

	t.Run("excessive width is rejected", func(t *testing.T) {
		// maxParseDepth+1 args must be rejected with a "too deep" error.
		src := makeApply(maxParseDepth + 1)
		_, err := Parse(src)
		if err == nil {
			t.Fatal("expected depth-limit error for wide application, got nil")
		}
		if !strings.Contains(err.Error(), "too deep") {
			t.Fatalf("expected a depth error, got: %v", err)
		}
	})
}

func TestParseApplyReleasesWidthOnArgumentError(t *testing.T) {
	p := NewParser("[(builtin addInteger) (con integer x)]")
	_, err := p.ParseTerm()
	if err == nil {
		t.Fatal("expected argument parse error, got nil")
	}
	if p.depth != 0 {
		t.Fatalf("parser depth leaked after argument parse error: got %d, want 0", p.depth)
	}
}
