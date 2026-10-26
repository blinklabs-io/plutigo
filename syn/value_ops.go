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
	"bytes"
	"errors"
	"math/big"
	"sort"
)

// ErrValueOverflow is returned when a Value operation produces a quantity
// outside the signed 128-bit range.
var ErrValueOverflow = errors.New("value quantity out of range")

// ErrValueUninitialized is returned for a Value that bypassed NewValue.
var ErrValueUninitialized = errors.New("uninitialized value")

var (
	valueQuantityMax = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
	valueQuantityMin = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))
)

func quantityInRange(q *big.Int) bool {
	return q.Cmp(valueQuantityMin) >= 0 && q.Cmp(valueQuantityMax) <= 0
}

func valueNegativeCount(entries []IConstant) int {
	n := 0
	for _, entry := range entries {
		policy, ok := entry.(*ProtoPair)
		if !ok || policy == nil {
			continue
		}
		tokens, ok := policy.Second.(*ProtoList)
		if !ok || tokens == nil {
			continue
		}
		for _, tok := range tokens.List {
			pair, ok := tok.(*ProtoPair)
			if !ok || pair == nil {
				continue
			}
			if q, ok := pair.Second.(*Integer); ok && q != nil &&
				q.Inner != nil && q.Inner.Sign() < 0 {
				n++
			}
		}
	}
	return n
}

// HasNegative reports whether any quantity in the Value is negative. It is
// answered from cached metadata.
func (v *Value) HasNegative() bool {
	return v != nil && v.cachedNegatives > 0
}

// The helpers below read canonical Value entries: a policy entry is
// (policyID, [token entry]) and a token entry is (tokenID, quantity), both
// sorted by key.

// valueKey returns the policy or token ID of a policy or token entry.
func valueKey(entry IConstant) []byte {
	return entry.(*ProtoPair).First.(*ByteString).Inner
}

func valuePolicyTokens(entry IConstant) []IConstant {
	return entry.(*ProtoPair).Second.(*ProtoList).List
}

func valueTokenQuantity(entry IConstant) *big.Int {
	return entry.(*ProtoPair).Second.(*Integer).Inner
}

// searchValueKey binary searches policy or token entries for id, returning
// its index or the index at which it would be inserted.
func searchValueKey(entries []IConstant, id []byte) (int, bool) {
	i := sort.Search(len(entries), func(i int) bool {
		return bytes.Compare(valueKey(entries[i]), id) >= 0
	})
	return i, i < len(entries) && bytes.Equal(valueKey(entries[i]), id)
}

func newValueTokenPair(id []byte, q *big.Int) *ProtoPair {
	return &ProtoPair{
		FstType: valueTokenType.First,
		SndType: valueTokenType.Second,
		First:   &ByteString{Inner: id},
		Second:  newInteger(q),
	}
}

func newValuePolicyPair(id []byte, tokens []IConstant) *ProtoPair {
	return &ProtoPair{
		FstType: valueEntryType.First,
		SndType: valueEntryType.Second,
		First:   &ByteString{Inner: id},
		Second:  &ProtoList{LTyp: valueTokenType, List: tokens},
	}
}

// ownedValueKey returns the key for entry i of a list searched for key: the
// existing entry's key when found, which the new entry may share, otherwise
// a copy of the caller's key.
func ownedValueKey(entries []IConstant, i int, found bool, key []byte) []byte {
	if found {
		return valueKey(entries[i])
	}
	return append([]byte(nil), key...)
}

// spliceValueEntry returns a copy of entries with item placed at index i:
// replacing the entry there when found, inserted before it otherwise. A nil
// item removes the found entry.
func spliceValueEntry(entries []IConstant, i int, found bool, item IConstant) []IConstant {
	switch {
	case item == nil:
		out := make([]IConstant, 0, len(entries)-1)
		out = append(out, entries[:i]...)
		return append(out, entries[i+1:]...)
	case found:
		out := make([]IConstant, len(entries))
		copy(out, entries)
		out[i] = item
		return out
	default:
		out := make([]IConstant, 0, len(entries)+1)
		out = append(out, entries[:i]...)
		out = append(out, item)
		return append(out, entries[i:]...)
	}
}

// Insert returns a Value with the quantity for (policyID, tokenID) set to
// amount; zero removes the entry. Only the touched policy's token list and the
// outer policy list are copied, everything else is shared with the receiver,
// which is never modified. The caller must have checked key lengths and that
// amount is within the signed 128-bit range. The receiver itself is returned
// when nothing changes.
func (v *Value) Insert(policyID, tokenID []byte, amount *big.Int) (*Value, error) {
	total, ok := v.TotalSize()
	if !ok {
		return nil, ErrValueUninitialized
	}
	pi, pFound := searchValueKey(v.entries, policyID)
	oldTokens := []IConstant{}
	ti, tFound := 0, false
	if pFound {
		oldTokens = valuePolicyTokens(v.entries[pi])
		ti, tFound = searchValueKey(oldTokens, tokenID)
	}
	zero := amount.Sign() == 0
	if zero && !tFound {
		return v, nil
	}

	negatives := v.cachedNegatives
	if tFound && valueTokenQuantity(oldTokens[ti]).Sign() < 0 {
		negatives--
	}
	if amount.Sign() < 0 {
		negatives++
	}

	var token IConstant
	if !zero {
		token = newValueTokenPair(
			ownedValueKey(oldTokens, ti, tFound, tokenID), new(big.Int).Set(amount))
	}
	newTokens := spliceValueEntry(oldTokens, ti, tFound, token)

	var policy IConstant
	if len(newTokens) > 0 {
		policy = newValuePolicyPair(
			ownedValueKey(v.entries, pi, pFound, policyID), newTokens)
	}
	entries := spliceValueEntry(v.entries, pi, pFound, policy)

	maxInner := v.cachedMaxInner
	switch {
	case len(newTokens) >= maxInner:
		maxInner = len(newTokens)
	case len(oldTokens) == maxInner:
		// The largest policy shrank; recompute.
		maxInner = valueMaxInnerSize(entries)
	}
	return &Value{
		entries:         entries,
		cachedTotalSize: total + len(newTokens) - len(oldTokens) + 1,
		cachedMaxInner:  maxInner,
		cachedNegatives: negatives,
	}, nil
}

