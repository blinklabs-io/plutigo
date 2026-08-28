package syn

import (
	"bytes"
	"math/big"
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

func TestWord64Boundary(t *testing.T) {
	encoded := append(bytes.Repeat([]byte{0xff}, 9), 0x01)
	d := newDecoder(encoded)
	got, err := d.word64()
	if err != nil {
		t.Fatalf("word64 returned error: %v", err)
	}
	if got != ^uint64(0) {
		t.Fatalf("word64 = %d, want %d", got, ^uint64(0))
	}
}

// TestDecodeConstrTagPreservesWord64 verifies that constructor tags retain
// their complete FLAT word on every architecture. In particular, a tag with
// bit 32 set must not be narrowed through uint on 32-bit targets.
func TestDecodeConstrTagPreservesWord64(t *testing.T) {
	const tag = uint64(1)<<32 | 7

	encoded, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV3,
		Term:    &Constr[DeBruijn]{Tag: tag},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	for name, decode := range map[string]func([]byte) (*Program[DeBruijn], error){
		"generic decoder":  Decode[DeBruijn],
		"debruijn decoder": DecodeDeBruijn,
	} {
		t.Run(name, func(t *testing.T) {
			program, err := decode(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			constr, ok := program.Term.(*Constr[DeBruijn])
			if !ok {
				t.Fatalf("decoded term has type %T, want *Constr", program.Term)
			}
			if constr.Tag != tag {
				t.Fatalf("constructor tag = %d, want %d", constr.Tag, tag)
			}
		})
	}
}

func TestBigWordSmallLargeValue(t *testing.T) {
	values := []struct {
		name string
		word *big.Int
	}{
		{name: "zero", word: new(big.Int)},
		{name: "uint64 max", word: new(big.Int).SetUint64(^uint64(0))},
		{name: "above uint64", word: new(big.Int).Lsh(big.NewInt(1), 64)},
		{
			name: "crosses several machine words",
			word: new(big.Int).Sub(
				new(big.Int).Lsh(big.NewInt(1), 129),
				big.NewInt(1),
			),
		},
		{
			name: "large",
			word: new(big.Int).Sub(
				new(big.Int).Lsh(big.NewInt(1), 4096),
				big.NewInt(1),
			),
		},
	}
	alignments := []struct {
		name       string
		prefixBits byte
		prefix     byte
	}{
		{name: "aligned"},
		{name: "one bit", prefixBits: 1, prefix: 1},
		{name: "unaligned", prefixBits: 3, prefix: 5},
		{name: "seven bits", prefixBits: 7, prefix: 0x55},
	}

	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			for _, alignment := range alignments {
				t.Run(alignment.name, func(t *testing.T) {
					e := newEncoder()
					if alignment.prefixBits != 0 {
						e.bits(alignment.prefixBits, alignment.prefix)
					}
					e.bigWord(value.word)
					if e.usedBits != 0 {
						e.nextWord()
					}

					d := newDecoder(e.buffer)
					if alignment.prefixBits != 0 {
						prefix, err := d.bits8(alignment.prefixBits)
						if err != nil {
							t.Fatalf("decode prefix: %v", err)
						}
						if prefix != alignment.prefix {
							t.Fatalf("prefix = %d, want %d", prefix, alignment.prefix)
						}
					}

					small, large, err := d.bigWordSmall()
					if err != nil {
						t.Fatalf("decode big word: %v", err)
					}
					got := large
					if got == nil {
						got = new(big.Int).SetUint64(small)
					}
					if got.Cmp(value.word) != 0 {
						t.Fatalf(
							"decoded word differs: bit length = %d, want %d",
							got.BitLen(), value.word.BitLen(),
						)
					}
				})
			}
		})
	}
}

func TestBigWordSmallLargeValueAllocations(t *testing.T) {
	const continuationGroups = 2048
	encoded := append(
		bytes.Repeat([]byte{0xff}, continuationGroups),
		0x7f,
	)

	d := newDecoder(encoded)
	_, decoded, err := d.bigWordSmall()
	if err != nil {
		t.Fatalf("decode big word: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded large word is nil")
	}
	wantBitLen := len(encoded) * 7
	if decoded.BitLen() != wantBitLen {
		t.Fatalf("decoded bit length = %d, want %d", decoded.BitLen(), wantBitLen)
	}

	var decodedBitLen int
	allocs := testing.AllocsPerRun(5, func() {
		d := newDecoder(encoded)
		_, decoded, _ := d.bigWordSmall()
		if decoded != nil {
			decodedBitLen = decoded.BitLen()
		}
	})
	if decodedBitLen != wantBitLen {
		t.Fatalf("measured decode bit length = %d, want %d", decodedBitLen, wantBitLen)
	}
	if allocs > 32 {
		t.Fatalf("large integer decode allocated %.0f objects, want at most 32", allocs)
	}
}

func TestBigWordSmallTruncatedError(t *testing.T) {
	tests := []struct {
		name       string
		prefixBits byte
		prefix     byte
		want       string
	}{
		{name: "aligned", want: "end of buffer"},
		{
			name:       "unaligned",
			prefixBits: 3,
			prefix:     5,
			want:       "NotEnoughBits(8)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEncoder()
			if tt.prefixBits != 0 {
				e.bits(tt.prefixBits, tt.prefix)
			}
			e.bits(8, 0x80)
			if e.usedBits != 0 {
				e.nextWord()
			}

			d := newDecoder(e.buffer)
			if tt.prefixBits != 0 {
				if _, err := d.bits8(tt.prefixBits); err != nil {
					t.Fatalf("decode prefix: %v", err)
				}
			}
			_, _, err := d.bigWordSmall()
			if err == nil {
				t.Fatal("truncated big word decoded without error")
			}
			if err.Error() != tt.want {
				t.Fatalf("decode error = %q, want %q", err, tt.want)
			}
		})
	}
}

var benchmarkBigWordSmall *big.Int

func BenchmarkBigWordSmallLargeValue(b *testing.B) {
	tests := []struct {
		name               string
		continuationGroups int
	}{
		{name: "128_groups", continuationGroups: 128},
		{name: "2048_groups", continuationGroups: 2048},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			encoded := append(
				bytes.Repeat([]byte{0xff}, tt.continuationGroups),
				0x7f,
			)
			b.SetBytes(int64(len(encoded)))
			b.ReportAllocs()
			for b.Loop() {
				d := newDecoder(encoded)
				_, word, err := d.bigWordSmall()
				if err != nil {
					b.Fatalf("decode big word: %v", err)
				}
				benchmarkBigWordSmall = word
			}
		})
	}
}
