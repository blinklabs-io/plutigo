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
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func evalTextAt(t *testing.T, src string, major uint) error {
	t.Helper()
	evalCtx := NewDefaultEvalContext(
		lang.LanguageVersionV3,
		ProtoVersion{Major: major},
	)
	_, _, err := EvaluateText(
		context.Background(),
		src,
		lang.LanguageVersionV3,
		evalCtx,
		DefaultExBudget,
	)
	return err
}

func TestCaseOnBuiltinValuesRequiresVanRossem(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"bool":    `(program 1.1.0 (case (con bool True) (con integer 1) (con integer 2)))`,
		"integer": `(program 1.1.0 (case (con integer 1) (con integer 10) (con integer 20)))`,
		"list":    `(program 1.1.0 (case (con (list integer) [7, 8]) (lam h (lam t h)) (con integer 0)))`,
		"pair": `(program 1.1.0 (case
  [ [ (builtin mkPairData) (con data (I 1)) ] (con data (I 2)) ]
  (lam first (lam second first))))`,
		"unit": `(program 1.1.0 (case (con unit ()) (con integer 3)))`,
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := evalTextAt(t, src, 10)
			if err == nil {
				t.Fatal("expected failure at protocol version 10")
			}
			if !strings.Contains(err.Error(), "before protocol version 11") {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := evalTextAt(t, src, 11); err != nil {
				t.Fatalf("protocol version 11: %v", err)
			}
		})
	}
}

func TestCaseOnConstrIsAvailableBeforeVanRossem(t *testing.T) {
	t.Parallel()

	const src = `(program 1.1.0 (case (constr 0 (con integer 5)) (lam x x)))`
	if err := evalTextAt(t, src, 10); err != nil {
		t.Fatalf("case on constr at protocol version 10: %v", err)
	}
}

func TestWriteBitsInputLimit(t *testing.T) {
	t.Parallel()

	program := func(length int) string {
		return fmt.Sprintf(
			`(program 1.0.0 [ [ [ (builtin writeBits) `+
				`[ [ (builtin replicateByte) (con integer %d) ] (con integer 0) ] ] `+
				`(con (list integer) [0]) ] (con bool True) ])`,
			length,
		)
	}
	if err := evalTextAt(t, program(writeBitsMaximumInputLength), 11); err != nil {
		t.Fatalf("maximum input length: %v", err)
	}
	err := evalTextAt(t, program(writeBitsMaximumInputLength+1), 11)
	if err == nil || !strings.Contains(err.Error(), "input too long") {
		t.Fatalf("over-limit input at protocol version 11: %v", err)
	}
	if err := evalTextAt(t, program(writeBitsMaximumInputLength+1), 10); err != nil {
		t.Fatalf("over-limit input before van Rossem: %v", err)
	}
}

func TestCaseOnBuiltinDataRequiresVanRossem(t *testing.T) {
	t.Parallel()

	// constrData yields a built-in Data value; casing on Data stays
	// unsupported after the gate lifts, so only the earlier protocol's error
	// is distinguishable.
	const src = `(program 1.1.0
  (case [ [ (builtin constrData) (con integer 0) ] (con (list data) []) ]
    (con integer 1)))`
	err := evalTextAt(t, src, 10)
	if err == nil || !strings.Contains(err.Error(), "before protocol version 11") {
		t.Fatalf("protocol version 10: %v", err)
	}
}

func TestCaseOnByteStringKeepsItsOwnError(t *testing.T) {
	t.Parallel()

	const src = `(program 1.1.0 (case (con bytestring #00) (con integer 1)))`
	for _, major := range []uint{10, 11} {
		err := evalTextAt(t, src, major)
		if err == nil || !strings.Contains(err.Error(), "bytestring") {
			t.Fatalf("protocol version %d: %v", major, err)
		}
	}
}

func TestCaseWithoutProtocolVersionIsUnrestricted(t *testing.T) {
	t.Parallel()

	program, err := syn.Parse(
		`(program 1.1.0 (case (con bool True) (con integer 1) (con integer 2)))`,
	)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	deBruijn, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	// testEvalContext leaves the protocol major unset.
	machine := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
	if _, err := machine.Run(deBruijn.Term); err != nil {
		t.Fatalf("protocol major 0: %v", err)
	}
}
