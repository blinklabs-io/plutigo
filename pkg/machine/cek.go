package machine

import (
	"errors"
	"fmt"

	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type Machine struct {
	costs           CostModel
	slippage        uint32
	exBudget        ExBudget
	unbudgetedSteps [10]uint32
	Logs            []string
}

func NewMachine(slippage uint32) Machine {
	return Machine{
		costs:    DefaultCostModel,
		slippage: slippage,
		exBudget: DefaultExBudget,
		Logs:     make([]string, 0),

		unbudgetedSteps: [10]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
}

func (m *Machine) Run(term syn.Term[syn.Eval]) (syn.Term[syn.Eval], error) {
	startupBudget := m.costs.machineCosts.startup
	if err := m.spendBudget(startupBudget); err != nil {
		return nil, err
	}

	var state MachineState = Compute{Ctx: NoFrame{}, Env: Env([]Value{}), Term: term}
	var err error

	for {
		switch v := state.(type) {
		case Compute:
			state, err = m.compute(v.Ctx, v.Env, v.Term)
		case Return:
			state, err = m.returnCompute(v.Ctx, v.Value)
		case Done:
			return v.term, nil
		default:
			panic("THIS SHOULD NOT HAPPEN")
		}
		if err != nil {
			return nil, err
		}
	}
}

func (m *Machine) compute(
	context MachineContext,
	env Env,
	term syn.Term[syn.Eval],
) (MachineState, error) {
	var state MachineState

	switch t := term.(type) {
	case *syn.Var[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, err
		}

		value, exists := env.lookup(uint(t.Name.LookupIndex()))

		if !exists {
			return nil, errors.New("open term evaluated")
		}

		state = Return{Ctx: context, Value: *value}
	case *syn.Delay[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}

		value := Delay{
			Body: t.Term,
			Env:  env,
		}

		state = Return{Ctx: context, Value: value}
	case *syn.Lambda[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}

		value := Lambda{
			ParameterName: t.ParameterName,
			Body:          t.Body,
			Env:           env,
		}

		state = Return{Ctx: context, Value: value}
	case *syn.Apply[syn.Eval]:

		if err := m.stepAndMaybeSpend(ExApply); err != nil {
			return nil, err
		}

		frame := FrameAwaitFunTerm{
			Env:  env,
			Term: t.Argument,
			Ctx:  context,
		}

		state = Compute{
			Ctx:  frame,
			Env:  env,
			Term: t.Function,
		}
	case *syn.Constant:
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, err
		}

		state = Return{
			Ctx: context,
			Value: Constant{
				Constant: t.Con,
			},
		}

	case *syn.Force[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExForce); err != nil {
			return nil, err
		}

		frame := FrameForce{
			Ctx: context,
		}

		state = Compute{
			Ctx:  frame,
			Env:  env,
			Term: t.Term,
		}

	case *syn.Error:
		return nil, errors.New("Eval Failure")

	case *syn.Builtin:
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, err
		}

		state = Return{
			Ctx: context,
			Value: Builtin{
				Func:   t.DefaultFunction,
				Args:   []Value{},
				Forces: 0,
			},
		}
	case *syn.Constr[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExConstr); err != nil {
			return nil, err
		}

		fields := *t.Fields

		if len(fields) == 0 {
			state = Return{
				Ctx: context,
				Value: Constr{
					Tag:    t.Tag,
					Fields: []Value{},
				},
			}
		} else {
			first_field := fields[0]

			rest := fields[1:]

			frame := FrameConstr{
				Ctx:            context,
				Tag:            t.Tag,
				Fields:         rest,
				ResolvedFields: []Value{},
				Env:            env,
			}

			state = Compute{
				Ctx:  frame,
				Env:  env,
				Term: first_field,
			}
		}
	case *syn.Case[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExCase); err != nil {
			return nil, err
		}

		frame := FrameCases{
			Env:      env,
			Ctx:      context,
			Branches: *t.Branches,
		}

		state = Compute{
			Ctx:  frame,
			Env:  env,
			Term: t.Constr,
		}
	default:
		fmt.Printf("THING %v\n", state)
		panic("WHAT HAPPENED HERE???")
	}

	return state, nil
}

