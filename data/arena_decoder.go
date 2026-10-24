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
	"math/big"
)

const (
	dataDecodeChunkSize = 64
	dataDecodeRetainCap = 2
	dataSliceRetainCap  = 4
)

type arenaChunks[S any] struct {
	chunks      [][]S
	chunkIdx    int
	offset      int
	activeChunk []S // cached pointer to chunks[chunkIdx]; nil when uninitialized
}

func (a *arenaChunks[S]) used() int {
	if len(a.chunks) == 0 {
		return 0
	}
	return a.chunkIdx*dataDecodeChunkSize + a.offset
}

// alloc returns the next free slot. The fast path uses the cached
// activeChunk so we avoid reloading chunks[chunkIdx] on every call.
func (a *arenaChunks[S]) alloc() *S {
	if a.offset < len(a.activeChunk) {
		slot := &a.activeChunk[a.offset]
		a.offset++
		return slot
	}
	return a.allocSlow()
}

//go:noinline
func (a *arenaChunks[S]) allocSlow() *S {
	if nextIdx := a.chunkIdx + 1; nextIdx < len(a.chunks) {
		chunk := a.chunks[nextIdx]
		if chunk != nil {
			a.chunkIdx = nextIdx
			a.activeChunk = chunk
			a.offset = 1
			return &chunk[0]
		}
	}

	chunk := make([]S, dataDecodeChunkSize)
	a.chunks = append(a.chunks, chunk)
	a.chunkIdx = len(a.chunks) - 1
	a.activeChunk = chunk
	a.offset = 1
	return &chunk[0]
}

func (a *arenaChunks[S]) setActiveChunkAfterReset() {
	if len(a.chunks) > 0 {
		a.activeChunk = a.chunks[0]
	} else {
		a.activeChunk = nil
	}
}

func (a *arenaChunks[S]) reset(retainCap int) {
	used := a.used()
	retained := len(a.chunks)
	if retained > retainCap {
		retained = retainCap
	}
	if used > 0 && retained > 0 {
		remaining := used
		maxRetained := retained * dataDecodeChunkSize
		if remaining > maxRetained {
			remaining = maxRetained
		}
		for i := 0; i < retained && remaining > 0; i++ {
			chunk := a.chunks[i]
			if chunk == nil {
				continue
			}
			clearCount := len(chunk)
			if remaining < clearCount {
				clearCount = remaining
			}
			clear(chunk[:clearCount])
			remaining -= clearCount
		}
	}
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
	a.setActiveChunkAfterReset()
}

func resetBigIntChunks(a *arenaChunks[big.Int], retainCap int) {
	// Keep big.Int word backing arrays across decoder reuse; integer decode
	// overwrites every allocated value before returning it.
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
	a.setActiveChunkAfterReset()
}

type arenaSlices[S any] struct {
	chunks [][]S
	pos    int
	// cur is the chunk holding pos and curBase the total length of the chunks
	// before it. Nothing behind pos is reused before reset, so alloc resumes
	// at cur: walking from the first chunk makes a decode quadratic in its
	// allocation count.
	cur     int
	curBase int
}

// alloc returns n elements whose capacity equals n, so an append by a caller
// of a decoded value reallocates instead of writing into the neighbouring
// live allocation.
func (a *arenaSlices[S]) alloc(n int) []S {
	if n == 0 {
		return make([]S, 0)
	}

	for a.cur < len(a.chunks) {
		chunk := a.chunks[a.cur]
		start := a.pos - a.curBase
		if start+n <= len(chunk) {
			a.pos += n
			return chunk[start : start+n : start+n]
		}
		a.curBase += len(chunk)
		a.pos = a.curBase
		a.cur++
	}

	size := dataDecodeChunkSize
	if n > size {
		size = n
	}
	chunk := make([]S, size)
	a.chunks = append(a.chunks, chunk)
	a.pos += n
	return chunk[:n:n]
}

func (a *arenaSlices[S]) reset(retainCap int) {
	retainedUsed := a.pos
	if len(a.chunks) > retainCap {
		maxRetained := 0
		for i := range retainCap {
			maxRetained += len(a.chunks[i])
		}
		if retainedUsed > maxRetained {
			retainedUsed = maxRetained
		}
	}
	remaining := retainedUsed
	for i := range a.chunks {
		if remaining <= 0 {
			break
		}
		chunk := a.chunks[i]
		if chunk == nil {
			continue
		}
		clearCount := len(chunk)
		if remaining < clearCount {
			clearCount = remaining
		}
		clear(chunk[:clearCount])
		remaining -= clearCount
	}
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.pos = 0
	a.cur = 0
	a.curBase = 0
}

