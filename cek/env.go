package cek

import (
	"math/bits"

	"github.com/blinklabs-io/plutigo/syn"
)

// envIndexLevels covers every environment depth accepted by the parser and
// FLAT decoder (both cap nesting at 100,000). Entry n is the ancestor at
// distance 2^(n+1)-1. Deeper chains built through the exported Env API compose
// multiple maximum-distance jumps.
const (
	envIndexLevels    = 17
	envMaxIndexedJump = (1 << envIndexLevels) - 1
)

type envIndex[T syn.Eval] [envIndexLevels]*Env[T]

type Env[T syn.Eval] struct {
	data  Value[T]
	next  *Env[T]
	index *envIndex[T]
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
		if env.index != nil {
			ancestor := env.index[1]
			if ancestor != nil {
				ancestor = ancestor.next
				if ancestor != nil {
					return ancestor.data, true
				}
			}
		}
	case 6:
		if env.index != nil {
			ancestor := env.index[1]
			if ancestor != nil {
				ancestor = ancestor.next
			}
			if ancestor != nil {
				ancestor = ancestor.next
				if ancestor != nil {
					return ancestor.data, true
				}
			}
		}
	case 7:
		if env.index != nil {
			ancestor := env.index[1]
			for range 3 {
				if ancestor != nil {
					ancestor = ancestor.next
				}
			}
			if ancestor != nil {
				return ancestor.data, true
			}
		}
	case 8:
		if env.index != nil {
			if ancestor := env.index[2]; ancestor != nil {
				return ancestor.data, true
			}
		}
	}
	if idx&(idx-1) == 0 {
		level := bits.TrailingZeros(uint(idx)) - 1
		if level < envIndexLevels && env.index != nil {
			if ancestor := env.index[level]; ancestor != nil {
				return ancestor.data, true
			}
		}
	}

	env = envAncestor(env, idx-1)
	if env == nil {
		return zero, false
	}
	return env.data, true
}

func envAncestor[T syn.Eval](env *Env[T], distance int) *Env[T] {
	for distance != 0 {
		level := bits.Len(uint(distance+1)) - 2
		if level >= envIndexLevels {
			level = envIndexLevels - 1
		}
		jump := min((1<<(level+1))-1, envMaxIndexedJump)
		var ancestor *Env[T]
		if env.index != nil {
			ancestor = env.index[level]
		}
		if ancestor == nil {
			// Keep package-local manually constructed Env values working.
			// Environments made through Extend or Machine.extendEnv always
			// have a complete index and take the direct path above.
			ancestor = env
			for range jump {
				ancestor = ancestor.next
				if ancestor == nil {
					return nil
				}
			}
		}
		env = ancestor
		distance -= jump
	}
	return env
}

func initEnvIndex[T syn.Eval](env, parent *Env[T]) {
	env.next = parent
	if env.index == nil {
		env.index = &envIndex[T]{}
	}
	env.index[0] = parent
	for level := 1; level < envIndexLevels; level++ {
		previous := env.index[level-1]
		if previous == nil || previous.index == nil {
			break
		}
		previous = previous.index[level-1]
		if previous == nil {
			break
		}
		env.index[level] = previous.next
	}
}

func (e *Env[T]) Extend(data Value[T]) *Env[T] {
	env := &Env[T]{
		data: data,
	}
	initEnvIndex(env, e)
	return env
}

func (e *Env[T]) Lookup(name int) (Value[T], bool) {
	return lookupEnv(e, name)
}
