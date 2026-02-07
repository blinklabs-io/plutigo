package builtin

// PlutusVersion represents a Plutus ledger language version.
// This is used to determine which builtins are available.
type PlutusVersion int

const (
	PlutusV1 PlutusVersion = 1
	PlutusV2 PlutusVersion = 2
	PlutusV3 PlutusVersion = 3
	PlutusV4 PlutusVersion = 4

	// PlutusVUnreleased represents builtins that are defined but not yet
	// available on mainnet. These will fail availability checks for all versions.
	PlutusVUnreleased PlutusVersion = 999
)

// builtinIntroducedIn maps each builtin to the Plutus version it was introduced in.
// Builtins can only be used in their introduced version or later.
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
	// Multi-index array
	MultiIndexArray: PlutusV4,

	// Value/Data conversion builtins
	ValueData:   PlutusV4,
	UnValueData: PlutusV4,

	// Unreleased builtins - defined but not yet available on mainnet
	DropList: PlutusVUnreleased,
}

// VanRossemProtoVersion is the Cardano protocol major version at which
// all builtins become available in all Plutus language versions.
const VanRossemProtoVersion uint = 11

// IntroducedIn returns the Plutus version in which the builtin was introduced.
func (f DefaultFunction) IntroducedIn() PlutusVersion {
	return builtinIntroducedIn[f]
}

// IsAvailableIn returns true if the builtin is available in the given Plutus version.
func (f DefaultFunction) IsAvailableIn(version PlutusVersion) bool {
	return builtinIntroducedIn[f] <= version
}

// IsAvailableInWithProto returns true if the builtin is available given the
// Plutus language version and Cardano protocol major version.
//
// At protocol version >= 11 (van Rossem hard fork), all builtins become
// available in all language versions, with two exceptions:
//   - MultiIndexArray remains V4-only (not part of PV11 cost model params)
//   - Builtins marked PlutusVUnreleased that are NOT activated at PV11 remain unavailable
//
// For protocol versions < 11, the original version-based gating applies.
func (f DefaultFunction) IsAvailableInWithProto(version PlutusVersion, protoMajor uint) bool {
	if protoMajor >= VanRossemProtoVersion {
		introduced := builtinIntroducedIn[f]
		// MultiIndexArray is V4-only, not part of PV11
		if f == MultiIndexArray {
			return introduced <= version
		}
		// DropList becomes available at PV11 in all versions
		if introduced == PlutusVUnreleased {
			return f == DropList
		}
		// All other builtins are available in all versions at PV11
		return true
	}
	// Pre-PV11: use language version gating
	return builtinIntroducedIn[f] <= version
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
