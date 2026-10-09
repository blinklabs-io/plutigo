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
	"testing"

	"github.com/blinklabs-io/plutigo/syn"
)

func TestUnwrapStringPreservesEscapes(t *testing.T) {
	value := &Constant{&syn.String{Inner: `\u00A9\n\8712`}}

	out, err := unwrapString[syn.DeBruijn](value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `\u00A9\n\8712` {
		t.Fatalf("got %q want %q", out, `\u00A9\n\8712`)
	}
}
