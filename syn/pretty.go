package syn

import (
	"fmt"
	"strings"

	"github.com/blinklabs-io/plutigo/data"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

const (
	// defaultPrettyMaxDepth is the nesting depth the decoders admit.
	defaultPrettyMaxDepth = maxFlatDecodeDepth
	// defaultPrettyMaxOutputBytes bounds the output of the convenience
	// formatters. Indentation grows with nesting, so a deeply nested unary
	// term renders quadratically large.
	defaultPrettyMaxOutputBytes = 16 << 20
)

// PrettyLimits bounds the work of [PrettyWithLimits] and
// [PrettyTermWithLimits]. A non-positive field selects its default.
type PrettyLimits struct {
	// MaxDepth is the deepest term nesting that will be formatted.
	MaxDepth int
	// MaxOutputBytes is the most output that will be produced.
	MaxOutputBytes int
}

// Pretty formats a Program. It returns an empty string when the program
// exceeds the default [PrettyLimits]; use [PrettyWithLimits] to observe the
// error or to choose other limits.
func Pretty[T Binder](p *Program[T]) string {
	out, _ := PrettyWithLimits(p, PrettyLimits{})

	return out
}

// PrettyTerm formats a Term. It returns an empty string when the term exceeds
// the default [PrettyLimits]; use [PrettyTermWithLimits] to observe the error
// or to choose other limits.
func PrettyTerm[T Binder](t Term[T]) string {
	out, _ := PrettyTermWithLimits[T](t, PrettyLimits{})

	return out
}

// PrettyWithLimits formats a Program, failing with an error instead of
// producing output past the limits.
func PrettyWithLimits[T Binder](
	p *Program[T],
	limits PrettyLimits,
) (string, error) {
	pp := newLimitedPrettyPrinter(2, limits)
	printProgram(pp, p)

	return pp.result()
}

// PrettyTermWithLimits formats a Term, failing with an error instead of
// producing output past the limits.
func PrettyTermWithLimits[T Binder](
	t Term[T],
	limits PrettyLimits,
) (string, error) {
	pp := newLimitedPrettyPrinter(2, limits)
	printTerm[T](pp, t)

	return pp.result()
}

// PrettyPrinter manages the state for pretty-printing AST nodes
type PrettyPrinter struct {
	builder    strings.Builder
	indent     int
	indentSize int
	maxDepth   int
	maxBytes   int
	// err is the first limit that was exceeded. Once set, nothing more is
	// written.
	err error
}

// NewPrettyPrinter creates a new PrettyPrinter with the specified indent size
// and the default limits
func NewPrettyPrinter(indentSize int) *PrettyPrinter {
	return newLimitedPrettyPrinter(indentSize, PrettyLimits{})
}

func newLimitedPrettyPrinter(
	indentSize int,
	limits PrettyLimits,
) *PrettyPrinter {
	pp := &PrettyPrinter{
		indentSize: indentSize,
		maxDepth:   limits.MaxDepth,
		maxBytes:   limits.MaxOutputBytes,
	}
	if pp.maxDepth <= 0 {
		pp.maxDepth = defaultPrettyMaxDepth
	}
	if pp.maxBytes <= 0 {
		pp.maxBytes = defaultPrettyMaxOutputBytes
	}

	return pp
}

func (pp *PrettyPrinter) result() (string, error) {
	if pp.err != nil {
		return "", pp.err
	}

	return pp.builder.String(), nil
}

// Reset clears the accumulated output and indentation state so the
// printer can be reused
func (pp *PrettyPrinter) Reset() {
	pp.builder.Reset()
	pp.indent = 0
	pp.err = nil
}

// reserve reports whether n more bytes fit in the output allowance, recording
// the limit error when they do not.
func (pp *PrettyPrinter) reserve(n int) bool {
	if pp.err != nil {
		return false
	}
	if n > pp.maxBytes-pp.builder.Len() {
		pp.err = fmt.Errorf("pretty output exceeds %d bytes", pp.maxBytes)

		return false
	}

	return true
}

// write writes a string to the builder
func (pp *PrettyPrinter) write(s string) {
	if pp.reserve(len(s)) {
		pp.builder.WriteString(s)
	}
}

// writef writes a formatted string to the builder
func (pp *PrettyPrinter) writef(format string, args ...any) {
	pp.write(fmt.Sprintf(format, args...))
}

// writeIndent writes the current indentation. The allowance is checked before
// the indentation is built, so a deep term cannot allocate more than the
// output limit.
func (pp *PrettyPrinter) writeIndent() {
	n := pp.indent * pp.indentSize
	if pp.reserve(n) {
		pp.builder.WriteString(strings.Repeat(" ", n))
	}
}

// increaseIndent increases the indentation level
func (pp *PrettyPrinter) increaseIndent() {
	if pp.indent >= pp.maxDepth && pp.err == nil {
		pp.err = fmt.Errorf("pretty depth exceeds %d", pp.maxDepth)
	}
	pp.indent++
}

// decreaseIndent decreases the indentation level
func (pp *PrettyPrinter) decreaseIndent() {
	if pp.indent > 0 {
		pp.indent--
	}
}

// printProgram formats the Program node
func printProgram[T Binder](pp *PrettyPrinter, prog *Program[T]) {
	pp.write("(program ")

	pp.write(
		fmt.Sprintf(
			"%d.%d.%d",
			prog.Version[0],
			prog.Version[1],
			prog.Version[2],
		),
	)

	pp.write("\n")

	pp.increaseIndent()
	pp.writeIndent()

	printTerm[T](pp, prog.Term)

	pp.decreaseIndent()
	pp.write("\n")

	pp.write(")")

	pp.write("\n")
}

// printTerm dispatches to the appropriate term printing method
func printTerm[T Binder](pp *PrettyPrinter, term Term[T]) {
	if pp.err != nil {
		return
	}
	switch t := term.(type) {
	case *Var[T]:
		pp.write(t.Name.TextName())
	case *Lambda[T]:
		pp.write("(lam ")

		pp.write(t.ParameterName.TextName())

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Body)

		pp.decreaseIndent()
		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Delay[T]:
		pp.write("(delay")

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Term)

		pp.decreaseIndent()
		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Force[T]:
		pp.write("(force")

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Term)

		pp.decreaseIndent()
		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Apply[T]:
		pp.write("[")

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Function)

		pp.write("\n")
		pp.writeIndent()

		printTerm[T](pp, t.Argument)

		pp.decreaseIndent()
		pp.write("\n")
		pp.writeIndent()

		pp.write("]")
	case *Builtin:
		pp.write("(builtin ")

		pp.write(t.String()) // Assumes DefaultFunction has a String method

		pp.write(")")
	case *Constr[T]:
		pp.write(fmt.Sprintf("(constr %d", t.Tag))

		if len(t.Fields) > 0 {
			pp.write("\n")
			pp.increaseIndent()

			for _, field := range t.Fields {
				pp.writeIndent()

				printTerm[T](pp, field)

				pp.write("\n")
			}

			pp.decreaseIndent()
			pp.writeIndent()
		}

		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Case[T]:
		pp.write("(case ")

		printTerm[T](pp, t.Constr)

		if len(t.Branches) > 0 {
			pp.write("\n")
			pp.increaseIndent()

			for _, branch := range t.Branches {
				pp.writeIndent()

				printTerm[T](pp, branch)

				pp.write("\n")
			}

			pp.decreaseIndent()
			pp.writeIndent()
		}

		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Error:
		pp.write("(error )")
	case *Constant:
		pp.printConstant(t, false)
	default:
		panic(fmt.Sprintf("unknown term: %T: %v", t, t))
	}
}

