package syn

import (
	"fmt"
	"strings"
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

	pp.write(fmt.Sprintf("%d.%d.%d", prog.Version[0], prog.Version[1], prog.Version[2]))

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
	default:
		pp.write(fmt.Sprintf("unknown constant: %v", c))
	}

	pp.write(")")
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

// Updated AST types with String methods and modified fields

// Integer implements String
func (i Integer) String() string {
	return i.Inner.String()
}

// ByteString implements String
func (b ByteString) String() string {
	pp := NewPrettyPrinter(2)

	pp.write("#")

	for _, byteVal := range b.Inner {
		pp.builder.WriteString(fmt.Sprintf("%02x", byteVal))
	}

	return pp.builder.String()
}

// String implements String
func (s String) String() string {
	return fmt.Sprintf("\"%s\"", escapeString(s.Inner))
}

// Unit implements String
func (u Unit) String() string {
	return "()"
}

// Bool implements String
func (b Bool) String() string {
	if b.Inner {
		return "True"
	}

	return "False"
}

// Pair implements String
func (p ProtoPair) String() string {
	panic("TODO")
}

// List implements String
func (p ProtoList) String() string {
	panic("TODO")
}

// List implements String
func (p Data) String() string {
	return "some data bro"
}
