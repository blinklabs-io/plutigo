package data

import (
	"strings"
	"testing"
)

// deepNestedJSONWithSiblings builds a document of `depth` nested Lists where
// every level also carries a sizable ByteString sibling. A decoder that
// re-parses the remaining subtree at each nesting level does
// O(depth * size) work on this shape.
func deepNestedJSONWithSiblings(depth, siblingHexLen int) string {
	sibling := `{"bytes":"` + strings.Repeat("ab", siblingHexLen/2) + `"}`
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"list":[`)
		sb.WriteString(sibling)
		sb.WriteString(`,`)
	}
	sb.WriteString(`{"int":0}`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`]}`)
	}
	return sb.String()
}

// wideFlatJSON builds a single List with many small Integer items.
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

func BenchmarkDecodeJSONDeepNested(b *testing.B) {
	input := []byte(deepNestedJSONWithSiblings(240, 4096))
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeJSON(input); err != nil {
			b.Fatalf("DecodeJSON error: %v", err)
		}
	}
}

func BenchmarkDecodeJSONWideFlat(b *testing.B) {
	input := []byte(wideFlatJSON(10_000))
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeJSON(input); err != nil {
			b.Fatalf("DecodeJSON error: %v", err)
		}
	}
}
