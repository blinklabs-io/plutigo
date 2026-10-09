// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// converter resolves names to De Bruijn indices in constant time: it records,
// for each Unique bound by an enclosing lambda, the binding depth, so a lookup
// is one map access regardless of nesting.
type converter struct {
	depth  int
	levels map[Unique]int
}

func newConverter() *converter {
	return &converter{levels: make(map[Unique]int)}
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
		shadowed, wasBound := c.bind(t.ParameterName.Unique)

		index, err := c.getIndex(&t.ParameterName)
		if err != nil {
			return nil, err
		}

		c.depth++

		body, err := nameToIndex(c, t.Body, convertName)
		if err != nil {
			return nil, err
		}

		c.depth--

		c.unbind(t.ParameterName.Unique, shadowed, wasBound)

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
	level, ok := c.levels[name.Unique]
	if !ok {
		return 0, errors.New("FreeUnique")
	}

	return DeBruijn(c.depth - level), nil
}

// bind makes unique visible at the current depth and returns the binding it
// shadows, to be passed to unbind.
func (c *converter) bind(unique Unique) (shadowed int, wasBound bool) {
	shadowed, wasBound = c.levels[unique]
	c.levels[unique] = c.depth

	return shadowed, wasBound
}

func (c *converter) unbind(unique Unique, shadowed int, wasBound bool) {
	if wasBound {
		c.levels[unique] = shadowed
	} else {
		delete(c.levels, unique)
	}
}
