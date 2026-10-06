package cek

import (
	"errors"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
)

func unValueDataInput(policies ...[2]data.PlutusData) *data.Map {
	return &data.Map{Pairs: policies}
}

func unValueDataPolicy(key []byte, tokens ...[2]data.PlutusData) [2]data.PlutusData {
	return [2]data.PlutusData{
		&data.ByteString{Inner: key},
		&data.Map{Pairs: tokens},
	}
}

func unValueDataToken(key []byte, quantity *big.Int) [2]data.PlutusData {
	return [2]data.PlutusData{&data.ByteString{Inner: key}, data.NewInteger(quantity)}
}

func TestUnValueDataCanonicalForm(t *testing.T) {
	t.Parallel()
	one := big.NewInt(1)
	limit := new(big.Int).Lsh(one, 127)
	long := make([]byte, 33)

	invalid := map[string]struct {
		input *data.Map
		code  ErrorCode
	}{
		"nil policy key":      {unValueDataInput([2]data.PlutusData{(*data.ByteString)(nil), &data.Map{Pairs: [][2]data.PlutusData{unValueDataToken(nil, one)}}}), ErrCodeInvalidArgument},
		"nil token map":       {unValueDataInput([2]data.PlutusData{data.NewByteString(nil), (*data.Map)(nil)}), ErrCodeInvalidArgument},
		"nil token key":       {unValueDataInput(unValueDataPolicy(nil, [2]data.PlutusData{(*data.ByteString)(nil), data.NewInteger(one)})), ErrCodeInvalidArgument},
		"nil quantity":        {unValueDataInput(unValueDataPolicy(nil, [2]data.PlutusData{data.NewByteString(nil), (*data.Integer)(nil)})), ErrCodeInvalidArgument},
		"policy key too long": {unValueDataInput(unValueDataPolicy(long, unValueDataToken([]byte{1}, one))), ErrCodeInvalidArgument},
		"token key too long":  {unValueDataInput(unValueDataPolicy([]byte{1}, unValueDataToken(long, one))), ErrCodeInvalidArgument},
		"policies out of order": {unValueDataInput(
			unValueDataPolicy([]byte{2}, unValueDataToken([]byte{1}, one)),
			unValueDataPolicy([]byte{1}, unValueDataToken([]byte{1}, one)),
		), ErrCodeInvalidArgument},
		"duplicate policy": {unValueDataInput(
			unValueDataPolicy([]byte{1}, unValueDataToken([]byte{1}, one)),
			unValueDataPolicy([]byte{1}, unValueDataToken([]byte{2}, one)),
		), ErrCodeInvalidArgument},
		"duplicate empty policy": {unValueDataInput(
			unValueDataPolicy(nil, unValueDataToken([]byte{1}, one)),
			unValueDataPolicy([]byte{}, unValueDataToken([]byte{1}, one)),
		), ErrCodeInvalidArgument},
		"tokens out of order": {unValueDataInput(unValueDataPolicy([]byte{1},
			unValueDataToken([]byte{2}, one), unValueDataToken([]byte{1}, one),
		)), ErrCodeInvalidArgument},
		"duplicate empty token": {unValueDataInput(unValueDataPolicy([]byte{1},
			unValueDataToken(nil, one), unValueDataToken([]byte{}, one),
		)), ErrCodeInvalidArgument},
		"empty token map":      {unValueDataInput(unValueDataPolicy([]byte{1})), ErrCodeInvalidArgument},
		"zero quantity":        {unValueDataInput(unValueDataPolicy([]byte{1}, unValueDataToken([]byte{1}, new(big.Int)))), ErrCodeInvalidArgument},
		"quantity above range": {unValueDataInput(unValueDataPolicy([]byte{1}, unValueDataToken([]byte{1}, limit))), ErrCodeOverflow},
		"quantity below range": {unValueDataInput(unValueDataPolicy([]byte{1}, unValueDataToken([]byte{1}, new(big.Int).Sub(new(big.Int).Neg(limit), one)))), ErrCodeOverflow},
	}
	for name, tc := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			t.Parallel()
			fn := newTestBuiltin(builtin.UnValueData).ApplyArg(
				&Constant{&syn.Data{Inner: tc.input}},
			)
			_, err := evalBuiltinWithError(t, newTestMachineV4(), fn)
			if err == nil {
				t.Fatal("unValueData accepted a non-canonical Value")
			}
			var builtinErr *BuiltinError
			if !errors.As(err, &builtinErr) {
				t.Fatalf("error = %T %v, want *BuiltinError", err, err)
			}
			if builtinErr.Code != tc.code {
				t.Fatalf("error code = %v, want %v", builtinErr.Code, tc.code)
			}
		})
	}

	t.Run("accepts quantity bounds", func(t *testing.T) {
		t.Parallel()
		input := unValueDataInput(unValueDataPolicy([]byte{1},
			unValueDataToken([]byte{1}, new(big.Int).Neg(limit)),
			unValueDataToken([]byte{2}, new(big.Int).Sub(limit, one)),
		))
		fn := newTestBuiltin(builtin.UnValueData).ApplyArg(
			&Constant{&syn.Data{Inner: input}},
		)
		if _, err := evalBuiltinWithError(t, newTestMachineV4(), fn); err != nil {
			t.Fatalf("unValueData rejected canonical bounds: %v", err)
		}
	})
}
