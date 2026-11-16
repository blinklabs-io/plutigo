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

package syn

import "slices"

// policyIndices returns the sorted, de-duplicated positions of the requested
// policies in the Value. An ID the Value does not hold, including one longer
// than any policy ID a Value can hold, has no position and is skipped. It
// costs O(p log m) for p IDs and m policies.
func (v *Value) policyIndices(ids [][]byte) []int {
	indices := make([]int, 0, len(ids))
	for _, id := range ids {
		if i, found := searchValueKey(v.entries, id); found {
			indices = append(indices, i)
		}
	}
	slices.Sort(indices)
	return slices.Compact(indices)
}

// KeepPolicies returns the Value restricted to the given policy IDs. The
// result shares its policy entries with v, which are immutable. The order of
// ids and repeated ids do not affect the result.
func (v *Value) KeepPolicies(ids [][]byte) (*Value, error) {
	if _, ok := v.TotalSize(); !ok {
		return nil, ErrValueUninitialized
	}
	indices := v.policyIndices(ids)
	kept := make([]IConstant, len(indices))
	for i, at := range indices {
		kept[i] = v.entries[at]
	}
	return newOwnedValue(kept), nil
}

// DropPolicies returns the Value without the given policy IDs. The result
// shares its policy entries with v, which are immutable. The cached asset and
// negative counts are adjusted by the removed policies only, so the cost does
// not grow with the number of tokens that remain.
func (v *Value) DropPolicies(ids [][]byte) (*Value, error) {
	if _, ok := v.TotalSize(); !ok {
		return nil, ErrValueUninitialized
	}
	indices := v.policyIndices(ids)
	if len(indices) == 0 {
		return v, nil
	}
	total := v.cachedTotalSize - 1
	negatives := v.cachedNegatives
	remaining := make([]IConstant, 0, len(v.entries)-len(indices))
	maxInner := 0
	next := 0
	for i, entry := range v.entries {
		tokens := valuePolicyTokens(entry)
		if next < len(indices) && indices[next] == i {
			next++
			total -= len(tokens)
			negatives -= valueNegativeCount([]IConstant{entry})
			continue
		}
		remaining = append(remaining, entry)
		maxInner = max(maxInner, len(tokens))
	}
	return &Value{
		entries:         remaining,
		cachedTotalSize: total + 1,
		cachedMaxInner:  maxInner,
		cachedNegatives: negatives,
	}, nil
}
