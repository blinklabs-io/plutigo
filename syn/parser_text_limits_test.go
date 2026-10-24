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
	"errors"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/syn/lex"
)

func nestedPairProgram(depth int) string {
	var b strings.Builder
	b.WriteString("(program 1.0.0 (con ")
	for range depth {
		b.WriteString("(pair unit ")
	}
	b.WriteString("unit")
	b.WriteString(strings.Repeat(")", depth))
	b.WriteString(" ")
	for range depth {
		b.WriteString("((),")
	}
	b.WriteString("()")
	b.WriteString(strings.Repeat(")", depth))
	b.WriteString("))")
	return b.String()
}

// TestNestedPairConstantParses parses a deeply nested pair constant, which
// the parser used to type check once per level against the whole subtree.
func TestNestedPairConstantParses(t *testing.T) {
	const depth = 8000
	prog, err := Parse(nestedPairProgram(depth))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := prog.Term.(*Constant)
	if !ok {
		t.Fatalf("term is %T, want *Constant", prog.Term)
	}
	checkPairChain(t, c.Con, depth)
}

func TestNestedPairConstantTypeMismatchStillRejected(t *testing.T) {
	for _, in := range []string{
		"(program 1.0.0 (con (pair unit (pair integer unit)) ((),(true,()))))",
		"(program 1.0.0 (con (pair unit (pair integer unit)) ((),(1,1))))",
		"(program 1.0.0 (con (list (pair integer bool)) [(1,True),(2,3)]))",
	} {
		if _, err := Parse(in); err == nil {
			t.Errorf("expected error for %s", in)
		}
	}
}

func TestIntegerLiteralTooLongTypedError(t *testing.T) {
	digits := strings.Repeat("7", lex.MaxIntegerDigits+1)
	for name, in := range map[string]string{
		"integer constant": "(program 1.0.0 (con integer " + digits + "))",
		"data constr tag":  "(program 1.0.0 (con data (Constr " + digits + " [])))",
		"data I":           "(program 1.0.0 (con data (I " + digits + ")))",
	} {
		_, err := Parse(in)
		if !errors.Is(err, lex.ErrIntegerTooLong) {
			t.Errorf("%s: err = %v, want ErrIntegerTooLong", name, err)
		}
	}
	ok := "(program 1.0.0 (con integer " + strings.Repeat("7", lex.MaxIntegerDigits) + "))"
	if _, err := Parse(ok); err != nil {
		t.Errorf("at-limit literal rejected: %v", err)
	}
}
