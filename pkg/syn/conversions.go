package syn

import (
	"errors"
	"fmt"
)

func NameToNamedDeBruijn(p *Program[Name]) (*Program[NamedDeBruijn], error) {
	converter := newConverter()

	t, err := nameToIndex(converter, p.Term, func(s string, d DeBruijn) NamedDeBruijn {
		return NamedDeBruijn{
			Text:  s,
			Index: d,
		}
	})
	if err != nil {
		return nil, err
	}

	program := &Program[NamedDeBruijn]{
		Version: p.Version,
		Term:    t,
	}

	return program, nil
}

func NameToDeBruijn(p *Program[Name]) (*Program[DeBruijn], error) {
	converter := newConverter()

	t, err := nameToIndex(converter, p.Term, func(s string, d DeBruijn) DeBruijn {
		return DeBruijn(d)
	})
	if err != nil {
		return nil, err
	}

	program := &Program[DeBruijn]{
		Version: p.Version,
		Term:    t,
	}

	return program, nil
}

type converter struct {
	currentLevel  uint
	currentUnique Unique
	levels        []biMap
}

func newConverter() *converter {
	return &converter{
		currentLevel:  0,
		currentUnique: 0,
		levels: []biMap{
			{
				left:  make(map[Unique]uint),
				right: make(map[uint]Unique),
			},
		},
	}
}

func nameToIndex[T any](c *converter, term Term[Name], converter func(string, DeBruijn) T) (Term[T], error) {
	var converted Term[T]

	switch t := term.(type) {
	case *Var[Name]:
		index, err := c.getIndex(&t.Name)
		if err != nil {
			return nil, err
		}

		converted = &Var[T]{
			Name: converter(t.Name.Text, index),
		}
	case *Delay[Name]:
		inner, err := nameToIndex(c, t.Term, converter)
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

		body, err := nameToIndex(c, t.Body, converter)
		if err != nil {
			return nil, err
		}

		c.endScope()

		c.removeUnique(t.ParameterName.Unique)

		converted = &Lambda[T]{
			ParameterName: converter(t.ParameterName.Text, index),
			Body:          body,
		}
	case *Apply[Name]:
		f, err := nameToIndex(c, t.Function, converter)
		if err != nil {
			return nil, err
		}

		arg, err := nameToIndex(c, t.Argument, converter)
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
		inner, err := nameToIndex(c, t.Term, converter)
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
		var fields []Term[T]

		for _, f := range t.Fields {
			item, err := nameToIndex(c, f, converter)
			if err != nil {
				return nil, err
			}

			fields = append(fields, item)
		}

		converted = &Constr[T]{
			Tag:    t.Tag,
			Fields: fields,
		}
	case *Case[Name]:
		var branches []Term[T]

		for _, b := range t.Branches {
			item, err := nameToIndex(c, b, converter)
			if err != nil {
				return nil, err
			}

			branches = append(branches, item)

		}

		constr, err := nameToIndex(c, t.Constr, converter)
		if err != nil {
			return nil, err
		}

		converted = &Case[T]{
			Constr:   constr,
			Branches: branches,
		}
	default:
		fmt.Printf("%#v", t)
		panic("HOW THE FUCK")
	}

	return converted, nil
}

// getIndex finds the DeBruijn index for a given name
func (c *converter) getIndex(name *Name) (DeBruijn, error) {
	for i := len(c.levels) - 1; i >= 0; i-- {
		scope := &c.levels[i]
		if foundLevel, ok := scope.getByUnique(name.Unique); ok {
			index := c.currentLevel - foundLevel
			return DeBruijn(index), nil
		}
	}

	return 0, errors.New("FreeUnique")
}

//nolint:unused
// getUnique finds the Unique identifier for a given DeBruijn index
func (c *converter) getUnique(index DeBruijn) (Unique, error) {
	for i := len(c.levels) - 1; i >= 0; i-- {
		indexVal := uint(index)
		if c.currentLevel < indexVal {
			return 0, errors.New("FreeIndex")
		}

		level := c.currentLevel - indexVal

		if unique, ok := c.levels[i].getByLevel(level); ok {
			return unique, nil
		}
	}

	return 0, errors.New("FreeIndex")
}

// declareUnique adds a unique identifier to the current scope
func (c *converter) declareUnique(unique Unique) {
	scope := &c.levels[c.currentLevel]
	scope.insert(unique, c.currentLevel)
}

// removeUnique removes a unique identifier from the current scope
func (c *converter) removeUnique(unique Unique) {
	scope := &c.levels[c.currentLevel]
	scope.remove(unique, c.currentLevel)
}

//nolint:unused
// declareBinder declares a new binder in the current scope
func (c *converter) declareBinder() {
	scope := &c.levels[c.currentLevel]
	scope.insert(c.currentUnique, c.currentLevel)
	c.currentUnique++
}

// startScope begins a new variable scope
func (c *converter) startScope() {
	c.currentLevel++
	c.levels = append(c.levels, biMap{
		left:  make(map[Unique]uint),
		right: make(map[uint]Unique),
	})
}

// endScope ends the current variable scope
func (c *converter) endScope() {
	c.currentLevel--
	c.levels = c.levels[:len(c.levels)-1]
}