// printConstant formats a Constant node
func (pp *PrettyPrinter) printConstant(c *Constant, inCollection bool) {
	if !inCollection {
		pp.write("(con ")
	}

	switch con := c.Con.(type) {
	case *Integer:
		if !inCollection {
			pp.write("integer ")
		}

		pp.write(con.Inner.String())
	case *ByteString:
		if !inCollection {
			pp.write("bytestring ")
		}
		pp.write("#")

		for _, b := range con.Inner {
			pp.writef("%02x", b)
		}
	case *String:
		if !inCollection {
			pp.write("string ")
		}

		pp.write("\"")

		pp.write(escapeString(con.Inner))

		pp.write("\"")
	case *Unit:
		if !inCollection {
			pp.write("unit ")
		}

		pp.write("()")
	case *Bool:
		if !inCollection {
			pp.write("bool ")
		}

		if con.Inner {
			pp.write("True")
		} else {
			pp.write("False")
		}
	case *ProtoList:
		if !inCollection {
			pp.write("(list ")
			pp.printType(con.LTyp)
			pp.write(") ")
		}

		pp.printConstantCollection(con.List)
	case *ProtoArray:
		if !inCollection {
			pp.write("(array ")
			pp.printType(con.ATyp)
			pp.write(") ")
		}
		pp.printConstantCollection(con.Array)
	case *Value:
		if !inCollection {
			pp.write("value ")
		}
		pp.printConstantCollection(con.Entries)
	case *ProtoPair:
		if !inCollection {
			pp.write("(pair ")
			pp.printType(con.FstType)
			pp.write(" ")
			pp.printType(con.SndType)
			pp.write(") ")
		}
		pp.write("(")
		pp.printConstant(&Constant{Con: con.First}, true)
		pp.write(", ")
		pp.printConstant(&Constant{Con: con.Second}, true)
		pp.write(")")
	case *Data:
		if !inCollection {
			pp.write("data ")
			pp.write("(")
		}

		pp.printPlutusData(con.Inner)

		if !inCollection {
			pp.write(")")
		}
	case *Bls12_381G1Element:
		if !inCollection {
			pp.write("bls12_381_G1_element ")
		}
		pp.write("0x")

		affine := new(bls.G1Affine).FromJacobian(con.Inner)

		for _, b := range affine.Bytes() {
			pp.writef("%02x", b)
		}
	case *Bls12_381G2Element:
		if !inCollection {
			pp.write("bls12_381_G2_element ")
		}
		pp.write("0x")

		affine := new(bls.G2Affine).FromJacobian(con.Inner)

		for _, b := range affine.Bytes() {
			pp.writef("%02x", b)
		}
	default:
		pp.write(fmt.Sprintf("unknown constant: %v", c))
	}

	if !inCollection {
		pp.write(")")
	}
}

