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

import (
	"math/big"
	"testing"
)

func policyTestValue(t *testing.T) *Value {
	t.Helper()
	entry := func(policy string, quantities ...int64) IConstant {
		tokens := make([]IConstant, len(quantities))
		for i, q := range quantities {
			tokens[i] = newValueTokenPair([]byte{byte(i)}, big.NewInt(q))
		}
		return newValuePolicyPair([]byte(policy), tokens)
	}
	v, err := NewValue([]IConstant{
		entry("", 1),
		entry("a", 5, -7),
		entry("b", -2, 9, 3),
		entry("c", 1),
	})
	if err != nil {
		t.Fatalf("NewValue: %v", err)
	}
	return v
}

// The cached shape of a filtered Value must equal the shape of the same
// entries rebuilt from scratch; valueContains and the cost model read it.
func TestValuePolicyFiltersKeepShapeCachesConsistent(t *testing.T) {
	t.Parallel()
	ids := [][]byte{{}, []byte("a"), []byte("b"), []byte("c"), []byte("zz")}
	for mask := 0; mask < 1<<len(ids); mask++ {
		var subset [][]byte
		for i, id := range ids {
			if mask&(1<<i) != 0 {
				subset = append(subset, id)
			}
		}
		// A repeated ID must not change the result.
		subset = append(subset, subset...)
		for name, filter := range map[string]func(*Value, [][]byte) (*Value, error){
			"keep": (*Value).KeepPolicies,
			"drop": (*Value).DropPolicies,
		} {
			got, err := filter(policyTestValue(t), subset)
			if err != nil {
				t.Fatalf("%s mask %d: %v", name, mask, err)
			}
			want, err := NewValue(got.Entries())
			if err != nil {
				t.Fatalf("%s mask %d: rebuild: %v", name, mask, err)
			}
			gt, gp, gm, _ := got.Sizes()
			wt, wp, wm, _ := want.Sizes()
			if gt != wt || gp != wp || gm != wm ||
				got.HasNegative() != want.HasNegative() ||
				got.cachedNegatives != want.cachedNegatives {
				t.Fatalf(
					"%s mask %d: caches (%d,%d,%d,neg %d) != rebuilt (%d,%d,%d,neg %d)",
					name, mask, gt, gp, gm, got.cachedNegatives,
					wt, wp, wm, want.cachedNegatives,
				)
			}
		}
	}
}

func TestValuePolicyFiltersPartitionTheValue(t *testing.T) {
	t.Parallel()
	v := policyTestValue(t)
	ids := [][]byte{[]byte("b"), []byte("a"), []byte("a")}
	kept, err := v.KeepPolicies(ids)
	if err != nil {
		t.Fatal(err)
	}
	dropped, err := v.DropPolicies(ids)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := kept.PolicyIDs(); len(got) != 2 ||
		string(got[0]) != "a" || string(got[1]) != "b" {
		t.Fatalf("kept policies = %q, want [a b]", got)
	}
	if got, _ := dropped.PolicyIDs(); len(got) != 2 ||
		len(got[0]) != 0 || string(got[1]) != "c" {
		t.Fatalf("dropped policies = %q, want [<empty> c]", got)
	}
	if total, _ := v.TotalSize(); total != 7 {
		t.Fatalf("input mutated: total size %d, want 7", total)
	}
}

func TestValuePolicyFiltersRejectUninitializedValue(t *testing.T) {
	t.Parallel()
	var v Value
	if _, err := v.KeepPolicies(nil); err != ErrValueUninitialized {
		t.Fatalf("KeepPolicies error = %v", err)
	}
	if _, err := v.DropPolicies(nil); err != ErrValueUninitialized {
		t.Fatalf("DropPolicies error = %v", err)
	}
}
