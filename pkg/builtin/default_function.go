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
	"sha2256":                         Sha2_256,
	"sha3256":                         Sha3_256,
	"blake2B256":                      Blake2b_256,
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
