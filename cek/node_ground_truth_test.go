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
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// nodeGroundTruth is the outcome of scripts run on a real cardano-node
// (see the provenance field of the fixture): each script was submitted in a
// transaction and the ledger's verdict and ExUnits recorded.
type nodeGroundTruth struct {
	Provenance map[string]string  `json:"provenance"`
	CostModels map[string][]int64 `json:"costModels"`
	Protocol   uint               `json:"protocolMajor"`
	Scripts    []struct {
		ID      string `json:"id"`
		Lang    string `json:"lang"`
		Purpose string `json:"purpose"`
		Flat    string `json:"flat"`
		// Outcome is ok, phase1-reject (the ledger refused to
		// deserialise the script: MalformedScriptWitnesses) or
		// phase2-fail (evaluation failed).
		Outcome string `json:"outcome"`
		// ExUnits is the ledger's consumption for ok outcomes.
		ExUnits *nodeExUnits `json:"exunits,omitempty"`
		// Budget, when set, is the budget the transaction declared.
		Budget *nodeExUnits `json:"budget,omitempty"`
	} `json:"scripts"`
	// Serialise holds programs (lam ctx [serialiseData X]) and the bytes
	// the node's serialiseData produced for X.
	Serialise []struct {
		ID    string `json:"id"`
		Flat  string `json:"flat"`
		Bytes string `json:"bytes"`
	} `json:"serialise"`
}

// nodeExUnits is a budget or consumption as the ledger reports it.
type nodeExUnits struct {
	Mem   int64 `json:"mem"`
	Steps int64 `json:"steps"`
}

func (u nodeExUnits) budget() cek.ExBudget {
	return cek.ExBudget{Mem: u.Mem, Cpu: u.Steps}
}

// nodeTxBudget is the per-transaction execution budget of the test network
// the node fixtures were recorded on.
var nodeTxBudget = cek.ExBudget{Mem: 10_000_000_000, Cpu: 10_000_000_000_000}

// loadNodeFixture decodes the gzipped JSON fixture testdata/name.
func loadNodeFixture[F any](t *testing.T, name string) *F {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var fx F
	if err := json.NewDecoder(zr).Decode(&fx); err != nil {
		t.Fatal(err)
	}
	return &fx
}

// ledgerArgs stands in Constr 0 [] for each argument the ledger passes a
// script of the given language and purpose; the fixture scripts ignore them.
func ledgerArgs(language, purpose string) []data.PlutusData {
	nargs := 1
	if language != "V3" {
		nargs = 2
		if purpose == "spend" {
			nargs = 3
		}
	}
	args := make([]data.PlutusData, nargs)
	for i := range args {
		args[i] = data.NewConstr(0)
	}
	return args
}

func groundTruthLang(t *testing.T, s string) (lang.LanguageVersion, string) {
	t.Helper()
	switch s {
	case "V1":
		return lang.LanguageVersionV1, "PlutusV1"
	case "V2":
		return lang.LanguageVersionV2, "PlutusV2"
	case "V3":
		return lang.LanguageVersionV3, "PlutusV3"
	}
	t.Fatalf("unknown language %q", s)
	return lang.LanguageVersion{}, ""
}

// TestNodeGroundTruth replays every script recorded from the node and
// requires the same outcome and, for successes, the same ExUnits. Scripts
// ignore their arguments, so Constr 0 [] stands in for each ledger argument.
func TestNodeGroundTruth(t *testing.T) {
	gt := loadNodeFixture[nodeGroundTruth](t, "node_ground_truth_pv11.json.gz")
	for _, sc := range gt.Scripts {
		t.Run(sc.ID, func(t *testing.T) {
			lv, ln := groundTruthLang(t, sc.Lang)
			flat, err := hex.DecodeString(sc.Flat)
			if err != nil {
				t.Fatal(err)
			}
			pc := syn.ProgramContext{LedgerLanguage: lv, ProtocolMajor: gt.Protocol}
			got, ex := "ok", cek.ExBudget{}
			prog, err := syn.DecodeDeBruijnWithContext(flat, pc)
			switch {
			case err != nil:
				got = "phase1-reject"
			case syn.ValidateTermVersionForExecution(prog.Version, pc) != nil:
				got = "phase2-fail"
			default:
				ec, err := cek.NewEvalContext(lv, cek.ProtoVersion{Major: gt.Protocol}, gt.CostModels[ln])
				if err != nil {
					t.Fatal(err)
				}
				budget := nodeTxBudget
				if sc.Budget != nil {
					budget = sc.Budget.budget()
				}
				m := cek.NewMachine[syn.DeBruijn](lv, 0, ec)
				m.ExBudget = budget
				isUnit, err := m.RunScriptContext(context.Background(), prog.Term, ledgerArgs(sc.Lang, sc.Purpose)...)
				ex = budget.Sub(&m.ExBudget)
				if err != nil || (sc.Lang == "V3" && !isUnit) {
					got = "phase2-fail"
				}
			}
			if got != sc.Outcome {
				t.Fatalf("outcome %s, node %s", got, sc.Outcome)
			}
			if sc.Outcome == "ok" && sc.ExUnits != nil && ex != sc.ExUnits.budget() {
				t.Fatalf("exunits mem=%d steps=%d, node mem=%d steps=%d",
					ex.Mem, ex.Cpu, sc.ExUnits.Mem, sc.ExUnits.Steps)
			}
		})
	}
}

var groundTruthBytes = regexp.MustCompile(`#([0-9a-f]*)`)

// TestNodeGroundTruthSerialiseData requires serialiseData to produce the
// node's bytes for Data constants decoded from many CBOR forms.
func TestNodeGroundTruthSerialiseData(t *testing.T) {
	gt := loadNodeFixture[nodeGroundTruth](t, "node_ground_truth_pv11.json.gz")
	for _, sc := range gt.Serialise {
		t.Run(sc.ID, func(t *testing.T) {
			flat, err := hex.DecodeString(sc.Flat)
			if err != nil {
				t.Fatal(err)
			}
			pc := syn.ProgramContext{LedgerLanguage: lang.LanguageVersionV3, ProtocolMajor: gt.Protocol}
			prog, err := syn.DecodeDeBruijnWithContext(flat, pc)
			if err != nil {
				t.Fatal(err)
			}
			ec, err := cek.NewEvalContext(lang.LanguageVersionV3, cek.ProtoVersion{Major: gt.Protocol}, gt.CostModels["PlutusV3"])
			if err != nil {
				t.Fatal(err)
			}
			m := cek.NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, ec)
			m.ExBudget = nodeTxBudget
			term := &syn.Apply[syn.DeBruijn]{
				Function: prog.Term,
				Argument: &syn.Constant{Con: &syn.Data{Inner: data.NewConstr(0)}},
			}
			res, err := m.Run(term)
			if err != nil {
				t.Fatal(err)
			}
			match := groundTruthBytes.FindStringSubmatch(syn.PrettyTerm[syn.DeBruijn](res))
			if match == nil || match[1] != sc.Bytes {
				t.Fatalf("serialiseData = %v, node %s", match, sc.Bytes)
			}
		})
	}
}
