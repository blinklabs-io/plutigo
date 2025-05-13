package cek

import "github.com/blinklabs-io/plutigo/pkg/syn"

type Env[T syn.Eval] []Value[T]

func (e *Env[T]) lookup(name uint) (Value[T], bool) {
	idx := len(*e) - int(name)

	if !indexExists(*e, idx) {
		return nil, false
	}

	return (*e)[idx], true
}
