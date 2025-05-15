package cek

import (
	"errors"
	"fmt"
	"math"

	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type Machine[T syn.Eval] struct {
	costs    CostModel
	slippage uint32
	ExBudget ExBudget
	Logs     []string

	unbudgetedSteps [10]uint32
}

func NewMachine[T syn.Eval](slippage uint32) *Machine[T] {
	return &Machine[T]{
		costs:    DefaultCostModel,
		slippage: slippage,
		ExBudget: DefaultExBudget,
		Logs:     make([]string, 0),

		unbudgetedSteps: [10]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
}

func (m *Machine[T]) Run(term syn.Term[T]) (syn.Term[T], error) {
	startupBudget := m.costs.machineCosts.startup
	if err := m.spendBudget(startupBudget); err != nil {
		return nil, err
	}

	initialEnv := Env[T]([]Value[T]{})

	var err error
	var state MachineState[T] = &Compute[T]{
		Ctx:  &NoFrame{},
		Env:  initialEnv,
		Term: term,
	}

	for {
		switch v := state.(type) {
		case *Compute[T]:
			state, err = m.compute(v.Ctx, v.Env, v.Term)
		case *Return[T]:
			state, err = m.returnCompute(v.Ctx, v.Value)
		case *Done[T]:
			return v.term, nil
		default:
			panic("unknown machine state")
		}
		if err != nil {
			return nil, err
		}
	}
}

func (m *Machine[T]) compute(
	context MachineContext[T],
	env Env[T],
	term syn.Term[T],
) (MachineState[T], error) {
	var state MachineState[T]

	switch t := term.(type) {
	case *syn.Var[T]:
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, err
		}

		value, exists := env.lookup(t.Name.LookupIndex())

		if !exists {
			return nil, errors.New("open term evaluated")
		}

		state = &Return[T]{Ctx: context, Value: value}
	case *syn.Delay[T]:
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}

		value := &Delay[T]{
			Body: t.Term,
			Env:  env,
		}

		state = &Return[T]{Ctx: context, Value: value}
	case *syn.Lambda[T]:
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}

		value := &Lambda[T]{
			ParameterName: t.ParameterName,
			Body:          t.Body,
			Env:           env,
		}

		state = &Return[T]{Ctx: context, Value: value}
	case *syn.Apply[T]:
		if err := m.stepAndMaybeSpend(ExApply); err != nil {
			return nil, err
		}

		frame := &FrameAwaitFunTerm[T]{
			Env:  env,
			Term: t.Argument,
			Ctx:  context,
		}

		state = &Compute[T]{
			Ctx:  frame,
			Env:  env,
			Term: t.Function,
		}
	case *syn.Constant:
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, err
		}

		state = &Return[T]{
			Ctx: context,
			Value: &Constant{
				Constant: t.Con,
			},
		}
	case *syn.Force[T]:
		if err := m.stepAndMaybeSpend(ExForce); err != nil {
			return nil, err
		}

		frame := &FrameForce[T]{
			Ctx: context,
		}

		state = &Compute[T]{
			Ctx:  frame,
			Env:  env,
			Term: t.Term,
		}
	case *syn.Error:
		return nil, errors.New("error explicitly called")

	case *syn.Builtin:
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, err
		}

		state = &Return[T]{
			Ctx: context,
			Value: &Builtin[T]{
				Func:   t.DefaultFunction,
				Args:   []Value[T]{},
				Forces: 0,
			},
		}
	case *syn.Constr[T]:
		if err := m.stepAndMaybeSpend(ExConstr); err != nil {
			return nil, err
		}

		fields := t.Fields

		if len(fields) == 0 {
			state = &Return[T]{
				Ctx: context,
				Value: &Constr[T]{
					Tag:    t.Tag,
					Fields: []Value[T]{},
				},
			}
		} else {
			first_field := fields[0]

			rest := fields[1:]

			frame := &FrameConstr[T]{
				Ctx:            context,
				Tag:            t.Tag,
				Fields:         rest,
				ResolvedFields: []Value[T]{},
				Env:            env,
			}

			state = &Compute[T]{
				Ctx:  frame,
				Env:  env,
				Term: first_field,
			}
		}
	case *syn.Case[T]:
		if err := m.stepAndMaybeSpend(ExCase); err != nil {
			return nil, err
		}

		frame := &FrameCases[T]{
			Env:      env,
			Ctx:      context,
			Branches: t.Branches,
		}

		state = &Compute[T]{
			Ctx:  frame,
			Env:  env,
			Term: t.Constr,
		}
	default:
		panic(fmt.Sprintf("unknown term: %v", term))
	}

	return state, nil
}

