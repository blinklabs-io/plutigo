package plutus

import (
	"errors"
)

type MachineState interface {
	isDone() bool
}

type MachineContext interface{}

type Value interface{}

type Return struct {
	ctx   MachineContext
	value Value
}

func (r Return) isDone() bool {
	return false
}

type Env []Value

type Compute struct {
	ctx  MachineContext
	env  Env
	term Term[NamedDeBruijn]
}

func (c Compute) isDone() bool {
	return false
}

type Done struct {
	term Term[NamedDeBruijn]
}

func (d Done) isDone() bool {
	return true
}

type FrameAwaitArg struct {
	value Value
	ctx   MachineContext
}

type FrameAwaitFunTerm struct {
	env  Env
	term Term[NamedDeBruijn]
	ctx  MachineContext
}

type FrameAwaitFunValue struct {
	value Value
	ctx   MachineContext
}

type FrameForce struct{ ctx MachineContext }

type FrameConstr struct {
	env            Env
	tag            uint64
	fields         []Term[NamedDeBruijn]
	resolvedFields []Value
	ctx            MachineContext
}

type FrameCases struct {
	env      Env
	branches []Term[NamedDeBruijn]
	ctx      MachineContext
}

type NoFrame struct{}

type ExBudget struct {
	mem int64
	cpu int64
}

func (ex ExBudget) occurrences(n uint32) ExBudget {
	return ExBudget{
		mem: ex.mem * int64(n),
		cpu: ex.cpu * int64(n),
	}
}

var DefaultExBudget = ExBudget{
	mem: 14000000,
	cpu: 10000000000,
}

type MachineCosts struct {
	startup  ExBudget
	variable ExBudget
	constant ExBudget
	lambda   ExBudget
	delay    ExBudget
	force    ExBudget
	apply    ExBudget
	constr   ExBudget
	ccase    ExBudget
	/// Just the cost of evaluating a Builtin node not the builtin itself.
	builtin ExBudget
}

func (mc MachineCosts) get(kind StepKind) ExBudget {
	switch kind {
	case ExConstant:
		return mc.constant
	case ExVar:
		return mc.variable
	case ExLambda:
		return mc.lambda
	case ExDelay:
		return mc.delay
	case ExForce:
		return mc.force
	case ExApply:
		return mc.apply
	case ExBuiltin:
		return mc.builtin
	case ExConstr:
		return mc.constr
	case ExCase:
		return mc.ccase
	default:
		panic("invalid step kind")
	}
}

var DefaultMachineCosts = MachineCosts{
	startup: ExBudget{mem: 100, cpu: 100},
	variable: ExBudget{
		mem: 100,
		cpu: 23000,
	},
	constant: ExBudget{
		mem: 100,
		cpu: 23000,
	},
	lambda: ExBudget{
		mem: 100,
		cpu: 23000,
	},
	delay: ExBudget{
		mem: 100,
		cpu: 23000,
	},
	force: ExBudget{
		mem: 100,
		cpu: 23000,
	},
	apply: ExBudget{
		mem: 100,
		cpu: 23000,
	},
	builtin: ExBudget{
		mem: 100,
		cpu: 23000,
	},
	// Placeholder values
	constr: ExBudget{
		mem: 30000000000,
		cpu: 30000000000,
	},
	ccase: ExBudget{
		mem: 30000000000,
		cpu: 30000000000,
	},
}

type StepKind uint8

const (
	ExConstant StepKind = iota
	ExVar
	ExLambda
	ExApply
	ExDelay
	ExForce
	ExBuiltin
	ExConstr
	ExCase
)

type CostModel struct {
	machineCosts MachineCosts
	// builtinCosts map[Builtin]ExBudget
}

var DefaultCostModel = CostModel{
	machineCosts: DefaultMachineCosts,
}

type Machine struct {
	costs           CostModel
	slippage        uint32
	exBudget        ExBudget
	unbudgetedSteps [10]uint32
	Logs            []string
}

func CreateMachine(slippage uint32) Machine {
	return Machine{
		DefaultCostModel,
		slippage,
		DefaultExBudget,
		[10]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		make([]string, 0),
	}
}

func (m *Machine) Run(term *Term[NamedDeBruijn]) (Term[NamedDeBruijn], error) {
	startupBudget := m.costs.machineCosts.startup
	m.spendBudget(startupBudget)

	var state MachineState = Compute{ctx: NoFrame{}, env: make([]Value, 0), term: term}

	var err error
	for {
		switch state.(type) {
		case Compute:
			state, err = m.compute()
		case Return:
			state, err = m.returnCompute()
		case Done:
			return state.(Done).term, nil
		}

		if err != nil {
			return nil, err
		}
	}
}

func (m *Machine) compute() (MachineState, error) {
	return nil, nil
}

func (m *Machine) returnCompute() (MachineState, error) {
	return nil, nil
}

func (m *Machine) forceEvaluate() {}

func (m *Machine) applyEvaluate() {}

func (m *Machine) evalBuiltinApp() {}

func (m *Machine) lookupVar() {}

func (m *Machine) stepAndMaybeSpend() {}

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
