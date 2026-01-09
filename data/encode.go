package data

import (
	"bytes"

	"github.com/fxamacker/cbor/v2"
)

// encMode is cached at package level to avoid recreation on every encode call
var encMode cbor.EncMode

func init() {
	opts := cbor.EncOptions{
		// NOTE: set any additional encoder options here
	}
	var err error
	encMode, err = opts.EncMode()
	if err != nil {
		panic("failed to initialize CBOR encoder: " + err.Error())
	}
}

// Encode encodes a PlutusData value into CBOR bytes.
func Encode(pd PlutusData) ([]byte, error) {
	return cborMarshal(pd)
}

// cborMarshal acts like cbor.Marshal but allows us to set our own encoder options
func cborMarshal(data any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := encMode.NewEncoder(buf)
	err := enc.Encode(data)
	return buf.Bytes(), err
}
