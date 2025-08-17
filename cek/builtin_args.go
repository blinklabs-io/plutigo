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
		temp := b
		acc := []Value[T]{}

		for {
			if temp == nil {
				break
			}

			acc = append(acc, temp.data)

			temp = temp.next
		}

		for i := len(acc) - 1; i >= 0; i-- {
			if !yield(acc[i]) {
				return
			}
		}

	}
}

func (b *BuiltinArgs[T]) Extend(data Value[T]) *BuiltinArgs[T] {
	return &BuiltinArgs[T]{
		data: data,
		next: b,
	}
}

func (b *BuiltinArgs[T]) Extract(holder *argHolder[T], count uint) {
	temp := b

	for {
		if temp == nil {
			break
		}

		holder[count-1] = temp.data

		temp = temp.next
		count--
	}
}
