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
	"fmt"
	"math/big"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// Differential tests of the sharing list implementation against a naive
// copy-based model: every list ever produced is kept together with its
// expected contents and re-checked at the end, so any write into storage
// that was already handed out (aliasing bug) is detected.

type listKind int

const (
	kInt   listKind = iota // Constant (list integer)
	kDataC                 // Constant (list data)
	kDataL                 // dataListValue
	kMap                   // dataMapValue
	kPairC                 // Constant (list (pair data data))
	kindCount
)

func (k listKind) String() string {
	return [...]string{"int", "dataC", "dataL", "map", "pairC"}[k]
}

type modelList struct {
	kind  listKind
	value Value[syn.DeBruijn]
	elems []int64
}

type diffHarness struct {
	t     *testing.T
	m     *Machine[syn.DeBruijn]
	rng   *rand.Rand
	lists []modelList
	// snapshots of slices handed out by conversions, re-checked at the end
	snapData  [][]data.PlutusData
	snapPairs [][][2]data.PlutusData
	snapWant  [][]int64
	nextID    int64
}

func pairData(id int64) [2]data.PlutusData {
	return [2]data.PlutusData{data.NewInteger(big.NewInt(id)), data.NewInteger(big.NewInt(id + 1))}
}

func dataConst(id int64) *syn.Data {
	return &syn.Data{Inner: data.NewInteger(big.NewInt(id))}
}

func pairConst(id int64) *syn.ProtoPair {
	p := pairData(id)
	return &syn.ProtoPair{
		FstType: sharedDataType, SndType: sharedDataType,
		First: &syn.Data{Inner: p[0]}, Second: &syn.Data{Inner: p[1]},
	}
}

func dataID(t *testing.T, d data.PlutusData) int64 {
	t.Helper()
	i, ok := d.(*data.Integer)
	if !ok {
		t.Fatalf("expected data integer, got %T", d)
	}
	return i.Inner.Int64()
}

func pairID(t *testing.T, p [2]data.PlutusData) int64 {
	t.Helper()
	a, b := dataID(t, p[0]), dataID(t, p[1])
	if b != a+1 {
		t.Fatalf("corrupt pair (%d, %d)", a, b)
	}
	return a
}

func constID(t *testing.T, c syn.IConstant) int64 {
	t.Helper()
	switch v := c.(type) {
	case *syn.Integer:
		return v.Inner.Int64()
	case *syn.Data:
		return dataID(t, v.Inner)
	case *syn.ProtoPair:
		f, ok1 := v.First.(*syn.Data)
		s, ok2 := v.Second.(*syn.Data)
		if !ok1 || !ok2 {
			t.Fatalf("bad pair constant %v", v)
		}
		return pairID(t, [2]data.PlutusData{f.Inner, s.Inner})
	default:
		t.Fatalf("unexpected element %T", c)
		return 0
	}
}

// readList decodes a list value without flattening it (read-only walk).
func readList(t *testing.T, v Value[syn.DeBruijn]) []int64 {
	t.Helper()
	var out []int64
	switch l := v.(type) {
	case *Constant:
		pl, ok := l.Constant.(*syn.ProtoList)
		if !ok {
			t.Fatalf("not a list constant: %T", l.Constant)
		}
		for _, c := range pl.List {
			out = append(out, constID(t, c))
		}
	case *dataListValue[syn.DeBruijn]:
		for _, d := range l.items {
			out = append(out, dataID(t, d))
		}
	case *dataMapValue[syn.DeBruijn]:
		for _, p := range l.items {
			out = append(out, pairID(t, p))
		}
	case *consConstList[syn.DeBruijn]:
		l.cell.each(func(c syn.IConstant) { out = append(out, constID(t, c)) })
	case *consDataList[syn.DeBruijn]:
		l.cell.each(func(d data.PlutusData) { out = append(out, dataID(t, d)) })
	case *consDataMap[syn.DeBruijn]:
		l.cell.each(func(p [2]data.PlutusData) { out = append(out, pairID(t, p)) })
	default:
		t.Fatalf("not a list value: %T", v)
	}
	return out
}

