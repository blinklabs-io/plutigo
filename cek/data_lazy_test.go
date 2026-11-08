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
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/internal/lazydata"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// Lists built by mkCons onto a constant list and onto a list taken out of a
// data value; each is [I 1, I 2, I 3].
const (
	lazyConstTail = "[(force (builtin mkCons)) (con data (I 1)) (con (list data) [I 2, I 3])]"
	lazyDataTail  = "[(force (builtin mkCons)) (con data (I 1)) [(builtin unListData) (con data (List [I 2, I 3]))]]"
	lazyPairs     = "[(force (builtin mkCons)) [(builtin mkPairData) (con data (I 1)) (con data (I 2))] (con (list (pair data data)) [(I 3, I 4)])]"
	eagerList     = "(con data (List [I 1, I 2, I 3]))"
)

func evalLazyDataProgram(t *testing.T, term string, major uint) string {
	t.Helper()
	ec := NewDefaultEvalContext(lang.LanguageVersionV3, ProtoVersion{Major: major})
	result, _, err := EvaluateText(
		context.Background(),
		"(program 1.1.0 "+term+")",
		lang.LanguageVersionV3,
		ec,
		DefaultExBudget,
	)
	if err != nil {
		t.Fatalf("evaluate %s: %v", term, err)
	}
	return syn.PrettyTerm[syn.DeBruijn](result)
}

// TestLazyDataMatchesEagerData checks that data built by listData, mapData and
// constrData over lists made by mkCons is observed exactly as the same data
// written out.
func TestLazyDataMatchesEagerData(t *testing.T) {
	tests := map[string]string{
		"listData equals":         "[(builtin equalsData) [(builtin listData) " + lazyConstTail + "] " + eagerList + "]",
		"listData of data equals": "[(builtin equalsData) [(builtin listData) " + lazyDataTail + "] " + eagerList + "]",
		"listData serialises": "[(builtin equalsByteString) [(builtin serialiseData) [(builtin listData) " + lazyDataTail + "]] " +
			"[(builtin serialiseData) " + eagerList + "]]",
		"constrData equals": "[(builtin equalsData) [(builtin constrData) (con integer 5) " + lazyConstTail + "] " +
			"(con data (Constr 5 [I 1, I 2, I 3]))]",
		"constrData serialises": "[(builtin equalsByteString) [(builtin serialiseData) [(builtin constrData) (con integer 5) " + lazyDataTail + "]] " +
			"[(builtin serialiseData) (con data (Constr 5 [I 1, I 2, I 3]))]]",
		"mapData equals": "[(builtin equalsData) [(builtin mapData) " + lazyPairs + "] " +
			"(con data (Map [(I 1, I 2), (I 3, I 4)]))]",
		"unListData tail": "[(builtin equalsData) [(builtin listData) [(force (builtin tailList)) [(builtin unListData) [(builtin listData) " + lazyConstTail + "]]]] " +
			"(con data (List [I 2, I 3]))]",
		"unConstrData fields": "[(builtin equalsData) [(builtin listData) [(force (force (builtin sndPair))) [(builtin unConstrData) [(builtin constrData) (con integer 5) " + lazyDataTail + "]]]] " +
			eagerList + "]",
		"unMapData head": "[(builtin equalsData) [(force (force (builtin fstPair))) [(force (builtin headList)) [(builtin unMapData) [(builtin mapData) " + lazyPairs + "]]]] " +
			"(con data (I 1))]",
		"nested": "[(builtin equalsData) [(builtin listData) [(force (builtin mkCons)) [(builtin listData) " + lazyConstTail + "] (con (list data) [])]] " +
			"(con data (List [List [I 1, I 2, I 3]]))]",
		"chooseData": "[(force (builtin chooseData)) [(builtin listData) " + lazyConstTail + "] " +
			"(con bool False) (con bool False) (con bool True) (con bool False) (con bool False)]",
	}
	for name, term := range tests {
		t.Run(name, func(t *testing.T) {
			if got := evalLazyDataProgram(t, term, 11); got != "(con bool True)" {
				t.Fatalf("%s = %s, want (con bool True)", term, got)
			}
		})
	}
	t.Run("case on data", func(t *testing.T) {
		term := "(case [(builtin constrData) (con integer 0) " + lazyConstTail + "] " +
			"(lam fs [(builtin equalsData) [(builtin listData) fs] " + eagerList + "]))"
		if got := evalLazyDataProgram(t, term, 12); got != "(con bool True)" {
			t.Fatalf("%s = %s, want (con bool True)", term, got)
		}
	})
}

