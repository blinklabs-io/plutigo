// <nilaway skip stack-machine>
// The standalone NilAway driver times out on this large generic stack loop.
package cek

import (
	"math"

	"github.com/blinklabs-io/plutigo/syn"
)

type stackFrameKind uint8

const (
	frameAwaitArg stackFrameKind = iota
	frameAwaitArgLambda
	frameAwaitArgBuiltin
	frameAwaitFunTerm
	frameAwaitFunValue
	frameForce
	frameConstr
	frameCases
)

type stackFrame[T syn.Eval] struct {
	kind stackFrameKind

	value   Value[T]
	builtin *Builtin[T]
	env     *Env[T]
	term    syn.Term[T]

	tag            uint
	fields         []syn.Term[T]
	resolvedFields []Value[T]
	branches       []syn.Term[T]
}

func (m *Machine[T]) resetFrameStack() {
	if m.frameStackUsed > 0 {
		clear(m.frameStack[:m.frameStackUsed])
	}
	m.frameStack = m.frameStack[:0]
	m.frameStackUsed = 0
}

func (m *Machine[T]) pushFrame(frame stackFrame[T]) {
	m.frameStack = append(m.frameStack, frame)
	if len(m.frameStack) > m.frameStackUsed {
		m.frameStackUsed = len(m.frameStack)
	}
}

func (m *Machine[T]) pushFrameSlot() *stackFrame[T] {
	frameIdx := len(m.frameStack)
	if frameIdx < cap(m.frameStack) {
		m.frameStack = m.frameStack[:frameIdx+1]
	} else {
		m.frameStack = append(m.frameStack, stackFrame[T]{})
	}
	if len(m.frameStack) > m.frameStackUsed {
		m.frameStackUsed = len(m.frameStack)
	}
	return &m.frameStack[frameIdx]
}

func (m *Machine[T]) peekFrame() (*stackFrame[T], bool) {
	if len(m.frameStack) == 0 {
		return nil, false
	}
	return &m.frameStack[len(m.frameStack)-1], true
}

func (m *Machine[T]) pushApplyFrames(args []Value[T]) {
	for i := len(args) - 1; i >= 0; i-- {
		frame := m.pushFrameSlot()
		frame.kind = frameAwaitFunValue
		frame.value = args[i]
	}
}

func (m *Machine[T]) pushApplyFrameValue(arg Value[T]) {
	frame := m.pushFrameSlot()
	frame.kind = frameAwaitFunValue
	frame.value = arg
}

func (m *Machine[T]) pushApplyFrames2(first, second Value[T]) {
	m.pushApplyFrameValue(second)
	m.pushApplyFrameValue(first)
}

func (m *Machine[T]) pushAwaitArgFrame(funValue Value[T]) {
	frame := m.pushFrameSlot()
	switch f := funValue.(type) {
	case *Lambda[T]:
		frame.kind = frameAwaitArgLambda
		frame.env = f.Env
		frame.term = f.AST.Body
	case *Builtin[T]:
		frame.kind = frameAwaitArgBuiltin
		frame.builtin = f
	default:
		frame.kind = frameAwaitArg
		frame.value = funValue
	}
}

func isImmediateTerm[T syn.Eval](term syn.Term[T]) bool {
	switch t := term.(type) {
	case *syn.Apply[T], *syn.Force[T], *syn.Case[T]:
		return false
	case *syn.Constr[T]:
		return len(t.Fields) == 0
	default:
		return true
	}
}

func (m *Machine[T]) computeKnownImmediateValue(
	env *Env[T],
	term syn.Term[T],
) (Value[T], error) {
	switch t := term.(type) {
	case *syn.Var[T]:
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, err
		}
		value, ok := lookupEnv(env, t.Name.LookupIndex())
		if !ok {
			return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
		}
		return value, nil
	case *syn.Delay[T]:
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}
		return m.allocDelay(t, env), nil
	case *syn.Lambda[T]:
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}
		return m.allocLambda(t, env), nil
	case *syn.Constant:
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, err
		}
		return machineConstantValue(m, t.Con), nil
	case *syn.Error:
		return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
	case *syn.Builtin:
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, err
		}
		return m.builtinValues[t.DefaultFunction], nil
	case *syn.Constr[T]:
		if err := m.stepAndMaybeSpend(ExConstr); err != nil {
			return nil, err
		}
		return m.allocConstr(t.Tag, nil), nil
	default:
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "non-immediate term passed to computeKnownImmediateValue",
		}
	}
}

