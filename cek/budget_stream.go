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

package cek

import (
	"github.com/blinklabs-io/plutigo/data"
)

// Streamed builtin costs.
//
// The reference does not charge a builtin call as one amount. A costing
// function maps the lazy size streams of its arguments (ExMemoryUsage
// CostRose values flattened in preorder: one element per Data node, for
// example) to a lazy stream of costs, and the machine spends that stream one
// element at a time, stopping at the first element that overdraws the budget
// (PlutusCore.Evaluation.Machine.CostStream and ExBudgetStream). A linear
// costing function, for instance, yields its intercept and then slope*size for
// each element of the argument's size stream.
//
// The elements always add up to the cost plutigo charges in one amount, so a
// call that fits the budget consumes the same either way. Only an overdrawing
// call differs: the reference stops part way through the stream, and its
// remaining budget then reflects only the elements up to and including the one
// that overdrew. plutigo charges the total on the success path and replays the
// stream only after an overdraw, to reproduce that remaining budget.

// costChunks is a pull iterator over a non-empty cost stream.
type costChunks interface {
	// next returns the next element and whether it is the last one. It must
	// not be called again after it reported the last element.
	next() (chunk int64, last bool)
}

// singleChunk is a stream of one element (the reference's CostLast).
type singleChunk struct {
	value int64
}

func (s *singleChunk) next() (int64, bool) {
	return s.value, true
}

// peekChunks lets a combinator inspect an element before consuming it.
type peekChunks struct {
	src    costChunks
	value  int64
	last   bool
	peeked bool
}

func (p *peekChunks) peek() (int64, bool) {
	if !p.peeked {
		p.value, p.last = p.src.next()
		p.peeked = true
	}
	return p.value, p.last
}

func (p *peekChunks) next() (int64, bool) {
	value, last := p.peek()
	p.peeked = false
	return value, last
}

// dataSizeChunks is the size stream of a Data value: one element per node in
// preorder, 4 plus the size of an integer or bytestring leaf (the reference's
// ExMemoryUsage Data). With nodeCount set every node counts 1 instead (the
// reference's DataNodeCount, used by unValueData).
type dataSizeChunks struct {
	walker    plutusDataWalker
	stack     []dataWalkFrame
	nodeCount bool
}

func newDataSizeChunks(d data.PlutusData, nodeCount bool) *dataSizeChunks {
	return &dataSizeChunks{
		walker:    newPlutusDataWalker(d),
		nodeCount: nodeCount,
	}
}

func (c *dataSizeChunks) next() (int64, bool) {
	node, stack, _ := c.walker.next(c.stack)
	c.stack = stack
	size := int64(1)
	if !c.nodeCount {
		// The evaluation path validates every node it charges for before
		// the stream is replayed, so the error cannot occur here.
		nodeSize, _ := dataNodeExMem(node)
		size = int64(nodeSize)
	}
	return size, !c.walker.hasNext
}

// listSizeChunks is the size stream of a list of n elements: 100 for every
// full batch of 100 elements, then the number left over (the reference's
// ExMemoryUsage [a], which drops 100 elements at a time, so a list whose length
// is a multiple of 100 ends with a 0 element).
type listSizeChunks struct {
	left int64
}

func (c *listSizeChunks) next() (int64, bool) {
	if c.left >= 100 {
		c.left -= 100
		return 100, false
	}
	return c.left, true
}

// mapChunks applies f to every element (the reference's mapCostStream).
type mapChunks struct {
	src costChunks
	f   func(int64) int64
}

func (c *mapChunks) next() (int64, bool) {
	value, last := c.src.next()
	return c.f(value), last
}

// addChunks adds two streams (the reference's addCostStream): two
// one-element streams add into one element; otherwise the elements of the two
// streams are interleaved, starting with the left one, until one of them runs
// out, and the rest of the other follows.
type addChunks struct {
	cur, other *peekChunks
	state      uint8 // 0: not started, 1: interleaving, 2: draining other
}

func newAddChunks(left, right costChunks) *addChunks {
	return &addChunks{
		cur:   &peekChunks{src: left},
		other: &peekChunks{src: right},
	}
}

func (c *addChunks) next() (int64, bool) {
	if c.state == 0 {
		l, lLast := c.cur.peek()
		r, rLast := c.other.peek()
		if lLast && rLast {
			return satAdd(l, r), true
		}
		c.state = 1
	}
	if c.state == 2 {
		return c.other.next()
	}
	value, last := c.cur.next()
	if last {
		c.state = 2
	} else {
		c.cur, c.other = c.other, c.cur
	}
	return value, false
}

// minChunks is the element-wise minimum of two streams (the reference's
// minCostStream): it emits the smaller head, carries the difference over as
// the other stream's new head, and ends as soon as either stream does.
type minChunks struct {
	left, right minSide
}

type minSide struct {
	src      costChunks
	carry    int64
	hasCarry bool
	done     bool
}

// uncons returns the side's head and whether more elements follow it.
func (s *minSide) uncons() (int64, bool) {
	if s.hasCarry {
		s.hasCarry = false
		return s.carry, !s.done
	}
	value, last := s.src.next()
	s.done = last
	return value, !last
}