// TestLazyDataResultIsPlainData checks that a result built over a list made
// by mkCons reaches the caller as plain data, with no evaluator storage.
func TestLazyDataResultIsPlainData(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
	program, err := syn.Parse("(program 1.1.0 [(builtin listData) [(force (builtin mkCons)) [(builtin listData) " +
		lazyConstTail + "] (con (list data) [])]])")
	if err != nil {
		t.Fatal(err)
	}
	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Run(dbProgram.Term)
	if err != nil {
		t.Fatal(err)
	}
	constant, ok := result.(*syn.Constant)
	if !ok {
		t.Fatalf("result is %T", result)
	}
	outer := constant.Con.(*syn.Data).Inner.(*data.List)
	if lazydata.Hooks.SourceOf(outer) != nil {
		t.Fatal("result list still holds evaluator storage")
	}
	inner, ok := outer.Items()[0].(*data.List)
	if !ok {
		t.Fatalf("result item is %T", outer.Items()[0])
	}
	if lazydata.Hooks.SourceOf(inner) != nil {
		t.Fatal("nested result list still holds evaluator storage")
	}
	if got := inner.String(); got != "List[Integer(1) Integer(2) Integer(3)]" {
		t.Fatalf("nested result = %s", got)
	}
}

// TestIllTypedListConstantIsRejected checks that a list constant whose
// elements do not have its declared element type is rejected before
// evaluation, so listData never wraps one.
func TestIllTypedListConstantIsRejected(t *testing.T) {
	illTyped := &syn.Constant{
		Con: syn.WrapProtoList(&syn.TData{},
			[]syn.IConstant{&syn.Integer{Inner: big.NewInt(1)}}),
	}
	terms := map[string]syn.Term[syn.DeBruijn]{
		"constant": illTyped,
		"listData of mkCons": &syn.Apply[syn.DeBruijn]{
			Function: &syn.Builtin{DefaultFunction: builtin.ListData},
			Argument: &syn.Apply[syn.DeBruijn]{
				Function: &syn.Apply[syn.DeBruijn]{
					Function: &syn.Force[syn.DeBruijn]{Term: &syn.Builtin{DefaultFunction: builtin.MkCons}},
					Argument: &syn.Constant{Con: &syn.Data{Inner: data.NewInteger(big.NewInt(0))}},
				},
				Argument: illTyped,
			},
		},
	}
	for name, term := range terms {
		t.Run(name, func(t *testing.T) {
			m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
			_, err := m.Run(term)
			var typeErr *TypeError
			if !errors.As(err, &typeErr) {
				t.Fatalf("Run = %v, want a TypeError", err)
			}
		})
	}
}

