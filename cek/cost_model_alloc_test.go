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
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

// discoverCostModelLanguageVersions walks lang's version numbering (1.0.0,
// 1.1.0, ...) until GetParamNamesForVersion has no names, so a version added
// to lang is covered here even if costModelLanguageVersions is not updated.
func discoverCostModelLanguageVersions(t *testing.T) []lang.LanguageVersion {
	t.Helper()
	var versions []lang.LanguageVersion
	for minor := uint32(0); ; minor++ {
		version := lang.LanguageVersion{1, minor, 0}
		if lang.GetParamNamesForVersion(version) == nil {
			break
		}
		versions = append(versions, version)
	}
	if len(versions) < 4 {
		t.Fatalf(
			"discovered %d language versions with cost-model parameters, want at least 4",
			len(versions),
		)
	}
	return versions
}

// TestCostModelParameterUpdatesDoNotAllocate pins that applying a full
// parameter list allocates nothing beyond building the default model: the
// allocation count of costModelFromList with every parameter equals the count
// with none. Splitting each parameter name per call adds one allocation per
// parameter.
func TestCostModelParameterUpdatesDoNotAllocate(t *testing.T) {
	// No t.Parallel(): testing.AllocsPerRun panics in parallel tests.
	for _, version := range discoverCostModelLanguageVersions(t) {
		names := lang.GetParamNamesForVersion(version)
		semantics := GetSemantics(version, ProtoVersion{Major: 10})
		params := synthCostModelParams(names)

		t.Run(fmt.Sprintf("v%d.%d.%d", version[0], version[1], version[2]), func(t *testing.T) {
			build := func(data []int64) float64 {
				return testing.AllocsPerRun(20, func() {
					if _, err := costModelFromList(version, semantics, data); err != nil {
						t.Fatalf("costModelFromList: %v", err)
					}
				})
			}
			empty := build(nil)
			full := build(params)
			if full != empty {
				t.Fatalf(
					"applying %d parameters allocated %.0f times, want 0 (%.0f with all parameters, %.0f with none)",
					len(params),
					full-empty,
					full,
					empty,
				)
			}
		})
	}
}

// TestSplitParamName checks every known parameter name against strings.Split,
// confirms the lookup does not allocate, and checks that an unknown name is
// still split correctly.
func TestSplitParamName(t *testing.T) {
	// No t.Parallel(): testing.AllocsPerRun panics in parallel tests.
	for _, version := range discoverCostModelLanguageVersions(t) {
		names := lang.GetParamNamesForVersion(version)
		for _, name := range names {
			got, want := splitParamName(name), strings.Split(name, "-")
			if !slices.Equal(got, want) {
				t.Fatalf("splitParamName(%q) = %q, want %q", name, got, want)
			}
		}
		allocs := testing.AllocsPerRun(20, func() {
			for _, name := range names {
				paramPartsSink = splitParamName(name)
			}
		})
		if allocs != 0 {
			t.Errorf(
				"splitParamName over %d names of version %v allocated %.0f times, want 0",
				len(names),
				version,
				allocs,
			)
		}
	}

	const unknown = "notAParameter-cpu-arguments-intercept"
	want := []string{"notAParameter", "cpu", "arguments", "intercept"}
	if got := splitParamName(unknown); !slices.Equal(got, want) {
		t.Fatalf("splitParamName(%q) = %q, want %q", unknown, got, want)
	}
}

var paramPartsSink []string
