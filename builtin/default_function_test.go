package builtin

import (
	"testing"
)

func TestFromByte(t *testing.T) {
	tests := []struct {
		input    byte
		expected DefaultFunction
		hasError bool
	}{
		{0, AddInteger, false},
		{1, SubtractInteger, false},
		{MaxDefaultFunction, DefaultFunction(MaxDefaultFunction), false},
		{MaxDefaultFunction + 1, 0, true},
		{255, 0, true},
	}

	for _, test := range tests {
		result, err := FromByte(test.input)
		if test.hasError {
			if err == nil {
				t.Errorf(
					"Expected error for input %d, but got none",
					test.input,
				)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %d: %v", test.input, err)
			}
			if result != test.expected {
				t.Errorf("For input %d, expected %v, got %v", test.input, test.expected, result)
			}
		}
	}
}

func TestDefaultFunctionString(t *testing.T) {
	tests := []struct {
		df       DefaultFunction
		expected string
	}{
		{AddInteger, "addInteger"},
		{SubtractInteger, "subtractInteger"},
		{Sha2_256, "sha2_256"},
		{IfThenElse, "ifThenElse"},
		{IndexArray, "indexArray"},
	}

	for _, test := range tests {
		result := test.df.String()
		if result != test.expected {
			t.Errorf(
				"For %v, expected %s, got %s",
				test.df,
				test.expected,
				result,
			)
		}
	}
}

func TestDefaultFunctionForceCount(t *testing.T) {
	tests := []struct {
		df       DefaultFunction
		expected uint
	}{
		{AddInteger, 0},
		{IfThenElse, 1},
		{ChooseUnit, 1},
		{Trace, 1},
		{LengthOfArray, 1},
		{FstPair, 2},
		{SndPair, 2},
	}

	for _, test := range tests {
		result := test.df.ForceCount()
		if result != test.expected {
			t.Errorf(
				"For %v, expected %d, got %d",
				test.df,
				test.expected,
				result,
			)
		}
	}
}

func TestDefaultFunctionArity(t *testing.T) {
	tests := []struct {
		df       DefaultFunction
		expected uint
	}{
		{AddInteger, 2},
		{Sha2_256, 1},
		{IfThenElse, 3},
		{ChooseUnit, 2},
		{ScaleValue, 2},
		{FstPair, 1},
		{VerifyEd25519Signature, 3},
	}

	for _, test := range tests {
		result := test.df.Arity()
		if result != test.expected {
			t.Errorf(
				"For %v, expected %d, got %d",
				test.df,
				test.expected,
				result,
			)
		}
	}
}

func TestDefaultFunctionV4TagsMatchPlutusSpec(t *testing.T) {
	tests := []struct {
		fn  DefaultFunction
		tag byte
	}{
		{DropList, 88},
		{LengthOfArray, 89},
		{ListToArray, 90},
		{IndexArray, 91},
		{Bls12_381_G1_MultiScalarMul, 92},
		{Bls12_381_G2_MultiScalarMul, 93},
		{InsertCoin, 94},
		{LookupCoin, 95},
		{UnionValue, 96},
		{ValueContains, 97},
		{ValueData, 98},
		{UnValueData, 99},
		{ScaleValue, 100},
	}

	for _, tt := range tests {
		t.Run(tt.fn.String(), func(t *testing.T) {
			if byte(tt.fn) != tt.tag {
				t.Fatalf("%s tag = %d, want %d", tt.fn, tt.fn, tt.tag)
			}
			got, err := FromByte(tt.tag)
			if err != nil {
				t.Fatalf("FromByte(%d): %v", tt.tag, err)
			}
			if got != tt.fn {
				t.Fatalf("FromByte(%d) = %s, want %s", tt.tag, got, tt.fn)
			}
		})
	}
}

func TestBuiltinsMap(t *testing.T) {
	// Test a few key mappings
	expectedMappings := map[string]DefaultFunction{
		"addInteger":      AddInteger,
		"subtractInteger": SubtractInteger,
		"sha2_256":        Sha2_256,
		"ifThenElse":      IfThenElse,
		"indexArray":      IndexArray,
	}

	for name, expected := range expectedMappings {
		actual, exists := Builtins[name]
		if !exists {
			t.Errorf("Builtin %s not found in map", name)
		} else if actual != expected {
			t.Errorf("For %s, expected %v, got %v", name, expected, actual)
		}
	}
}

func TestConstants(t *testing.T) {
	if MinDefaultFunction != 0 {
		t.Errorf("MinDefaultFunction should be 0, got %d", MinDefaultFunction)
	}
	expectedTotal := int(MaxDefaultFunction) - int(MinDefaultFunction) + 1
	if int(TotalBuiltinCount) != expectedTotal {
		t.Errorf(
			"TotalBuiltinCount should be %d (MaxDefaultFunction - MinDefaultFunction + 1), got %d",
			expectedTotal,
			TotalBuiltinCount,
		)
	}
}

func FuzzFromByte(f *testing.F) {
	validBytes := []byte{0, 1, 10, 20, 50, 87, 88, 89, 90, MaxDefaultFunction - 1, MaxDefaultFunction}
	invalidBytes := []byte{MaxDefaultFunction + 1, 255}
	for _, b := range validBytes {
		f.Add(b)
	}
	for _, b := range invalidBytes {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b byte) {
		df, err := FromByte(b)
		if b <= MaxDefaultFunction {
			if err != nil {
				t.Errorf(
					"FromByte(%d) should not error for valid byte, got %v",
					b,
					err,
				)
			}
			if df != DefaultFunction(b) {
				t.Errorf(
					"FromByte(%d) should return DefaultFunction(%d), got %v",
					b,
					b,
					df,
				)
			}
			name := df.String()
			if name != "" {
				if got, ok := Builtins[name]; !ok || got != df {
					t.Errorf("builtin reverse lookup for %q = %v, %v; want %v, true", name, got, ok, df)
				}
			}
		} else {
			if err == nil {
				t.Errorf("FromByte(%d) should error for invalid byte > MaxDefaultFunction (%d)", b, MaxDefaultFunction)
			}
		}
	})
}
