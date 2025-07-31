package builtin

import "fmt"

type DefaultFunction uint8

const (
	// Integer functions
	AddInteger            DefaultFunction = 0
	SubtractInteger       DefaultFunction = 1
	MultiplyInteger       DefaultFunction = 2
	DivideInteger         DefaultFunction = 3
	QuotientInteger       DefaultFunction = 4
	RemainderInteger      DefaultFunction = 5
	ModInteger            DefaultFunction = 6
	EqualsInteger         DefaultFunction = 7
	LessThanInteger       DefaultFunction = 8
	LessThanEqualsInteger DefaultFunction = 9
	// ByteString functions
	AppendByteString         DefaultFunction = 10
	ConsByteString           DefaultFunction = 11
	SliceByteString          DefaultFunction = 12
	LengthOfByteString       DefaultFunction = 13
	IndexByteString          DefaultFunction = 14
	EqualsByteString         DefaultFunction = 15
	LessThanByteString       DefaultFunction = 16
	LessThanEqualsByteString DefaultFunction = 17
	// Cryptography and hash functions
	Sha2_256                        DefaultFunction = 18
	Sha3_256                        DefaultFunction = 19
	Blake2b_256                     DefaultFunction = 20
	Keccak_256                      DefaultFunction = 71
	Blake2b_224                     DefaultFunction = 72
	VerifyEd25519Signature          DefaultFunction = 21
	VerifyEcdsaSecp256k1Signature   DefaultFunction = 52
	VerifySchnorrSecp256k1Signature DefaultFunction = 53
	// String functions
	AppendString DefaultFunction = 22
	EqualsString DefaultFunction = 23
	EncodeUtf8   DefaultFunction = 24
	DecodeUtf8   DefaultFunction = 25
	// Bool function
	IfThenElse DefaultFunction = 26
	// Unit function
	ChooseUnit DefaultFunction = 27
	// Tracing function
	Trace DefaultFunction = 28
	// Pairs functions
	FstPair DefaultFunction = 29
	SndPair DefaultFunction = 30
	// List functions
	ChooseList DefaultFunction = 31
	MkCons     DefaultFunction = 32
	HeadList   DefaultFunction = 33
	TailList   DefaultFunction = 34
	NullList   DefaultFunction = 35
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	ChooseData    DefaultFunction = 36
	ConstrData    DefaultFunction = 37
	MapData       DefaultFunction = 38
	ListData      DefaultFunction = 39
	IData         DefaultFunction = 40
	BData         DefaultFunction = 41
	UnConstrData  DefaultFunction = 42
	UnMapData     DefaultFunction = 43
	UnListData    DefaultFunction = 44
	UnIData       DefaultFunction = 45
	UnBData       DefaultFunction = 46
	EqualsData    DefaultFunction = 47
	SerialiseData DefaultFunction = 51
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	MkPairData    DefaultFunction = 48
	MkNilData     DefaultFunction = 49
	MkNilPairData DefaultFunction = 50

	// BLS Builtins
	Bls12_381_G1_Add         DefaultFunction = 54
	Bls12_381_G1_Neg         DefaultFunction = 55
	Bls12_381_G1_ScalarMul   DefaultFunction = 56
	Bls12_381_G1_Equal       DefaultFunction = 57
	Bls12_381_G1_Compress    DefaultFunction = 58
	Bls12_381_G1_Uncompress  DefaultFunction = 59
	Bls12_381_G1_HashToGroup DefaultFunction = 60
	Bls12_381_G2_Add         DefaultFunction = 61
	Bls12_381_G2_Neg         DefaultFunction = 62
	Bls12_381_G2_ScalarMul   DefaultFunction = 63
	Bls12_381_G2_Equal       DefaultFunction = 64
	Bls12_381_G2_Compress    DefaultFunction = 65
	Bls12_381_G2_Uncompress  DefaultFunction = 66
	Bls12_381_G2_HashToGroup DefaultFunction = 67
	Bls12_381_MillerLoop     DefaultFunction = 68
	Bls12_381_MulMlResult    DefaultFunction = 69
	Bls12_381_FinalVerify    DefaultFunction = 70

	// Conversions
	IntegerToByteString DefaultFunction = 73
	ByteStringToInteger DefaultFunction = 74

	// Logical
	AndByteString        DefaultFunction = 75
	OrByteString         DefaultFunction = 76
	XorByteString        DefaultFunction = 77
	ComplementByteString DefaultFunction = 78
	ReadBit              DefaultFunction = 79
	WriteBits            DefaultFunction = 80
	ReplicateByte        DefaultFunction = 81

	// Bitwise
	ShiftByteString  DefaultFunction = 82
	RotateByteString DefaultFunction = 83
	CountSetBits     DefaultFunction = 84
	FindFirstSetBit  DefaultFunction = 85

	// Ripemd_160
	Ripemd_160 DefaultFunction = 86

	// Batch 6
	ExpModInteger DefaultFunction = 87
	CaseList      DefaultFunction = 88
	CaseData      DefaultFunction = 89
	DropList      DefaultFunction = 90

	// Arrays
	LengthOfArray DefaultFunction = 91
	ListToArray   DefaultFunction = 92
	IndexArray    DefaultFunction = 93
)

