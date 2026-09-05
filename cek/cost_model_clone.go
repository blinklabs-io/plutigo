// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cek

// Deep copying for cost models.
//
// BuiltinCosts is an array of *CostingFunc, and each CostingFunc holds its cpu
// and mem models in Arguments interface values that are always pointers. A
// struct copy of the array therefore shares every model with its source, and
// BuiltinCosts.update mutates those models in place. Cloning has to allocate a
// new model for every entry, or building a cost model from protocol parameters
// writes into whatever it was cloned from -- including the package-level
// DefaultBuiltinCosts.
//
// Every Arguments implementation is either plain int64 fields or embeds a
// struct of them, with one exception: ConstantOrTwoArguments carries a nested
// TwoArgument. Those three types clone that nested model as well; the rest
// clone by value.

// cloneTwoArgument deep-copies a nested two-argument model. Every Arguments
// implementation clones to a pointer of its own type, so the assertion back to
// TwoArgument holds for any value that satisfied it going in.
func cloneTwoArgument(model TwoArgument) TwoArgument {
	if model == nil {
		return nil
	}
	cloned, ok := model.cloneArguments().(TwoArgument)
	if !ok {
		return model
	}
	return cloned
}

func (x AboveAndBelowDiagonalModel) cloneArguments() Arguments {
	c := x
	c.model = cloneTwoArgument(x.model)
	return &c
}

func (x AddedSizesModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ConstAboveDiagonalIntoQuadraticXAndYModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ConstAboveDiagonalModel) cloneArguments() Arguments {
	c := x
	c.model = cloneTwoArgument(x.model)
	return &c
}

func (x ConstBelowDiagonalModel) cloneArguments() Arguments {
	c := x
	c.model = cloneTwoArgument(x.model)
	return &c
}

func (x ConstantCost) cloneArguments() Arguments {
	c := x
	return &c
}

func (x DropListCost) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ExpMod) cloneArguments() Arguments {
	c := x
	return &c
}

func (x FourLinearInU) cloneArguments() Arguments {
	c := x
	return &c
}

func (x LinearCost) cloneArguments() Arguments {
	c := x
	return &c
}

func (x LinearInX) cloneArguments() Arguments {
	c := x
	return &c
}

func (x LinearInXAndY) cloneArguments() Arguments {
	c := x
	return &c
}

func (x LinearInY) cloneArguments() Arguments {
	c := x
	return &c
}

func (x LinearOnDiagonalModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x MaxSizeModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x MinSizeModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x MultipliedSizesModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x QuadraticInXModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x QuadraticInYModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x SubtractedSizesModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeAddedSizesModel) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeLinearInMaxYZ) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeLinearInX) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeLinearInY) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeLinearInYandZ) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeLinearInZ) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeLiteralInYorLinearInZ) cloneArguments() Arguments {
	c := x
	return &c
}

func (x ThreeQuadraticInZ) cloneArguments() Arguments {
	c := x
	return &c
}

func (x WithInteractionInXAndY) cloneArguments() Arguments {
	c := x
	return &c
}

// clone returns a CostingFunc sharing no mutable state with cf.
func (cf *CostingFunc[T]) clone() *CostingFunc[T] {
	if cf == nil {
		return nil
	}
	ret := &CostingFunc[T]{}
	if any(cf.mem) != nil {
		if cloned, ok := cf.mem.cloneArguments().(T); ok {
			ret.mem = cloned
		} else {
			ret.mem = cf.mem
		}
	}
	if any(cf.cpu) != nil {
		if cloned, ok := cf.cpu.cloneArguments().(T); ok {
			ret.cpu = cloned
		} else {
			ret.cpu = cf.cpu
		}
	}
	return ret
}
