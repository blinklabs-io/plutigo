package cek

import "github.com/blinklabs-io/plutigo/pkg/builtin"

type BuiltinCosts map[builtin.DefaultFunction]*CostingFunc[Arguments]

var DefaultBuiltinCosts = BuiltinCosts{
	builtin.AddInteger: &CostingFunc[Arguments]{
		mem: &MaxSizeModel{MaxSize{
			intercept: 1,
			slope:     1,
		}},
		cpu: &MaxSizeModel{MaxSize{
			intercept: 100788,
			slope:     420,
		}},
	},
}

type CostingFunc[T Arguments] struct {
	mem T
	cpu T
}

func CostSingle[T OneArgument](cf CostingFunc[T], x func() ExMem) ExBudget {
	memConstants := cf.mem.HasConstants()

	cpuConstants := cf.cpu.HasConstants()

	var usedBudget ExBudget

	if memConstants[0] && cpuConstants[0] {
		usedBudget = ExBudget{
			Mem: int64(cf.mem.Cost(ExMem(0))),
			Cpu: int64(cf.cpu.Cost(ExMem(0))),
		}
	} else {
		i := x()
		usedBudget = ExBudget{
			Mem: int64(cf.mem.Cost(i)),
			Cpu: int64(cf.cpu.Cost(i)),
		}
	}

	return usedBudget
}

// Function to cost TwoArguments similar to CostSingle for OneArgument
func CostPair[T TwoArgument](cf CostingFunc[T], x, y func() ExMem) ExBudget {
	memConstants := cf.mem.HasConstants()
	cpuConstants := cf.cpu.HasConstants()

	var xMem ExMem
	var yMem ExMem

	if memConstants[0] && cpuConstants[0] {
		xMem = ExMem(0)
	} else {
		xMem = x()
	}

	if memConstants[1] && cpuConstants[1] {
		yMem = ExMem(1)
	} else {
		yMem = y()
	}

	return ExBudget{
		Mem: int64(cf.mem.CostTwo(xMem, yMem)),
		Cpu: int64(cf.cpu.CostTwo(xMem, yMem)),
	}
}

// Function to cost ThreeArguments
func CostTriple[T ThreeArgument](cf CostingFunc[T], x, y, z func() ExMem) ExBudget {
	memConstants := cf.mem.HasConstants()
	cpuConstants := cf.cpu.HasConstants()

	var xMem, yMem, zMem ExMem

	if memConstants[0] && cpuConstants[0] {
		xMem = ExMem(0)
	} else {
		xMem = x()
	}

	if memConstants[1] && cpuConstants[1] {
		yMem = ExMem(0)
	} else {
		yMem = y()
	}

	if memConstants[2] && cpuConstants[2] {
		zMem = ExMem(0)
	} else {
		zMem = z()
	}

	return ExBudget{
		Mem: int64(cf.mem.CostThree(xMem, yMem, zMem)),
		Cpu: int64(cf.cpu.CostThree(xMem, yMem, zMem)),
	}
}

// Function to cost SixArguments
func CostSextuple[T SixArgument](cf CostingFunc[T], a, b, c, d, e, f func() ExMem) ExBudget {
	memConstants := cf.mem.HasConstants()
	cpuConstants := cf.cpu.HasConstants()

	var aMem, bMem, cMem, dMem, eMem, fMem ExMem

	// Check each argument individually
	if memConstants[0] && cpuConstants[0] {
		aMem = ExMem(0)
	} else {
		aMem = a()
	}

	if memConstants[1] && cpuConstants[1] {
		bMem = ExMem(0)
	} else {
		bMem = b()
	}

	if memConstants[2] && cpuConstants[2] {
		cMem = ExMem(0)
	} else {
		cMem = c()
	}

	if memConstants[3] && cpuConstants[3] {
		dMem = ExMem(0)
	} else {
		dMem = d()
	}

	if memConstants[4] && cpuConstants[4] {
		eMem = ExMem(0)
	} else {
		eMem = e()
	}

	if memConstants[5] && cpuConstants[5] {
		fMem = ExMem(0)
	} else {
		fMem = f()
	}

	return ExBudget{
		Mem: int64(cf.mem.CostSix(aMem, bMem, cMem, dMem, eMem, fMem)),
		Cpu: int64(cf.cpu.CostSix(aMem, bMem, cMem, dMem, eMem, fMem)),
	}
}

