package data

import (
	"bytes"

	"github.com/fxamacker/cbor/v2"
)

// Encode encodes a PlutusData value into CBOR bytes.
func Encode(pd PlutusData) ([]byte, error) {
	return cborMarshal(pd)
}

// cborMarshal acts like cbor.Marshal but allows us to set our own encoder options
func cborMarshal(data any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	opts := cbor.EncOptions{
		// NOTE: set any additional encoder options here
	}
	em, err := opts.EncMode()
	if err != nil {
		return nil, err
	}
	enc := em.NewEncoder(buf)
	err = enc.Encode(data)
	return buf.Bytes(), err
}
