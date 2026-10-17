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
	if err := validateValueEntries(entries); err == nil {
		return entries, nil
	}

	totals := make(map[string]map[string]*big.Int, len(entries))
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
		policyTotals := totals[policyKey]
		if policyTotals == nil {
			policyTotals = make(map[string]*big.Int, len(tokens.List))
			totals[policyKey] = policyTotals
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
			total := policyTotals[tokenKey]
			if total == nil {
				policyTotals[tokenKey] = new(big.Int).Set(quantity.Inner)
			} else {
				total.Add(total, quantity.Inner)
			}
		}
	}

	policyIDs := make([]string, 0, len(totals))
	for policyID, tokens := range totals {
		for _, quantity := range tokens {
			if quantity.Sign() != 0 {
				policyIDs = append(policyIDs, policyID)
				break
			}
		}
	}
	sort.Strings(policyIDs)
	normalized := make([]IConstant, 0, len(policyIDs))
	for _, policyID := range policyIDs {
		tokenIDs := make([]string, 0, len(totals[policyID]))
		for tokenID, quantity := range totals[policyID] {
			if quantity.Sign() != 0 {
				tokenIDs = append(tokenIDs, tokenID)
			}
		}
		sort.Strings(tokenIDs)
		tokenEntries := make([]IConstant, 0, len(tokenIDs))
		for _, tokenID := range tokenIDs {
			quantity := totals[policyID][tokenID]
			if err := data.CheckValueQuantity(len(normalized), len(tokenEntries), quantity); err != nil {
				return nil, err
			}
			tokenEntries = append(tokenEntries, &ProtoPair{
				FstType: valueEntryType.Second.(*TList).Typ.(*TPair).First,
				SndType: valueEntryType.Second.(*TList).Typ.(*TPair).Second,
				First:   &ByteString{Inner: []byte(tokenID)},
				Second:  newInteger(new(big.Int).Set(quantity)),
			})
		}
		normalized = append(normalized, &ProtoPair{
			FstType: valueEntryType.First,
			SndType: valueEntryType.Second,
			First:   &ByteString{Inner: []byte(policyID)},
			Second:  &ProtoList{LTyp: valueEntryType.Second.(*TList).Typ, List: tokenEntries},
		})
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
	policyIndex := sort.Search(len(v.entries), func(i int) bool {
		policy := v.entries[i].(*ProtoPair).First.(*ByteString)
		return bytes.Compare(policy.Inner, policyID) >= 0
	})
	if policyIndex == len(v.entries) {
		return nil, false
	}
	policy := v.entries[policyIndex].(*ProtoPair)
	if !bytes.Equal(policy.First.(*ByteString).Inner, policyID) {
		return nil, false
	}
	tokens := policy.Second.(*ProtoList).List
	tokenIndex := sort.Search(len(tokens), func(i int) bool {
		token := tokens[i].(*ProtoPair).First.(*ByteString)
		return bytes.Compare(token.Inner, tokenID) >= 0
	})
	if tokenIndex == len(tokens) {
		return nil, false
	}
	token := tokens[tokenIndex].(*ProtoPair)
	if !bytes.Equal(token.First.(*ByteString).Inner, tokenID) {
		return nil, false
	}
	return new(big.Int).Set(token.Second.(*Integer).Inner), true
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
