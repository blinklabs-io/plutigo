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
	"math/big"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/internal/lazydata"
	"github.com/blinklabs-io/plutigo/syn"
)

// listData, mapData and constrData are costed as constants, as in the
// reference, where they wrap a persistent list in O(1). For a list built by
// mkCons they therefore wrap its cons cells as a lazydata.Source instead of
// copying them; the contiguous elements are built only when something costed
// by size (equalsData, serialiseData, a result copy) reads them. unListData,
// unMapData, unConstrData and case on Data hand the cells back as a list
// value, so taking such data apart again is O(1) as well. mkCons with a
// list-valued head does the same for the list constant it stores, and
// reading that element back yields the cells.

// dataCellSource is a data list built by mkCons onto a list of data.
type dataCellSource struct {
	cell *consCell[data.PlutusData]
}

func (s *dataCellSource) Len() int { return s.cell.n }

func (s *dataCellSource) Materialize() []data.PlutusData {
	flat, _ := s.cell.flatten()
	return flat
}

func (*dataCellSource) Seal(lazydata.Token) {}

// pairCellSource is a data map built by mkCons onto a list of data pairs.
type pairCellSource struct {
	cell *consCell[[2]data.PlutusData]
}

func (s *pairCellSource) Len() int { return s.cell.n }

func (s *pairCellSource) Materialize() [][2]data.PlutusData {
	flat, _ := s.cell.flatten()
	return flat
}

func (*pairCellSource) Seal(lazydata.Token) {}

// convCellSource is a list built by mkCons whose elements, of type S, become
// elements of type D on first use; a nil conv means S and D are the same.
type convCellSource[S, D any] struct {
	typ  syn.Typ
	cell *consCell[S]
	conv func(S) (D, error)
	out  []D
}

func (s *convCellSource[S, D]) Len() int { return s.cell.n }

func (s *convCellSource[S, D]) Materialize() []D {
	if s.out == nil {
		flat, buf := s.cell.flatten()
		if s.conv == nil {
			s.out = any(flat).([]D)
			return s.out
		}
		var (
			out []D
			err error
		)
		if buf != nil {
			out, err = convertList[S, D](nil, buf, flat, s.conv)
		} else {
			out, err = convertSuffix(&convBuf[D]{}, flat, s.conv)
		}
		if err != nil {
			// lazyDataItems and lazyDataPairs convert every element
			// before building the source, so this cannot fail.
			panic(err)
		}
		s.out = out
	}
	return s.out
}

func (*convCellSource[S, D]) Seal(lazydata.Token) {}

// constCellSource is a constant list of data, or of data pairs, built by
// mkCons; its elements are converted on first use.
type constCellSource[D any] = convCellSource[syn.IConstant, D]

// checkConstCells converts every element of the list starting at cell with
// conv, so that a later lazy conversion cannot fail. Cells found converted
// are skipped, and the cells walked are marked once the whole list passes,
// so a chain built by mkCons is checked once.
func checkConstCells[D any](
	cell *consCell[syn.IConstant],
	conv func(syn.IConstant) (D, error),
) error {
	for c := cell; c != nil && !c.checked; c = c.next {
		if _, err := conv(c.head); err != nil {
			return err
		}
		if c.next == nil {
			for _, item := range c.base {
				if _, err := conv(item); err != nil {
					return err
				}
			}
		}
	}
	for c := cell; c != nil && !c.checked; c = c.next {
		c.checked = true
	}
	return nil
}

// lazyDataItems returns a source over value when it is a list of data built
// by mkCons, or nil. The elements of a constant list are converted first so
// that an element of the wrong type is reported as the eager path reports it.
func lazyDataItems[T syn.Eval](
	value Value[T],
	conv func(syn.IConstant) (data.PlutusData, error),
) (lazydata.Source[data.PlutusData], error) {
	switch v := value.(type) {
	case *consDataList[T]:
		return &dataCellSource{cell: v.cell}, nil
	case *consConstList[T]:
		if syn.EqualType(v.typ, sharedDataType) {
			if err := checkConstCells(v.cell, conv); err != nil {
				return nil, err
			}
			return &constCellSource[data.PlutusData]{
				typ:  v.typ,
				cell: v.cell,
				conv: conv,
			}, nil
		}
	}
	return nil, nil
}

