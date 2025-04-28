package machine

import (
	"errors"
	"math/big"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/syn"
)

func (m *Machine) evalBuiltinApp(b Builtin) (Value, error) {
	// Budgeting
	var evalValue Value

	switch b.Func {
	case builtin.AddInteger:
		arg1, err := unwrapInteger(b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger(b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		var newInt big.Int

		newInt.Add(arg1, arg2)

		evalValue = Constant{
			Constant: syn.Integer{
				Inner: &newInt,
			},
		}

	case builtin.SubtractInteger:
		arg1, err := unwrapInteger(b.Args[0])
		if err != nil {
			return nil, err
		}

		arg2, err := unwrapInteger(b.Args[1])
		if err != nil {
			return nil, err
		}

		// TODO: The budgeting

		var newInt big.Int

		newInt.Sub(arg1, arg2)

		evalValue = Constant{
			Constant: syn.Integer{
				Inner: &newInt,
			},
		}

	}

	return evalValue, nil
}

func unwrapInteger(value Value) (*big.Int, error) {

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
