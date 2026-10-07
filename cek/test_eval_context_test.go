package cek

// testEvalContext is the unversioned context the tests in this package run
// under: the default cost model, SemanticsVariantC and no protocol major
// version, so builtin availability follows the language version alone.
func testEvalContext() *EvalContext {
	return &EvalContext{
		CostModel:        DefaultCostModel,
		SemanticsVariant: SemanticsVariantC,
	}
}