var Builtins map[string]DefaultFunction = map[string]DefaultFunction{
	// Integer functions
	"addInteger":            AddInteger,
	"subtractInteger":       SubtractInteger,
	"multiplyInteger":       MultiplyInteger,
	"divideInteger":         DivideInteger,
	"quotientInteger":       QuotientInteger,
	"remainderInteger":      RemainderInteger,
	"modInteger":            ModInteger,
	"equalsInteger":         EqualsInteger,
	"lessThanInteger":       LessThanInteger,
	"lessThanEqualsInteger": LessThanEqualsInteger,
	// ByteString functions
	"appendByteString":         AppendByteString,
	"consByteString":           ConsByteString,
	"sliceByteString":          SliceByteString,
	"lengthOfByteString":       LengthOfByteString,
	"indexByteString":          IndexByteString,
	"equalsByteString":         EqualsByteString,
	"lessThanByteString":       LessThanByteString,
	"lessThanEqualsByteString": LessThanEqualsByteString,
	// Cryptography and hash functions
	"sha2_256":                        Sha2_256,
	"sha3_256":                        Sha3_256,
	"blake2b_256":                     Blake2b_256,
	"keccak_256":                      Keccak_256,
	"blake2b_224":                     Blake2b_224,
	"verifyEd25519Signature":          VerifyEd25519Signature,
	"verifyEcdsaSecp256k1Signature":   VerifyEcdsaSecp256k1Signature,
	"verifySchnorrSecp256k1Signature": VerifySchnorrSecp256k1Signature,
	// String functions
	"appendString": AppendString,
	"equalsString": EqualsString,
	"encodeUtf8":   EncodeUtf8,
	"decodeUtf8":   DecodeUtf8,
	// Bool function
	"ifThenElse": IfThenElse,
	// Unit function
	"chooseUnit": ChooseUnit,
	// Tracing function
	"trace": Trace,
	// Pairs functions
	"fstPair": FstPair,
	"sndPair": SndPair,
	// List functions
	"chooseList": ChooseList,
	"mkCons":     MkCons,
	"headList":   HeadList,
	"tailList":   TailList,
	"nullList":   NullList,
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	"chooseData":    ChooseData,
	"constrData":    ConstrData,
	"mapData":       MapData,
	"listData":      ListData,
	"iData":         IData,
	"bData":         BData,
	"unConstrData":  UnConstrData,
	"unMapData":     UnMapData,
	"unListData":    UnListData,
	"unIData":       UnIData,
	"unBData":       UnBData,
	"equalsData":    EqualsData,
	"serialiseData": SerialiseData,
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	"mkPairData":    MkPairData,
	"mkNilData":     MkNilData,
	"mkNilPairData": MkNilPairData,
	// BLS Builtins
	"bls12_381_G1_add":         Bls12_381_G1_Add,
	"bls12_381_G1_neg":         Bls12_381_G1_Neg,
	"bls12_381_G1_scalarMul":   Bls12_381_G1_ScalarMul,
	"bls12_381_G1_equal":       Bls12_381_G1_Equal,
	"bls12_381_G1_compress":    Bls12_381_G1_Compress,
	"bls12_381_G1_uncompress":  Bls12_381_G1_Uncompress,
	"bls12_381_G1_hashToGroup": Bls12_381_G1_HashToGroup,
	"bls12_381_G2_add":         Bls12_381_G2_Add,
	"bls12_381_G2_neg":         Bls12_381_G2_Neg,
	"bls12_381_G2_scalarMul":   Bls12_381_G2_ScalarMul,
	"bls12_381_G2_equal":       Bls12_381_G2_Equal,
	"bls12_381_G2_compress":    Bls12_381_G2_Compress,
	"bls12_381_G2_uncompress":  Bls12_381_G2_Uncompress,
	"bls12_381_G2_hashToGroup": Bls12_381_G2_HashToGroup,
	"bls12_381_millerLoop":     Bls12_381_MillerLoop,
	"bls12_381_mulMlResult":    Bls12_381_MulMlResult,
	"bls12_381_finalVerify":    Bls12_381_FinalVerify,
	// Conversions
	"integerToByteString": IntegerToByteString,
	"byteStringToInteger": ByteStringToInteger,
	// Logical
	"andByteString":        AndByteString,
	"orByteString":         OrByteString,
	"xorByteString":        XorByteString,
	"complementByteString": ComplementByteString,
	"readBit":              ReadBit,
	"writeBits":            WriteBits,
	"replicateByte":        ReplicateByte,
	// Bitwise
	"shiftByteString":  ShiftByteString,
	"rotateByteString": RotateByteString,
	"countSetBits":     CountSetBits,
	"findFirstSetBit":  FindFirstSetBit,
	// Ripemd_160
	"ripemd_160": Ripemd_160,
	// Batch 6
	"expModInteger": ExpModInteger,
	"caseList":      CaseList,
	"caseData":      CaseData,
	"dropList":      DropList,
	// Arrays
	"lengthOfArray": LengthOfArray,
	"listToArray":   ListToArray,
	"indexArray":    IndexArray,
}

