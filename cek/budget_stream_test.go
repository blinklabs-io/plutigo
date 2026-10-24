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
	"errors"
	"math/big"
	"slices"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
)

// sliceChunks is a test stream over fixed elements.
type sliceChunks struct {
	values []int64
}

func (s *sliceChunks) next() (int64, bool) {
	v := s.values[0]
	s.values = s.values[1:]
	return v, len(s.values) == 0
}

func chunksOf(values ...int64) func() costChunks {
	return func() costChunks { return &sliceChunks{values: slices.Clone(values)} }
}

func drain(c costChunks) []int64 {
	var out []int64
	for {
		v, last := c.next()
		out = append(out, v)
		if last {
			return out
		}
	}
}

// The expected streams follow the reference's definitions in
// PlutusCore.Evaluation.Machine.CostStream and CostingFun.Core.
func TestCostStreamCombinators(t *testing.T) {
	tests := []struct {
		name   string
		stream costChunks
		want   []int64
	}{
		{"add singles", newAddChunks(chunksOf(1)(), chunksOf(2)()), []int64{3}},
		{"add long left", newAddChunks(chunksOf(1, 2, 3)(), chunksOf(10)()), []int64{1, 10, 2, 3}},
		{"add single left", newAddChunks(chunksOf(1)(), chunksOf(10, 20)()), []int64{1, 10, 20}},
		{"add interleave", newAddChunks(chunksOf(1, 2)(), chunksOf(10, 20, 30)()), []int64{1, 10, 2, 20, 30}},
		{"min singles", newMinChunks(chunksOf(5)(), chunksOf(3)()), []int64{3}},
		{"min carries difference", newMinChunks(chunksOf(4, 4)(), chunksOf(3, 10)()), []int64{3, 1, 4}},
		{"min ends with shorter", newMinChunks(chunksOf(4)(), chunksOf(4, 1)()), []int64{4}},
		{"min right shorter", newMinChunks(chunksOf(1, 1, 1)(), chunksOf(2)()), []int64{1, 1}},
		{"linear single", scaleLinearly(10, 2, chunksOf(3)()), []int64{16}},
		{"linear stream", scaleLinearly(10, 2, chunksOf(3, 4)()), []int64{10, 6, 8}},
		{
			"quadratic single",
			&quadraticChunks{src: &peekChunks{src: chunksOf(5)()}, c0: 1, c1: 2, c2: 3},
			[]int64{86},
		},
		{
			"quadratic stream",
			&quadraticChunks{src: &peekChunks{src: chunksOf(1, 2)()}, c0: 1, c1: 2, c2: 3},
			[]int64{1, 5, 28},
		},
		{"list empty", &listSizeChunks{left: 0}, []int64{0}},
		{"list short", &listSizeChunks{left: 99}, []int64{99}},
		{"list one batch", &listSizeChunks{left: 100}, []int64{100, 0}},
		{"list batches", &listSizeChunks{left: 250}, []int64{100, 100, 50}},
		{
			"data preorder",
			newDataSizeChunks(data.NewConstr(0,
				data.NewList(data.NewInteger(big.NewInt(0))),
				data.NewMap([][2]data.PlutusData{{
					data.NewInteger(big.NewInt(1)),
					data.NewByteString(make([]byte, 9)),
				}}),
			), false),
			[]int64{4, 4, 5, 4, 5, 6},
		},
		{
			"data node count",
			newDataSizeChunks(data.NewList(data.NewInteger(big.NewInt(7)), data.NewList()), true),
			[]int64{1, 1, 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := drain(tt.stream); !slices.Equal(got, tt.want) {
				t.Fatalf("stream = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpendChunksStopsAtOverdraw(t *testing.T) {
	tests := []struct {
		name          string
		before        ExBudget
		cpu, mem      []int64
		wantRemaining ExBudget
		wantRequested ExBudget
		wantErr       bool
	}{
		{
			name:          "fits",
			before:        ExBudget{Cpu: 24, Mem: 1},
			cpu:           []int64{10, 6, 8},
			mem:           []int64{1},
			wantRemaining: ExBudget{Cpu: 0, Mem: 0},
		},
		{
			name:          "cpu overdraws on the second chunk",
			before:        ExBudget{Cpu: 15, Mem: 100},
			cpu:           []int64{10, 6, 8},
			mem:           []int64{1},
			wantRemaining: ExBudget{Cpu: -1, Mem: 99},
			wantRequested: ExBudget{Cpu: 6},
			wantErr:       true,
		},
		{
			name:          "mem overdraws on the padded tail",
			before:        ExBudget{Cpu: 100, Mem: 3},
			cpu:           []int64{1},
			mem:           []int64{2, 2, 2},
			wantRemaining: ExBudget{Cpu: 99, Mem: -1},
			wantRequested: ExBudget{Mem: 2},
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			err := m.spendChunks(tt.before, chunksOf(tt.cpu...)(), chunksOf(tt.mem...)())
			if m.ExBudget != tt.wantRemaining {
				t.Fatalf("remaining = %+v, want %+v", m.ExBudget, tt.wantRemaining)
			}
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			var budgetErr *BudgetError
			if !errors.As(err, &budgetErr) {
				t.Fatalf("error = %v, want BudgetError", err)
			}
			if budgetErr.Requested != tt.wantRequested {
				t.Fatalf("requested = %+v, want %+v", budgetErr.Requested, tt.wantRequested)
			}
			if got := budgetErr.Available.Sub(&budgetErr.Requested); got != m.ExBudget {
				t.Fatalf("available - requested = %+v, want remaining %+v", got, m.ExBudget)
			}
		})
	}
}
