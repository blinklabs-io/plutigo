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

package tests

import (
	"context"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// The node path: a program decoded once (and cached by the ledger), then
// evaluated as a ledger script for every transaction. The corpus programs are
// fully applied, so they are run with no trusted data arguments. PlutusV3 at
// protocol version 11 matches the Lantr benchmark's parameters.
var nodePathContext = syn.ProgramContext{
	LedgerLanguage: lang.LanguageVersionV3,
	ProtocolMajor:  11,
}

// BenchmarkNodeScriptContext evaluates a cached program with
// Machine.RunScriptContext, which walks the program before every run.
func BenchmarkNodeScriptContext(b *testing.B) {
	evalContext := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 11})
	forEachBenchFile(b, func(b *testing.B, name string, content []byte) {
		program, err := syn.DecodeDeBruijnWithContext(content, nodePathContext)
		if err != nil {
			b.Fatalf("decode error: %v", err)
		}
		b.Run(name, func(b *testing.B) {
			machine := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, evalContext)
			for b.Loop() {
				machine.ExBudget = cek.DefaultExBudget
				if _, err := machine.RunScriptContext(context.Background(), program.Term); err != nil {
					b.Fatalf("eval error: %v", err)
				}
			}
		})
	})
}

// BenchmarkNodePrepared evaluates a cached PreparedProgram with
// cek.RunPreparedScriptContext, which skips the per-run walk.
func BenchmarkNodePrepared(b *testing.B) {
	evalContext := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 11})
	forEachBenchFile(b, func(b *testing.B, name string, content []byte) {
		prepared, err := cek.PrepareProgram(content, nodePathContext)
		if err != nil {
			b.Fatalf("prepare error: %v", err)
		}
		b.Run(name, func(b *testing.B) {
			machine := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, evalContext)
			for b.Loop() {
				machine.ExBudget = cek.DefaultExBudget
				if _, err := cek.RunPreparedScriptContext(context.Background(), machine, prepared); err != nil {
					b.Fatalf("eval error: %v", err)
				}
			}
		})
	})
}
