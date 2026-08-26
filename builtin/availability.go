package builtin

import "slices"

// PlutusVersion represents a Plutus ledger language version.
// This is used to determine which builtins are available.
type PlutusVersion int

const (
	PlutusV1 PlutusVersion = 1
	PlutusV2 PlutusVersion = 2
	PlutusV3 PlutusVersion = 3
	PlutusV4 PlutusVersion = 4

	// PlutusVUnreleased is reserved for protocol-independent legacy metadata.
	// Live availability must be checked with IsAvailableInWithProto.
	PlutusVUnreleased PlutusVersion = 999
)

// builtinIntroducedIn maps each builtin to its protocol-independent legacy
// language metadata. Live availability must be checked with
// IsAvailableInWithProto because ledger languages receive batches at different
// protocol versions.
var builtinIntroducedIn = [TotalBuiltinCount]PlutusVersion{
	// V1 (Alonzo) - Original builtins
	// Integer functions
	AddInteger:            PlutusV1,
	SubtractInteger:       PlutusV1,
	MultiplyInteger:       PlutusV1,
	DivideInteger:         PlutusV1,
	QuotientInteger:       PlutusV1,
	RemainderInteger:      PlutusV1,
	ModInteger:            PlutusV1,
	EqualsInteger:         PlutusV1,
	LessThanInteger:       PlutusV1,
	LessThanEqualsInteger: PlutusV1,
	// ByteString functions
	AppendByteString:         PlutusV1,
	ConsByteString:           PlutusV1,
	SliceByteString:          PlutusV1,
	LengthOfByteString:       PlutusV1,
	IndexByteString:          PlutusV1,
	EqualsByteString:         PlutusV1,
	LessThanByteString:       PlutusV1,
	LessThanEqualsByteString: PlutusV1,
	// Cryptography and hash functions (V1)
	Sha2_256:               PlutusV1,
	Sha3_256:               PlutusV1,
	Blake2b_256:            PlutusV1,
	VerifyEd25519Signature: PlutusV1,
	// String functions
	AppendString: PlutusV1,
	EqualsString: PlutusV1,
	EncodeUtf8:   PlutusV1,
	DecodeUtf8:   PlutusV1,
	// Bool function
	IfThenElse: PlutusV1,
	// Unit function
	ChooseUnit: PlutusV1,
	// Tracing function
	Trace: PlutusV1,
	// Pairs functions
	FstPair: PlutusV1,
	SndPair: PlutusV1,
	// List functions
	ChooseList: PlutusV1,
	MkCons:     PlutusV1,
	HeadList:   PlutusV1,
	TailList:   PlutusV1,
	NullList:   PlutusV1,
	// Data functions
	ChooseData:   PlutusV1,
	ConstrData:   PlutusV1,
	MapData:      PlutusV1,
	ListData:     PlutusV1,
	IData:        PlutusV1,
	BData:        PlutusV1,
	UnConstrData: PlutusV1,
	UnMapData:    PlutusV1,
	UnListData:   PlutusV1,
	UnIData:      PlutusV1,
	UnBData:      PlutusV1,
	EqualsData:   PlutusV1,
	// Misc constructors
	MkPairData:    PlutusV1,
	MkNilData:     PlutusV1,
	MkNilPairData: PlutusV1,

	// V2 (Vasil) - Added builtins
	SerialiseData:                   PlutusV2,
	VerifyEcdsaSecp256k1Signature:   PlutusV2,
	VerifySchnorrSecp256k1Signature: PlutusV2,

	// V3 (Chang) - Added builtins
	// BLS12-381 operations
	Bls12_381_G1_Add:         PlutusV3,
	Bls12_381_G1_Neg:         PlutusV3,
	Bls12_381_G1_ScalarMul:   PlutusV3,
	Bls12_381_G1_Equal:       PlutusV3,
	Bls12_381_G1_Compress:    PlutusV3,
	Bls12_381_G1_Uncompress:  PlutusV3,
	Bls12_381_G1_HashToGroup: PlutusV3,
	Bls12_381_G2_Add:         PlutusV3,
	Bls12_381_G2_Neg:         PlutusV3,
	Bls12_381_G2_ScalarMul:   PlutusV3,
	Bls12_381_G2_Equal:       PlutusV3,
	Bls12_381_G2_Compress:    PlutusV3,
	Bls12_381_G2_Uncompress:  PlutusV3,
	Bls12_381_G2_HashToGroup: PlutusV3,
	Bls12_381_MillerLoop:     PlutusV3,
	Bls12_381_MulMlResult:    PlutusV3,
	Bls12_381_FinalVerify:    PlutusV3,
	// Additional hash functions
	Keccak_256:  PlutusV3,
	Blake2b_224: PlutusV3,
	Ripemd_160:  PlutusV3,
	// Integer/ByteString conversions
	IntegerToByteString: PlutusV3,
	ByteStringToInteger: PlutusV3,
	// Bitwise operations
	AndByteString:        PlutusV3,
	OrByteString:         PlutusV3,
	XorByteString:        PlutusV3,
	ComplementByteString: PlutusV3,
	ReadBit:              PlutusV3,
	WriteBits:            PlutusV3,
	ReplicateByte:        PlutusV3,
	ShiftByteString:      PlutusV3,
	RotateByteString:     PlutusV3,
	CountSetBits:         PlutusV3,
	FindFirstSetBit:      PlutusV3,
	// Modular exponentiation
	ExpModInteger: PlutusV3,

	// V4 (Conway+) - Added builtins
	// Array operations
	LengthOfArray: PlutusV4,
	ListToArray:   PlutusV4,
	IndexArray:    PlutusV4,
	// Multi-scalar multiplication
	Bls12_381_G1_MultiScalarMul: PlutusV4,
	Bls12_381_G2_MultiScalarMul: PlutusV4,
	// Value/coin operations
	InsertCoin:    PlutusV4,
	LookupCoin:    PlutusV4,
	ScaleValue:    PlutusV4,
	UnionValue:    PlutusV4,
	ValueContains: PlutusV4,
	// Value/Data conversion builtins
	ValueData:   PlutusV4,
	UnValueData: PlutusV4,
	// Unreleased builtins - defined but not yet available on mainnet
	DropList:        PlutusVUnreleased,
	MultiIndexArray: PlutusVUnreleased,
	Policies:        PlutusVUnreleased,
	AssetCount:      PlutusVUnreleased,
}

