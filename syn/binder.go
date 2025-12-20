package syn

import (
	"fmt"
	"math"
)

type Binder interface {
	VarEncode(e *encoder) error
	VarDecode(d *decoder) (Binder, error)

	ParameterEncode(e *encoder) error
	ParameterDecode(d *decoder) (Binder, error)

	TextName() string

	fmt.Stringer
}

type Eval interface {
	Binder
	LookupIndex() int
}

type Name struct {
	Text   string
	Unique Unique
}

func (n Name) VarEncode(e *encoder) error {
	err := e.utf8(n.Text)
	if err != nil {
		return err
	}

	if n.Unique > math.MaxUint {
		return fmt.Errorf("unique value out of range: %d", n.Unique)
	}

	e.word(uint(n.Unique))

	return nil
}

func (n Name) VarDecode(d *decoder) (Binder, error) {
	text, err := d.utf8()
	if err != nil {
		return nil, err
	}

	i, err := d.word()
	if err != nil {
		return nil, err
	}

	name := Name{
		Text:   text,
		Unique: Unique(i),
	}

	return name, nil
}

func (n Name) ParameterEncode(e *encoder) error {
	return n.VarEncode(e)
}

func (n Name) ParameterDecode(d *decoder) (Binder, error) {
	return n.VarDecode(d)
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

func (n NamedDeBruijn) VarEncode(e *encoder) error {
	err := e.utf8(n.Text)
	if err != nil {
		return err
	}

	if n.Index < 0 {
		return fmt.Errorf("negative DeBruijn index: %d", n.Index)
	}

	e.word(uint(n.Index)) //nolint:gosec

	return nil
}

func (n NamedDeBruijn) VarDecode(d *decoder) (Binder, error) {
	text, err := d.utf8()
	if err != nil {
		return nil, err
	}

	i, err := d.word()
	if err != nil {
		return nil, err
	}

	if i > math.MaxInt {
		return nil, fmt.Errorf("DeBruijn index too large: %d", i)
	}

	nd := NamedDeBruijn{
		Text:  text,
		Index: DeBruijn(i),
	}

	return nd, nil
}

func (n NamedDeBruijn) ParameterEncode(e *encoder) error {
	return n.VarEncode(e)
}

func (n NamedDeBruijn) ParameterDecode(d *decoder) (Binder, error) {
	return n.VarDecode(d)
}

func (n NamedDeBruijn) TextName() string {
	return fmt.Sprintf("%s_%d", n.Text, n.Index)
}

func (n NamedDeBruijn) String() string {
	return fmt.Sprintf("NamedDeBruijn: %s %d", n.Text, n.Index)
}

func (n NamedDeBruijn) LookupIndex() int {
	return int(n.Index)
}

type Unique uint64

// An index into the Machine's environment
// which powers var lookups
type DeBruijn int

func (n DeBruijn) VarEncode(e *encoder) error {
	if n < 0 {
		return fmt.Errorf("negative DeBruijn index: %d", n)
	}
	e.word(uint(n)) //nolint:gosec

	return nil
}

func (n DeBruijn) VarDecode(d *decoder) (Binder, error) {
	i, err := d.word()
	if err != nil {
		return nil, err
	}

	if i > math.MaxInt {
		return nil, fmt.Errorf("DeBruijn index too large: %d", i)
	}

	return DeBruijn(i), nil
}

func (n DeBruijn) ParameterEncode(e *encoder) error {
	// this is correct, in DeBruijn we never encode the param
	return nil
}

func (n DeBruijn) ParameterDecode(d *decoder) (Binder, error) {
	// it's actually always zero, trust, see above
	return DeBruijn(0), nil
}

func (n DeBruijn) TextName() string {
	return fmt.Sprintf("i_%d", n)
}

func (n DeBruijn) String() string {
	return fmt.Sprintf("DeBruijn: %d", n)
}

func (n DeBruijn) LookupIndex() int {
	return int(n)
}
