package cek

import (
	"fmt"

	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type MachineContext[T syn.Eval] interface {
	fmt.Stringer

	isMachineContext()
}

type FrameAwaitArg[T syn.Eval] struct {
	Value Value[T]
	Ctx   MachineContext[T]
}

func (f FrameAwaitArg[T]) String() string {
	return fmt.Sprintf("FrameAwaitArg(value=%v, ctx=%v)", f.Value, f.Ctx)
}

func (f FrameAwaitArg[T]) isMachineContext() {}

type FrameAwaitFunTerm[T syn.Eval] struct {
	Env  Env[T]
	Term syn.Term[T]
	Ctx  MachineContext[T]
}

func (f FrameAwaitFunTerm[T]) String() string {
	return fmt.Sprintf(
		"FrameAwaitFunTerm(env=%v, term=%v, ctx=%v)",
		f.Env,
		f.Term,
		f.Ctx,
	)
}

func (f FrameAwaitFunTerm[T]) isMachineContext() {}

type FrameAwaitFunValue[T syn.Eval] struct {
	Value Value[T]
	Ctx   MachineContext[T]
}

func (f FrameAwaitFunValue[T]) String() string {
	return fmt.Sprintf("FrameAwaitFunValue(value=%v, ctx=%v)", f.Value, f.Ctx)
}

func (f FrameAwaitFunValue[T]) isMachineContext() {}

type FrameForce[T syn.Eval] struct {
	Ctx MachineContext[T]
}

func (f FrameForce[T]) String() string {
	return fmt.Sprintf("FrameForce(ctx=%v)", f.Ctx)
}

func (f FrameForce[T]) isMachineContext() {}

type FrameConstr[T syn.Eval] struct {
	Env            Env[T]
	Tag            uint
	Fields         []syn.Term[T]
	ResolvedFields []Value[T]
	Ctx            MachineContext[T]
}

func (f FrameConstr[T]) String() string {
	return fmt.Sprintf(
		"FrameConstr(env=%v, tag=%v, fields=%v, resolvedFields=%v, ctx=%v)",
		f.Env,
		f.Tag,
		f.Fields,
		f.ResolvedFields,
		f.Ctx,
	)
}

func (f FrameConstr[T]) isMachineContext() {}

type FrameCases[T syn.Eval] struct {
	Env      Env[T]
	Branches []syn.Term[T]
	Ctx      MachineContext[T]
}

func (f FrameCases[T]) String() string {
	return fmt.Sprintf(
		"FrameCases(env=%v, branches=%v, ctx=%v)",
		f.Env,
		f.Branches,
		f.Ctx,
	)
}

func (f FrameCases[T]) isMachineContext() {}

type NoFrame struct{}

func (f NoFrame) String() string {
	return "NoFrame"
}

func (f NoFrame) isMachineContext() {}
