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
	"verifyEcdsaSecp256K1Signature":   VerifyEcdsaSecp256k1Signature,
	"verifySchnorrSecp256K1Signature": VerifySchnorrSecp256k1Signature,
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
}

func (f DefaultFunction) ForceCount() int {
	switch f {
	// Integer functions
	case AddInteger:
		return 0
	case SubtractInteger:
		return 0
	case MultiplyInteger:
		return 0
	case DivideInteger:
		return 0
	case QuotientInteger:
		return 0
	case RemainderInteger:
		return 0
	case ModInteger:
		return 0
	case EqualsInteger:
		return 0
	case LessThanInteger:
		return 0
	case LessThanEqualsInteger:
		return 0
	// ByteString functions
	case AppendByteString:
		return 0
	case ConsByteString:
		return 0
	case SliceByteString:
		return 0
	case LengthOfByteString:
		return 0
	case IndexByteString:
		return 0
	case EqualsByteString:
		return 0
	case LessThanByteString:
		return 0
	case LessThanEqualsByteString:
		return 0
	// Cryptography and hash functions
	case Sha2_256:
		return 0
	case Sha3_256:
		return 0
	case Blake2b_256:
		return 0
	case Keccak_256:
		return 0
	case Blake2b_224:
		return 0
	case VerifyEd25519Signature:
		return 0
	case VerifyEcdsaSecp256k1Signature:
		return 0
	case VerifySchnorrSecp256k1Signature:
		return 0
	// String functions
	case AppendString:
		return 0
	case EqualsString:
		return 0
	case EncodeUtf8:
		return 0
	case DecodeUtf8:
		return 0
	// Bool function
	case IfThenElse:
		return 1
	// Unit function
	case ChooseUnit:
		return 1
	// Tracing function
	case Trace:
		return 1
	// Pairs functions
	case FstPair:
		return 2
	case SndPair:
		return 2
	// List functions
	case ChooseList:
		return 2
	case MkCons:
		return 1
	case HeadList:
		return 1
	case TailList:
		return 1
	case NullList:
		return 1
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	case ChooseData:
		return 1
	case ConstrData:
		return 0
	case MapData:
		return 0
	case ListData:
		return 0
	case IData:
		return 0
	case BData:
		return 0
	case UnConstrData:
		return 0
	case UnMapData:
		return 0
	case UnListData:
		return 0
	case UnIData:
		return 0
	case UnBData:
		return 0
	case EqualsData:
		return 0
	case SerialiseData:
		return 0
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	case MkPairData:
		return 0
	case MkNilData:
		return 0
	case MkNilPairData:
		return 0
	case Bls12_381_G1_Add:
		return 0
	case Bls12_381_G1_Neg:
		return 0
	case Bls12_381_G1_ScalarMul:
		return 0
	case Bls12_381_G1_Equal:
		return 0
	case Bls12_381_G1_Compress:
		return 0
	case Bls12_381_G1_Uncompress:
		return 0
	case Bls12_381_G1_HashToGroup:
		return 0
	case Bls12_381_G2_Add:
		return 0
	case Bls12_381_G2_Neg:
		return 0
	case Bls12_381_G2_ScalarMul:
		return 0
	case Bls12_381_G2_Equal:
		return 0
	case Bls12_381_G2_Compress:
		return 0
	case Bls12_381_G2_Uncompress:
		return 0
	case Bls12_381_G2_HashToGroup:
		return 0
	case Bls12_381_MillerLoop:
		return 0
	case Bls12_381_MulMlResult:
		return 0
	case Bls12_381_FinalVerify:
		return 0
	case IntegerToByteString:
		return 0
	case ByteStringToInteger:
		return 0
	case AndByteString:
		return 0
	case OrByteString:
		return 0
	case XorByteString:
		return 0
	case ComplementByteString:
		return 0
	case ReadBit:
		return 0
	case WriteBits:
		return 0
	case ReplicateByte:
		return 0
	case ShiftByteString:
		return 0
	case RotateByteString:
		return 0
	case CountSetBits:
		return 0
	case FindFirstSetBit:
		return 0
	case Ripemd_160:
		return 0

	default:
		panic("Forces")
	}
}

