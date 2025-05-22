package syn

// Widths
const (
	TermTagWidth    byte = 4
	ConstTagWidth   byte = 4
	BuiltinTagWidth byte = 7
)

// Term Tags
const (
	VarTag      byte = 0
	DelayTag    byte = 1
	LambdaTag   byte = 2
	ApplyTag    byte = 3
	ConstantTag byte = 4
	ForceTag    byte = 5
	ErrorTag    byte = 6
	BuiltinTag  byte = 7
	ConstrTag   byte = 8
	CaseTag     byte = 9
)

// Constant Tags
const (
	IntegerTag        byte = 0
	ByteStringTag     byte = 1
	StringTag         byte = 2
	UnitTag           byte = 3
	BoolTag           byte = 4
	DataTag           byte = 8
	ProtoListOneTag   byte = 7
	ProtoListTwoTag   byte = 5
	ProtoPairOneTag   byte = 7
	ProtoPairTwoTag   byte = 7
	ProtoPairThreeTag byte = 6
)
