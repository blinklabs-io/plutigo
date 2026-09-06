package data

// Normalize returns a deep copy of pd with every Constr, Map, and List node's
// definite/indefinite-length CBOR array encoding reset to the package default
// (see MarshalCBOR on each type: indefinite for non-empty, definite for
// empty), discarding whatever encoding style the value originally carried
// from Decode.
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
func Normalize(pd PlutusData) PlutusData {
	switch v := pd.(type) {
	case *Constr:
		fields := make([]PlutusData, len(v.Fields))
		for i, f := range v.Fields {
			fields[i] = Normalize(f)
		}
		return &Constr{Tag: v.Tag, Fields: fields}
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
