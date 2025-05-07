package builtin

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
		return 1
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
		return "sha2256"
	case Sha3_256:
		return "sha3256"
	case Blake2b_256:
		return "blake2B256"
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
	default:
		panic("unknown builtin")
	}
}
