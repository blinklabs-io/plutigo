package cek

import "github.com/blinklabs-io/plutigo/syn"

type MachineState[T syn.Eval] interface {
	isMachineState()
}

type Return[T syn.Eval] struct {
	Ctx   MachineContext[T]
	Value Value[T]
}

func (r Return[T]) isMachineState() {}

type Compute[T syn.Eval] struct {
	Ctx  MachineContext[T]
	Env  *Env[T]
	Term syn.Term[T]
}

func (c Compute[T]) isMachineState() {}

type Done[T syn.Eval] struct {
	term syn.Term[T]
}

func (d Done[T]) isMachineState() {}
