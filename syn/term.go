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

package syn

import (
	"github.com/blinklabs-io/plutigo/builtin"
)

type Term[T any] interface {
	isTerm()
}

// x
type Var[T any] struct {
	Name T
}

func (Var[T]) isTerm() {}

// (delay x)
type Delay[T any] struct {
	Term Term[T]
}

func (Delay[T]) isTerm() {}

// (force x)
type Force[T any] struct {
	Term Term[T]
}

func (Force[T]) isTerm() {}

// (lam x x)
type Lambda[T any] struct {
	ParameterName T
	Body          Term[T]
}

func (Lambda[T]) isTerm() {}

// [ (lam x x) (con integer 1) ]
type Apply[T any] struct {
	Function Term[T]
	Argument Term[T]
}

func (Apply[T]) isTerm() {}

// (builtin addInteger)
type Builtin struct {
	builtin.DefaultFunction
}

func (Builtin) isTerm() {}

// (constr 0 (con integer 1) (con string "1234"))
type Constr[T any] struct {
	Tag    uint64
	Fields []Term[T]
}

func (Constr[T]) isTerm() {}

// (case (constr 0) (constr 1 (con integer 1)))
type Case[T any] struct {
	Constr   Term[T]
	Branches []Term[T]
}

func (Case[T]) isTerm() {}

// (error )
type Error struct{}

func (Error) isTerm() {}

// (con integer 1)
type Constant struct {
	Con IConstant
}

func (Constant) isTerm() {}
