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
)

func TestDecodeRejectsConstrAndCaseBeforeVersion110(t *testing.T) {
	t.Parallel()

	for name, term := range map[string]Term[DeBruijn]{
		"constr": &Constr[DeBruijn]{Tag: 0},
		"case": &Case[DeBruijn]{
			Constr:   &Constant{Con: &Integer{Inner: big.NewInt(1)}},
			Branches: nil,
		},
	} {
		for version, wantErr := range map[string]bool{"1.0.0": true, "1.1.0": false} {
			program := &Program[DeBruijn]{Term: term}
			program.Version = uplcVersion100
			if !wantErr {
				program.Version = uplcVersion110
			}
			encoded, err := Encode(program)
			if err != nil {
				t.Fatalf("%s %s: encode: %v", name, version, err)
			}
			_, err = Decode[DeBruijn](encoded)
			if wantErr != (err != nil) {
				t.Fatalf("%s %s: Decode error = %v, want error %t", name, version, err, wantErr)
			}
			if wantErr && !strings.Contains(err.Error(), name+" is not available in UPLC version 1.0.0") {
				t.Fatalf("%s %s: error = %v", name, version, err)
			}
		}
	}
}

// Decoding accepts an open term: a stored reference script can be one, and
// only evaluating it is refused.
func TestDecodeAcceptsOpenTerm(t *testing.T) {
	t.Parallel()

	encoded, err := Encode(&Program[DeBruijn]{
		Version: uplcVersion100,
		Term:    &Lambda[DeBruijn]{Body: &Var[DeBruijn]{Name: 2}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := Decode[DeBruijn](encoded); err != nil {
		t.Fatalf("Decode of an open term: %v", err)
	}
}
