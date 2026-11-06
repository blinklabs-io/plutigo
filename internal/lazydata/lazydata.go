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

// Package lazydata lets the evaluator build PlutusData lists, maps and
// constructor fields, and list constants, over its own persistent lists
// without copying them, and recover those lists again.
//
// A Source is not safe for concurrent use. Data holding one must not leave
// the evaluation that built it: the evaluator hands results to its caller
// as copies (data.CloneContext), which hold plain slices.
package lazydata

import "math/big"

// Token seals Source: only packages that can import lazydata can name it.
type Token struct{}

// Source is an immutable sequence of n >= 1 elements whose contiguous form is
// built on demand.
type Source[E any] interface {
	Len() int
	// Materialize returns the elements as a slice the caller must not
	// modify. Repeated calls return the same slice.
	Materialize() []E
	Seal(Token)
}

// Hooks are set by packages data and syn. The arguments and results are the
// types named in each comment.
var Hooks struct {
	// NewList returns a *data.List over a Source[data.PlutusData].
	NewList func(src any) any
	// NewMap returns a *data.Map over a Source[[2]data.PlutusData].
	NewMap func(src any) any
	// NewConstr returns a *data.Constr with the given tag over a
	// Source[data.PlutusData].
	NewConstr func(tag *big.Int, src any) any
	// SourceOf returns the Source behind a *data.List, *data.Map or
	// *data.Constr, or nil.
	SourceOf func(pd any) any
	// NewProtoList returns a *syn.ProtoList with element type typ, a
	// syn.Typ, over a Source[syn.IConstant]. It is set by package syn.
	NewProtoList func(typ any, src any) any
	// ProtoListSource returns the Source behind a *syn.ProtoList, or nil.
	// It is set by package syn.
	ProtoListSource func(pl any) any
}
