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

// Field-mapping equivalence for cost-model construction (plutigo#402).
//
// costModelFromList/costModelFromMap dispatch each parameter name to a
// specific MachineCosts or BuiltinCosts field. This file captures the
// complete resulting CostModel -- every machine cost field and every
// builtin's cost function, including nested argument-model shapes -- as a
// deterministic text dump, and pins it against a committed golden file. The
// golden file was captured from the pre-refactor code (strings.Split called
// directly in update()); the same test must still pass after the parameter
// name split is precomputed, proving that path routes every parameter to
// the same field as before.
//
// Regenerate after a deliberate cost-model change with:
//
//	go test ./cek/ -run TestCostModelFieldMappingGolden -update-golden

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
)

var updateGolden = flag.Bool(
	"update-golden",
	false,
	"regenerate cek/testdata/cost_model_golden.txt from the current code",
)

const costModelGoldenPath = "testdata/cost_model_golden.txt"

// costModelFieldMappingCases enumerates every (language version, semantics
// variant) pair reachable through GetSemantics: V1/V2 pair only with
// A/B/D, V3/V4 only with C/E (cek/semantics.go).
var costModelFieldMappingCases = []struct {
	name      string
	version   lang.LanguageVersion
	semantics SemanticsVariant
}{
	{"v1_variantA", lang.LanguageVersionV1, SemanticsVariantA},
	{"v1_variantB", lang.LanguageVersionV1, SemanticsVariantB},
	{"v1_variantD", lang.LanguageVersionV1, SemanticsVariantD},
	{"v2_variantA", lang.LanguageVersionV2, SemanticsVariantA},
	{"v2_variantB", lang.LanguageVersionV2, SemanticsVariantB},
	{"v2_variantD", lang.LanguageVersionV2, SemanticsVariantD},
	{"v3_variantC", lang.LanguageVersionV3, SemanticsVariantC},
	{"v3_variantE", lang.LanguageVersionV3, SemanticsVariantE},
	{"v4_variantC", lang.LanguageVersionV4, SemanticsVariantC},
	{"v4_variantE", lang.LanguageVersionV4, SemanticsVariantE},
}

// synthCostModelParams builds a parameter list the same length as names,
// where position i carries a value unique to that position. A field-mapping
// bug that routes parameter i into parameter j's field changes exactly one
// number in the dump, so the golden comparison catches it regardless of
// which field it lands in.
func synthCostModelParams(names []string) []int64 {
	params := make([]int64, len(names))
	for i := range params {
		params[i] = 1_000_000 + int64(i)
	}
	return params
}

// dumpCostModel renders the complete CostModel deterministically: every
// machine cost field, and every builtin's cpu/mem argument model with all
// of its fields (including nested TwoArgument models). It uses reflection
// instead of fmt's %#v because %#v prints a struct's *pointer* fields as a
// raw address rather than dereferencing them, which several Arguments
// implementations have (e.g. ConstAboveDiagonalModel.model) -- that would
// make the dump non-deterministic across runs instead of catching a
// mismatch.
func dumpCostModel(cm CostModel) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "machineCosts: %s\n", dumpReflectValue(reflect.ValueOf(cm.machineCosts)))

	for i := range cm.builtinCosts {
		name := builtin.DefaultFunction(i).String() //nolint:gosec // i is a valid array index
		cf := cm.builtinCosts[i]
		if cf == nil {
			fmt.Fprintf(&sb, "builtin[%3d] %-40s <nil>\n", i, name)
			continue
		}
		fmt.Fprintf(
			&sb,
			"builtin[%3d] %-40s mem=%s cpu=%s\n",
			i,
			name,
			dumpReflectValue(reflect.ValueOf(cf.mem)),
			dumpReflectValue(reflect.ValueOf(cf.cpu)),
		)
	}

	return sb.String()
}

func dumpReflectValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Invalid:
		return "nil"
	case reflect.Ptr:
		if v.IsNil() {
			return "nil"
		}
		return "&" + dumpReflectValue(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return "nil"
		}
		return dumpReflectValue(v.Elem())
	case reflect.Struct:
		var sb strings.Builder
		t := v.Type()
		sb.WriteString(t.Name())
		sb.WriteString("{")
		for i := range v.NumField() {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(t.Field(i).Name)
			sb.WriteString(":")
			sb.WriteString(dumpReflectValue(v.Field(i)))
		}
		sb.WriteString("}")
		return sb.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool())
	default:
		return fmt.Sprintf("!UNSUPPORTED(%s)", v.Kind())
	}
}

// TestCostModelFieldMappingGolden pins the complete field mapping performed
// by costModelFromList/costModelFromMap, for every reachable (version,
// semantics) pair plus the real preview-network V1 and V3 cost models. It
// must fail if any single parameter lands in a different field than before.
func TestCostModelFieldMappingGolden(t *testing.T) {
	t.Parallel()

	var got strings.Builder
	for _, tc := range costModelFieldMappingCases {
		names := lang.GetParamNamesForVersion(tc.version)
		if len(names) == 0 {
			t.Fatalf("%s: no parameter names for version %v", tc.name, tc.version)
		}
		params := synthCostModelParams(names)

		cm, err := costModelFromList(tc.version, tc.semantics, params)
		if err != nil {
			t.Fatalf("%s: costModelFromList: %v", tc.name, err)
		}

		fmt.Fprintf(&got, "=== synthetic_%s ===\n", tc.name)
		got.WriteString(dumpCostModel(cm))
	}

	// Real preview-network PlutusV1 cost model (alonzo-genesis.json),
	// exercised through costModelFromMap.
	cmV1, err := costModelFromMap(
		lang.LanguageVersionV1,
		SemanticsVariantA,
		realPreviewV1CostModelParams,
	)
	if err != nil {
		t.Fatalf("real_v1_variantA: costModelFromMap: %v", err)
	}
	fmt.Fprintf(&got, "=== real_v1_variantA ===\n")
	got.WriteString(dumpCostModel(cmV1))

	// Real preview-network PlutusV3 cost model (conway-genesis.json),
	// exercised through costModelFromList. It has 251 values for 350 V3
	// parameter names, so it also pins short-list handling: the 99 missing
	// parameters must all read as maxBound, not as compiled-in defaults.
	cmV3, err := costModelFromList(
		lang.LanguageVersionV3,
		SemanticsVariantC,
		realPreviewV3CostModelParams,
	)
	if err != nil {
		t.Fatalf("real_v3_variantC: costModelFromList: %v", err)
	}
	fmt.Fprintf(&got, "=== real_v3_variantC ===\n")
	got.WriteString(dumpCostModel(cmV3))

	if *updateGolden {
		if err := os.WriteFile(costModelGoldenPath, []byte(got.String()), 0o600); err != nil {
			t.Fatalf("write golden file %s: %v", costModelGoldenPath, err)
		}
		t.Skipf("regenerated %s; re-run without -update-golden to verify", costModelGoldenPath)
	}

	want, err := os.ReadFile(costModelGoldenPath)
	if err != nil {
		t.Fatalf(
			"read golden file %s: %v (regenerate with: go test ./cek/ -run TestCostModelFieldMappingGolden -update-golden)",
			costModelGoldenPath,
			err,
		)
	}

	if got.String() != string(want) {
		t.Fatalf(
			"cost model field mapping diverged from golden %s.\n"+
				"If this divergence is an intentional cost-model change, regenerate with:\n"+
				"  go test ./cek/ -run TestCostModelFieldMappingGolden -update-golden\n\n--- got ---\n%s",
			costModelGoldenPath,
			got.String(),
		)
	}
}