func (s *minSide) recons(value int64) {
	s.carry = value
	s.hasCarry = true
}

func newMinChunks(left, right costChunks) *minChunks {
	return &minChunks{left: minSide{src: left}, right: minSide{src: right}}
}

func (c *minChunks) next() (int64, bool) {
	l, lMore := c.left.uncons()
	r, rMore := c.right.uncons()
	switch {
	case l < r:
		c.right.recons(r - l)
		return l, !lMore
	case l > r:
		c.left.recons(l - r)
		return r, !rMore
	default:
		return l, !lMore || !rMore
	}
}

// quadraticChunks evaluates c0 + c1*x + c2*x^2 on a stream (the reference's
// scaleQuadratically): c0 first, then for each element x, with r the sum of
// the elements before it, 2*c2*x*r + c2*x*x + c1*x.
type quadraticChunks struct {
	src        *peekChunks
	c0, c1, c2 int64
	running    int64
	started    bool
}

func (c *quadraticChunks) term(x int64) int64 {
	a, b := c.c2, c.c1
	return satAdd(
		satAdd(satMul(satMul(satMul(2, a), x), c.running), satMul(satMul(a, x), x)),
		satMul(b, x),
	)
}

func (c *quadraticChunks) next() (int64, bool) {
	if !c.started {
		c.started = true
		x, last := c.src.peek()
		if last {
			c.src.next()
			return satAdd(c.c0, c.term(x)), true
		}
		return c.c0, false
	}
	x, last := c.src.next()
	out := c.term(x)
	c.running = satAdd(c.running, x)
	return out, last
}

// scaleLinearly is the reference's scaleLinearly: intercept + slope*stream.
func scaleLinearly(intercept, slope int64, src costChunks) costChunks {
	return newAddChunks(
		&singleChunk{value: intercept},
		&mapChunks{src: src, f: func(x int64) int64 { return satMul(slope, x) }},
	)
}

// oneArgumentChunks returns the reference's cost stream for a one-argument
// costing function, or false when the model is not one the reference streams.
func oneArgumentChunks(model OneArgument, x func() costChunks) (costChunks, bool) {
	switch mdl := model.(type) {
	case *ConstantCost:
		return &singleChunk{value: mdl.c}, true
	case *LinearCost:
		return scaleLinearly(mdl.intercept, mdl.slope, x()), true
	case *QuadraticInXModel:
		return &quadraticChunks{
			src: &peekChunks{src: x()},
			c0:  mdl.coeff0,
			c1:  mdl.coeff1,
			c2:  mdl.coeff2,
		}, true
	default:
		return nil, false
	}
}

// twoArgumentChunks is oneArgumentChunks for two-argument costing functions.
func twoArgumentChunks(
	model TwoArgument,
	x, y func() costChunks,
) (costChunks, bool) {
	switch mdl := model.(type) {
	case *ConstantCost:
		return &singleChunk{value: mdl.c}, true
	case *AddedSizesModel:
		return scaleLinearly(mdl.intercept, mdl.slope, newAddChunks(x(), y())), true
	case *MinSizeModel:
		return scaleLinearly(mdl.intercept, mdl.slope, newMinChunks(x(), y())), true
	case *LinearInX:
		return scaleLinearly(mdl.intercept, mdl.slope, x()), true
	case *LinearInY:
		return scaleLinearly(mdl.intercept, mdl.slope, y()), true
	default:
		return nil, false
	}
}

// threeArgumentChunks is oneArgumentChunks for three-argument costing
// functions that are linear in one argument.
func threeArgumentChunks(
	model ThreeArgument,
	x, y, z func() costChunks,
) (costChunks, bool) {
	switch mdl := model.(type) {
	case *ConstantCost:
		return &singleChunk{value: mdl.c}, true
	case *ThreeLinearInX:
		return scaleLinearly(mdl.intercept, mdl.slope, x()), true
	case *ThreeLinearInY:
		return scaleLinearly(mdl.intercept, mdl.slope, y()), true
	case *ThreeLinearInZ:
		return scaleLinearly(mdl.intercept, mdl.slope, z()), true
	default:
		return nil, false
	}
}

// singleChunkOf is the size stream of an argument whose size is one element.
func singleChunkOf(size ExMem) func() costChunks {
	return func() costChunks { return &singleChunk{value: int64(size)} }
}

// spendChunks charges a cost stream, given as separate CPU and memory
// streams, element by element from the budget before (the reference's
// zipCostStream and spendBudgets: the shorter stream is padded with zeros).
// It returns the BudgetError of the element that overdrew, with ExBudget set
// to the remaining budget after that element, or nil if the whole stream fit.
func (m *Machine[T]) spendChunks(before ExBudget, cpu, mem costChunks) error {
	m.ExBudget = before
	cpuDone, memDone := false, false
	for {
		var chunk ExBudget
		if !cpuDone {
			chunk.Cpu, cpuDone = cpu.next()
		}
		if !memDone {
			chunk.Mem, memDone = mem.next()
		}
		if err := m.spendBudget(chunk); err != nil {
			return err
		}
		if cpuDone && memDone {
			return nil
		}
	}
}