func valueID(t *testing.T, v Value[syn.DeBruijn]) int64 {
	t.Helper()
	switch x := v.(type) {
	case *Constant:
		return constID(t, x.Constant)
	case *dataValue[syn.DeBruijn]:
		return dataID(t, x.item)
	case *pairValue[syn.DeBruijn]:
		a, b := valueID(t, x.first), valueID(t, x.second)
		if b != a+1 {
			t.Fatalf("corrupt pair value (%d, %d)", a, b)
		}
		return a
	default:
		t.Fatalf("unexpected element value %T", v)
		return 0
	}
}

func (h *diffHarness) call(fn builtin.DefaultFunction, args ...Value[syn.DeBruijn]) (Value[syn.DeBruijn], ExBudget) {
	h.t.Helper()
	b := newTestBuiltin(fn)
	for _, a := range args {
		b = b.ApplyArg(a)
	}
	before := h.m.ExBudget
	v, err := h.m.evalBuiltinApp(b)
	if err != nil {
		h.t.Fatalf("%s: %v", fn, err)
	}
	return v, before.Sub(&h.m.ExBudget)
}

// callBoth runs fn on the list argument as is and on its contiguous form and
// checks both charge the same budget; it returns the first result.
func (h *diffHarness) callBoth(
	fn builtin.DefaultFunction,
	pre []Value[syn.DeBruijn],
	list Value[syn.DeBruijn],
) Value[syn.DeBruijn] {
	h.t.Helper()
	v1, c1 := h.call(fn, append(slices.Clone(pre), list)...)
	_, c2 := h.call(fn, append(slices.Clone(pre), flattenLazyList[syn.DeBruijn](list))...)
	if c1 != c2 {
		h.t.Fatalf("%s on %T charged %+v, contiguous form charged %+v", fn, list, c1, c2)
	}
	return v1
}

func (h *diffHarness) add(kind listKind, v Value[syn.DeBruijn], elems []int64) {
	h.t.Helper()
	if got := readList(h.t, v); !slices.Equal(got, elems) {
		h.t.Fatalf("%v list = %v, want %v", kind, got, elems)
	}
	h.lists = append(h.lists, modelList{kind: kind, value: v, elems: elems})
}

func (h *diffHarness) headFor(kind listKind, id int64) Value[syn.DeBruijn] {
	switch kind {
	case kInt:
		return int64Constant(id)
	case kDataC, kDataL:
		if h.rng.IntN(2) == 0 {
			return &Constant{dataConst(id)}
		}
		return h.m.allocDataValue(data.NewInteger(big.NewInt(id)))
	default:
		if h.rng.IntN(2) == 0 {
			return &Constant{pairConst(id)}
		}
		p := pairData(id)
		return h.m.allocDataPairValue(p[0], p[1])
	}
}

// foreign adds a list of n fresh elements of the given kind that was not
// built by mkCons (like a program literal or a list decoded from Data).
func (h *diffHarness) foreign(kind listKind, n int) {
	ids := make([]int64, n)
	for i := range ids {
		h.nextID += 2
		ids[i] = h.nextID
	}
	constList := func(typ syn.Typ, elem func(int64) syn.IConstant) Value[syn.DeBruijn] {
		l := make([]syn.IConstant, n)
		for i, id := range ids {
			l[i] = elem(id)
		}
		return &Constant{&syn.ProtoList{LTyp: typ, List: l}}
	}
	var v Value[syn.DeBruijn]
	switch kind {
	case kInt:
		v = constList(&syn.TInteger{}, func(id int64) syn.IConstant { return &syn.Integer{Inner: big.NewInt(id)} })
	case kDataC:
		v = constList(sharedDataType, func(id int64) syn.IConstant { return dataConst(id) })
	case kPairC:
		v = constList(sharedPairDataType, func(id int64) syn.IConstant { return pairConst(id) })
	case kDataL:
		l := make([]data.PlutusData, n)
		for i, id := range ids {
			l[i] = data.NewInteger(big.NewInt(id))
		}
		v = &dataListValue[syn.DeBruijn]{items: l}
	case kMap:
		l := make([][2]data.PlutusData, n)
		for i, id := range ids {
			l[i] = pairData(id)
		}
		v = &dataMapValue[syn.DeBruijn]{items: l}
	default:
		h.t.Fatalf("unknown list kind %d", kind)
	}
	h.add(kind, v, ids)
}

