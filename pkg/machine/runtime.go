package machine

import (
	"errors"
	"math/big"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

func evalBuiltinApp[T syn.Eval](m *Machine, b Builtin[T]) (Value[T], error) {
	// Budgeting
	var evalValue Value[T]

	switch b.Func {
	case builtin.AddInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		var newInt big.Int

		newInt.Add(arg1, arg2)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}

	case builtin.SubtractInteger:
		arg1, err := unwrapInteger[T](b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger[T](b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		var newInt big.Int

		newInt.Sub(arg1, arg2)

		evalValue = Constant{
			Constant: &syn.Integer{
				Inner: &newInt,
			},
		}

	}

	return evalValue, nil
}

func unwrapInteger[T syn.Eval](value Value[T]) (*big.Int, error) {

	var i *big.Int

	switch v := value.(type) {
	case Constant:
		switch c := v.Constant.(type) {
		case *syn.Integer:
			i = c.Inner
		default:
			return nil, errors.New("Value not an Integer")
		}
	default:
		return nil, errors.New("Value not a Constant")
	}

	return i, nil
}
