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

func init() {
	lazydata.Hooks.NewList = func(src any) any {
		return &List{lazy: src.(lazydata.Source[PlutusData])}
	}
	lazydata.Hooks.NewMap = func(src any) any {
		return &Map{lazy: src.(lazydata.Source[[2]PlutusData])}
	}
	lazydata.Hooks.NewConstr = func(tag *big.Int, src any) any {
		return &Constr{Tag: tag, lazy: src.(lazydata.Source[PlutusData])}
	}
	lazydata.Hooks.SourceOf = func(pd any) any {
		switch v := pd.(type) {
		case *List:
			if v != nil && v.lazy != nil {
				return v.lazy
			}
		case *Map:
			if v != nil && v.lazy != nil {
				return v.lazy
			}
		case *Constr:
			if v != nil && v.lazy != nil {
				return v.lazy
			}
		}
		return nil
	}
}
