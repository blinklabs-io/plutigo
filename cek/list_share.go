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
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
)

// Sharing-based list construction and conversion.
//
// The Plutus cost model charges mkCons, headList, tailList, nullList,
// chooseList, listData, mapData and constrData a size-independent amount, so
// none of them may do work proportional to the list length, or a script can
// chain calls to burn CPU and memory the budget does not account for. Lists
// are immutable, so instead of copying they share storage:
//
//   - consCell: every mkCons result is a persistent cons cell (list_lazy.go),
//     so building, head, tail, null, case and drop are O(1) per cell no
//     matter how lists share tails.
//   - frontBuf: a cell also carries its contiguous form when that is cheap to
//     keep. The form lives in a heap buffer that keeps free slots in front of
//     the longest list handed out from it (its "front"); consing onto exactly
//     that list fills one free slot, so chains (any number of them,
//     interleaved in any order) stay contiguous in O(1) amortised. The
//     buffer is reached only through the cells that own it, so it lives
//     exactly as long as some list using it is reachable.
//   - convBuf: the element-wise conversion of a buffer (constant list <->
//     data list, for listData/mapData/constrData and for list-valued mkCons
//     heads) is memoised on the buffer and grows at the front with it, so
//     converting any suffix or front extension costs O(new elements).
//     Lists not built here (program literals, lists decoded from Data) use a
//     per-run cache keyed by the address of their last element instead.
//
// What cannot be made O(1): two different cells consed onto the same tail
// are two distinct lists, and listData/mapData/constrData (and a list used as
// a mkCons head in a list of lists) must produce a contiguous slice for each
// of them, because data.List/Map/Constr expose their elements as slices. That
// copy is O(length) and happens at most once per cell.
//
// Safety invariants (aliasing):
//   - A slot of a frontBuf or convBuf is written only while it lies outside
//     every slice already handed out: slices are only handed out as suffixes
//     buf[lo:], and writes only go to buf[:lo]. Every slot is therefore
//     written once before it becomes visible, and never after.
//   - Buffers and cells are ordinary heap memory, never machine arena
//     memory, so no arena reset or reuse can recycle storage a list still
//     references; the garbage collector keeps it alive.
//   - The per-run caches key entries by a pointer to the source's last
//     element, which keeps the source array alive while the entry exists, so
//     the address cannot be reused for different contents. List storage is
//     never mutated once handed out (machine arenas, which do recycle memory,
//     are only reset between runs, after the caches are dropped).
//   - All state is per machine (machines are not shared across goroutines);
//     the caches are dropped at the end of every run.

const (
	// consMinHeadroom is the minimum number of free front slots in a new
	// buffer.
	consMinHeadroom = 4
	// consSmallCopy is the tail length up to which a list not built by
	// mkCons is copied into a new buffer when consed onto, so that short
	// lists stay contiguous; longer ones are referenced, not copied.
	consSmallCopy = 8
	// convMinCap is the minimum capacity of a conversion buffer.
	convMinCap = 8
)

// frontBuf is a heap array whose suffix base[lo:] is the longest list handed
// out from it. base[:lo] has never been visible to anyone.
type frontBuf[E any] struct {
	base []E
	lo   int
	// sealed marks a full buffer whose front was moved to a larger buffer;
	// it can never be extended again.
	sealed bool
	// conv is the *convBuf of the element conversion this list type admits.
	conv any
}

func newFrontBuf[E any](head E, tail []E, headroom int) *frontBuf[E] {
	base := make([]E, headroom+1+len(tail))
	copy(base[headroom+1:], tail)
	base[headroom] = head
	return &frontBuf[E]{base: base, lo: headroom}
}

// isFront reports whether s, which must be a suffix of b.base, is the
// longest list handed out from b.
func (b *frontBuf[E]) isFront(s []E) bool {
	return !b.sealed && len(s) == len(b.base)-b.lo
}

// consInto returns head prepended to s as a contiguous slice and the buffer
// it lives in. s must be a suffix of b.base when b is non-nil. When s is b's
// front the head goes into a free slot (O(1)); a full front moves to a buffer
// with len(s) free slots (O(1) amortised, by doubling). Otherwise s is copied
// into a new buffer with the given headroom. s is never modified.
func consInto[E any](b *frontBuf[E], head E, s []E, headroom int) ([]E, *frontBuf[E]) {
	if b != nil && b.isFront(s) {
		if b.lo > 0 {
			b.lo--
			b.base[b.lo] = head
			return b.base[b.lo:], b
		}
		b.sealed = true
		headroom = max(headroom, len(s))
	}
	nb := newFrontBuf(head, s, max(headroom, consMinHeadroom))
	return nb.base[nb.lo:], nb
}

// convBuf holds the converted form of the last `covered` elements of a
// source array in dst[len(dst)-covered:]; dst[:len(dst)-covered] is free.
type convBuf[D any] struct {
	dst     []D
	covered int
}

