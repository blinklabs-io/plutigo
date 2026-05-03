// <nilaway skip stack-machine>
package cek

import (
	"unsafe"

	"github.com/blinklabs-io/plutigo/syn"
)

var nilControlTermDeBruijn syn.Term[syn.DeBruijn] = &syn.Error{}

type termInterfaceDeBruijn struct {
	tab  unsafe.Pointer
	data unsafe.Pointer
}

var (
	applyTermTabDeBruijn  = termTabDeBruijn(&syn.Apply[syn.DeBruijn]{})
	forceTermTabDeBruijn  = termTabDeBruijn(&syn.Force[syn.DeBruijn]{})
	caseTermTabDeBruijn   = termTabDeBruijn(&syn.Case[syn.DeBruijn]{})
	constrTermTabDeBruijn = termTabDeBruijn(&syn.Constr[syn.DeBruijn]{})
)

func termTabDeBruijn(term syn.Term[syn.DeBruijn]) unsafe.Pointer {
	return (*termInterfaceDeBruijn)(unsafe.Pointer(&term)).tab
}

func isImmediateTermDeBruijn(term syn.Term[syn.DeBruijn]) bool {
	termIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&term))
	switch termIface.tab {
	case applyTermTabDeBruijn, forceTermTabDeBruijn, caseTermTabDeBruijn:
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

func pushFrameSlotDeBruijn(
	frameStack []stackFrame[syn.DeBruijn],
	frameStackUsed int,
) ([]stackFrame[syn.DeBruijn], int, *stackFrame[syn.DeBruijn]) {
	frameIdx := len(frameStack)
	if frameIdx < cap(frameStack) {
		frameStack = frameStack[:frameIdx+1]
	} else {
		frameStack = append(frameStack, stackFrame[syn.DeBruijn]{})
	}
	if len(frameStack) > frameStackUsed {
		frameStackUsed = len(frameStack)
	}
	return frameStack, frameStackUsed, &frameStack[frameIdx]
}

func pushAwaitArgFrameDeBruijn(
	frameStack []stackFrame[syn.DeBruijn],
	frameStackUsed int,
	funValue Value[syn.DeBruijn],
) ([]stackFrame[syn.DeBruijn], int) {
	var frame *stackFrame[syn.DeBruijn]
	frameStack, frameStackUsed, frame = pushFrameSlotDeBruijn(
		frameStack,
		frameStackUsed,
	)
	switch f := funValue.(type) {
	case *Lambda[syn.DeBruijn]:
		frame.kind = frameAwaitArgLambda
		frame.env = f.Env
		frame.term = f.AST.Body
	case *Builtin[syn.DeBruijn]:
		frame.kind = frameAwaitArgBuiltin
		frame.builtin = f
	default:
		frame.kind = frameAwaitArg
		frame.value = funValue
	}
	return frameStack, frameStackUsed
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
	envChunkPos := m.envChunkPos
	envActiveChunk := m.envActiveChunk
	envActiveChunkLimit := m.envActiveChunkLimit
	syncEnvArena := func() {
		m.envChunkPos = envChunkPos
		m.envActiveChunk = envActiveChunk
		m.envActiveChunkLimit = envActiveChunkLimit
	}
	defer syncEnvArena()
	extendEnvLocal := func(parent *Env[syn.DeBruijn], data Value[syn.DeBruijn]) *Env[syn.DeBruijn] {
		pos := envChunkPos
		chunk := envActiveChunk
		if chunk == nil || pos == envActiveChunkLimit {
			chunkIdx := pos / envChunkSize
			if chunkIdx == len(m.envChunks) {
				m.envChunks = append(m.envChunks, make([]Env[syn.DeBruijn], envChunkSize))
			}
			chunk = m.envChunks[chunkIdx]
			if chunk == nil {
				chunk = make([]Env[syn.DeBruijn], envChunkSize)
				m.envChunks[chunkIdx] = chunk
			}
			envActiveChunk = chunk
			envActiveChunkLimit = (chunkIdx + 1) * envChunkSize
		}
		env := &chunk[pos%envChunkSize]
		envChunkPos = pos + 1
		env.data = data
		env.next = parent
		if parent != nil {
			skip := parent.next
			if skip != nil {
				skip = skip.next
				if skip != nil {
					env.skip4 = skip.next
				}
			}
		}
		return env
	}
	frameStack := m.frameStack
	frameStackUsed := m.frameStackUsed
	syncFrameStack := func() {
		m.frameStack = frameStack
		m.frameStackUsed = frameStackUsed
	}
	defer syncFrameStack()

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
						currentEnv = extendEnvLocal(currentEnv, argValue)
						currentValue = nil
						returning = false
						continue
					}
					var frame *stackFrame[syn.DeBruijn]
					frameStack, frameStackUsed, frame = pushFrameSlotDeBruijn(
						frameStack,
						frameStackUsed,
					)
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
						if lambdaValue, ok := funValue.(*Lambda[syn.DeBruijn]); ok {
							currentTerm = lambdaValue.AST.Body
							currentEnv = extendEnvLocal(lambdaValue.Env, argValue)
							currentValue = nil
							returning = false
						} else {
							currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
								funValue,
								argValue,
							)
						}
						if err != nil {
							return nil, err
						}
						if currentTerm == nil {
							if !returning {
								return nil, &InternalError{
									Code:    ErrCodeInternalError,
									Message: "nil control term in DeBruijn evaluator",
								}
							}
							currentTerm = nilControlTermDeBruijn
						}
						continue
					}

					frameStack, frameStackUsed = pushAwaitArgFrameDeBruijn(
						frameStack,
						frameStackUsed,
						funValue,
					)
					currentTerm = t.Argument
					continue
				}

				var frame *stackFrame[syn.DeBruijn]
				frameStack, frameStackUsed, frame = pushFrameSlotDeBruijn(
					frameStack,
					frameStackUsed,
				)
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
					if currentTerm == nil {
						if !returning {
							return nil, &InternalError{
								Code:    ErrCodeInternalError,
								Message: "nil control term in DeBruijn evaluator",
							}
						}
						currentTerm = nilControlTermDeBruijn
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
					if currentTerm == nil {
						if !returning {
							return nil, &InternalError{
								Code:    ErrCodeInternalError,
								Message: "nil control term in DeBruijn evaluator",
							}
						}
						currentTerm = nilControlTermDeBruijn
					}
					continue
				}

				var frame *stackFrame[syn.DeBruijn]
				frameStack, frameStackUsed, frame = pushFrameSlotDeBruijn(
					frameStack,
					frameStackUsed,
				)
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

				var frame *stackFrame[syn.DeBruijn]
				frameStack, frameStackUsed, frame = pushFrameSlotDeBruijn(
					frameStack,
					frameStackUsed,
				)
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
					syncFrameStack()
					currentTerm, currentEnv, currentValue, returning, err = m.caseEvaluateStack(
						currentEnv,
						t.Branches,
						scrutinee,
					)
					frameStack = m.frameStack
					frameStackUsed = m.frameStackUsed
					if err != nil {
						return nil, err
					}
					if currentTerm == nil {
						if !returning {
							return nil, &InternalError{
								Code:    ErrCodeInternalError,
								Message: "nil control term in DeBruijn evaluator",
							}
						}
						currentTerm = nilControlTermDeBruijn
					}
					continue
				}

				var frame *stackFrame[syn.DeBruijn]
				frameStack, frameStackUsed, frame = pushFrameSlotDeBruijn(
					frameStack,
					frameStackUsed,
				)
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

		n := len(frameStack)
		if n == 0 {
			return m.finishValue(currentValue)
		}
		frameIdx := n - 1
		frame := &frameStack[frameIdx]

		switch frame.kind {
		case frameAwaitArg:
			function := frame.value
			frameStack = frameStack[:frameIdx]

			var err error
			if lambdaValue, ok := function.(*Lambda[syn.DeBruijn]); ok {
				currentTerm = lambdaValue.AST.Body
				currentEnv = extendEnvLocal(lambdaValue.Env, currentValue)
				currentValue = nil
				returning = false
			} else {
				currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
					function,
					currentValue,
				)
			}
			if err != nil {
				return nil, err
			}
			if currentTerm == nil {
				if !returning {
					return nil, &InternalError{
						Code:    ErrCodeInternalError,
						Message: "nil control term in DeBruijn evaluator",
					}
				}
				currentTerm = nilControlTermDeBruijn
			}
		case frameAwaitArgLambda:
			env := frame.env
			body := frame.term
			frameStack = frameStack[:frameIdx]

			currentTerm = body
			currentEnv = extendEnvLocal(env, currentValue)
			currentValue = nil
			returning = false
		case frameAwaitArgBuiltin:
			builtinValue := frame.builtin
			frameStack = frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				builtinValue,
				currentValue,
			)
			if err != nil {
				return nil, err
			}
			if currentTerm == nil {
				if !returning {
					return nil, &InternalError{
						Code:    ErrCodeInternalError,
						Message: "nil control term in DeBruijn evaluator",
					}
				}
				currentTerm = nilControlTermDeBruijn
			}
		case frameAwaitFunTerm:
			env := frame.env
			term := frame.term
			frameStack = frameStack[:frameIdx]

			if isImmediateTermDeBruijn(term) {
				argValue, err := computeKnownImmediateValueNoSlippageDeBruijn(m, env, term)
				if err != nil {
					return nil, err
				}
				if lambdaValue, ok := currentValue.(*Lambda[syn.DeBruijn]); ok {
					currentTerm = lambdaValue.AST.Body
					currentEnv = extendEnvLocal(lambdaValue.Env, argValue)
					currentValue = nil
					returning = false
				} else {
					currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
						currentValue,
						argValue,
					)
				}
				if err != nil {
					return nil, err
				}
				if currentTerm == nil {
					if !returning {
						return nil, &InternalError{
							Code:    ErrCodeInternalError,
							Message: "nil control term in DeBruijn evaluator",
						}
					}
					currentTerm = nilControlTermDeBruijn
				}
				continue
			}

			frameStack, frameStackUsed = pushAwaitArgFrameDeBruijn(
				frameStack,
				frameStackUsed,
				currentValue,
			)
			currentEnv = env
			currentTerm = term
			returning = false
		case frameAwaitFunValue:
			arg := frame.value
			frameStack = frameStack[:frameIdx]

			var err error
			if lambdaValue, ok := currentValue.(*Lambda[syn.DeBruijn]); ok {
				currentTerm = lambdaValue.AST.Body
				currentEnv = extendEnvLocal(lambdaValue.Env, arg)
				currentValue = nil
				returning = false
			} else {
				currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
					currentValue,
					arg,
				)
			}
			if err != nil {
				return nil, err
			}
			if currentTerm == nil {
				if !returning {
					return nil, &InternalError{
						Code:    ErrCodeInternalError,
						Message: "nil control term in DeBruijn evaluator",
					}
				}
				currentTerm = nilControlTermDeBruijn
			}
		case frameForce:
			frameStack = frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.forceEvaluateStack(
				currentValue,
			)
			if err != nil {
				return nil, err
			}
			if currentTerm == nil {
				if !returning {
					return nil, &InternalError{
						Code:    ErrCodeInternalError,
						Message: "nil control term in DeBruijn evaluator",
					}
				}
				currentTerm = nilControlTermDeBruijn
			}
		case frameConstr:
			frame.resolvedFields = append(frame.resolvedFields, currentValue)
			if len(frame.fields) == 0 {
				resolvedFields := frame.resolvedFields
				tag := frame.tag
				frameStack = frameStack[:frameIdx]

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
			frameStack = frameStack[:frameIdx]

			var err error
			syncFrameStack()
			currentTerm, currentEnv, currentValue, returning, err = m.caseEvaluateStack(
				env,
				branches,
				currentValue,
			)
			frameStack = m.frameStack
			frameStackUsed = m.frameStackUsed
			if err != nil {
				return nil, err
			}
			if currentTerm == nil {
				if !returning {
					return nil, &InternalError{
						Code:    ErrCodeInternalError,
						Message: "nil control term in DeBruijn evaluator",
					}
				}
				currentTerm = nilControlTermDeBruijn
			}
		default:
			return nil, &InternalError{
				Code:    ErrCodeInternalError,
				Message: "unknown stack frame in DeBruijn evaluator",
			}
		}
	}
}
