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
	if name <= 0 {
		return nil, false
	}

	switch name {
	case 1:
		if e != nil {
			return e.data, true
		}
		return nil, false
	case 2:
		if e != nil && e.next != nil {
			return e.next.data, true
		}
		return nil, false
	case 3:
		if e != nil && e.next != nil && e.next.next != nil {
			return e.next.next.data, true
		}
		return nil, false
	case 4:
		if e != nil && e.next != nil && e.next.next != nil && e.next.next.next != nil {
			return e.next.next.next.data, true
		}
		return nil, false
	}

	current := e
	for remaining := name; current != nil; remaining-- {
		if remaining == 1 {
			return current.data, true
		}
		current = current.next
	}

	return nil, false
}
