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
	"github.com/blinklabs-io/plutigo/data"
)

// Plutus V4 script context builders.
//
// These types serialize values supplied by callers. Ledger integrations remain
// responsible for deriving those values from transaction and ledger state.
// data.PlutusData fields must contain non-nil values already encoded in their
// V4-specific shape.
// Constructor tags and field order follow PlutusLedgerApi.V4.Contexts at
// https://github.com/IntersectMBO/plutus/blob/39981dd733ae276975958e40b0291ce1b24781d5/plutus-ledger-api/src/PlutusLedgerApi/V4/Contexts.hs.

// Maybe represents the Maybe encoding used by Plutus data: Nothing is
// constructor 0 with no fields, and Just is constructor 1 with one field.
type Maybe struct {
	value   data.PlutusData
	present bool
}

// Nothing returns the absent value for a V4 optional field.
func Nothing() Maybe {
	return Maybe{}
}

// Just returns the present value for a V4 optional field.
func Just(value data.PlutusData) Maybe {
	return Maybe{value: value, present: true}
}

// ToData serializes the optional value.
func (m Maybe) ToData() data.PlutusData {
	if !m.present {
		return data.NewConstr(0)
	}
	return data.NewConstr(1, m.value)
}

// AccountIDV4 is a V4 account identifier. It encodes as its credential without
// an additional constructor, matching the ledger newtype representation.
type AccountIDV4 struct {
	Credential data.PlutusData
}

// ToData returns the credential representation of the account identifier.
func (a AccountIDV4) ToData() data.PlutusData {
	return a.Credential
}

// PubKeyCredential constructs a public-key credential from its hash bytes.
func PubKeyCredential(hash []byte) data.PlutusData {
	return data.NewConstr(0, data.NewByteString(hash))
}

// ScriptCredential constructs a script credential from its hash bytes.
func ScriptCredential(hash []byte) data.PlutusData {
	return data.NewConstr(1, data.NewByteString(hash))
}

// AddressV4 is a Plutus V4 address, containing a payment credential and an
// optional staking account identifier.
type AddressV4 struct {
	Credential     data.PlutusData
	StakingAccount Maybe
}

// ToData serializes the V4 address as a two-field list.
func (a AddressV4) ToData() data.PlutusData {
	return data.NewList(a.Credential, a.StakingAccount.ToData())
}

// TxOutRefV4 identifies a transaction output by transaction ID and index.
type TxOutRefV4 struct {
	TxID  data.PlutusData
	Index data.PlutusData
}

// ToData serializes the output reference as a two-field list.
func (r TxOutRefV4) ToData() data.PlutusData {
	return data.NewList(r.TxID, r.Index)
}

// OutputDatumV4 is a V4 output datum variant.
type OutputDatumV4 interface {
	ToData() data.PlutusData
	isOutputDatum()
}

// NoOutputDatumV4 represents an output without a datum.
type NoOutputDatumV4 struct{}

func (NoOutputDatumV4) isOutputDatum() {}

// ToData serializes the absent output datum as constructor 0.
func (NoOutputDatumV4) ToData() data.PlutusData {
	return data.NewConstr(0)
}

// OutputDatumHashV4 represents an output that contains only a datum hash.
type OutputDatumHashV4 struct {
	Hash data.PlutusData
}

func (OutputDatumHashV4) isOutputDatum() {}

// ToData serializes a datum hash as constructor 1.
func (d OutputDatumHashV4) ToData() data.PlutusData {
	return data.NewConstr(1, d.Hash)
}

// InlineOutputDatumV4 represents an output with an inline datum.
type InlineOutputDatumV4 struct {
	Datum data.PlutusData
}

func (InlineOutputDatumV4) isOutputDatum() {}

// ToData serializes an inline datum as constructor 2.
func (d InlineOutputDatumV4) ToData() data.PlutusData {
	return data.NewConstr(2, d.Datum)
}

