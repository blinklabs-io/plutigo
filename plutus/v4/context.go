// Package v4 builds Plutus V4 context data structures.
//
// The package serializes values supplied by callers. Ledger integrations remain
// responsible for deriving those values from transaction and ledger state.
// Data fields must contain non-nil Plutus data values already encoded in their
// V4-specific shape.
// Constructor tags and field order follow PlutusLedgerApi.V4.Contexts at
// https://github.com/IntersectMBO/plutus/blob/39981dd733ae276975958e40b0291ce1b24781d5/plutus-ledger-api/src/PlutusLedgerApi/V4/Contexts.hs.
package v4

import (
	"github.com/blinklabs-io/plutigo/data"
)

// Data is a Plutus data value accepted by the builders in this package.
type Data = data.PlutusData

// List constructs a Plutus data list.
func List(items ...Data) Data {
	return data.NewList(items...)
}

// MapEntry is a key/value pair in a Plutus data map.
type MapEntry [2]Data

// Map constructs a Plutus data map. Entry order is preserved.
func Map(entries ...MapEntry) Data {
	pairs := make([][2]Data, len(entries))
	for i, entry := range entries {
		pairs[i] = [2]Data{entry[0], entry[1]}
	}
	return data.NewMap(pairs)
}

// Maybe represents the Maybe encoding used by Plutus data: Nothing is
// constructor 0 with no fields, and Just is constructor 1 with one field.
type Maybe struct {
	value   Data
	present bool
}

// Nothing returns the absent value for a V4 optional field.
func Nothing() Maybe {
	return Maybe{}
}

// Just returns the present value for a V4 optional field.
func Just(value Data) Maybe {
	return Maybe{value: value, present: true}
}

// ToData serializes the optional value.
func (m Maybe) ToData() Data {
	if !m.present {
		return data.NewConstr(0)
	}
	return data.NewConstr(1, m.value)
}

// AccountID is a V4 account identifier. It encodes as its credential without
// an additional constructor, matching the ledger newtype representation.
type AccountID struct {
	Credential Data
}

// ToData returns the credential representation of the account identifier.
func (a AccountID) ToData() Data {
	return a.Credential
}

// PubKeyCredential constructs a public-key credential from its hash bytes.
func PubKeyCredential(hash []byte) Data {
	return data.NewConstr(0, data.NewByteString(hash))
}

// ScriptCredential constructs a script credential from its hash bytes.
func ScriptCredential(hash []byte) Data {
	return data.NewConstr(1, data.NewByteString(hash))
}

// Address is a Plutus V4 address, containing a payment credential and an
// optional staking account identifier.
type Address struct {
	Credential     Data
	StakingAccount Maybe
}

// ToData serializes the V4 address as a two-field list.
func (a Address) ToData() Data {
	return data.NewList(a.Credential, a.StakingAccount.ToData())
}

// TxOutRef identifies a transaction output by transaction ID and index.
type TxOutRef struct {
	TxID  Data
	Index Data
}

// ToData serializes the output reference as a two-field list.
func (r TxOutRef) ToData() Data {
	return data.NewList(r.TxID, r.Index)
}

// OutputDatum is a V4 output datum variant.
type OutputDatum interface {
	ToData() Data
	isOutputDatum()
}

// NoOutputDatum represents an output without a datum.
type NoOutputDatum struct{}

func (NoOutputDatum) isOutputDatum() {}

// ToData serializes the absent output datum as constructor 0.
func (NoOutputDatum) ToData() Data {
	return data.NewConstr(0)
}

// OutputDatumHash represents an output that contains only a datum hash.
type OutputDatumHash struct {
	Hash Data
}

func (OutputDatumHash) isOutputDatum() {}

// ToData serializes a datum hash as constructor 1.
func (d OutputDatumHash) ToData() Data {
	return data.NewConstr(1, d.Hash)
}

// InlineOutputDatum represents an output with an inline datum.
type InlineOutputDatum struct {
	Datum Data
}

func (InlineOutputDatum) isOutputDatum() {}

// ToData serializes an inline datum as constructor 2.
func (d InlineOutputDatum) ToData() Data {
	return data.NewConstr(2, d.Datum)
}

// TxOut is a Plutus V4 transaction output.
type TxOut struct {
	Address         Address
	Value           Data
	Datum           OutputDatum
	ReferenceScript Maybe
}

