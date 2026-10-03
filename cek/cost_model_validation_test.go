package cek

import (
	"math"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

var recurringStepCostParams = []string{
	"cekApplyCost",
	"cekBuiltinCost",
	"cekConstCost",
	"cekDelayCost",
	"cekForceCost",
	"cekLamCost",
	"cekVarCost",
	"cekConstrCost",
	"cekCaseCost",
}

func paramIndex(t *testing.T, names []string, name string) int {
	t.Helper()
	for i, n := range names {
		if n == name {
			return i
		}
	}
	t.Fatalf("no cost model parameter %q", name)
	return -1
}

func TestNewEvalContextRejectsNonPositiveRecurringStepCosts(t *testing.T) {
	t.Parallel()

	version := lang.LanguageVersionV3
	names := lang.GetParamNamesForVersion(version)
	if _, err := NewEvalContext(
		version,
		ProtoVersion{Major: 11},
		synthCostModelParams(names),
	); err != nil {
		t.Fatalf("valid model rejected: %v", err)
	}

	for _, step := range recurringStepCostParams {
		for _, suffix := range []string{"exBudgetCPU", "exBudgetMemory"} {
			for _, value := range []int64{0, -1} {
				param := step + "-" + suffix
				t.Run(param, func(t *testing.T) {
					t.Parallel()
					idx := paramIndex(t, names, param)
					params := synthCostModelParams(names)
					params[idx] = value
					_, err := NewEvalContext(
						version,
						ProtoVersion{Major: 11},
						params,
					)
					if err == nil {
						t.Fatalf("%s=%d accepted, want error", param, value)
					}
					if !strings.Contains(err.Error(), param) {
						t.Fatalf("error %q does not name %s", err, param)
					}
				})
			}
		}
	}
}

func TestNewEvalContextRejectsNegativeStartupCost(t *testing.T) {
	t.Parallel()

	version := lang.LanguageVersionV3
	names := lang.GetParamNamesForVersion(version)
	for _, suffix := range []string{"exBudgetCPU", "exBudgetMemory"} {
		params := synthCostModelParams(names)
		params[paramIndex(t, names, "cekStartupCost-"+suffix)] = -1
		if _, err := NewEvalContext(
			version,
			ProtoVersion{Major: 11},
			params,
		); err == nil {
			t.Fatalf("negative cekStartupCost-%s accepted", suffix)
		}
	}
}

func TestNewMachineRejectsUnvalidatedCostModel(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("NewMachine accepted a zero-cost evaluation context")
		}
	}()
	NewMachine[syn.DeBruijn](
		lang.LanguageVersionV3,
		0,
		&EvalContext{SemanticsVariant: SemanticsVariantC, ProtoMajor: 11},
	)
}

func TestNewMachineRejectsNilEvalContext(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("NewMachine accepted a nil evaluation context")
		}
	}()
	NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, nil)
}

func TestBuiltinCostFormulasSaturate(t *testing.T) {
	t.Parallel()

	const big = math.MaxInt64
	huge := ExMem(1 << 40)
	tests := []struct {
		name string
		got  int64
	}{
		{"linear", LinearCost{slope: big, intercept: big}.Cost(huge)},
		{"linear_in_x_and_y", LinearInXAndY{TwoVariableLinearSize{
			slope1: big, slope2: big, intercept: big,
		}}.CostTwo(huge, huge)},
		{"added_sizes", AddedSizesModel{AddedSizes{
			slope: big, intercept: big,
		}}.CostTwo(huge, huge)},
		{"multiplied_sizes", MultipliedSizesModel{MultipliedSizes{
			slope: 1, intercept: 0,
		}}.CostTwo(ExMem(math.MaxInt64), ExMem(math.MaxInt64))},
		{"quadratic_in_x", QuadraticInXModel{QuadraticFunction{
			coeff0: 0, coeff1: 0, coeff2: 1,
		}}.Cost(ExMem(math.MaxInt64))},
		{"quadratic_in_y", QuadraticInYModel{QuadraticFunction{
			coeff0: 0, coeff1: 0, coeff2: big,
		}}.CostTwo(0, huge)},
		{"with_interaction", WithInteractionInXAndY{
			c00: 0, c01: 0, c10: 0, c11: big,
		}.CostTwo(huge, huge)},
		{"three_added_sizes", ThreeAddedSizesModel{AddedSizes{
			slope: big, intercept: big,
		}}.CostThree(huge, huge, huge)},
		{"three_quadratic_in_z", ThreeQuadraticInZ{QuadraticFunction{
			coeff0: 0, coeff1: 0, coeff2: big,
		}}.CostThree(0, 0, huge)},
		{"three_linear_in_y_and_z", ThreeLinearInYandZ{TwoVariableLinearSize{
			slope1: big, slope2: big, intercept: big,
		}}.CostThree(0, huge, huge)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != math.MaxInt64 {
				t.Fatalf("cost = %d, want saturation at MaxInt64", tt.got)
			}
		})
	}
}

