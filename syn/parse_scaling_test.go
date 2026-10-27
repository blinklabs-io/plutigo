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

package syn

import (
	"runtime"
	"strings"
	"testing"
)

// maxScalingRatio bounds the growth in bytes allocated when the input grows
// fourfold. Linear parsing allocates about 4 to 5 times as much; a quadratic
// path would allocate about 16 times as much.
const maxScalingRatio = 8

// allocatedBytes returns the fewest bytes any of a few runs of f allocated, so
// that a stray allocation from the runtime does not fail the ratio check. It
// reads process-wide counters, so callers must not run in parallel.
func allocatedBytes(f func()) uint64 {
	best := ^uint64(0)
	for range 3 {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		f()
		runtime.ReadMemStats(&after)
		best = min(best, after.TotalAlloc-before.TotalAlloc)
	}
	return best
}

// TestParseAllocationScalesLinearly pins that parsing literals does not grow
// faster than linearly in the input size. It replaces wall-clock checks, which
// are noisy, with allocation counts, which are deterministic enough to catch a
// quadratic regression.
func TestParseAllocationScalesLinearly(t *testing.T) {
	nestedPair := func(n int) string {
		typ, val := "integer", "1"
		for range n {
			typ = "(pair integer " + typ + ")"
			val = "(1, " + val + ")"
		}
		return "(program 1.0.0 (con " + typ + " " + val + "))"
	}
	tests := []struct {
		name         string
		small, large int
		src          func(n int) string
	}{
		{"hex bytestring", 4000, 16000, func(n int) string {
			return "(program 1.0.0 (con bytestring #" + strings.Repeat("ab", n) + "))"
		}},
		{"string", 4000, 16000, func(n int) string {
			return `(program 1.0.0 (con string "` + strings.Repeat("a", 2*n) + `"))`
		}},
		{"string with escapes", 4000, 16000, func(n int) string {
			return `(program 1.0.0 (con string "` + strings.Repeat(`a\n`, n) + `"))`
		}},
		{"nested pair", 1000, 4000, nestedPair},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			smallSrc, largeSrc := tt.src(tt.small), tt.src(tt.large)
			parse := func(src string) func() {
				return func() {
					if _, err := Parse(src); err != nil {
						t.Fatalf("parse failed: %v", err)
					}
				}
			}
			smallBytes := allocatedBytes(parse(smallSrc))
			largeBytes := allocatedBytes(parse(largeSrc))
			if largeBytes > smallBytes*maxScalingRatio {
				t.Fatalf(
					"parsing %d bytes allocated %d, but %d bytes allocated %d: more than %dx for 4x the input",
					len(largeSrc), largeBytes, len(smallSrc), smallBytes, maxScalingRatio,
				)
			}
		})
	}
}
