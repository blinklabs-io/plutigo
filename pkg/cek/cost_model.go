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
	startup: ExBudget{Mem: 100, Cpu: 100},
	variable: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	constant: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	lambda: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	delay: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	force: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	apply: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	builtin: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	// Placeholder values
	constr: ExBudget{
		Mem: 100,
		Cpu: 16000,
	},
	ccase: ExBudget{
		Mem: 100,
		Cpu: 16000,
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
