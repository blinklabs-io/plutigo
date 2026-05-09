// <nilaway skip stack-machine>
package cek

import "github.com/blinklabs-io/plutigo/syn"

func isImmediateTermDeBruijn(term syn.Term[syn.DeBruijn]) bool {
	switch t := term.(type) {
	case *syn.Apply[syn.DeBruijn], *syn.Force[syn.DeBruijn], *syn.Case[syn.DeBruijn]:
		return false
	case *syn.Constr[syn.DeBruijn]:
		return len(t.Fields) == 0
	default:
		return true
	}
}

func lookupEnvDeBruijn(
	env *Env[syn.DeBruijn],
	idx int,
) (Value[syn.DeBruijn], bool) {
	var zero Value[syn.DeBruijn]
	if idx <= 0 {
		return zero, false
	}
	if env == nil {
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
		env = advanceEnv4DeBruijn(env)
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 6:
		env = advanceEnv4DeBruijn(env)
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 7:
		env = advanceEnv4DeBruijn(env)
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
	case 8:
		env = advanceEnv4DeBruijn(env)
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

	current := env
	remaining := idx - 1
	for remaining >= 4 {
		current = advanceEnv4DeBruijn(current)
		if current == nil {
			return zero, false
		}
		remaining -= 4
	}
	for remaining > 0 {
		current = current.next
		if current == nil {
			return zero, false
		}
		remaining--
	}

	return current.data, true
}

func advanceEnv4DeBruijn(env *Env[syn.DeBruijn]) *Env[syn.DeBruijn] {
	if env == nil {
		return nil
	}
	if env.skip4 != nil {
		return env.skip4
	}
	for range 4 {
		env = env.next
		if env == nil {
			return nil
		}
	}
	return env
}

func computeKnownImmediateValueNoSlippageDeBruijn(
	m *Machine[syn.DeBruijn],
	env *Env[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
) (Value[syn.DeBruijn], error) {
	switch t := term.(type) {
	case *syn.Var[syn.DeBruijn]:
		if !m.spendStepNoSlippage(ExVar) {
			return nil, m.budgetErrorForStep(ExVar)
		}
		value, ok := lookupEnvDeBruijn(env, int(t.Name))
		if !ok {
			return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
		}
		return value, nil
	case *syn.Delay[syn.DeBruijn]:
		if !m.spendStepNoSlippage(ExDelay) {
			return nil, m.budgetErrorForStep(ExDelay)
		}
		return m.allocDelay(t, env), nil
	case *syn.Lambda[syn.DeBruijn]:
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
	case *syn.Constr[syn.DeBruijn]:
		if !m.spendStepNoSlippage(ExConstr) {
			return nil, m.budgetErrorForStep(ExConstr)
		}
		return m.allocConstr(t.Tag, nil), nil
	default:
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "non-immediate term passed to computeKnownImmediateValueNoSlippageDeBruijn",
		}
	}
}

func runStackNoSlippageDeBruijn(
	m *Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
) (syn.Term[syn.DeBruijn], error) {
	var currentEnv *Env[syn.DeBruijn]
	currentTerm := term
	var currentValue Value[syn.DeBruijn]
	returning := false

	for {
		if !returning {
			switch t := currentTerm.(type) {
			case *syn.Var[syn.DeBruijn]:
				if !m.spendStepNoSlippage(ExVar) {
					return nil, m.budgetErrorForStep(ExVar)
				}

				value, ok := lookupEnvDeBruijn(currentEnv, int(t.Name))
				if !ok {
					return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
				}

				currentValue = value
				returning = true
			case *syn.Delay[syn.DeBruijn]:
				if !m.spendStepNoSlippage(ExDelay) {
					return nil, m.budgetErrorForStep(ExDelay)
				}

				currentValue = m.allocDelay(t, currentEnv)
				returning = true
			case *syn.Lambda[syn.DeBruijn]:
				if !m.spendStepNoSlippage(ExLambda) {
					return nil, m.budgetErrorForStep(ExLambda)
				}

				currentValue = m.allocLambda(t, currentEnv)
				returning = true
			case *syn.Apply[syn.DeBruijn]:
				if !m.spendStepNoSlippage(ExApply) {
					return nil, m.budgetErrorForStep(ExApply)
				}

				if lambda, ok := t.Function.(*syn.Lambda[syn.DeBruijn]); ok {
					if !m.spendStepNoSlippage(ExLambda) {
						return nil, m.budgetErrorForStep(ExLambda)
					}
					if isImmediateTermDeBruijn(t.Argument) {
						argValue, err := computeKnownImmediateValueNoSlippageDeBruijn(
							m,
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
					funValue, err := computeKnownImmediateValueNoSlippageDeBruijn(
						m,
						currentEnv,
						t.Function,
					)
					if err != nil {
						return nil, err
					}

					if isImmediateTermDeBruijn(t.Argument) {
						argValue, err := computeKnownImmediateValueNoSlippageDeBruijn(
							m,
							currentEnv,
							t.Argument,
						)
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
			case *syn.Force[syn.DeBruijn]:
				if !m.spendStepNoSlippage(ExForce) {
					return nil, m.budgetErrorForStep(ExForce)
				}

				if builtinTerm, ok := t.Term.(*syn.Builtin); ok {
					if !m.spendStepNoSlippage(ExBuiltin) {
						return nil, m.budgetErrorForStep(ExBuiltin)
					}
					var err error
					currentTerm, currentEnv, currentValue, returning, err = m.forceEvaluateStack(
						m.builtinValues[builtinTerm.DefaultFunction],
					)
					if err != nil {
						return nil, err
					}
					continue
				}

				if isImmediateTermDeBruijn(t.Term) {
					forcedValue, err := computeKnownImmediateValueNoSlippageDeBruijn(
						m,
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
			case *syn.Error:
				return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
			case *syn.Builtin:
				if !m.spendStepNoSlippage(ExBuiltin) {
					return nil, m.budgetErrorForStep(ExBuiltin)
				}

				currentValue = m.builtinValues[t.DefaultFunction]
				returning = true
			case *syn.Constr[syn.DeBruijn]:
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
			case *syn.Case[syn.DeBruijn]:
				if !m.spendStepNoSlippage(ExCase) {
					return nil, m.budgetErrorForStep(ExCase)
				}

				if isImmediateTermDeBruijn(t.Constr) {
					scrutinee, err := computeKnownImmediateValueNoSlippageDeBruijn(
						m,
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
				argValue, err := computeKnownImmediateValueNoSlippageDeBruijn(m, env, term)
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
