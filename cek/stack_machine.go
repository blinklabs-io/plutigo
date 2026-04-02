package cek

import (
	"math"

	"github.com/blinklabs-io/plutigo/syn"
)

type stackFrameKind uint8

const (
	frameAwaitArg stackFrameKind = iota
	frameAwaitFunTerm
	frameAwaitFunValue
	frameForce
	frameConstr
	frameCases
)

type stackFrame[T syn.Eval] struct {
	kind stackFrameKind

	value Value[T]
	env   *Env[T]
	term  syn.Term[T]

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

func (m *Machine[T]) peekFrame() (*stackFrame[T], bool) {
	if len(m.frameStack) == 0 {
		return nil, false
	}
	return &m.frameStack[len(m.frameStack)-1], true
}

func (m *Machine[T]) dropFrame() {
	m.frameStack = m.frameStack[:len(m.frameStack)-1]
}

func (m *Machine[T]) pushApplyFrames(args []Value[T]) {
	for i := len(args) - 1; i >= 0; i-- {
		m.pushFrame(stackFrame[T]{
			kind:  frameAwaitFunValue,
			value: args[i],
		})
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

				currentValue = m.allocDelay(t.Term, currentEnv)
				returning = true
			case *syn.Lambda[T]:
				if err := m.stepAndMaybeSpend(ExLambda); err != nil {
					return nil, err
				}

				currentValue = m.allocLambda(t.ParameterName, t.Body, currentEnv)
				returning = true
			case *syn.Apply[T]:
				if err := m.stepAndMaybeSpend(ExApply); err != nil {
					return nil, err
				}

				funValue, ok, err := m.computeImmediateValue(currentEnv, t.Function)
				if err != nil {
					return nil, err
				}
				if ok {
					argValue, argImmediate, err := m.computeImmediateValue(currentEnv, t.Argument)
					if err != nil {
						return nil, err
					}
					if argImmediate {
						currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
							funValue,
							argValue,
						)
						if err != nil {
							return nil, err
						}
						continue
					}

					m.pushFrame(stackFrame[T]{
						kind:  frameAwaitArg,
						value: funValue,
					})
					currentTerm = t.Argument
					continue
				}

				m.pushFrame(stackFrame[T]{
					kind: frameAwaitFunTerm,
					env:  currentEnv,
					term: t.Argument,
				})
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

				forcedValue, ok, err := m.computeImmediateValue(currentEnv, t.Term)
				if err != nil {
					return nil, err
				}
				if ok {
					currentTerm, currentEnv, currentValue, returning, err = m.forceEvaluateStack(
						forcedValue,
					)
					if err != nil {
						return nil, err
					}
					continue
				}

				m.pushFrame(stackFrame[T]{kind: frameForce})
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

				m.pushFrame(stackFrame[T]{
					kind:           frameConstr,
					env:            currentEnv,
					tag:            t.Tag,
					fields:         t.Fields[1:],
					resolvedFields: m.allocValueElems(len(t.Fields))[:0],
				})
				currentTerm = t.Fields[0]
			case *syn.Case[T]:
				if err := m.stepAndMaybeSpend(ExCase); err != nil {
					return nil, err
				}

				scrutinee, ok, err := m.computeImmediateValue(currentEnv, t.Constr)
				if err != nil {
					return nil, err
				}
				if ok {
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

				m.pushFrame(stackFrame[T]{
					kind:     frameCases,
					env:      currentEnv,
					branches: t.Branches,
				})
				currentTerm = t.Constr
			default:
				panic("unknown term")
			}

			continue
		}

		frame, ok := m.peekFrame()
		if !ok {
			return m.finishValue(currentValue)
		}

		switch frame.kind {
		case frameAwaitArg:
			function := frame.value
			m.dropFrame()

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				function,
				currentValue,
			)
			if err != nil {
				return nil, err
			}
		case frameAwaitFunTerm:
			env := frame.env
			term := frame.term
			m.dropFrame()

			argValue, ok, err := m.computeImmediateValue(env, term)
			if err != nil {
				return nil, err
			}
			if ok {
				currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
					currentValue,
					argValue,
				)
				if err != nil {
					return nil, err
				}
				continue
			}

			m.pushFrame(stackFrame[T]{
				kind:  frameAwaitArg,
				value: currentValue,
			})
			currentEnv = env
			currentTerm = term
			returning = false
		case frameAwaitFunValue:
			arg := frame.value
			m.dropFrame()

			var err error
			currentTerm, currentEnv, currentValue, returning, err = m.applyEvaluateStack(
				currentValue,
				arg,
			)
			if err != nil {
				return nil, err
			}
		case frameForce:
			m.dropFrame()

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
				m.dropFrame()

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
			m.dropFrame()

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
		return f.Body, m.extendEnv(f.Env, arg), nil, false, nil
	case *Builtin[T]:
		if !f.NeedsForce() && f.IsArrow() {
			nextArgCount := f.ArgCount + 1
			nextArgs := m.extendBuiltinArgs(f.Args, arg)
			if f.Func.Arity() == nextArgCount && f.Func.ForceCount() == f.Forces {
				resolved, err := m.evalBuiltinApp(
					m.allocBuiltin(f.Func, f.Forces, nextArgCount, nextArgs),
				)
				if err != nil {
					return nil, nil, nil, false, err
				}
				return nil, nil, resolved, true, nil
			}
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
		return v.Body, v.Env, nil, false, nil
	case *Builtin[T]:
		if v.NeedsForce() {
			nextForces := v.Forces + 1
			if v.Func.ForceCount() == nextForces && v.Func.Arity() == v.ArgCount {
				resolved, err := m.evalBuiltinApp(
					m.allocBuiltin(v.Func, nextForces, v.ArgCount, v.Args),
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
		var args []Value[T]
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
				args = m.allocValueElems(2)
				args[0] = m.allocConstant(cval.List[0])
				tail := m.allocProtoListConstant(cval.LTyp, cval.List[1:])
				args[1] = m.allocConstant(tail)
			}
		case *syn.ProtoPair:
			branchRule = 1
			tag = 0
			args = m.allocValueElems(2)
			args[0] = m.allocConstant(cval.First)
			args[1] = m.allocConstant(cval.Second)
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

		m.pushApplyFrames(args)
		return branches[tag], env, nil, false, nil
	default:
		return nil, nil, nil, false, &TypeError{
			Code:    ErrCodeNonConstrScrutinized,
			Message: "NonConstrScrutinized",
		}
	}
}
