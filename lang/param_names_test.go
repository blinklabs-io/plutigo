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

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestCostModelParameterTablesMatchPlutus(t *testing.T) {
	tests := []struct {
		name   string
		got    []string
		count  int
		digest string
	}{
		{"V1", CostModelParamNamesV1, 332, "3a393171455c599367bff99e0fa96fc0f5599e0dbeae5cbdb161637047408603"},
		{"V2", CostModelParamNamesV2, 332, "d23d1c05c497da742d69769bb983b4fe55fa2bb84fc3d1864380b19016683ec3"},
		{"V3", CostModelParamNamesV3, 350, "b3aaffbf5d43580dd0c47061dd12514b8e34447aecdab980ebc4bb5285d52df6"},
		{"V4", CostModelParamNamesV4, 361, "838845e67ec39c28e9e1d921e2ec1abc3fac3b6b08a030dddcd490343125b0f9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != tt.count {
				t.Fatalf("parameter count = %d, want %d", len(tt.got), tt.count)
			}
			digest := sha256.Sum256([]byte(strings.Join(tt.got, "\x00")))
			if got := hex.EncodeToString(digest[:]); got != tt.digest {
				t.Fatalf("parameter order/content digest = %s, want %s", got, tt.digest)
			}
		})
	}
}
