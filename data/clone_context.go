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
	"context"
	"errors"
	"fmt"
	"math/big"
)

type cloneContextItem struct {
	source PlutusData
	target *PlutusData
	depth  int
}

type cloneContextState struct {
	ctx       context.Context
	maxDepth  int
	maxNodes  int
	remaining int
	work      []cloneContextItem
}

func (s *cloneContextState) checkAdditionalNodes(nodes int) error {
	if nodes < 0 || nodes > s.remaining {
		return &DecodeLimitError{
			Limit:  "clone node count",
			Max:    s.maxNodes,
			Actual: s.maxNodes + 1,
		}
	}
	return nil
}

func (s *cloneContextState) schedule(
	source PlutusData,
	target *PlutusData,
	depth int,
) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if depth > s.maxDepth {
		return &DecodeLimitError{
			Limit:  "clone nesting depth",
			Max:    s.maxDepth,
			Actual: depth,
		}
	}
	if err := s.checkAdditionalNodes(1); err != nil {
		return err
	}
	s.remaining--
	s.work = append(s.work, cloneContextItem{
		source: source,
		target: target,
		depth:  depth,
	})
	return nil
}

func (s *cloneContextState) cloneMap(
	source *Map,
	target *Map,
	depth int,
	chargeRoot bool,
) error {
	if chargeRoot {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		if depth > s.maxDepth {
			return &DecodeLimitError{
				Limit:  "clone nesting depth",
				Max:    s.maxDepth,
				Actual: depth,
			}
		}
		if err := s.checkAdditionalNodes(1); err != nil {
			return err
		}
		s.remaining--
	}
	if len(source.Pairs) > s.remaining/2 {
		return &DecodeLimitError{
			Limit:  "clone node count",
			Max:    s.maxNodes,
			Actual: s.maxNodes + 1,
		}
	}
	target.Pairs = make([][2]PlutusData, len(source.Pairs))
	target.useIndef = cloneUseIndef(source.useIndef)
	for i := len(source.Pairs) - 1; i >= 0; i-- {
		if err := s.schedule(source.Pairs[i][1], &target.Pairs[i][1], depth+1); err != nil {
			return err
		}
		if err := s.schedule(source.Pairs[i][0], &target.Pairs[i][0], depth+1); err != nil {
			return err
		}
	}
	return nil
}

// CloneContext copies a PlutusData tree while honoring cancellation and
// bounding the nodes and nesting processed by the copy.
func CloneContext(
	ctx context.Context,
	value PlutusData,
	maxDepth int,
	maxNodes int,
) (PlutusData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if maxDepth < 0 || maxNodes < 1 {
		return nil, fmt.Errorf(
			"invalid PlutusData clone limits: depth=%d nodes=%d",
			maxDepth,
			maxNodes,
		)
	}

	var result PlutusData
	state := cloneContextState{
		ctx:       ctx,
		maxDepth:  maxDepth,
		maxNodes:  maxNodes,
		remaining: maxNodes,
		work:      make([]cloneContextItem, 0, 1),
	}
	if err := state.schedule(value, &result, 0); err != nil {
		return nil, err
	}

	for len(state.work) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		last := len(state.work) - 1
		item := state.work[last]
		state.work = state.work[:last]

		switch source := item.source.(type) {
		case *Constr:
			if source == nil {
				return nil, errors.New("cannot clone a nil PlutusData constructor")
			}
			if err := state.checkAdditionalNodes(len(source.Fields)); err != nil {
				return nil, err
			}
			tag := new(big.Int)
			if source.Tag != nil {
				tag.Set(source.Tag)
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			cloned := &Constr{
				Tag:      tag,
				Fields:   make([]PlutusData, len(source.Fields)),
				useIndef: cloneUseIndef(source.useIndef),
			}
			*item.target = cloned
			for i := len(source.Fields) - 1; i >= 0; i-- {
				if err := state.schedule(source.Fields[i], &cloned.Fields[i], item.depth+1); err != nil {
					return nil, err
				}
			}
		case Constr:
			copy := source
			state.work = append(state.work, cloneContextItem{
				source: &copy,
				target: item.target,
				depth:  item.depth,
			})
		case *Map:
			if source == nil {
				return nil, errors.New("cannot clone a nil PlutusData map")
			}
			cloned := &Map{}
			*item.target = cloned
			if err := state.cloneMap(source, cloned, item.depth, false); err != nil {
				return nil, err
			}
		case Map:
			copy := source
			state.work = append(state.work, cloneContextItem{
				source: &copy,
				target: item.target,
				depth:  item.depth,
			})
		case *Value:
			if source == nil {
				return nil, errors.New("cannot clone a nil PlutusData Value")
			}
			cloned := &Value{}
			*item.target = cloned
			if source.Inner != nil {
				cloned.Inner = &Map{}
				if err := state.cloneMap(source.Inner, cloned.Inner, item.depth+1, true); err != nil {
					return nil, err
				}
			}
		case Value:
			copy := source
			state.work = append(state.work, cloneContextItem{
				source: &copy,
				target: item.target,
				depth:  item.depth,
			})
		case *Integer:
			if source == nil || source.Inner == nil {
				return nil, errors.New("cannot clone a nil PlutusData Integer")
			}
			cloned := new(big.Int).Set(source.Inner)
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			*item.target = &Integer{Inner: cloned}
		case Integer:
			if source.Inner == nil {
				return nil, errors.New("cannot clone a nil PlutusData Integer")
			}
			cloned := new(big.Int).Set(source.Inner)
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			*item.target = &Integer{Inner: cloned}
		case *ByteString:
			if source == nil {
				return nil, errors.New("cannot clone a nil PlutusData ByteString")
			}
			cloned, err := cloneByteSliceContext(ctx, source.Inner)
			if err != nil {
				return nil, err
			}
			*item.target = &ByteString{Inner: cloned}
		case ByteString:
			cloned, err := cloneByteSliceContext(ctx, source.Inner)
			if err != nil {
				return nil, err
			}
			*item.target = &ByteString{Inner: cloned}
		case *List:
			if source == nil {
				return nil, errors.New("cannot clone a nil PlutusData list")
			}
			if err := state.checkAdditionalNodes(len(source.Items)); err != nil {
				return nil, err
			}
			cloned := &List{
				Items:    make([]PlutusData, len(source.Items)),
				useIndef: cloneUseIndef(source.useIndef),
			}
			*item.target = cloned
			for i := len(source.Items) - 1; i >= 0; i-- {
				if err := state.schedule(source.Items[i], &cloned.Items[i], item.depth+1); err != nil {
					return nil, err
				}
			}
		case List:
			copy := source
			state.work = append(state.work, cloneContextItem{
				source: &copy,
				target: item.target,
				depth:  item.depth,
			})
		default:
			return nil, fmt.Errorf("unsupported PlutusData type %T", item.source)
		}
	}
	return result, nil
}

const cloneByteSliceChunkSize = 64 * 1024

func cloneByteSliceContext(ctx context.Context, source []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, nil
	}
	target := make([]byte, len(source))
	for offset := 0; offset < len(source); offset += cloneByteSliceChunkSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		copy(
			target[offset:min(offset+cloneByteSliceChunkSize, len(source))],
			source[offset:],
		)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return target, nil
}
