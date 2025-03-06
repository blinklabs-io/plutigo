package syn

import (
	"math/big"
)

type IConstant interface{}

type Integer struct {
	Inner *big.Int
}

type ByteString struct {
	Inner []uint8
}

type String struct {
	Inner string
}

type Unit struct{}

type Bool struct {
	Inner bool
}

type ProtoList struct {
}