func (h *diffHarness) checkExMem(l modelList) {
	h.t.Helper()
	if _, lazy := l.value.(lazyListValue[syn.DeBruijn]); !lazy {
		return
	}
	got := l.value.toExMem()
	want := flattenLazyList[syn.DeBruijn](l.value).toExMem()
	if got != want {
		h.t.Fatalf("toExMem %d, contiguous form %d", got, want)
	}
}

func (h *diffHarness) snapshot(items []data.PlutusData, pairs [][2]data.PlutusData, want []int64) {
	h.snapData = append(h.snapData, items)
	h.snapPairs = append(h.snapPairs, pairs)
	h.snapWant = append(h.snapWant, slices.Clone(want))
}

// pickList returns a random list, biased towards recent lists so long chains
// and many branches form.
func (h *diffHarness) pickList() modelList {
	src := h.lists[h.rng.IntN(len(h.lists))]
	if h.rng.IntN(3) > 0 {
		lo := max(0, len(h.lists)-8)
		src = h.lists[lo+h.rng.IntN(len(h.lists)-lo)]
	}
	return src
}

func (h *diffHarness) step() {
	src := h.pickList()
	switch op := h.rng.IntN(20); {
	case op < 8:
		h.stepCons(src)
	case op < 10:
		h.stepTail(src)
	case op < 11:
		h.stepDrop(src)
	case op < 12:
		h.stepInspect(src)
	case op < 15:
		h.stepConvert(src)
	case op < 17:
		h.stepNest(src)
	case op < 18:
		h.foreign(listKind(h.rng.IntN(int(kindCount))), h.rng.IntN(20))
	case op < 19:
		h.stepFlatten(src)
	default: // drop per-run caches, as at the end of a run
		h.m.listShare.reset()
	}
}

func (h *diffHarness) stepCons(src modelList) {
	h.nextID += 2
	id := h.nextID
	v := h.callBoth(builtin.MkCons, []Value[syn.DeBruijn]{h.headFor(src.kind, id)}, src.value)
	h.add(src.kind, v, append([]int64{id}, src.elems...))
}

func (h *diffHarness) stepTail(src modelList) {
	if len(src.elems) == 0 {
		return
	}
	v := h.callBoth(builtin.TailList, nil, src.value)
	h.add(src.kind, v, slices.Clone(src.elems[1:]))
}

func (h *diffHarness) stepDrop(src modelList) {
	k := h.rng.IntN(len(src.elems) + 3)
	v := h.callBoth(builtin.DropList, []Value[syn.DeBruijn]{int64Constant(int64(k))}, src.value)
	h.add(src.kind, v, slices.Clone(src.elems[min(k, len(src.elems)):]))
}

// stepInspect checks nullList, chooseList and headList.
func (h *diffHarness) stepInspect(src modelList) {
	t := h.t
	null := h.callBoth(builtin.NullList, nil, src.value)
	if got := null.(*Constant).Constant.(*syn.Bool).Inner; got != (len(src.elems) == 0) {
		t.Fatalf("nullList = %v for %v", got, src.elems)
	}
	a, b := int64Constant(1), int64Constant(2)
	v1, c1 := h.call(builtin.ChooseList, src.value, a, b)
	_, c2 := h.call(builtin.ChooseList, flattenLazyList[syn.DeBruijn](src.value), a, b)
	if c1 != c2 || (v1 == a) != (len(src.elems) == 0) {
		t.Fatalf("chooseList mismatch")
	}
	if len(src.elems) > 0 {
		hd := h.callBoth(builtin.HeadList, nil, src.value)
		if got := valueID(t, hd); got != src.elems[0] {
			t.Fatalf("headList = %d, want %d", got, src.elems[0])
		}
	}
}

