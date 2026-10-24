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
	"encoding/hex"
	"math"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func runFlatHex(t *testing.T, input string, proto uint) (string, ExBudget, error) {
	t.Helper()
	b, err := hex.DecodeString(input)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	ctx := syn.ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: proto}
	program, err := syn.DecodeDeBruijnWithContext(b, ctx)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		0,
		NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: proto}),
	)
	start := ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
	m.ExBudget = start
	result, err := m.Run(program.Term)
	if err != nil {
		return "", start.Sub(&m.ExBudget), err
	}
	return syn.PrettyTerm[syn.DeBruijn](result), start.Sub(&m.ExBudget), nil
}

// TestSerialiseDataIsCanonical pins PlutusCore.Data.encodeData: whatever
// encoding a Data constant was decoded from, serialiseData writes maps
// definite and non-empty lists and constructor fields indefinite.
func TestSerialiseDataIsCanonical(t *testing.T) {
	// [(builtin serialiseData) (con data <d8798101>)]: Constr 0 [I 1] with
	// a definite-length field list.
	got, _, err := runFlatHex(t, "0100003766980104d87981010001", 10)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := "(con bytestring #d8799f01ff)"; got != want {
		t.Fatalf("serialiseData = %s, want %s", got, want)
	}
	for _, tc := range []struct{ data, want string }{
		{"(List [I 1])", "#9f01ff"},
		{"(Map [(I 1, I 2)])", "#a10102"},
		{"(List [])", "#80"},
		{"(Constr 0 [])", "#d87980"},
	} {
		got, err := runTextProgram(t, "(program 1.0.0 [(builtin serialiseData) (con data "+tc.data+")])", 10)
		if err != nil || got != "(con bytestring "+tc.want+")" {
			t.Fatalf("serialiseData %s = %s, %v; want %s", tc.data, got, err, tc.want)
		}
	}
}

// TestFreeHugeIndexInUntakenBranch: an index above MaxInt decodes (the
// reference reads any Word64) and, inside a case branch that is not taken,
// is never scope-checked before protocol 12.
func TestFreeHugeIndexInUntakenBranch(t *testing.T) {
	got, _, err := runFlatHex(t, "01010098005200284040404040404040400081", 10)
	if err != nil || got != "(con integer 1)" {
		t.Fatalf("result %q, %v; want (con integer 1)", got, err)
	}
}

// TestValueDataCostsTotalSize: valueData is costed by the argument's
// Value.totalSize (ValueTotalSize). The expected budget is computed by hand
// from the reference builtinCostModelC.json and machine costs: startup
// 100 cpu / 100 mem, three steps (apply, builtin, constant) at 16000 / 100
// each, and valueData's linear_in_x cost at x = totalSize = 4: cpu
// 1000 + 38159*4 = 153636, mem 2 + 22*4 = 90. Total 201736 cpu, 490 mem.
func TestValueDataCostsTotalSize(t *testing.T) {
	src := "(program 1.1.0 [(builtin valueData) (con value [(#aa, [(#01, 1), (#02, 1)]), (#bb, [(#01, 1), (#02, 1)])])])"
	named, err := syn.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	program, err := syn.NameToDeBruijn(named)
	if err != nil {
		t.Fatal(err)
	}
	m := NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		0,
		NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: 11}),
	)
	start := ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
	m.ExBudget = start
	if _, err := m.Run(program.Term); err != nil {
		t.Fatal(err)
	}
	if got, want := start.Sub(&m.ExBudget), (ExBudget{Cpu: 201736, Mem: 490}); got != want {
		t.Fatalf("valueData budget %+v, want %+v", got, want)
	}
}
