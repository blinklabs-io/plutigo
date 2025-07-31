package cek

import "github.com/blinklabs-io/plutigo/syn"

type Env[T syn.Eval] struct {
	data Value[T]
	next *Env[T]
}

func (e *Env[T]) Extend(data Value[T]) *Env[T] {
	return &Env[T]{
		data: data,
		next: e,
	}
}

func (e *Env[T]) Lookup(name int) (Value[T], bool) {
	var temp *Env[T] = e

	if name <= 0 {
		return nil, false
	}

	for range name - 1 {
		if temp == nil {
			return nil, false
		}

		temp = temp.next
	}

	if temp == nil {
		return nil, false
	} else {
		return temp.data, true
	}
}
