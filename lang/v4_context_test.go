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

package lang

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
)

func integer(value int64) data.PlutusData {
	return data.NewInteger(big.NewInt(value))
}

func emptyTxInfo() TxInfoV4 {
	return TxInfoV4{
		ID:                      data.NewByteString(nil),
		SubTxIndex:              Nothing(),
		Inputs:                  []data.PlutusData{},
		ReferenceInputs:         []data.PlutusData{},
		Outputs:                 []data.PlutusData{},
		Mint:                    integer(0),
		TxCerts:                 []data.PlutusData{},
		Withdrawals:             [][2]data.PlutusData{},
		DirectDeposits:          [][2]data.PlutusData{},
		AccountBalanceIntervals: [][2]data.PlutusData{},
		ValidRange:              integer(0),
		Guards:                  []data.PlutusData{},
		RequiredTopLevelGuards:  [][2]data.PlutusData{},
		Redeemers:               [][2]data.PlutusData{},
		Data:                    [][2]data.PlutusData{},
		Votes:                   [][2]data.PlutusData{},
		ProposalProcedures:      []data.PlutusData{},
		CurrentTreasuryAmount:   Nothing(),
		TreasuryDonation:        integer(0),
	}
}

func emptyTopTxInfo() TopTxInfoV4 {
	return TopTxInfoV4{
		SubTransactions:                 []TxInfoV4{},
		Datums:                          [][2]data.PlutusData{},
		StartingAccountBalanceIntervals: data.NewMap(nil),
		Simplified: TopTxInfoSimplifiedV4{
			IDs:                    []data.PlutusData{},
			Inputs:                 []data.PlutusData{},
			ReferenceInputs:        []data.PlutusData{},
			Outputs:                []data.PlutusData{},
			Mints:                  integer(0),
			Burns:                  integer(0),
			TxCerts:                []data.PlutusData{},
			Withdrawals:            [][2]data.PlutusData{},
			DirectDeposits:         [][2]data.PlutusData{},
			ValidRange:             integer(0),
			Guards:                 []data.PlutusData{},
			RequiredTopLevelGuards: []data.PlutusData{},
			RedeemerHashes:         []data.PlutusData{},
			Data:                   [][2]data.PlutusData{},
			Votes:                  [][2]data.PlutusData{},
			ProposalProcedures:     []data.PlutusData{},
			CurrentTreasuryAmount:  Nothing(),
			TreasuryDonations:      integer(0),
		},
	}
}

func assertDataEqual(t *testing.T, got, want data.PlutusData) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("data mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestMaybeData(t *testing.T) {
	assertDataEqual(t, Nothing().ToData(), data.NewConstr(0))
	assertDataEqual(t, Just(integer(7)).ToData(), data.NewConstr(1, integer(7)))
}

func TestV4AddressData(t *testing.T) {
	credential := PubKeyCredential([]byte{0xaa})
	account := AccountIDV4{Credential: ScriptCredential([]byte{0xbb})}
	address := AddressV4{
		Credential:     credential,
		StakingAccount: Just(account.ToData()),
	}

	assertDataEqual(t, address.ToData(), data.NewList(
		data.NewConstr(0, data.NewByteString([]byte{0xaa})),
		data.NewConstr(1, data.NewConstr(1, data.NewByteString([]byte{0xbb}))),
	))
	assertDataEqual(t, (AddressV4{Credential: credential}).ToData(), data.NewList(
		credential,
		data.NewConstr(0),
	))
}

func TestV4TransactionOutputData(t *testing.T) {
	address := AddressV4{Credential: PubKeyCredential([]byte{1})}
	ref := TxOutRefV4{TxID: data.NewByteString([]byte{2}), Index: integer(3)}
	tests := []struct {
		name string
		got  OutputDatumV4
		want data.PlutusData
	}{
		{"nil defaults to none", nil, data.NewConstr(0)},
		{"none", NoOutputDatumV4{}, data.NewConstr(0)},
		{"hash", OutputDatumHashV4{integer(4)}, data.NewConstr(1, integer(4))},
		{"inline", InlineOutputDatumV4{integer(5)}, data.NewConstr(2, integer(5))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := TxOutV4{
				Address:         address,
				Value:           integer(6),
				Datum:           tt.got,
				ReferenceScript: Just(integer(7)),
			}
			wantOut := data.NewList(
				address.ToData(),
				integer(6),
				tt.want,
				data.NewConstr(1, integer(7)),
			)
			assertDataEqual(t, out.ToData(), wantOut)
			assertDataEqual(t, (TxInInfoV4{OutRef: ref, Resolved: out}).ToData(), data.NewList(ref.ToData(), wantOut))
		})
	}
}

