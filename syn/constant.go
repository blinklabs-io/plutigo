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
	"fmt"
	"math/big"
	"sort"

	"github.com/blinklabs-io/plutigo/data"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type IConstant interface {
	isConstant()

	Typ() Typ
}

// (con integer 1)
type Integer struct {
	Inner       *big.Int
	cachedExMem int64
	cachedInt64 int64
	cachedMeta  uint8
}

const (
	integerMetaCached uint8 = 1 << iota
	integerMetaInt64
)

func newInteger(inner *big.Int) *Integer {
	ret := &Integer{}
	ret.SetInner(inner)
	return ret
}

// SetInner replaces the integer payload and refreshes the cached metadata used
// by the evaluator hot path.
func (i *Integer) SetInner(inner *big.Int) {
	i.Inner = inner
	i.cachedExMem = integerExMemWords(inner)
	i.cachedInt64 = 0
	i.cachedMeta = integerMetaCached
	if inner != nil && inner.IsInt64() {
		i.cachedMeta |= integerMetaInt64
		i.cachedInt64 = inner.Int64()
	}
}

func (i *Integer) CachedInt64() (int64, bool) {
	if i != nil && i.cachedMeta&integerMetaCached != 0 {
		return i.cachedInt64, i.cachedMeta&integerMetaInt64 != 0
	}
	if i == nil || i.Inner == nil || !i.Inner.IsInt64() {
		return 0, false
	}
	return i.Inner.Int64(), true
}

func (i *Integer) ExMemWords() int64 {
	if i != nil && i.cachedMeta&integerMetaCached != 0 {
		return i.cachedExMem
	}
	if i == nil {
		return integerExMemWords(nil)
	}
	return integerExMemWords(i.Inner)
}

func integerExMemWords(inner *big.Int) int64 {
	if inner == nil || inner.Sign() == 0 {
		return 1
	}
	if inner.IsInt64() || inner.IsUint64() {
		return 1
	}
	return int64((inner.BitLen()-1)/64 + 1)
}

func (Integer) isConstant() {}

func (i Integer) Typ() Typ {
	return &TInteger{}
}

// (con bytestring #aaBB)
type ByteString struct {
	Inner []byte
}

func (ByteString) isConstant() {}

func (bs ByteString) Typ() Typ {
	return &TByteString{}
}

// (con string "hello world")
type String struct {
	Inner string
}

func (String) isConstant() {}

func (s String) Typ() Typ {
	return &TString{}
}

// (con unit ())
type Unit struct{}

func (Unit) isConstant() {}

func (u Unit) Typ() Typ {
	return &TUnit{}
}

// (con bool True)
type Bool struct {
	Inner bool
}

func (Bool) isConstant() {}

func (b Bool) Typ() Typ {
	return &TBool{}
}

type ProtoList struct {
	LTyp Typ
	List []IConstant
}

func (ProtoList) isConstant() {}

func (pl ProtoList) Typ() Typ {
	return &TList{Typ: pl.LTyp}
}

type ProtoArray struct {
	ATyp  Typ
	Array []IConstant
}

func (ProtoArray) isConstant() {}

func (pa ProtoArray) Typ() Typ {
	return &TArray{Typ: pa.ATyp}
}

type Value struct {
	entries         []IConstant
	cachedTotalSize int
	cachedMaxInner  int
	// cachedNegatives counts the negative quantities held by the Value so
	// valueContains can reject negative inputs without a traversal.
	cachedNegatives int
}

// NewValue normalizes and snapshots entries into an immutable Value. Value
// builtins use the cached shape metrics for the same constant-time lookups as
// the reference implementation.
func NewValue(entries []IConstant) (*Value, error) {
	normalized, err := normalizeValueEntries(entries)
	if err != nil {
		return nil, err
	}
	return newOwnedValue(cloneValueEntries(normalized)), nil
}

func newOwnedValue(entries []IConstant) *Value {
	return &Value{
		entries:         entries,
		cachedTotalSize: valueTotalSize(entries) + 1,
		cachedMaxInner:  valueMaxInnerSize(entries),
		cachedNegatives: valueNegativeCount(entries),
	}
}

