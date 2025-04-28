package syn

import (
	"errors"
	"fmt"
)

func (p *Program[Name]) Eval() (*Program[Eval], error) {
	var converter converter

	t, err := converter.nameToEval(p.Term)
	if err != nil {
		return nil, err
	}

	program := &Program[Eval]{
		Version: p.Version,
		Term:    t,
	}

	return program, nil
}

func (p *Program[Name]) NamedDeBruijn() (*Program[NamedDeBruijn], error) {
	var converter converter

	t, err := converter.nameToNamedDebruijn(p.Term)
	if err != nil {
		return nil, err
	}

	program := &Program[NamedDeBruijn]{
		Version: p.Version,
		Term:    t,
	}

	return program, nil
}

func (p *Program[Name]) DeBruijn() (*Program[DeBruijn], error) {
	panic("hi")
}

type converter struct {
	currentLevel  uint
	currentUnique Unique
	levels        []biMap
}

func (c *converter) nameToNamedDebruijn(term Term[Name]) (Term[NamedDeBruijn], error) {
	var converted Term[NamedDeBruijn]

	switch t := term.(type) {
	case *Var[Name]:
		index, err := c.getIndex(&t.Name)
		if err != nil {
			return nil, err
		}

		converted = &Var[NamedDeBruijn]{
			Name: NamedDeBruijn{
				Text:  t.Name.Text,
				Index: index,
			},
		}
	case *Delay[Name]:
		inner, err := c.nameToNamedDebruijn(t.Term)
		if err != nil {
			return nil, err
		}

		converted = &Delay[NamedDeBruijn]{
			Term: inner,
		}
	case *Lambda[Name]:
		c.declareUnique(t.ParameterName.Unique)

		index, err := c.getIndex(&t.ParameterName)
		if err != nil {
			return nil, err
		}

		c.startScope()

		body, err := c.nameToNamedDebruijn(t.Body)
		if err != nil {
			return nil, err
		}

		c.endScope()

		c.removeUnique(t.ParameterName.Unique)

		converted = &Lambda[NamedDeBruijn]{
			ParameterName: NamedDeBruijn{
				Text:  t.ParameterName.Text,
				Index: index,
			},
			Body: body,
		}
	case *Apply[Name]:

		f, err := c.nameToNamedDebruijn(t.Function)
		if err != nil {
			return nil, err
		}

		arg, err := c.nameToNamedDebruijn(t.Argument)
		if err != nil {
			return nil, err
		}

		converted = &Apply[NamedDeBruijn]{
			Function: f,
			Argument: arg,
		}
	case *Constant:
		converted = t
	case *Force[Name]:
		inner, err := c.nameToNamedDebruijn(t.Term)
		if err != nil {
			return nil, err
		}

		converted = &Force[NamedDeBruijn]{
			Term: inner,
		}

	case *Error:
		converted = t

	case *Builtin:
		converted = t

	case *Constr[Name]:
		var fields []Term[NamedDeBruijn]

		for _, f := range *t.Fields {
			item, err := c.nameToNamedDebruijn(f)
			if err != nil {
				return nil, err
			}

			fields = append(fields, item)
		}

		converted = &Constr[NamedDeBruijn]{
			Tag:    t.Tag,
			Fields: &fields,
		}
	case *Case[Name]:
		var branches []Term[NamedDeBruijn]

		for _, b := range *t.Branches {
			item, err := c.nameToNamedDebruijn(b)
			if err != nil {
				return nil, err
			}

			branches = append(branches, item)

		}

		constr, err := c.nameToNamedDebruijn(t.Constr)
		if err != nil {
			return nil, err
		}

		converted = &Case[NamedDeBruijn]{
			Constr:   constr,
			Branches: &branches,
		}
	default:
		fmt.Printf("%#v", t)
		panic("HOW THE FUCK")
	}

	return converted, nil
}

func (c *converter) nameToEval(term Term[Name]) (Term[Eval], error) {
	var converted Term[Eval]

	switch t := term.(type) {
	case *Var[Name]:
		index, err := c.getIndex(&t.Name)
		if err != nil {
			return nil, err
		}

		converted = &Var[Eval]{
			Name: NamedDeBruijn{
				Text:  t.Name.Text,
				Index: index,
			},
		}
	case *Delay[Name]:
		inner, err := c.nameToEval(t.Term)
		if err != nil {
			return nil, err
		}

		converted = &Delay[Eval]{
			Term: inner,
		}
	case *Lambda[Name]:
		c.declareUnique(t.ParameterName.Unique)

		index, err := c.getIndex(&t.ParameterName)
		if err != nil {
			return nil, err
		}

		c.startScope()

		body, err := c.nameToEval(t.Body)
		if err != nil {
			return nil, err
		}

		c.endScope()

		c.removeUnique(t.ParameterName.Unique)

		converted = &Lambda[Eval]{
			ParameterName: NamedDeBruijn{
				Text:  t.ParameterName.Text,
				Index: index,
			},
			Body: body,
		}
	case *Apply[Name]:

		f, err := c.nameToEval(t.Function)
		if err != nil {
			return nil, err
		}

		arg, err := c.nameToEval(t.Argument)
		if err != nil {
			return nil, err
		}

		converted = &Apply[Eval]{
			Function: f,
			Argument: arg,
		}
	case *Constant:
		converted = t
	case *Force[Name]:
		inner, err := c.nameToEval(t.Term)
		if err != nil {
			return nil, err
		}

		converted = &Force[Eval]{
			Term: inner,
		}

	case *Error:
		converted = t

	case *Builtin:
		converted = t

	case *Constr[Name]:
		var fields []Term[Eval]

		for _, f := range *t.Fields {
			item, err := c.nameToEval(f)
			if err != nil {
				return nil, err
			}

			fields = append(fields, item)
		}

		converted = &Constr[Eval]{
			Tag:    t.Tag,
			Fields: &fields,
		}
	case *Case[Name]:
		var branches []Term[Eval]

		for _, b := range *t.Branches {
			item, err := c.nameToEval(b)
			if err != nil {
				return nil, err
			}

			branches = append(branches, item)

		}

		constr, err := c.nameToEval(t.Constr)
		if err != nil {
			return nil, err
		}

		converted = &Case[Eval]{
			Constr:   constr,
			Branches: &branches,
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
