package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

// addInteger-01 is copied byte-for-byte from the upstream Plutus conformance
// corpus at 3109dc1e6501ecbbed0b7ae27361c255d7a16173.
var (
	referenceAddIntegerFlat = []byte{
		0x01, 0x00, 0x00, 0x33, 0x70, 0x09, 0x00, 0x12, 0x40, 0x05,
	}
	referenceAddIntegerFlatExpected = []byte{
		0x01, 0x00, 0x00, 0x48, 0x01, 0x01,
	}
)

const (
	referenceAddIntegerText         = "(program 1.0.0 [ [ (builtin addInteger) (con integer 1)] (con integer 1) ])"
	referenceAddIntegerTextExpected = "(program 1.0.0 (con integer 2))"
	referenceAddIntegerBudget       = "({cpu: 181308\n| mem: 602})"

	referenceKeccakText = `-- Test vector from the upstream Plutus conformance corpus
(program 1.0.0
 [
  [
   (builtin equalsByteString)
   [
    (builtin keccak_256)
    (con bytestring #AAFDC9243D3D4A096558A360CC27C8D862F0BE73DB5E88AA55)
   ]
  ]
  (con bytestring #6FFFA070B865BE3EE766DC2DB49B6AA55C369F7DE3703ADA2612D754145C01E6)
 ]
)`
	referenceKeccakTextExpected = "(program 1.0.0 (con bool True))"
	referenceKeccakBudget       = "({cpu: 2660757\n| mem: 805})"

	referenceLengthOfArrayText = `-- Measuring the length of an empty array
(program 1.1.0
  [
    (force (builtin lengthOfArray))
    (con (array bool) [])
  ]
)`
	referenceLengthOfArrayTextExpected = "(program 1.1.0 (con integer 0))"
	referenceLengthOfArrayBudget       = "({cpu: 295983\n| mem: 510})"
)

