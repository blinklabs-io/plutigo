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
	"bytes"
	"errors"
	"fmt"
	"math/big"
)

// The walkers in this file (Equal, Clone, Normalize, CBOR encoding) traverse
// PlutusData with an explicit stack instead of recursion. The decoders accept
// any nesting depth the input can encode (one level per byte of a
// transaction), and a Go stack overflow is a fatal error that no
// recover can catch, so no walker may recurse once per level. Each stack
// frame steps through one container's children in place, so the stack grows
// with nesting depth, not width.

// childFrame steps through the children of one container: items for a list
// or constructor, pairs (key then value) for a map.
type childFrame struct {
	items []PlutusData
	pairs [][2]PlutusData
	next  int
}

// done reports whether every child has been returned.
func (f *childFrame) done() bool {
	return f.next >= len(f.items)+2*len(f.pairs)
}

// nextChild returns the frame's next child, if any.
func (f *childFrame) nextChild() (PlutusData, bool) {
	if f.next < len(f.items) {
		child := f.items[f.next]
		f.next++
		return child, true
	}
	if f.next < 2*len(f.pairs) {
		child := f.pairs[f.next/2][f.next%2]
		f.next++
		return child, true
	}
	return nil, false
}

// equalData reports whether a and b are equal, with the semantics of the
// Equal methods: a node equals another of the same kind (b's containers in
// pointer form) with equal contents.
func equalData(a, b PlutusData) bool {
	type pairFrame struct {
		left, right childFrame
	}
	var stackBuf [16]pairFrame
	stack := stackBuf[:0]
	x, y := a, b
	for {
		left, right, ok := equalShallow(x, y)
		if !ok {
			return false
		}
		if len(left.items) > 0 || len(left.pairs) > 0 {
			stack = append(stack, pairFrame{left: left, right: right})
		}
		for {
			if len(stack) == 0 {
				return true
			}
			top := &stack[len(stack)-1]
			var more bool
			if x, more = top.left.nextChild(); more {
				y, _ = top.right.nextChild()
				if top.left.done() {
					// Drop the frame before descending into its last child,
					// so a chain of single-child containers needs no stack.
					stack = stack[:len(stack)-1]
				}
				break
			}
			top.left, top.right = childFrame{}, childFrame{}
			stack = stack[:len(stack)-1]
		}
	}
}

// equalShallow compares the nodes x and y without their children, and
// returns their children for the caller to compare pairwise; equal nodes
// have equally many.
func equalShallow(x, y PlutusData) (childFrame, childFrame, bool) {
	var none childFrame
	switch l := x.(type) {
	case *Constr:
		if l == nil {
			return none, none, false
		}
		return equalConstr(l, y)
	case Constr:
		return equalConstr(&l, y)
	case *List:
		if l == nil {
			return none, none, false
		}
		return equalList(l, y)
	case List:
		return equalList(&l, y)
	case *Map:
		if l == nil {
			return none, none, false
		}
		return equalMap(l, y)
	case Map:
		return equalMap(&l, y)
	case *Value:
		if l == nil {
			return none, none, false
		}
		return equalValue(l, y)
	case Value:
		return equalValue(&l, y)
	case *Integer:
		if l == nil {
			return none, none, false
		}
		return none, none, l.equalLeaf(y)
	case Integer:
		return none, none, l.equalLeaf(y)
	case *ByteString:
		if l == nil {
			return none, none, false
		}
		return none, none, l.equalLeaf(y)
	case ByteString:
		return none, none, l.equalLeaf(y)
	default:
		return none, none, false
	}
}

func equalConstr(l *Constr, y PlutusData) (childFrame, childFrame, bool) {
	r, ok := y.(*Constr)
	if !ok || r == nil || constrTagCmp(l.Tag, r.Tag) != 0 ||
		len(l.Fields) != len(r.Fields) {
		return childFrame{}, childFrame{}, false
	}
	return childFrame{items: l.Fields}, childFrame{items: r.Fields}, true
}