var defaultFunctionForceCount = [TotalBuiltinCount]int{
	// Integer functions
	AddInteger:            0,
	SubtractInteger:       0,
	MultiplyInteger:       0,
	DivideInteger:         0,
	QuotientInteger:       0,
	RemainderInteger:      0,
	ModInteger:            0,
	EqualsInteger:         0,
	LessThanInteger:       0,
	LessThanEqualsInteger: 0,
	// ByteString functions
	AppendByteString:         0,
	ConsByteString:           0,
	SliceByteString:          0,
	LengthOfByteString:       0,
	IndexByteString:          0,
	EqualsByteString:         0,
	LessThanByteString:       0,
	LessThanEqualsByteString: 0,
	// Cryptography and hash functions
	Sha2_256:                        0,
	Sha3_256:                        0,
	Blake2b_256:                     0,
	Keccak_256:                      0,
	Blake2b_224:                     0,
	VerifyEd25519Signature:          0,
	VerifyEcdsaSecp256k1Signature:   0,
	VerifySchnorrSecp256k1Signature: 0,
	// String functions
	AppendString: 0,
	EqualsString: 0,
	EncodeUtf8:   0,
	DecodeUtf8:   0,
	// Bool function
	IfThenElse: 1,
	// Unit function
	ChooseUnit: 1,
	// Tracing function
	Trace: 1,
	// Pairs functions
	FstPair: 2,
	SndPair: 2,
	// List functions
	ChooseList: 2,
	MkCons:     1,
	HeadList:   1,
	TailList:   1,
	NullList:   1,
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	ChooseData:    1,
	ConstrData:    0,
	MapData:       0,
	ListData:      0,
	IData:         0,
	BData:         0,
	UnConstrData:  0,
	UnMapData:     0,
	UnListData:    0,
	UnIData:       0,
	UnBData:       0,
	EqualsData:    0,
	SerialiseData: 0,
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	MkPairData:               0,
	MkNilData:                0,
	MkNilPairData:            0,
	Bls12_381_G1_Add:         0,
	Bls12_381_G1_Neg:         0,
	Bls12_381_G1_ScalarMul:   0,
	Bls12_381_G1_Equal:       0,
	Bls12_381_G1_Compress:    0,
	Bls12_381_G1_Uncompress:  0,
	Bls12_381_G1_HashToGroup: 0,
	Bls12_381_G2_Add:         0,
	Bls12_381_G2_Neg:         0,
	Bls12_381_G2_ScalarMul:   0,
	Bls12_381_G2_Equal:       0,
	Bls12_381_G2_Compress:    0,
	Bls12_381_G2_Uncompress:  0,
	Bls12_381_G2_HashToGroup: 0,
	Bls12_381_MillerLoop:     0,
	Bls12_381_MulMlResult:    0,
	Bls12_381_FinalVerify:    0,
	IntegerToByteString:      0,
	ByteStringToInteger:      0,
	AndByteString:            0,
	OrByteString:             0,
	XorByteString:            0,
	ComplementByteString:     0,
	ReadBit:                  0,
	WriteBits:                0,
	ReplicateByte:            0,
	ShiftByteString:          0,
	RotateByteString:         0,
	CountSetBits:             0,
	FindFirstSetBit:          0,
	Ripemd_160:               0,
	// Batch 6
	ExpModInteger: 0,
	CaseList:      2,
	CaseData:      1,
	DropList:      1,
	// Arrays
	LengthOfArray: 1,
	ListToArray:   1,
	IndexArray:    1,
}

func (f DefaultFunction) ForceCount() int {
	return defaultFunctionForceCount[f]
}

