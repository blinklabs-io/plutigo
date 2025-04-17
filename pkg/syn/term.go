package syn

import (
	"errors"
	"fmt"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
)

type Term[T any] interface {
	fmt.Stringer
	isTerm()
}

type Name struct {
	Text   string
	Unique Unique
}

func (n Name) BinderEncode(e any) error {
	return nil
}

func (n Name) BinderDecode(d any) (*Binder, error) {
	return nil, errors.New("fill in this method")
}

func (n Name) TextName() string {
	return n.Text
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

func (n DeBruijn) BinderEncode(e any) error {
	return nil
}

func (n DeBruijn) BinderDecode(d any) (*Binder, error) {
	return nil, errors.New("fill in this method")
}

func (n DeBruijn) TextName() string {
	return fmt.Sprintf("i_%d", n)
}

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

func (Var[T]) isTerm() {}
func (v Var[T]) String() string {
	return "TODO"
}

// (delay x)
type Delay[T Binder] struct {
	Term[T]
}

func (Delay[T]) isTerm() {}
func (v Delay[T]) String() string {
	return "TODO"
}

// (force x)
type Force[T Binder] struct {
	Term[T]
}

func (Force[T]) isTerm() {}
func (v Force[T]) String() string {
	return "TODO"
}

// (lam x x)
type Lambda[T Binder] struct {
	ParameterName T
	Body          Term[T]
}

func (Lambda[T]) isTerm() {}
func (v Lambda[T]) String() string {
	return "TODO"
}

// [ (lam x x) (con integer 1) ]
type Apply[T Binder] struct {
	Function Term[T]
	Argument Term[T]
}

func (Apply[T]) isTerm() {}
func (v Apply[T]) String() string {
	return "TODO"
}

// (builtin addInteger)
type Builtin struct {
	builtin.DefaultFunction
}

func (Builtin) isTerm() {}
func (v Builtin) String() string {
	return "TODO"
}

// (constr 0 (con integer 1) (con string "1234"))
type Constr[T Binder] struct {
	Tag    uint64
	Fields *[]Term[T]
}

func (Constr[T]) isTerm() {}
func (v Constr[T]) String() string {
	return "TODO"
}

// (case (constr 0) (constr 1 (con integer 1)))
type Case[T Binder] struct {
	Constr   Term[T]
	Branches []Term[T]
}

func (Case[T]) isTerm() {}
func (v Case[T]) String() string {
	return "TODO"
}

// (error )
type Error struct{}

func (Error) isTerm() {}
func (v Error) String() string {
	return "TODO"
}

// (con integer 1)
type Constant struct {
	IConstant
}

func (Constant) isTerm() {}
func (v Constant) String() string {
	return "TODO"
}
