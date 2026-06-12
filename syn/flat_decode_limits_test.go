package syn

import (
	"bytes"
	"math/bits"
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

// TestDecodeRejectsTrailingBytes verifies the flat decoder consumes the whole
// input: a valid program followed by trailing garbage bytes must be rejected,
// while the exact same bytes without the garbage decode fine. Both the generic
// decode path and the fast DeBruijn path must agree.
func TestDecodeRejectsTrailingBytes(t *testing.T) {
	// A binder-free program so the same encoding is valid for every binder
	// type (DeBruijn and NamedDeBruijn binders encode differently).
	encoded, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV1,
		Term:    &Error{},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	tests := []struct {
		name     string
		trailing []byte
		wantErr  bool
	}{
		{"no trailing bytes decodes", nil, false},
		{"trailing zero byte rejected", []byte{0x00}, true},
		{"trailing 0xFF byte rejected", []byte{0xFF}, true},
		{"multiple trailing bytes rejected", []byte{0x00, 0xFF, 0x00}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]byte, 0, len(encoded)+len(tt.trailing))
			input = append(input, encoded...)
			input = append(input, tt.trailing...)

			_, fastErr := DecodeDeBruijn(input)
			_, genericFastErr := Decode[DeBruijn](input)
			_, genericErr := Decode[NamedDeBruijn](input)

			for path, err := range map[string]error{
				"DecodeDeBruijn":        fastErr,
				"Decode[DeBruijn]":      genericFastErr,
				"Decode[NamedDeBruijn]": genericErr,
			} {
				if !tt.wantErr {
					if err != nil {
						t.Errorf("%s: decode should succeed, got: %v", path, err)
					}
					continue
				}
				if err == nil {
					t.Errorf("%s: expected trailing-bytes error, got nil", path)
					continue
				}
				if !strings.Contains(err.Error(), "trailing bytes") {
					t.Errorf(
						"%s: expected error containing %q, got: %v",
						path, "trailing bytes", err,
					)
				}
			}
		})
	}
}

// TestDecodeVersionOverflowFailsFast verifies that an oversized version varint
// is rejected immediately after the version words are read, before any attempt
// to decode the term. The input deliberately has no term bytes: a decoder that
// validates the version only after the term decode reports "end of buffer"
// instead of the version error.
func TestDecodeVersionOverflowFailsFast(t *testing.T) {
	// major = 2^32 (exceeds MaxUint32), minor = 1, patch = 1, then nothing.
	input := []byte{0x80, 0x80, 0x80, 0x80, 0x10, 0x01, 0x01}

	wantErrText := "version numbers too large"
	if bits.UintSize == 32 {
		// On 32-bit targets the varint itself overflows the machine word, so
		// word() rejects it even earlier.
		wantErrText = "overflows machine word"
	}

	_, fastErr := DecodeDeBruijn(input)
	_, genericFastErr := Decode[DeBruijn](input)
	_, genericErr := Decode[NamedDeBruijn](input)

	for path, err := range map[string]error{
		"DecodeDeBruijn":        fastErr,
		"Decode[DeBruijn]":      genericFastErr,
		"Decode[NamedDeBruijn]": genericErr,
	} {
		if err == nil {
			t.Errorf("%s: expected error, got nil", path)
			continue
		}
		if !strings.Contains(err.Error(), wantErrText) {
			t.Errorf(
				"%s: expected error containing %q, got: %v",
				path, wantErrText, err,
			)
		}
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
