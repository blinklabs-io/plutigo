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
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/syn"
)

// referenceValueCostParams are the epoch-659 mainnet PlutusV3 cost-model
// parameters for the value builtins under test (Koios /epoch_params?_epoch_no=659,
// "PlutusV3" list, indices 313-331 in lang.CostModelParamNamesV3 order).
const (
	insertCoinCPUIntercept int64 = 356924
	insertCoinCPUSlope     int64 = 18413
	insertCoinMemIntercept int64 = 45
	insertCoinMemSlope     int64 = 21
)

// newValueCostTestInput builds a Value constant with policyCount policies,
// each holding tokensPerPolicy tokens, as the reference's Value packs them:
// Map PolicyId (Map TokenName Quantity).
func newValueCostTestInput(policyCount, tokensPerPolicy int) *Constant {
	entries := make([]syn.IConstant, 0, policyCount)
	for p := range policyCount {
		tokens := make([]syn.IConstant, 0, tokensPerPolicy)
		for t := range tokensPerPolicy {
			tokens = append(tokens, &syn.ProtoPair{
				FstType: &syn.TByteString{},
				SndType: &syn.TInteger{},
				First: &syn.ByteString{
					Inner: []byte{byte(p >> 8), byte(p), byte(t >> 8), byte(t)},
				},
				Second: &syn.Integer{Inner: big.NewInt(1)},
			})
		}
		entries = append(entries, &syn.ProtoPair{
			FstType: &syn.TByteString{},
			SndType: &syn.TList{Typ: &syn.TPair{
				First:  &syn.TByteString{},
				Second: &syn.TInteger{},
			}},
			First: &syn.ByteString{Inner: []byte{byte(p >> 8), byte(p)}},
			Second: &syn.ProtoList{
				LTyp: &syn.TPair{
					First:  &syn.TByteString{},
					Second: &syn.TInteger{},
				},
				List: tokens,
			},
		})
	}
	return &Constant{&syn.Value{Entries: entries}}
}

// builtinCost runs a builtin over the given arguments on a fresh V4 machine and
// returns what it spent, so a test can pin the cost of one application without
// the surrounding CEK steps.
func builtinCost(
	t *testing.T,
	fn builtin.DefaultFunction,
	args ...Value[syn.DeBruijn],
) ExBudget {
	t.Helper()
	m := newTestMachineV4()
	b := newTestBuiltin(fn)
	for _, arg := range args {
		b = b.ApplyArg(arg)
	}
	before := m.ExBudget
	if _, err := evalBuiltinWithError(t, m, b); err != nil {
		t.Fatalf("%s returned error: %v", fn, err)
	}
	return ExBudget{
		Cpu: before.Cpu - m.ExBudget.Cpu,
		Mem: before.Mem - m.ExBudget.Mem,
	}
}