func TestScriptPurposeData(t *testing.T) {
	ref := TxOutRefV4{TxID: data.NewByteString([]byte{1}), Index: integer(2)}
	procedure := data.NewConstr(4, integer(8))
	tests := []struct {
		name string
		got  ScriptPurposeV4
		want data.PlutusData
	}{
		{"minting", MintingPurposeV4{integer(1), integer(2)}, data.NewConstr(0, integer(1), integer(2))},
		{"spending", SpendingPurposeV4{integer(1), ref}, data.NewConstr(1, integer(1), ref.ToData())},
		{"withdrawing", WithdrawingPurposeV4{integer(1), integer(2)}, data.NewConstr(2, integer(1), integer(2))},
		{"certifying", CertifyingPurposeV4{integer(1), integer(2), integer(3)}, data.NewConstr(3, integer(1), integer(2), integer(3))},
		{"voting", VotingPurposeV4{integer(1), integer(2)}, data.NewConstr(4, integer(1), integer(2))},
		{"proposing", ProposingPurposeV4{integer(1), integer(2), procedure}, data.NewConstr(5, integer(1), integer(2), procedure)},
		{"guarding", GuardingPurposeV4{integer(1), integer(2)}, data.NewConstr(6, integer(1), integer(2))},
		{"observing alias", ObservingPurposeV4{integer(1), integer(2)}, data.NewConstr(6, integer(1), integer(2))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertDataEqual(t, tt.got.ToData(), tt.want)
		})
	}
}

func TestScriptInfoData(t *testing.T) {
	ref := TxOutRefV4{TxID: data.NewByteString([]byte{1}), Index: integer(2)}
	top := emptyTopTxInfo()
	maybeTop := Just(top.ToData())
	tests := []struct {
		name string
		got  ScriptInfoV4
		want data.PlutusData
	}{
		{"minting", MintingScriptV4{integer(1)}, data.NewConstr(0, integer(1))},
		{"spending", SpendingScriptV4{ref, Just(integer(3))}, data.NewConstr(1, ref.ToData(), data.NewConstr(1, integer(3)))},
		{"withdrawing", WithdrawingScriptV4{integer(1)}, data.NewConstr(2, integer(1))},
		{"certifying", CertifyingScriptV4{integer(1), integer(2)}, data.NewConstr(3, integer(1), integer(2))},
		{"voting", VotingScriptV4{integer(1)}, data.NewConstr(4, integer(1))},
		{"proposing", ProposingScriptV4{integer(1), integer(2)}, data.NewConstr(5, integer(1), integer(2))},
		{"guarding", GuardingScriptV4{integer(1), maybeTop}, data.NewConstr(6, integer(1), data.NewConstr(1, top.ToData()))},
		{"observing alias", ObservingScriptV4{integer(1), Nothing()}, data.NewConstr(6, integer(1), data.NewConstr(0))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertDataEqual(t, tt.got.ToData(), tt.want)
		})
	}
}

func TestTxInfoDataFieldOrder(t *testing.T) {
	tx := TxInfoV4{
		ID:                      integer(1),
		SubTxIndex:              Just(integer(2)),
		Inputs:                  []data.PlutusData{integer(3)},
		ReferenceInputs:         []data.PlutusData{integer(4)},
		Outputs:                 []data.PlutusData{integer(5)},
		Mint:                    integer(6),
		TxCerts:                 []data.PlutusData{integer(7)},
		Withdrawals:             [][2]data.PlutusData{{integer(8), integer(9)}},
		DirectDeposits:          [][2]data.PlutusData{{integer(10), integer(11)}},
		AccountBalanceIntervals: [][2]data.PlutusData{{integer(12), integer(13)}},
		ValidRange:              integer(14),
		Guards:                  []data.PlutusData{integer(15)},
		RequiredTopLevelGuards:  [][2]data.PlutusData{{integer(16), integer(17)}},
		Redeemers:               [][2]data.PlutusData{{integer(18), integer(19)}},
		Data:                    [][2]data.PlutusData{{integer(20), integer(21)}},
		Votes:                   [][2]data.PlutusData{{integer(22), integer(23)}},
		ProposalProcedures:      []data.PlutusData{integer(24)},
		CurrentTreasuryAmount:   Just(integer(25)),
		TreasuryDonation:        integer(26),
	}
	want := data.NewList(
		integer(1),
		data.NewConstr(1, integer(2)),
		data.NewList(integer(3)),
		data.NewList(integer(4)),
		data.NewList(integer(5)),
		integer(6),
		data.NewList(integer(7)),
		data.NewMap([][2]data.PlutusData{{integer(8), integer(9)}}),
		data.NewMap([][2]data.PlutusData{{integer(10), integer(11)}}),
		data.NewMap([][2]data.PlutusData{{integer(12), integer(13)}}),
		integer(14),
		data.NewList(integer(15)),
		data.NewMap([][2]data.PlutusData{{integer(16), integer(17)}}),
		data.NewMap([][2]data.PlutusData{{integer(18), integer(19)}}),
		data.NewMap([][2]data.PlutusData{{integer(20), integer(21)}}),
		data.NewMap([][2]data.PlutusData{{integer(22), integer(23)}}),
		data.NewList(integer(24)),
		data.NewConstr(1, integer(25)),
		integer(26),
	)
	got := tx.ToData()
	list := got.(*data.List)
	if len(list.Items) != 19 {
		t.Fatalf("TxInfo has %d fields, want 19", len(list.Items))
	}
	assertDataEqual(t, got, want)
}

