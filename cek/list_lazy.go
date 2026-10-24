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

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
)

// consCell is one cell of a persistent list: head followed by next when
// next is non-nil, otherwise by the contiguous list base (a list not built by
// mkCons). Cells are immutable except for the memoised contiguous form
// (flat, buf), which is written at most once. When set, flat is a suffix of
// buf.base ending at its last element.
type consCell[E any] struct {
	head E
	next *consCell[E]
	base []E
	n    int // length of the list starting at this cell (>= 1)
	flat []E
	buf  *frontBuf[E]
}

// consOntoCell prepends head to the list starting at cell t in O(1)
// amortised. The result stays contiguous when t is the front of its buffer.
// It initialises c, which the caller allocates together with its value.
func consOntoCell[E any](c *consCell[E], head E, t *consCell[E]) {
	*c = consCell[E]{head: head, next: t, n: t.n + 1}
	if t.flat != nil && t.buf != nil && t.buf.isFront(t.flat) {
		c.flat, c.buf = consInto(t.buf, head, t.flat, 0)
	}
}

// consOntoSlice prepends head to a list not built by mkCons. Short lists are
// copied (bounded work) so chains started from them stay contiguous; longer
// ones are only referenced.
func consOntoSlice[E any](c *consCell[E], head E, s []E) {
	*c = consCell[E]{head: head, base: s, n: len(s) + 1}
	if len(s) <= consSmallCopy {
		c.flat, c.buf = consInto(nil, head, s, 0)
	}
}

// flatten returns the list as a contiguous slice and the buffer it lives in.
// Every cell on the way is memoised and cells are prepended through the
// buffer front, so flattening costs O(unflattened cells + foreign base) once;
// a cell whose tail was already extended by another cell (a branch) costs
// one copy of the tail.
func (x *consCell[E]) flatten() ([]E, *frontBuf[E]) {
	if x.flat != nil {
		return x.flat, x.buf
	}
	path := make([]*consCell[E], 0, 8)
	var tail []E
	var buf *frontBuf[E]
	for c := x; ; {
		if c.flat != nil {
			tail, buf = c.flat, c.buf
			break
		}
		path = append(path, c)
		if c.next == nil {
			tail = c.base
			break
		}
		c = c.next
	}
	for i := len(path) - 1; i >= 0; i-- {
		// When tail must be copied, leave room for the i cells still to be
		// prepended in this walk.
		tail, buf = consInto(buf, path[i].head, tail, i)
		path[i].flat, path[i].buf = tail, buf
	}
	return tail, buf
}

// drop removes the first k elements (0 <= k <= x.n), walking at most k
// cells. It returns the cell the result starts at, or nil and the remaining
// contiguous list.
func (x *consCell[E]) drop(k int) (*consCell[E], []E) {
	c := x
	for k > 0 {
		k--
		if c.next == nil {
			return nil, c.base[k:len(c.base):len(c.base)]
		}
		c = c.next
	}
	return c, nil
}

// each calls f for every element in order.
func (x *consCell[E]) each(f func(E)) {
	for c := x; ; c = c.next {
		f(c.head)
		if c.next == nil {
			for _, item := range c.base {
				f(item)
			}
			return
		}
	}
}

// List values built by mkCons. Each is observationally identical to the
// contiguous value it flattens to:
//
//	consConstList -> *Constant{*syn.ProtoList}
//	consDataList  -> *dataListValue
//	consDataMap   -> *dataMapValue
//
// They are always non-empty. Builtins that only need the head, the tail,
// emptiness or a bounded prefix handle them directly; listData, mapData and
// constrData use their memoised contiguous form; every other consumer
// flattens them first (unwrapList, materializeConstantValue, discharge), so
// code that does not know about them still sees the contiguous value.
type lazyListValue[T syn.Eval] interface {
	Value[T]
	lazyLen() int
	// lazyHead and lazyTail return what headList and tailList return for
	// the equivalent contiguous value.
	lazyHead(m *Machine[T]) Value[T]
	lazyTail(m *Machine[T]) Value[T]
	// lazyCaseHead returns the head as passed to a case branch.
	lazyCaseHead(m *Machine[T]) Value[T]
	// lazyDrop drops 0 <= k <= lazyLen() elements, walking at most k cells.
	lazyDrop(m *Machine[T], k int) Value[T]
	// flatten returns the equivalent contiguous value.
	flatten() Value[T]
}

func flattenLazyList[T syn.Eval](value Value[T]) Value[T] {
	if lazy, ok := value.(lazyListValue[T]); ok {
		return lazy.flatten()
	}
	return value
}

type consConstList[T syn.Eval] struct {
	typ  syn.Typ // element type
	cell *consCell[syn.IConstant]
}

func (l *consConstList[T]) String() string {
	return fmt.Sprintf("ConsList[%d]", l.cell.n)
}

func (*consConstList[T]) isValue() {}

// toExMem matches listExMem of the flattened list.
func (l *consConstList[T]) toExMem() ExMem {
	var acc ExMem
	l.cell.each(func(item syn.IConstant) {
		acc += iconstantExMem(item)() + ConsCost
	})
	return ExMem(NilCost) + acc
}

func (l *consConstList[T]) lazyLen() int { return l.cell.n }

func (l *consConstList[T]) lazyHead(m *Machine[T]) Value[T] {
	return dataAwareConstantValue(m, l.cell.head)
}

func (l *consConstList[T]) lazyCaseHead(m *Machine[T]) Value[T] {
	return m.allocConstant(l.cell.head)
}