func writeConformanceFixture(
	t *testing.T,
	path string,
	contents []byte,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func writeReferenceAddIntegerCase(t *testing.T, root string) string {
	t.Helper()
	caseDir := filepath.Join(
		root,
		"builtin",
		"semantics",
		"addInteger",
		"addInteger-01",
	)
	stem := filepath.Join(caseDir, "addInteger-01")
	writeConformanceFixture(t, stem+".uplc", []byte(referenceAddIntegerText))
	writeConformanceFixture(
		t,
		stem+".uplc.expected",
		[]byte(referenceAddIntegerTextExpected),
	)
	writeConformanceFixture(t, stem+".flat", referenceAddIntegerFlat)
	writeConformanceFixture(
		t,
		stem+".flat.expected",
		referenceAddIntegerFlatExpected,
	)
	writeConformanceFixture(
		t,
		stem+".budget.expected",
		[]byte(referenceAddIntegerBudget),
	)
	return stem
}

func TestConformanceHarnessRunsReferenceTextAndFlat(t *testing.T) {
	root := t.TempDir()
	stem := writeReferenceAddIntegerCase(t, root)

	cases, err := discoverConformanceCases(root)
	if err != nil {
		t.Fatalf("discover reference cases: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("discovered %d cases, want text and FLAT cases", len(cases))
	}

	formats := map[conformanceFormat]bool{}
	for _, testCase := range cases {
		formats[testCase.format] = true
		if testCase.budgetPath != stem+".budget.expected" {
			t.Fatalf(
				"budget path = %q, want canonical sidecar",
				testCase.budgetPath,
			)
		}
		if err := runConformanceCase(testCase); err != nil {
			t.Fatalf("run %s: %v", filepath.Ext(testCase.inputPath), err)
		}
	}
	if !formats[conformanceText] || !formats[conformanceFlat] {
		t.Fatalf("discovered formats = %v, want text and FLAT", formats)
	}
}

func TestConformanceHarnessRunsV4ReferenceText(t *testing.T) {
	root := t.TempDir()
	stem := filepath.Join(
		root,
		"builtin",
		"semantics",
		"lengthOfArray",
		"lengthOfArray-01",
		"lengthOfArray-01",
	)
	writeConformanceFixture(
		t,
		stem+".uplc",
		[]byte(referenceLengthOfArrayText),
	)
	writeConformanceFixture(
		t,
		stem+".uplc.expected",
		[]byte(referenceLengthOfArrayTextExpected),
	)
	writeConformanceFixture(
		t,
		stem+".budget.expected",
		[]byte(referenceLengthOfArrayBudget),
	)

	cases, err := discoverConformanceCases(root)
	if err != nil {
		t.Fatalf("discover V4 case: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("discovered %d cases, want 1", len(cases))
	}
	if err := runConformanceCase(cases[0]); err != nil {
		t.Fatalf("run V4 reference case: %v", err)
	}
}

func TestConformanceHarnessRunsV3ReferenceText(t *testing.T) {
	root := t.TempDir()
	stem := filepath.Join(
		root,
		"builtin",
		"semantics",
		"keccak_256",
		"keccak_256-length-200",
		"keccak_256-length-200",
	)
	writeConformanceFixture(t, stem+".uplc", []byte(referenceKeccakText))
	writeConformanceFixture(
		t,
		stem+".uplc.expected",
		[]byte(referenceKeccakTextExpected),
	)
	writeConformanceFixture(
		t,
		stem+".uplc.budget.expected",
		[]byte(referenceKeccakBudget),
	)

	cases, err := discoverConformanceCases(root)
	if err != nil {
		t.Fatalf("discover V3 case: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("discovered %d cases, want 1", len(cases))
	}
	if version, protocol := conformanceLanguageForPath(cases[0].inputPath); version != lang.LanguageVersionV3 || protocol != 11 {
		t.Fatalf("V3 language selection = %v/%d, want %v/11", version, protocol, lang.LanguageVersionV3)
	}
	if err := runConformanceCase(cases[0]); err != nil {
		t.Fatalf("run V3 reference case: %v", err)
	}
}

func TestConformanceHarnessRejectsWrongFlatResult(t *testing.T) {
	root := t.TempDir()
	stem := writeReferenceAddIntegerCase(t, root)
	if err := os.Remove(stem + ".uplc"); err != nil {
		t.Fatalf("remove text input: %v", err)
	}
	if err := os.Remove(stem + ".uplc.expected"); err != nil {
		t.Fatalf("remove text expected result: %v", err)
	}

	wrongExpected := append([]byte(nil), referenceAddIntegerFlatExpected...)
	wrongExpected[len(wrongExpected)-1] ^= 0x01
	writeConformanceFixture(t, stem+".flat.expected", wrongExpected)

	cases, err := discoverConformanceCases(root)
	if err != nil {
		t.Fatalf("discover mismatched case: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("discovered %d cases, want 1", len(cases))
	}
	err = runConformanceCase(cases[0])
	if err == nil || !strings.Contains(err.Error(), "result mismatch") {
		t.Fatalf("wrong reference result error = %v, want result mismatch", err)
	}
}

func TestConformanceHarnessAcceptsReferenceDecodeFailure(t *testing.T) {
	root := t.TempDir()
	stem := filepath.Join(root, "term", "truncated", "truncated")
	writeConformanceFixture(
		t,
		stem+".flat",
		referenceAddIntegerFlat[:len(referenceAddIntegerFlat)-1],
	)
	writeConformanceFixture(
		t,
		stem+".flat.expected",
		[]byte("parse/decode error"),
	)
	writeConformanceFixture(
		t,
		stem+".budget.expected",
		[]byte("parse/decode error"),
	)

	cases, err := discoverConformanceCases(root)
	if err != nil {
		t.Fatalf("discover decode-failure case: %v", err)
	}
	if err := runConformanceCase(cases[0]); err != nil {
		t.Fatalf("run decode-failure case: %v", err)
	}
}

func TestConformanceHarnessRequiresBudgetSidecar(t *testing.T) {
	root := t.TempDir()
	stem := filepath.Join(root, "term", "missing-budget", "missing-budget")
	writeConformanceFixture(t, stem+".uplc", []byte(referenceAddIntegerText))
	writeConformanceFixture(
		t,
		stem+".uplc.expected",
		[]byte(referenceAddIntegerTextExpected),
	)

	_, err := discoverConformanceCases(root)
	if err == nil || !strings.Contains(err.Error(), "budget result") {
		t.Fatalf("missing budget error = %v, want budget result error", err)
	}
}
