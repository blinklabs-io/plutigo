package syn

// Widths
const TermTagWidth byte = 4
const ConstTagWidth byte = 4
const BuiltinTagWidth byte = 7

// Term Tags
const VarTag byte = 0
const DelayTag byte = 1
const LambdaTag byte = 2
const ApplyTag byte = 3
const ConstantTag byte = 4
const ForceTag byte = 5
const ErrorTag byte = 6
const BuiltinTag byte = 7
const ConstrTag byte = 8
const CaseTag byte = 9

// Constant Tags
const IntegerTag byte = 0
const ByteStringTag byte = 1
const StringTag byte = 2
const UnitTag byte = 3
const BoolTag byte = 4
const DataTag byte = 8
const ProtoListOneTag byte = 7
const ProtoListTwoTag byte = 5
const ProtoPairOneTag byte = 7
const ProtoPairTwoTag byte = 7
const ProtoPairThreeTag byte = 6