func TestTopTxInfoSimplifiedDataFieldOrder(t *testing.T) {
	info := TopTxInfoSimplifiedV4{
		IDs:                    []data.PlutusData{integer(1)},
		Inputs:                 []data.PlutusData{integer(2)},
		ReferenceInputs:        []data.PlutusData{integer(3)},
		Outputs:                []data.PlutusData{integer(4)},
		Mints:                  integer(5),
		Burns:                  integer(6),
		TxCerts:                []data.PlutusData{integer(7)},
		Withdrawals:            [][2]data.PlutusData{{integer(8), integer(9)}},
		DirectDeposits:         [][2]data.PlutusData{{integer(10), integer(11)}},
		ValidRange:             integer(12),
		Guards:                 []data.PlutusData{integer(13)},
		RequiredTopLevelGuards: []data.PlutusData{integer(14)},
		RedeemerHashes:         []data.PlutusData{integer(15)},
		Data:                   [][2]data.PlutusData{{integer(16), integer(17)}},
		Votes:                  [][2]data.PlutusData{{integer(18), integer(19)}},
		ProposalProcedures:     []data.PlutusData{integer(20)},
		CurrentTreasuryAmount:  Just(integer(21)),
		TreasuryDonations:      integer(22),
	}
	want := data.NewList(
		data.NewList(integer(1)),
		data.NewList(integer(2)),
		data.NewList(integer(3)),
		data.NewList(integer(4)),
		integer(5),
		integer(6),
		data.NewList(integer(7)),
		data.NewMap([][2]data.PlutusData{{integer(8), integer(9)}}),
		data.NewMap([][2]data.PlutusData{{integer(10), integer(11)}}),
		integer(12),
		data.NewList(integer(13)),
		data.NewList(integer(14)),
		data.NewList(integer(15)),
		data.NewMap([][2]data.PlutusData{{integer(16), integer(17)}}),
		data.NewMap([][2]data.PlutusData{{integer(18), integer(19)}}),
		data.NewList(integer(20)),
		data.NewConstr(1, integer(21)),
		integer(22),
	)
	got := info.ToData()
	list := got.(*data.List)
	if len(list.Items) != 18 {
		t.Fatalf("TopTxInfoSimplified has %d fields, want 18", len(list.Items))
	}
	assertDataEqual(t, got, want)
}

func TestNestedTopTxInfoAndScriptContextData(t *testing.T) {
	info := emptyTxInfo()
	info.ID = data.NewByteString([]byte{1})
	info.TreasuryDonation = integer(2)
	summary := emptyTopTxInfo().Simplified
	summary.IDs = []data.PlutusData{info.ID}
	summary.TreasuryDonations = integer(3)
	top := TopTxInfoV4{
		SubTransactions:                 []TxInfoV4{info},
		Datums:                          [][2]data.PlutusData{{info.ID, integer(4)}},
		StartingAccountBalanceIntervals: data.NewMap(nil),
		Simplified:                      summary,
	}
	topData := top.ToData()
	topList := topData.(*data.List)
	if len(topList.Items) != 4 {
		t.Fatalf("TopTxInfo has %d fields, want 4", len(topList.Items))
	}
	assertDataEqual(t, topList.Items[0], data.NewList(info.ToData()))

	context := ScriptContextV4{
		TxInfo:     info,
		Redeemer:   integer(5),
		ScriptInfo: GuardingScriptV4{Index: integer(6), TopTxInfo: Just(topData)},
		ScriptHash: data.NewByteString([]byte{7}),
	}
	contextData := context.ToData()
	contextList := contextData.(*data.List)
	if len(contextList.Items) != 4 {
		t.Fatalf("ScriptContext has %d fields, want 4", len(contextList.Items))
	}
	assertDataEqual(t, contextList.Items[2], data.NewConstr(6, integer(6), data.NewConstr(1, topData)))
	encoded, err := data.Encode(contextData)
	if err != nil {
		t.Fatalf("encode V4 context: %v", err)
	}
	decoded, err := data.Decode(encoded)
	if err != nil {
		t.Fatalf("decode V4 context: %v", err)
	}
	assertDataEqual(t, decoded, contextData)
}

func TestAccountBalanceIntervalTags(t *testing.T) {
	tests := []struct {
		name string
		got  AccountBalanceIntervalV4
		want data.PlutusData
	}{
		{"lower", AccountBalanceLowerBoundV4{integer(1)}, data.NewConstr(0, integer(1))},
		{"upper", AccountBalanceUpperBoundV4{integer(2)}, data.NewConstr(1, integer(2))},
		{"both", AccountBalanceBothBoundsV4{integer(3), integer(4)}, data.NewConstr(2, integer(3), integer(4))},
		{"exact", AccountBalanceExactV4{integer(5)}, data.NewConstr(3, integer(5))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertDataEqual(t, tt.got.ToData(), tt.want)
		})
	}
}