// TestInsertCoinCostsMaxDepthNotSize pins insertCoin's fourth-argument metric to
// the reference's ValueMaxDepth -- the sum of the bit lengths of the policy
// count and the largest per-policy token count -- rather than a count of
// entries.
//
// Costing the entry count instead over-charges every multi-policy Value, which
// is a consensus bug: on mainnet tx
// 5e32d273e8088cd805601c1fc4eb86bea487bd669459b7af80c997d22b267b18 (slot
// 199576795) spent a 3-policy, 3-token Value, where depth is 3 and the entry
// count is 5, so dingo charged 2 * 18413 = 36826 CPU and 2 * 21 = 42 memory
// units more than the reference and rejected a canonical block.
func TestInsertCoinCostsMaxDepthNotSize(t *testing.T) {
	tests := []struct {
		name            string
		policyCount     int
		tokensPerPolicy int
		wantMaxDepth    uint64
	}{
		{name: "empty", policyCount: 0, tokensPerPolicy: 0, wantMaxDepth: 0},
		{name: "one policy one token", policyCount: 1, tokensPerPolicy: 1, wantMaxDepth: 2},
		{name: "one policy three tokens", policyCount: 1, tokensPerPolicy: 3, wantMaxDepth: 3},
		{name: "two policies one token", policyCount: 2, tokensPerPolicy: 1, wantMaxDepth: 3},
		{name: "three policies one token", policyCount: 3, tokensPerPolicy: 1, wantMaxDepth: 3},
		{name: "four policies one token", policyCount: 4, tokensPerPolicy: 1, wantMaxDepth: 4},
		{name: "eight policies one token", policyCount: 8, tokensPerPolicy: 1, wantMaxDepth: 5},
		{name: "five policies two tokens", policyCount: 5, tokensPerPolicy: 2, wantMaxDepth: 5},
		{name: "eight policies four tokens", policyCount: 8, tokensPerPolicy: 4, wantMaxDepth: 7},
		{name: "sixteen policies one token", policyCount: 16, tokensPerPolicy: 1, wantMaxDepth: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := builtinCost(
				t,
				builtin.InsertCoin,
				&Constant{&syn.ByteString{Inner: []byte{0xaa}}},
				&Constant{&syn.ByteString{Inner: []byte{0xbb}}},
				&Constant{&syn.Integer{Inner: big.NewInt(5)}},
				newValueCostTestInput(tt.policyCount, tt.tokensPerPolicy),
			)
			wantCPU := insertCoinCPUIntercept + insertCoinCPUSlope*int64(tt.wantMaxDepth)
			wantMem := insertCoinMemIntercept + insertCoinMemSlope*int64(tt.wantMaxDepth)
			if cost.Cpu != wantCPU || cost.Mem != wantMem {
				t.Fatalf(
					"insertCoin over %d policies x %d tokens cost (%d cpu, %d mem), want (%d cpu, %d mem) for max depth %d",
					tt.policyCount,
					tt.tokensPerPolicy,
					cost.Cpu,
					cost.Mem,
					wantCPU,
					wantMem,
					tt.wantMaxDepth,
				)
			}
		})
	}
}

// TestLookupCoinCostsMaxDepthNotSize pins lookupCoin's third-argument metric to
// the same ValueMaxDepth as insertCoin. Both search a Value by policy and then
// by token, so they are costed on depth for the same reason.
func TestLookupCoinCostsMaxDepthNotSize(t *testing.T) {
	// lookupCoin-cpu-arguments-intercept and -slope, epoch 659 PlutusV3.
	const (
		cpuIntercept int64 = 219951
		cpuSlope     int64 = 9444
	)

	tests := []struct {
		name            string
		policyCount     int
		tokensPerPolicy int
		wantMaxDepth    uint64
	}{
		{name: "empty", policyCount: 0, tokensPerPolicy: 0, wantMaxDepth: 0},
		{name: "one policy one token", policyCount: 1, tokensPerPolicy: 1, wantMaxDepth: 2},
		{name: "three policies one token", policyCount: 3, tokensPerPolicy: 1, wantMaxDepth: 3},
		{name: "four policies one token", policyCount: 4, tokensPerPolicy: 1, wantMaxDepth: 4},
		{name: "eight policies one token", policyCount: 8, tokensPerPolicy: 1, wantMaxDepth: 5},
		{name: "eight policies four tokens", policyCount: 8, tokensPerPolicy: 4, wantMaxDepth: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := builtinCost(
				t,
				builtin.LookupCoin,
				&Constant{&syn.ByteString{Inner: []byte{0xaa}}},
				&Constant{&syn.ByteString{Inner: []byte{0xbb}}},
				newValueCostTestInput(tt.policyCount, tt.tokensPerPolicy),
			)
			wantCPU := cpuIntercept + cpuSlope*int64(tt.wantMaxDepth)
			if cost.Cpu != wantCPU {
				t.Fatalf(
					"lookupCoin over %d policies x %d tokens cost %d cpu, want %d cpu for max depth %d",
					tt.policyCount,
					tt.tokensPerPolicy,
					cost.Cpu,
					wantCPU,
					tt.wantMaxDepth,
				)
			}
		})
	}
}

