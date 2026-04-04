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
	switch idx {
	case 1:
		return env.data, true
	case 2:
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 3:
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 4:
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	}

	current := env.next
	remaining := idx - 1
	for remaining > 2 && current != nil {
		next := current.next
		if next == nil {
			current = nil
			break
		}
		current = next.next
		remaining -= 2
	}
	if current == nil {
		return zero, false
	}
	if remaining > 1 {
		current = current.next
		if current == nil {
			return zero, false
		}
	}

	return current.data, true
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