// ToData serializes the four V4 transaction output fields in ledger order.
func (o TxOut) ToData() Data {
	var datum Data
	if o.Datum != nil {
		datum = o.Datum.ToData()
	}
	return data.NewList(
		o.Address.ToData(),
		o.Value,
		datum,
		o.ReferenceScript.ToData(),
	)
}

// TxInInfo pairs an output reference with its resolved output.
type TxInInfo struct {
	OutRef   TxOutRef
	Resolved TxOut
}

// ToData serializes the reference and resolved output as a two-field list.
func (i TxInInfo) ToData() Data {
	return data.NewList(i.OutRef.ToData(), i.Resolved.ToData())
}

// AccountBalanceInterval is a V4 account balance interval variant.
type AccountBalanceInterval interface {
	ToData() Data
	isAccountBalanceInterval()
}

// AccountBalanceLowerBound represents an interval with only a lower bound.
type AccountBalanceLowerBound struct {
	Amount Data
}

func (AccountBalanceLowerBound) isAccountBalanceInterval() {}

// ToData serializes a lower-bound interval as constructor 0.
func (i AccountBalanceLowerBound) ToData() Data {
	return data.NewConstr(0, i.Amount)
}

// AccountBalanceUpperBound represents an interval with only an upper bound.
type AccountBalanceUpperBound struct {
	Amount Data
}

func (AccountBalanceUpperBound) isAccountBalanceInterval() {}

// ToData serializes an upper-bound interval as constructor 1.
func (i AccountBalanceUpperBound) ToData() Data {
	return data.NewConstr(1, i.Amount)
}

// AccountBalanceBothBounds represents an interval with lower and upper bounds.
type AccountBalanceBothBounds struct {
	Lower Data
	Upper Data
}

func (AccountBalanceBothBounds) isAccountBalanceInterval() {}

// ToData serializes a bounded interval as constructor 2.
func (i AccountBalanceBothBounds) ToData() Data {
	return data.NewConstr(2, i.Lower, i.Upper)
}

// AccountBalanceExact represents an exact account balance.
type AccountBalanceExact struct {
	Amount Data
}

func (AccountBalanceExact) isAccountBalanceInterval() {}

// ToData serializes an exact balance as constructor 3.
func (i AccountBalanceExact) ToData() Data {
	return data.NewConstr(3, i.Amount)
}

// ScriptPurpose is a V4 transaction purpose variant.
type ScriptPurpose interface {
	ToData() Data
	isScriptPurpose()
}

// MintingPurpose identifies a minting purpose.
type MintingPurpose struct {
	ScriptHash     Data
	CurrencySymbol Data
}

func (MintingPurpose) isScriptPurpose() {}

// ToData serializes a minting purpose as constructor 0.
func (p MintingPurpose) ToData() Data {
	return data.NewConstr(0, p.ScriptHash, p.CurrencySymbol)
}

// SpendingPurpose identifies a spending purpose.
type SpendingPurpose struct {
	ScriptHash Data
	OutRef     TxOutRef
}

func (SpendingPurpose) isScriptPurpose() {}

// ToData serializes a spending purpose as constructor 1.
func (p SpendingPurpose) ToData() Data {
	return data.NewConstr(1, p.ScriptHash, p.OutRef.ToData())
}

// WithdrawingPurpose identifies a withdrawal purpose.
type WithdrawingPurpose struct {
	ScriptHash Data
	Credential Data
}

func (WithdrawingPurpose) isScriptPurpose() {}

// ToData serializes a withdrawal purpose as constructor 2.
func (p WithdrawingPurpose) ToData() Data {
	return data.NewConstr(2, p.ScriptHash, p.Credential)
}

// CertifyingPurpose identifies a certificate purpose.
type CertifyingPurpose struct {
	ScriptHash  Data
	Index       Data
	Certificate Data
}

func (CertifyingPurpose) isScriptPurpose() {}

// ToData serializes a certificate purpose as constructor 3.
func (p CertifyingPurpose) ToData() Data {
	return data.NewConstr(3, p.ScriptHash, p.Index, p.Certificate)
}

// VotingPurpose identifies a voting purpose.
type VotingPurpose struct {
	ScriptHash Data
	Voter      Data
}

func (VotingPurpose) isScriptPurpose() {}

