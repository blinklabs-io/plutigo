package plutus

import (
	"errors"
)

type Machine struct {
	slippage         uint32
	unbudgeted_steps [8]uint32
	Logs             []string
}

func CreateMachine(slippage uint32) Machine {
	return Machine{
		slippage,
		[8]uint32{0, 0, 0, 0, 0, 0, 0, 0},
		make([]string, 0),
	}
}

func (m *Machine) Run(term *Term[NamedDeBruijn]) (*Term[NamedDeBruijn], error) {
	return nil, errors.New("anything for now")
}

func (m *Machine) compute() {}

func (m *Machine) returnCompute() {}

func (m *Machine) forceEvaluate() {}

func (m *Machine) applyEvaluate() {}

func (m *Machine) evalBuiltinApp() {}

func (m *Machine) lookupVar() {}

func (m *Machine) stepAndMaybeSpend() {}

func (m *Machine) spendUnbudgetedSteps() {}

func (m *Machine) spendBudget() {}
