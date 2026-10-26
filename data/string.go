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

package data

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// The String methods of the container types write into a single
// strings.Builder. Formatting each level with fmt.Sprintf would copy the whole
// rendered subtree once per nesting level, which is quadratic for deeply
// nested values. The walk uses an explicit stack rather than recursion, since
// decoded values may be nested as deeply as their encoding allows.

// stringFrame renders the children of one container: items separated by a
// space, or map pairs as "[key value]" separated by a space, then closer.
type stringFrame struct {
	children childFrame
	isMap    bool
	closer   string
}

// writePlutusDataString appends the String form of pd to sb. Nil values render
// as "<nil>", as fmt's %v does. Types outside this package fall back to fmt.
func writePlutusDataString(sb *strings.Builder, pd PlutusData) {
	var stackBuf [16]stringFrame
	stack := stackBuf[:0]
	for {
		stack = writeStringShallow(sb, pd, stack)
		for {
			if len(stack) == 0 {
				return
			}
			top := &stack[len(stack)-1]
			index := top.children.next
			var more bool
			if pd, more = top.children.nextChild(); more {
				switch {
				case !top.isMap:
					if index > 0 {
						sb.WriteByte(' ')
					}
				case index%2 == 0:
					if index > 0 {
						sb.WriteString("] ")
					}
					sb.WriteByte('[')
				default:
					sb.WriteByte(' ')
				}
				break
			}
			if top.isMap && index > 0 {
				sb.WriteByte(']')
			}
			sb.WriteString(top.closer)
			stack = stack[:len(stack)-1]
		}
	}
}

// writeStringShallow writes pd, or for a container its opening text, and
// pushes the frames that render the rest of it.
func writeStringShallow(sb *strings.Builder, pd PlutusData, stack []stringFrame) []stringFrame {
	switch v := pd.(type) {
	case nil:
	case *List:
		if v != nil {
			return openListString(sb, v, stack)
		}
	case List:
		return openListString(sb, &v, stack)
	case *Map:
		if v != nil {
			return openMapString(sb, v, stack)
		}
	case Map:
		return openMapString(sb, &v, stack)
	case *Constr:
		if v != nil {
			return openConstrString(sb, v, stack)
		}
	case Constr:
		return openConstrString(sb, &v, stack)
	case *Integer:
		if v != nil {
			writeIntegerString(sb, v)
			return stack
		}
	case Integer:
		writeIntegerString(sb, &v)
		return stack
	case *ByteString:
		if v != nil {
			writeByteStringString(sb, v)
			return stack
		}
	case ByteString:
		writeByteStringString(sb, &v)
		return stack
	case *Value:
		if v != nil {
			return openValueString(sb, v, stack)
		}
	case Value:
		return openValueString(sb, &v, stack)
	default:
		fmt.Fprint(sb, pd)
		return stack
	}
	sb.WriteString("<nil>")
	return stack
}

func openListString(sb *strings.Builder, l *List, stack []stringFrame) []stringFrame {
	sb.WriteString("List[")
	return append(stack, stringFrame{children: childFrame{items: l.Items}, closer: "]"})
}

func openMapString(sb *strings.Builder, m *Map, stack []stringFrame) []stringFrame {
	sb.WriteString("Map[")
	return append(stack, stringFrame{children: childFrame{pairs: m.Pairs}, isMap: true, closer: "]"})
}

func openConstrString(sb *strings.Builder, c *Constr, stack []stringFrame) []stringFrame {
	sb.WriteString("Constr{tag: ")
	sb.WriteString(constrTagString(c.Tag))
	sb.WriteString(", fields: [")
	return append(stack, stringFrame{children: childFrame{items: c.Fields}, closer: "]}"})
}

func openValueString(sb *strings.Builder, v *Value, stack []stringFrame) []stringFrame {
	sb.WriteString("Value{")
	if v.Inner == nil {
		sb.WriteString("<nil>}")
		return stack
	}
	stack = append(stack, stringFrame{closer: "}"})
	return openMapString(sb, v.Inner, stack)
}

func writeIntegerString(sb *strings.Builder, i *Integer) {
	sb.WriteString("Integer(")
	sb.WriteString(i.value().String())
	sb.WriteByte(')')
}

func writeByteStringString(sb *strings.Builder, b *ByteString) {
	sb.WriteString("ByteString(")
	sb.WriteString(hex.EncodeToString(b.Inner))
	sb.WriteByte(')')
}
