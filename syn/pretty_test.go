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

package syn

import (
	"math/big"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

func TestPrettyTerm(t *testing.T) {
	// Test with a simple integer constant
	term := &Constant{Con: &Integer{Inner: big.NewInt(42)}}

	result := PrettyTerm[DeBruijn](term)

	if result == "" {
		t.Errorf("PrettyTerm returned empty string")
	}

	// Should contain "42"
	if !strings.Contains(result, "42") {
		t.Errorf("PrettyTerm output does not contain '42': %s", result)
	}
}

func TestPrettyTermComplex(t *testing.T) {
	// Test with a lambda
	term := &Lambda[DeBruijn]{
		Body: &Constant{Con: &Integer{Inner: big.NewInt(1)}},
	}

	result := PrettyTerm[DeBruijn](term)

	if result == "" {
		t.Errorf("PrettyTerm returned empty string")
	}

	// Should contain lambda syntax
	if !strings.Contains(result, "lam") {
		t.Errorf("PrettyTerm output does not contain 'lam': %s", result)
	}
}

func TestPrettyPrinterReset(t *testing.T) {
	pp := NewPrettyPrinter(2)
	pp.write("hello")
	pp.increaseIndent()

	pp.Reset()

	if got := pp.builder.String(); got != "" {
		t.Errorf("expected empty builder after Reset, got %q", got)
	}

	if pp.indent != 0 {
		t.Errorf("expected indent 0 after Reset, got %d", pp.indent)
	}
}

func TestPrettyInvalidValueIsNotSilentlyRewritten(t *testing.T) {
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersionV4,
		Term: &Constant{Con: &Data{Inner: &data.Value{
			Inner: &data.Map{Pairs: [][2]data.PlutusData{{
				data.NewByteString([]byte{0xaa}),
				data.NewInteger(big.NewInt(1)),
			}}},
		}}},
	}

	pretty := Pretty(program)
	if !strings.Contains(pretty, "Value{invalid:") {
		t.Fatalf("invalid Value pretty output = %q, want explicit invalid marker", pretty)
	}
}