// TxOutV4 is a Plutus V4 transaction output. A nil Datum is encoded as
// NoOutputDatumV4.
type TxOutV4 struct {
	Address         AddressV4
	Value           data.PlutusData
	Datum           OutputDatumV4
	ReferenceScript Maybe
}

// ToData serializes the four V4 transaction output fields in ledger order.
func (o TxOutV4) ToData() data.PlutusData {
	datum := OutputDatumV4(NoOutputDatumV4{})
	if o.Datum != nil {
		datum = o.Datum
	}
	return data.NewList(
		o.Address.ToData(),
		o.Value,
		datum.ToData(),
		o.ReferenceScript.ToData(),
	)
}

// TxInInfoV4 pairs an output reference with its resolved output.
type TxInInfoV4 struct {
	OutRef   TxOutRefV4
	Resolved TxOutV4
}

// ToData serializes the reference and resolved output as a two-field list.
func (i TxInInfoV4) ToData() data.PlutusData {
	return data.NewList(i.OutRef.ToData(), i.Resolved.ToData())
}

// AccountBalanceIntervalV4 is a V4 account balance interval variant.
type AccountBalanceIntervalV4 interface {
	ToData() data.PlutusData
	isAccountBalanceInterval()
}

// AccountBalanceLowerBoundV4 represents an interval with only a lower bound.
type AccountBalanceLowerBoundV4 struct {
	Amount data.PlutusData
}

func (AccountBalanceLowerBoundV4) isAccountBalanceInterval() {}

// ToData serializes a lower-bound interval as constructor 0.
func (i AccountBalanceLowerBoundV4) ToData() data.PlutusData {
	return data.NewConstr(0, i.Amount)
}

// AccountBalanceUpperBoundV4 represents an interval with only an upper bound.
type AccountBalanceUpperBoundV4 struct {
	Amount data.PlutusData
}

func (AccountBalanceUpperBoundV4) isAccountBalanceInterval() {}

// ToData serializes an upper-bound interval as constructor 1.
func (i AccountBalanceUpperBoundV4) ToData() data.PlutusData {
	return data.NewConstr(1, i.Amount)
}

// AccountBalanceBothBoundsV4 represents an interval with lower and upper bounds.
type AccountBalanceBothBoundsV4 struct {
	Lower data.PlutusData
	Upper data.PlutusData
}

func (AccountBalanceBothBoundsV4) isAccountBalanceInterval() {}

// ToData serializes a bounded interval as constructor 2.
func (i AccountBalanceBothBoundsV4) ToData() data.PlutusData {
	return data.NewConstr(2, i.Lower, i.Upper)
}

// AccountBalanceExactV4 represents an exact account balance.
type AccountBalanceExactV4 struct {
	Amount data.PlutusData
}

func (AccountBalanceExactV4) isAccountBalanceInterval() {}

// ToData serializes an exact balance as constructor 3.
func (i AccountBalanceExactV4) ToData() data.PlutusData {
	return data.NewConstr(3, i.Amount)
}

// ScriptPurposeV4 is a V4 transaction purpose variant.
type ScriptPurposeV4 interface {
	ToData() data.PlutusData
	isScriptPurpose()
}

// MintingPurposeV4 identifies a minting purpose.
type MintingPurposeV4 struct {
	ScriptHash     data.PlutusData
	CurrencySymbol data.PlutusData
}

func (MintingPurposeV4) isScriptPurpose() {}

// ToData serializes a minting purpose as constructor 0.
func (p MintingPurposeV4) ToData() data.PlutusData {
	return data.NewConstr(0, p.ScriptHash, p.CurrencySymbol)
}

// SpendingPurposeV4 identifies a spending purpose.
type SpendingPurposeV4 struct {
	ScriptHash data.PlutusData
	OutRef     TxOutRefV4
}

func (SpendingPurposeV4) isScriptPurpose() {}

