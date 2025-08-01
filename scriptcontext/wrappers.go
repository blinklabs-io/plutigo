package scriptcontext

import (
	"math/big"
	"reflect"

	"github.com/blinklabs-io/gouroboros/cbor"
	lcommon "github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
)

// ToPlutusData is an interface that represents types that support serialization to PlutusData when building a ScriptContext
type ToPlutusData interface {
	ToPlutusData() data.PlutusData
}

type Option[T ToPlutusData] struct {
	Value ToPlutusData
}

func (o Option[T]) ToPlutusData() data.PlutusData {
	if o.Value == nil {
		return data.NewConstr(0)
	}
	return data.NewConstr(
		1,
		o.Value.ToPlutusData(),
	)
}

type KeyValuePairs[K any, V any] []KeyValuePair[K, V]

func (k KeyValuePairs[K, V]) ToPlutusData() data.PlutusData {
	pairs := make([][2]data.PlutusData, len(k))
	for i, tmpPair := range k {
		pairs[i] = [2]data.PlutusData{
			toPlutusData(tmpPair.Key),
			toPlutusData(tmpPair.Value),
		}
	}
	return data.NewMap(pairs)
}

type KeyValuePair[K any, V any] struct {
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
			return data.NewConstr(1)
		}
		return data.NewConstr(0)
	case int64:
		return data.NewInteger(new(big.Int).SetInt64(v))
	case uint64:
		return data.NewInteger(new(big.Int).SetUint64(v))
	case []ToPlutusData:
		tmpItems := make([]data.PlutusData, len(v))
		for i, item := range v {
			tmpItems[i] = item.ToPlutusData()
		}
		return data.NewList(tmpItems...)
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice:
			tmpItems := make([]data.PlutusData, rv.Len())
			for i := range rv.Len() {
				item := rv.Index(i)
				tmpItems[i] = toPlutusData(item.Interface())
			}
			return data.NewList(tmpItems...)
		case reflect.Map:
			tmpPairs := make([][2]data.PlutusData, rv.Len())
			for i, k := range rv.MapKeys() {
				v := rv.MapIndex(k)
				tmpPairs[i] = [2]data.PlutusData{
					toPlutusData(k.Interface()),
					toPlutusData(v.Interface()),
				}
			}
			return data.NewMap(tmpPairs)
		}
	}
	return nil
}

type Coin int64

func (c Coin) ToPlutusData() data.PlutusData {
	return data.NewInteger(new(big.Int).SetInt64(int64(c)))
}

type PositiveCoin uint64

func (c PositiveCoin) ToPlutusData() data.PlutusData {
	return data.NewInteger(new(big.Int).SetUint64(uint64(c)))
}

type ResolvedInput lcommon.Utxo

func (r ResolvedInput) ToPlutusData() data.PlutusData {
	return data.NewConstr(
		0,
		r.Id.ToPlutusData(),
		r.Output.ToPlutusData(),
	)
}

type Redeemer struct {
	Tag     lcommon.RedeemerTag
	Index   uint32
	Data    data.PlutusData
	ExUnits lcommon.ExUnits
}

func (r Redeemer) ToPlutusData() data.PlutusData {
	return r.Data
}

func lazyValueToPlutusData(lv cbor.LazyValue) data.PlutusData {
	var pdWrap data.PlutusDataWrapper
	if err := pdWrap.UnmarshalCBOR(lv.Cbor()); err != nil {
		return nil
	}
	return pdWrap.Data
}