func (m *Machine[T]) spendStepNoSlippage(step StepKind) bool {
	memCost := m.stepCostMem[step]
	cpuCost := m.stepCostCpu[step]
	m.ExBudget.Mem -= memCost
	m.ExBudget.Cpu -= cpuCost
	return m.ExBudget.Mem >= 0 && m.ExBudget.Cpu >= 0
}

func (m *Machine[T]) budgetErrorForStep(step StepKind) *BudgetError {
	memCost := m.stepCostMem[step]
	cpuCost := m.stepCostCpu[step]
	return &BudgetError{
		Code: ErrCodeBudgetExhausted,
		Requested: ExBudget{
			Cpu: cpuCost,
			Mem: memCost,
		},
		Available: ExBudget{
			Cpu: m.ExBudget.Cpu + cpuCost,
			Mem: m.ExBudget.Mem + memCost,
		},
		Message: "out of budget",
	}
}

func (m *Machine[T]) computeKnownImmediateValueNoSlippage(
	env *Env[T],
	term syn.Term[T],
) (Value[T], error) {
	switch t := term.(type) {
	case *syn.Var[T]:
		if !m.spendStepNoSlippage(ExVar) {
			return nil, m.budgetErrorForStep(ExVar)
		}
		value, ok := lookupEnv(env, t.Name.LookupIndex())
		if !ok {
			return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
		}
		return value, nil
	case *syn.Delay[T]:
		if !m.spendStepNoSlippage(ExDelay) {
			return nil, m.budgetErrorForStep(ExDelay)
		}
		return m.allocDelay(t, env), nil
	case *syn.Lambda[T]:
		if !m.spendStepNoSlippage(ExLambda) {
			return nil, m.budgetErrorForStep(ExLambda)
		}
		return m.allocLambda(t, env), nil
	case *syn.Constant:
		if !m.spendStepNoSlippage(ExConstant) {
			return nil, m.budgetErrorForStep(ExConstant)
		}
		return machineConstantValue(m, t.Con), nil
	case *syn.Error:
		return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
	case *syn.Builtin:
		if !m.spendStepNoSlippage(ExBuiltin) {
			return nil, m.budgetErrorForStep(ExBuiltin)
		}
		return m.builtinValues[t.DefaultFunction], nil
	case *syn.Constr[T]:
		if !m.spendStepNoSlippage(ExConstr) {
			return nil, m.budgetErrorForStep(ExConstr)
		}
		return m.allocConstr(t.Tag, nil), nil
	default:
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "non-immediate term passed to computeKnownImmediateValueNoSlippage",
		}
	}
}

