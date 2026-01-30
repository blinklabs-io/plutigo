package builtin

import (
	"testing"
)

func TestBuiltinAvailability(t *testing.T) {
	tests := []struct {
		name      string
		fn        DefaultFunction
		version   PlutusVersion
		available bool
	}{
		// V1 builtins should be available in all versions
		{"addInteger in V1", AddInteger, PlutusV1, true},
		{"addInteger in V2", AddInteger, PlutusV2, true},
		{"addInteger in V3", AddInteger, PlutusV3, true},
		{"addInteger in V4", AddInteger, PlutusV4, true},

		// V2 builtins
		{"serialiseData in V1", SerialiseData, PlutusV1, false},
		{"serialiseData in V2", SerialiseData, PlutusV2, true},
		{"serialiseData in V3", SerialiseData, PlutusV3, true},
		{"verifyEcdsaSecp256k1Signature in V1", VerifyEcdsaSecp256k1Signature, PlutusV1, false},
		{"verifyEcdsaSecp256k1Signature in V2", VerifyEcdsaSecp256k1Signature, PlutusV2, true},
		{"verifySchnorrSecp256k1Signature in V1", VerifySchnorrSecp256k1Signature, PlutusV1, false},
		{"verifySchnorrSecp256k1Signature in V2", VerifySchnorrSecp256k1Signature, PlutusV2, true},

		// V3 builtins
		{"bls12_381_G1_add in V2", Bls12_381_G1_Add, PlutusV2, false},
		{"bls12_381_G1_add in V3", Bls12_381_G1_Add, PlutusV3, true},
		{"integerToByteString in V2", IntegerToByteString, PlutusV2, false},
		{"integerToByteString in V3", IntegerToByteString, PlutusV3, true},
		{"keccak_256 in V2", Keccak_256, PlutusV2, false},
		{"keccak_256 in V3", Keccak_256, PlutusV3, true},
		{"andByteString in V2", AndByteString, PlutusV2, false},
		{"andByteString in V3", AndByteString, PlutusV3, true},

		// V4 builtins
		{"lengthOfArray in V3", LengthOfArray, PlutusV3, false},
		{"lengthOfArray in V4", LengthOfArray, PlutusV4, true},
		{"insertCoin in V3", InsertCoin, PlutusV3, false},
		{"insertCoin in V4", InsertCoin, PlutusV4, true},
		{"valueContains in V3", ValueContains, PlutusV3, false},
		{"valueContains in V4", ValueContains, PlutusV4, true},
		{"bls12_381_G1_multiScalarMul in V3", Bls12_381_G1_MultiScalarMul, PlutusV3, false},
		{"bls12_381_G1_multiScalarMul in V4", Bls12_381_G1_MultiScalarMul, PlutusV4, true},

		// Unreleased builtins
		{"dropList in V4", DropList, PlutusV4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn.IsAvailableIn(tt.version)
			if got != tt.available {
				t.Errorf("IsAvailableIn() = %v, want %v", got, tt.available)
			}
		})
	}
}

func TestLanguageVersionToPlutusVersion(t *testing.T) {
	tests := []struct {
		name    string
		version [3]uint32
		want    PlutusVersion
	}{
		{"V1", [3]uint32{1, 0, 0}, PlutusV1},
		{"V2", [3]uint32{1, 1, 0}, PlutusV2},
		{"V3", [3]uint32{1, 2, 0}, PlutusV3},
		{"V4", [3]uint32{1, 3, 0}, PlutusV4},
		{"Future V4+", [3]uint32{1, 4, 0}, PlutusV4},
		{"Unknown defaults to V1", [3]uint32{2, 0, 0}, PlutusV1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LanguageVersionToPlutusVersion(tt.version)
			if got != tt.want {
				t.Errorf("LanguageVersionToPlutusVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntroducedIn(t *testing.T) {
	// Verify that key builtins return the correct version
	if AddInteger.IntroducedIn() != PlutusV1 {
		t.Errorf("AddInteger.IntroducedIn() = %v, want V1", AddInteger.IntroducedIn())
	}
	if SerialiseData.IntroducedIn() != PlutusV2 {
		t.Errorf("SerialiseData.IntroducedIn() = %v, want V2", SerialiseData.IntroducedIn())
	}
	if Bls12_381_G1_Add.IntroducedIn() != PlutusV3 {
		t.Errorf("Bls12_381_G1_Add.IntroducedIn() = %v, want V3", Bls12_381_G1_Add.IntroducedIn())
	}
	if LengthOfArray.IntroducedIn() != PlutusV4 {
		t.Errorf("LengthOfArray.IntroducedIn() = %v, want V4", LengthOfArray.IntroducedIn())
	}
	if DropList.IntroducedIn() != PlutusVUnreleased {
		t.Errorf("DropList.IntroducedIn() = %v, want unreleased", DropList.IntroducedIn())
	}
}