func (m *Machine[T]) returnCompute(
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	var state MachineState[T]
	var err error

	switch c := context.(type) {
	case *FrameAwaitArg[T]:
		state, err = m.applyEvaluate(c.Ctx, c.Value, value)

		if err != nil {
			return nil, err
		}
	case *FrameAwaitFunTerm[T]:
		state = &Compute[T]{
			Ctx: &FrameAwaitArg[T]{
				Ctx:   c.Ctx,
				Value: value,
			},
			Env:  c.Env,
			Term: c.Term,
		}
	case *FrameAwaitFunValue[T]:
		state, err = m.applyEvaluate(c.Ctx, value, c.Value)

		if err != nil {
			return nil, err
		}
	case *FrameForce[T]:
		state, err = m.forceEvaluate(c.Ctx, value)

		if err != nil {
			return nil, err
		}
	case *FrameConstr[T]:
		resolvedFields := append(c.ResolvedFields, value)

		fields := c.Fields

		if len(fields) == 0 {
			state = &Return[T]{
				Ctx: c.Ctx,
				Value: &Constr[T]{
					Tag:    c.Tag,
					Fields: resolvedFields,
				},
			}
		} else {
			first_field := fields[0]
			rest := fields[1:]

			frame := &FrameConstr[T]{
				Ctx:            context,
				Tag:            c.Tag,
				Fields:         rest,
				ResolvedFields: resolvedFields,
				Env:            c.Env,
			}

			state = &Compute[T]{
				Ctx:  frame,
				Env:  c.Env,
				Term: first_field,
			}
		}
	case *FrameCases[T]:
		switch v := value.(type) {
		case *Constr[T]:
			if v.Tag > math.MaxInt {
				return nil, errors.New("MaxIntExceeded")
			}
			if indexExists(c.Branches, int(v.Tag)) {
				state = &Compute[T]{
					Ctx:  transferArgStack(v.Fields, c.Ctx),
					Env:  c.Env,
					Term: c.Branches[v.Tag],
				}
			} else {
				return nil, errors.New("MissingCaseBranch")
			}
		default:
			return nil, errors.New("NonConstrScrutinized")
		}
	case *NoFrame:
		if m.unbudgetedSteps[9] > 0 {
			if err := m.spendUnbudgetedSteps(); err != nil {
				return nil, err
			}
		}

		state = &Done[T]{
			term: dischargeValue[T](value),
		}
	default:
		panic(fmt.Sprintf("unknown context %v", context))
	}
	return state, nil
}

func (m *Machine[T]) forceEvaluate(
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	var state MachineState[T]

	switch v := value.(type) {
	case *Delay[T]:
		state = &Compute[T]{
			Ctx:  context,
			Env:  v.Env,
			Term: v.Body,
		}
	case *Builtin[T]:
		if v.NeedsForce() {
			var resolved Value[T]

			b := v.ConsumeForce()

			if b.IsReady() {
				var err error

				resolved, err = m.evalBuiltinApp(b)

				if err != nil {
					return nil, err
				}
			} else {
				resolved = b
			}

			state = &Return[T]{
				Ctx:   context,
				Value: resolved,
			}
		} else {
			return nil, errors.New("BuiltinTermArgumentExpected")
		}
	default:
		return nil, errors.New("NonPolymorphicInstantiation")
	}

	return state, nil
}

func (m *Machine[T]) applyEvaluate(
	context MachineContext[T],
	function Value[T],
	arg Value[T],
) (MachineState[T], error) {
	var state MachineState[T]

	switch f := function.(type) {
	case *Lambda[T]:
		env := make(Env[T], len(f.Env))

		copy(env, f.Env)

		env = append(env, arg)

		state = &Compute[T]{
			Ctx:  context,
			Env:  env,
			Term: f.Body,
		}
	case *Builtin[T]:
		if !f.NeedsForce() && f.IsArrow() {
			var resolved Value[T]

			b := f.ApplyArg(arg)

			if b.IsReady() {
				var err error

				resolved, err = m.evalBuiltinApp(b)

				if err != nil {
					return nil, err
				}
			} else {
				resolved = b
			}

			state = &Return[T]{
				Ctx:   context,
				Value: resolved,
			}
		} else {
			return nil, errors.New("UnexpectedBuiltinTermArgument")
		}
	default:
		return nil, errors.New("NonFunctionalApplication")
	}

	return state, nil
}

