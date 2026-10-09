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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// The builtin batches below are written out by name from the reference
// ledger's PlutusLedgerApi.Common.Versions rather than read from
// builtin.IsAvailableInWithProto, so the expectations do not derive from the
// table under test.
var (
	pvBatch2  = []string{"serialiseData"}
	pvBatch3  = []string{"verifyEcdsaSecp256k1Signature", "verifySchnorrSecp256k1Signature"}
	pvBatch4a = []string{
		"bls12_381_G1_add", "bls12_381_G1_neg", "bls12_381_G1_scalarMul",
		"bls12_381_G1_equal", "bls12_381_G1_compress",
		"bls12_381_G1_uncompress", "bls12_381_G1_hashToGroup",
		"bls12_381_G2_add", "bls12_381_G2_neg", "bls12_381_G2_scalarMul",
		"bls12_381_G2_equal", "bls12_381_G2_compress",
		"bls12_381_G2_uncompress", "bls12_381_G2_hashToGroup",
		"bls12_381_millerLoop", "bls12_381_mulMlResult",
		"bls12_381_finalVerify", "keccak_256", "blake2b_224",
	}
	pvBatch4b = []string{"integerToByteString", "byteStringToInteger"}
	pvBatch5  = []string{
		"andByteString", "orByteString", "xorByteString",
		"complementByteString", "readBit", "writeBits", "replicateByte",
		"shiftByteString", "rotateByteString", "countSetBits",
		"findFirstSetBit", "ripemd_160",
	}
	pvBatch6 = []string{
		"expModInteger", "dropList", "lengthOfArray", "listToArray",
		"indexArray", "bls12_381_G1_multiScalarMul",
		"bls12_381_G2_multiScalarMul", "insertCoin", "lookupCoin",
		"unionValue", "valueContains", "valueData", "unValueData",
		"scaleValue",
	}
)

func concatNames(batches ...[]string) []string {
	var all []string
	for _, batch := range batches {
		all = append(all, batch...)
	}
	return all
}

// pvAvailabilityCase is one availability-sensitive program with the ledger
// language and protocol version it is validated and evaluated at, and whether
// the builtin is available there.
type pvAvailabilityCase struct {
	name      string
	language  lang.LanguageVersion
	protocol  uint
	program   string
	available bool
}

// saturatedBuiltinProgram applies the builtin to unit arguments after its
// forces. The machine decides availability when the application is saturated,
// before it looks at the argument types.
func saturatedBuiltinProgram(t *testing.T, name string) string {
	t.Helper()
	fn, ok := builtin.Builtins[name]
	if !ok {
		t.Fatalf("unknown builtin %q", name)
	}
	term := "(builtin " + name + ")"
	for range fn.ForceCount() {
		term = "(force " + term + ")"
	}
	if fn.Arity() > 0 {
		term = "[ " + term + strings.Repeat(" (con unit ())", int(fn.Arity())) + " ]"
	}
	return "(program 1.0.0 " + term + ")"
}

func pvAvailabilityCases(t *testing.T) []pvAvailabilityCase {
	t.Helper()
	languages := []struct {
		name string
		lang lang.LanguageVersion
		// gained becomes available at PV11; stable is available at PV10 and PV11.
		gained []string
		stable []string
	}{
		{
			"V1", lang.LanguageVersionV1,
			concatNames(pvBatch2, pvBatch3, pvBatch4a, pvBatch4b, pvBatch5, pvBatch6),
			[]string{"addInteger", "ifThenElse"},
		},
		{
			"V2", lang.LanguageVersionV2,
			concatNames(pvBatch4a, pvBatch5, pvBatch6),
			concatNames([]string{"addInteger"}, pvBatch2, pvBatch3, pvBatch4b),
		},
		{
			"V3", lang.LanguageVersionV3,
			pvBatch6,
			concatNames([]string{"addInteger"}, pvBatch2, pvBatch3, pvBatch4a, pvBatch4b, pvBatch5),
		},
	}

	var cases []pvAvailabilityCase
	add := func(language string, version lang.LanguageVersion, name string, pv uint, available bool) {
		cases = append(cases, pvAvailabilityCase{
			name:      fmt.Sprintf("%s/%s/PV%d", language, name, pv),
			language:  version,
			protocol:  pv,
			program:   saturatedBuiltinProgram(t, name),
			available: available,
		})
	}
	for _, l := range languages {
		for _, name := range l.gained {
			add(l.name, l.lang, name, 10, false)
			add(l.name, l.lang, name, 11, true)
		}
		for _, name := range l.stable {
			add(l.name, l.lang, name, 10, true)
			add(l.name, l.lang, name, 11, true)
		}
	}
	return cases
}