// ToData serializes a voting purpose as constructor 4.
func (p VotingPurpose) ToData() Data {
	return data.NewConstr(4, p.ScriptHash, p.Voter)
}

// ProposingPurpose identifies a proposal purpose.
type ProposingPurpose struct {
	ScriptHash        Data
	Index             Data
	ProposalProcedure Data
}

func (ProposingPurpose) isScriptPurpose() {}

// ToData serializes a proposal purpose as constructor 5.
func (p ProposingPurpose) ToData() Data {
	return data.NewConstr(5, p.ScriptHash, p.Index, p.ProposalProcedure)
}

// GuardingPurpose identifies the V4 guard purpose introduced for observing
// scripts. It corresponds to the Observe purpose from CIP-0112 and uses the
// Guarding name in the V4 ledger API.
type GuardingPurpose struct {
	ScriptHash Data
	Index      Data
}

func (GuardingPurpose) isScriptPurpose() {}

// ToData serializes a guarding purpose as constructor 6.
func (p GuardingPurpose) ToData() Data {
	return data.NewConstr(6, p.ScriptHash, p.Index)
}

// ObservingPurpose is an alias for GuardingPurpose, the V4 ledger name for the
// Observe purpose described by CIP-0112.
type ObservingPurpose = GuardingPurpose

// ScriptInfo is a V4 script information variant.
type ScriptInfo interface {
	ToData() Data
	isScriptInfo()
}

// MintingScript is information for a minting script.
type MintingScript struct {
	CurrencySymbol Data
}

func (MintingScript) isScriptInfo() {}

// ToData serializes minting script information as constructor 0.
func (s MintingScript) ToData() Data {
	return data.NewConstr(0, s.CurrencySymbol)
}

// SpendingScript is information for a spending script.
type SpendingScript struct {
	OutRef TxOutRef
	Datum  Maybe
}

func (SpendingScript) isScriptInfo() {}

// ToData serializes spending script information as constructor 1.
func (s SpendingScript) ToData() Data {
	return data.NewConstr(1, s.OutRef.ToData(), s.Datum.ToData())
}

// WithdrawingScript is information for a withdrawal script.
type WithdrawingScript struct {
	AccountID Data
}

func (WithdrawingScript) isScriptInfo() {}

// ToData serializes withdrawal script information as constructor 2.
func (s WithdrawingScript) ToData() Data {
	return data.NewConstr(2, s.AccountID)
}

// CertifyingScript is information for a certificate script.
type CertifyingScript struct {
	Index       Data
	Certificate Data
}

func (CertifyingScript) isScriptInfo() {}

// ToData serializes certificate script information as constructor 3.
func (s CertifyingScript) ToData() Data {
	return data.NewConstr(3, s.Index, s.Certificate)
}

// VotingScript is information for a voting script.
type VotingScript struct {
	Voter Data
}

func (VotingScript) isScriptInfo() {}

// ToData serializes voting script information as constructor 4.
func (s VotingScript) ToData() Data {
	return data.NewConstr(4, s.Voter)
}

// ProposingScript is information for a proposal script.
type ProposingScript struct {
	Index             Data
	ProposalProcedure Data
}

func (ProposingScript) isScriptInfo() {}

// ToData serializes proposal script information as constructor 5.
func (s ProposingScript) ToData() Data {
	return data.NewConstr(5, s.Index, s.ProposalProcedure)
}

// GuardingScript is information for a V4 guard script. TopTxInfo is absent
// when the guard runs in a sub-transaction and present at the top level.
type GuardingScript struct {
	Index     Data
	TopTxInfo Maybe
}

func (GuardingScript) isScriptInfo() {}

// ToData serializes guarding script information as constructor 6.
func (s GuardingScript) ToData() Data {
	return data.NewConstr(6, s.Index, s.TopTxInfo.ToData())
}

// ObservingScript is an alias for GuardingScript, the V4 ledger name for the
// Observe script type described by CIP-0112.
type ObservingScript = GuardingScript

