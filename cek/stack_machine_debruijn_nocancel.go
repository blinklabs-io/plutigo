// Copyright 2026 Blink Labs Software

package cek

import (
	"unsafe"

	"github.com/blinklabs-io/plutigo/syn"
)

// This loop mirrors the checked variant so Machine.Run does not carry
// cancellation state through every CEK step.
func runStackDeBruijnUnchecked(
	m *Machine[syn.DeBruijn],
	term syn.Term[syn.DeBruijn],
	steps *stepCounter,
) (syn.Term[syn.DeBruijn], error) {
	var currentEnv *Env[syn.DeBruijn]
	currentTerm := term
	var currentValue Value[syn.DeBruijn]
	var returning bool

evalTerm:
	{
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
			goto evalValue
		case delayTermTabDeBruijn:
			t := (*syn.Delay[syn.DeBruijn])(termIface.data)
			if !steps.count(ExDelay) {
				if err := steps.spend(m); err != nil {
					return nil, err
				}
			}

			currentValue = m.allocDelay(t, currentEnv)
			goto evalValue
		case lambdaTermTabDeBruijn:
			t := (*syn.Lambda[syn.DeBruijn])(termIface.data)
			if !steps.count(ExLambda) {
				if err := steps.spend(m); err != nil {
					return nil, err
				}
			}

			currentValue = m.allocLambda(t, currentEnv)
			goto evalValue
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
					goto evalTerm
				}
				frame := m.pushFrameSlot()
				frame.kind = frameAwaitArgLambda
				frame.env = currentEnv
				frame.term = lambda.Body
				currentTerm = t.Argument
				goto evalTerm
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
						goto evalTerm
					}
					currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
						funValue,
						argValue,
					)
					if err != nil {
						return nil, err
					}
					if returning {
						goto evalValue
					}
					goto evalTerm
				}
				m.pushAwaitArgFrame(funValue)
				currentTerm = t.Argument
				goto evalTerm
			}

			frame := m.pushFrameSlot()
			frame.kind = frameAwaitFunTerm
			frame.env = currentEnv
			frame.term = t.Argument
			currentTerm = t.Function
			goto evalTerm
		case constantTermTab:
			t := (*syn.Constant)(termIface.data)
			if !steps.count(ExConstant) {
				if err := steps.spend(m); err != nil {
					return nil, err
				}
			}

			currentValue = machineConstantValue(m, t.Con)
			goto evalValue
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
				if returning {
					goto evalValue
				}
				goto evalTerm
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
				if returning {
					goto evalValue
				}
				goto evalTerm
			}

			frame := m.pushFrameSlot()
			frame.kind = frameForce
			currentTerm = t.Term
			goto evalTerm
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
			goto evalValue
		case constrTermTabDeBruijn:
			t := (*syn.Constr[syn.DeBruijn])(termIface.data)
			if !steps.count(ExConstr) {
				if err := steps.spend(m); err != nil {
					return nil, err
				}
			}

			if len(t.Fields) == 0 {
				currentValue = m.allocConstr(t.Tag, nil)
				goto evalValue
			}

			frame := m.pushFrameSlot()
			frame.kind = frameConstr
			frame.env = currentEnv
			frame.tag = t.Tag
			frame.fields = t.Fields[1:]
			frame.resolvedFields = m.allocValueElems(len(t.Fields))[:0]
			currentTerm = t.Fields[0]
			goto evalTerm
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
				if returning {
					goto evalValue
				}
				goto evalTerm
			}

			frame := m.pushFrameSlot()
			frame.kind = frameCases
			frame.env = currentEnv
			frame.branches = t.Branches
			currentTerm = t.Constr
			goto evalTerm
		default:
			return nil, &InternalError{
				Code:    ErrCodeInternalError,
				Message: "unknown term in DeBruijn evaluator",
			}
		}

	}

evalValue:
	{
		if currentValue == nil {
			return nil, internalError("DeBruijn stack machine current value is nil")
		}
		if len(m.frameStack) == 0 {
			if err := steps.spend(m); err != nil {
				return nil, err
			}
			return m.finishValue(currentValue)
		}
		frameIdx := len(m.frameStack) - 1
		frame := &m.frameStack[frameIdx]

		switch frame.kind {
		case frameAwaitArg:
			function := frame.value
			m.frameStack = m.frameStack[:frameIdx]

			if lambda, ok := function.(*Lambda[syn.DeBruijn]); ok {
				currentTerm = lambda.AST.Body
				currentEnv = m.extendEnv(lambda.Env, currentValue)
				goto evalTerm
			}

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				function,
				currentValue,
			)
			if err != nil {
				return nil, err
			}
			if returning {
				goto evalValue
			}
			goto evalTerm
		case frameAwaitArgLambda:
			env := frame.env
			body := frame.term
			m.frameStack = m.frameStack[:frameIdx]

			currentTerm = body
			currentEnv = m.extendEnv(env, currentValue)
			goto evalTerm
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
			if returning {
				goto evalValue
			}
			goto evalTerm
		case frameAwaitFunTerm:
			env := frame.env
			term := frame.term
			m.frameStack = m.frameStack[:frameIdx]

			if isImmediateTermDeBruijn(term) {
				argValue, err := computeKnownImmediateValueDeBruijn(m, steps, env, term)
				if err != nil {
					return nil, err
				}
				if lambda, ok := currentValue.(*Lambda[syn.DeBruijn]); ok {
					currentTerm = lambda.AST.Body
					currentEnv = m.extendEnv(lambda.Env, argValue)
					goto evalTerm
				}
				currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
					currentValue,
					argValue,
				)
				if err != nil {
					return nil, err
				}
				if returning {
					goto evalValue
				}
				goto evalTerm
			}

			m.pushAwaitArgFrame(currentValue)
			currentEnv = env
			currentTerm = term
			goto evalTerm
		case frameAwaitFunValue:
			arg := frame.value
			m.frameStack = m.frameStack[:frameIdx]

			if lambda, ok := currentValue.(*Lambda[syn.DeBruijn]); ok {
				currentTerm = lambda.AST.Body
				currentEnv = m.extendEnv(lambda.Env, arg)
				goto evalTerm
			}

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				currentValue,
				arg,
			)
			if err != nil {
				return nil, err
			}
			if returning {
				goto evalValue
			}
			goto evalTerm
		case frameForce:
			m.frameStack = m.frameStack[:frameIdx]

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.forceEvaluateStack(
				currentValue,
			)
			if err != nil {
				return nil, err
			}
			if returning {
				goto evalValue
			}
			goto evalTerm
		case frameConstr:
			frame.resolvedFields = append(frame.resolvedFields, currentValue)
			if len(frame.fields) == 0 {
				resolvedFields := frame.resolvedFields
				tag := frame.tag
				m.frameStack = m.frameStack[:frameIdx]

				currentValue = m.allocConstr(tag, resolvedFields)
				goto evalValue
			}

			nextField := frame.fields[0]
			frame.fields = frame.fields[1:]
			currentEnv = frame.env
			currentTerm = nextField
			goto evalTerm
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
			if returning {
				goto evalValue
			}
			goto evalTerm
		default:
			return nil, &InternalError{
				Code:    ErrCodeInternalError,
				Message: "unknown stack frame in DeBruijn evaluator",
			}
		}
	}
}