func checkValueKeyLength(kind string, index int, key []byte) error {
	if len(key) > data.MaxValueKeyLength {
		return fmt.Errorf("Value %s %d key exceeds %d bytes", kind, index, data.MaxValueKeyLength)
	}
	return nil
}

func checkValueQuantityRange(policy, token int, quantity *big.Int) error {
	if quantity == nil {
		return fmt.Errorf("Value policy %d token %d quantity must be an integer", policy, token)
	}
	if quantity.Sign() == 0 {
		return nil
	}
	return data.CheckValueQuantity(policy, token, quantity)
}

func normalizeValueEntries(entries []IConstant) ([]IConstant, error) {
	return normalizeValueEntriesWithArena(entries, nil)
}

// Strict parser and decoder paths must validate before normalization hides
// duplicate, ordering, empty-token, or zero-quantity violations.
func prepareValueEntries(
	entries []IConstant,
	strict bool,
	arena *constantArena,
) ([]IConstant, error) {
	if strict {
		if err := validateValueEntries(entries); err != nil {
			return nil, err
		}
		return entries, nil
	}
	return normalizeValueEntriesWithArena(entries, arena)
}

type valueTokenTotal struct {
	id     []byte
	amount *big.Int
}

type valuePolicyTotal struct {
	id     []byte
	tokens map[string]valueTokenTotal
}