// TestScaleValueCostsTotalSizeNotOuterPlusInner pins scaleValue's second-argument
// metric to the reference's ValueTotalSize -- the number of (policy, token)
// pairs -- rather than max(0, outer + inner - 1). Scaling touches every pair,
// and the entry-count formula both skipped the final pair of every multi-token
// Value and added the policy count, so it disagreed with the reference in both
// directions.
func TestScaleValueCostsTotalSizeNotOuterPlusInner(t *testing.T) {
	// scaleValue-cpu-arguments-intercept and -slope, epoch 659 PlutusV3.
	const (
		cpuIntercept int64 = 1000
		cpuSlope     int64 = 277577
	)

	tests := []struct {
		name            string
		policyCount     int
		tokensPerPolicy int
		wantTotalSize   uint64
	}{
		{name: "empty", policyCount: 0, tokensPerPolicy: 0, wantTotalSize: 0},
		{name: "one policy one token", policyCount: 1, tokensPerPolicy: 1, wantTotalSize: 1},
		{name: "two policies one token", policyCount: 2, tokensPerPolicy: 1, wantTotalSize: 2},
		{name: "one policy eight tokens", policyCount: 1, tokensPerPolicy: 8, wantTotalSize: 8},
		{name: "three policies one token", policyCount: 3, tokensPerPolicy: 1, wantTotalSize: 3},
		{name: "eight policies four tokens", policyCount: 8, tokensPerPolicy: 4, wantTotalSize: 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := builtinCost(
				t,
				builtin.ScaleValue,
				&Constant{&syn.Integer{Inner: big.NewInt(3)}},
				newValueCostTestInput(tt.policyCount, tt.tokensPerPolicy),
			)
			wantCPU := cpuIntercept + cpuSlope*int64(tt.wantTotalSize)
			if cost.Cpu != wantCPU {
				t.Fatalf(
					"scaleValue over %d policies x %d tokens cost %d cpu, want %d cpu for total size %d",
					tt.policyCount,
					tt.tokensPerPolicy,
					cost.Cpu,
					wantCPU,
					tt.wantTotalSize,
				)
			}
		})
	}
}

// TestUnionValueCostsTotalSizeNotOuterCount pins both of unionValue's arguments
// to the reference's ValueTotalSize. The policy count alone ignored every
// additional token in a policy, so unionValue under-charged multi-token Values
// by up to a factor of the token count.
func TestUnionValueCostsTotalSizeNotOuterCount(t *testing.T) {
	// unionValue-cpu-arguments-{c00,c01,c10,c11}, epoch 659 PlutusV3.
	const (
		c00 int64 = 1000
		c01 int64 = 183150
		c10 int64 = 172116
		c11 int64 = 6
	)

	tests := []struct {
		name            string
		policyCount     int
		tokensPerPolicy int
		wantTotalSize   uint64
	}{
		{name: "empty", policyCount: 0, tokensPerPolicy: 0, wantTotalSize: 0},
		{name: "one policy one token", policyCount: 1, tokensPerPolicy: 1, wantTotalSize: 1},
		{name: "one policy three tokens", policyCount: 1, tokensPerPolicy: 3, wantTotalSize: 3},
		{name: "four policies one token", policyCount: 4, tokensPerPolicy: 1, wantTotalSize: 4},
		{name: "eight policies four tokens", policyCount: 8, tokensPerPolicy: 4, wantTotalSize: 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := newValueCostTestInput(tt.policyCount, tt.tokensPerPolicy)
			cost := builtinCost(t, builtin.UnionValue, value, value)
			size := int64(tt.wantTotalSize)
			wantCPU := c00 + c01*size + c10*size + c11*size*size
			if cost.Cpu != wantCPU {
				t.Fatalf(
					"unionValue over %d policies x %d tokens cost %d cpu, want %d cpu for total size %d",
					tt.policyCount,
					tt.tokensPerPolicy,
					cost.Cpu,
					wantCPU,
					tt.wantTotalSize,
				)
			}
		})
	}
}