// ToData serializes a spending purpose as constructor 1.
func (p SpendingPurposeV4) ToData() data.PlutusData {
	return data.NewConstr(1, p.ScriptHash, p.OutRef.ToData())
}

// WithdrawingPurposeV4 identifies a withdrawal purpose.
type WithdrawingPurposeV4 struct {
	ScriptHash data.PlutusData
	Credential data.PlutusData
}

func (WithdrawingPurposeV4) isScriptPurpose() {}

// ToData serializes a withdrawal purpose as constructor 2.
func (p WithdrawingPurposeV4) ToData() data.PlutusData {
	return data.NewConstr(2, p.ScriptHash, p.Credential)
}

// CertifyingPurposeV4 identifies a certificate purpose.
type CertifyingPurposeV4 struct {
	ScriptHash  data.PlutusData
	Index       data.PlutusData
	Certificate data.PlutusData
}

func (CertifyingPurposeV4) isScriptPurpose() {}

// ToData serializes a certificate purpose as constructor 3.
func (p CertifyingPurposeV4) ToData() data.PlutusData {
	return data.NewConstr(3, p.ScriptHash, p.Index, p.Certificate)
}

// VotingPurposeV4 identifies a voting purpose.
type VotingPurposeV4 struct {
	ScriptHash data.PlutusData
	Voter      data.PlutusData
}

func (VotingPurposeV4) isScriptPurpose() {}

// ToData serializes a voting purpose as constructor 4.
func (p VotingPurposeV4) ToData() data.PlutusData {
	return data.NewConstr(4, p.ScriptHash, p.Voter)
}

// ProposingPurposeV4 identifies a proposal purpose.
type ProposingPurposeV4 struct {
	ScriptHash        data.PlutusData
	Index             data.PlutusData
	ProposalProcedure data.PlutusData
}

func (ProposingPurposeV4) isScriptPurpose() {}

// ToData serializes a proposal purpose as constructor 5.
func (p ProposingPurposeV4) ToData() data.PlutusData {
	return data.NewConstr(5, p.ScriptHash, p.Index, p.ProposalProcedure)
}

// GuardingPurposeV4 identifies the V4 guard purpose introduced for observing
// scripts. It corresponds to the Observe purpose from CIP-0112 and uses the
// Guarding name in the V4 ledger API.
type GuardingPurposeV4 struct {
	ScriptHash data.PlutusData
	Index      data.PlutusData
}

func (GuardingPurposeV4) isScriptPurpose() {}

// ToData serializes a guarding purpose as constructor 6.
func (p GuardingPurposeV4) ToData() data.PlutusData {
	return data.NewConstr(6, p.ScriptHash, p.Index)
}

// ObservingPurposeV4 is an alias for GuardingPurposeV4, the V4 ledger name for the
// Observe purpose described by CIP-0112.
type ObservingPurposeV4 = GuardingPurposeV4

// ScriptInfoV4 is a V4 script information variant.
type ScriptInfoV4 interface {
	ToData() data.PlutusData
	isScriptInfo()
}

// MintingScriptV4 is information for a minting script.
type MintingScriptV4 struct {
	CurrencySymbol data.PlutusData
}

func (MintingScriptV4) isScriptInfo() {}

// ToData serializes minting script information as constructor 0.
func (s MintingScriptV4) ToData() data.PlutusData {
	return data.NewConstr(0, s.CurrencySymbol)
}

// SpendingScriptV4 is information for a spending script.
type SpendingScriptV4 struct {
	OutRef TxOutRefV4
	Datum  Maybe
}

func (SpendingScriptV4) isScriptInfo() {}

// ToData serializes spending script information as constructor 1.
func (s SpendingScriptV4) ToData() data.PlutusData {
	return data.NewConstr(1, s.OutRef.ToData(), s.Datum.ToData())
}

// WithdrawingScriptV4 is information for a withdrawal script.
type WithdrawingScriptV4 struct {
	AccountID data.PlutusData
}

