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

package cek_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"
	"runtime/debug"
	"slices"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// nodeDataLimits records what a real cardano-node did with scripts whose
// PlutusData constants are deeper than plutigo's former decode depth limit
// (16384 levels), and with scripts that build nested values at run time (see
// the provenance field of the fixture). Each constant script is regenerated
// here from its shape and checked against the SHA-256 of the FLAT program
// the node judged.
type nodeDataLimits struct {
	Provenance map[string]string  `json:"provenance"`
	CostModels map[string][]int64 `json:"costModels"`
	Protocol   uint               `json:"protocolMajor"`
	Scripts    []struct {
		ID    string `json:"id"`
		Lang  string `json:"lang"`
		Shape string `json:"shape"`
		N     int    `json:"n"`
		Body  string `json:"body"`
		// Flat is set for scripts that build their data at run time.
		Flat       string `json:"flat"`
		FlatSHA256 string `json:"flatSha256"`
		// Outcome is ok or phase2-fail (the ledger's verdict).
		Outcome string       `json:"outcome"`
		ExUnits *nodeExUnits `json:"exunits,omitempty"`
		Note    string       `json:"note,omitempty"`
	} `json:"scripts"`
}

// dataShape returns a PlutusData encoding of the given shape and size, and
// the canonical encoding serialiseData produces for it.
func dataShape(t *testing.T, shape string, n int) (raw, canonical []byte) {
	t.Helper()
	rep := func(unit []byte, k int) []byte { return bytes.Repeat(unit, k) }
	cat := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	head := func(major byte, k int) []byte {
		switch {
		case k < 24:
			return []byte{major<<5 | byte(k)}
		case k < 1<<8:
			return []byte{major<<5 | 24, byte(k)}
		case k < 1<<16:
			return binary.BigEndian.AppendUint16([]byte{major<<5 | 25}, uint16(k))
		default:
			return binary.BigEndian.AppendUint32([]byte{major<<5 | 26}, uint32(k))
		}
	}
	zero := []byte{0}
	switch shape {
	case "deepL":
		return cat(rep([]byte{0x81}, n), zero), cat(rep([]byte{0x9f}, n), zero, rep([]byte{0xff}, n))
	case "deepLI":
		b := cat(rep([]byte{0x9f}, n), zero, rep([]byte{0xff}, n))
		return b, b
	case "deepC":
		return cat(rep([]byte{0xd8, 0x79, 0x81}, n), zero),
			cat(rep([]byte{0xd8, 0x79, 0x9f}, n), zero, rep([]byte{0xff}, n))
	case "deepM":
		b := cat(rep([]byte{0xa1, 0x00}, n), zero)
		return b, b
	case "deepMI":
		return cat(rep([]byte{0xbf, 0x00}, n), zero, rep([]byte{0xff}, n)), cat(rep([]byte{0xa1, 0x00}, n), zero)
	case "wideL":
		return cat(head(4, n), rep(zero, n)), cat([]byte{0x9f}, rep(zero, n), []byte{0xff})
	case "wideLI":
		b := cat([]byte{0x9f}, rep(zero, n), []byte{0xff})
		return b, b
	case "wideM":
		b := cat(head(5, n), rep([]byte{0, 0}, n))
		return b, b
	case "wideMI":
		return cat([]byte{0xbf}, rep([]byte{0, 0}, n), []byte{0xff}), cat(head(5, n), rep([]byte{0, 0}, n))
	case "wideC":
		return cat([]byte{0xd8, 0x79}, head(4, n), rep(zero, n)), cat([]byte{0xd8, 0x79, 0x9f}, rep(zero, n), []byte{0xff})
	}
	t.Fatalf("unknown shape %q", shape)
	return nil, nil
}

// flatWriter is a minimal FLAT encoder for the probe scripts, matching the
// harness's encoder bit for bit (constants are written exactly as given,
// including non-canonical Data CBOR).
type flatWriter struct {
	buf  []byte
	bits int
}

func (w *flatWriter) bit(b bool) {
	if w.bits%8 == 0 {
		w.buf = append(w.buf, 0)
	}
	if b {
		w.buf[len(w.buf)-1] |= 0x80 >> (w.bits % 8)
	}
	w.bits++
}

func (w *flatWriter) uint(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bit(v>>i&1 == 1)
	}
}

func (w *flatWriter) natural(n *big.Int) {
	n = new(big.Int).Set(n)
	for {
		group := n.Uint64() & 0x7f
		n.Rsh(n, 7)
		if n.Sign() != 0 {
			group |= 0x80
		}
		w.uint(group, 8)
		if n.Sign() == 0 {
			return
		}
	}
}

func (w *flatWriter) filler() {
	for w.bits%8 != 7 {
		w.bit(false)
	}
	w.bit(true)
}

