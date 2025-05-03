package machine

import "github.com/blinklabs-io/plutigo/pkg/syn"

type Env[T syn.Eval] []Value[T]

func lookup[T syn.Eval](e *Env[T], name uint) (*Value[T], bool) {
	idx := len(*e) - int(name)

	if !indexExists(*e, idx) {
		return nil, false
	}

	return &(*e)[idx], true
}