func normalizeValueEntriesWithArena(
	entries []IConstant,
	arena *constantArena,
) ([]IConstant, error) {
	if err := validateValueEntries(entries); err == nil {
		return entries, nil
	}

	policies := make(map[string]valuePolicyTotal, len(entries))
	for policyIndex, entry := range entries {
		policy, ok := entry.(*ProtoPair)
		if !ok || policy == nil {
			return nil, fmt.Errorf("value policy entry %d is not a pair", policyIndex)
		}
		policyID, ok := policy.First.(*ByteString)
		if !ok || policyID == nil {
			return nil, fmt.Errorf("value policy entry %d has a non-bytestring key", policyIndex)
		}
		if err := checkValueKeyLength("policy", policyIndex, policyID.Inner); err != nil {
			return nil, err
		}
		tokens, ok := policy.Second.(*ProtoList)
		if !ok || tokens == nil {
			return nil, fmt.Errorf("value policy entry %d has a non-list payload", policyIndex)
		}

		policyKey := string(policyID.Inner)
		policyTotals, ok := policies[policyKey]
		if !ok {
			policyTotals = valuePolicyTotal{
				id:     policyID.Inner,
				tokens: make(map[string]valueTokenTotal, len(tokens.List)),
			}
		}
		for tokenIndex, entry := range tokens.List {
			token, ok := entry.(*ProtoPair)
			if !ok || token == nil {
				return nil, fmt.Errorf("value token entry %d:%d is not a pair", policyIndex, tokenIndex)
			}
			tokenID, ok := token.First.(*ByteString)
			if !ok || tokenID == nil {
				return nil, fmt.Errorf("value token entry %d:%d has a non-bytestring key", policyIndex, tokenIndex)
			}
			if err := checkValueKeyLength("token", tokenIndex, tokenID.Inner); err != nil {
				return nil, err
			}
			quantity, ok := token.Second.(*Integer)
			if !ok || quantity == nil || quantity.Inner == nil {
				return nil, fmt.Errorf("value token entry %d:%d has a non-integer quantity", policyIndex, tokenIndex)
			}
			if err := checkValueQuantityRange(policyIndex, tokenIndex, quantity.Inner); err != nil {
				return nil, err
			}

			tokenKey := string(tokenID.Inner)
			total, ok := policyTotals.tokens[tokenKey]
			if !ok {
				amount := new(big.Int)
				if arena == nil {
					amount.Set(quantity.Inner)
				} else {
					amount = arena.copyBigInt(quantity.Inner)
				}
				total = valueTokenTotal{id: tokenID.Inner, amount: amount}
			} else {
				total.amount.Add(total.amount, quantity.Inner)
			}
			policyTotals.tokens[tokenKey] = total
		}
		policies[policyKey] = policyTotals
	}

	policyIDs := make([]string, 0, len(policies))
	for policyID, policy := range policies {
		for _, token := range policy.tokens {
			if token.amount.Sign() != 0 {
				policyIDs = append(policyIDs, policyID)
				break
			}
		}
	}
	sort.Strings(policyIDs)
	var normalized []IConstant
	if arena == nil {
		normalized = make([]IConstant, 0, len(policyIDs))
	} else {
		normalized = arena.allocList(len(policyIDs))
	}
	tokenPairType := valueEntryType.Second.(*TList).Typ.(*TPair)
	policyListType := valueEntryType.Second.(*TList)
	for policyIndex, policyID := range policyIDs {
		policy := policies[policyID]
		tokenIDs := make([]string, 0, len(policy.tokens))
		for tokenID, token := range policy.tokens {
			if token.amount.Sign() != 0 {
				tokenIDs = append(tokenIDs, tokenID)
			}
		}
		sort.Strings(tokenIDs)
		var tokenEntries []IConstant
		if arena == nil {
			tokenEntries = make([]IConstant, len(tokenIDs))
		} else {
			tokenEntries = arena.allocList(len(tokenIDs))
		}
		for tokenIndex, tokenID := range tokenIDs {
			token := policy.tokens[tokenID]
			if err := data.CheckValueQuantity(policyIndex, tokenIndex, token.amount); err != nil {
				return nil, err
			}
			if arena == nil {
				tokenEntries[tokenIndex] = &ProtoPair{
					FstType: tokenPairType.First,
					SndType: tokenPairType.Second,
					First:   &ByteString{Inner: token.id},
					Second:  newInteger(token.amount),
				}
				continue
			}
			tokenEntries[tokenIndex] = arena.allocProtoPair(
				tokenPairType.First,
				tokenPairType.Second,
				arena.allocByteString(token.id),
				arena.allocInteger(token.amount),
			)
		}

		var tokenList *ProtoList
		if arena == nil {
			tokenList = &ProtoList{LTyp: policyListType.Typ, List: tokenEntries}
		} else {
			tokenList = arena.allocProtoList(policyListType.Typ, tokenEntries)
		}
		if arena == nil {
			normalized = append(normalized, &ProtoPair{
				FstType: valueEntryType.First,
				SndType: valueEntryType.Second,
				First:   &ByteString{Inner: policy.id},
				Second:  tokenList,
			})
			continue
		}
		normalized[policyIndex] = arena.allocProtoPair(
			valueEntryType.First,
			valueEntryType.Second,
			arena.allocByteString(policy.id),
			tokenList,
		)
	}
	if err := validateValueEntries(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

// TotalSize returns the cached number of distinct policy and token pairs.
// The boolean is false for Values that bypassed NewValue.
func (v *Value) TotalSize() (int, bool) {
	if v == nil || v.cachedTotalSize == 0 {
		return 0, false
	}
	return v.cachedTotalSize - 1, true
}

// Sizes returns the cached total asset count, policy count, and largest token
// count. The boolean is false for a zero Value that bypassed NewValue.
func (v *Value) Sizes() (int, int, int, bool) {
	total, ok := v.TotalSize()
	if !ok {
		return 0, 0, 0, false
	}
	return total, len(v.entries), v.cachedMaxInner, true
}

// Entries returns a deep copy of the canonical entries. Mutating the returned
// value cannot change the Value or invalidate its cached shape metrics.
func (v *Value) Entries() []IConstant {
	if _, ok := v.TotalSize(); !ok {
		return nil
	}
	return cloneValueEntries(v.entries)
}

// PolicyIDs returns copies of the canonical policy IDs without traversing
// their token maps. The boolean is false for an uninitialized Value.
func (v *Value) PolicyIDs() ([][]byte, bool) {
	if _, ok := v.TotalSize(); !ok {
		return nil, false
	}
	ret := make([][]byte, len(v.entries))
	for i, entry := range v.entries {
		policy := entry.(*ProtoPair).First.(*ByteString)
		ret[i] = append([]byte(nil), policy.Inner...)
	}
	return ret, true
}

// LookupCoin returns a copy of the quantity for a policy and token. It uses
// the canonical ordering to avoid traversing unrelated assets.
func (v *Value) LookupCoin(policyID, tokenID []byte) (*big.Int, bool) {
	if _, ok := v.TotalSize(); !ok {
		return nil, false
	}
	policyIndex, found := searchValueKey(v.entries, policyID)
	if !found {
		return nil, false
	}
	tokens := valuePolicyTokens(v.entries[policyIndex])
	tokenIndex, found := searchValueKey(tokens, tokenID)
	if !found {
		return nil, false
	}
	return new(big.Int).Set(valueTokenQuantity(tokens[tokenIndex])), true
}

func cloneValueEntries(entries []IConstant) []IConstant {
	ret := make([]IConstant, len(entries))
	for i, entry := range entries {
		policy := entry.(*ProtoPair)
		policyID := policy.First.(*ByteString)
		tokens := policy.Second.(*ProtoList)
		tokenEntries := make([]IConstant, len(tokens.List))
		for j, tokenEntry := range tokens.List {
			token := tokenEntry.(*ProtoPair)
			tokenID := token.First.(*ByteString)
			quantity := token.Second.(*Integer)
			tokenEntries[j] = &ProtoPair{
				FstType: &TByteString{},
				SndType: &TInteger{},
				First: &ByteString{
					Inner: append([]byte(nil), tokenID.Inner...),
				},
				Second: newInteger(new(big.Int).Set(quantity.Inner)),
			}
		}
		ret[i] = &ProtoPair{
			FstType: &TByteString{},
			SndType: &TList{Typ: &TPair{
				First:  &TByteString{},
				Second: &TInteger{},
			}},
			First: &ByteString{Inner: append([]byte(nil), policyID.Inner...)},
			Second: &ProtoList{
				LTyp: &TPair{First: &TByteString{}, Second: &TInteger{}},
				List: tokenEntries,
			},
		}
	}
	return ret
}

func valueTotalSize(entries []IConstant) int {
	total := 0
	for _, entry := range entries {
		policy, ok := entry.(*ProtoPair)
		if !ok {
			continue
		}
		tokens, ok := policy.Second.(*ProtoList)
		if !ok {
			continue
		}
		total += len(tokens.List)
	}
	return total
}

func valueMaxInnerSize(entries []IConstant) int {
	maxInner := 0
	for _, entry := range entries {
		policy, ok := entry.(*ProtoPair)
		if !ok || policy == nil {
			continue
		}
		tokens, ok := policy.Second.(*ProtoList)
		if !ok || tokens == nil {
			continue
		}
		maxInner = max(maxInner, len(tokens.List))
	}
	return maxInner
}

func (Value) isConstant() {}

func (Value) Typ() Typ {
	return &TValue{}
}

type ProtoPair struct {
	FstType Typ
	SndType Typ
	First   IConstant
	Second  IConstant
}

func (ProtoPair) isConstant() {}

func (pp ProtoPair) Typ() Typ {
	return &TPair{First: pp.FstType, Second: pp.SndType}
}

type Data struct {
	Inner data.PlutusData
}

func (Data) isConstant() {}

func (Data) Typ() Typ {
	return &TData{}
}

type Bls12_381G1Element struct {
	Inner *bls.G1Jac
}

func (Bls12_381G1Element) isConstant() {}

func (Bls12_381G1Element) Typ() Typ {
	return &TBls12_381G1Element{}
}

type Bls12_381G2Element struct {
	Inner *bls.G2Jac
}

func (Bls12_381G2Element) isConstant() {}

func (Bls12_381G2Element) Typ() Typ {
	return &TBls12_381G2Element{}
}

type Bls12_381MlResult struct {
	Inner *bls.GT
}

func (Bls12_381MlResult) isConstant() {}

func (Bls12_381MlResult) Typ() Typ {
	return &TBls12_381MlResult{}
}