// Contains reports whether every quantity in req is covered by the receiver,
// using binary search over the canonical ordering. Both Values are assumed
// non-negative (see HasNegative) and req holds no zero quantities.
func (v *Value) Contains(req *Value) bool {
	vTotal, ok1 := v.TotalSize()
	rTotal, ok2 := req.TotalSize()
	if !ok1 || !ok2 {
		return false
	}
	if rTotal > vTotal {
		return false
	}
	lo := 0
	for _, rp := range req.entries {
		idx, found := searchValueKey(v.entries[lo:], valueKey(rp))
		if !found {
			return false
		}
		idx += lo
		lo = idx + 1
		vTokens := valuePolicyTokens(v.entries[idx])
		rTokens := valuePolicyTokens(rp)
		if len(rTokens) > len(vTokens) {
			return false
		}
		tlo := 0
		for _, rt := range rTokens {
			ti, found := searchValueKey(vTokens[tlo:], valueKey(rt))
			if !found {
				return false
			}
			ti += tlo
			tlo = ti + 1
			if valueTokenQuantity(vTokens[ti]).Cmp(valueTokenQuantity(rt)) < 0 {
				return false
			}
		}
	}
	return true
}

func runCheck(check func() error) error {
	if check == nil {
		return nil
	}
	return check()
}

// Scale multiplies every quantity by factor. Zero products are dropped. It
// returns ErrValueOverflow if any product leaves the signed 128-bit range.
func (v *Value) Scale(factor *big.Int, check func() error) (*Value, error) {
	if _, ok := v.TotalSize(); !ok {
		return nil, ErrValueUninitialized
	}
	if factor.Cmp(big.NewInt(1)) == 0 {
		return v, nil
	}
	entries := make([]IConstant, 0, len(v.entries))
	for _, pe := range v.entries {
		old := valuePolicyTokens(pe)
		toks := make([]IConstant, 0, len(old))
		for _, te := range old {
			if err := runCheck(check); err != nil {
				return nil, err
			}
			prod := new(big.Int).Mul(valueTokenQuantity(te), factor)
			if !quantityInRange(prod) {
				return nil, ErrValueOverflow
			}
			if prod.Sign() == 0 {
				continue
			}
			toks = append(toks, newValueTokenPair(valueKey(te), prod))
		}
		if len(toks) == 0 {
			continue
		}
		entries = append(entries, newValuePolicyPair(valueKey(pe), toks))
	}
	return newOwnedValue(entries), nil
}

// UnionValue merges two Values by summing quantities with a single merge pass
// over their canonical orderings. Policy entries present in only one input are
// shared with it. It returns ErrValueOverflow if any sum leaves the signed
// 128-bit range. check, if non-nil, is polled during the merge and its error
// aborts the operation.
func UnionValue(a, b *Value, check func() error) (*Value, error) {
	if _, ok := a.TotalSize(); !ok {
		return nil, ErrValueUninitialized
	}
	if _, ok := b.TotalSize(); !ok {
		return nil, ErrValueUninitialized
	}
	entries, err := mergeValueEntries(a.entries, b.entries, check,
		func(pa, pb IConstant) (IConstant, error) {
			tokens, err := mergeValueEntries(
				valuePolicyTokens(pa), valuePolicyTokens(pb), check, sumValueTokens)
			if err != nil || len(tokens) == 0 {
				return nil, err
			}
			return newValuePolicyPair(valueKey(pa), tokens), nil
		})
	if err != nil {
		return nil, err
	}
	return newOwnedValue(entries), nil
}

func sumValueTokens(ta, tb IConstant) (IConstant, error) {
	sum := new(big.Int).Add(valueTokenQuantity(ta), valueTokenQuantity(tb))
	if !quantityInRange(sum) {
		return nil, ErrValueOverflow
	}
	if sum.Sign() == 0 {
		return nil, nil
	}
	return newValueTokenPair(valueKey(ta), sum), nil
}

// mergeValueEntries merges two key-sorted policy or token entry lists. An
// entry whose key is in only one list is shared; entries with equal keys are
// passed to combine, whose nil result drops the key. check is polled once per
// output step.
func mergeValueEntries(
	a, b []IConstant,
	check func() error,
	combine func(a, b IConstant) (IConstant, error),
) ([]IConstant, error) {
	out := make([]IConstant, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if err := runCheck(check); err != nil {
			return nil, err
		}
		var c int
		switch {
		case i == len(a):
			c = 1
		case j == len(b):
			c = -1
		default:
			c = bytes.Compare(valueKey(a[i]), valueKey(b[j]))
		}
		switch {
		case c < 0:
			out = append(out, a[i])
			i++
		case c > 0:
			out = append(out, b[j])
			j++
		default:
			merged, err := combine(a[i], b[j])
			if err != nil {
				return nil, err
			}
			if merged != nil {
				out = append(out, merged)
			}
			i++
			j++
		}
	}
	return out, nil
}
