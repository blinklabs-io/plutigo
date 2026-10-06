package data_test

import (
	"fmt"
	"math/big"

	"github.com/blinklabs-io/plutigo/data"
)

// PlutusData converts to and from the Cardano detailed JSON schema, and to
// and from CBOR.
func ExampleEncodeJSON() {
	datum := data.NewConstr(0, data.NewInteger(big.NewInt(42)))

	encoded, err := data.EncodeJSON(datum)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(string(encoded))

	decoded, err := data.DecodeJSON(encoded)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	cbor, err := data.Encode(decoded)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%x\n", cbor)
	// Output:
	// {"constructor":0,"fields":[{"int":42}]}
	// d8799f182aff
}
