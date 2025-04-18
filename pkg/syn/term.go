package syn

import (
	"fmt"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
)

type Term[T any] interface {
	fmt.Stringer
	isTerm()
}

// x
type Var[T any] struct {
	Name T
}

func (Var[T]) isTerm() {}
func (v Var[T]) String() string {
	return "TODO"
}

// (delay x)
type Delay[T any] struct {
	Term Term[T]
}

func (Delay[T]) isTerm() {}
func (v Delay[T]) String() string {
	return "TODO"
}

// (force x)
type Force[T any] struct {
	Term Term[T]
}

func (Force[T]) isTerm() {}
func (v Force[T]) String() string {
	return "TODO"
}

// (lam x x)
type Lambda[T any] struct {
	ParameterName T
	Body          Term[T]
}

func (Lambda[T]) isTerm() {}
func (v Lambda[T]) String() string {
	return "TODO"
}

// [ (lam x x) (con integer 1) ]
type Apply[T any] struct {
	Function Term[T]
	Argument Term[T]
}

func (Apply[T]) isTerm() {}
func (v Apply[T]) String() string {
	return "TODO"
}

// (builtin addInteger)
type Builtin struct {
	builtin.DefaultFunction
}

func (Builtin) isTerm() {}
func (v Builtin) String() string {
	return "TODO"
}

// (constr 0 (con integer 1) (con string "1234"))
type Constr[T any] struct {
	Tag    uint64
	Fields *[]Term[T]
}

func (Constr[T]) isTerm() {}
func (v Constr[T]) String() string {
	return "TODO"
}

// (case (constr 0) (constr 1 (con integer 1)))
type Case[T any] struct {
	Constr   Term[T]
	Branches *[]Term[T]
}

func (Case[T]) isTerm() {}
func (v Case[T]) String() string {
	return "TODO"
}

// (error )
type Error struct{}

func (Error) isTerm() {}
func (v Error) String() string {
	return "TODO"
}

// (con integer 1)
type Constant struct {
	Con IConstant
}

func (Constant) isTerm() {}
func (v Constant) String() string {
	return "TODO"
}
