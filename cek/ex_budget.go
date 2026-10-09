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

func (ex *ExBudget) Sub(other *ExBudget) ExBudget {
	return ExBudget{
		Mem: ex.Mem - other.Mem,
		Cpu: ex.Cpu - other.Cpu,
	}
}
