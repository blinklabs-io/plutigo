// Copyright 2026 Blink Labs Software

package cek

import (
	"unsafe"

	"github.com/blinklabs-io/plutigo/syn"
)

// This loop mirrors the checked variant so Machine.Run does not carry
// cancellation state through every CEK step.
func runStackNoSlippageDeBruijnUnchecked(
	m *Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
) (syn.Term[syn.DeBruijn], error) {
	var currentEnv *Env[syn.DeBruijn]
	currentTerm := term
	var currentValue Value[syn.DeBruijn]
	returning := false

	for {
		if !returning {
			if currentTerm == nil {
				return nil, internalError("DeBruijn stack machine current term is nil")
			}
			termIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&currentTerm))
			switch termIface.tab {
			case varTermTabDeBruijn:
				t := (*syn.Var[syn.DeBruijn])(termIface.data)
				if !m.spendStepNoSlippage(ExVar) {
					return nil, m.budgetErrorForStep(ExVar)
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
				if !m.spendStepNoSlippage(ExDelay) {
					return nil, m.budgetErrorForStep(ExDelay)
				}

				currentValue = m.allocDelay(t, currentEnv)
				returning = true
			case lambdaTermTabDeBruijn:
				t := (*syn.Lambda[syn.DeBruijn])(termIface.data)
				if !m.spendStepNoSlippage(ExLambda) {
					return nil, m.budgetErrorForStep(ExLambda)
				}

				currentValue = m.allocLambda(t, currentEnv)
				returning = true
			case applyTermTabDeBruijn:
				t := (*syn.Apply[syn.DeBruijn])(termIface.data)
				if !m.spendStepNoSlippage(ExApply) {
					return nil, m.budgetErrorForStep(ExApply)
				}

				funcIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&t.Function))
				if funcIface.tab == lambdaTermTabDeBruijn {
					lambda := (*syn.Lambda[syn.DeBruijn])(funcIface.data)
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
				if !m.spendStepNoSlippage(ExConstant) {
					return nil, m.budgetErrorForStep(ExConstant)
				}

				currentValue = machineConstantValue(m, t.Con)
				returning = true
			case forceTermTabDeBruijn:
				t := (*syn.Force[syn.DeBruijn])(termIface.data)
				if !m.spendStepNoSlippage(ExForce) {
					return nil, m.budgetErrorForStep(ExForce)
				}

				termIface := (*termInterfaceDeBruijn)(unsafe.Pointer(&t.Term))
				if termIface.tab == builtinTermTabDeBruijn {
					builtinTerm := (*syn.Builtin)(termIface.data)
					if !m.spendStepNoSlippage(ExBuiltin) {
						return nil, m.budgetErrorForStep(ExBuiltin)
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
			case errorTermTab:
				return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
			case builtinTermTabDeBruijn:
				t := (*syn.Builtin)(termIface.data)
				if !m.spendStepNoSlippage(ExBuiltin) {
					return nil, m.budgetErrorForStep(ExBuiltin)
				}

				currentValue = m.builtinNoArgValues[t.DefaultFunction][0]
				returning = true
			case constrTermTabDeBruijn:
				t := (*syn.Constr[syn.DeBruijn])(termIface.data)
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
			case caseTermTabDeBruijn:
				t := (*syn.Case[syn.DeBruijn])(termIface.data)
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

		if currentValue == nil {
			return nil, internalError("DeBruijn stack machine current value is nil")
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
