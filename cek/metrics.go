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

import "github.com/blinklabs-io/plutigo/builtin"

// numStepKinds is the number of [StepKind] values.
const numStepKinds = 9

// BuiltinMetrics is what one builtin cost over a run: how many times it was
// invoked and the budget those invocations charged.
type BuiltinMetrics struct {
	Calls  uint64
	Budget ExBudget
}

// Metrics are the measurements of the machine's most recent run, collected
// only after [Machine.EnableMetrics]. Counting never changes evaluation or
// budget accounting.
type Metrics struct {
	// Steps counts machine steps by [StepKind].
	Steps [numStepKinds]uint64
	// MaxStackDepth is the deepest the machine's frame stack grew.
	MaxStackDepth int
	// Builtins is indexed by [builtin.DefaultFunction]. Budget covers the
	// builtin's own costing function, not the machine step that reached it.
	Builtins [builtin.TotalBuiltinCount]BuiltinMetrics
}

// EnableMetrics turns on metrics collection for this machine's later runs.
func (m *Machine[T]) EnableMetrics() {
	if m.metrics == nil {
		m.metrics = &Metrics{}
	}
}

// Metrics returns the measurements of the most recent run, or nil when
// metrics are not enabled. Each run replaces them; the returned pointer stays
// valid for the machine's lifetime and is overwritten by the next run.
func (m *Machine[T]) Metrics() *Metrics {
	return m.metrics
}

// recordBuiltin attributes the budget spent since before to one invocation of
// fn. A metered machine routes every invocation through evalBuiltinApp.
func (m *Machine[T]) recordBuiltin(fn builtin.DefaultFunction, before ExBudget) {
	bm := &m.metrics.Builtins[fn]
	bm.Calls++
	spent := before.Sub(&m.ExBudget)
	bm.Budget.Cpu += spent.Cpu
	bm.Budget.Mem += spent.Mem
}
