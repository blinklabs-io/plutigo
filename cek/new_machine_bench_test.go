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

package cek

import (
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

// BenchmarkNewMachineLedgerContext measures per-evaluation machine setup the
// way a ledger drives it: one shared EvalContext carrying a real protocol
// major, and a fresh machine per script.
func BenchmarkNewMachineLedgerContext(b *testing.B) {
	evalContext := NewDefaultEvalContext(
		LanguageVersionV2,
		ProtoVersion{Major: 8, Minor: 0},
	)
	b.ReportAllocs()
	for b.Loop() {
		m := NewMachine[syn.DeBruijn](LanguageVersionV2, 200, evalContext)
		if m == nil {
			b.Fatal("nil machine")
		}
	}
}