// lazyDataPairs returns a source over value when it is a list of data pairs
// built by mkCons, or nil. Like lazyDataItems it converts the elements of a
// constant list first.
func lazyDataPairs[T syn.Eval](
	value Value[T],
) (lazydata.Source[[2]data.PlutusData], error) {
	switch v := value.(type) {
	case *consDataMap[T]:
		return &pairCellSource{cell: v.cell}, nil
	case *consConstList[T]:
		if syn.EqualType(v.typ, sharedPairDataType) {
			if err := checkConstCells(v.cell, mapDataElem); err != nil {
				return nil, err
			}
			return &constCellSource[[2]data.PlutusData]{
				typ:  v.typ,
				cell: v.cell,
				conv: mapDataElem,
			}, nil
		}
	}
	return nil, nil
}

func newLazyList(src lazydata.Source[data.PlutusData]) *data.List {
	return lazydata.Hooks.NewList(src).(*data.List)
}

func newLazyMap(src lazydata.Source[[2]data.PlutusData]) *data.Map {
	return lazydata.Hooks.NewMap(src).(*data.Map)
}

func newLazyConstr(
	tag *big.Int,
	src lazydata.Source[data.PlutusData],
) *data.Constr {
	return lazydata.Hooks.NewConstr(tag, src).(*data.Constr)
}

// lazyListValue returns the list value behind pd, a *data.List, *data.Map or
// *data.Constr that listData, mapData or constrData built over a list made by
// mkCons.
func lazyListValueOf[T syn.Eval](pd data.PlutusData) (Value[T], bool) {
	switch s := lazydata.Hooks.SourceOf(pd).(type) {
	case *dataCellSource:
		return &consDataList[T]{cell: s.cell}, true
	case *pairCellSource:
		return &consDataMap[T]{cell: s.cell}, true
	case *constCellSource[data.PlutusData]:
		return &consConstList[T]{typ: s.typ, cell: s.cell}, true
	case *constCellSource[[2]data.PlutusData]:
		return &consConstList[T]{typ: s.typ, cell: s.cell}, true
	}
	return nil, false
}

// dataItemsValue returns the items of a data list as a list value.
func (m *Machine[T]) dataItemsValue(list *data.List) Value[T] {
	if v, ok := lazyListValueOf[T](list); ok {
		return v
	}
	return m.allocDataListValue(list.Items())
}

// dataFieldsValue returns the fields of a constructor as a list value.
func (m *Machine[T]) dataFieldsValue(constr *data.Constr) Value[T] {
	if v, ok := lazyListValueOf[T](constr); ok {
		return v
	}
	return m.allocDataListValue(constr.Fields())
}

// dataPairsValue returns the pairs of a data map as a list value.
func (m *Machine[T]) dataPairsValue(dataMap *data.Map) Value[T] {
	if v, ok := lazyListValueOf[T](dataMap); ok {
		return v
	}
	return m.allocDataMapValue(dataMap.Pairs())
}

// cellIter walks a list built by mkCons, cell by cell and then its base.
type cellIter[E any] struct {
	cell *consCell[E]
	base []E
	i    int
}

func (it *cellIter[E]) next() (E, bool) {
	if c := it.cell; c != nil {
		if c.next != nil {
			it.cell = c.next
		} else {
			it.cell, it.base = nil, c.base
		}
		return c.head, true
	}
	if it.i < len(it.base) {
		it.i++
		return it.base[it.i-1], true
	}
	var zero E
	return zero, false
}