type Arguments interface {
	isArguments()
	HasConstants() []bool
}

type OneArgument interface {
	Cost(x ExMem) int
	Arguments
}

type ConstantCost struct {
	c int
}

func (ConstantCost) isArguments() {}

func (ConstantCost) HasConstants() []bool {
	return []bool{true, true, true, true, true, true}
}

func (c ConstantCost) Cost(x ExMem) int {
	return c.c
}

type LinearCost struct {
	slope     int
	intercept int
}

func (LinearCost) isArguments() {}

func (LinearCost) HasConstants() []bool {
	return []bool{false}
}

func (l LinearCost) Cost(x ExMem) int {
	return l.slope*int(x) + l.intercept
}

// TwoArgument interface for costing functions with two arguments
type TwoArgument interface {
	CostTwo(x, y ExMem) int
	Arguments
}

type TwoVariableLinearSize struct {
	intercept int
	slope1    int
	slope2    int
}

type AddedSizes struct {
	intercept int
	slope     int
}

type SubtractedSizes struct {
	intercept int
	slope     int
	minimum   int
}

type MultipliedSizes struct {
	intercept int
	slope     int
}

type MinSize struct {
	intercept int
	slope     int
}

type MaxSize struct {
	intercept int
	slope     int
}

type ConstantOrLinear struct {
	constant  int
	intercept int
	slope     int
}

type ConstantOrTwoArguments struct {
	constant int
	model    TwoArgument
}

type QuadraticFunction struct {
	coeff0 int
	coeff1 int
	coeff2 int
}

type TwoArgumentsQuadraticFunction struct {
	minimum int
	coeff00 int
	coeff10 int
	coeff01 int
	coeff20 int
	coeff11 int
	coeff02 int
}

// Implementations for TwoArguments variants
// Using existing ConstantCost for two arguments
func (c ConstantCost) CostTwo(x, y ExMem) int {
	return c.c
}

// LinearInX costs based only on the first argument
type LinearInX struct {
	LinearCost
}

func (l LinearInX) CostTwo(x, y ExMem) int {
	return l.Cost(x)
}

// Y is not used so constant
func (LinearInX) HasConstants() []bool {
	return []bool{false, true}
}

func (LinearInX) isArguments() {}

// LinearInY costs based only on the second argument
type LinearInY struct {
	LinearCost
}

func (l LinearInY) CostTwo(x, y ExMem) int {
	return l.Cost(y)
}

// X is not used so constant
func (LinearInY) HasConstants() []bool {
	return []bool{true, false}
}

func (LinearInY) isArguments() {}

// LinearInXAndY costs based on both arguments with different slopes
type LinearInXAndY struct {
	TwoVariableLinearSize
}

func (l LinearInXAndY) CostTwo(x, y ExMem) int {
	return l.slope1*int(x) + l.slope2*int(y) + l.intercept
}

func (LinearInXAndY) HasConstants() []bool {
	return []bool{false, false}
}

func (LinearInXAndY) isArguments() {}

// AddedSizesModel costs based on the sum of arguments
type AddedSizesModel struct {
	AddedSizes
}

func (a AddedSizesModel) CostTwo(x, y ExMem) int {
	return a.slope*(int(x)+int(y)) + a.intercept
}