func (m *Machine) returnCompute(context MachineContext, value Value) (MachineState, error) {
	var state MachineState
	var err error

	switch c := context.(type) {
	case FrameAwaitArg:
		state, err = m.applyEvaluate(c.Ctx, c.Value, value)

		if err != nil {
			return nil, err
		}
	case FrameAwaitFunTerm:
		state = Compute{
			Ctx: FrameAwaitArg{
				Ctx:   c.Ctx,
				Value: value,
			},
			Env:  c.Env,
			Term: c.Term,
		}
	case FrameAwaitFunValue:
		state, err = m.applyEvaluate(c.Ctx, value, c.Value)

		if err != nil {
			return nil, err
		}
	case FrameForce:
		state, err = m.forceEvaluate(c.Ctx, value)

		if err != nil {
			return nil, err
		}
	case FrameConstr:
		resolvedFields := append(c.ResolvedFields, value)

		fields := c.Fields

		if len(fields) == 0 {
			state = Return{
				Ctx: c.Ctx,
				Value: Constr{
					Tag:    c.Tag,
					Fields: resolvedFields,
				},
			}
		} else {
			first_field := fields[0]
			rest := fields[1:]

			frame := FrameConstr{
				Ctx:            context,
				Tag:            c.Tag,
				Fields:         rest,
				ResolvedFields: resolvedFields,
				Env:            c.Env,
			}

			state = Compute{
				Ctx:  frame,
				Env:  c.Env,
				Term: first_field,
			}
		}
	case FrameCases:
		switch v := value.(type) {
		case Constr:
			if indexExists(c.Branches, int(v.Tag)) {
				state = Compute{
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
	case NoFrame:
		if m.unbudgetedSteps[9] > 0 {
			if err := m.spendUnbudgetedSteps(); err != nil {
				return nil, err
			}
		}

		state = Done{
			term: dischargeValue(value),
		}
	}
	return state, nil
}

func (m *Machine) forceEvaluate(context MachineContext, value Value) (MachineState, error) {

	var state MachineState

	switch v := value.(type) {
	case Delay:
		state = Compute{
			Ctx:  context,
			Env:  v.Env,
			Term: v.Body,
		}
	case Builtin:
		if v.NeedsForce() {
			var resolved Value

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

			state = Return{
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

func (m *Machine) applyEvaluate(context MachineContext, function Value, arg Value) (MachineState, error) {
	var state MachineState

	switch f := function.(type) {
	case Lambda:
		env := make(Env, len(f.Env))
		copy(env, f.Env)

		env = append(env, arg)

		state = Compute{
			Ctx:  context,
			Env:  env,
			Term: f.Body,
		}
	case Builtin:
		if !f.NeedsForce() && f.IsArrow() {
			var resolved Value

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

			state = Return{
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

func transferArgStack(fields []Value, ctx MachineContext) MachineContext {
	c := ctx
	for arg := len(fields) - 1; arg >= 0; arg-- {
		c = FrameAwaitFunValue{
			Ctx:   c,
			Value: fields[arg],
		}
	}
	return c
}

func dischargeValue(value Value) syn.Term[syn.Eval] {
	var dischargedTerm syn.Term[syn.Eval]

	switch v := value.(type) {
	case Constant:
		dischargedTerm = syn.Constant{
			Con: v.Constant,
		}
	case Builtin:
		var forcedTerm syn.Term[syn.Eval]
		forcedTerm = syn.Builtin{
			DefaultFunction: v.Func,
		}

		for i := uint(0); i < v.Forces; i++ {
			forcedTerm = syn.Force[syn.Eval]{
				Term: forcedTerm,
			}
		}

		for _, arg := range v.Args {
			forcedTerm = syn.Apply[syn.Eval]{
				Function: forcedTerm,
				Argument: dischargeValue(arg),
			}
		}

		dischargedTerm = forcedTerm

	case Delay:
		dischargedTerm = syn.Delay[syn.Eval]{
			Term: withEnv(0, v.Env, v.Body),
		}

	case Lambda:
		dischargedTerm = syn.Lambda[syn.Eval]{
			ParameterName: v.ParameterName,
			Body:          withEnv(1, v.Env, v.Body),
		}

	case Constr:
		fields := []syn.Term[syn.Eval]{}

		for _, f := range v.Fields {
			fields = append(fields, dischargeValue(f))
		}

		dischargedTerm = syn.Constr[syn.Eval]{
			Tag:    v.Tag,
			Fields: &fields,
		}
	}

	return dischargedTerm
}

func withEnv(lamCnt uint, env Env, term syn.Term[syn.Eval]) syn.Term[syn.Eval] {
	var dischargedTerm syn.Term[syn.Eval]

	switch t := term.(type) {
	case syn.Var[syn.Eval]:
		if lamCnt >= uint(t.Name.LookupIndex()) {
			dischargedTerm = t
		} else if val, exists := env.lookup(uint(t.Name.LookupIndex()) - lamCnt); exists {
			dischargedTerm = dischargeValue(*val)
		} else {
			dischargedTerm = t
		}

	case syn.Lambda[syn.Eval]:
		dischargedTerm = syn.Lambda[syn.Eval]{
			ParameterName: t.ParameterName,
			Body:          withEnv(lamCnt+1, env, t.Body),
		}

	case syn.Apply[syn.Eval]:
		dischargedTerm = syn.Apply[syn.Eval]{
			Function: withEnv(lamCnt, env, t.Function),
			Argument: withEnv(lamCnt, env, t.Argument),
		}
	case syn.Delay[syn.Eval]:
		dischargedTerm = syn.Delay[syn.Eval]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case syn.Force[syn.Eval]:
		dischargedTerm = syn.Force[syn.Eval]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case syn.Constr[syn.Eval]:
		fields := []syn.Term[syn.Eval]{}

		for _, f := range *t.Fields {
			fields = append(fields, withEnv(lamCnt, env, f))
		}

		dischargedTerm = syn.Constr[syn.Eval]{
			Tag:    t.Tag,
			Fields: &fields,
		}
	case syn.Case[syn.Eval]:
		branches := []syn.Term[syn.Eval]{}

		for _, b := range *t.Branches {
			branches = append(branches, withEnv(lamCnt, env, b))
		}

		dischargedTerm = syn.Case[syn.Eval]{
			Constr:   withEnv(lamCnt, env, t.Constr),
			Branches: &branches,
		}
	default:
		dischargedTerm = t

	}

	return dischargedTerm
}

func (m *Machine) stepAndMaybeSpend(step StepKind) error {
	m.unbudgetedSteps[step] += 1
	m.unbudgetedSteps[9] += 1

	if m.unbudgetedSteps[9] >= m.slippage {
		if err := m.spendUnbudgetedSteps(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Machine) spendUnbudgetedSteps() error {
	for i := 0; i < len(m.unbudgetedSteps)-1; i++ {
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

func (m *Machine) spendBudget(exBudget ExBudget) error {
	m.exBudget.mem -= exBudget.mem
	m.exBudget.cpu -= exBudget.cpu

	if m.exBudget.mem < 0 || m.exBudget.cpu < 0 {
		return errors.New("out of budget")
	}

	return nil
}
