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
	"math"
	"math/rand/v2"
	"testing"
)

// TestConstAboveDiagonalQuadraticMatchesBig checks the int64 fast path of
// ConstAboveDiagonalIntoQuadraticXAndYModel.CostTwo against the exact
// math/big evaluation, including inputs and coefficients at and beyond the
// fast-path limit.
func TestConstAboveDiagonalQuadraticMatchesBig(t *testing.T) {
	const limit = quadraticFastLimit
	edges := []int64{
		0,
		1,
		2,
		4,
		1000,
		limit - 1,
		limit,
		limit + 1,
		1 << 31,
		math.MaxInt64,
	}
	coeffEdges := []int64{
		0,
		1,
		-1,
		1000,
		-1000,
		limit,
		-limit,
		limit + 1,
		-(limit + 1),
		math.MaxInt64,
		math.MinInt64,
	}

	rng := rand.New(rand.NewPCG(1, 2))
	pick := func(vals []int64) int64 { return vals[rng.IntN(len(vals))] }
	randCoeff := func() int64 {
		switch rng.IntN(3) {
		case 0:
			return pick(coeffEdges)
		case 1:
			return rng.Int64N(2*limit+1) - limit
		default:
			return rng.Int64N(1<<40) - 1<<39
		}
	}
	randInput := func() int64 {
		if rng.IntN(2) == 0 {
			return pick(edges)
		}
		return rng.Int64N(2 * limit)
	}

	models := []ConstAboveDiagonalIntoQuadraticXAndYModel{
		// Mainnet PlutusV3 divideInteger CPU parameters.
		{
			constant: 85848,
			TwoArgumentsQuadraticFunction: TwoArgumentsQuadraticFunction{
				minimum: 85848, coeff00: 123203, coeff10: 1716, coeff01: 7305,
				coeff20: 57, coeff11: 960, coeff02: -900,
			},
		},
	}
	for range 2000 {
		models = append(models, ConstAboveDiagonalIntoQuadraticXAndYModel{
			constant: rng.Int64N(1 << 20),
			TwoArgumentsQuadraticFunction: TwoArgumentsQuadraticFunction{
				minimum: randCoeff(),
				coeff00: randCoeff(), coeff10: randCoeff(), coeff01: randCoeff(),
				coeff20: randCoeff(), coeff11: randCoeff(), coeff02: randCoeff(),
			},
		})
	}

	for i, m := range models {
		for range 50 {
			x, y := randInput(), randInput()
			if x < y {
				x, y = y, x
			}
			got := m.CostTwo(ExMem(x), ExMem(y))
			want := m.costTwoBig(x, y)
			if got != want {
				t.Fatalf(
					"model %d %+v: CostTwo(%d, %d) = %d, want %d",
					i,
					m,
					x,
					y,
					got,
					want,
				)
			}
		}
		if got := m.CostTwo(ExMem(1), ExMem(2)); got != m.constant {
			t.Fatalf(
				"model %d: below diagonal = %d, want constant %d",
				i,
				got,
				m.constant,
			)
		}
	}
}

func BenchmarkConstAboveDiagonalQuadratic(b *testing.B) {
	m := ConstAboveDiagonalIntoQuadraticXAndYModel{
		constant: 85848,
		TwoArgumentsQuadraticFunction: TwoArgumentsQuadraticFunction{
			minimum: 85848, coeff00: 123203, coeff10: 1716, coeff01: 7305,
			coeff20: 57, coeff11: 960, coeff02: -900,
		},
	}
	b.Run("int64", func(b *testing.B) {
		for b.Loop() {
			_ = m.CostTwo(4, 2)
		}
	})
	b.Run("big", func(b *testing.B) {
		for b.Loop() {
			_ = m.costTwoBig(4, 2)
		}
	})
}
