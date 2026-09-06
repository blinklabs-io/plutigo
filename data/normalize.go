package data

import "math/big"

// Normalize returns a deep copy of pd with every Constr, Map, and List node's
// definite/indefinite-length CBOR encoding reset to the package default,
// discarding whatever encoding style the value originally carried from
// Decode.
//
// The default is per type, and Map is not like the other two: Constr and
// List encode indefinite when non-empty and definite when empty, while Map
// always encodes definite, matching Haskell's canonical CBOR. See MarshalCBOR
// on each type. So normalizing a map decoded from indefinite-length bytes
// moves it to definite, the opposite direction from the others.
//
// Decode preserves the original wire's definite/indefinite-length choice on
// each node so that re-encoding a decoded value reproduces the original
// bytes. That fidelity is correct when verifying something against its own
// original bytes (e.g. a script-data hash), but cardano-ledger's reference
// implementation does not carry that fidelity through when it builds values
// for a Plutus script to observe (redeemer data, datums): it always
// constructs them fresh, which is equivalent to always using the package
// default encoding. A value decoded from on-chain bytes and then handed to a
// script unchanged can therefore serialise to different bytes than the
// reference implementation's for the same semantic value whenever the
// original encoder chose definite-length arrays, and a script that hashes or
// compares serialised bytes (via SerialiseData) sees that difference.
// Callers building script-visible values from decoded data (redeemer data,
// inline or witness datums) should call Normalize first.
//
// Every container node is rebuilt, so resetting the encoding never writes
// through to the input. Integer and ByteString leaves are returned as-is:
// they hold no encoding state, and this package treats their contents as
// read-only.
func Normalize(pd PlutusData) PlutusData {
	switch v := pd.(type) {
	case *Constr:
		fields := make([]PlutusData, len(v.Fields))
		for i, f := range v.Fields {
			fields[i] = Normalize(f)
		}
		// Copy the tag rather than aliasing it: big.Int is mutable, so
		// sharing the pointer would let a write through one value reach
		// the other. A nil tag stays nil; MarshalCBOR treats it as zero.
		tag := v.Tag
		if tag != nil {
			tag = new(big.Int).Set(tag)
		}
		return &Constr{Tag: tag, Fields: fields}
	case *Map:
		pairs := make([][2]PlutusData, len(v.Pairs))
		for i, p := range v.Pairs {
			pairs[i] = [2]PlutusData{Normalize(p[0]), Normalize(p[1])}
		}
		return &Map{Pairs: pairs}
	case *List:
		items := make([]PlutusData, len(v.Items))
		for i, item := range v.Items {
			items[i] = Normalize(item)
		}
		return &List{Items: items}
	default:
		// Integer and ByteString carry no encoding-style state.
		return pd
	}
}
