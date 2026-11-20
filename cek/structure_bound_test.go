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
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// checkStructureBound decodes flat as RunFlat does and requires the
// decoder's structure bound to cover the walk RunFlat skips when the bound
// is small: the nodes the walk counts and the deepest term it reaches.
func checkStructureBound(t *testing.T, name string, flat []byte) {
	t.Helper()
	decoder := syn.NewDeBruijnDecoder()
	program, err := decoder.DecodeBorrowedWithScopeCheck(flat, true)
	if err != nil {
		return // RunFlat rejects it before any walk.
	}
	bound := decoder.StructureBound()
	var usage termStructureUsage
	if err := validateTermStructureUsage[syn.DeBruijn](
		context.Background(), false, program.Term, nil, true, &usage,
	); err != nil {
		if flatRunSkipsValidation(len(flat), bound) {
			t.Errorf("%s: walk rejects a program RunFlat would not walk (bound %d): %v", name, bound, err)
		}
		return
	}
	if int64(usage.work) > bound {
		t.Errorf("%s: walk counted %d nodes, structure bound %d", name, usage.work, bound)
	}
	if int64(usage.maxDepth) > bound {
		t.Errorf("%s: walk reached depth %d, structure bound %d", name, usage.maxDepth, bound)
	}
}

func encodeProgramText(t *testing.T, text string) []byte {
	t.Helper()
	program, err := syn.Parse(text)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	db, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("to DeBruijn: %v", err)
	}
	flat, err := syn.Encode(db)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return flat
}

// TestStructureBoundCoversWalk checks the bound on constant-heavy programs,
// whose structure per input bit is densest, and on every program the
// repository holds.
func TestStructureBoundCoversWalk(t *testing.T) {
	repeat := func(item string, n int) string {
		return strings.TrimSuffix(strings.Repeat(item+", ", n), ", ")
	}
	synthetic := map[string]string{
		"zero-width pair list": "(program 1.1.0 (con (list (pair unit unit)) [" +
			repeat("((), ())", 60_000) + "]))",
		"nested zero-width pairs": "(program 1.1.0 (con (list (pair (pair unit unit) (pair unit (pair unit unit)))) [" +
			repeat("(((), ()), ((), ((), ())))", 20_000) + "]))",
		"unit list":    "(program 1.1.0 (con (list unit) [" + repeat("()", 80_000) + "]))",
		"bool list":    "(program 1.1.0 (con (list bool) [" + repeat("True", 80_000) + "]))",
		"nested lists": "(program 1.1.0 (con (list (list (list integer))) [" + repeat("[[1, 2], [], [3]]", 5_000) + "]))",
		"pair of lists": "(program 1.1.0 (con (pair (list integer) (list bytestring)) ([" +
			repeat("7", 10_000) + "], [" + repeat("#ab", 10_000) + "])))",
		"array":     "(program 1.1.0 (con (array integer) [" + repeat("1", 20_000) + "]))",
		"data list": "(program 1.1.0 (con (list data) [" + repeat("Constr 0 [I 1, B #00, List [I 2], Map [(I 3, I 4)]]", 5_000) + "]))",
		"large data": "(program 1.1.0 (con data (List [" +
			repeat("Constr 1 [List [], Map [], I -5, B #deadbeef]", 20_000) + "])))",
		"bytestring": "(program 1.1.0 (con bytestring #" + strings.Repeat("5a", 40_000) + "))",
		"string":     "(program 1.1.0 (con string \"" + strings.Repeat("x", 40_000) + "\"))",
		"deep lambdas": "(program 1.1.0 " + strings.Repeat("(lam x ", 20_000) + "(con unit ())" +
			strings.Repeat(")", 20_000) + ")",
		"wide constr": "(program 1.1.0 (constr 0 " + strings.Repeat("(con integer 1) ", 30_000) + "))",
	}
	for name, text := range synthetic {
		t.Run(name, func(t *testing.T) {
			checkStructureBound(t, name, encodeProgramText(t, text))
		})
	}

	t.Run("bench programs", func(t *testing.T) {
		paths, err := filepath.Glob("../tests/bench/*.flat")
		if err != nil || len(paths) == 0 {
			t.Fatalf("no bench programs: %v", err)
		}
		for _, path := range paths {
			flat, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			checkStructureBound(t, filepath.Base(path), flat)
		}
	})

	t.Run("conformance programs", func(t *testing.T) {
		checked := 0
		err := filepath.WalkDir("../tests/conformance", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".uplc") {
				return err
			}
			text, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			program, err := syn.Parse(string(text))
			if err != nil {
				return nil
			}
			db, err := syn.NameToDeBruijn(program)
			if err != nil {
				return nil
			}
			flat, err := syn.Encode(db)
			if err != nil {
				return nil
			}
			checkStructureBound(t, path, flat)
			checked++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if checked < 500 {
			t.Fatalf("checked %d conformance programs, want at least 500", checked)
		}
	})

	t.Run("mainnet programs", func(t *testing.T) {
		paths, err := filepath.Glob("../replay/testdata/mainnet-*.json.gz")
		if err != nil || len(paths) == 0 {
			t.Fatalf("no mainnet corpus: %v", err)
		}
		for _, path := range paths {
			for name, flat := range appliedMainnetPrograms(t, path) {
				checkStructureBound(t, name, flat)
			}
		}
	})
}

// appliedMainnetPrograms returns each replay case's validator applied to its
// ledger arguments as Data constants, FLAT-encoded: the programs
// RunFlat evaluates in the benchmark's mainnet corpus.
func appliedMainnetPrograms(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	var corpus struct {
		Cases []struct {
			ID        string   `json:"id"`
			Program   string   `json:"flat_program_hex"`
			Arguments []string `json:"arguments_cbor_hex"`
		} `json:"cases"`
	}
	if err := json.NewDecoder(gz).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte, len(corpus.Cases))
	for _, c := range corpus.Cases {
		flat, err := applyDataArguments(c.Program, c.Arguments)
		if err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		out[c.ID] = flat
	}
	return out
}

func applyDataArguments(programHex string, argumentsHex []string) ([]byte, error) {
	raw, err := hex.DecodeString(programHex)
	if err != nil {
		return nil, err
	}
	program, err := syn.Decode[syn.DeBruijn](raw)
	if err != nil {
		return nil, err
	}
	term := program.Term
	for i, argumentHex := range argumentsHex {
		argument, err := hex.DecodeString(argumentHex)
		if err != nil {
			return nil, err
		}
		value, err := data.Decode(argument)
		if err != nil {
			return nil, fmt.Errorf("argument %d: %w", i, err)
		}
		term = &syn.Apply[syn.DeBruijn]{Function: term, Argument: &syn.Constant{Con: &syn.Data{Inner: value}}}
	}
	if program.Version == (lang.LanguageVersion{}) {
		return nil, errors.New("program has no version")
	}
	return syn.Encode(&syn.Program[syn.DeBruijn]{Version: program.Version, Term: term})
}