func (WithdrawingScriptV4) isScriptInfo() {}

// ToData serializes withdrawal script information as constructor 2.
func (s WithdrawingScriptV4) ToData() data.PlutusData {
	return data.NewConstr(2, s.AccountID)
}

// CertifyingScriptV4 is information for a certificate script.
type CertifyingScriptV4 struct {
	Index       data.PlutusData
	Certificate data.PlutusData
}

func (CertifyingScriptV4) isScriptInfo() {}

// ToData serializes certificate script information as constructor 3.
func (s CertifyingScriptV4) ToData() data.PlutusData {
	return data.NewConstr(3, s.Index, s.Certificate)
}

// VotingScriptV4 is information for a voting script.
type VotingScriptV4 struct {
	Voter data.PlutusData
}

func (VotingScriptV4) isScriptInfo() {}

// ToData serializes voting script information as constructor 4.
func (s VotingScriptV4) ToData() data.PlutusData {
	return data.NewConstr(4, s.Voter)
}

// ProposingScriptV4 is information for a proposal script.
type ProposingScriptV4 struct {
	Index             data.PlutusData
	ProposalProcedure data.PlutusData
}

func (ProposingScriptV4) isScriptInfo() {}

// ToData serializes proposal script information as constructor 5.
func (s ProposingScriptV4) ToData() data.PlutusData {
	return data.NewConstr(5, s.Index, s.ProposalProcedure)
}

// GuardingScriptV4 is information for a V4 guard script. TopTxInfo is absent
// when the guard runs in a sub-transaction and present at the top level.
type GuardingScriptV4 struct {
	Index     data.PlutusData
	TopTxInfo Maybe
}

func (GuardingScriptV4) isScriptInfo() {}

// ToData serializes guarding script information as constructor 6.
func (s GuardingScriptV4) ToData() data.PlutusData {
	return data.NewConstr(6, s.Index, s.TopTxInfo.ToData())
}

// ObservingScriptV4 is an alias for GuardingScriptV4, the V4 ledger name for the
// Observe script type described by CIP-0112.
type ObservingScriptV4 = GuardingScriptV4

// TxInfoV4 is the Plutus V4 transaction information record. Each list or map
// field uses the corresponding Go slice and is encoded in place. TxCerts,
// credentials, redeemers, datums, voters, governance actions, and time ranges
// are accepted as their already-encoded Plutus data values.
type TxInfoV4 struct {
	ID                      data.PlutusData
	SubTxIndex              Maybe
	Inputs                  []data.PlutusData
	ReferenceInputs         []data.PlutusData
	Outputs                 []data.PlutusData
	Mint                    data.PlutusData
	TxCerts                 []data.PlutusData
	Withdrawals             [][2]data.PlutusData
	DirectDeposits          [][2]data.PlutusData
	AccountBalanceIntervals [][2]data.PlutusData
	ValidRange              data.PlutusData
	Guards                  []data.PlutusData
	RequiredTopLevelGuards  [][2]data.PlutusData
	Redeemers               [][2]data.PlutusData
	Data                    [][2]data.PlutusData
	Votes                   [][2]data.PlutusData
	ProposalProcedures      []data.PlutusData
	CurrentTreasuryAmount   Maybe
	TreasuryDonation        data.PlutusData
}

// ToData serializes TxInfo as its 19-field V4 list. V4 TxInfo has no isValid
// field; validity is represented by ValidRange.
func (t TxInfoV4) ToData() data.PlutusData {
	return data.NewList(
		t.ID,
		t.SubTxIndex.ToData(),
		data.NewList(t.Inputs...),
		data.NewList(t.ReferenceInputs...),
		data.NewList(t.Outputs...),
		t.Mint,
		data.NewList(t.TxCerts...),
		data.NewMap(t.Withdrawals),
		data.NewMap(t.DirectDeposits),
		data.NewMap(t.AccountBalanceIntervals),
		t.ValidRange,
		data.NewList(t.Guards...),
		data.NewMap(t.RequiredTopLevelGuards),
		data.NewMap(t.Redeemers),
		data.NewMap(t.Data),
		data.NewMap(t.Votes),
		data.NewList(t.ProposalProcedures...),
		t.CurrentTreasuryAmount.ToData(),
		t.TreasuryDonation,
	)
}

