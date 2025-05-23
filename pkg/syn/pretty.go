package syn

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/blinklabs-io/plutigo/pkg/data"
	"github.com/phoreproject/bls"
)

// Pretty Print a Program
func Pretty[T Binder](p *Program[T]) string {
	pp := NewPrettyPrinter(2)

	return prettyPrintProgram(pp, p)
}

// Pretty Print a Program
func PrettyTerm[T Binder](t Term[T]) string {
	pp := NewPrettyPrinter(2)

	return prettyPrintTerm[T](pp, t)
}

// PrettyPrinter manages the state for pretty-printing AST nodes
type PrettyPrinter struct {
	builder    strings.Builder
	indent     int
	indentSize int
}

// NewPrettyPrinter creates a new PrettyPrinter with the specified indent size
func NewPrettyPrinter(indentSize int) *PrettyPrinter {
	return &PrettyPrinter{
		indentSize: indentSize,
	}
}

// write writes a string to the builder
func (pp *PrettyPrinter) write(s string) {
	pp.builder.WriteString(s)
}

// writeIndent writes the current indentation
func (pp *PrettyPrinter) writeIndent() {
	pp.write(strings.Repeat(" ", pp.indent*pp.indentSize))
}

// increaseIndent increases the indentation level
func (pp *PrettyPrinter) increaseIndent() {
	pp.indent++
}

// decreaseIndent decreases the indentation level
func (pp *PrettyPrinter) decreaseIndent() {
	if pp.indent > 0 {
		pp.indent--
	}
}

// PrettyPrintTerm formats a Term[Name] to a string
func prettyPrintTerm[T Binder](pp *PrettyPrinter, term Term[T]) string {
	printTerm[T](pp, term, true)

	return pp.builder.String()
}

func prettyPrintProgram[T Binder](pp *PrettyPrinter, prog *Program[T]) string {
	printProgram(pp, prog)

	return pp.builder.String()
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

	printTerm[T](pp, prog.Term, false)

	pp.decreaseIndent()
	pp.write("\n")

	pp.write(")")

	pp.write("\n")
}

// printTerm dispatches to the appropriate term printing method
func printTerm[T Binder](pp *PrettyPrinter, term Term[T], isTopLevel bool) {
	switch t := term.(type) {
	case *Var[T]:
		pp.write(t.Name.TextName())
	case *Lambda[T]:
		pp.write("(lam ")

		pp.write(t.ParameterName.TextName())

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Body, false)

		pp.decreaseIndent()
		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Delay[T]:
		pp.write("(delay")

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Term, false)

		pp.decreaseIndent()
		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Force[T]:
		pp.write("(force")

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Term, false)

		pp.decreaseIndent()
		pp.write("\n")
		pp.writeIndent()

		pp.write(")")
	case *Apply[T]:
		pp.write("[")

		pp.write("\n")
		pp.increaseIndent()
		pp.writeIndent()

		printTerm[T](pp, t.Function, false)

		pp.write("\n")
		pp.writeIndent()

		printTerm[T](pp, t.Argument, false)

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

				printTerm[T](pp, field, false)

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

		printTerm[T](pp, t.Constr, false)

		if len(t.Branches) > 0 {
			pp.write("\n")
			pp.increaseIndent()

			for _, branch := range t.Branches {
				pp.writeIndent()

				printTerm[T](pp, branch, false)

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
		pp.printConstant(t)
	default:
		fmt.Println(reflect.TypeOf(t))
		panic(fmt.Sprintf("unknown term: %v", t))
	}
}

// printConstant formats a Constant node
func (pp *PrettyPrinter) printConstant(c *Constant) {
	pp.write("(con ")

	switch con := c.Con.(type) {
	case *Integer:
		pp.write("integer ")

		pp.write(con.Inner.String())
	case *ByteString:
		pp.write("bytestring #")

		for _, b := range con.Inner {
			pp.builder.WriteString(fmt.Sprintf("%02x", b))
		}
	case *String:
		pp.write("string \"")

		pp.write(escapeString(con.Inner))

		pp.write("\"")
	case *Unit:
		pp.write("unit ()")
	case *Bool:
		pp.write("bool ")

		if con.Inner {
			pp.write("True")
		} else {
			pp.write("False")
		}
	case *ProtoList:
		pp.write("(list ")
		pp.printType(con.LTyp)
		pp.write(") ")
		if len(con.List) == 0 {
			pp.write("[]")
		} else {
			pp.write("[\n")
			pp.increaseIndent()
			for i, item := range con.List {
				pp.writeIndent()
				pp.printConstant(&Constant{Con: item})
				if i < len(con.List)-1 {
					pp.write(",")
				}
				pp.write("\n")
			}
			pp.decreaseIndent()
			pp.writeIndent()
			pp.write("]")
		}
	case *ProtoPair:
		pp.write("(pair ")
		pp.printType(con.FstType)
		pp.write(" ")
		pp.printType(con.SndType)
		pp.write(") (")
		pp.printConstant(&Constant{Con: con.First})
		pp.write(", ")
		pp.printConstant(&Constant{Con: con.Second})
		pp.write(")")
	case *Data:
		pp.write("data ")
		pp.printPlutusData(con.Inner)
	case *Bls12_381G1Element:
		pp.write("bls12_381_G1_element 0x")

		for _, b := range bls.CompressG1(con.Inner.ToAffine()) {
			pp.builder.WriteString(fmt.Sprintf("%02x", b))
		}
	case *Bls12_381G2Element:
		pp.write("bls12_381_G2_element 0x")

		for _, b := range bls.CompressG2(con.Inner.ToAffine()) {
			pp.builder.WriteString(fmt.Sprintf("%02x", b))
		}
	default:
		pp.write(fmt.Sprintf("unknown constant: %v", c))
	}

	pp.write(")")
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
	case *TList:
		pp.write("(list ")
		pp.printType(t.Typ)
		pp.write(")")
	case *TPair:
		pp.write("(pair ")
		pp.printType(t.First)
		pp.write(" ")
		pp.printType(t.Second)
		pp.write(")")
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
			pp.builder.WriteString(fmt.Sprintf("%02x", b))
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
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}
