package cek

import (
	"fmt"
	"log"
)

// EvalContext contains the cost model and semantics variant for a script evaluation
type EvalContext struct {
	CostModel        CostModel
	SemanticsVariant SemanticsVariant
	ProtoMajor       uint
}

// NewEvalContext returns a new EvalContext based on the provided language version, protocol version, and
// cost models from protocol parameters
func NewEvalContext(
	version LanguageVersion,
	protoVersion ProtoVersion,
	costModelParams []int64,
) (*EvalContext, error) {
	semantics := GetSemantics(version, protoVersion)

	// Diagnostic logging for debugging budget calculation issues
	if DebugBudget {
		semanticsName := "Unknown"
		switch semantics {
		case SemanticsVariantA:
			semanticsName = "VariantA (pre-Chang)"
		case SemanticsVariantB:
			semanticsName = "VariantB (post-Chang V1/V2)"
		case SemanticsVariantC:
			semanticsName = "VariantC (V3+)"
		}
		log.Printf("[PLUTIGO-DEBUG] NewEvalContext: langVersion=%v, protoVersion=%d.%d, semantics=%s, costModelParams=%d",
			version, protoVersion.Major, protoVersion.Minor, semanticsName, len(costModelParams))
	}

	ret := &EvalContext{
		SemanticsVariant: semantics,
	}
	ret.ProtoMajor = protoVersion.Major
	costModel, err := costModelFromList(version, ret.SemanticsVariant, costModelParams)
	if err != nil {
		return nil, fmt.Errorf("build cost model: %w", err)
	}
	ret.CostModel = costModel

	// Log the actual machine costs for debugging
	if DebugBudget {
		mc := costModel.machineCosts
		log.Printf("[PLUTIGO-DEBUG] MachineCosts after loading: startup=(cpu=%d, mem=%d), var=(cpu=%d, mem=%d), const=(cpu=%d, mem=%d), lambda=(cpu=%d, mem=%d), delay=(cpu=%d, mem=%d), force=(cpu=%d, mem=%d), apply=(cpu=%d, mem=%d), builtin=(cpu=%d, mem=%d)",
			mc.startup.Cpu, mc.startup.Mem,
			mc.variable.Cpu, mc.variable.Mem,
			mc.constant.Cpu, mc.constant.Mem,
			mc.lambda.Cpu, mc.lambda.Mem,
			mc.delay.Cpu, mc.delay.Mem,
			mc.force.Cpu, mc.force.Mem,
			mc.apply.Cpu, mc.apply.Mem,
			mc.builtin.Cpu, mc.builtin.Mem)
	}

	return ret, nil
}