func isUnavailableBuiltinError(err error) bool {
	var builtinErr *cek.BuiltinError
	if !errors.As(err, &builtinErr) || builtinErr == nil {
		return false
	}
	return strings.Contains(builtinErr.Message, "is not available in Plutus")
}

// TestConformanceBuiltinAvailabilityAcrossPV10AndPV11 evaluates one
// availability-sensitive program per builtin that gains availability at the
// PV10/PV11 boundary, at both protocol versions, for every ledger language
// that gains it, alongside builtins that do not change as controls.
func TestConformanceBuiltinAvailabilityAcrossPV10AndPV11(t *testing.T) {
	t.Parallel()

	for _, tc := range pvAvailabilityCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			context := syn.ProgramContext{
				LedgerLanguage: tc.language,
				ProtocolMajor:  tc.protocol,
			}

			_, validateErr := syn.ParseWithContext(tc.program, context)
			if tc.available && validateErr != nil {
				t.Fatalf("phase-1 validation rejected an available builtin: %v", validateErr)
			}
			if !tc.available && validateErr == nil {
				t.Fatal("phase-1 validation accepted an unavailable builtin")
			}

			program, err := syn.Parse(tc.program)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			dbProgram, err := syn.NameToDeBruijn(program)
			if err != nil {
				t.Fatalf("NameToDeBruijn: %v", err)
			}
			machine := cek.NewMachine[syn.DeBruijn](
				tc.language,
				0,
				cek.NewDefaultEvalContext(
					tc.language,
					cek.ProtoVersion{Major: tc.protocol},
				),
			)
			machine.ExBudget = cek.DefaultExBudget
			_, err = machine.Run(dbProgram.Term)
			if got := isUnavailableBuiltinError(err); got == tc.available {
				t.Fatalf(
					"machine run error = %v; unavailable-builtin error = %t, builtin available = %t",
					err, got, tc.available,
				)
			}
		})
	}
}

// TestConformanceAvailableBuiltinsEvaluateAcrossPV10AndPV11 runs one builtin
// per batch to a result, not just past the availability check: the same
// well-typed program fails at PV10 and produces its value at PV11.
func TestConformanceAvailableBuiltinsEvaluateAcrossPV10AndPV11(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language lang.LanguageVersion
		program  string
		want     string
	}{
		{
			"V1 serialiseData", lang.LanguageVersionV1,
			`(program 1.0.0 [ (builtin serialiseData) (con data (I 1)) ])`,
			`(con bytestring #01)`,
		},
		{
			"V1 integerToByteString", lang.LanguageVersionV1,
			`(program 1.0.0 [ (builtin integerToByteString) (con bool False) (con integer 0) (con integer 255) ])`,
			`(con bytestring #ff)`,
		},
		{
			"V2 andByteString", lang.LanguageVersionV2,
			`(program 1.0.0 [ (builtin andByteString) (con bool False) (con bytestring #ff) (con bytestring #0f) ])`,
			`(con bytestring #0f)`,
		},
		{
			"V2 keccak_256", lang.LanguageVersionV2,
			`(program 1.0.0 [ (builtin keccak_256) (con bytestring #) ])`,
			`(con bytestring #c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470)`,
		},
		{
			"V3 expModInteger", lang.LanguageVersionV3,
			`(program 1.0.0 [ (builtin expModInteger) (con integer 2) (con integer 3) (con integer 5) ])`,
			`(con integer 3)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, pv := range []uint{10, 11} {
				program, err := syn.Parse(tt.program)
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				dbProgram, err := syn.NameToDeBruijn(program)
				if err != nil {
					t.Fatalf("NameToDeBruijn: %v", err)
				}
				machine := cek.NewMachine[syn.DeBruijn](
					tt.language,
					0,
					cek.NewDefaultEvalContext(tt.language, cek.ProtoVersion{Major: pv}),
				)
				machine.ExBudget = cek.DefaultExBudget
				result, err := machine.Run(dbProgram.Term)
				if pv == 10 {
					if !isUnavailableBuiltinError(err) {
						t.Fatalf("PV10: error = %v, want unavailable builtin", err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("PV11: %v", err)
				}
				if got := syn.PrettyTerm[syn.DeBruijn](result); got != tt.want {
					t.Fatalf("PV11: result = %s, want %s", got, tt.want)
				}
			}
		})
	}
}
