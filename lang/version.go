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

package lang

type LanguageVersion = [3]uint32

var (
	LanguageVersionV1 = [3]uint32{1, 0, 0}
	LanguageVersionV2 = [3]uint32{1, 1, 0}
	LanguageVersionV3 = [3]uint32{1, 2, 0}
	LanguageVersionV4 = [3]uint32{1, 3, 0}
)

func GetParamNamesForVersion(version LanguageVersion) []string {
	switch version {
	case LanguageVersionV1:
		return CostModelParamNamesV1
	case LanguageVersionV2:
		return CostModelParamNamesV2
	case LanguageVersionV3:
		return CostModelParamNamesV3
	case LanguageVersionV4:
		return CostModelParamNamesV4
	default:
		return nil
	}
}
