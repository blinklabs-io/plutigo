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
	"errors"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestRunRecoversPanicAndMachineIsReusable(t *testing.T) {
	const src = "(program 1.1.0 [(builtin sha2_256) (con bytestring #00)])"
	program, err := syn.Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn failed: %v", err)
	}
	term := dbProgram.Term

	type runFn = func(*Machine[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error)
	runners := map[string]runFn{
		"Run": func(m *Machine[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error) {
			return m.Run(term)
		},
		"RunContext": func(m *Machine[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error) {
			return m.RunContext(context.Background(), term)
		},
		"RunContextWithTrustedDataArguments": func(m *Machine[syn.DeBruijn]) (syn.Term[syn.DeBruijn], error) {
			return m.RunContextWithTrustedDataArguments(
				context.Background(),
				term,
			)
		},
	}
	for name, run := range runners {
		t.Run(name, func(t *testing.T) {
			m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
			good := m.builtins
			// Private copy of the dispatch table with a panicking builtin.
			bad := *good
			bad[builtin.Sha2_256] = func(
				*Machine[syn.DeBruijn],
				*Builtin[syn.DeBruijn],
			) (Value[syn.DeBruijn], error) {
				panic("injected failure")
			}

			// A successful run first, so reuse follows a warm machine.
			if _, err := run(m); err != nil {
				t.Fatalf("baseline run failed: %v", err)
			}

			m.builtins = &bad
			result, err := run(m)
			if result != nil {
				t.Fatalf("expected nil result, got %v", result)
			}
			var internal *InternalError
			if !errors.As(err, &internal) {
				t.Fatalf("expected *InternalError, got %T: %v", err, err)
			}
			if internal.Code != ErrCodeInternalError ||
				!strings.Contains(internal.Message, "injected failure") {
				t.Fatalf("unexpected error: %v", internal)
			}
			if m.activeContext != nil || m.frameStackUsed != 0 {
				t.Fatal("machine state not reset after panic")
			}

			m.builtins = good
			result, err = run(m)
			if err != nil || result == nil {
				t.Fatalf("machine not reusable after panic: %v", err)
			}
		})
	}
}
