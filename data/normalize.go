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

// Normalize returns a deep copy of pd with every Constr, Map, and List node's
// definite/indefinite-length CBOR encoding reset to the package default,
// discarding whatever encoding style the value originally carried from
// Decode. A Value's inner Map is normalized the same way.
//
// The default is per type, and Map is not like Constr and List: Constr and
// List encode indefinite when non-empty and definite when empty, while Map
// always encodes definite, matching Haskell's canonical CBOR. See MarshalCBOR
// on each type. So normalizing a map decoded from indefinite-length bytes
// moves it to definite, the opposite direction from the others.
//
// Decode preserves the original wire's definite/indefinite-length choice on
// each node so that re-encoding a decoded value reproduces the original
// bytes. That fidelity is correct when verifying something against its own
// original bytes (e.g. a script-data hash), but cardano-ledger's reference
// implementation does not carry that fidelity through when it builds values
// for a Plutus script to observe (redeemer data, datums): it always
// constructs them fresh, which is equivalent to always using the package
// default encoding. A value decoded from on-chain bytes and then handed to a
// script unchanged can therefore serialise to different bytes than the
// reference implementation's for the same semantic value whenever the
// original encoder chose definite-length arrays, and a script that hashes or
// compares serialised bytes (via SerialiseData) sees that difference.
// Callers building script-visible values from decoded data (redeemer data,
// inline or witness datums) should call Normalize first.
//
// Every container node is rebuilt, so resetting the encoding never writes
// through to the input. Integer and ByteString leaves are returned as-is:
// they hold no encoding state, and this package treats their contents as
// read-only. Typed-nil container pointers are returned unchanged. Container
// nodes held in their value form rather than as a pointer are normalized too,
// and come back as pointers, the same conversion Clone makes.
//
// Normalize does not recurse, so it handles values of any nesting depth.
func Normalize(pd PlutusData) PlutusData {
	return copyData(pd, true)
}

// NormalizeShared returns pd itself when pd is already in the form Normalize
// produces, and Normalize(pd) otherwise. Either way the result encodes to the
// bytes Normalize(pd) encodes to and holds every container as a pointer, as
// Normalize's result does.
//
// A node is in that form when it is held as a pointer and its encoding flag
// is unset or already selects the package default for that node: indefinite
// length for a non-empty Constr or List, definite for a Map. An empty Constr
// or List encodes definite whatever its flag. Most on-chain data is encoded
// that way, so for it NormalizeShared allocates nothing.
//
// The result may share nodes with pd, so callers must treat both as
// read-only. Callers that need a value independent of pd use Normalize.
//
// NormalizeShared does not recurse, so it handles values of any nesting
// depth.
func NormalizeShared(pd PlutusData) PlutusData {
	if isNormalized(pd) {
		return pd
	}
	return Normalize(pd)
}

// normalizeFrame is a container whose children have not all been visited.
// Exactly one of items and pairs is set; i indexes the flattened children.
type normalizeFrame struct {
	items []PlutusData
	pairs [][2]PlutusData
	i     int
}

func (f *normalizeFrame) next() (PlutusData, bool) {
	if f.pairs != nil {
		if f.i >= 2*len(f.pairs) {
			return nil, false
		}
		child := f.pairs[f.i/2][f.i%2]
		f.i++
		return child, true
	}
	if f.i >= len(f.items) {
		return nil, false
	}
	child := f.items[f.i]
	f.i++
	return child, true
}

// isNormalized reports whether every node of pd is already in the form
// Normalize produces. It keeps one frame per level of nesting rather than one
// entry per pending child, so it allocates only when the nesting is deeper
// than 32 levels, however wide the containers are.
func isNormalized(pd PlutusData) bool {
	var buf [32]normalizeFrame
	stack := buf[:0]
	node := pd
	for {
		switch v := node.(type) {
		case *Constr:
			if v != nil {
				if len(v.Fields) > 0 && v.useIndef != nil && !*v.useIndef {
					return false
				}
				stack = append(stack, normalizeFrame{items: v.Fields})
			}
		case *List:
			if v != nil {
				if len(v.Items) > 0 && v.useIndef != nil && !*v.useIndef {
					return false
				}
				stack = append(stack, normalizeFrame{items: v.Items})
			}
		case *Map:
			if v != nil {
				if !mapIsNormalized(v) {
					return false
				}
				stack = append(stack, normalizeFrame{pairs: v.Pairs})
			}
		case *Value:
			if v != nil && v.Inner != nil {
				if !mapIsNormalized(v.Inner) {
					return false
				}
				stack = append(stack, normalizeFrame{pairs: v.Inner.Pairs})
			}
		case Constr, List, Map, Value:
			// Normalize returns containers as pointers.
			return false
		}
		for {
			if len(stack) == 0 {
				return true
			}
			child, ok := stack[len(stack)-1].next()
			if ok {
				node = child
				break
			}
			stack = stack[:len(stack)-1]
		}
	}
}

func mapIsNormalized(m *Map) bool {
	return m.useIndef == nil || !*m.useIndef
}
