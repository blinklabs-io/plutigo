package cek

import "github.com/blinklabs-io/plutigo/syn"

type Env[T syn.Eval] struct {
	data Value[T]
	next *Env[T]
}

func lookupEnv[T syn.Eval](env *Env[T], idx int) (Value[T], bool) {
	var zero Value[T]
	if idx <= 0 {
		return zero, false
	}
	if env == nil {
		return zero, false
	}
	if idx == 1 {
		return env.data, true
	}

	current := env
	for remaining := idx; current != nil; remaining-- {
		if remaining == 1 {
			return current.data, true
		}
		current = current.next
	}

	return zero, false
}

func (e *Env[T]) Extend(data Value[T]) *Env[T] {
	return &Env[T]{
		data: data,
		next: e,
	}
}

func (e *Env[T]) Lookup(name int) (Value[T], bool) {
	return lookupEnv(e, name)
}
