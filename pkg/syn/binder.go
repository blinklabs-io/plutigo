package syn

import (
	"errors"
	"fmt"
)

type Binder interface {
	// TODO: e should be a encoder
	BinderEncode(e any) error
	// TODO: d should be a decoder
	BinderDecode(d any) (*Binder, error)
	// TODO: maybe use String interface
	TextName() string

	fmt.Stringer
}

type Eval interface {
	Binder
	LookupIndex() uint64
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

func (n Name) String() string {
	return fmt.Sprintf("Name: %s %v", n.Text, n.Unique)
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
	return fmt.Sprintf("%s_%d", n.Text, n.Index)
}

func (n NamedDeBruijn) String() string {
	return fmt.Sprintf("NamedDeBruijn: %s %v", n.Text, n.Index)
}

func (n NamedDeBruijn) LookupIndex() uint64 {
	return uint64(n.Index)
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

func (n DeBruijn) LookupIndex() uint64 {
	return uint64(n)
}
