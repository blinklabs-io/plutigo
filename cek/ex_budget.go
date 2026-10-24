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

import "math"

type ExBudget struct {
	Mem int64
	Cpu int64
}

var DefaultExBudget = ExBudget{
	// Use mainnet-like limits by default; tests needing more should override explicitly
	Mem: 14_000_000,
	Cpu: 10_000_000_000,
}

func (ex *ExBudget) occurrences(n uint32) {
	ex.Mem = satMul(ex.Mem, int64(n))
	ex.Cpu = satMul(ex.Cpu, int64(n))
}

// Sub returns ex - other, saturated at the int64 bounds so that the consumed
// budget of a run that failed on a saturated charge does not wrap.
func (ex *ExBudget) Sub(other *ExBudget) ExBudget {
	return ExBudget{
		Mem: satSub(ex.Mem, other.Mem),
		Cpu: satSub(ex.Cpu, other.Cpu),
	}
}

// satSub returns x - y saturated at the int64 bounds, like the reference's
// SatInt budget arithmetic.
func satSub(x, y int64) int64 {
	d := x - y
	if (x^y) < 0 && (x^d) < 0 {
		if x < 0 {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return d
}
