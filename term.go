package plutus

import "errors"

type Term[T Binder] interface{}

type Name struct {
	Text   string
	Unique Unique
}

type NamedDeBruijn struct {
	Text  string
	Index DeBruijn
}

func (n NamedDeBruijn) BinderEncode(e any) error {
	return nil
}

func (n NamedDeBruijn) BinderDecode(d any) (*Binder, error) {
	return nil, errors.New("fill in this method")
}

func (n NamedDeBruijn) TextName() string {
	return n.Text
}

type Unique uint64

type DeBruijn uint64

type Binder interface {
	// TODO: e should be a encoder
	BinderEncode(e any) error
	// TODO: d should be a decoder
	BinderDecode(d any) (*Binder, error)
	// TODO: maybe use String interface
	TextName() string
}

// x
type Var[T Binder] struct {
	Name T
}

// (delay x)
type Delay[T Binder] struct {
	Term[T]
}

// (force x)
type Force[T Binder] struct {
	Term[T]
}

// (lam x x)
type Lambda[T Binder] struct {
	ParameterName T
	Body          Term[T]
}

// [ (lam x x) (con integer 1) ]
type Apply[T Binder] struct {
	Function Term[T]
	Argument Term[T]
}

type Builtin struct {
	DefaultFunction
}

type Error struct{}
