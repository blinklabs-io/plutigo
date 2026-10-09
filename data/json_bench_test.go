// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
