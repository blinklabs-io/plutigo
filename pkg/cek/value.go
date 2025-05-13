package cek

import (
	"fmt"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type Value[T syn.Eval] interface {
	fmt.Stringer
	toExMem() ExMem
	isValue()
}

type Constant struct {
	Constant syn.IConstant
}

func (c Constant) String() string {
	return fmt.Sprintf("%v", c.Constant)
}

func (Constant) isValue() {}

func (c Constant) toExMem() ExMem {
	return iconstantExMem(c.Constant)()
}

type Delay[T syn.Eval] struct {
	Body syn.Term[T]
	Env  Env[T]
}

func (d Delay[T]) String() string {
	return fmt.Sprintf("Delay[%T]", d.Body)
}

func (Delay[T]) isValue() {}

func (Delay[T]) toExMem() ExMem {
	return ExMem(1)
}

type Lambda[T syn.Eval] struct {
	ParameterName T
	Body          syn.Term[T]
	Env           Env[T]
}

func (l Lambda[T]) String() string {
	return fmt.Sprintf("Lambda[%v]", l.ParameterName)
}

func (l Lambda[T]) isValue() {}

func (Lambda[T]) toExMem() ExMem {
	return ExMem(1)
}

type Builtin[T syn.Eval] struct {
	Func   builtin.DefaultFunction
	Forces uint
	Args   []Value[T]
}

func (b Builtin[T]) String() string {
	return fmt.Sprintf("Builtin[%d args, %d forces]", len(b.Args), b.Forces)
}

func (b Builtin[T]) isValue() {}

func (b Builtin[T]) toExMem() ExMem {
	return ExMem(1)
}

func (b Builtin[T]) NeedsForce() bool {
	return b.Func.ForceCount() > int(b.Forces)
}

func (b *Builtin[T]) ConsumeForce() *Builtin[T] {
	return &Builtin[T]{
		Func:   b.Func,
		Forces: b.Forces + 1,
		Args:   b.Args,
	}
}

func (b *Builtin[T]) ApplyArg(arg Value[T]) *Builtin[T] {
	args := make([]Value[T], len(b.Args))
	copy(args, b.Args)

	args = append(args, arg)

	return &Builtin[T]{
		Func:   b.Func,
		Forces: b.Forces,
		Args:   args,
	}

}

func (b *Builtin[T]) IsReady() bool {
	return b.Func.Arity() == len(b.Args) && b.Func.ForceCount() == int(b.Forces)
}

func (b *Builtin[T]) IsArrow() bool {
	return b.Func.Arity() > len(b.Args)
}

type Constr[T syn.Eval] struct {
	Tag    uint
	Fields []Value[T]
}

func (c Constr[T]) String() string {
	return fmt.Sprintf("Constr[tag=%d, fields=%d]", c.Tag, len(c.Fields))
}

func (c Constr[T]) isValue() {}

func (c Constr[T]) toExMem() ExMem {
	return ExMem(1)
}