// TxInfo is the Plutus V4 transaction information record. Each list or map
// field uses the corresponding Go slice and is encoded in place. TxCerts,
// credentials, redeemers, datums, voters, governance actions, and time ranges
// are accepted as their already-encoded Plutus data values.
type TxInfo struct {
	ID                      Data
	SubTxIndex              Maybe
	Inputs                  []Data
	ReferenceInputs         []Data
	Outputs                 []Data
	Mint                    Data
	TxCerts                 []Data
	Withdrawals             []MapEntry
	DirectDeposits          []MapEntry
	AccountBalanceIntervals []MapEntry
	ValidRange              Data
	Guards                  []Data
	RequiredTopLevelGuards  []MapEntry
	Redeemers               []MapEntry
	Data                    []MapEntry
	Votes                   []MapEntry
	ProposalProcedures      []Data
	CurrentTreasuryAmount   Maybe
	TreasuryDonation        Data
}

// ToData serializes TxInfo as its 19-field V4 list. V4 TxInfo has no isValid
// field; validity is represented by ValidRange.
func (t TxInfo) ToData() Data {
	return data.NewList(
		t.ID,
		t.SubTxIndex.ToData(),
		List(t.Inputs...),
		List(t.ReferenceInputs...),
		List(t.Outputs...),
		t.Mint,
		List(t.TxCerts...),
		mapData(t.Withdrawals),
		mapData(t.DirectDeposits),
		mapData(t.AccountBalanceIntervals),
		t.ValidRange,
		List(t.Guards...),
		mapData(t.RequiredTopLevelGuards),
		mapData(t.Redeemers),
		mapData(t.Data),
		mapData(t.Votes),
		List(t.ProposalProcedures...),
		t.CurrentTreasuryAmount.ToData(),
		t.TreasuryDonation,
	)
}

// TopTxInfoSimplified is the aggregate view of a V4 top-level transaction.
type TopTxInfoSimplified struct {
	IDs                    []Data
	Inputs                 []Data
	ReferenceInputs        []Data
	Outputs                []Data
	Mints                  Data
	Burns                  Data
	TxCerts                []Data
	Withdrawals            []MapEntry
	DirectDeposits         []MapEntry
	ValidRange             Data
	Guards                 []Data
	RequiredTopLevelGuards []Data
	RedeemerHashes         []Data
	Data                   []MapEntry
	Votes                  []MapEntry
	ProposalProcedures     []Data
	CurrentTreasuryAmount  Maybe
	TreasuryDonations      Data
}

// ToData serializes the 18 fields in the V4 top-level summary.
func (t TopTxInfoSimplified) ToData() Data {
	return data.NewList(
		List(t.IDs...),
		List(t.Inputs...),
		List(t.ReferenceInputs...),
		List(t.Outputs...),
		t.Mints,
		t.Burns,
		List(t.TxCerts...),
		mapData(t.Withdrawals),
		mapData(t.DirectDeposits),
		t.ValidRange,
		List(t.Guards...),
		List(t.RequiredTopLevelGuards...),
		List(t.RedeemerHashes...),
		mapData(t.Data),
		mapData(t.Votes),
		List(t.ProposalProcedures...),
		t.CurrentTreasuryAmount.ToData(),
		t.TreasuryDonations,
	)
}

// TopTxInfo contains nested sub-transaction contexts and the aggregate V4
// transaction view surfaced to a top-level guard.
type TopTxInfo struct {
	SubTransactions                 []TxInfo
	Datums                          []MapEntry
	StartingAccountBalanceIntervals Data
	Simplified                      TopTxInfoSimplified
}

// ToData serializes top-level transaction information as a four-field list.
func (t TopTxInfo) ToData() Data {
	subTransactions := make([]Data, len(t.SubTransactions))
	for i := range t.SubTransactions {
		subTransactions[i] = t.SubTransactions[i].ToData()
	}
	return data.NewList(
		List(subTransactions...),
		mapData(t.Datums),
		t.StartingAccountBalanceIntervals,
		t.Simplified.ToData(),
	)
}

// ScriptContext is the V4 context provided to a script.
type ScriptContext struct {
	TxInfo     TxInfo
	Redeemer   Data
	ScriptInfo ScriptInfo
	ScriptHash Data
}

// ToData serializes the four V4 script context fields in ledger order.
func (s ScriptContext) ToData() Data {
	var scriptInfo Data
	if s.ScriptInfo != nil {
		scriptInfo = s.ScriptInfo.ToData()
	}
	return data.NewList(s.TxInfo.ToData(), s.Redeemer, scriptInfo, s.ScriptHash)
}

func mapData(entries []MapEntry) Data {
	return Map(entries...)
}
