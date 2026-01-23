package cek

import (
	"fmt"
)

// EvalContext contains the cost model and semantics variant for a script evaluation
type EvalContext struct {
	CostModel        CostModel
	SemanticsVariant SemanticsVariant
}

// NewEvalContext returns a new EvalContext based on the provided language version, protocol version, and
// cost models from protocol parameters
func NewEvalContext(
	version LanguageVersion,
	protoVersion ProtoVersion,
	costModelParams []int64,
) (*EvalContext, error) {
	ret := &EvalContext{
		SemanticsVariant: GetSemantics(version, protoVersion),
	}
	costModel, err := CostModelFromList(version, costModelParams)
	if err != nil {
		return nil, fmt.Errorf("build cost model: %w", err)
	}
	ret.CostModel = costModel
	return ret, nil
}