// lazyDataCursor yields the children of a data container built over a list
// made by mkCons in order (a map's pairs as key, value), without building
// the container's contiguous elements, so walks that size data stay
// proportional to the nodes they visit.
type lazyDataCursor struct {
	items   *cellIter[data.PlutusData]
	pairs   *cellIter[[2]data.PlutusData]
	consts  *cellIter[syn.IConstant]
	pending data.PlutusData
}

func newLazyDataCursor(pd data.PlutusData) *lazyDataCursor {
	switch s := lazydata.Hooks.SourceOf(pd).(type) {
	case *dataCellSource:
		return &lazyDataCursor{items: &cellIter[data.PlutusData]{cell: s.cell}}
	case *pairCellSource:
		return &lazyDataCursor{pairs: &cellIter[[2]data.PlutusData]{cell: s.cell}}
	case *constCellSource[data.PlutusData]:
		return &lazyDataCursor{consts: &cellIter[syn.IConstant]{cell: s.cell}}
	case *constCellSource[[2]data.PlutusData]:
		return &lazyDataCursor{consts: &cellIter[syn.IConstant]{cell: s.cell}}
	}
	return nil
}

func (c *lazyDataCursor) next() (data.PlutusData, bool) {
	if c.pending != nil {
		pd := c.pending
		c.pending = nil
		return pd, true
	}
	switch {
	case c.items != nil:
		return c.items.next()
	case c.pairs != nil:
		pair, ok := c.pairs.next()
		if !ok {
			return nil, false
		}
		c.pending = pair[1]
		return pair[0], true
	case c.consts != nil:
		item, ok := c.consts.next()
		if !ok {
			return nil, false
		}
		switch v := item.(type) {
		case *syn.Data:
			return v.Inner, true
		case *syn.ProtoPair:
			first, _ := v.First.(*syn.Data)
			second, _ := v.Second.(*syn.Data)
			if first == nil || second == nil {
				return nil, false
			}
			c.pending = second.Inner
			return first.Inner, true
		}
	}
	return nil, false
}

// lazyListConstant returns a list constant over a list value built by
// mkCons, as mkCons needs for a list-valued head, without building its
// contiguous elements.
func lazyListConstant[T syn.Eval](value Value[T]) (*syn.ProtoList, bool) {
	var (
		typ syn.Typ
		src lazydata.Source[syn.IConstant]
	)
	switch v := value.(type) {
	case *consConstList[T]:
		typ = v.typ
		src = &convCellSource[syn.IConstant, syn.IConstant]{typ: v.typ, cell: v.cell}
	case *consDataList[T]:
		typ = sharedDataType
		src = &convCellSource[data.PlutusData, syn.IConstant]{
			typ:  typ,
			cell: v.cell,
			conv: dataItemConstant,
		}
	case *consDataMap[T]:
		typ = sharedPairDataType
		src = &convCellSource[[2]data.PlutusData, syn.IConstant]{
			typ:  typ,
			cell: v.cell,
			conv: dataPairConstant,
		}
	default:
		return nil, false
	}
	return lazydata.Hooks.NewProtoList(typ, src).(*syn.ProtoList), true
}

// lazyListConstantValue returns the list value behind a list constant built
// by lazyListConstant.
func lazyListConstantValue[T syn.Eval](c syn.IConstant) (Value[T], bool) {
	pl, ok := c.(*syn.ProtoList)
	if !ok {
		return nil, false
	}
	switch s := lazydata.Hooks.ProtoListSource(pl).(type) {
	case *convCellSource[syn.IConstant, syn.IConstant]:
		return &consConstList[T]{typ: s.typ, cell: s.cell}, true
	case *convCellSource[data.PlutusData, syn.IConstant]:
		return &consDataList[T]{cell: s.cell}, true
	case *convCellSource[[2]data.PlutusData, syn.IConstant]:
		return &consDataMap[T]{cell: s.cell}, true
	}
	return nil, false
}

// elementValue returns a list or pair element as a value.
func (m *Machine[T]) elementValue(c syn.IConstant) Value[T] {
	if v, ok := lazyListConstantValue[T](c); ok {
		return v
	}
	return m.allocConstant(c)
}