func (m *Machine[T]) runStackNoSlippage(term syn.Term[T]) (syn.Term[T], error) {
	var currentEnv *Env[T]
	currentTerm := term
	var currentValue Value[T]
	returning := false

	for {
		if !returning {
			switch t := currentTerm.(type) {
			case *syn.Var[T]:
				if !m.spendStepNoSlippage(ExVar) {
					return nil, m.budgetErrorForStep(ExVar)
				}

				value, ok := lookupEnv(currentEnv, t.Name.LookupIndex())
				if !ok {
					return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
				}

				currentValue = value
				returning = true
			case *syn.Delay[T]:
				if !m.spendStepNoSlippage(ExDelay) {
					return nil, m.budgetErrorForStep(ExDelay)
				}

				currentValue = m.allocDelay(t, currentEnv)
				returning = true
			case *syn.Lambda[T]:
				if !m.spendStepNoSlippage(ExLambda) {
					return nil, m.budgetErrorForStep(ExLambda)
				}

				currentValue = m.allocLambda(t, currentEnv)
				returning = true
			case *syn.Apply[T]:
				if !m.spendStepNoSlippage(ExApply) {
					return nil, m.budgetErrorForStep(ExApply)
				}

				if lambda, ok := t.Function.(*syn.Lambda[T]); ok {
					if !m.spendStepNoSlippage(ExLambda) {
						return nil, m.budgetErrorForStep(ExLambda)
					}
					if isImmediateTerm[T](t.Argument) {
						argValue, err := m.computeKnownImmediateValueNoSlippage(currentEnv, t.Argument)
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

				if isImmediateTerm[T](t.Function) {
					funValue, err := m.computeKnownImmediateValueNoSlippage(currentEnv, t.Function)
					if err != nil {
						return nil, err
					}

					if isImmediateTerm[T](t.Argument) {
						argValue, err := m.computeKnownImmediateValueNoSlippage(currentEnv, t.Argument)
						if err != nil {
							return nil, err
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
			case *syn.Constant:
				if !m.spendStepNoSlippage(ExConstant) {
					return nil, m.budgetErrorForStep(ExConstant)
				}

				currentValue = machineConstantValue(m, t.Con)
				returning = true
			case *syn.Force[T]:
				if !m.spendStepNoSlippage(ExForce) {
					return nil, m.budgetErrorForStep(ExForce)
				}

				if isImmediateTerm[T](t.Term) {
					forcedValue, err := m.computeKnownImmediateValueNoSlippage(currentEnv, t.Term)
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
			case *syn.Error:
				return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
			case *syn.Builtin:
				if !m.spendStepNoSlippage(ExBuiltin) {
					return nil, m.budgetErrorForStep(ExBuiltin)
				}

				currentValue = m.builtinValues[t.DefaultFunction]
				returning = true
			case *syn.Constr[T]:
				if !m.spendStepNoSlippage(ExConstr) {
					return nil, m.budgetErrorForStep(ExConstr)
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
			case *syn.Case[T]:
				if !m.spendStepNoSlippage(ExCase) {
					return nil, m.budgetErrorForStep(ExCase)
				}

				if isImmediateTerm[T](t.Constr) {
					scrutinee, err := m.computeKnownImmediateValueNoSlippage(currentEnv, t.Constr)
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
				panic("unknown term")
			}

			continue
		}

		if len(m.frameStack) == 0 {
			return m.finishValue(currentValue)
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
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(builtinValue, currentValue)
			if err != nil {
				return nil, err
			}
		case frameAwaitFunTerm:
			env := frame.env
			term := frame.term
			m.frameStack = m.frameStack[:frameIdx]

			if isImmediateTerm[T](term) {
				argValue, err := m.computeKnownImmediateValueNoSlippage(env, term)
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
			panic("unknown stack frame")
		}
	}
}

func (m *Machine[T]) runStack(term syn.Term[T]) (syn.Term[T], error) {
	var currentEnv *Env[T]
	currentTerm := term
	var currentValue Value[T]
	returning := false

	for {
		if !returning {
			switch t := currentTerm.(type) {
			case *syn.Var[T]:
				if err := m.stepAndMaybeSpend(ExVar); err != nil {
					return nil, err
				}

				value, ok := lookupEnv(currentEnv, t.Name.LookupIndex())
				if !ok {
					return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
				}

				currentValue = value
				returning = true
			case *syn.Delay[T]:
				if err := m.stepAndMaybeSpend(ExDelay); err != nil {
					return nil, err
				}

				currentValue = m.allocDelay(t, currentEnv)
				returning = true
			case *syn.Lambda[T]:
				if err := m.stepAndMaybeSpend(ExLambda); err != nil {
					return nil, err
				}

				currentValue = m.allocLambda(t, currentEnv)
				returning = true
			case *syn.Apply[T]:
				if err := m.stepAndMaybeSpend(ExApply); err != nil {
					return nil, err
				}

				if lambda, ok := t.Function.(*syn.Lambda[T]); ok {
					if err := m.stepAndMaybeSpend(ExLambda); err != nil {
						return nil, err
					}
					if isImmediateTerm[T](t.Argument) {
						argValue, err := m.computeKnownImmediateValue(currentEnv, t.Argument)
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

				if isImmediateTerm[T](t.Function) {
					funValue, err := m.computeKnownImmediateValue(currentEnv, t.Function)
					if err != nil {
						return nil, err
					}

					if isImmediateTerm[T](t.Argument) {
						argValue, err := m.computeKnownImmediateValue(currentEnv, t.Argument)
						if err != nil {
							return nil, err
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
			case *syn.Constant:
				if err := m.stepAndMaybeSpend(ExConstant); err != nil {
					return nil, err
				}

				currentValue = machineConstantValue(m, t.Con)
				returning = true
			case *syn.Force[T]:
				if err := m.stepAndMaybeSpend(ExForce); err != nil {
					return nil, err
				}

				if isImmediateTerm[T](t.Term) {
					forcedValue, err := m.computeKnownImmediateValue(currentEnv, t.Term)
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
			case *syn.Error:
				return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
			case *syn.Builtin:
				if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
					return nil, err
				}

				currentValue = m.builtinValues[t.DefaultFunction]
				returning = true
			case *syn.Constr[T]:
				if err := m.stepAndMaybeSpend(ExConstr); err != nil {
					return nil, err
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
			case *syn.Case[T]:
				if err := m.stepAndMaybeSpend(ExCase); err != nil {
					return nil, err
				}

				if isImmediateTerm[T](t.Constr) {
					scrutinee, err := m.computeKnownImmediateValue(currentEnv, t.Constr)
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
				panic("unknown term")
			}

			continue
		}

		if len(m.frameStack) == 0 {
			return m.finishValue(currentValue)
		}
		frameIdx := len(m.frameStack) - 1
		frame := &m.frameStack[frameIdx]

		switch frame.kind {
		case frameAwaitArg:
			function := frame.value
			frame.value = nil
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
			frame.env = nil
			frame.term = nil
			m.frameStack = m.frameStack[:frameIdx]

			currentTerm = body
			currentEnv = m.extendEnv(env, currentValue)
			currentValue = nil
			returning = false
		case frameAwaitArgBuiltin:
			builtinValue := frame.builtin
			frame.builtin = nil
			m.frameStack = m.frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(builtinValue, currentValue)
			if err != nil {
				return nil, err
			}
		case frameAwaitFunTerm:
			env := frame.env
			term := frame.term
			frame.env = nil
			frame.term = nil
			m.frameStack = m.frameStack[:frameIdx]

			if isImmediateTerm[T](term) {
				argValue, err := m.computeKnownImmediateValue(env, term)
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
			frame.value = nil
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
				frame.env = nil
				frame.fields = nil
				frame.resolvedFields = nil
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
			frame.env = nil
			frame.branches = nil
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
			panic("unknown stack frame")
		}
	}
}

func (m *Machine[T]) applyEvaluateStack(
	function Value[T],
	arg Value[T],
) (syn.Term[T], *Env[T], Value[T], bool, error) {
	switch f := function.(type) {
	case *Lambda[T]:
		return f.AST.Body, m.extendEnv(f.Env, arg), nil, false, nil
	case *Builtin[T]:
		// Cache the per-builtin force/arity table lookups so we don't repeat
		// them for each branch below.
		forceCount := f.Func.ForceCount()
		arity := f.Func.Arity()
		if forceCount <= f.Forces && arity > f.ArgCount {
			nextArgCount := f.ArgCount + 1
			if forceCount == f.Forces {
				switch nextArgCount {
				case 1:
					if resolved, handled, err := m.evalUnaryBuiltinFast(f.Func, arg); handled {
						if err != nil {
							return nil, nil, nil, false, err
						}
						return nil, nil, resolved, true, nil
					}
				case 2:
					if f.Args != nil {
						if resolved, handled, err := m.evalBinaryBuiltinFast(f.Func, f.Args.data, arg); handled {
							if err != nil {
								return nil, nil, nil, false, err
							}
							return nil, nil, resolved, true, nil
						}
					}
				case 3:
					if f.Args != nil && f.Args.next != nil {
						if resolved, handled, err := m.evalTernaryBuiltinFast(
							f.Func,
							f.Args.next.data,
							f.Args.data,
							arg,
						); handled {
							if err != nil {
								return nil, nil, nil, false, err
							}
							return nil, nil, resolved, true, nil
						}
					}
				}
				if arity == nextArgCount {
					resolved, err := m.evalBuiltinAppWithArg(
						f.Func,
						f.Forces,
						nextArgCount,
						f.Args,
						arg,
					)
					if err != nil {
						return nil, nil, nil, false, err
					}
					return nil, nil, resolved, true, nil
				}
			}
			nextArgs := m.extendBuiltinArgs(f.Args, arg)
			return nil, nil, m.allocBuiltin(f.Func, f.Forces, nextArgCount, nextArgs), true, nil
		}
		return nil, nil, nil, false, &TypeError{
			Code:    ErrCodeUnexpectedBuiltinArg,
			Message: "UnexpectedBuiltinTermArgument",
		}
	default:
		return nil, nil, nil, false, &TypeError{
			Code:    ErrCodeNonFunctionalApp,
			Message: "NonFunctionalApplication",
		}
	}
}

func (m *Machine[T]) forceEvaluateStack(
	value Value[T],
) (syn.Term[T], *Env[T], Value[T], bool, error) {
	switch v := value.(type) {
	case *Delay[T]:
		return v.AST.Term, v.Env, nil, false, nil
	case *Builtin[T]:
		forceCount := v.Func.ForceCount()
		if forceCount > v.Forces {
			nextForces := v.Forces + 1
			if forceCount == nextForces && v.Func.Arity() == v.ArgCount {
				resolved, err := m.evalBuiltinAppReady(
					v.Func,
					nextForces,
					v.ArgCount,
					v.Args,
				)
				if err != nil {
					return nil, nil, nil, false, err
				}
				return nil, nil, resolved, true, nil
			}
			return nil, nil, m.allocBuiltin(v.Func, nextForces, v.ArgCount, v.Args), true, nil
		}
		return nil, nil, nil, false, &TypeError{
			Code:    ErrCodeBuiltinForceExpected,
			Message: "BuiltinTermArgumentExpected",
		}
	default:
		return nil, nil, nil, false, &TypeError{
			Code:    ErrCodeNonPolymorphic,
			Message: "NonPolymorphicInstantiation",
		}
	}
}

func (m *Machine[T]) caseEvaluateStack(
	env *Env[T],
	branches []syn.Term[T],
	value Value[T],
) (syn.Term[T], *Env[T], Value[T], bool, error) {
	switch v := value.(type) {
	case *Constr[T]:
		if v.Tag > math.MaxInt {
			return nil, nil, nil, false, &ScriptError{
				Code:    ErrCodeMaxIntExceeded,
				Message: "MaxIntExceeded",
			}
		}
		if !indexExists(branches, int(v.Tag)) {
			return nil, nil, nil, false, &ScriptError{
				Code:    ErrCodeMissingCaseBranch,
				Message: "MissingCaseBranch",
			}
		}

		m.pushApplyFrames(v.Fields)
		return branches[v.Tag], env, nil, false, nil
	case *Constant:
		var tag int
		var firstArg Value[T]
		var secondArg Value[T]
		branchRule := 0

		switch cval := v.Constant.(type) {
		case *syn.Bool:
			branchRule = 2
			if cval.Inner {
				tag = 1
			} else {
				tag = 0
			}
		case *syn.Unit:
			branchRule = 1
			tag = 0
		case *syn.Integer:
			if cval.Inner.Sign() < 0 {
				return nil, nil, nil, false, &ScriptError{
					Code:    ErrCodeCaseOnNegativeInt,
					Message: "case on negative integer",
				}
			}
			if !cval.Inner.IsInt64() {
				return nil, nil, nil, false, &ScriptError{
					Code:    ErrCodeCaseIntOutOfRange,
					Message: "case on integer out of range",
				}
			}
			ival := cval.Inner.Int64()
			if ival > int64(math.MaxInt) {
				return nil, nil, nil, false, &ScriptError{
					Code:    ErrCodeCaseIntOutOfRange,
					Message: "case on integer out of range",
				}
			}
			tag = int(ival)
		case *syn.ByteString:
			return nil, nil, nil, false, &ScriptError{
				Code:    ErrCodeCaseOnByteString,
				Message: "case on bytestring constant not allowed",
			}
		case *syn.ProtoList:
			branchRule = 2
			if len(cval.List) == 0 {
				tag = 1
			} else {
				tag = 0
				firstArg = m.allocConstant(cval.List[0])
				tail := m.allocProtoListConstant(cval.LTyp, cval.List[1:])
				secondArg = m.allocConstant(tail)
			}
		case *syn.ProtoPair:
			branchRule = 1
			tag = 0
			firstArg = m.allocConstant(cval.First)
			secondArg = m.allocConstant(cval.Second)
		default:
			return nil, nil, nil, false, &TypeError{
				Code:    ErrCodeNonConstrScrutinized,
				Message: "NonConstrScrutinized",
			}
		}

		switch branchRule {
		case 1:
			if len(branches) != 1 {
				return nil, nil, nil, false, &ScriptError{
					Code:    ErrCodeInvalidBranchCount,
					Message: "InvalidCaseBranchCount",
				}
			}
		case 2:
			if len(branches) < 1 || len(branches) > 2 {
				return nil, nil, nil, false, &ScriptError{
					Code:    ErrCodeInvalidBranchCount,
					Message: "InvalidCaseBranchCount",
				}
			}
		}

		if !indexExists(branches, tag) {
			return nil, nil, nil, false, &ScriptError{
				Code:    ErrCodeMissingCaseBranch,
				Message: "MissingCaseBranch",
			}
		}

		if firstArg != nil {
			m.pushApplyFrames2(firstArg, secondArg)
		}
		return branches[tag], env, nil, false, nil
	case *dataListValue[T]:
		if len(branches) < 1 || len(branches) > 2 {
			return nil, nil, nil, false, &ScriptError{
				Code:    ErrCodeInvalidBranchCount,
				Message: "InvalidCaseBranchCount",
			}
		}
		if len(v.items) == 0 {
			if !indexExists(branches, 1) {
				return nil, nil, nil, false, &ScriptError{
					Code:    ErrCodeMissingCaseBranch,
					Message: "MissingCaseBranch",
				}
			}
			return branches[1], env, nil, false, nil
		}
		m.pushApplyFrames2(
			m.allocDataValue(v.items[0]),
			m.allocDataListValue(v.items[1:]),
		)
		return branches[0], env, nil, false, nil
	case *dataMapValue[T]:
		if len(branches) < 1 || len(branches) > 2 {
			return nil, nil, nil, false, &ScriptError{
				Code:    ErrCodeInvalidBranchCount,
				Message: "InvalidCaseBranchCount",
			}
		}
		if len(v.items) == 0 {
			if !indexExists(branches, 1) {
				return nil, nil, nil, false, &ScriptError{
					Code:    ErrCodeMissingCaseBranch,
					Message: "MissingCaseBranch",
				}
			}
			return branches[1], env, nil, false, nil
		}
		m.pushApplyFrames2(
			m.allocDataPairValue(v.items[0][0], v.items[0][1]),
			m.allocDataMapValue(v.items[1:]),
		)
		return branches[0], env, nil, false, nil
	case *pairValue[T]:
		if len(branches) != 1 {
			return nil, nil, nil, false, &ScriptError{
				Code:    ErrCodeInvalidBranchCount,
				Message: "InvalidCaseBranchCount",
			}
		}
		m.pushApplyFrames2(v.first, v.second)
		return branches[0], env, nil, false, nil
	default:
		return nil, nil, nil, false, &TypeError{
			Code:    ErrCodeNonConstrScrutinized,
			Message: "NonConstrScrutinized",
		}
	}
}