// VanRossemProtoVersion is the Cardano protocol major version at which batch 6
// becomes available. Availability remains specific to the Plutus ledger
// language; it is not a global switch for every builtin.
const VanRossemProtoVersion uint = 11

const (
	alonzoProtoVersion    uint = 5
	vasilProtoVersion     uint = 7
	valentineProtoVersion uint = 8
	changProtoVersion     uint = 9
	plominProtoVersion    uint = 10
	dijkstraProtoVersion  uint = 12
)

type availabilityBatch struct {
	introducedAt uint
	functions    []DefaultFunction
}

func builtinRange(first, last DefaultFunction) []DefaultFunction {
	ret := make([]DefaultFunction, 0, int(last-first)+1)
	for f := first; f <= last; f++ {
		ret = append(ret, f)
	}
	return ret
}

func combineBatches(batches ...[]DefaultFunction) []DefaultFunction {
	length := 0
	for _, batch := range batches {
		length += len(batch)
	}
	ret := make([]DefaultFunction, 0, length)
	for _, batch := range batches {
		ret = append(ret, batch...)
	}
	return ret
}

var (
	batch1  = builtinRange(AddInteger, MkNilPairData)
	batch2  = []DefaultFunction{SerialiseData}
	batch3  = []DefaultFunction{VerifyEcdsaSecp256k1Signature, VerifySchnorrSecp256k1Signature}
	batch4a = builtinRange(Bls12_381_G1_Add, Blake2b_224)
	batch4b = []DefaultFunction{IntegerToByteString, ByteStringToInteger}
	batch5  = builtinRange(AndByteString, Ripemd_160)
	batch6  = builtinRange(ExpModInteger, ScaleValue)
)

// builtinAvailability mirrors PlutusLedgerApi.Common.Versions. A builtin's
// availability depends on the (ledger language, protocol version) pair: later
// protocol versions can extend an existing language without extending every
// other language at the same time.
var builtinAvailability = map[PlutusVersion][]availabilityBatch{
	PlutusV1: {
		{introducedAt: alonzoProtoVersion, functions: batch1},
		{
			introducedAt: VanRossemProtoVersion,
			functions:    combineBatches(batch2, batch3, batch4a, batch4b, batch5, batch6),
		},
	},
	PlutusV2: {
		{introducedAt: vasilProtoVersion, functions: combineBatches(batch1, batch2)},
		{introducedAt: valentineProtoVersion, functions: batch3},
		{introducedAt: plominProtoVersion, functions: batch4b},
		{
			introducedAt: VanRossemProtoVersion,
			functions:    combineBatches(batch4a, batch5, batch6),
		},
	},
	PlutusV3: {
		{
			introducedAt: changProtoVersion,
			functions:    combineBatches(batch1, batch2, batch3, batch4a, batch4b),
		},
		{introducedAt: plominProtoVersion, functions: batch5},
		{introducedAt: VanRossemProtoVersion, functions: batch6},
	},
	PlutusV4: {
		{
			introducedAt: dijkstraProtoVersion,
			functions:    combineBatches(batch1, batch2, batch3, batch4a, batch4b, batch5, batch6),
		},
	},
}

// IntroducedIn returns the builtin's protocol-independent legacy language
// metadata. Use IsAvailableInWithProto for live ledger availability.
func (f DefaultFunction) IntroducedIn() PlutusVersion {
	return builtinIntroducedIn[f]
}

// IsAvailableIn returns the legacy language-only availability. Use
// IsAvailableInWithProto for live ledger availability.
func (f DefaultFunction) IsAvailableIn(version PlutusVersion) bool {
	return builtinIntroducedIn[f] <= version
}

// IsAvailableInWithProto returns true if the builtin is available for the
// supplied Plutus ledger language and Cardano protocol major version.
func (f DefaultFunction) IsAvailableInWithProto(
	version PlutusVersion,
	protoMajor uint,
) bool {
	if protoMajor == 0 {
		return f.IsAvailableIn(version)
	}
	for _, batch := range builtinAvailability[version] {
		if protoMajor < batch.introducedAt {
			continue
		}
		if slices.Contains(batch.functions, f) {
			return true
		}
	}
	return false
}

// LanguageVersionToPlutusVersion converts a [3]uint32 language version to PlutusVersion.
// Returns PlutusV1 as default for unrecognized versions.
func LanguageVersionToPlutusVersion(version [3]uint32) PlutusVersion {
	switch {
	case version[0] == 1 && version[1] == 0 && version[2] == 0:
		return PlutusV1
	case version[0] == 1 && version[1] == 1 && version[2] == 0:
		return PlutusV2
	case version[0] == 1 && version[1] == 2 && version[2] == 0:
		return PlutusV3
	case version[0] == 1 && version[1] >= 3:
		return PlutusV4
	default:
		// For unknown versions, be conservative and use V1
		return PlutusV1
	}
}