func (f DefaultFunction) Arity() int {
	switch f {
	// Integer functions
	case AddInteger:
		return 2
	case SubtractInteger:
		return 2
	case MultiplyInteger:
		return 2
	case DivideInteger:
		return 2
	case QuotientInteger:
		return 2
	case RemainderInteger:
		return 2
	case ModInteger:
		return 2
	case EqualsInteger:
		return 2
	case LessThanInteger:
		return 2
	case LessThanEqualsInteger:
		return 2
	// ByteString functions
	case AppendByteString:
		return 2
	case ConsByteString:
		return 2
	case SliceByteString:
		return 3
	case LengthOfByteString:
		return 1
	case IndexByteString:
		return 2
	case EqualsByteString:
		return 2
	case LessThanByteString:
		return 2
	case LessThanEqualsByteString:
		return 2
	// Cryptography and hash functions
	case Sha2_256:
		return 1
	case Sha3_256:
		return 1
	case Blake2b_256:
		return 1
	case Keccak_256:
		return 1
	case Blake2b_224:
		return 1
	case VerifyEd25519Signature:
		return 3
	case VerifyEcdsaSecp256k1Signature:
		return 3
	case VerifySchnorrSecp256k1Signature:
		return 3
	// String functions
	case AppendString:
		return 2
	case EqualsString:
		return 2
	case EncodeUtf8:
		return 1
	case DecodeUtf8:
		return 1
	// Bool function
	case IfThenElse:
		return 3
	// Unit function
	case ChooseUnit:
		return 2
	// Tracing function
	case Trace:
		return 2
	// Pairs functions
	case FstPair:
		return 1
	case SndPair:
		return 1
	// List functions
	case ChooseList:
		return 3
	case MkCons:
		return 2
	case HeadList:
		return 1
	case TailList:
		return 1
	case NullList:
		return 1
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	case ChooseData:
		return 6
	case ConstrData:
		return 2
	case MapData:
		return 1
	case ListData:
		return 1
	case IData:
		return 1
	case BData:
		return 1
	case UnConstrData:
		return 1
	case UnMapData:
		return 1
	case UnListData:
		return 1
	case UnIData:
		return 1
	case UnBData:
		return 1
	case EqualsData:
		return 2
	case SerialiseData:
		return 1
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	case MkPairData:
		return 2
	case MkNilData:
		return 1
	case MkNilPairData:
		return 1
	case Bls12_381_G1_Add:
		return 2
	case Bls12_381_G1_Neg:
		return 1
	case Bls12_381_G1_ScalarMul:
		return 2
	case Bls12_381_G1_Equal:
		return 2
	case Bls12_381_G1_Compress:
		return 1
	case Bls12_381_G1_Uncompress:
		return 1
	case Bls12_381_G1_HashToGroup:
		return 2
	case Bls12_381_G2_Add:
		return 2
	case Bls12_381_G2_Neg:
		return 1
	case Bls12_381_G2_ScalarMul:
		return 2
	case Bls12_381_G2_Equal:
		return 2
	case Bls12_381_G2_Compress:
		return 1
	case Bls12_381_G2_Uncompress:
		return 1
	case Bls12_381_G2_HashToGroup:
		return 2
	case Bls12_381_MillerLoop:
		return 2
	case Bls12_381_MulMlResult:
		return 2
	case Bls12_381_FinalVerify:
		return 2
	case IntegerToByteString:
		return 3
	case ByteStringToInteger:
		return 2
	case AndByteString:
		return 3
	case OrByteString:
		return 3
	case XorByteString:
		return 3
	case ComplementByteString:
		return 1
	case ReadBit:
		return 2
	case WriteBits:
		return 3
	case ReplicateByte:
		return 2
	case ShiftByteString:
		return 2
	case RotateByteString:
		return 2
	case CountSetBits:
		return 1
	case FindFirstSetBit:
		return 1
	case Ripemd_160:
		return 1

	default:
		panic("WTF")
	}
}