var defaultFunctionArity = [TotalBuiltinCount]int{
	// Integer functions
	AddInteger:            2,
	SubtractInteger:       2,
	MultiplyInteger:       2,
	DivideInteger:         2,
	QuotientInteger:       2,
	RemainderInteger:      2,
	ModInteger:            2,
	EqualsInteger:         2,
	LessThanInteger:       2,
	LessThanEqualsInteger: 2,
	// ByteString functions
	AppendByteString:         2,
	ConsByteString:           2,
	SliceByteString:          3,
	LengthOfByteString:       1,
	IndexByteString:          2,
	EqualsByteString:         2,
	LessThanByteString:       2,
	LessThanEqualsByteString: 2,
	// Cryptography and hash functions
	Sha2_256:                        1,
	Sha3_256:                        1,
	Blake2b_256:                     1,
	Keccak_256:                      1,
	Blake2b_224:                     1,
	VerifyEd25519Signature:          3,
	VerifyEcdsaSecp256k1Signature:   3,
	VerifySchnorrSecp256k1Signature: 3,
	// String functions
	AppendString: 2,
	EqualsString: 2,
	EncodeUtf8:   1,
	DecodeUtf8:   1,
	// Bool function
	IfThenElse: 3,
	// Unit function
	ChooseUnit: 2,
	// Tracing function
	Trace: 2,
	// Pairs functions
	FstPair: 1,
	SndPair: 1,
	// List functions
	ChooseList: 3,
	MkCons:     2,
	HeadList:   1,
	TailList:   1,
	NullList:   1,
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	ChooseData:    6,
	ConstrData:    2,
	MapData:       1,
	ListData:      1,
	IData:         1,
	BData:         1,
	UnConstrData:  1,
	UnMapData:     1,
	UnListData:    1,
	UnIData:       1,
	UnBData:       1,
	EqualsData:    2,
	SerialiseData: 1,
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	MkPairData:               2,
	MkNilData:                1,
	MkNilPairData:            1,
	Bls12_381_G1_Add:         2,
	Bls12_381_G1_Neg:         1,
	Bls12_381_G1_ScalarMul:   2,
	Bls12_381_G1_Equal:       2,
	Bls12_381_G1_Compress:    1,
	Bls12_381_G1_Uncompress:  1,
	Bls12_381_G1_HashToGroup: 2,
	Bls12_381_G2_Add:         2,
	Bls12_381_G2_Neg:         1,
	Bls12_381_G2_ScalarMul:   2,
	Bls12_381_G2_Equal:       2,
	Bls12_381_G2_Compress:    1,
	Bls12_381_G2_Uncompress:  1,
	Bls12_381_G2_HashToGroup: 2,
	Bls12_381_MillerLoop:     2,
	Bls12_381_MulMlResult:    2,
	Bls12_381_FinalVerify:    2,
	IntegerToByteString:      3,
	ByteStringToInteger:      2,
	AndByteString:            3,
	OrByteString:             3,
	XorByteString:            3,
	ComplementByteString:     1,
	ReadBit:                  2,
	WriteBits:                3,
	ReplicateByte:            2,
	ShiftByteString:          2,
	RotateByteString:         2,
	CountSetBits:             1,
	FindFirstSetBit:          1,
	Ripemd_160:               1,
	// Batch 6
	ExpModInteger: 3,
	CaseList:      3,
	CaseData:      6,
	DropList:      2,
	// Arrays
	LengthOfArray: 1,
	ListToArray:   1,
	IndexArray:    2,
}

func (f DefaultFunction) Arity() int {
	return defaultFunctionArity[f]
}