// Decoder reuses arena-backed storage for decoded PlutusData values.
//
// Memory model: each Decode call bump-allocates from the internal arenas and
// NEVER frees on its own. Every value returned by Decode (and all of its
// nested children) stays valid until the next Reset, which is what lets a
// single program containing many Data constants share one Decoder. The flip
// side is that arena memory grows with the TOTAL number of bytes ever decoded,
// not with the size of the largest single input.
//
// Therefore a long-lived Decoder reused across many independent inputs (e.g.
// one Decoder pooled per connection) MUST call Reset between batches once it is
// done with the previously returned values; otherwise memory grows without
// bound (an OOM DoS on attacker-controlled input streams). If you do not need
// to retain decoded values across calls, prefer the package-level Decode, which
// does not retain anything between calls.
type Decoder struct {
	bigInts     arenaChunks[big.Int]
	integers    arenaChunks[Integer]
	byteStrings arenaChunks[ByteString]
	values      arenaChunks[Value]
	constrs     arenaChunks[Constr]
	lists       arenaChunks[List]
	maps        arenaChunks[Map]
	items       arenaSlices[PlutusData]
	pairs       arenaSlices[[2]PlutusData]
	bytes       arenaSlices[byte]
}

// NewDecoder returns a ready-to-use Decoder with empty internal pools.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Reset clears all internal arena pools so the Decoder can be reused; previously returned values become invalid.
func (d *Decoder) Reset() {
	resetBigIntChunks(&d.bigInts, dataDecodeRetainCap)
	d.integers.reset(dataDecodeRetainCap)
	d.byteStrings.reset(dataDecodeRetainCap)
	d.values.reset(dataDecodeRetainCap)
	d.constrs.reset(dataDecodeRetainCap)
	d.lists.reset(dataDecodeRetainCap)
	d.maps.reset(dataDecodeRetainCap)
	d.items.reset(dataSliceRetainCap)
	d.pairs.reset(dataSliceRetainCap)
	d.bytes.reset(dataSliceRetainCap)
}

// Decode decodes CBOR bytes into PlutusData; the Decoder is not safe for
// concurrent use. Decode does not free arena memory — see the Decoder type
// docs for the accumulate-until-Reset memory model and when callers must Reset.
// Like the package-level Decode, it has no depth or node limit and runs in
// time and memory linear in len(data).
func (d *Decoder) Decode(data []byte) (PlutusData, error) {
	return wrapDecodeError(decodeAll(d, data, false))
}

// DecodeWithValue is Decode that also admits the tag 1401 Value
// constructor (see the package-level DecodeWithValue).
func (d *Decoder) DecodeWithValue(data []byte) (PlutusData, error) {
	return wrapDecodeError(decodeAll(d, data, true))
}

// DecodePrefix is Decode for a Data constant inside a FLAT program: it
// decodes the item at the start of data and ignores any bytes after it (see
// the package-level DecodePrefix).
func (d *Decoder) DecodePrefix(data []byte) (PlutusData, error) {
	return wrapDecodeError(decodePrefix(d, data))
}

// The allocation helpers below take values from the arenas of d, or from the
// heap when d is nil (the package-level decoders).

func (d *Decoder) allocBigInt() *big.Int {
	if d == nil {
		return new(big.Int)
	}
	return d.bigInts.alloc()
}

func (d *Decoder) allocInteger(inner *big.Int) *Integer {
	if d == nil {
		return &Integer{Inner: inner}
	}
	integer := d.integers.alloc()
	integer.Inner = inner
	return integer
}

func (d *Decoder) allocByteString(inner []byte) *ByteString {
	if d == nil {
		return &ByteString{Inner: inner}
	}
	value := d.byteStrings.alloc()
	value.Inner = inner
	return value
}

func (d *Decoder) allocBytes(n int) []byte {
	if d == nil {
		return make([]byte, n)
	}
	return d.bytes.alloc(n)
}

func (d *Decoder) allocList() *List {
	if d == nil {
		return &List{}
	}
	return d.lists.alloc()
}

func (d *Decoder) allocConstr() *Constr {
	if d == nil {
		return &Constr{}
	}
	return d.constrs.alloc()
}

func (d *Decoder) allocMap() *Map {
	if d == nil {
		return &Map{}
	}
	return d.maps.alloc()
}

func (d *Decoder) allocValue() *Value {
	if d == nil {
		return &Value{}
	}
	return d.values.alloc()
}

func (d *Decoder) allocPairs(n int) [][2]PlutusData {
	if d == nil {
		return make([][2]PlutusData, n)
	}
	return d.pairs.alloc(n)
}

// copyItems returns a copy of items whose capacity equals its length, so an
// append by a caller of a decoded value reallocates instead of writing into a
// neighbouring arena allocation.
func (d *Decoder) copyItems(items []PlutusData) []PlutusData {
	var out []PlutusData
	if d == nil {
		out = make([]PlutusData, len(items))
	} else {
		out = d.items.alloc(len(items))
	}
	copy(out, items)
	return out
}
