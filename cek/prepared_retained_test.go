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
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// retainedShapes returns programs whose retained memory is far from
// proportional to their encoded size: long constant lists of tiny or
// zero-width items, nested lists, data lists and wide constructors.
func retainedShapes(t *testing.T) map[string][]byte {
	t.Helper()
	const n = 20000
	var ints, units, bools, empties, lists, pairs []syn.IConstant
	var dataItems []data.PlutusData
	fields := make([]syn.Term[syn.DeBruijn], 0, n)
	for i := range n {
		ints = append(ints, &syn.Integer{Inner: big.NewInt(int64(i % 60))})
		units = append(units, &syn.Unit{})
		bools = append(bools, &syn.Bool{Inner: true})
		empties = append(empties, &syn.ByteString{Inner: []byte{}})
		lists = append(lists, &syn.ProtoList{LTyp: &syn.TUnit{}})
		pairs = append(pairs, &syn.ProtoPair{
			FstType: &syn.TUnit{}, SndType: &syn.TUnit{},
			First: &syn.Unit{}, Second: &syn.Unit{},
		})
		dataItems = append(dataItems, data.NewInteger(big.NewInt(int64(i%20))))
		fields = append(fields, &syn.Delay[syn.DeBruijn]{Term: &syn.Error{}})
	}
	con := func(c syn.IConstant) syn.Term[syn.DeBruijn] { return &syn.Constant{Con: c} }
	terms := map[string]syn.Term[syn.DeBruijn]{
		"integer list": con(&syn.ProtoList{LTyp: &syn.TInteger{}, List: ints}),
		"unit list":    con(&syn.ProtoList{LTyp: &syn.TUnit{}, List: units}),
		"bool list":    con(&syn.ProtoList{LTyp: &syn.TBool{}, List: bools}),
		"empty bytestring list": con(&syn.ProtoList{
			LTyp: &syn.TByteString{}, List: empties,
		}),
		"nested empty lists": con(&syn.ProtoList{
			LTyp: &syn.TList{Typ: &syn.TUnit{}}, List: lists,
		}),
		"unit pair list": con(&syn.ProtoList{
			LTyp: &syn.TPair{First: &syn.TUnit{}, Second: &syn.TUnit{}},
			List: pairs,
		}),
		"data list":         con(&syn.Data{Inner: data.NewList(dataItems...)}),
		"wide constructor":  &syn.Constr[syn.DeBruijn]{Fields: fields},
		"string":            con(&syn.String{Inner: string(make([]byte, n))}),
		"constructor field": &syn.Constr[syn.DeBruijn]{Fields: fields[:5]},
	}
	out := make(map[string][]byte, len(terms))
	for name, term := range terms {
		flat, err := syn.Encode(&syn.Program[syn.DeBruijn]{
			Version: lang.LanguageVersion{1, 1, 0},
			Term:    term,
		})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out[name] = flat
	}
	return out
}

// TestPreparedRetainedBytesTracksHeap checks that RetainedBytes stays close
// to, and never below, the heap a prepared program actually keeps alive, for
// the benchmark scripts and for shapes whose memory is not proportional to
// their size.
func TestPreparedRetainedBytesTracksHeap(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("measures the heap")
	}
	paths, err := filepath.Glob("../tests/bench/*.flat")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no benchmark scripts (%v)", err)
	}
	programs := retainedShapes(t)
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		programs[filepath.Base(p)] = b
	}
	// Protocol 10 admits the wide constructor, which PV11 limits.
	pc := syn.ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: 10}
	const copies = 20
	for name, b := range programs {
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
		// Small programs carry the fixed overhead, so the upper bound
		// allows a few kilobytes of slack.
		if ratio < 1 || (ratio > 1.5 && float64(estimate)-actual > 4096) {
			t.Errorf("%s: RetainedBytes %d is %.2f times the %.0f bytes retained",
				name, estimate, ratio, actual)
		}
	}
}
