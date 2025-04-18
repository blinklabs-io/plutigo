package machine

import "github.com/blinklabs-io/plutigo/pkg/syn"

type MachineState interface {
	isDone() bool
}

type Return struct {
	Ctx   MachineContext
	Value Value
}

func (r Return) isDone() bool {
	return false
}

type Compute struct {
	Ctx  MachineContext
	Env  Env
	Term syn.Term[syn.Eval]
}

func (c Compute) isDone() bool {
	return false
}

type Done struct {
	term syn.Term[syn.Eval]
}

func (d Done) isDone() bool {
	return true
}