func equalList(l *List, y PlutusData) (childFrame, childFrame, bool) {
	r, ok := y.(*List)
	if !ok || r == nil || len(l.Items) != len(r.Items) {
		return childFrame{}, childFrame{}, false
	}
	return childFrame{items: l.Items}, childFrame{items: r.Items}, true
}

func equalMap(l *Map, y PlutusData) (childFrame, childFrame, bool) {
	r, ok := y.(*Map)
	if !ok || r == nil || len(l.Pairs) != len(r.Pairs) {
		return childFrame{}, childFrame{}, false
	}
	return childFrame{pairs: l.Pairs}, childFrame{pairs: r.Pairs}, true
}

func equalValue(l *Value, y PlutusData) (childFrame, childFrame, bool) {
	r, ok := y.(*Value)
	if !ok || r == nil || (l.Inner == nil) != (r.Inner == nil) {
		return childFrame{}, childFrame{}, false
	}
	if l.Inner == nil {
		return childFrame{}, childFrame{}, true
	}
	if len(l.Inner.Pairs) != len(r.Inner.Pairs) {
		return childFrame{}, childFrame{}, false
	}
	return childFrame{pairs: l.Inner.Pairs}, childFrame{pairs: r.Inner.Pairs}, true
}

func (i Integer) equalLeaf(pd PlutusData) bool {
	r, ok := pd.(*Integer)
	return ok && r != nil && i.value().Cmp(r.value()) == 0
}

func (b ByteString) equalLeaf(pd PlutusData) bool {
	r, ok := pd.(*ByteString)
	return ok && r != nil && bytes.Equal(b.Inner, r.Inner)
}

// copyFrame fills the children of one copied container: dstItems or
// dstPairs receive copies of the source children in order.
type copyFrame struct {
	src      childFrame
	dstItems []PlutusData
	dstPairs [][2]PlutusData
}

// copyData returns a deep copy of pd: the Clone semantics, or with
// normalize set the Normalize semantics.
func copyData(pd PlutusData, normalize bool) PlutusData {
	root, frame := copyShallow(pd, normalize)
	if frame == nil {
		return root
	}
	stack := []copyFrame{*frame}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		slot := top.src.next
		child, ok := top.src.nextChild()
		if !ok {
			stack = stack[:len(stack)-1]
			continue
		}
		copied, childFrame := copyShallow(child, normalize)
		if top.dstItems != nil {
			top.dstItems[slot] = copied
		} else {
			top.dstPairs[slot/2][slot%2] = copied
		}
		if top.src.done() {
			stack = stack[:len(stack)-1]
		}
		if childFrame != nil {
			stack = append(stack, *childFrame)
		}
	}
	return root
}

// copyShallow copies the node pd without its children. For a container with
// children it returns the frame that copies them into the new node.
func copyShallow(pd PlutusData, normalize bool) (PlutusData, *copyFrame) {
	switch v := pd.(type) {
	case *Constr:
		if v == nil {
			return pd, nil
		}
		return copyConstr(v, normalize)
	case Constr:
		return copyConstr(&v, normalize)
	case *List:
		if v == nil {
			return pd, nil
		}
		return copyList(v, normalize)
	case List:
		return copyList(&v, normalize)
	case *Map:
		if v == nil {
			return pd, nil
		}
		return copyMap(v, normalize)
	case Map:
		return copyMap(&v, normalize)
	case *Value:
		if v == nil {
			return pd, nil
		}
		return copyValue(v, normalize)
	case Value:
		return copyValue(&v, normalize)
	case *Integer:
		if v == nil || normalize {
			return pd, nil
		}
		return v.Clone(), nil
	case Integer:
		if normalize {
			return pd, nil
		}
		return v.Clone(), nil
	case *ByteString:
		if v == nil || normalize {
			return pd, nil
		}
		return v.Clone(), nil
	case ByteString:
		if normalize {
			return pd, nil
		}
		return v.Clone(), nil
	default:
		// nil, or a type outside this package: nothing to copy.
		return pd, nil
	}
}

