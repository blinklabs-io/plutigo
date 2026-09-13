package cek

import (
	"fmt"
	"log"
)

// EvalContext contains the cost model and semantics variant for a script evaluation.
//
// A *EvalContext is immutable after construction and safe to share and reuse
// concurrently across any number of goroutines and evaluations: NewMachine
// only reads from it (cek/machine.go), and nothing on the evaluation path --
// [Machine.Run], [Machine.RunContext], or anything they call -- ever writes
// through it. The only code that mutates a CostModel's MachineCosts or
// BuiltinCosts is their unexported update methods, and those run solely
// during construction, from costModelFromList/costModelFromMap
// (cek/cost_model.go); no reachable step of evaluation calls them.
//
// This makes it safe, and recommended, to build one EvalContext per distinct
// (LanguageVersion, ProtoVersion.Major, cost model parameter list) and cache
// it for reuse across every redeemer evaluation that shares that key, rather
// than calling [NewEvalContext] again per evaluation. ProtoVersion.Minor is
// not part of the key: [GetSemantics] switches only on Major, and EvalContext
// does not retain a ProtoVersion at all (only ProtoMajor). See
// TestEvalContextReuseIsRaceFree for a -race-covered proof of concurrent
// reuse, and TestCostModelConstructionIsolation for the concurrent
// *construction* isolation this guarantee builds on.
type EvalContext struct {
	CostModel        CostModel
	SemanticsVariant SemanticsVariant
	ProtoMajor       uint
}

// NewDefaultEvalContext builds an EvalContext using the default cost model
// while still honoring the supplied protocol version for semantics and builtin
// availability gating.
func NewDefaultEvalContext(
	version LanguageVersion,
	protoVersion ProtoVersion,
) *EvalContext {
	semantics := GetSemantics(version, protoVersion)
	costModel := DefaultCostModel
	if builtinCosts, err := buildBuiltinCosts(version, semantics); err == nil {
		costModel = DefaultCostModel.Clone()
		costModel.builtinCosts = builtinCosts
	}
	return &EvalContext{
		CostModel:        costModel,
		SemanticsVariant: semantics,
		ProtoMajor:       protoVersion.Major,
	}
}

// NewEvalContext returns a new EvalContext based on the provided language version, protocol version, and
// cost models from protocol parameters.
//
// The returned *EvalContext is immutable after construction (see the type's
// doc comment) and reusable across evaluations and goroutines. Callers that
// evaluate many redeemers under the same (version, protoVersion.Major,
// costModelParams) should call NewEvalContext once and cache the result keyed
// on that tuple, rather than rebuilding it per evaluation.
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
			semanticsName = "VariantC (pre-van Rossem V3+)"
		case SemanticsVariantD:
			semanticsName = "VariantD (van Rossem V1/V2)"
		case SemanticsVariantE:
			semanticsName = "VariantE (van Rossem V3+)"
		}
		log.Printf(
			"[PLUTIGO-DEBUG] NewEvalContext: langVersion=%v, protoVersion=%d.%d, semantics=%s, costModelParams=%d",
			version,
			protoVersion.Major,
			protoVersion.Minor,
			semanticsName,
			len(costModelParams),
		)
	}

	ret := &EvalContext{
		SemanticsVariant: semantics,
	}
	ret.ProtoMajor = protoVersion.Major
	costModel, err := costModelFromList(
		version,
		ret.SemanticsVariant,
		costModelParams,
	)
	if err != nil {
		return nil, fmt.Errorf("build cost model: %w", err)
	}
	ret.CostModel = costModel

	// Log the actual machine costs for debugging
	if DebugBudget {
		mc := costModel.machineCosts
		log.Printf(
			"[PLUTIGO-DEBUG] MachineCosts after loading: startup=(cpu=%d, mem=%d), var=(cpu=%d, mem=%d), const=(cpu=%d, mem=%d), lambda=(cpu=%d, mem=%d), delay=(cpu=%d, mem=%d), force=(cpu=%d, mem=%d), apply=(cpu=%d, mem=%d), builtin=(cpu=%d, mem=%d)",
			mc.startup.Cpu,
			mc.startup.Mem,
			mc.variable.Cpu,
			mc.variable.Mem,
			mc.constant.Cpu,
			mc.constant.Mem,
			mc.lambda.Cpu,
			mc.lambda.Mem,
			mc.delay.Cpu,
			mc.delay.Mem,
			mc.force.Cpu,
			mc.force.Mem,
			mc.apply.Cpu,
			mc.apply.Mem,
			mc.builtin.Cpu,
			mc.builtin.Mem,
		)
	}

	return ret, nil
}
