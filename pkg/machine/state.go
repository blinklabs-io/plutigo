package machine

import "github.com/blinklabs-io/plutigo/pkg/syn"

type MachineState interface {
	isDone() bool
}

type Return struct {
	ctx   MachineContext
	value Value
}

func (r Return) isDone() bool {
	return false
}

type Compute struct {
	ctx  MachineContext
	env  Env
	term syn.Term[syn.NamedDeBruijn]
}

func (c Compute) isDone() bool {
	return false
}

type Done struct {
	term syn.Term[syn.NamedDeBruijn]
}

func (d Done) isDone() bool {
	return true
}