func copyConstr(v *Constr, normalize bool) (PlutusData, *copyFrame) {
	fields := make([]PlutusData, len(v.Fields))
	out := &Constr{Fields: fields}
	if normalize {
		// A nil tag stays nil; MarshalCBOR treats it as zero. The tag is
		// copied rather than aliased: big.Int is mutable.
		if v.Tag != nil {
			out.Tag = new(big.Int).Set(v.Tag)
		}
	} else {
		out.Tag = new(big.Int)
		if v.Tag != nil {
			out.Tag.Set(v.Tag)
		}
		out.useIndef = cloneUseIndef(v.useIndef)
	}
	if len(fields) == 0 {
		return out, nil
	}
	return out, &copyFrame{src: childFrame{items: v.Fields}, dstItems: fields}
}

func copyList(v *List, normalize bool) (PlutusData, *copyFrame) {
	items := make([]PlutusData, len(v.Items))
	out := &List{Items: items}
	if !normalize {
		out.useIndef = cloneUseIndef(v.useIndef)
	}
	if len(items) == 0 {
		return out, nil
	}
	return out, &copyFrame{src: childFrame{items: v.Items}, dstItems: items}
}

func copyMap(v *Map, normalize bool) (*Map, *copyFrame) {
	pairs := make([][2]PlutusData, len(v.Pairs))
	out := &Map{Pairs: pairs}
	if !normalize {
		out.useIndef = cloneUseIndef(v.useIndef)
	}
	if len(pairs) == 0 {
		return out, nil
	}
	return out, &copyFrame{src: childFrame{pairs: v.Pairs}, dstPairs: pairs}
}

func copyValue(v *Value, normalize bool) (PlutusData, *copyFrame) {
	if v.Inner == nil {
		return &Value{}, nil
	}
	inner, frame := copyMap(v.Inner, normalize)
	return &Value{Inner: inner}, frame
}

// encodeFrame writes the children of one container, then its break byte
// when it is indefinite-length.
type encodeFrame struct {
	children childFrame
	indef    bool
	desc     string
}

// appendPlutusDataCBOR writes the CBOR encoding of pd to buf. Containers
// write their children into the same buffer, so the work is linear in the
// size of the encoding.
func appendPlutusDataCBOR(buf *bytes.Buffer, pd PlutusData) error {
	stack := make([]encodeFrame, 0, 16)
	for {
		frame, err := appendCBORShallow(buf, pd)
		if err != nil {
			if n := len(stack); n > 0 {
				top := &stack[n-1]
				return wrapChildError(top.desc, top.children.next-1, err)
			}
			return err
		}
		if frame != nil {
			stack = append(stack, *frame)
		}
		for {
			if len(stack) == 0 {
				return nil
			}
			top := &stack[len(stack)-1]
			var more bool
			if pd, more = top.children.nextChild(); more {
				break
			}
			if top.indef {
				buf.WriteByte(0xff) // End indefinite-length container
			}
			stack = stack[:len(stack)-1]
		}
	}
}

// appendCBORShallow writes pd, or for a container only its head, and returns
// the frame that writes the container's children.
func appendCBORShallow(buf *bytes.Buffer, pd PlutusData) (*encodeFrame, error) {
	switch v := pd.(type) {
	case *List:
		if v != nil {
			return v.appendCBORHead(buf), nil
		}
	case List:
		return v.appendCBORHead(buf), nil
	case *Map:
		if v != nil {
			return v.appendCBORHead(buf), nil
		}
	case Map:
		return v.appendCBORHead(buf), nil
	case *Constr:
		if v != nil {
			return v.appendCBORHead(buf)
		}
	case Constr:
		return v.appendCBORHead(buf)
	case *Integer:
		if v != nil && v.Inner != nil {
			appendIntegerCBOR(buf, v.Inner)
			return nil, nil
		}
	case Integer:
		if v.Inner != nil {
			appendIntegerCBOR(buf, v.Inner)
			return nil, nil
		}
	case *ByteString:
		if v != nil {
			appendDataByteString(buf, v.Inner)
			return nil, nil
		}
	case ByteString:
		appendDataByteString(buf, v.Inner)
		return nil, nil
	}
	encoded, err := cborMarshal(pd)
	if err != nil {
		return nil, err
	}
	buf.Write(encoded)
	return nil, nil
}

