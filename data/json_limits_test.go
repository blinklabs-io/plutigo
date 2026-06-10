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
			"beyond limit is rejected",
			nestedListJSON(MaxDecodeNestingDepth + 50),
			"depth",
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

func TestDecodeJSONMapPairRejectsUnknownFieldBeforeValueDecode(t *testing.T) {
	input := `{"map":[{"k":{"int":1},"junk":` +
		nestedListJSON(MaxDecodeNestingDepth+50) +
		`,"v":{"int":2}}]}`

	_, err := DecodeJSON([]byte(input))
	if err == nil {
		t.Fatal("expected unknown Map pair field error, got nil")
	}
	if !strings.Contains(err.Error(), `unexpected Map pair 0 field "junk"`) {
		t.Fatalf("expected unknown Map pair field error, got: %v", err)
	}
}

// TestDecodeJSONNodeLimit verifies the JSON decoder enforces a node-count cap.
func TestDecodeJSONNodeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a >1M-node document; skipped in -short mode")
	}
	var sb strings.Builder
	sb.WriteString(`{"list":[`)
	n := MaxDecodeNodes + 10
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"int":0}`)
	}
	sb.WriteString(`]}`)

	tests := []struct {
		name             string
		input            string
		wantErrSubstring string
	}{
		{"beyond limit is rejected", sb.String(), "node"},
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
