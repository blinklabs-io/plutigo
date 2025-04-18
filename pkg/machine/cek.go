package machine

import (
	"errors"

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
			state, err = m.returnCompute()
		case Done:
			return v.term, nil
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
	case syn.Var[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, err
		}

		value, exists := env.lookup(uint(t.Name.LookupIndex()))

		if !exists {
			return nil, errors.New("open term evaluated")
		}

		state = Return{Ctx: context, Value: *value}
	case syn.Delay[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}

		value := Delay{
			Body: t.Term,
			Env:  env,
		}

		state = Return{Ctx: context, Value: value}
	case syn.Lambda[syn.Eval]:
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}

		value := Lambda{
			ParameterName: t.ParameterName,
			Body:          t.Body,
			Env:           env,
		}

		state = Return{Ctx: context, Value: value}
	case syn.Apply[syn.Eval]:
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
	case syn.Constant:
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, err
		}

		state = Return{
			Ctx: context,
			Value: Constant{
				Constant: t.Con,
			},
		}

	case syn.Force[syn.Eval]:
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

	case syn.Error:
		return nil, errors.New("Eval Failure")

	case syn.Builtin:
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, err
		}

		state = Return{
			Ctx: context,
			Value: Builtin{
				Func: t.DefaultFunction,
				Args: []Value{},
				// placeholder
				Forces: 0,
			},
		}
	case syn.Constr[syn.Eval]:
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
	case syn.Case[syn.Eval]:
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
	}

	return state, nil
}

func (m *Machine) returnCompute() (MachineState, error) {
	return nil, nil
}

func (m *Machine) forceEvaluate() {}

func (m *Machine) applyEvaluate() {}

func (m *Machine) evalBuiltinApp() {}

func (m *Machine) lookupVar() {}

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
	for i := range m.unbudgetedSteps {
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
