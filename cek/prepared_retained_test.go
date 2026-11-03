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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// TestPreparedRetainedBytesTracksHeap checks that RetainedBytes stays close
// to, and not far below, the heap a prepared program actually keeps alive.
func TestPreparedRetainedBytesTracksHeap(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("measures the heap")
	}
	paths, err := filepath.Glob("../tests/bench/*.flat")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no benchmark scripts (%v)", err)
	}
	pc := syn.ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: 11}
	const copies = 20
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		progs := make([]*PreparedProgram, 0, copies)
		for range copies {
			prepared, err := PrepareProgram(b, pc)
			if err != nil {
				t.Fatal(err)
			}
			if prepared == nil {
				t.Fatal("PrepareProgram returned no program")
			}
			progs = append(progs, prepared)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		actual := float64(after.HeapAlloc-before.HeapAlloc) / copies
		estimate := progs[0].RetainedBytes()
		runtime.KeepAlive(progs)
		ratio := float64(estimate) / actual
		if ratio < 0.95 || ratio > 1.4 {
			t.Errorf("%s: RetainedBytes %d is %.2f times the %.0f bytes retained",
				filepath.Base(p), estimate, ratio, actual)
		}
	}
}
