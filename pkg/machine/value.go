package machine

import (
	"fmt"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type Value interface {
	fmt.Stringer
	isValue()
}

type Constant struct {
	Constant syn.IConstant
}

func (c Constant) String() string {
	return fmt.Sprintf("%v", c.Constant)
}

func (c Constant) isValue() {}

type Delay struct {
	Body syn.Term[syn.Eval]
	Env  Env
}

func (d Delay) String() string {
	return fmt.Sprintf("Delay[%T]", d.Body)
}

func (d Delay) isValue() {}

type Lambda struct {
	ParameterName syn.Eval
	Body          syn.Term[syn.Eval]
	Env           Env
}

func (l Lambda) String() string {
	return fmt.Sprintf("Lambda[%v]", l.ParameterName)
}

func (l Lambda) isValue() {}

type Builtin struct {
	Func   builtin.DefaultFunction
	Forces uint
	Args   []Value
}

func (b Builtin) String() string {
	return fmt.Sprintf("Builtin[%d args, %d forces]", len(b.Args), b.Forces)
}

func (b Builtin) isValue() {}

func (b Builtin) NeedsForce() bool {
	return b.Func.ForceCount() > int(b.Forces)
}

func (b Builtin) ConsumeForce() Builtin {
	return Builtin{
		Func:   b.Func,
		Forces: b.Forces + 1,
		Args:   b.Args,
	}
}

func (b Builtin) ApplyArg(arg Value) Builtin {
	args := make([]Value, len(b.Args)+1)
	copy(args, b.Args)

	args = append(args, arg)

	return Builtin{
		Func:   b.Func,
		Forces: b.Forces,
		Args:   args,
	}

}

func (b Builtin) IsReady() bool {
	return b.Func.Arity() == len(b.Args) && b.Func.ForceCount() == int(b.Forces)
}

func (b Builtin) IsArrow() bool {
	return b.Func.Arity() > len(b.Args)
}

type Constr struct {
	Tag    uint64
	Fields []Value
}

func (c Constr) String() string {
	return fmt.Sprintf("Constr[tag=%d, fields=%d]", c.Tag, len(c.Fields))
}

func (c Constr) isValue() {}
