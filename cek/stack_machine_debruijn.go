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

// <nilaway skip stack-machine>
package cek

import (
	"context"
	"unsafe"

	"github.com/blinklabs-io/plutigo/syn"
)

// termInterfaceDeBruijn mirrors Go's runtime layout for an interface value
// holding a syn.Term[syn.DeBruijn]. Its itab pointer supports fast dispatch in
// the evaluator loop and in immediate-term checks.
type termInterfaceDeBruijn struct {
	tab  unsafe.Pointer
	data unsafe.Pointer
}

var (
	varTermTabDeBruijn     = termTabDeBruijn(&syn.Var[syn.DeBruijn]{})
	delayTermTabDeBruijn   = termTabDeBruijn(&syn.Delay[syn.DeBruijn]{})
	applyTermTabDeBruijn   = termTabDeBruijn(&syn.Apply[syn.DeBruijn]{})
	forceTermTabDeBruijn   = termTabDeBruijn(&syn.Force[syn.DeBruijn]{})
	constantTermTab        = termTabDeBruijn(&syn.Constant{})
	errorTermTab           = termTabDeBruijn(&syn.Error{})
	caseTermTabDeBruijn    = termTabDeBruijn(&syn.Case[syn.DeBruijn]{})
	constrTermTabDeBruijn  = termTabDeBruijn(&syn.Constr[syn.DeBruijn]{})
	lambdaTermTabDeBruijn  = termTabDeBruijn(&syn.Lambda[syn.DeBruijn]{})
	builtinTermTabDeBruijn = termTabDeBruijn(&syn.Builtin{})
)

func termTabDeBruijn(term syn.Term[syn.DeBruijn]) unsafe.Pointer {
	return (*termInterfaceDeBruijn)(unsafe.Pointer(&term)).tab
}

type stepCounter struct {
	counts    [9]uint32
	remaining uint32
	slippage  uint32
	// ctx, when set, is polled for cancellation each time the batched steps
	// are spent, so cancellation is observed within one slippage batch.
	ctx context.Context
}

func newStepCounter(slippage uint32) stepCounter {
	return stepCounter{remaining: slippage, slippage: slippage}
}

func (s *stepCounter) count(step StepKind) bool {
	s.counts[step]++
	s.remaining--
	return s.remaining != 0
}

//go:noinline
func (s *stepCounter) spend(m *Machine[syn.DeBruijn]) error {
	if s.ctx != nil {
		if err := s.ctx.Err(); err != nil {
			return err
		}
	}
	if s.remaining == s.slippage {
		return nil
	}
	if m.stepBatchFits && !DebugBudget {
		var total ExBudget
		for i, n := range s.counts {
			total.Cpu += int64(n) * m.stepCosts[i].Cpu
			total.Mem += int64(n) * m.stepCosts[i].Mem
		}
		if total.Cpu <= m.ExBudget.Cpu && total.Mem <= m.ExBudget.Mem {
			m.ExBudget.Cpu -= total.Cpu
			m.ExBudget.Mem -= total.Mem
			s.counts = [len(s.counts)]uint32{}
			s.remaining = s.slippage
			return nil
		}
	}
	for i := range uint8(len(s.counts)) {
		unspentStepBudget := m.stepCosts[StepKind(i)]
		unspentStepBudget.occurrences(s.counts[i])
		if err := m.spendBudget(unspentStepBudget); err != nil {
			return err
		}
		s.counts[i] = 0
	}
	s.remaining = s.slippage
	return nil
}

func isImmediateTermDeBruijn(term syn.Term[syn.DeBruijn]) bool {
	termIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&term))
	if termIface.tab == applyTermTabDeBruijn {
		return false
	}
	switch termIface.tab {
	case forceTermTabDeBruijn, caseTermTabDeBruijn:
		return false
	case constrTermTabDeBruijn:
		return len((*syn.Constr[syn.DeBruijn])(termIface.data).Fields) == 0
	default:
		return true
	}
}

func lookupEnvDeBruijn(
	env *Env[syn.DeBruijn],
	idx int,
) (Value[syn.DeBruijn], bool) {
	var zero Value[syn.DeBruijn]
	if idx <= 0 || env == nil {
		return zero, false
	}
	switch idx {
	case 1:
		return env.data, true
	case 2:
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 3:
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 4:
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 5:
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 6:
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	}
	env = envAncestor(env, idx-1)
	if env == nil {
		return zero, false
	}
	return env.data, true
}

func computeKnownImmediateValueDeBruijn(
	m *Machine[syn.DeBruijn],
	steps *stepCounter,
	env *Env[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
) (Value[syn.DeBruijn], error) {
	termIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&term))
	switch termIface.tab {
	case varTermTabDeBruijn:
		t := (*syn.Var[syn.DeBruijn])(termIface.data)
		if !steps.count(ExVar) {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
		}
		value, ok := lookupEnvDeBruijn(env, int(t.Name))
		if !ok {
			return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
		}
		return value, nil
	case delayTermTabDeBruijn:
		t := (*syn.Delay[syn.DeBruijn])(termIface.data)
		if !steps.count(ExDelay) {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
		}
		return m.allocDelay(t, env), nil
	case lambdaTermTabDeBruijn:
		t := (*syn.Lambda[syn.DeBruijn])(termIface.data)
		if !steps.count(ExLambda) {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
		}
		return m.allocLambda(t, env), nil
	case constantTermTab:
		t := (*syn.Constant)(termIface.data)
		if !steps.count(ExConstant) {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
		}
		return machineConstantValue(m, t.Con), nil
	case errorTermTab:
		return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
	case builtinTermTabDeBruijn:
		t := (*syn.Builtin)(termIface.data)
		if !steps.count(ExBuiltin) {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
		}
		return m.builtinNoArgValues[t.DefaultFunction][0], nil
	case constrTermTabDeBruijn:
		t := (*syn.Constr[syn.DeBruijn])(termIface.data)
		if !steps.count(ExConstr) {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
		}
		return m.allocConstr(t.Tag, nil), nil
	default:
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "non-immediate term passed to computeKnownImmediateValueDeBruijn",
		}
	}
}

func runStackDeBruijn(
	ctx context.Context,
	checkCancellation bool,
	m *Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
	steps *stepCounter,
) (syn.Term[syn.DeBruijn], error) {
	// Cancellation is polled where the step batch is spent rather than on
	// every step, so a cancellable run uses the same loop as Run. A context
	// that can never be canceled is not polled at all.
	if checkCancellation && ctx.Done() != nil {
		steps.ctx = ctx
	}
	return runStackDeBruijnUnchecked(m, term, steps)
}
