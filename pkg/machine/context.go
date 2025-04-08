package machine

import "github.com/blinklabs-io/plutigo/pkg/syn"

type MachineContext interface{}

type FrameAwaitArg struct {
	value Value
	ctx   MachineContext
}

type FrameAwaitFunTerm struct {
	env  Env
	term syn.Term[syn.NamedDeBruijn]
	ctx  MachineContext
}

type FrameAwaitFunValue struct {
	value Value
	ctx   MachineContext
}

type FrameForce struct{ ctx MachineContext }

type FrameConstr struct {
	env            Env
	tag            uint64
	fields         []syn.Term[syn.NamedDeBruijn]
	resolvedFields []Value
	ctx            MachineContext
}

type FrameCases struct {
	env      Env
	branches []syn.Term[syn.NamedDeBruijn]
	ctx      MachineContext
}

type NoFrame struct{}