// convertSuffix returns conv applied to every element of src, which must end
// at the end of the source array e describes. Elements are converted in
// order, so the first failing element (and its error) is the same as for a
// plain loop; already converted elements are known to succeed.
func convertSuffix[S, D any](e *convBuf[D], src []S, conv func(S) (D, error)) ([]D, error) {
	n := len(src)
	if n <= e.covered {
		return e.dst[len(e.dst)-n:], nil
	}
	if len(e.dst) < n {
		// A first conversion is sized to fit; a source that has grown at
		// the front since is likely to keep growing, so double.
		size := n + convMinCap
		if e.covered > 0 {
			size = max(2*n, convMinCap)
		}
		grown := make([]D, size)
		copy(grown[size-e.covered:], e.dst[len(e.dst)-e.covered:])
		e.dst = grown
	}
	start := len(e.dst) - n
	need := n - e.covered
	for i := range need {
		d, err := conv(src[i])
		if err != nil {
			return nil, err
		}
		e.dst[start+i] = d
	}
	e.covered = n
	return e.dst[start:], nil
}

// ownedConv returns the conversion buffer stored on b.
func ownedConv[S, D any](b *frontBuf[S]) *convBuf[D] {
	e, _ := b.conv.(*convBuf[D])
	if e == nil {
		e = &convBuf[D]{}
		b.conv = e
	}
	return e
}

// convCache memoises the conversions of source arrays not owned by a
// frontBuf (program literals, lists decoded from Data). Any two slices that
// end at the same element are suffixes of the same immutable array, so one
// entry serves all of them (e.g. successive tails of a literal list). The
// cache lives for one run; it retains at most a constant factor more than
// the machine's arenas already keep alive for that run (every value a
// builtin can receive is referenced from the run's arenas or the program).
type convCache[S, D any] struct {
	lastKey *S
	last    *convBuf[D]
	byEnd   map[*S]*convBuf[D]
}

func (c *convCache[S, D]) lookup(src []S) *convBuf[D] {
	key := &src[len(src)-1]
	if c.last != nil && c.lastKey == key {
		return c.last
	}
	e := c.byEnd[key]
	if e == nil {
		e = &convBuf[D]{}
		if c.byEnd == nil {
			c.byEnd = make(map[*S]*convBuf[D])
		}
		c.byEnd[key] = e
	}
	c.lastKey, c.last = key, e
	return e
}

// convertList converts src element-wise, memoised on owner when src is a
// suffix of owner.base (a list built by mkCons), otherwise in cache.
func convertList[S, D any](
	cache *convCache[S, D],
	owner *frontBuf[S],
	src []S,
	conv func(S) (D, error),
) ([]D, error) {
	if len(src) == 0 {
		return make([]D, 0), nil
	}
	var e *convBuf[D]
	if owner != nil {
		e = ownedConv[S, D](owner)
	} else {
		e = cache.lookup(src)
	}
	return convertSuffix(e, src, conv)
}

// listShare holds the per-machine caches for lists not built by mkCons.
type listShare struct {
	// constant list of data -> []PlutusData (listData, constrData)
	toData convCache[syn.IConstant, data.PlutusData]
	// constant list of pair data data -> [][2]PlutusData (mapData)
	toPairs convCache[syn.IConstant, [2]data.PlutusData]
	// data list / data map -> constant list elements (mkCons heads)
	fromData  convCache[data.PlutusData, syn.IConstant]
	fromPairs convCache[[2]data.PlutusData, syn.IConstant]
}

func (l *listShare) reset() {
	*l = listShare{}
}

func dataItemConstant(item data.PlutusData) (syn.IConstant, error) {
	return &syn.Data{Inner: item}, nil
}

func dataPairConstant(item [2]data.PlutusData) (syn.IConstant, error) {
	return &syn.ProtoPair{
		FstType: sharedDataType,
		SndType: sharedDataType,
		First:   &syn.Data{Inner: item[0]},
		Second:  &syn.Data{Inner: item[1]},
	}, nil
}

// dataListConstant is materializeDataListConstant through the caches.
func (l *listShare) dataListConstant(
	owner *frontBuf[data.PlutusData],
	items []data.PlutusData,
) *syn.ProtoList {
	list, _ := convertList(&l.fromData, owner, items, dataItemConstant)
	return &syn.ProtoList{LTyp: sharedDataType, List: list}
}

// dataMapConstant is materializeDataMapConstant through the caches.
func (l *listShare) dataMapConstant(
	owner *frontBuf[[2]data.PlutusData],
	items [][2]data.PlutusData,
) *syn.ProtoList {
	list, _ := convertList(&l.fromPairs, owner, items, dataPairConstant)
	return &syn.ProtoList{LTyp: sharedPairDataType, List: list}
}
