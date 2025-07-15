package scriptcontext

import (
	"math/big"

	"github.com/blinklabs-io/plutigo/pkg/data"
)

// ToPlutusData is an interface that represents types that support serialization to PlutusData when building a ScriptContext
type ToPlutusData interface {
	ToPlutusData() data.PlutusData
}

type Option[T ToPlutusData] struct {
	Value ToPlutusData
}

func (o Option[T]) ToPlutusData() data.PlutusData {
	// TODO: double-check this
	if o.Value == nil {
		return data.NewConstr(0, []data.PlutusData{})
	}
	return data.NewConstr(
		1,
		[]data.PlutusData{
			o.Value.ToPlutusData(),
		},
	)
	return nil
}

type KeyValuePairs[K ToPlutusData, V ToPlutusData] []KeyValuePair[K, V]

type KeyValuePair[K ToPlutusData, V ToPlutusData] struct {
	Key   K
	Value V
}

func toPlutusData(val any) data.PlutusData {
	if pd, ok := val.(ToPlutusData); ok {
		return pd.ToPlutusData()
	}
	switch v := val.(type) {
	case bool:
		if v {
			return data.NewConstr(1, []data.PlutusData{})
		}
		return data.NewConstr(0, []data.PlutusData{})
	default:
		// XXX: add explicit error return?
		panic("unknown type")
	}
}

type Coin int64

func (c Coin) ToPlutusData() data.PlutusData {
	return data.NewInteger(new(big.Int).SetInt64(int64(c)))
}

type PositiveCoin uint64

func (c PositiveCoin) ToPlutusData() data.PlutusData {
	return data.NewInteger(new(big.Int).SetUint64(uint64(c)))
}