// TestLazyListConstantsMatchEager checks that lists of lists built by
// mkCons, whose elements share their cells, evaluate and discharge exactly as
// the same lists written out (want, evaluated as a constant).
func TestLazyListConstantsMatchEager(t *testing.T) {
	const (
		intHead  = "[(force (builtin mkCons)) (con integer 1) (con (list integer) [2, 3])]"
		dataHead = "[(force (builtin mkCons)) (con data (I 1)) [(builtin unListData) (con data (List [I 2]))]]"
		mapHead  = "[(force (builtin mkCons)) [(builtin mkPairData) (con data (I 1)) (con data (I 2))] [(builtin unMapData) (con data (Map [(I 3, I 4)]))]]"
	)
	cons := func(head, tail string) string {
		return "[(force (builtin mkCons)) " + head + " " + tail + "]"
	}
	tests := []struct {
		name  string
		term  string
		major uint
		want  string
	}{
		{
			"list of integer lists", cons(intHead, "(con (list (list integer)) [])"), 11,
			"(con (list (list integer)) [[1, 2, 3]])",
		},
		{
			"list of data lists", cons(dataHead, "(con (list (list data)) [])"), 11,
			"(con (list (list data)) [[I 1, I 2]])",
		},
		{
			"list of maps", cons(mapHead, "(con (list (list (pair data data))) [])"), 11,
			"(con (list (list (pair data data))) [[(I 1, I 2), (I 3, I 4)]])",
		},
		{"head read back", "[(force (builtin headList)) [(force (builtin tailList)) [(force (builtin headList)) " +
			cons(intHead, "(con (list (list integer)) [])") + "]]]", 11, "(con integer 2)"},
		{"data head read back", "[(builtin listData) [(force (builtin headList)) " +
			cons(dataHead, "(con (list (list data)) [])") + "]]", 11, "(con data (List [I 1, I 2]))"},
		{"case head", "(case " + cons(intHead, "(con (list (list integer)) [])") +
			" (lam h (lam tl [(force (builtin headList)) h])))", 11, "(con integer 1)"},
		{
			"nested twice", cons(cons(intHead, "(con (list (list integer)) [])"), "(con (list (list (list integer))) [])"), 11,
			"(con (list (list (list integer))) [[[1, 2, 3]]])",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := evalLazyDataProgram(t, tt.want, tt.major)
			if got := evalLazyDataProgram(t, tt.term, tt.major); got != want {
				t.Fatalf("%s = %s, want %s", tt.term, got, want)
			}
		})
	}
}

// TestLazyListConstantResultIsPlain checks that a list of lists built by
// mkCons reaches the caller with no evaluator storage in its elements.
func TestLazyListConstantResultIsPlain(t *testing.T) {
	m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
	program, err := syn.Parse("(program 1.1.0 [(force (builtin mkCons)) [(force (builtin mkCons)) (con integer 1) (con (list integer) [2])] (con (list (list integer)) [])])")
	if err != nil {
		t.Fatal(err)
	}
	dbProgram, err := syn.NameToDeBruijn(program)
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Run(dbProgram.Term)
	if err != nil {
		t.Fatal(err)
	}
	outer := result.(*syn.Constant).Con.(*syn.ProtoList)
	if lazydata.Hooks.ProtoListSource(outer) != nil {
		t.Fatal("result list holds evaluator storage")
	}
	inner, ok := outer.List()[0].(*syn.ProtoList)
	if !ok {
		t.Fatalf("result element is %T", outer.List()[0])
	}
	if lazydata.Hooks.ProtoListSource(inner) != nil {
		t.Fatal("result element holds evaluator storage")
	}
}

// TestTrustedIllTypedListConstantIsTypeError checks that data builtins over a
// list made by mkCons onto an ill-typed constant report a TypeError, as they
// do over the same list unwrapped, when term validation is skipped.
func TestTrustedIllTypedListConstantIsTypeError(t *testing.T) {
	apply := func(f syn.Term[syn.DeBruijn], args ...syn.Term[syn.DeBruijn]) syn.Term[syn.DeBruijn] {
		for _, a := range args {
			f = &syn.Apply[syn.DeBruijn]{Function: f, Argument: a}
		}
		return f
	}
	cons := &syn.Force[syn.DeBruijn]{Term: &syn.Builtin{DefaultFunction: builtin.MkCons}}
	dataHead := &syn.Constant{Con: &syn.Data{Inner: data.NewInteger(big.NewInt(0))}}
	pairHead := apply(
		&syn.Builtin{DefaultFunction: builtin.MkPairData}, dataHead, dataHead)
	intItem := &syn.Integer{Inner: big.NewInt(1)}
	badData := &syn.Constant{Con: syn.WrapProtoList(&syn.TData{}, []syn.IConstant{intItem})}
	pairType := &syn.TPair{First: &syn.TData{}, Second: &syn.TData{}}
	badPairs := &syn.Constant{Con: syn.WrapProtoList(pairType, []syn.IConstant{intItem})}
	tag := &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(0)}}

	listOf := apply(cons, dataHead, badData)
	tests := map[string]syn.Term[syn.DeBruijn]{
		"listData":   apply(&syn.Builtin{DefaultFunction: builtin.ListData}, listOf),
		"constrData": apply(&syn.Builtin{DefaultFunction: builtin.ConstrData}, tag, listOf),
		"mapData": apply(&syn.Builtin{DefaultFunction: builtin.MapData},
			apply(cons, pairHead, badPairs)),
		"listData two cells": apply(&syn.Builtin{DefaultFunction: builtin.ListData},
			apply(cons, dataHead, listOf)),
	}
	for name, term := range tests {
		t.Run(name, func(t *testing.T) {
			m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
			_, err := m.runContext(context.Background(), false, term, nil, true)
			var typeErr *TypeError
			if !errors.As(err, &typeErr) {
				t.Fatalf("runContext = %v, want a TypeError", err)
			}
		})
	}
}