// arrayFrame writes the head of a CBOR array of items and returns the frame
// that writes them. When useIndef is true and the slice is non-empty,
// indefinite-length encoding is used. Empty slices always use definite-length
// encoding regardless of useIndef, matching Haskell's cborg behavior.
func arrayFrame(
	buf *bytes.Buffer,
	items []PlutusData,
	useIndef bool,
	desc string,
) *encodeFrame {
	useIndef = useIndef && len(items) > 0
	if useIndef {
		buf.WriteByte(CborTypeArray | CborIndefFlag)
	} else {
		appendCBORHead(buf, CborTypeArray, uint64(len(items)))
	}
	return &encodeFrame{
		children: childFrame{items: items},
		indef:    useIndef,
		desc:     desc,
	}
}

// wrapChildError adds the position of the child that failed to encode.
func wrapChildError(desc string, index int, err error) error {
	if desc == "map" {
		if index%2 == 0 {
			return fmt.Errorf("encode map key: %w", err)
		}
		return fmt.Errorf("encode map value: %w", err)
	}
	return fmt.Errorf("failed to encode %s %d: %w", desc, index, err)
}

// ErrSharedNodes is returned by CheckTree for a value in which some
// container's children are reachable along more than one path.
var ErrSharedNodes = errors.New("PlutusData shares child nodes or is cyclic")

// CheckTree reports whether pd is a finite tree: no container's children
// (its item, field or pair array) are reachable along two paths. Every value
// the decoders produce is a tree, and the walk then visits each node once,
// so its work is linear in the memory the value occupies. A value built by
// hand that shares or cycles children is rejected with ErrSharedNodes
// instead: walking it as a tree would take time exponential in its size or
// never end. Nil nodes are skipped. The walk does not recurse.
func CheckTree(pd PlutusData) error {
	seen := make(map[any]struct{})
	visit := func(key any) error {
		if _, ok := seen[key]; ok {
			return ErrSharedNodes
		}
		seen[key] = struct{}{}
		return nil
	}
	var stackBuf [16]childFrame
	stack := stackBuf[:0]
	for {
		frame := containerChildren(pd)
		switch {
		case len(frame.items) > 0:
			if err := visit(&frame.items[0]); err != nil {
				return err
			}
			stack = append(stack, frame)
		case len(frame.pairs) > 0:
			if err := visit(&frame.pairs[0]); err != nil {
				return err
			}
			stack = append(stack, frame)
		}
		for {
			if len(stack) == 0 {
				return nil
			}
			top := &stack[len(stack)-1]
			var more bool
			if pd, more = top.nextChild(); more {
				if top.done() {
					stack = stack[:len(stack)-1]
				}
				break
			}
			stack = stack[:len(stack)-1]
		}
	}
}

// containerChildren returns a frame over the children of pd, which is empty
// for leaves and nil nodes.
func containerChildren(pd PlutusData) childFrame {
	switch v := pd.(type) {
	case *Constr:
		if v != nil {
			return childFrame{items: v.Fields}
		}
	case Constr:
		return childFrame{items: v.Fields}
	case *List:
		if v != nil {
			return childFrame{items: v.Items}
		}
	case List:
		return childFrame{items: v.Items}
	case *Map:
		if v != nil {
			return childFrame{pairs: v.Pairs}
		}
	case Map:
		return childFrame{pairs: v.Pairs}
	case *Value:
		if v != nil && v.Inner != nil {
			return childFrame{pairs: v.Inner.Pairs}
		}
	case Value:
		if v.Inner != nil {
			return childFrame{pairs: v.Inner.Pairs}
		}
	}
	return childFrame{}
}
