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
	"strings"

	"github.com/blinklabs-io/plutigo/lang"
)

// costModelLanguageVersions lists every language version with a cost-model
// parameter name list. Add new versions here when lang gains one.
var costModelLanguageVersions = []lang.LanguageVersion{
	lang.LanguageVersionV1,
	lang.LanguageVersionV2,
	lang.LanguageVersionV3,
	lang.LanguageVersionV4,
}

// paramNameParts maps each cost-model parameter name to its "-"-separated
// parts. It is built during package initialization and never written
// afterwards, so concurrent cost-model construction reads it without
// synchronization.
var paramNameParts = buildParamNameParts()

func buildParamNameParts() map[string][]string {
	parts := make(map[string][]string)
	for _, version := range costModelLanguageVersions {
		for _, name := range lang.GetParamNamesForVersion(version) {
			if _, ok := parts[name]; !ok {
				parts[name] = strings.Split(name, "-")
			}
		}
	}
	return parts
}

// splitParamName returns the "-"-separated parts of a cost-model parameter
// name. Names from costModelLanguageVersions return a shared slice that
// callers must not modify; any other name is split on each call and not
// retained.
func splitParamName(name string) []string {
	if parts, ok := paramNameParts[name]; ok {
		return parts
	}
	return strings.Split(name, "-")
}