// TestConsOntoCheckedTailChecksNewHead checks that a cell consed onto a list
// already checked by mapData is itself checked, so a malformed head is a
// TypeError as it is over the same list without the earlier check.
func TestConsOntoCheckedTailChecksNewHead(t *testing.T) {
	cons := &syn.Force[syn.DeBruijn]{Term: &syn.Builtin{DefaultFunction: builtin.MkCons}}
	app := func(f syn.Term[syn.DeBruijn], a syn.Term[syn.DeBruijn]) syn.Term[syn.DeBruijn] {
		return &syn.Apply[syn.DeBruijn]{Function: f, Argument: a}
	}
	dataHead := &syn.Constant{Con: &syn.Data{Inner: data.NewInteger(big.NewInt(0))}}
	goodPair := app(app(&syn.Builtin{DefaultFunction: builtin.MkPairData}, dataHead), dataHead)
	pairType := &syn.TPair{First: &syn.TData{}, Second: &syn.TData{}}
	goodPairs := &syn.Constant{Con: syn.WrapProtoList(pairType, []syn.IConstant{
		&syn.ProtoPair{
			FstType: &syn.TData{}, SndType: &syn.TData{},
			First:  &syn.Data{Inner: data.NewInteger(big.NewInt(1))},
			Second: &syn.Data{Inner: data.NewInteger(big.NewInt(2))},
		},
	})}
	badPair := &syn.Constant{Con: &syn.ProtoPair{
		FstType: &syn.TData{}, SndType: &syn.TData{},
		First:  &syn.Integer{Inner: big.NewInt(1)},
		Second: &syn.Data{Inner: data.NewInteger(big.NewInt(2))},
	}}
	mapData := &syn.Builtin{DefaultFunction: builtin.MapData}
	shared := &syn.Var[syn.DeBruijn]{Name: 1}
	sharedInner := &syn.Var[syn.DeBruijn]{Name: 2}
	checkedFirst := app(
		&syn.Lambda[syn.DeBruijn]{Body: app(
			&syn.Lambda[syn.DeBruijn]{Body: app(mapData, app(app(cons, badPair), sharedInner))},
			app(mapData, shared),
		)},
		app(app(cons, goodPair), goodPairs),
	)
	unchecked := app(mapData, app(app(cons, badPair), app(app(cons, goodPair), goodPairs)))
	for name, term := range map[string]syn.Term[syn.DeBruijn]{
		"checked tail":   checkedFirst,
		"unchecked tail": unchecked,
	} {
		t.Run(name, func(t *testing.T) {
			m := NewMachine[syn.DeBruijn](lang.LanguageVersionV3, 0, testEvalContext())
			_, err := m.runContext(context.Background(), false, term, nil, true)
			var typeErr *TypeError
			if !errors.As(err, &typeErr) {
				t.Fatalf("runContext = %v, want a TypeError", err)
			}
		})
	}
}
