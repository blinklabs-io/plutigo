package syn

import (
	"bytes"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

// buildNestedDelay returns a term consisting of `depth` nested Delay wrappers
// around an Error leaf.
func buildNestedDelay(depth int) Term[DeBruijn] {
	var term Term[DeBruijn] = &Error{}
	for i := 0; i < depth; i++ {
		term = &Delay[DeBruijn]{Term: term}
	}
	return term
}

// TestDecodeTermDepthLimit verifies the flat decoder rejects pathologically
// deep term nesting with an error instead of recursing until the Go stack
// overflows (a fatal, unrecoverable crash). A modestly deep term still decodes.
func TestDecodeTermDepthLimit(t *testing.T) {
	tests := []struct {
		name        string
		depth       int
		wantErrText string
	}{
		{"modest depth decodes", 1000, ""},
		// 200000 is below the Go stack-overflow point (so the pre-fix decoder
		// would happily decode it) but above the decoder's depth limit.
		{"excessive depth is rejected, not crashed", 200000, "too deep"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := Encode(&Program[DeBruijn]{
				Version: lang.LanguageVersionV1,
				Term:    buildNestedDelay(tt.depth),
			})
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			_, err = Decode[DeBruijn](encoded)
			if tt.wantErrText == "" {
				if err != nil {
					t.Fatalf("decode should succeed, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErrText, err)
			}
		})
	}
}

// TestWordOverflowRejected verifies that word() rejects a varint that overflows
// the machine word instead of silently truncating it.
func TestWordOverflowRejected(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		want        uint
		wantErrText string
	}{
		{
			name: "overflow is rejected",
			// 10 continuation groups (each with non-zero data bits) followed by a
			// terminating group: this is a complete varint whose value overflows
			// the machine word. Without an overflow check, word() silently
			// truncates it and returns a wrong value instead of erroring.
			input:       append(bytes.Repeat([]byte{0xFF}, 10), 0x7F),
			wantErrText: "overflow",
		},
		{
			name:  "small word decodes",
			input: []byte{0x80, 0x01}, // 0 | (1 << 7) = 128
			want:  128,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDecoder(tt.input)
			got, err := d.word()
			if tt.wantErrText == "" {
				if err != nil {
					t.Fatalf("unexpected error decoding word: %v", err)
				}
				if got != tt.want {
					t.Fatalf("expected %d, got %d", tt.want, got)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErrText, err)
			}
		})
	}
}
