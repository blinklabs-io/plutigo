package syn

import (
	"fmt"
	"math/big"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
)

type termInterner struct {
	interned      map[string]Unique
	uniqueCounter Unique
}

func newTermInterner() *termInterner {
	return &termInterner{
		interned:      make(map[string]Unique),
		uniqueCounter: Unique(0),
	}
}

func (b *termInterner) intern(currentTerm Term[Name]) {
	switch term := (currentTerm).(type) {
	case *Var[Name]:
		b.internName(&term.Name)
	case *Delay[Name]:
		b.intern(term.Term)
	case *Force[Name]:
		b.intern(term.Term)
	case *Lambda[Name]:
		b.internName(&term.ParameterName)
		b.intern(term.Body)
	case *Apply[Name]:
		b.intern(term.Function)
		b.intern(term.Argument)
	case *Constr[Name]:
		for _, field := range term.Fields {
			b.intern(field)
		}
	case *Case[Name]:
		b.intern(term.Constr)
		for _, branch := range term.Branches {
			b.intern(branch)
		}
	case *Builtin:
		// No names to intern
	case *Constant:
		// No names to intern
	case *Error:
		// No names to intern
	default:
		// Debug: print the type
		println("intern: unhandled type", fmt.Sprintf("%T", currentTerm))
		return
	}
}

func (b *termInterner) internName(name *Name) {
	if unique, exists := b.interned[name.Text]; exists {
		name.Unique = unique
	} else {
		name.Unique = b.uniqueCounter
		b.interned[name.Text] = b.uniqueCounter
		b.uniqueCounter++
	}
}

func Intern(term Term[Name]) Term[Name] {
	b := newTermInterner()

	b.intern(term)

	return term
}

func NewProgram(version lang.LanguageVersion, term Term[Name]) *Program[Name] {
	return &Program[Name]{
		Version: version,
		Term:    term,
	}
}

// NewName creates a new Name with the provided text and a default Unique value of 0.
func NewName(text string, unique Unique) Name {
	return Name{
		Text:   text,
		Unique: unique,
	}
}

// NewRawName creates a new Name with the provided text and a default Unique value of 0.
func NewRawName(text string) Name {
	return NewName(text, 0)
}

func NewVar(name string, unique Unique) Term[Name] {
	return &Var[Name]{
		Name: NewName(name, unique),
	}
}

func NewRawVar(name string) Term[Name] {
	return &Var[Name]{
		Name: NewRawName(name),
	}
}

func NewApply(function Term[Name], argument Term[Name]) Term[Name] {
	return &Apply[Name]{
		Function: function,
		Argument: argument,
	}
}

func NewLambda(parameterName Name, body Term[Name]) Term[Name] {
	return &Lambda[Name]{
		ParameterName: parameterName,
		Body:          body,
	}
}

func NewDelay(term Term[Name]) Term[Name] {
	return &Delay[Name]{Term: term}
}

func NewForce(term Term[Name]) Term[Name] {
	return &Force[Name]{Term: term}
}

func NewConstant(con IConstant) Term[Name] {
	return &Constant{
		Con: con,
	}
}

func NewSimpleInteger(value int) Term[Name] {
	return NewConstant(&Integer{
		Inner: big.NewInt(int64(value)),
	})
}

func NewInteger(value *big.Int) Term[Name] {
	return NewConstant(&Integer{
		Inner: value,
	})
}

func NewBool(value bool) Term[Name] {
	return NewConstant(&Bool{
		Inner: value,
	})
}

func NewBuiltin(fn builtin.DefaultFunction) Term[Name] {
	return &Builtin{
		DefaultFunction: fn,
	}
}

func AddInteger() Term[Name] {
	return NewBuiltin(builtin.AddInteger)
}

func SubtractInteger() Term[Name] {
	return NewBuiltin(builtin.SubtractInteger)
}

func IfThenElse() Term[Name] {
	return NewBuiltin(builtin.IfThenElse)
}