// stepConvert runs listData, constrData or mapData and converts back.
func (h *diffHarness) stepConvert(src modelList) {
	switch src.kind {
	case kDataC, kDataL:
		fn := builtin.ListData
		var pre []Value[syn.DeBruijn]
		if h.rng.IntN(3) == 0 {
			fn = builtin.ConstrData
			pre = []Value[syn.DeBruijn]{int64Constant(7)}
		}
		v := h.callBoth(fn, pre, src.value)
		var items []data.PlutusData
		switch d := v.(*dataValue[syn.DeBruijn]).item.(type) {
		case *data.List:
			items = d.Items
		case *data.Constr:
			items = d.Fields
		}
		h.snapshot(items, nil, src.elems)
		back, _ := h.call(builtin.UnListData, h.m.allocDataValue(data.NewList(items...)))
		h.add(kDataL, back, slices.Clone(src.elems))
		if fn == builtin.ListData {
			back2, _ := h.call(builtin.UnListData, v)
			h.add(kDataL, back2, slices.Clone(src.elems))
		}
	case kMap, kPairC:
		v := h.callBoth(builtin.MapData, nil, src.value)
		pairs := v.(*dataValue[syn.DeBruijn]).item.(*data.Map).Pairs
		h.snapshot(nil, pairs, src.elems)
		back, _ := h.call(builtin.UnMapData, v)
		h.add(kMap, back, slices.Clone(src.elems))
	}
}

// stepNest uses the list as the mkCons head of a list of lists and reads it
// back, which converts a data list to its constant form.
func (h *diffHarness) stepNest(src modelList) {
	kind := src.kind
	var elemTyp syn.Typ
	switch kind {
	case kInt:
		elemTyp = &syn.TInteger{}
	case kDataC, kDataL:
		elemTyp = sharedDataType
		kind = kDataC
	default:
		elemTyp = sharedPairDataType
		kind = kPairC
	}
	nilOuter := &Constant{&syn.ProtoList{LTyp: &syn.TList{Typ: elemTyp}, List: nil}}
	outer, _ := h.call(builtin.MkCons, src.value, nilOuter)
	inner, _ := h.call(builtin.HeadList, outer)
	if got := readList(h.t, inner); !slices.Equal(got, src.elems) {
		h.t.Fatalf("nested list = %v, want %v", got, src.elems)
	}
	h.add(kind, inner, slices.Clone(src.elems))
}

// stepFlatten flattens the list (which memoises its cells) and checks it.
func (h *diffHarness) stepFlatten(src modelList) {
	if got := readList(h.t, flattenLazyList[syn.DeBruijn](src.value)); !slices.Equal(got, src.elems) {
		h.t.Fatalf("flattened = %v, want %v", got, src.elems)
	}
	h.checkExMem(src)
}

func (h *diffHarness) verifyAll() {
	t := h.t
	for i, l := range h.lists {
		if got := readList(t, l.value); !slices.Equal(got, l.elems) {
			t.Fatalf("list %d (%v) changed: %v, want %v", i, l.kind, got, l.elems)
		}
	}
	for i := range h.snapWant {
		var got []int64
		for _, d := range h.snapData[i] {
			got = append(got, dataID(t, d))
		}
		for _, p := range h.snapPairs[i] {
			got = append(got, pairID(t, p))
		}
		if !slices.Equal(got, h.snapWant[i]) {
			t.Fatalf("converted slice %d changed: %v, want %v", i, got, h.snapWant[i])
		}
	}
}

func TestListSharingDifferential(t *testing.T) {
	for seed := range uint64(24) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			ec := NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: 11})
			m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
			m.ExBudget = ExBudget{Cpu: 1 << 62, Mem: 1 << 62}
			h := &diffHarness{t: t, m: m, rng: rand.New(rand.NewPCG(seed, 0x5eed))}
			for k := range kindCount {
				h.foreign(k, 0)
				h.foreign(k, 3)
				h.foreign(k, consSmallCopy+5)
			}
			for range 4000 {
				h.step()
			}
			h.verifyAll()
			for _, l := range h.lists {
				if got := readList(t, flattenLazyList[syn.DeBruijn](l.value)); !slices.Equal(got, l.elems) {
					t.Fatalf("flattened %v = %v, want %v", l.kind, got, l.elems)
				}
				h.checkExMem(l)
			}
			h.verifyAll()
		})
	}
}