// TestBatchedStepCostsDoNotWrap charges four delay and four force steps with
// a per-step cost whose four-fold product wraps int64 to a small value. The
// machine must report budget exhaustion rather than charge the wrapped cost.
func TestBatchedStepCostsDoNotWrap(t *testing.T) {
	t.Parallel()

	version := lang.LanguageVersionV3
	names := lang.GetParamNamesForVersion(version)
	params := synthCostModelParams(names)
	for _, step := range recurringStepCostParams {
		params[paramIndex(t, names, step+"-exBudgetCPU")] = 1<<62 + 1
	}
	evalCtx, err := NewEvalContext(version, ProtoVersion{Major: 11}, params)
	if err != nil {
		t.Fatalf("NewEvalContext: %v", err)
	}
	program, err := syn.Parse(`(program 1.1.0
		(force (force (force (force
		  (delay (delay (delay (delay (con integer 1))))))))))`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	db, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn: %v", err)
	}
	machine := NewMachine[syn.DeBruijn](version, 100, evalCtx)
	machine.ExBudget = ExBudget{Cpu: math.MaxInt64, Mem: math.MaxInt64}
	if _, err := machine.Run(db.Term); !IsBudgetError(err) {
		t.Fatalf("Run error = %v, want a budget error", err)
	}
}

func TestSaturatingArithmeticBounds(t *testing.T) {
	t.Parallel()

	const (
		maxI = int64(math.MaxInt64)
		minI = int64(math.MinInt64)
	)
	adds := []struct{ x, y, want int64 }{
		{1, 2, 3},
		{maxI, 1, maxI},
		{maxI, maxI, maxI},
		{minI, -1, minI},
		{minI, maxI, -1},
		{maxI, minI, -1},
		{5, -7, -2},
	}
	for _, tt := range adds {
		if got := satAdd(tt.x, tt.y); got != tt.want {
			t.Errorf("satAdd(%d, %d) = %d, want %d", tt.x, tt.y, got, tt.want)
		}
	}
	muls := []struct{ x, y, want int64 }{
		{0, maxI, 0},
		{3, 4, 12},
		{-3, 4, -12},
		{maxI, 2, maxI},
		{maxI, -2, minI},
		{minI, -1, maxI},
		{-1, minI, maxI},
		{minI, 1, minI},
		{1 << 32, 1 << 32, maxI},
		{-(1 << 32), 1 << 32, minI},
	}
	for _, tt := range muls {
		if got := satMul(tt.x, tt.y); got != tt.want {
			t.Errorf("satMul(%d, %d) = %d, want %d", tt.x, tt.y, got, tt.want)
		}
	}
}

// TestNegativeBuiltinCostNeverIncreasesBudget loads a cost model whose
// addInteger cost comes out negative. The machine must refuse the charge
// rather than credit the budget.
func TestNegativeBuiltinCostNeverIncreasesBudget(t *testing.T) {
	t.Parallel()

	version := lang.LanguageVersionV3
	names := lang.GetParamNamesForVersion(version)
	params := synthCostModelParams(names)
	params[paramIndex(t, names, "addInteger-cpu-arguments-intercept")] = -1 << 40
	evalCtx, err := NewEvalContext(version, ProtoVersion{Major: 11}, params)
	if err != nil {
		t.Fatalf("NewEvalContext: %v", err)
	}
	program, err := syn.Parse(`(program 1.1.0
		[ [ (builtin addInteger) (con integer 1) ] (con integer 1) ])`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	db, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatalf("NameToDeBruijn: %v", err)
	}

	machine := NewMachine[syn.DeBruijn](version, 0, evalCtx)
	initial := ExBudget{Cpu: 1_000_000_000, Mem: 1_000_000_000}
	machine.ExBudget = initial
	_, err = machine.Run(db.Term)
	if !IsBudgetError(err) {
		t.Fatalf("Run error = %v, want a budget error", err)
	}
	if machine.ExBudget.Cpu > initial.Cpu || machine.ExBudget.Mem > initial.Mem {
		t.Fatalf("budget grew from %+v to %+v", initial, machine.ExBudget)
	}
}