func (l *consConstList[T]) rest(m *Machine[T], cell *consCell[syn.IConstant], base []syn.IConstant) Value[T] {
	if cell != nil {
		return &consConstList[T]{typ: l.typ, cell: cell}
	}
	return m.allocConstant(m.allocProtoListConstant(l.typ, base))
}

func (l *consConstList[T]) lazyTail(m *Machine[T]) Value[T] {
	return l.rest(m, l.cell.next, l.cell.base)
}

func (l *consConstList[T]) lazyDrop(m *Machine[T], k int) Value[T] {
	if k == 0 {
		return l
	}
	cell, base := l.cell.drop(k)
	return l.rest(m, cell, base)
}

func (l *consConstList[T]) flatten() Value[T] {
	flat, _ := l.cell.flatten()
	return &Constant{Constant: &syn.ProtoList{LTyp: l.typ, List: flat}}
}

type consDataList[T syn.Eval] struct {
	cell *consCell[data.PlutusData]
}

func (l *consDataList[T]) String() string {
	return fmt.Sprintf("ConsDataList[%d]", l.cell.n)
}

func (*consDataList[T]) isValue() {}

// toExMem matches dataListValue.toExMem of the flattened list.
func (l *consDataList[T]) toExMem() ExMem {
	acc := ExMem(NilCost)
	l.cell.each(func(item data.PlutusData) {
		acc += dataExMem(item)() + ConsCost
	})
	return acc
}

func (l *consDataList[T]) lazyLen() int { return l.cell.n }

func (l *consDataList[T]) lazyHead(m *Machine[T]) Value[T] {
	return m.allocDataValue(l.cell.head)
}

func (l *consDataList[T]) lazyCaseHead(m *Machine[T]) Value[T] {
	return m.allocDataValue(l.cell.head)
}

func (l *consDataList[T]) rest(m *Machine[T], cell *consCell[data.PlutusData], base []data.PlutusData) Value[T] {
	if cell != nil {
		return &consDataList[T]{cell: cell}
	}
	return m.allocDataListValue(base)
}

func (l *consDataList[T]) lazyTail(m *Machine[T]) Value[T] {
	return l.rest(m, l.cell.next, l.cell.base)
}

func (l *consDataList[T]) lazyDrop(m *Machine[T], k int) Value[T] {
	if k == 0 {
		return l
	}
	cell, base := l.cell.drop(k)
	return l.rest(m, cell, base)
}

func (l *consDataList[T]) flatten() Value[T] {
	flat, _ := l.cell.flatten()
	return &dataListValue[T]{items: flat}
}

type consDataMap[T syn.Eval] struct {
	cell *consCell[[2]data.PlutusData]
}

func (l *consDataMap[T]) String() string {
	return fmt.Sprintf("ConsDataMapList[%d]", l.cell.n)
}

func (*consDataMap[T]) isValue() {}

// toExMem matches dataMapValue.toExMem of the flattened list.
func (l *consDataMap[T]) toExMem() ExMem {
	acc := ExMem(NilCost)
	l.cell.each(func(item [2]data.PlutusData) {
		acc += ExMem(PairCost) + dataExMem(item[0])() + dataExMem(item[1])() + ConsCost
	})
	return acc
}

func (l *consDataMap[T]) lazyLen() int { return l.cell.n }

func (l *consDataMap[T]) lazyHead(m *Machine[T]) Value[T] {
	return m.allocDataPairValue(l.cell.head[0], l.cell.head[1])
}

func (l *consDataMap[T]) lazyCaseHead(m *Machine[T]) Value[T] {
	return m.allocDataPairValue(l.cell.head[0], l.cell.head[1])
}

func (l *consDataMap[T]) rest(m *Machine[T], cell *consCell[[2]data.PlutusData], base [][2]data.PlutusData) Value[T] {
	if cell != nil {
		return &consDataMap[T]{cell: cell}
	}
	return m.allocDataMapValue(base)
}

func (l *consDataMap[T]) lazyTail(m *Machine[T]) Value[T] {
	return l.rest(m, l.cell.next, l.cell.base)
}

func (l *consDataMap[T]) lazyDrop(m *Machine[T], k int) Value[T] {
	if k == 0 {
		return l
	}
	cell, base := l.cell.drop(k)
	return l.rest(m, cell, base)
}

func (l *consDataMap[T]) flatten() Value[T] {
	flat, _ := l.cell.flatten()
	return &dataMapValue[T]{items: flat}
}

// The values mkCons returns are allocated together with their first cell, so
// a cons costs one allocation.

type constListAlloc[T syn.Eval] struct {
	v consConstList[T]
	c consCell[syn.IConstant]
}

func newConstListAlloc[T syn.Eval](typ syn.Typ) *constListAlloc[T] {
	a := &constListAlloc[T]{}
	a.v = consConstList[T]{typ: typ, cell: &a.c}
	return a
}

type dataListAlloc[T syn.Eval] struct {
	v consDataList[T]
	c consCell[data.PlutusData]
}

func newDataListAlloc[T syn.Eval]() *dataListAlloc[T] {
	a := &dataListAlloc[T]{}
	a.v = consDataList[T]{cell: &a.c}
	return a
}

type dataMapAlloc[T syn.Eval] struct {
	v consDataMap[T]
	c consCell[[2]data.PlutusData]
}

func newDataMapAlloc[T syn.Eval]() *dataMapAlloc[T] {
	a := &dataMapAlloc[T]{}
	a.v = consDataMap[T]{cell: &a.c}
	return a
}