func (f DefaultFunction) String() string {
	switch f {
	// Integer functions
	case AddInteger:
		return "addInteger"
	case SubtractInteger:
		return "subtractInteger"
	case MultiplyInteger:
		return "multiplyInteger"
	case DivideInteger:
		return "divideInteger"
	case QuotientInteger:
		return "quotientInteger"
	case RemainderInteger:
		return "remainderInteger"
	case ModInteger:
		return "modInteger"
	case EqualsInteger:
		return "equalsInteger"
	case LessThanInteger:
		return "lessThanInteger"
	case LessThanEqualsInteger:
		return "lessThanEqualsInteger"
	// ByteString functions
	case AppendByteString:
		return "appendByteString"
	case ConsByteString:
		return "consByteString"
	case SliceByteString:
		return "sliceByteString"
	case LengthOfByteString:
		return "lengthOfByteString"
	case IndexByteString:
		return "indexByteString"
	case EqualsByteString:
		return "equalsByteString"
	case LessThanByteString:
		return "lessThanByteString"
	case LessThanEqualsByteString:
		return "lessThanEqualsByteString"
	// Cryptography and hash functions
	case Sha2_256:
		return "sha2_256"
	case Sha3_256:
		return "sha3_256"
	case Blake2b_256:
		return "blake2b_256"
	case Keccak_256:
		return "keccak_256"
	case Blake2b_224:
		return "blake2b_224"
	case VerifyEd25519Signature:
		return "verifyEd25519Signature"
	case VerifyEcdsaSecp256k1Signature:
		return "verifyEcdsaSecp256K1Signature"
	case VerifySchnorrSecp256k1Signature:
		return "verifySchnorrSecp256K1Signature"
	// String functions
	case AppendString:
		return "appendString"
	case EqualsString:
		return "equalsString"
	case EncodeUtf8:
		return "encodeUtf8"
	case DecodeUtf8:
		return "decodeUtf8"
	// Bool function
	case IfThenElse:
		return "ifThenElse"
	// Unit function
	case ChooseUnit:
		return "chooseUnit"
	// Tracing function
	case Trace:
		return "trace"
	// Pairs functions
	case FstPair:
		return "fstPair"
	case SndPair:
		return "sndPair"
	// List functions
	case ChooseList:
		return "chooseList"
	case MkCons:
		return "mkCons"
	case HeadList:
		return "headList"
	case TailList:
		return "tailList"
	case NullList:
		return "nullList"
	// Data functions
	// It is convenient to have a "choosing" function for a data type that has more than two
	// constructors to get pattern matching over it and we may end up having multiple such data
	// types hence we include the name of the data type as a suffix.
	case ChooseData:
		return "chooseData"
	case ConstrData:
		return "constrData"
	case MapData:
		return "mapData"
	case ListData:
		return "listData"
	case IData:
		return "iData"
	case BData:
		return "bData"
	case UnConstrData:
		return "unConstrData"
	case UnMapData:
		return "unMapData"
	case UnListData:
		return "unListData"
	case UnIData:
		return "unIData"
	case UnBData:
		return "unBData"
	case EqualsData:
		return "equalsData"
	case SerialiseData:
		return "serialiseData"
	// Misc constructors
	// Constructors that we need for constructing e.g. Data. Polymorphic builtin
	// constructors are often problematic (See note [Representable built-in
	// functions over polymorphic built-in types])
	case MkPairData:
		return "mkPairData"
	case MkNilData:
		return "mkNilData"
	case MkNilPairData:
		return "mkNilPairData"
	case Bls12_381_G1_Add:
		return "bls12_381_G1_add"
	case Bls12_381_G1_Neg:
		return "bls12_381_G1_neg"
	case Bls12_381_G1_ScalarMul:
		return "bls12_381_G1_scalarMul"
	case Bls12_381_G1_Equal:
		return "bls12_381_G1_equal"
	case Bls12_381_G1_Compress:
		return "bls12_381_G1_compress"
	case Bls12_381_G1_Uncompress:
		return "bls12_381_G1_uncompress"
	case Bls12_381_G1_HashToGroup:
		return "bls12_381_G1_hashToGroup"
	case Bls12_381_G2_Add:
		return "bls12_381_G2_add"
	case Bls12_381_G2_Neg:
		return "bls12_381_G2_neg"
	case Bls12_381_G2_ScalarMul:
		return "bls12_381_G2_scalarMul"
	case Bls12_381_G2_Equal:
		return "bls12_381_G2_equal"
	case Bls12_381_G2_Compress:
		return "bls12_381_G2_compress"
	case Bls12_381_G2_Uncompress:
		return "bls12_381_G2_uncompress"
	case Bls12_381_G2_HashToGroup:
		return "bls12_381_G2_hashToGroup"
	case Bls12_381_MillerLoop:
		return "bls12_381_millerLoop"
	case Bls12_381_MulMlResult:
		return "bls12_381_mulMlResult"
	case Bls12_381_FinalVerify:
		return "bls12_381_finalVerify"
	case IntegerToByteString:
		return "integerToByteString"
	case ByteStringToInteger:
		return "byteStringToInteger"
	case AndByteString:
		return "andByteString"
	case OrByteString:
		return "orByteString"
	case XorByteString:
		return "xorByteString"
	case ComplementByteString:
		return "complementByteString"
	case ReadBit:
		return "readBit"
	case WriteBits:
		return "writeBits"
	case ReplicateByte:
		return "replicateByte"
	case ShiftByteString:
		return "shiftByteString"
	case RotateByteString:
		return "rotateByteString"
	case CountSetBits:
		return "countSetBits"
	case FindFirstSetBit:
		return "findFirstSetBit"
	case Ripemd_160:
		return "ripemd_160"
	default:
		panic("unknown builtin")
	}
}

// Smallest DefaultFunction
const MinDefaultFunction byte = 0

// Smallest DefaultFunction
const MaxDefaultFunction byte = 86

func FromByte(tag byte) (DefaultFunction, error) {
	// only need to check if greater than because
	// the lowest possible value for byte is zero anyways
	if tag > MaxDefaultFunction {
		return 0, fmt.Errorf("DefaultFunctionNotFound(%d)", tag)
	}

	return DefaultFunction(tag), nil
}
