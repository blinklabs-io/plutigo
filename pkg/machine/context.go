package machine

import (
	"fmt"

	"github.com/blinklabs-io/plutigo/pkg/syn"
)

type MachineContext interface {
	fmt.Stringer
	isMachineContext()
}

type FrameAwaitArg struct {
	Value []Value
	Ctx   MachineContext
}

func (f FrameAwaitArg) String() string {
	return fmt.Sprintf("FrameAwaitArg(value=%v, ctx=%v)", f.Value, f.Ctx)
}

func (f FrameAwaitArg) isMachineContext() {}

type FrameAwaitFunTerm struct {
	Env  Env
	Term syn.Term[syn.Eval]
	Ctx  MachineContext
}

func (f FrameAwaitFunTerm) String() string {
	return fmt.Sprintf("FrameAwaitFunTerm(env=%v, term=%v, ctx=%v)", f.Env, f.Term, f.Ctx)
}

func (f FrameAwaitFunTerm) isMachineContext() {}

type FrameAwaitFunValue struct {
	Value []Value
	Ctx   MachineContext
}

func (f FrameAwaitFunValue) String() string {
	return fmt.Sprintf("FrameAwaitFunValue(value=%v, ctx=%v)", f.Value, f.Ctx)
}

func (f FrameAwaitFunValue) isMachineContext() {}

type FrameForce struct {
	Ctx MachineContext
}

func (f FrameForce) String() string {
	return fmt.Sprintf("FrameForce(ctx=%v)", f.Ctx)
}

func (f FrameForce) isMachineContext() {}

type FrameConstr struct {
	Env            Env
	Tag            uint64
	Fields         []syn.Term[syn.Eval]
	ResolvedFields []Value
	Ctx            MachineContext
}

func (f FrameConstr) String() string {
	return fmt.Sprintf("FrameConstr(env=%v, tag=%v, fields=%v, resolvedFields=%v, ctx=%v)", f.Env, f.Tag, f.Fields, f.ResolvedFields, f.Ctx)
}

func (f FrameConstr) isMachineContext() {}

type FrameCases struct {
	Env      Env
	Branches []syn.Term[syn.Eval]
	Ctx      MachineContext
}

func (f FrameCases) String() string {
	return fmt.Sprintf("FrameCases(env=%v, branches=%v, ctx=%v)", f.Env, f.Branches, f.Ctx)
}

func (f FrameCases) isMachineContext() {}

type NoFrame struct{}

func (f NoFrame) String() string {
	return "NoFrame"
}

func (f NoFrame) isMachineContext() {}
