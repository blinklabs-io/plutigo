package cek

import (
	"iter"

	"github.com/blinklabs-io/plutigo/syn"
)

type argHolder[T syn.Eval] [6]Value[T]

func newArgHolder[T syn.Eval]() argHolder[T] {
	return argHolder[T]([6]Value[T]{})
}

type BuiltinArgs[T syn.Eval] struct {
	data Value[T]
	next *BuiltinArgs[T]
}

func (b *BuiltinArgs[T]) Iter() iter.Seq[Value[T]] {
	return func(yield func(Value[T]) bool) {
		var visit func(*BuiltinArgs[T]) bool
		visit = func(args *BuiltinArgs[T]) bool {
			if args == nil {
				return true
			}
			if !visit(args.next) {
				return false
			}
			return yield(args.data)
		}

		visit(b)
	}
}

func (b *BuiltinArgs[T]) Extend(data Value[T]) *BuiltinArgs[T] {
	return &BuiltinArgs[T]{
		data: data,
		next: b,
	}
}

// Extract copies the oldest count arguments into holder in call order.
// Callers must ensure count does not exceed the current arg chain length.
func (b *BuiltinArgs[T]) Extract(holder *argHolder[T], count uint) {
	switch count {
	case 6:
		holder[5] = b.data
		b = b.next
		fallthrough
	case 5:
		holder[4] = b.data
		b = b.next
		fallthrough
	case 4:
		holder[3] = b.data
		b = b.next
		fallthrough
	case 3:
		holder[2] = b.data
		b = b.next
		fallthrough
	case 2:
		holder[1] = b.data
		b = b.next
		fallthrough
	case 1:
		holder[0] = b.data
	}
}
