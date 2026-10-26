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