func (w *flatWriter) bytestring(b []byte) {
	w.filler()
	for len(b) > 0 {
		chunk := b[:min(len(b), 255)]
		w.uint(uint64(len(chunk)), 8)
		for _, c := range chunk {
			w.uint(uint64(c), 8)
		}
		b = b[len(chunk):]
	}
	w.uint(0, 8)
}

// probeTerm is a term of a probe script.
type probeTerm func(w *flatWriter)

func pVar(i int64) probeTerm {
	return func(w *flatWriter) { w.uint(0, 4); w.natural(big.NewInt(i)) }
}
func pDelay(t probeTerm) probeTerm { return func(w *flatWriter) { w.uint(1, 4); t(w) } }
func pLam(t probeTerm) probeTerm   { return func(w *flatWriter) { w.uint(2, 4); t(w) } }
func pApp(f, a probeTerm) probeTerm {
	return func(w *flatWriter) { w.uint(3, 4); f(w); a(w) }
}
func pForce(t probeTerm) probeTerm { return func(w *flatWriter) { w.uint(5, 4); t(w) } }
func pErr() probeTerm              { return func(w *flatWriter) { w.uint(6, 4) } }
func pBuiltin(tag uint64) probeTerm {
	return func(w *flatWriter) { w.uint(7, 4); w.uint(tag, 7) }
}
func pCon(typ uint64, value func(w *flatWriter)) probeTerm {
	return func(w *flatWriter) {
		w.uint(4, 4)
		w.bit(true)
		w.uint(typ, 4)
		w.bit(false)
		value(w)
	}
}
func pUnit() probeTerm { return pCon(3, func(*flatWriter) {}) }
func pInt(n int64) probeTerm {
	return pCon(0, func(w *flatWriter) {
		z := big.NewInt(n)
		if n >= 0 {
			z.Lsh(z, 1)
		} else {
			z.Neg(z).Lsh(z, 1).Sub(z, big.NewInt(1))
		}
		w.natural(z)
	})
}
func pBytes(b []byte) probeTerm { return pCon(1, func(w *flatWriter) { w.bytestring(b) }) }
func pData(cbor []byte) probeTerm {
	return pCon(8, func(w *flatWriter) { w.bytestring(cbor) })
}

// Builtin tags used by the probes.
const (
	tagEqualsInteger      = 7
	tagSliceByteString    = 12
	tagLengthOfByteString = 13
	tagEqualsByteString   = 15
	tagIfThenElse         = 26
	tagEqualsData         = 47
	tagSerialiseData      = 51
)

func pAssertAll(conds ...probeTerm) probeTerm {
	t := pUnit()
	for _, cond := range slices.Backward(conds) {
		t = pForce(pApp(pApp(pApp(pForce(pBuiltin(tagIfThenElse)), cond), pDelay(t)), pDelay(pErr())))
	}
	return t
}

// probeProgram returns the FLAT program body applied to the Data constant
// raw, whose serialiseData output is canonical.
func probeProgram(t *testing.T, body string, raw, canonical []byte) []byte {
	t.Helper()
	d := pData(raw)
	var term probeTerm
	version := []int64{1, 1, 0}
	switch body {
	case "ign":
		term = pLam(pApp(pLam(pUnit()), d))
	case "eq":
		term = pLam(pApp(pLam(pAssertAll(pApp(pApp(pBuiltin(tagEqualsData), pVar(1)), pVar(1)))), d))
	case "serlen":
		term = pLam(pApp(pLam(pAssertAll(pApp(pApp(pBuiltin(tagEqualsInteger),
			pApp(pBuiltin(tagLengthOfByteString), pVar(1))), pInt(int64(len(canonical)))))),
			pApp(pBuiltin(tagSerialiseData), d)))
	case "ser":
		l := len(canonical)
		k := min(64, l)
		slice := func(start int) probeTerm {
			return pApp(pApp(pBuiltin(tagEqualsByteString),
				pApp(pApp(pApp(pBuiltin(tagSliceByteString), pInt(int64(start))), pInt(int64(k))), pVar(1))),
				pBytes(canonical[start:start+k]))
		}
		term = pLam(pApp(pLam(pAssertAll(
			pApp(pApp(pBuiltin(tagEqualsInteger), pApp(pBuiltin(tagLengthOfByteString), pVar(1))), pInt(int64(l))),
			slice(0), slice(l/2), slice(l-k))),
			pApp(pBuiltin(tagSerialiseData), d)))
	case "retV2":
		term = pLam(pLam(d))
		version = []int64{1, 0, 0}
	case "retV3":
		term = pLam(d)
	default:
		t.Fatalf("unknown body %q", body)
	}
	var w flatWriter
	for _, v := range version {
		w.natural(big.NewInt(v))
	}
	term(&w)
	w.filler()
	return w.buf
}

