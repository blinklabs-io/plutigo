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
	"testing"
	"unsafe"
)

// TestContainerSizes guards the size of the containers the decoder and the
// evaluator allocate most. The length form and lazy source share one header
// pointer so these stay in the size classes they had before lazy sources
// existed; a new field should go in the header instead.
func TestContainerSizes(t *testing.T) {
	for name, tc := range map[string]struct{ got, want uintptr }{
		"Constr": {unsafe.Sizeof(Constr{}), 40},
		"List":   {unsafe.Sizeof(List{}), 32},
		"Map":    {unsafe.Sizeof(Map{}), 32},
	} {
		if unsafe.Sizeof(uintptr(0)) != 8 {
			t.Skip("sizes are for 64-bit platforms")
		}
		if tc.got != tc.want {
			t.Errorf("%s is %d bytes, want %d", name, tc.got, tc.want)
		}
	}
}
