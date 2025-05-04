package cek

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
