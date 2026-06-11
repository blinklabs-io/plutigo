package syn

import (
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
