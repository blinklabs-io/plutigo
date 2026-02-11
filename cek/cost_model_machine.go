package cek

import (
	"errors"
	"fmt"
	"strings"
)

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

func (mc *MachineCosts) update(param string, val int64) error {
	paramParts := strings.Split(param, "-")
	if len(paramParts) != 2 {
		return errors.New("malformed machine cost update param: " + param)
	}
	var exBudget *ExBudget
	switch paramParts[0] {
	case "cekApplyCost":
		exBudget = &mc.apply
	case "cekBuiltinCost":
		exBudget = &mc.builtin
	case "cekConstCost":
		exBudget = &mc.constant
	case "cekDelayCost":
		exBudget = &mc.delay
	case "cekForceCost":
		exBudget = &mc.force
	case "cekLamCost":
		exBudget = &mc.lambda
	case "cekStartupCost":
		exBudget = &mc.startup
	case "cekVarCost":
		exBudget = &mc.variable
	case "cekConstrCost":
		exBudget = &mc.constr
	case "cekCaseCost":
		exBudget = &mc.ccase
	default:
		return errors.New("unknown machine cost prefix: " + paramParts[0])
	}
	switch paramParts[1] {
	case "exBudgetCPU":
		exBudget.Cpu = int64(val)
	case "exBudgetMemory":
		exBudget.Mem = int64(val)
	default:
		return fmt.Errorf(
			"unknown machine cost suffix for prefix %s: %s",
			paramParts[0],
			paramParts[1],
		)
	}
	return nil
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