// TopTxInfoSimplifiedV4 is the aggregate view of a V4 top-level transaction.
type TopTxInfoSimplifiedV4 struct {
	IDs                    []data.PlutusData
	Inputs                 []data.PlutusData
	ReferenceInputs        []data.PlutusData
	Outputs                []data.PlutusData
	Mints                  data.PlutusData
	Burns                  data.PlutusData
	TxCerts                []data.PlutusData
	Withdrawals            [][2]data.PlutusData
	DirectDeposits         [][2]data.PlutusData
	ValidRange             data.PlutusData
	Guards                 []data.PlutusData
	RequiredTopLevelGuards []data.PlutusData
	RedeemerHashes         []data.PlutusData
	Data                   [][2]data.PlutusData
	Votes                  [][2]data.PlutusData
	ProposalProcedures     []data.PlutusData
	CurrentTreasuryAmount  Maybe
	TreasuryDonations      data.PlutusData
}

// ToData serializes the 18 fields in the V4 top-level summary.
func (t TopTxInfoSimplifiedV4) ToData() data.PlutusData {
	return data.NewList(
		data.NewList(t.IDs...),
		data.NewList(t.Inputs...),
		data.NewList(t.ReferenceInputs...),
		data.NewList(t.Outputs...),
		t.Mints,
		t.Burns,
		data.NewList(t.TxCerts...),
		data.NewMap(t.Withdrawals),
		data.NewMap(t.DirectDeposits),
		t.ValidRange,
		data.NewList(t.Guards...),
		data.NewList(t.RequiredTopLevelGuards...),
		data.NewList(t.RedeemerHashes...),
		data.NewMap(t.Data),
		data.NewMap(t.Votes),
		data.NewList(t.ProposalProcedures...),
		t.CurrentTreasuryAmount.ToData(),
		t.TreasuryDonations,
	)
}

// TopTxInfoV4 contains nested sub-transaction contexts and the aggregate V4
// transaction view surfaced to a top-level guard.
type TopTxInfoV4 struct {
	SubTransactions                 []TxInfoV4
	Datums                          [][2]data.PlutusData
	StartingAccountBalanceIntervals data.PlutusData
	Simplified                      TopTxInfoSimplifiedV4
}

// ToData serializes top-level transaction information as a four-field list.
func (t TopTxInfoV4) ToData() data.PlutusData {
	subTransactions := make([]data.PlutusData, len(t.SubTransactions))
	for i := range t.SubTransactions {
		subTransactions[i] = t.SubTransactions[i].ToData()
	}
	return data.NewList(
		data.NewList(subTransactions...),
		data.NewMap(t.Datums),
		t.StartingAccountBalanceIntervals,
		t.Simplified.ToData(),
	)
}

// ScriptContextV4 is the V4 context provided to a script. ScriptInfo must be a
// concrete variant because V4 has no absent ScriptInfo constructor.
type ScriptContextV4 struct {
	TxInfo     TxInfoV4
	Redeemer   data.PlutusData
	ScriptInfo ScriptInfoV4
	ScriptHash data.PlutusData
}

// ToData serializes the four V4 script context fields in ledger order.
func (s ScriptContextV4) ToData() data.PlutusData {
	var scriptInfo data.PlutusData
	if s.ScriptInfo != nil {
		scriptInfo = s.ScriptInfo.ToData()
	}
	return data.NewList(s.TxInfo.ToData(), s.Redeemer, scriptInfo, s.ScriptHash)
}