func (pp *PrettyPrinter) printConstantCollection(items []IConstant) {
	if len(items) == 0 {
		pp.write("[]")
		return
	}
	pp.write("[\n")
	pp.increaseIndent()
	for i, item := range items {
		pp.writeIndent()
		pp.printConstant(&Constant{Con: item}, true)
		if i < len(items)-1 {
			pp.write(",")
		}
		pp.write("\n")
	}
	pp.decreaseIndent()
	pp.writeIndent()
	pp.write("]")
}

// printType formats a Typ interface
func (pp *PrettyPrinter) printType(typ Typ) {
	switch t := typ.(type) {
	case *TInteger:
		pp.write("integer")
	case *TByteString:
		pp.write("bytestring")
	case *TString:
		pp.write("string")
	case *TUnit:
		pp.write("unit")
	case *TBool:
		pp.write("bool")
	case *TData:
		pp.write("data")
	case *TBls12_381G1Element:
		pp.write("bls12_381_G1_element")
	case *TBls12_381G2Element:
		pp.write("bls12_381_G2_element")
	case *TBls12_381MlResult:
		pp.write("bls12_381_mlresult")
	case *TList:
		pp.write("(list ")
		pp.printType(t.Typ)
		pp.write(")")
	case *TArray:
		pp.write("(array ")
		pp.printType(t.Typ)
		pp.write(")")
	case *TPair:
		pp.write("(pair ")
		pp.printType(t.First)
		pp.write(" ")
		pp.printType(t.Second)
		pp.write(")")
	case *TValue:
		pp.write("value")
	default:
		pp.write(fmt.Sprintf("unknown type: %v", typ))
	}
}

