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
		{
			"V1",
			CostModelParamNamesV1,
			351,
			"f07c1d5a2c663ba2aa0e0d381b763f4aa083b210ed4fa9a521746466cc014b4f",
		},
		{
			"V2",
			CostModelParamNamesV2,
			351,
			"4714337805a3445669ba82ad6bdb3658484220124c171e0d3451b3d5e919a98d",
		},
		{
			"V3",
			CostModelParamNamesV3,
			369,
			"67e947b488918a6200b87899b1d2333f6bc56ff7bb1e60404d2e83372b2e032e",
		},
		{
			"V4",
			CostModelParamNamesV4,
			369,
			"67e947b488918a6200b87899b1d2333f6bc56ff7bb1e60404d2e83372b2e032e",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != tt.count {
				t.Fatalf("parameter count = %d, want %d", len(tt.got), tt.count)
			}
			digest := sha256.Sum256([]byte(strings.Join(tt.got, "\x00")))
			if got := hex.EncodeToString(digest[:]); got != tt.digest {
				t.Fatalf(
					"parameter order/content digest = %s, want %s",
					got,
					tt.digest,
				)
			}
		})
	}
}
