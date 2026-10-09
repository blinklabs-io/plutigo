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
	"fmt"
	"math/big"
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
	// MaxDepth bounds recursive term, constant, type and data formatting.
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
	depth      int
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
	pp.depth = 0
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
	if pp.err == nil {
		pp.write(fmt.Sprintf(format, args...))
	}
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
	if c, ok := term.(*Constant); ok {
		pp.printConstant(c, false)
		return
	}
	if !pp.enter() {
		return
	}
	defer pp.leave()
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
				if pp.err != nil {
					break
				}
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
				if pp.err != nil {
					break
				}
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
	default:
		panic(fmt.Sprintf("unknown term: %T: %v", t, t))
	}
}

// printConstant formats a Constant node
func (pp *PrettyPrinter) printConstant(c *Constant, inCollection bool) {
	if !pp.enter() {
		return
	}
	defer pp.leave()
	if !inCollection {
		pp.write("(con ")
	}

	switch con := c.Con.(type) {
	case *Integer:
		if !inCollection {
			pp.write("integer ")
		}

		pp.writeInteger(con.Inner)
	case *ByteString:
		if !inCollection {
			pp.write("bytestring ")
		}
		pp.write("#")

		pp.writeHex(con.Inner)
	case *String:
		if !inCollection {
			pp.write("string ")
		}

		pp.write("\"")

		pp.writeEscapedString(con.Inner)

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

		if !pp.reserve(bls.SizeOfG1AffineCompressed * 2) {
			return
		}
		affine := new(bls.G1Affine).FromJacobian(con.Inner)

		encoded := affine.Bytes()
		pp.writeHex(encoded[:])
	case *Bls12_381G2Element:
		if !inCollection {
			pp.write("bls12_381_G2_element ")
		}
		pp.write("0x")

		if !pp.reserve(bls.SizeOfG2AffineCompressed * 2) {
			return
		}
		affine := new(bls.G2Affine).FromJacobian(con.Inner)

		encoded := affine.Bytes()
		pp.writeHex(encoded[:])
	default:
		pp.write(fmt.Sprintf("unknown constant: %v", c))
	}

	if !inCollection {
		pp.write(")")
	}
}

func (pp *PrettyPrinter) printConstantCollection(items []IConstant) {
	if pp.err != nil {
		return
	}
	if len(items) == 0 {
		pp.write("[]")
		return
	}
	pp.write("[\n")
	pp.increaseIndent()
	for i, item := range items {
		if pp.err != nil {
			break
		}
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
	if !pp.enter() {
		return
	}
	defer pp.leave()
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
	if !pp.enter() {
		return
	}
	defer pp.leave()
	switch d := pd.(type) {
	case *data.Integer:
		pp.write("I ")
		pp.writeInteger(d.Inner)
	case *data.ByteString:
		pp.write("B #")
		pp.writeHex(d.Inner)
	case *data.List:
		if len(d.Items) == 0 {
			pp.write("List []")
		} else {
			pp.write("List [\n")
			pp.increaseIndent()
			for i, item := range d.Items {
				if pp.err != nil {
					break
				}
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
				if pp.err != nil {
					break
				}
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
		pp.write("Constr ")
		pp.writeInteger(d.Tag)
		pp.write(" ")
		if len(d.Fields) == 0 {
			pp.write("[]")
		} else {
			pp.write("[\n")
			pp.increaseIndent()
			for i, field := range d.Fields {
				if pp.err != nil {
					break
				}
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
	if pp.err != nil {
		return
	}
	if value.Inner != nil {
		allowance := pp.maxBytes - pp.builder.Len()
		remaining := allowance
		if len(value.Inner.Pairs) > remaining/7 {
			pp.reserve(allowance + 1)
			return
		}
		// Even empty keys need seven bytes per policy and six per token.
		// Check this lower bound before Validate walks every token.
		remaining -= len(value.Inner.Pairs) * 7
		for _, policy := range value.Inner.Pairs {
			if tokens, ok := policy[1].(*data.Map); ok && tokens != nil {
				if len(tokens.Pairs) > remaining/6 {
					pp.reserve(allowance + 1)
					return
				}
				remaining -= len(tokens.Pairs) * 6
			}
		}
	}
	if err := value.Validate(); err != nil {
		pp.write(fmt.Sprintf("Value{invalid: %v}", err))
		return
	}
	pp.write("V [")
	if value.Inner != nil {
		for i, policy := range value.Inner.Pairs {
			if pp.err != nil {
				break
			}
			if i > 0 {
				pp.write(", ")
			}
			pp.write("(")
			pp.printValueAtom(policy[0])
			pp.write(", [")
			tokens := policy[1].(*data.Map)
			for j, token := range tokens.Pairs {
				if pp.err != nil {
					break
				}
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
	if pp.err != nil {
		return
	}
	switch value := pd.(type) {
	case *data.ByteString:
		pp.write("#")
		pp.writeHex(value.Inner)
	case *data.Integer:
		pp.writeInteger(value.Inner)
	default:
		pp.printPlutusData(pd)
	}
}

func (pp *PrettyPrinter) writeEscapedString(s string) {
	for _, r := range s {
		if pp.err != nil {
			break
		}
		switch r {
		case '"':
			pp.write("\\\"")
		case '\\':
			pp.write("\\\\")
		case '\n':
			pp.write("\\n")
		case '\t':
			pp.write("\\t")
		case '\a':
			pp.write("\\a")
		case '\b':
			pp.write("\\b")
		case '\f':
			pp.write("\\f")
		case '\r':
			pp.write("\\r")
		case '\v':
			pp.write("\\v")
		default:
			if r < 0x20 || r == 0x7f {
				pp.writef("\\o%03o", r)
			} else {
				pp.write(string(r))
			}
		}
	}
}

func (pp *PrettyPrinter) enter() bool {
	if pp.err != nil {
		return false
	}
	if pp.depth >= pp.maxDepth {
		pp.err = fmt.Errorf("pretty depth exceeds %d", pp.maxDepth)
		return false
	}
	pp.depth++
	return true
}

func (pp *PrettyPrinter) leave() { pp.depth-- }

func (pp *PrettyPrinter) writeHex(input []byte) {
	remaining := pp.maxBytes - pp.builder.Len()
	if len(input) > remaining/2 {
		pp.reserve(remaining + 1)
		return
	}
	if !pp.reserve(2 * len(input)) {
		return
	}
	const hex = "0123456789abcdef"
	for _, b := range input {
		pp.builder.WriteByte(hex[b>>4])
		pp.builder.WriteByte(hex[b&15])
	}
}

func (pp *PrettyPrinter) writeInteger(n *big.Int) {
	if pp.err != nil {
		return
	}
	// 2^10 > 10^3: this lower bound rejects oversized magnitudes before
	// big.Int.String allocates its decimal representation.
	minimum := (n.BitLen() / 10) * 3
	if !pp.reserve(minimum) {
		return
	}
	pp.write(n.String())
}
