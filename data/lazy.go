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

	"github.com/blinklabs-io/plutigo/internal/lazydata"
)

// WrapList returns a List over items without copying them. The caller must
// not modify items afterwards.
func WrapList(items []PlutusData) *List {
	return &List{items: items}
}

// WrapMap returns a Map over pairs without copying them. The caller must not
// modify pairs afterwards.
func WrapMap(pairs [][2]PlutusData) *Map {
	return &Map{pairs: pairs}
}

// WrapConstr returns a Constr with the given tag over fields without copying
// either. The caller must not modify them afterwards.
func WrapConstr(tag *big.Int, fields []PlutusData) *Constr {
	return &Constr{Tag: tag, fields: fields}
}

// A lazy container is allocated together with its header.
type (
	lazyList struct {
		list List
		hdr  itemsHeader
	}
	lazyMap struct {
		m   Map
		hdr pairsHeader
	}
	lazyConstr struct {
		constr Constr
		hdr    itemsHeader
	}
)

func init() {
	lazydata.Hooks.NewList = func(src any) any {
		a := &lazyList{}
		a.hdr.src = src.(lazydata.Source[PlutusData])
		a.list.hdr = &a.hdr
		return &a.list
	}
	lazydata.Hooks.NewMap = func(src any) any {
		a := &lazyMap{}
		a.hdr.src = src.(lazydata.Source[[2]PlutusData])
		a.m.hdr = &a.hdr
		return &a.m
	}
	lazydata.Hooks.NewConstr = func(tag *big.Int, src any) any {
		a := &lazyConstr{}
		a.hdr.src = src.(lazydata.Source[PlutusData])
		a.constr.Tag = tag
		a.constr.hdr = &a.hdr
		return &a.constr
	}
	lazydata.Hooks.SourceOf = func(pd any) any {
		switch v := pd.(type) {
		case *List:
			if v != nil {
				if src := v.hdr.source(); src != nil {
					return src
				}
			}
		case *Map:
			if v != nil {
				if src := v.hdr.source(); src != nil {
					return src
				}
			}
		case *Constr:
			if v != nil {
				if src := v.hdr.source(); src != nil {
					return src
				}
			}
		}
		return nil
	}
}
