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
	return fmt.Sprintf("Var: %v", v.Name)
}

// (delay x)
type Delay[T any] struct {
	Term Term[T]
}

func (Delay[T]) isTerm() {}
func (v Delay[T]) String() string {
	return fmt.Sprintf("Delay: %v", v.Term)
}

// (force x)
type Force[T any] struct {
	Term Term[T]
}

func (Force[T]) isTerm() {}
func (v Force[T]) String() string {
	return fmt.Sprintf("Force: %v", v.Term)
}

// (lam x x)
type Lambda[T any] struct {
	ParameterName T
	Body          Term[T]
}

func (Lambda[T]) isTerm() {}
func (v Lambda[T]) String() string {
	return fmt.Sprintf("Lambda: %v %v", v.ParameterName, v.Body)
}

// [ (lam x x) (con integer 1) ]
type Apply[T any] struct {
	Function Term[T]
	Argument Term[T]
}

func (Apply[T]) isTerm() {}
func (v Apply[T]) String() string {
	return fmt.Sprintf("Apply: %v %v", v.Function, v.Argument)
}

// (builtin addInteger)
type Builtin struct {
	builtin.DefaultFunction
}

func (Builtin) isTerm() {}
func (v Builtin) String() string {
	return fmt.Sprintf("Builtin: %v", v.DefaultFunction)
}

// (constr 0 (con integer 1) (con string "1234"))
type Constr[T any] struct {
	Tag    uint
	Fields *[]Term[T]
}

func (Constr[T]) isTerm() {}
func (v Constr[T]) String() string {
	return fmt.Sprintf("Constr: %v %v", v.Tag, v.Fields)
}

// (case (constr 0) (constr 1 (con integer 1)))
type Case[T any] struct {
	Constr   Term[T]
	Branches *[]Term[T]
}

func (Case[T]) isTerm() {}
func (v Case[T]) String() string {
	return fmt.Sprintf("Case: %v %v", v.Constr, v.Branches)
}

// (error )
type Error struct{}

func (Error) isTerm() {}
func (v Error) String() string {
	return fmt.Sprintf("Error")
}

// (con integer 1)
type Constant struct {
	Con IConstant
}

func (Constant) isTerm() {}
func (v Constant) String() string {
	return fmt.Sprintf("Constant: %v", v.Con)
}
