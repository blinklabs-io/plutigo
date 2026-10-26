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
	"bytes"

	"github.com/fxamacker/cbor/v2"
)

// encMode is cached at package level to avoid recreation on every encode call
var encMode cbor.EncMode

func init() {
	opts := cbor.EncOptions{
		// NOTE: set any additional encoder options here
	}
	var err error
	encMode, err = opts.EncMode()
	if err != nil {
		panic("failed to initialize CBOR encoder: " + err.Error())
	}
}

// Encode encodes a PlutusData value into CBOR bytes.
//
// It writes containers itself rather than through the cbor library, whose
// encoder re-validates every MarshalCBOR result with a nesting limit of
// 65535 levels: PlutusData may be nested far deeper (a decoded transaction
// datum up to its size in bytes, a value built by a script up to its
// budget).
func Encode(pd PlutusData) ([]byte, error) {
	return marshalPlutusDataCBOR(pd)
}

// cborMarshal acts like cbor.Marshal but allows us to set our own encoder options
func cborMarshal(data any) ([]byte, error) {
	em := encMode
	if em == nil {
		panic("CBOR encoder not initialized")
	}
	var buf bytes.Buffer
	enc := em.NewEncoder(&buf)
	err := enc.Encode(data)
	return buf.Bytes(), err
}
