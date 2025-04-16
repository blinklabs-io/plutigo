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

func (m *Machine) Run(term *syn.Term[syn.NamedDeBruijn]) (syn.Term[syn.NamedDeBruijn], error) {
	startupBudget := m.costs.machineCosts.startup
	if err := m.spendBudget(startupBudget); err != nil {
		return nil, err
	}

	var state MachineState = Compute{ctx: NoFrame{}, env: make([]Value, 0), term: *term}
	var err error

	for {
		switch v := state.(type) {
		case Compute:
			state, err = m.compute(v.ctx, v.env, v.term)
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
	term syn.Term[syn.NamedDeBruijn],
) (MachineState, error) {
	var state MachineState

	switch t := term.(type) {
	case syn.Var[syn.NamedDeBruijn]:
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, err
		}

		value, exists := env.lookup(uint(t.Name.Index))

		if !exists {
			return nil, errors.New("open term evaluated")
		}

		state = Return{ctx: context, value: value}
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