func (AddedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (AddedSizesModel) isArguments() {}

// SubtractedSizesModel costs based on the difference of arguments
type SubtractedSizesModel struct {
	SubtractedSizes
}

func (s SubtractedSizesModel) CostTwo(x, y ExMem) int {
	diff := max(int(x)-int(y), s.minimum)
	return s.slope*diff + s.intercept
}

func (SubtractedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (SubtractedSizesModel) isArguments() {}

// MultipliedSizesModel costs based on the product of arguments
type MultipliedSizesModel struct {
	MultipliedSizes
}

func (m MultipliedSizesModel) CostTwo(x, y ExMem) int {
	return m.slope*(int(x)*int(y)) + m.intercept
}

func (MultipliedSizesModel) HasConstants() []bool {
	return []bool{false, false}
}

func (MultipliedSizesModel) isArguments() {}

// MinSizeModel costs based on the minimum of arguments
type MinSizeModel struct {
	MinSize
}

func (m MinSizeModel) CostTwo(x, y ExMem) int {
	min := int(x)
	if int(y) < min {
		min = int(y)
	}
	return m.slope*min + m.intercept
}

func (MinSizeModel) HasConstants() []bool {
	return []bool{false, false}
}

func (MinSizeModel) isArguments() {}

// MaxSizeModel costs based on the maximum of arguments
type MaxSizeModel struct {
	MaxSize
}

func (m MaxSizeModel) CostTwo(x, y ExMem) int {
	max := int(x)
	if int(y) > max {
		max = int(y)
	}
	return m.slope*max + m.intercept
}

func (MaxSizeModel) HasConstants() []bool {
	return []bool{false, false}
}

func (MaxSizeModel) isArguments() {}

// LinearOnDiagonalModel costs linearly when arguments are equal, constant otherwise
type LinearOnDiagonalModel struct {
	ConstantOrLinear
}

func (l LinearOnDiagonalModel) CostTwo(x, y ExMem) int {
	if int(x) == int(y) {
		return l.slope*int(x) + l.intercept
	}
	return l.constant
}

func (LinearOnDiagonalModel) HasConstants() []bool {
	return []bool{false, false}
}

func (LinearOnDiagonalModel) isArguments() {}

// ConstAboveDiagonalModel costs constant when x < y, uses another model otherwise
type ConstAboveDiagonalModel struct {
	ConstantOrTwoArguments
}

func (c ConstAboveDiagonalModel) CostTwo(x, y ExMem) int {
	if int(x) < int(y) {
		return c.constant
	}
	return c.model.CostTwo(x, y)
}

func (ConstAboveDiagonalModel) HasConstants() []bool {
	return []bool{false, false}
}

func (ConstAboveDiagonalModel) isArguments() {}

// ConstBelowDiagonalModel costs constant when x > y, uses another model otherwise
type ConstBelowDiagonalModel struct {
	ConstantOrTwoArguments
}

func (c ConstBelowDiagonalModel) CostTwo(x, y ExMem) int {
	if int(x) > int(y) {
		return c.constant
	}
	return c.model.CostTwo(x, y)
}

func (ConstBelowDiagonalModel) HasConstants() []bool {
	return []bool{false, false}
}

func (ConstBelowDiagonalModel) isArguments() {}

// QuadraticInYModel costs based on a quadratic function of y
type QuadraticInYModel struct {
	QuadraticFunction
}

func (q QuadraticInYModel) CostTwo(x, y ExMem) int {
	yVal := int(y)
	return q.coeff0 + (q.coeff1 * yVal) + (q.coeff2 * yVal * yVal)
}

// X is not used so constant
func (QuadraticInYModel) HasConstants() []bool {
	return []bool{true, false}
}

func (QuadraticInYModel) isArguments() {}

// ConstAboveDiagonalIntoQuadraticXAndYModel costs constant when x < y, uses a quadratic function otherwise
type ConstAboveDiagonalIntoQuadraticXAndYModel struct {
	constant int
	TwoArgumentsQuadraticFunction
}

func (c ConstAboveDiagonalIntoQuadraticXAndYModel) CostTwo(x, y ExMem) int {
	if int(x) < int(y) {
		return c.constant
	}

	xVal, yVal := int(x), int(y)
	result := c.coeff00 +
		c.coeff10*xVal +
		c.coeff01*yVal +
		c.coeff20*xVal*xVal +
		c.coeff11*xVal*yVal +
		c.coeff02*yVal*yVal

	if result < c.minimum {
		return c.minimum
	}
	return result
}

func (ConstAboveDiagonalIntoQuadraticXAndYModel) HasConstants() []bool {
	return []bool{false, false}
}

func (ConstAboveDiagonalIntoQuadraticXAndYModel) isArguments() {}

// ThreeArgument interface for costing functions with three arguments
type ThreeArgument interface {
	CostThree(x, y, z ExMem) int
	Arguments
}

// Implementations for ThreeArguments variants

// Using existing ConstantCost for three arguments
func (c ConstantCost) CostThree(x, y, z ExMem) int {
	return c.c
}

// ThreeAddedSizesModel costs based on the sum of three arguments
type ThreeAddedSizesModel struct {
	AddedSizes
}

func (a ThreeAddedSizesModel) CostThree(x, y, z ExMem) int {
	return a.slope*(int(x)+int(y)+int(z)) + a.intercept
}

func (ThreeAddedSizesModel) HasConstants() []bool {
	return []bool{false, false, false}
}

func (ThreeAddedSizesModel) isArguments() {}

// LinearInX costs based only on the first argument
type ThreeLinearInX struct {
	LinearCost
}

func (l ThreeLinearInX) CostThree(x, y, z ExMem) int {
	return l.Cost(x)
}

// Y,Z are not used so constant
func (ThreeLinearInX) HasConstants() []bool {
	return []bool{false, true, true}
}

func (ThreeLinearInX) isArguments() {}

// LinearInY costs based only on the second argument
type ThreeLinearInY struct {
	LinearCost
}

func (l ThreeLinearInY) CostThree(x, y, z ExMem) int {
	return l.Cost(y)
}

// X,Z are not used so constant
func (ThreeLinearInY) HasConstants() []bool {
	return []bool{true, false, true}
}

func (ThreeLinearInY) isArguments() {}

// LinearInZ costs based only on the third argument
type ThreeLinearInZ struct {
	LinearCost
}

func (l ThreeLinearInZ) CostThree(x, y, z ExMem) int {
	return l.Cost(z)
}

// X,Y are not used so constant
func (ThreeLinearInZ) HasConstants() []bool {
	return []bool{true, true, false}
}

func (ThreeLinearInZ) isArguments() {}

// QuadraticInZ costs based on a quadratic function of the third argument
type ThreeQuadraticInZ struct {
	QuadraticFunction
}

// X,Y are not used so constant
func (q ThreeQuadraticInZ) CostThree(x, y, z ExMem) int {
	zVal := int(z)
	return q.coeff0 + (q.coeff1 * zVal) + (q.coeff2 * zVal * zVal)
}

func (ThreeQuadraticInZ) HasConstants() []bool {
	return []bool{true, true, false}
}

func (ThreeQuadraticInZ) isArguments() {}

// LiteralInYorLinearInZ costs y if y != 0, otherwise linear in z
type ThreeLiteralInYorLinearInZ struct {
	LinearCost
}

func (l ThreeLiteralInYorLinearInZ) CostThree(x, y, z ExMem) int {
	if int(y) == 0 {
		return l.slope*int(z) + l.intercept
	}
	return int(y)
}

// X is not used so constant
func (ThreeLiteralInYorLinearInZ) HasConstants() []bool {
	return []bool{true, false, false}
}

func (ThreeLiteralInYorLinearInZ) isArguments() {}

// LinearInMaxYZ costs based on the maximum of y and z
type ThreeLinearInMaxYZ struct {
	LinearCost
}

func (l ThreeLinearInMaxYZ) CostThree(x, y, z ExMem) int {
	max := int(y)
	if int(z) > max {
		max = int(z)
	}
	return l.slope*max + l.intercept
}

// X is not used so constant
func (ThreeLinearInMaxYZ) HasConstants() []bool {
	return []bool{true, false, false}
}

func (ThreeLinearInMaxYZ) isArguments() {}

// LinearInYandZ costs based on both y and z arguments
type ThreeLinearInYandZ struct {
	TwoVariableLinearSize
}

func (l ThreeLinearInYandZ) CostThree(x, y, z ExMem) int {
	return l.slope1*int(y) + l.slope2*int(z) + l.intercept
}

// X is not used so constant
func (ThreeLinearInYandZ) HasConstants() []bool {
	return []bool{true, false, false}
}

func (ThreeLinearInYandZ) isArguments() {}

// SixArgument interface for costing functions with six arguments
type SixArgument interface {
	CostSix(a, b, c, d, e, f ExMem) int
	Arguments
}

// Implementations for SixArguments variants

// Using existing ConstantCost for six arguments
func (c ConstantCost) CostSix(a, b, c2, d, e, f ExMem) int {
	return c.c
}
