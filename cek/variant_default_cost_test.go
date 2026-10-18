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
	"math"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func consumedByDefaultContext(
	t *testing.T,
	src string,
	version lang.LanguageVersion,
	protoMajor uint,
) ExBudget {
	t.Helper()
	named, err := syn.Parse(src)
	if err != nil {
		t.Fatalf("parse program: %v", err)
	}
	program, err := syn.NameToDeBruijn(named)
	if err != nil {
		t.Fatalf("convert program: %v", err)
	}
	machine := NewMachine[syn.DeBruijn](
		version,
		200,
		NewDefaultEvalContext(version, ProtoVersion{Major: protoMajor}),
	)
	initial := ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
	machine.ExBudget = initial
	if _, err := machine.Run(program.Term); err != nil {
		t.Fatalf("run program: %v", err)
	}
	return initial.Sub(&machine.ExBudget)
}

// Semantics variant A has its own default machine step cost and builtin CPU
// costs; the later variants' defaults must not stand in for them.
func TestDefaultEvalContextUsesVariantACosts(t *testing.T) {
	t.Parallel()

	const constant = `(program 1.0.0 (con integer 1))`
	const add = `(program 1.0.0
		[ [ (builtin addInteger) (con integer 1) ] (con integer 1) ])`
	for _, tc := range []struct {
		name     string
		src      string
		version  lang.LanguageVersion
		protocol uint
		want     ExBudget
	}{
		// startup 100 + one constant step.
		{"V1 constant before Chang", constant, lang.LanguageVersionV1, 8, ExBudget{Cpu: 23100, Mem: 200}},
		{"V2 constant before Chang", constant, lang.LanguageVersionV2, 8, ExBudget{Cpu: 23100, Mem: 200}},
		{"V1 constant at Chang", constant, lang.LanguageVersionV1, 9, ExBudget{Cpu: 16100, Mem: 200}},
		{"V2 constant at Chang", constant, lang.LanguageVersionV2, 9, ExBudget{Cpu: 16100, Mem: 200}},
		// startup 100 + five steps of 23000, plus addInteger at
		// 205665 + 812 * 1 CPU and 2 memory.
		{"V1 addInteger before Chang", add, lang.LanguageVersionV1, 8, ExBudget{Cpu: 321577, Mem: 602}},
		{"V2 addInteger before Chang", add, lang.LanguageVersionV2, 8, ExBudget{Cpu: 321577, Mem: 602}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := consumedByDefaultContext(t, tc.src, tc.version, tc.protocol); got != tc.want {
				t.Fatalf("consumed budget = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDefaultVariantACostModelIsComplete(t *testing.T) {
	t.Parallel()
	for _, version := range []lang.LanguageVersion{
		lang.LanguageVersionV1,
		lang.LanguageVersionV2,
	} {
		if _, err := defaultVariantACostModel(version); err != nil {
			t.Fatalf("variant A cost model for %v: %v", version, err)
		}
	}
	if _, err := defaultVariantACostModel(lang.LanguageVersionV3); err == nil {
		t.Fatal("PlutusV3 has no variant A cost model")
	}
}
