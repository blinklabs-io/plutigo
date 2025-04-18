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

type Constr struct {
	Tag    uint64
	Fields []Value
}

func (c Constr) String() string {
	return fmt.Sprintf("Constr[tag=%d, fields=%d]", c.Tag, len(c.Fields))
}

func (c Constr) isValue() {}
