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

import "github.com/blinklabs-io/plutigo/internal/lazydata"

// itemsHeader and pairsHeader hold what most lists, maps and constructors do
// not need, behind one pointer so the containers stay small: an explicit CBOR
// length form, and for a container the evaluator built over a shared list,
// the source of its elements. Containers with only a length form share the
// static headers below, so only a lazy container has a header of its own,
// allocated together with it. They are separate concrete types, not one
// generic one, so the accessors that check them stay inlinable.
type itemsHeader struct {
	src   lazydata.Source[PlutusData]
	indef *bool // nil when the length form is unset
}

type pairsHeader struct {
	src   lazydata.Source[[2]PlutusData]
	indef *bool // nil when the length form is unset
}

var (
	itemsHdrDef   = &itemsHeader{indef: &sharedUseIndefFalse}
	itemsHdrIndef = &itemsHeader{indef: &sharedUseIndefTrue}
	pairsHdrDef   = &pairsHeader{indef: &sharedUseIndefFalse}
	pairsHdrIndef = &pairsHeader{indef: &sharedUseIndefTrue}
)

// itemsHdr returns the shared header of a list or constructor with the given
// length form, which may be unset.
func itemsHdr(indef *bool) *itemsHeader {
	switch {
	case indef == nil:
		return nil
	case *indef:
		return itemsHdrIndef
	default:
		return itemsHdrDef
	}
}

// pairsHdr returns the shared header of a map with the given length form,
// which may be unset.
func pairsHdr(indef *bool) *pairsHeader {
	switch {
	case indef == nil:
		return nil
	case *indef:
		return pairsHdrIndef
	default:
		return pairsHdrDef
	}
}

func (h *itemsHeader) useIndef() *bool {
	if h == nil {
		return nil
	}
	return h.indef
}

func (h *pairsHeader) useIndef() *bool {
	if h == nil {
		return nil
	}
	return h.indef
}

func (h *itemsHeader) source() lazydata.Source[PlutusData] {
	if h == nil {
		return nil
	}
	return h.src
}

func (h *pairsHeader) source() lazydata.Source[[2]PlutusData] {
	if h == nil {
		return nil
	}
	return h.src
}
