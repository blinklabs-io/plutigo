package syn

import (
	"math/big"
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

func valueDataProgram(t *testing.T, valueText string) string {
	t.Helper()
	return `(program 1.3.0 (con data (V ` + valueText + `)))`
}

func TestParsePlutusValueCanonicalForm(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("01", 33)
	qtyLimit := new(big.Int).Lsh(big.NewInt(1), 127)
	maxQty := new(big.Int).Sub(qtyLimit, big.NewInt(1)).String()
	minQty := new(big.Int).Neg(qtyLimit).String()
	pastMax := qtyLimit.String()
	pastMin := new(big.Int).Sub(new(big.Int).Neg(qtyLimit), big.NewInt(1)).String()

	valid := map[string]string{
		"max quantity": `[(#aa, [(#bb, ` + maxQty + `)])]`,
		"min quantity": `[(#aa, [(#bb, ` + minQty + `)])]`,
		"ascending":    `[(#aa, [(#01, 1), (#02, -1)]), (#bb, [(#01, 1)])]`,
	}
	for name, text := range valid {
		t.Run("accepts "+name, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(valueDataProgram(t, text)); err != nil {
				t.Fatalf("Parse: %v", err)
			}
		})
	}

	invalid := map[string]string{
		"policy key too long":   `[(#` + long + `, [(#bb, 1)])]`,
		"token key too long":    `[(#aa, [(#` + long + `, 1)])]`,
		"policies out of order": `[(#bb, [(#01, 1)]), (#aa, [(#01, 1)])]`,
		"duplicate policy":      `[(#aa, [(#01, 1)]), (#aa, [(#02, 1)])]`,
		"tokens out of order":   `[(#aa, [(#02, 1), (#01, 1)])]`,
		"duplicate token":       `[(#aa, [(#01, 1), (#01, 2)])]`,
		"empty token map":       `[(#aa, [])]`,
		"zero quantity":         `[(#aa, [(#bb, 0)])]`,
		"quantity above range":  `[(#aa, [(#bb, ` + pastMax + `)])]`,
		"quantity below range":  `[(#aa, [(#bb, ` + pastMin + `)])]`,
	}
	for name, text := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(valueDataProgram(t, text)); err == nil {
				t.Fatal("Parse accepted a non-canonical Value")
			}
		})
	}
}

func TestEncodeRejectsNonCanonicalDataValue(t *testing.T) {
	t.Parallel()
	tokens := &data.Map{Pairs: [][2]data.PlutusData{{
		data.NewByteString([]byte{1}),
		data.NewInteger(new(big.Int)),
	}}}
	inner := &data.Map{Pairs: [][2]data.PlutusData{{
		data.NewByteString([]byte{1}),
		tokens,
	}}}
	_, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV4,
		Term: &Constant{Con: &Data{
			Inner: &data.Value{Inner: inner},
		}},
	})
	if err == nil {
		t.Fatal("Encode accepted a data Value with a zero quantity")
	}
}
