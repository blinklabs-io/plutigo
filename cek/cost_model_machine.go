package cek

import (
	"errors"
	"fmt"
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
	paramParts := splitParamName(param)
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

// validate rejects machine costs under which evaluation could fail to consume
// budget: every step kind must charge a positive amount of both CPU and memory
// so that a recurring step always depletes the budget, and the one-off startup
// charge must not be negative so that it cannot increase it.
func (mc MachineCosts) validate() error {
	if err := mc.startup.validateCharge("cekStartupCost", 0); err != nil {
		return err
	}
	steps := []struct {
		name   string
		budget ExBudget
	}{
		{"cekVarCost", mc.variable},
		{"cekConstCost", mc.constant},
		{"cekLamCost", mc.lambda},
		{"cekDelayCost", mc.delay},
		{"cekForceCost", mc.force},
		{"cekApplyCost", mc.apply},
		{"cekBuiltinCost", mc.builtin},
		{"cekConstrCost", mc.constr},
		{"cekCaseCost", mc.ccase},
	}
	for _, step := range steps {
		if err := step.budget.validateCharge(step.name, 1); err != nil {
			return err
		}
	}
	return nil
}

func (ex ExBudget) validateCharge(name string, minimum int64) error {
	if ex.Cpu < minimum {
		return fmt.Errorf(
			"%s-exBudgetCPU is %d, must be at least %d",
			name,
			ex.Cpu,
			minimum,
		)
	}
	if ex.Mem < minimum {
		return fmt.Errorf(
			"%s-exBudgetMemory is %d, must be at least %d",
			name,
			ex.Mem,
			minimum,
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