func transferArgStack[T syn.Eval](
	fields []Value[T],
	ctx MachineContext[T],
) MachineContext[T] {
	c := ctx

	for arg := len(fields) - 1; arg >= 0; arg-- {
		c = &FrameAwaitFunValue[T]{
			Ctx:   c,
			Value: fields[arg],
		}
	}

	return c
}

func dischargeValue[T syn.Eval](value Value[T]) syn.Term[T] {
	var dischargedTerm syn.Term[T]

	switch v := value.(type) {
	case *Constant:
		dischargedTerm = &syn.Constant{
			Con: v.Constant,
		}
	case *Builtin[T]:
		var forcedTerm syn.Term[T]

		forcedTerm = &syn.Builtin{
			DefaultFunction: v.Func,
		}

		for range uint(v.Forces) {
			forcedTerm = &syn.Force[T]{
				Term: forcedTerm,
			}
		}

		for _, arg := range v.Args {
			forcedTerm = &syn.Apply[T]{
				Function: forcedTerm,
				Argument: dischargeValue[T](arg),
			}
		}

		dischargedTerm = forcedTerm
	case *Delay[T]:
		dischargedTerm = &syn.Delay[T]{
			Term: withEnv(0, v.Env, v.Body),
		}

	case *Lambda[T]:
		dischargedTerm = &syn.Lambda[T]{
			ParameterName: v.ParameterName,
			Body:          withEnv(1, v.Env, v.Body),
		}

	case *Constr[T]:
		fields := []syn.Term[T]{}

		for _, f := range v.Fields {
			fields = append(fields, dischargeValue[T](f))
		}

		dischargedTerm = &syn.Constr[T]{
			Tag:    v.Tag,
			Fields: fields,
		}
	}

	return dischargedTerm
}

func withEnv[T syn.Eval](lamCnt int, env Env[T], term syn.Term[T]) syn.Term[T] {
	var dischargedTerm syn.Term[T]

	switch t := term.(type) {
	case *syn.Var[T]:
		if lamCnt >= t.Name.LookupIndex() {
			dischargedTerm = t
		} else if val, exists := env.lookup(t.Name.LookupIndex() - lamCnt); exists {
			dischargedTerm = dischargeValue[T](val)
		} else {
			dischargedTerm = t
		}

	case *syn.Lambda[T]:
		dischargedTerm = &syn.Lambda[T]{
			ParameterName: t.ParameterName,
			Body:          withEnv(lamCnt+1, env, t.Body),
		}

	case *syn.Apply[T]:
		dischargedTerm = &syn.Apply[T]{
			Function: withEnv(lamCnt, env, t.Function),
			Argument: withEnv(lamCnt, env, t.Argument),
		}
	case *syn.Delay[T]:
		dischargedTerm = &syn.Delay[T]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case *syn.Force[T]:
		dischargedTerm = &syn.Force[T]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case *syn.Constr[T]:
		fields := []syn.Term[T]{}

		for _, f := range t.Fields {
			fields = append(fields, withEnv(lamCnt, env, f))
		}

		dischargedTerm = &syn.Constr[T]{
			Tag:    t.Tag,
			Fields: fields,
		}
	case *syn.Case[T]:
		branches := []syn.Term[T]{}

		for _, b := range t.Branches {
			branches = append(branches, withEnv(lamCnt, env, b))
		}

		dischargedTerm = &syn.Case[T]{
			Constr:   withEnv(lamCnt, env, t.Constr),
			Branches: branches,
		}
	default:
		dischargedTerm = t
	}

	return dischargedTerm
}

func (m *Machine[T]) stepAndMaybeSpend(step StepKind) error {
	m.unbudgetedSteps[step] += 1
	m.unbudgetedSteps[9] += 1

	if m.unbudgetedSteps[9] >= m.slippage {
		if err := m.spendUnbudgetedSteps(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Machine[T]) spendUnbudgetedSteps() error {
	for i := range len(m.unbudgetedSteps) - 1 {
		unspent_step_budget :=
			m.costs.machineCosts.get(StepKind(i))

		unspent_step_budget.occurrences(m.unbudgetedSteps[i])

		if err := m.spendBudget(unspent_step_budget); err != nil {
			return err
		}

		m.unbudgetedSteps[i] = 0
	}

	m.unbudgetedSteps[9] = 0

	return nil
}

func (m *Machine[T]) spendBudget(exBudget ExBudget) error {
	m.ExBudget.Mem -= exBudget.Mem
	m.ExBudget.Cpu -= exBudget.Cpu

	if m.ExBudget.Mem < 0 || m.ExBudget.Cpu < 0 {
		return errors.New("out of budget")
	}

	return nil
}