// TestNodeDataLimits replays every recorded script and requires the node's
// outcome and, for successes, its exact ExUnits: Data constants past the
// former depth limit decode and evaluate, and equalsData, serialiseData and
// run-time construction of nested Data cost what the node charges.
func TestNodeDataLimits(t *testing.T) {
	gt := loadNodeFixture[nodeDataLimits](t, "node_data_limits_pv11.json.gz")
	for _, sc := range gt.Scripts {
		t.Run(sc.ID, func(t *testing.T) {
			var flat []byte
			if sc.Flat != "" {
				var err error
				if flat, err = hex.DecodeString(sc.Flat); err != nil {
					t.Fatal(err)
				}
			} else {
				raw, canonical := dataShape(t, sc.Shape, sc.N)
				flat = probeProgram(t, sc.Body, raw, canonical)
			}
			if sum := sha256.Sum256(flat); hex.EncodeToString(sum[:]) != sc.FlatSHA256 {
				t.Fatalf("regenerated script differs from the one the node ran")
			}
			lv, ln := groundTruthLang(t, sc.Lang)
			pc := syn.ProgramContext{LedgerLanguage: lv, ProtocolMajor: gt.Protocol}
			prog, err := syn.DecodeDeBruijnWithContext(flat, pc)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			ec, err := cek.NewEvalContext(lv, cek.ProtoVersion{Major: gt.Protocol}, gt.CostModels[ln])
			if err != nil {
				t.Fatal(err)
			}
			m := cek.NewMachine[syn.DeBruijn](lv, 0, ec)
			m.ExBudget = nodeTxBudget
			isUnit, err := m.RunScriptContext(context.Background(), prog.Term, ledgerArgs(sc.Lang, "")...)
			ex := nodeTxBudget.Sub(&m.ExBudget)
			got := "ok"
			if err != nil || (sc.Lang == "V3" && !isUnit) {
				got = "phase2-fail"
			}
			if got != sc.Outcome {
				t.Fatalf("outcome %s (%v), node %s (%s)", got, err, sc.Outcome, sc.Note)
			}
			if sc.ExUnits != nil && ex != sc.ExUnits.budget() {
				t.Fatalf("exunits mem=%d steps=%d, node mem=%d steps=%d",
					ex.Mem, ex.Cpu, sc.ExUnits.Mem, sc.ExUnits.Steps)
			}
		})
	}
}

// deepDataLevels is the depth of the stack-safety probes: past the former
// 16384-level decode limit and deep enough that a walker recursing once per
// level would overflow deepDataMaxStack.
const (
	deepDataLevels   = 100_000
	deepDataMaxStack = 1 << 20
)

// TestDeepDataStackSafety decodes and evaluates scripts over Data constants
// deepDataLevels deep on a small goroutine stack: decoding, validation,
// equalsData and serialiseData walk Data with an explicit stack, so an
// accidental per-level recursion crashes the test binary instead of passing.
func TestDeepDataStackSafety(t *testing.T) {
	ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 11})
	for _, shape := range []string{"deepL", "deepC", "deepM"} {
		for _, body := range []string{"eq", "serlen"} {
			t.Run(shape+"-"+body, func(t *testing.T) {
				raw, canonical := dataShape(t, shape, deepDataLevels)
				flat := probeProgram(t, body, raw, canonical)
				var err error
				withMaxStack(deepDataMaxStack, func() {
					var prog *syn.Program[syn.DeBruijn]
					prog, err = syn.DecodeDeBruijnWithContext(flat, syn.ProgramContext{
						LedgerLanguage: lang.LanguageVersionV3,
						ProtocolMajor:  11,
					})
					if err != nil {
						return
					}
					m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
					m.ExBudget = nodeTxBudget
					var isUnit bool
					isUnit, err = m.RunScriptContext(context.Background(), prog.Term, data.NewConstr(0))
					if err == nil && !isUnit {
						err = errors.New("result is not unit")
					}
				})
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// withMaxStack runs f on a new goroutine with the maximum goroutine stack
// size lowered to limit. The limit is process-wide, so callers must not run
// in parallel.
func withMaxStack(limit int, f func()) {
	old := debug.SetMaxStack(limit)
	defer debug.SetMaxStack(old)
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	<-done
}

// TestSharedDataConstantRejected checks that the pre-evaluation validation
// rejects a hand-built Data constant whose children are shared: a graph no
// decoder produces, which a tree walk could take exponential time over.
func TestSharedDataConstantRejected(t *testing.T) {
	ec := cek.NewDefaultEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: 11})
	// Each level holds its child twice: 2^40 tree nodes in 40 values.
	var shared data.PlutusData = data.NewInteger(big.NewInt(0))
	for range 40 {
		shared = data.NewList(shared, shared)
	}
	m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
	m.ExBudget = nodeTxBudget
	term := &syn.Apply[syn.DeBruijn]{
		Function: &syn.Lambda[syn.DeBruijn]{Body: &syn.Constant{Con: &syn.Unit{}}},
		Argument: &syn.Constant{Con: &syn.Data{Inner: shared}},
	}
	if _, err := m.Run(term); err == nil {
		t.Fatal("shared constant accepted")
	}
}
