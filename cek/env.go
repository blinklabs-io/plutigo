package cek

import "github.com/blinklabs-io/plutigo/syn"

type Env[T syn.Eval] struct {
	data  Value[T]
	next  *Env[T]
	skip4 *Env[T]
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
	case 5:
		env = advanceEnv4(env)
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 6:
		env = advanceEnv4(env)
		if env == nil {
			return zero, false
		}
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	case 7:
		env = advanceEnv4(env)
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
	case 8:
		env = advanceEnv4(env)
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
		env = env.next
		if env == nil {
			return zero, false
		}
		return env.data, true
	}

	current := env
	remaining := idx - 1
	for remaining >= 4 {
		current = advanceEnv4(current)
		if current == nil {
			return zero, false
		}
		remaining -= 4
	}
	for remaining > 0 {
		current = current.next
		if current == nil {
			return zero, false
		}
		remaining--
	}

	return current.data, true
}

func advanceEnv4[T syn.Eval](env *Env[T]) *Env[T] {
	if env == nil {
		return nil
	}
	if env.skip4 != nil {
		return env.skip4
	}
	for range 4 {
		env = env.next
		if env == nil {
			return nil
		}
	}
	return env
}

func (e *Env[T]) Extend(data Value[T]) *Env[T] {
	var skip4 *Env[T]
	if e != nil {
		skip := e.next
		if skip != nil {
			skip = skip.next
			if skip != nil {
				skip4 = skip.next
			}
		}
	}
	return &Env[T]{
		data:  data,
		next:  e,
		skip4: skip4,
	}
}

func (e *Env[T]) Lookup(name int) (Value[T], bool) {
	return lookupEnv(e, name)
}
