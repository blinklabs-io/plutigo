package scriptcontext

import (
	lcommon "github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/pkg/data"
)

type ScriptContext interface {
	isScriptContext()
	ToPlutusData() data.PlutusData
}

type ScriptContextV1V2 struct {
	TxInfo  TxInfo
	Purpose ScriptPurpose
}

func (ScriptContextV1V2) isScriptContext() {}

func (s ScriptContextV1V2) ToPlutusData() data.PlutusData {
	// TODO
	return nil
}

type ScriptContextV3 struct {
	TxInfo   TxInfo
	Redeemer data.PlutusData
	Purpose  ScriptInfo
}

func (ScriptContextV3) isScriptContext() {}

func (s ScriptContextV3) ToPlutusData() data.PlutusData {
	// TODO
	return nil
}

type TxInfo interface {
	isTxInfo()
	ToPlutusData() data.PlutusData
}

type TxInfoV1 struct {
	Inputs       []lcommon.Utxo
	Ouputs       []lcommon.Utxo
	Fee          uint64
	Mint         lcommon.MultiAsset[lcommon.MultiAssetTypeMint]
	Certificates []lcommon.Certificate
	// TODO
	// pub withdrawals: Vec<(Address, Coin)>,
	ValidRange  TimeRange
	Signatories []lcommon.Blake2b224
	Data        KeyValuePairs[lcommon.Blake2b256, data.PlutusData]
	// TODO
	// pub redeemers: KeyValuePairs<ScriptPurpose, Redeemer>,
	Id lcommon.Blake2b256
}

func (TxInfoV1) isTxInfo() {}

func (t TxInfoV1) ToPlutusData() data.PlutusData {
	// TODO
	return nil
}

type TxInfoV2 struct {
	Inputs          []lcommon.Utxo
	ReferenceInputs []lcommon.Utxo
	Ouputs          []lcommon.Utxo
	Fee             uint64
	Mint            lcommon.MultiAsset[lcommon.MultiAssetTypeMint]
	Certificates    []lcommon.Certificate
	// TODO
	// pub withdrawals: KeyValuePairs<Address, Coin>,
	ValidRange  TimeRange
	Signatories []lcommon.Blake2b224
	// TODO
	// pub redeemers: KeyValuePairs<ScriptPurpose, Redeemer>,
	Data KeyValuePairs[lcommon.Blake2b256, data.PlutusData]
	Id   lcommon.Blake2b256
}

func (TxInfoV2) isTxInfo() {}

func (t TxInfoV2) ToPlutusData() data.PlutusData {
	// TODO
	return nil
}

type TxInfoV3 struct {
	Inputs          []lcommon.Utxo
	ReferenceInputs []lcommon.Utxo
	Ouputs          []lcommon.Utxo
	Fee             uint64
	Mint            lcommon.MultiAsset[lcommon.MultiAssetTypeMint]
	Certificates    []lcommon.Certificate
	// TODO
	// pub withdrawals: KeyValuePairs<Address, Coin>,
	ValidRange  TimeRange
	Signatories []lcommon.Blake2b224
	// TODO
	// pub redeemers: KeyValuePairs<ScriptPurpose, Redeemer>,
	Data                  KeyValuePairs[lcommon.Blake2b256, data.PlutusData]
	Id                    lcommon.Blake2b256
	Votes                 KeyValuePairs[lcommon.Voter, KeyValuePairs[lcommon.GovActionId, lcommon.VotingProcedure]]
	ProposalProcedures    []lcommon.ProposalProcedure
	CurrentTreasuryAmount Option[Coin]
	TreasuryDonation      Option[PositiveCoin]
}

func (TxInfoV3) isTxInfo() {}

func (t TxInfoV3) ToPlutusData() data.PlutusData {
	// TODO
	return nil
}

type TimeRange struct {
	lowerBound uint64
	upperBound uint64
}

/*
pub enum ScriptInfo<T> {
    Minting(PolicyId),
    Spending(TransactionInput, T),
    Rewarding(StakeCredential),
    Certifying(usize, Certificate),
    Voting(Voter),
    Proposing(usize, ProposalProcedure),
}
*/