var defaultFunctionToString = [TotalBuiltinCount]string{
	// Integer functions
	AddInteger:            "addInteger",
	SubtractInteger:       "subtractInteger",
	MultiplyInteger:       "multiplyInteger",
	DivideInteger:         "divideInteger",
	QuotientInteger:       "quotientInteger",
	RemainderInteger:      "remainderInteger",
	ModInteger:            "modInteger",
	EqualsInteger:         "equalsInteger",
	LessThanInteger:       "lessThanInteger",
	LessThanEqualsInteger: "lessThanEqualsInteger",
	// ByteString functions
	AppendByteString:         "appendByteString",
	ConsByteString:           "consByteString",
	SliceByteString:          "sliceByteString",
	LengthOfByteString:       "lengthOfByteString",
	IndexByteString:          "indexByteString",
	EqualsByteString:         "equalsByteString",
	LessThanByteString:       "lessThanByteString",
	LessThanEqualsByteString: "lessThanEqualsByteString",
	// Cryptography and hash functions
	Sha2_256:                        "sha2_256",
	Sha3_256:                        "sha3_256",
	Blake2b_256:                     "blake2b_256",
	Keccak_256:                      "keccak_256",
	Blake2b_224:                     "blake2b_224",
	VerifyEd25519Signature:          "verifyEd25519Signature",
	VerifyEcdsaSecp256k1Signature:   "verifyEcdsaSecp256k1Signature",
	VerifySchnorrSecp256k1Signature: "verifySchnorrSecp256k1Signature",
	// String functions
	AppendString: "appendString",
	EqualsString: "equalsString",
	EncodeUtf8:   "encodeUtf8",
	DecodeUtf8:   "decodeUtf8",
	// Bool function
	IfThenElse: "ifThenElse",
	// Unit function
	ChooseUnit: "chooseUnit",
	// Tracing function
	Trace: "trace",
	// Pairs functions
	FstPair: "fstPair",
	SndPair: "sndPair",
	// List functions
	ChooseList: "chooseList",
	MkCons:     "mkCons",
	HeadList:   "headList",
	TailList:   "tailList",
	NullList:   "nullList",
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	ChooseData:    "chooseData",
	ConstrData:    "constrData",
	MapData:       "mapData",
	ListData:      "listData",
	IData:         "iData",
	BData:         "bData",
	UnConstrData:  "unConstrData",
	UnMapData:     "unMapData",
	UnListData:    "unListData",
	UnIData:       "unIData",
	UnBData:       "unBData",
	EqualsData:    "equalsData",
	SerialiseData: "serialiseData",
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	MkPairData:               "mkPairData",
	MkNilData:                "mkNilData",
	MkNilPairData:            "mkNilPairData",
	Bls12_381_G1_Add:         "bls12_381_G1_add",
	Bls12_381_G1_Neg:         "bls12_381_G1_neg",
	Bls12_381_G1_ScalarMul:   "bls12_381_G1_scalarMul",
	Bls12_381_G1_Equal:       "bls12_381_G1_equal",
	Bls12_381_G1_Compress:    "bls12_381_G1_compress",
	Bls12_381_G1_Uncompress:  "bls12_381_G1_uncompress",
	Bls12_381_G1_HashToGroup: "bls12_381_G1_hashToGroup",
	Bls12_381_G2_Add:         "bls12_381_G2_add",
	Bls12_381_G2_Neg:         "bls12_381_G2_neg",
	Bls12_381_G2_ScalarMul:   "bls12_381_G2_scalarMul",
	Bls12_381_G2_Equal:       "bls12_381_G2_equal",
	Bls12_381_G2_Compress:    "bls12_381_G2_compress",
	Bls12_381_G2_Uncompress:  "bls12_381_G2_uncompress",
	Bls12_381_G2_HashToGroup: "bls12_381_G2_hashToGroup",
	Bls12_381_MillerLoop:     "bls12_381_millerLoop",
	Bls12_381_MulMlResult:    "bls12_381_mulMlResult",
	Bls12_381_FinalVerify:    "bls12_381_finalVerify",
	IntegerToByteString:      "integerToByteString",
	ByteStringToInteger:      "byteStringToInteger",
	AndByteString:            "andByteString",
	OrByteString:             "orByteString",
	XorByteString:            "xorByteString",
	ComplementByteString:     "complementByteString",
	ReadBit:                  "readBit",
	WriteBits:                "writeBits",
	ReplicateByte:            "replicateByte",
	ShiftByteString:          "shiftByteString",
	RotateByteString:         "rotateByteString",
	CountSetBits:             "countSetBits",
	FindFirstSetBit:          "findFirstSetBit",
	Ripemd_160:               "ripemd_160",
	// Batch 6
	ExpModInteger: "expModInteger",
	CaseList:      "caseList",
	CaseData:      "caseData",
	DropList:      "dropList",
	// Arrays
	LengthOfArray: "lengthOfArray",
	ListToArray:   "listToArray",
	IndexArray:    "indexArray",
}

func (f DefaultFunction) String() string {
	return defaultFunctionToString[f]
}

// Smallest DefaultFunction
const MinDefaultFunction byte = 0

// Smallest DefaultFunction
const MaxDefaultFunction byte = 93

// Total Builtin Count
const TotalBuiltinCount byte = MaxDefaultFunction + 1

func FromByte(tag byte) (DefaultFunction, error) {
	// only need to check if greater than because
	// the lowest possible value for byte is zero anyways
	if tag > MaxDefaultFunction {
		return 0, fmt.Errorf("DefaultFunctionNotFound(%d)", tag)
	}

	return DefaultFunction(tag), nil
}
