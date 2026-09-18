package syn

import (
	"errors"
	"fmt"
)

func NameToNamedDeBruijn(p *Program[Name]) (*Program[NamedDeBruijn], error) {
	return convertNameProgram(p, func(s string, d DeBruijn) NamedDeBruijn {
		return NamedDeBruijn{Text: s, Index: d}
	})
}

func NameToDeBruijn(p *Program[Name]) (*Program[DeBruijn], error) {
	return convertNameProgram(p, func(_ string, d DeBruijn) DeBruijn {
		return d
	})
}

func convertNameProgram[T any](
	p *Program[Name],
	convertName func(string, DeBruijn) T,
) (*Program[T], error) {
	converter := newConverter()
	t, err := nameToIndex(converter, p.Term, convertName)
	if err != nil {
		return nil, err
	}

	return &Program[T]{
		Version: p.Version,
		Term:    t,
	}, nil
}

type converter struct {
	scopes []map[Unique]struct{}
}

func newConverter() *converter {
	return &converter{
		scopes: []map[Unique]struct{}{
			make(map[Unique]struct{}),
		},
	}
}

func nameToIndex[T any](
	c *converter,
	term Term[Name],
	convertName func(string, DeBruijn) T,
) (Term[T], error) {
	var converted Term[T]

	switch t := term.(type) {
	case *Var[Name]:
		index, err := c.getIndex(&t.Name)
		if err != nil {
			return nil, err
		}

		converted = &Var[T]{
			Name: convertName(t.Name.Text, index),
		}
	case *Delay[Name]:
		inner, err := nameToIndex(c, t.Term, convertName)
		if err != nil {
			return nil, err
		}

		converted = &Delay[T]{
			Term: inner,
		}
	case *Lambda[Name]:
		c.declareUnique(t.ParameterName.Unique)

		index, err := c.getIndex(&t.ParameterName)
		if err != nil {
			return nil, err
		}

		c.startScope()

		body, err := nameToIndex(c, t.Body, convertName)
		if err != nil {
			return nil, err
		}

		c.endScope()

		c.removeUnique(t.ParameterName.Unique)

		converted = &Lambda[T]{
			ParameterName: convertName(t.ParameterName.Text, index),
			Body:          body,
		}
	case *Apply[Name]:
		f, err := nameToIndex(c, t.Function, convertName)
		if err != nil {
			return nil, err
		}

		arg, err := nameToIndex(c, t.Argument, convertName)
		if err != nil {
			return nil, err
		}

		converted = &Apply[T]{
			Function: f,
			Argument: arg,
		}
	case *Constant:
		converted = t
	case *Force[Name]:
		inner, err := nameToIndex(c, t.Term, convertName)
		if err != nil {
			return nil, err
		}

		converted = &Force[T]{
			Term: inner,
		}
	case *Error:
		converted = t
	case *Builtin:
		converted = t
	case *Constr[Name]:
		fields := make([]Term[T], len(t.Fields))

		for i, f := range t.Fields {
			item, err := nameToIndex(c, f, convertName)
			if err != nil {
				return nil, err
			}

			fields[i] = item
		}

		converted = &Constr[T]{
			Tag:    t.Tag,
			Fields: fields,
		}
	case *Case[Name]:
		branches := make([]Term[T], len(t.Branches))

		for i, b := range t.Branches {
			item, err := nameToIndex(c, b, convertName)
			if err != nil {
				return nil, err
			}

			branches[i] = item
		}

		constr, err := nameToIndex(c, t.Constr, convertName)
		if err != nil {
			return nil, err
		}

		converted = &Case[T]{
			Constr:   constr,
			Branches: branches,
		}
	default:
		panic(fmt.Sprintf("unknown Term type: %T", term))
	}

	return converted, nil
}

func (c *converter) getIndex(name *Name) (DeBruijn, error) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if _, ok := c.scopes[i][name.Unique]; ok {
			return DeBruijn(len(c.scopes) - 1 - i), nil
		}
	}

	return 0, errors.New("FreeUnique")
}

func (c *converter) declareUnique(unique Unique) {
	c.scopes[len(c.scopes)-1][unique] = struct{}{}
}

func (c *converter) removeUnique(unique Unique) {
	delete(c.scopes[len(c.scopes)-1], unique)
}

func (c *converter) startScope() {
	c.scopes = append(c.scopes, make(map[Unique]struct{}))
}

func (c *converter) endScope() {
	c.scopes = c.scopes[:len(c.scopes)-1]
}
