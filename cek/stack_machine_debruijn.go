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
	// Select the loop once per run so Run's hot path carries no context state.
	if checkCancellation {
		return runStackDeBruijnChecked(ctx, m, term, steps)
	}
	return runStackDeBruijnUnchecked(m, term, steps)
}

func runStackDeBruijnChecked(
	ctx context.Context,
	m *Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
	steps *stepCounter,
) (syn.Term[syn.DeBruijn], error) {
	var currentEnv *Env[syn.DeBruijn]
	currentTerm := term
	var currentValue Value[syn.DeBruijn]
	returning := false

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !returning {
			if currentTerm == nil {
				return nil, internalError("DeBruijn stack machine current term is nil")
			}
			termIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&currentTerm))
			switch termIface.tab {
			case varTermTabDeBruijn:
				t := (*syn.Var[syn.DeBruijn])(termIface.data)
				if !steps.count(ExVar) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				value, ok := lookupEnvDeBruijn(currentEnv, int(t.Name))
				if !ok {
					return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
				}
				if value == nil {
					return nil, internalError("DeBruijn environment lookup returned nil value")
				}

				currentValue = value
				returning = true
			case delayTermTabDeBruijn:
				t := (*syn.Delay[syn.DeBruijn])(termIface.data)
				if !steps.count(ExDelay) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				currentValue = m.allocDelay(t, currentEnv)
				returning = true
			case lambdaTermTabDeBruijn:
				t := (*syn.Lambda[syn.DeBruijn])(termIface.data)
				if !steps.count(ExLambda) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				currentValue = m.allocLambda(t, currentEnv)
				returning = true
			case applyTermTabDeBruijn:
				t := (*syn.Apply[syn.DeBruijn])(termIface.data)
				if !steps.count(ExApply) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				funcIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&t.Function))
				if funcIface.tab == lambdaTermTabDeBruijn {
					lambda := (*syn.Lambda[syn.DeBruijn])(funcIface.data)
					if !steps.count(ExLambda) {
						if err := steps.spend(m); err != nil {
							return nil, err
						}
					}
					if isImmediateTermDeBruijn(t.Argument) {
						argValue, err := computeKnownImmediateValueDeBruijn(
							m,
							steps,
							currentEnv,
							t.Argument,
						)
						if err != nil {
							return nil, err
						}
						currentTerm = lambda.Body
						currentEnv = m.extendEnv(currentEnv, argValue)
						currentValue = nil
						returning = false
						continue
					}
					frame := m.pushFrameSlot()
					frame.kind = frameAwaitArgLambda
					frame.env = currentEnv
					frame.term = lambda.Body
					currentTerm = t.Argument
					continue
				}

				if isImmediateTermDeBruijn(t.Function) {
					funValue, err := computeKnownImmediateValueDeBruijn(
						m,
						steps,
						currentEnv,
						t.Function,
					)
					if err != nil {
						return nil, err
					}

					if isImmediateTermDeBruijn(t.Argument) {
						argValue, err := computeKnownImmediateValueDeBruijn(
							m,
							steps,
							currentEnv,
							t.Argument,
						)
						if err != nil {
							return nil, err
						}
						// Fast path: when funValue is a Lambda value we can
						// extend its env and continue without paying the
						// function-call + 5-return-slot cost of
						// applyEvaluateStack.
						if lambdaValue, ok := funValue.(*Lambda[syn.DeBruijn]); ok {
							currentTerm = lambdaValue.AST.Body
							currentEnv = m.extendEnv(lambdaValue.Env, argValue)
							currentValue = nil
							returning = false
							continue
						}
						currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
							funValue,
							argValue,
						)
						if err != nil {
							return nil, err
						}
						continue
					}
					m.pushAwaitArgFrame(funValue)
					currentTerm = t.Argument
					continue
				}

				frame := m.pushFrameSlot()
				frame.kind = frameAwaitFunTerm
				frame.env = currentEnv
				frame.term = t.Argument
				currentTerm = t.Function
			case constantTermTab:
				t := (*syn.Constant)(termIface.data)
				if !steps.count(ExConstant) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				currentValue = machineConstantValue(m, t.Con)
				returning = true
			case forceTermTabDeBruijn:
				t := (*syn.Force[syn.DeBruijn])(termIface.data)
				if !steps.count(ExForce) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				termIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&t.Term))
				if termIface.tab == builtinTermTabDeBruijn {
					builtinTerm := (*syn.Builtin)(termIface.data)
					if !steps.count(ExBuiltin) {
						if err := steps.spend(m); err != nil {
							return nil, err
						}
					}
					var err error
					currentTerm, currentEnv, currentValue, returning, err = m.forceEvaluateStack(
						m.builtinNoArgValues[builtinTerm.DefaultFunction][0],
					)
					if err != nil {
						return nil, err
					}
					continue
				}

				if isImmediateTermDeBruijn(t.Term) {
					forcedValue, err := computeKnownImmediateValueDeBruijn(
						m,
						steps,
						currentEnv,
						t.Term,
					)
					if err != nil {
						return nil, err
					}
					currentTerm, currentEnv, currentValue, returning, err = m.forceEvaluateStack(
						forcedValue,
					)
					if err != nil {
						return nil, err
					}
					continue
				}

				frame := m.pushFrameSlot()
				frame.kind = frameForce
				currentTerm = t.Term
			case errorTermTab:
				return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
			case builtinTermTabDeBruijn:
				t := (*syn.Builtin)(termIface.data)
				if !steps.count(ExBuiltin) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				currentValue = m.builtinNoArgValues[t.DefaultFunction][0]
				returning = true
			case constrTermTabDeBruijn:
				t := (*syn.Constr[syn.DeBruijn])(termIface.data)
				if !steps.count(ExConstr) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				if len(t.Fields) == 0 {
					currentValue = m.allocConstr(t.Tag, nil)
					returning = true
					continue
				}

				frame := m.pushFrameSlot()
				frame.kind = frameConstr
				frame.env = currentEnv
				frame.tag = t.Tag
				frame.fields = t.Fields[1:]
				frame.resolvedFields = m.allocValueElems(len(t.Fields))[:0]
				currentTerm = t.Fields[0]
			case caseTermTabDeBruijn:
				t := (*syn.Case[syn.DeBruijn])(termIface.data)
				if !steps.count(ExCase) {
					if err := steps.spend(m); err != nil {
						return nil, err
					}
				}

				if isImmediateTermDeBruijn(t.Constr) {
					scrutinee, err := computeKnownImmediateValueDeBruijn(
						m,
						steps,
						currentEnv,
						t.Constr,
					)
					if err != nil {
						return nil, err
					}
					currentTerm, currentEnv, currentValue, returning, err = m.caseEvaluateStack(
						currentEnv,
						t.Branches,
						scrutinee,
					)
					if err != nil {
						return nil, err
					}
					continue
				}

				frame := m.pushFrameSlot()
				frame.kind = frameCases
				frame.env = currentEnv
				frame.branches = t.Branches
				currentTerm = t.Constr
			default:
				return nil, &InternalError{
					Code:    ErrCodeInternalError,
					Message: "unknown term in DeBruijn evaluator",
				}
			}

			continue
		}

		if currentValue == nil {
			return nil, internalError("DeBruijn stack machine current value is nil")
		}
		if len(m.frameStack) == 0 {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
			return m.finishValueContext(ctx, true, currentValue)
		}
		frameIdx := len(m.frameStack) - 1
		frame := &m.frameStack[frameIdx]

		switch frame.kind {
		case frameAwaitArg:
			function := frame.value
			m.frameStack = m.frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				function,
				currentValue,
			)
			if err != nil {
				return nil, err
			}
		case frameAwaitArgLambda:
			env := frame.env
			body := frame.term
			m.frameStack = m.frameStack[:frameIdx]

			currentTerm = body
			currentEnv = m.extendEnv(env, currentValue)
			currentValue = nil
			returning = false
		case frameAwaitArgBuiltin:
			builtinValue := frame.builtin
			m.frameStack = m.frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				builtinValue,
				currentValue,
			)
			if err != nil {
				return nil, err
			}
		case frameAwaitFunTerm:
			env := frame.env
			term := frame.term
			m.frameStack = m.frameStack[:frameIdx]

			if isImmediateTermDeBruijn(term) {
				argValue, err := computeKnownImmediateValueDeBruijn(m, steps, env, term)
				if err != nil {
					return nil, err
				}
				currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
					currentValue,
					argValue,
				)
				if err != nil {
					return nil, err
				}
				continue
			}

			m.pushAwaitArgFrame(currentValue)
			currentEnv = env
			currentTerm = term
			returning = false
		case frameAwaitFunValue:
			arg := frame.value
			m.frameStack = m.frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				currentValue,
				arg,
			)
			if err != nil {
				return nil, err
			}
		case frameForce:
			m.frameStack = m.frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.forceEvaluateStack(
				currentValue,
			)
			if err != nil {
				return nil, err
			}
		case frameConstr:
			frame.resolvedFields = append(frame.resolvedFields, currentValue)
			if len(frame.fields) == 0 {
				resolvedFields := frame.resolvedFields
				tag := frame.tag
				m.frameStack = m.frameStack[:frameIdx]

				currentValue = m.allocConstr(tag, resolvedFields)
				returning = true
				continue
			}

			nextField := frame.fields[0]
			frame.fields = frame.fields[1:]
			currentEnv = frame.env
			currentTerm = nextField
			returning = false
		case frameCases:
			env := frame.env
			branches := frame.branches
			m.frameStack = m.frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.caseEvaluateStack(
				env,
				branches,
				currentValue,
			)
			if err != nil {
				return nil, err
			}
		default:
			return nil, &InternalError{
				Code:    ErrCodeInternalError,
				Message: "unknown stack frame in DeBruijn evaluator",
			}
		}
	}
}