// printPlutusData formats a PlutusData node
func (pp *PrettyPrinter) printPlutusData(pd data.PlutusData) {
	switch d := pd.(type) {
	case *data.Integer:
		pp.write("I ")
		pp.write(d.Inner.String())
	case *data.ByteString:
		pp.write("B #")
		for _, b := range d.Inner {
			pp.writef("%02x", b)
		}
	case *data.List:
		if len(d.Items) == 0 {
			pp.write("List []")
		} else {
			pp.write("List [\n")
			pp.increaseIndent()
			for i, item := range d.Items {
				pp.writeIndent()
				pp.printPlutusData(item)
				if i < len(d.Items)-1 {
					pp.write(",")
				}
				pp.write("\n")
			}
			pp.decreaseIndent()
			pp.writeIndent()
			pp.write("]")
		}
	case *data.Map:
		if len(d.Pairs) == 0 {
			pp.write("Map []")
		} else {
			pp.write("Map [\n")
			pp.increaseIndent()
			for i, pair := range d.Pairs {
				pp.writeIndent()
				pp.write("(")
				pp.printPlutusData(pair[0])
				pp.write(", ")
				pp.printPlutusData(pair[1])
				pp.write(")")
				if i < len(d.Pairs)-1 {
					pp.write(",")
				}
				pp.write("\n")
			}
			pp.decreaseIndent()
			pp.writeIndent()
			pp.write("]")
		}
	case *data.Value:
		pp.printPlutusValue(d)
	case *data.Constr:
		pp.write(fmt.Sprintf("Constr %d ", d.Tag))
		if len(d.Fields) == 0 {
			pp.write("[]")
		} else {
			pp.write("[\n")
			pp.increaseIndent()
			for i, field := range d.Fields {
				pp.writeIndent()
				pp.printPlutusData(field)
				if i < len(d.Fields)-1 {
					pp.write(",")
				}
				pp.write("\n")
			}
			pp.decreaseIndent()
			pp.writeIndent()
			pp.write("]")
		}
	default:
		pp.write(fmt.Sprintf("unknown PlutusData: %v", pd))
	}
}

func (pp *PrettyPrinter) printPlutusValue(value *data.Value) {
	if err := value.Validate(); err != nil {
		pp.write(fmt.Sprintf("Value{invalid: %v}", err))
		return
	}
	pp.write("V [")
	if value.Inner != nil {
		for i, policy := range value.Inner.Pairs {
			if i > 0 {
				pp.write(", ")
			}
			pp.write("(")
			pp.printValueAtom(policy[0])
			pp.write(", [")
			tokens := policy[1].(*data.Map)
			for j, token := range tokens.Pairs {
				if j > 0 {
					pp.write(", ")
				}
				pp.write("(")
				pp.printValueAtom(token[0])
				pp.write(", ")
				pp.printValueAtom(token[1])
				pp.write(")")
			}
			pp.write("])")
		}
	}
	pp.write("]")
}

func (pp *PrettyPrinter) printValueAtom(pd data.PlutusData) {
	switch value := pd.(type) {
	case *data.ByteString:
		pp.write("#")
		for _, b := range value.Inner {
			pp.writef("%02x", b)
		}
	case *data.Integer:
		pp.write(value.Inner.String())
	default:
		pp.printPlutusData(pd)
	}
}

// escapeString escapes special characters in a string for printing
func escapeString(s string) string {
	var builder strings.Builder

	for _, r := range s {
		switch r {
		case '"':
			builder.WriteString("\\\"")
		case '\\':
			builder.WriteString("\\\\")
		case '\n':
			builder.WriteString("\\n")
		case '\t':
			builder.WriteString("\\t")
		case '\a':
			builder.WriteString("\\a")
		case '\b':
			builder.WriteString("\\b")
		case '\f':
			builder.WriteString("\\f")
		case '\r':
			builder.WriteString("\\r")
		case '\v':
			builder.WriteString("\\v")
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&builder, "\\o%03o", r)
			} else {
				builder.WriteRune(r)
			}
		}
	}

	return builder.String()
}
