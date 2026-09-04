package data

import (
	"strings"
	"testing"
)

func nestedListJSON(depth int) string {
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"list":[`)
	}
	sb.WriteString(`{"int":0}`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`]}`)
	}
	return sb.String()
}

func nestedMapKeyJSON(depth int) string {
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"map":[{"k":`)
	}
	sb.WriteString(`{"int":0}`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`,"v":{"int":0}}]}`)
	}
	return sb.String()
}

func wideFlatJSON(items int) string {
	var sb strings.Builder
	sb.WriteString(`{"list":[`)
	for i := 0; i < items; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"int":1}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func wideEmptyConstrListJSON(items int) string {
	var sb strings.Builder
	sb.WriteString(`{"list":[`)
	for i := 0; i < items; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"constructor":0,"fields":[]}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

// TestDecodeJSONDepthLimit verifies the JSON PlutusData decoder enforces the
// same nesting-depth limit as the CBOR decoder, rather than recursing
// arbitrarily deep on untrusted input.
func TestDecodeJSONDepthLimit(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantErrSubstring string
	}{
		{"within limit decodes", nestedListJSON(100), ""},
		{
			"parser rejects nesting before building the full tree",
			nestedListJSON(MaxDecodeNestingDepth * 2),
			"PlutusData JSON tree nesting exceeds max depth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tt.input))
			if tt.wantErrSubstring == "" {
				if err != nil {
					t.Fatalf("expected success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstring) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErrSubstring, err)
			}
		})
	}
}

func TestDecodeJSONMapDepthBoundary(t *testing.T) {
	t.Run("exactly at PlutusData limit decodes", func(t *testing.T) {
		if _, err := DecodeJSON([]byte(nestedMapKeyJSON(MaxDecodeNestingDepth - 1))); err != nil {
			t.Fatalf("expected success at depth limit, got: %v", err)
		}
	})
	t.Run("one past PlutusData limit preserves semantic error", func(t *testing.T) {
		_, err := DecodeJSON([]byte(nestedMapKeyJSON(MaxDecodeNestingDepth)))
		if err == nil {
			t.Fatal("expected depth error, got nil")
		}
		want := "PlutusData JSON nesting exceeds max depth 256"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error containing %q, got: %v", want, err)
		}
	})
}

func TestDecodeJSONMapPairRejectsUnknownFieldBeforeValueDecode(t *testing.T) {
	input := `{"map":[{"k":{"int":1},"junk":` +
		nestedListJSON(100) +
		`,"v":{"int":2}}]}`

	_, err := DecodeJSON([]byte(input))
	if err == nil {
		t.Fatal("expected unknown Map pair field error, got nil")
	}
	if !strings.Contains(err.Error(), `unexpected Map pair 0 field "junk"`) {
		t.Fatalf("expected unknown Map pair field error, got: %v", err)
	}
}

// TestDecodeJSONParserNodeAllowance verifies parser accounting leaves room
// for JSON structural nodes in an otherwise valid datum below the semantic
// PlutusData node limit.
func TestDecodeJSONParserNodeAllowance(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 500K-item document; skipped in -short mode")
	}

	_, err := DecodeJSON([]byte(wideFlatJSON(MaxDecodeNodes / 2)))
	if err != nil {
		t.Fatalf("DecodeJSON rejected a semantically bounded flat list: %v", err)
	}
}

func TestDecodeJSONParserNodeAllowanceRejectsValidConstrList(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a ~1M-item document; skipped in -short mode")
	}

	if _, err := DecodeJSON([]byte(wideEmptyConstrListJSON(MaxDecodeNodes - 10))); err != nil {
		t.Fatalf("DecodeJSON rejected a semantically bounded Constr list: %v", err)
	}
}

// TestDecodeJSONNodeLimit verifies the JSON parser enforces its node-count cap
// before it allocates a complete tree for a datum exceeding the semantic cap.
func TestDecodeJSONNodeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a >2.5M-node document; skipped in -short mode")
	}

	tests := []struct {
		name             string
		input            string
		wantErrSubstring string
	}{
		{
			"parser rejects before building the full tree",
			wideFlatJSON(maxJSONParseNodes / 2),
			"PlutusData JSON tree exceeds max node count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tt.input))
			if tt.wantErrSubstring == "" {
				if err != nil {
					t.Fatalf("expected success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstring) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErrSubstring, err)
			}
		})
	}
}

func FuzzDecodeJSON(f *testing.F) {
	for _, input := range []string{
		`{"int":0}`,
		`{"list":[{"int":1},{"bytes":"ab"}]}`,
		`{"map":[{"k":{"int":1},"v":{"int":2}}]}`,
		`{"constructor":0,"fields":[{"int":1}]}`,
		`{"int":`,
		nestedListJSON(MaxDecodeNestingDepth),
		nestedListJSON(maxJSONParseNestingDepth),
	} {
		f.Add([]byte(input))
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 16*1024 {
			t.Skip()
		}
		_, _ = DecodeJSON(input)
	})
}
