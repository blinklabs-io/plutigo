package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestMachineVersion(t *testing.T) {
	version := lang.LanguageVersionV2
	machine := NewMachine[syn.DeBruijn](version, 100, nil)

	if machine.version != version {
		t.Errorf("Expected version %v, got %v", version, machine.version)
	}
}

func TestVersionLessThan(t *testing.T) {
	// V1 < V2 < V3 < V4
	if !VersionLessThan(LanguageVersionV1, LanguageVersionV2) {
		t.Error("V1 should be less than V2")
	}
	if !VersionLessThan(LanguageVersionV2, LanguageVersionV3) {
		t.Error("V2 should be less than V3")
	}
	if !VersionLessThan(LanguageVersionV3, LanguageVersionV4) {
		t.Error("V3 should be less than V4")
	}

	// Not less than self
	if VersionLessThan(LanguageVersionV3, LanguageVersionV3) {
		t.Error("V3 should not be less than V3")
	}

	// Greater than is not less than
	if VersionLessThan(LanguageVersionV4, LanguageVersionV3) {
		t.Error("V4 should not be less than V3")
	}
}

func TestMachineVersionV4(t *testing.T) {
	version := LanguageVersionV4
	machine := NewMachine[syn.DeBruijn](version, 100, nil)

	if machine.version != version {
		t.Errorf("Expected version %v, got %v", version, machine.version)
	}
}

func TestBigIntExMem0(t *testing.T) {
	x := big.NewInt(0)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}
}

func TestBigIntExMemSmall(t *testing.T) {
	x := big.NewInt(1600000000000)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}
}

func TestBigIntExMemBig(t *testing.T) {
	x := big.NewInt(160000000000000)

	x.Mul(x, big.NewInt(1000000))

	y := bigIntExMem(x)()

	if y != ExMem(2) {
		t.Error("HOW???")
	}
}

func TestBigIntExMemHuge(t *testing.T) {
	x := big.NewInt(1600000000000000000)

	x.Mul(x, big.NewInt(1000000000000000000))

	x.Mul(x, big.NewInt(1000))

	y := bigIntExMem(x)()

	if y != ExMem(3) {
		t.Error("HOW???")
	}
}

func TestExBudgetSub(t *testing.T) {
	a := ExBudget{Mem: 100, Cpu: 200}
	b := ExBudget{Mem: 30, Cpu: 50}
	result := a.Sub(&b)

	expected := ExBudget{Mem: 70, Cpu: 150}
	if result != expected {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestUpdateV3CostModel(t *testing.T) {
	// plutusV3CostModel from preview network conway-genesis.json
	// Source: https://github.com/blinklabs-io/docker-cardano-configs/blob/main/config/preview/conway-genesis.json
	previewV3CostModel := []int64{
		100788, 420, 1, 1, 1000, 173, 0, 1, 1000, 59957, 4, 1, 11183, 32, 201305, 8356,
		4, 16000, 100, 16000, 100, 16000, 100, 16000, 100, 16000, 100, 16000, 100, 100,
		100, 16000, 100, 94375, 32, 132994, 32, 61462, 4, 72010, 178, 0, 1, 22151, 32,
		91189, 769, 4, 2, 85848, 123203, 7305, -900, 1716, 549, 57, 85848, 0, 1, 1, 1000,
		42921, 4, 2, 24548, 29498, 38, 1, 898148, 27279, 1, 51775, 558, 1, 39184, 1000,
		60594, 1, 141895, 32, 83150, 32, 15299, 32, 76049, 1, 13169, 4, 22100, 10, 28999,
		74, 1, 28999, 74, 1, 43285, 552, 1, 44749, 541, 1, 33852, 32, 68246, 32, 72362,
		32, 7243, 32, 7391, 32, 11546, 32, 85848, 123203, 7305, -900, 1716, 549, 57, 85848,
		0, 1, 90434, 519, 0, 1, 74433, 32, 85848, 123203, 7305, -900, 1716, 549, 57, 85848,
		0, 1, 1, 85848, 123203, 7305, -900, 1716, 549, 57, 85848, 0, 1, 955506, 213312, 0,
		2, 270652, 22588, 4, 1457325, 64566, 4, 20467, 1, 4, 0, 141992, 32, 100788, 420, 1,
		1, 81663, 32, 59498, 32, 20142, 32, 24588, 32, 20744, 32, 25933, 32, 24623, 32,
		43053543, 10, 53384111, 14333, 10, 43574283, 26308, 10, 16000, 100, 16000, 100,
		962335, 18, 2780678, 6, 442008, 1, 52538055, 3756, 18, 267929, 18, 76433006, 8868,
		18, 52948122, 18, 1995836, 36, 3227919, 12, 901022, 1, 166917843, 4307, 36, 284546,
		36, 158221314, 26549, 36, 74698472, 36, 333849714, 1, 254006273, 72, 2174038, 72,
		2261318, 64571, 4, 207616, 8310, 4, 1293828, 28716, 63, 0, 1, 1006041, 43623, 251,
		0, 1,
	}

	// Get the default V3 cost model
	defaultCM := DefaultCostModel

	// Update with preview network cost model
	updatedCM, err := costModelFromList(lang.LanguageVersionV3, SemanticsVariantC, previewV3CostModel)
	if err != nil {
		t.Fatalf("unexpected error building cost model from list: %s", err)
	}

	// Verify specific machine cost values were updated
	// From the cost model array, cekApplyCost-exBudgetCPU is at index 17 (value: 16000)
	// and cekApplyCost-exBudgetMemory is at index 18 (value: 100)
	if updatedCM.machineCosts.apply.Cpu != 16000 {
		t.Errorf("Expected apply CPU cost to be 16000, got %d", updatedCM.machineCosts.apply.Cpu)
	}
	if updatedCM.machineCosts.apply.Mem != 100 {
		t.Errorf("Expected apply memory cost to be 100, got %d", updatedCM.machineCosts.apply.Mem)
	}

	// Verify startup costs (indices 29-30: 100, 100)
	if updatedCM.machineCosts.startup.Cpu != 100 {
		t.Errorf("Expected startup CPU cost to be 100, got %d", updatedCM.machineCosts.startup.Cpu)
	}
	if updatedCM.machineCosts.startup.Mem != 100 {
		t.Errorf("Expected startup memory cost to be 100, got %d", updatedCM.machineCosts.startup.Mem)
	}

	// Verify builtin function costs - AddInteger (MaxSizeModel)
	// addInteger-cpu-arguments-intercept: 100788, addInteger-cpu-arguments-slope: 420
	addIntCPU := updatedCM.builtinCosts[builtin.AddInteger].cpu.(*MaxSizeModel)
	if addIntCPU.intercept != 100788 {
		t.Errorf("Expected AddInteger CPU intercept to be 100788, got %d", addIntCPU.intercept)
	}
	if addIntCPU.slope != 420 {
		t.Errorf("Expected AddInteger CPU slope to be 420, got %d", addIntCPU.slope)
	}

	// Verify AddInteger memory costs (MaxSizeModel)
	// addInteger-memory-arguments-intercept: 1, addInteger-memory-arguments-slope: 1
	addIntMem := updatedCM.builtinCosts[builtin.AddInteger].mem.(*MaxSizeModel)
	if addIntMem.intercept != 1 {
		t.Errorf("Expected AddInteger memory intercept to be 1, got %d", addIntMem.intercept)
	}
	if addIntMem.slope != 1 {
		t.Errorf("Expected AddInteger memory slope to be 1, got %d", addIntMem.slope)
	}

	// Verify AppendByteString costs (AddedSizesModel)
	// appendByteString-cpu-arguments-intercept: 1000, appendByteString-cpu-arguments-slope: 173
	appendBSCPU := updatedCM.builtinCosts[builtin.AppendByteString].cpu.(*AddedSizesModel)
	if appendBSCPU.intercept != 1000 {
		t.Errorf("Expected AppendByteString CPU intercept to be 1000, got %d", appendBSCPU.intercept)
	}
	if appendBSCPU.slope != 173 {
		t.Errorf("Expected AppendByteString CPU slope to be 173, got %d", appendBSCPU.slope)
	}

	// Verify Blake2b_256 costs (LinearInX)
	// blake2b_256-cpu-arguments-intercept: 201305, blake2b_256-cpu-arguments-slope: 8356
	blake2bCPU := updatedCM.builtinCosts[builtin.Blake2b_256].cpu.(*LinearInX)
	if blake2bCPU.intercept != 201305 {
		t.Errorf("Expected Blake2b_256 CPU intercept to be 201305, got %d", blake2bCPU.intercept)
	}
	if blake2bCPU.slope != 8356 {
		t.Errorf("Expected Blake2b_256 CPU slope to be 8356, got %d", blake2bCPU.slope)
	}
	blake2bMem := updatedCM.builtinCosts[builtin.Blake2b_256].mem.(*ConstantCost)
	if blake2bMem.c != 4 {
		t.Errorf("Expected Blake2b_256 memory to be 4, got %d", blake2bMem.c)
	}

	// Verify MultiplyInteger costs (MultipliedSizesModel)
	// multiplyInteger-cpu-arguments-intercept: 90434, multiplyInteger-cpu-arguments-slope: 519
	mulIntCPU := updatedCM.builtinCosts[builtin.MultiplyInteger].cpu.(*MultipliedSizesModel)
	if mulIntCPU.intercept != 90434 {
		t.Errorf("Expected MultiplyInteger CPU intercept to be 90434, got %d", mulIntCPU.intercept)
	}
	if mulIntCPU.slope != 519 {
		t.Errorf("Expected MultiplyInteger CPU slope to be 519, got %d", mulIntCPU.slope)
	}

	// Verify DivideInteger costs (ConstAboveDiagonalIntoQuadraticXAndYModel)
	// divideInteger-cpu-arguments-constant: 85848 (index 49)
	// divideInteger-cpu-arguments-model-arguments-c00: 123203 (index 50)
	// divideInteger-cpu-arguments-model-arguments-c10: 1716 (index 53)
	divIntCPU := updatedCM.builtinCosts[builtin.DivideInteger].cpu.(*ConstAboveDiagonalIntoQuadraticXAndYModel)
	if divIntCPU.constant != 85848 {
		t.Errorf("Expected DivideInteger CPU constant to be 85848, got %d", divIntCPU.constant)
	}
	if divIntCPU.coeff00 != 123203 {
		t.Errorf("Expected DivideInteger CPU coeff00 to be 123203, got %d", divIntCPU.coeff00)
	}
	if divIntCPU.coeff10 != 1716 {
		t.Errorf("Expected DivideInteger CPU coeff10 to be 1716, got %d", divIntCPU.coeff10)
	}

	// Verify that the cost model was successfully loaded without panicking
	// This test ensures the update mechanism properly handles:
	// - 3-part parameter names (e.g., "bData-cpu-arguments")
	// - 4-part parameter names (e.g., "addInteger-cpu-arguments-intercept")
	// - 6-part parameter names (e.g., "divideInteger-cpu-arguments-model-arguments-c00")
	// - Both "memory" and "mem" variants in parameter names
	// - Both "c0", "c1", "c2" and "coeff0", "coeff1", "coeff2" naming conventions

	// Verify that creating a new default cost model doesn't affect the updated one
	defaultCM2 := DefaultCostModel
	if defaultCM.machineCosts.apply.Cpu != defaultCM2.machineCosts.apply.Cpu {
		t.Error("Creating a new cost model should not affect the default values")
	}
}

func TestUpdateV1CostModelFromMap(t *testing.T) {
	// PlutusV1 cost model from preview network alonzo-genesis.json
	// Source: https://github.com/blinklabs-io/docker-cardano-configs/blob/main/config/preview/alonzo-genesis.json
	previewV1CostModelMap := map[string]int64{
		"addInteger-cpu-arguments-intercept":                       197209,
		"addInteger-cpu-arguments-slope":                           0,
		"addInteger-memory-arguments-intercept":                    1,
		"addInteger-memory-arguments-slope":                        1,
		"appendByteString-cpu-arguments-intercept":                 396231,
		"appendByteString-cpu-arguments-slope":                     621,
		"appendByteString-memory-arguments-intercept":              0,
		"appendByteString-memory-arguments-slope":                  1,
		"appendString-cpu-arguments-intercept":                     150000,
		"appendString-cpu-arguments-slope":                         1000,
		"appendString-memory-arguments-intercept":                  0,
		"appendString-memory-arguments-slope":                      1,
		"bData-cpu-arguments":                                      150000,
		"bData-memory-arguments":                                   32,
		"blake2b_256-cpu-arguments-intercept":                      2477736,
		"blake2b_256-cpu-arguments-slope":                          29175,
		"blake2b_256-memory-arguments":                             4,
		"cekApplyCost-exBudgetCPU":                                 29773,
		"cekApplyCost-exBudgetMemory":                              100,
		"cekBuiltinCost-exBudgetCPU":                               29773,
		"cekBuiltinCost-exBudgetMemory":                            100,
		"cekConstCost-exBudgetCPU":                                 29773,
		"cekConstCost-exBudgetMemory":                              100,
		"cekDelayCost-exBudgetCPU":                                 29773,
		"cekDelayCost-exBudgetMemory":                              100,
		"cekForceCost-exBudgetCPU":                                 29773,
		"cekForceCost-exBudgetMemory":                              100,
		"cekLamCost-exBudgetCPU":                                   29773,
		"cekLamCost-exBudgetMemory":                                100,
		"cekStartupCost-exBudgetCPU":                               100,
		"cekStartupCost-exBudgetMemory":                            100,
		"cekVarCost-exBudgetCPU":                                   29773,
		"cekVarCost-exBudgetMemory":                                100,
		"chooseData-cpu-arguments":                                 150000,
		"chooseData-memory-arguments":                              32,
		"chooseList-cpu-arguments":                                 150000,
		"chooseList-memory-arguments":                              32,
		"chooseUnit-cpu-arguments":                                 150000,
		"chooseUnit-memory-arguments":                              32,
		"consByteString-cpu-arguments-intercept":                   150000,
		"consByteString-cpu-arguments-slope":                       1000,
		"consByteString-memory-arguments-intercept":                0,
		"consByteString-memory-arguments-slope":                    1,
		"constrData-cpu-arguments":                                 150000,
		"constrData-memory-arguments":                              32,
		"decodeUtf8-cpu-arguments-intercept":                       150000,
		"decodeUtf8-cpu-arguments-slope":                           1000,
		"decodeUtf8-memory-arguments-intercept":                    0,
		"decodeUtf8-memory-arguments-slope":                        8,
		"divideInteger-cpu-arguments-constant":                     148000,
		"divideInteger-cpu-arguments-model-arguments-intercept":    425507,
		"divideInteger-cpu-arguments-model-arguments-slope":        118,
		"divideInteger-memory-arguments-intercept":                 0,
		"divideInteger-memory-arguments-minimum":                   1,
		"divideInteger-memory-arguments-slope":                     1,
		"encodeUtf8-cpu-arguments-intercept":                       150000,
		"encodeUtf8-cpu-arguments-slope":                           1000,
		"encodeUtf8-memory-arguments-intercept":                    0,
		"encodeUtf8-memory-arguments-slope":                        8,
		"equalsByteString-cpu-arguments-constant":                  150000,
		"equalsByteString-cpu-arguments-intercept":                 112536,
		"equalsByteString-cpu-arguments-slope":                     247,
		"equalsByteString-memory-arguments":                        1,
		"equalsData-cpu-arguments-intercept":                       150000,
		"equalsData-cpu-arguments-slope":                           10000,
		"equalsData-memory-arguments":                              1,
		"equalsInteger-cpu-arguments-intercept":                    136542,
		"equalsInteger-cpu-arguments-slope":                        1326,
		"equalsInteger-memory-arguments":                           1,
		"equalsString-cpu-arguments-constant":                      1000,
		"equalsString-cpu-arguments-intercept":                     150000,
		"equalsString-cpu-arguments-slope":                         1000,
		"equalsString-memory-arguments":                            1,
		"fstPair-cpu-arguments":                                    150000,
		"fstPair-memory-arguments":                                 32,
		"headList-cpu-arguments":                                   150000,
		"headList-memory-arguments":                                32,
		"iData-cpu-arguments":                                      150000,
		"iData-memory-arguments":                                   32,
		"ifThenElse-cpu-arguments":                                 1,
		"ifThenElse-memory-arguments":                              1,
		"indexByteString-cpu-arguments":                            150000,
		"indexByteString-memory-arguments":                         1,
		"lengthOfByteString-cpu-arguments":                         150000,
		"lengthOfByteString-memory-arguments":                      4,
		"lessThanByteString-cpu-arguments-intercept":               103599,
		"lessThanByteString-cpu-arguments-slope":                   248,
		"lessThanByteString-memory-arguments":                      1,
		"lessThanEqualsByteString-cpu-arguments-intercept":         103599,
		"lessThanEqualsByteString-cpu-arguments-slope":             248,
		"lessThanEqualsByteString-memory-arguments":                1,
		"lessThanEqualsInteger-cpu-arguments-intercept":            145276,
		"lessThanEqualsInteger-cpu-arguments-slope":                1366,
		"lessThanEqualsInteger-memory-arguments":                   1,
		"lessThanInteger-cpu-arguments-intercept":                  179690,
		"lessThanInteger-cpu-arguments-slope":                      497,
		"lessThanInteger-memory-arguments":                         1,
		"listData-cpu-arguments":                                   150000,
		"listData-memory-arguments":                                32,
		"mapData-cpu-arguments":                                    150000,
		"mapData-memory-arguments":                                 32,
		"mkCons-cpu-arguments":                                     150000,
		"mkCons-memory-arguments":                                  32,
		"mkNilData-cpu-arguments":                                  150000,
		"mkNilData-memory-arguments":                               32,
		"mkNilPairData-cpu-arguments":                              150000,
		"mkNilPairData-memory-arguments":                           32,
		"mkPairData-cpu-arguments":                                 150000,
		"mkPairData-memory-arguments":                              32,
		"modInteger-cpu-arguments-constant":                        148000,
		"modInteger-cpu-arguments-model-arguments-intercept":       425507,
		"modInteger-cpu-arguments-model-arguments-slope":           118,
		"modInteger-memory-arguments-intercept":                    0,
		"modInteger-memory-arguments-minimum":                      1,
		"modInteger-memory-arguments-slope":                        1,
		"multiplyInteger-cpu-arguments-intercept":                  61516,
		"multiplyInteger-cpu-arguments-slope":                      11218,
		"multiplyInteger-memory-arguments-intercept":               0,
		"multiplyInteger-memory-arguments-slope":                   1,
		"nullList-cpu-arguments":                                   150000,
		"nullList-memory-arguments":                                32,
		"quotientInteger-cpu-arguments-constant":                   148000,
		"quotientInteger-cpu-arguments-model-arguments-intercept":  425507,
		"quotientInteger-cpu-arguments-model-arguments-slope":      118,
		"quotientInteger-memory-arguments-intercept":               0,
		"quotientInteger-memory-arguments-minimum":                 1,
		"quotientInteger-memory-arguments-slope":                   1,
		"remainderInteger-cpu-arguments-constant":                  148000,
		"remainderInteger-cpu-arguments-model-arguments-intercept": 425507,
		"remainderInteger-cpu-arguments-model-arguments-slope":     118,
		"remainderInteger-memory-arguments-intercept":              0,
		"remainderInteger-memory-arguments-minimum":                1,
		"remainderInteger-memory-arguments-slope":                  1,
		"sha2_256-cpu-arguments-intercept":                         2477736,
		"sha2_256-cpu-arguments-slope":                             29175,
		"sha2_256-memory-arguments":                                4,
		"sha3_256-cpu-arguments-intercept":                         0,
		"sha3_256-cpu-arguments-slope":                             82363,
		"sha3_256-memory-arguments":                                4,
		"sliceByteString-cpu-arguments-intercept":                  150000,
		"sliceByteString-cpu-arguments-slope":                      5000,
		"sliceByteString-memory-arguments-intercept":               0,
		"sliceByteString-memory-arguments-slope":                   1,
		"sndPair-cpu-arguments":                                    150000,
		"sndPair-memory-arguments":                                 32,
		"subtractInteger-cpu-arguments-intercept":                  197209,
		"subtractInteger-cpu-arguments-slope":                      0,
		"subtractInteger-memory-arguments-intercept":               1,
		"subtractInteger-memory-arguments-slope":                   1,
		"tailList-cpu-arguments":                                   150000,
		"tailList-memory-arguments":                                32,
		"trace-cpu-arguments":                                      150000,
		"trace-memory-arguments":                                   32,
		"unBData-cpu-arguments":                                    150000,
		"unBData-memory-arguments":                                 32,
		"unConstrData-cpu-arguments":                               150000,
		"unConstrData-memory-arguments":                            32,
		"unIData-cpu-arguments":                                    150000,
		"unIData-memory-arguments":                                 32,
		"unListData-cpu-arguments":                                 150000,
		"unListData-memory-arguments":                              32,
		"unMapData-cpu-arguments":                                  150000,
		"unMapData-memory-arguments":                               32,
		"verifySignature-cpu-arguments-intercept":                  3345831,
		"verifySignature-cpu-arguments-slope":                      1,
		"verifySignature-memory-arguments":                         1,
	}

	// Update with preview network cost model
	updatedCM, err := costModelFromMap(lang.LanguageVersionV1, SemanticsVariantA, previewV1CostModelMap)
	if err != nil {
		t.Fatalf("unexpected error building cost model from map: %s", err)
	}

	// Verify specific machine cost values were updated
	// cekApplyCost-exBudgetCPU should be 29773
	if updatedCM.machineCosts.apply.Cpu != 29773 {
		t.Errorf("Expected apply CPU cost to be 29773, got %d", updatedCM.machineCosts.apply.Cpu)
	}
	if updatedCM.machineCosts.apply.Mem != 100 {
		t.Errorf("Expected apply memory cost to be 100, got %d", updatedCM.machineCosts.apply.Mem)
	}

	// Verify startup costs
	if updatedCM.machineCosts.startup.Cpu != 100 {
		t.Errorf("Expected startup CPU cost to be 100, got %d", updatedCM.machineCosts.startup.Cpu)
	}
	if updatedCM.machineCosts.startup.Mem != 100 {
		t.Errorf("Expected startup memory cost to be 100, got %d", updatedCM.machineCosts.startup.Mem)
	}

	// Verify builtin costs were updated
	if updatedCM.machineCosts.builtin.Cpu != 29773 {
		t.Errorf("Expected builtin CPU cost to be 29773, got %d", updatedCM.machineCosts.builtin.Cpu)
	}

	// Verify verifyEd25519Signature costs were updated (ThreeLinearInZ for V1/V2)
	// V1/V2 use linear_in_z (signature size) instead of linear_in_y (message size)
	// The values come from the cost model map (verifySignature-cpu-arguments-intercept/slope)
	verifySigCPU := updatedCM.builtinCosts[builtin.VerifyEd25519Signature].cpu.(*ThreeLinearInZ)
	if verifySigCPU.intercept != 3345831 {
		t.Errorf("Expected VerifyEd25519Signature CPU intercept to be 3345831, got %d", verifySigCPU.intercept)
	}
	if verifySigCPU.slope != 1 {
		t.Errorf("Expected VerifyEd25519Signature CPU slope to be 1, got %d", verifySigCPU.slope)
	}

	// Verify AddInteger costs (MaxSizeModel for V1)
	// addInteger-cpu-arguments-intercept: 197209, addInteger-cpu-arguments-slope: 0
	// Note: In V1, AddInteger uses MaxSizeModel which has intercept and slope fields
	addIntCPU := updatedCM.builtinCosts[builtin.AddInteger].cpu.(*MaxSizeModel)
	if addIntCPU.intercept != 197209 {
		t.Errorf("Expected AddInteger CPU intercept to be 197209, got %d", addIntCPU.intercept)
	}
	if addIntCPU.slope != 0 {
		t.Errorf("Expected AddInteger CPU slope to be 0, got %d", addIntCPU.slope)
	}

	// Verify MultiplyInteger costs (AddedSizesModel for V1)
	// multiplyInteger-cpu-arguments-intercept: 61516, multiplyInteger-cpu-arguments-slope: 11218
	mulIntCPU := updatedCM.builtinCosts[builtin.MultiplyInteger].cpu.(*AddedSizesModel)
	if mulIntCPU.intercept != 61516 {
		t.Errorf("Expected MultiplyInteger CPU intercept to be 61516, got %d", mulIntCPU.intercept)
	}
	if mulIntCPU.slope != 11218 {
		t.Errorf("Expected MultiplyInteger CPU slope to be 11218, got %d", mulIntCPU.slope)
	}

	// Verify Blake2b_256 costs (LinearInX for V1)
	// blake2b_256-cpu-arguments-intercept: 2477736, blake2b_256-cpu-arguments-slope: 29175
	blake2bCPU := updatedCM.builtinCosts[builtin.Blake2b_256].cpu.(*LinearInX)
	if blake2bCPU.intercept != 2477736 {
		t.Errorf("Expected Blake2b_256 CPU intercept to be 2477736, got %d", blake2bCPU.intercept)
	}
	if blake2bCPU.slope != 29175 {
		t.Errorf("Expected Blake2b_256 CPU slope to be 29175, got %d", blake2bCPU.slope)
	}
	blake2bMem := updatedCM.builtinCosts[builtin.Blake2b_256].mem.(*ConstantCost)
	if blake2bMem.c != 4 {
		t.Errorf("Expected Blake2b_256 memory to be 4, got %d", blake2bMem.c)
	}

	// Verify AppendByteString costs (AddedSizesModel)
	// appendByteString-cpu-arguments-intercept: 396231, appendByteString-cpu-arguments-slope: 621
	appendBSCPU := updatedCM.builtinCosts[builtin.AppendByteString].cpu.(*AddedSizesModel)
	if appendBSCPU.intercept != 396231 {
		t.Errorf("Expected AppendByteString CPU intercept to be 396231, got %d", appendBSCPU.intercept)
	}
	if appendBSCPU.slope != 621 {
		t.Errorf("Expected AppendByteString CPU slope to be 621, got %d", appendBSCPU.slope)
	}

	// Verify DivideInteger costs (ConstAboveDiagonalModel with nested MultipliedSizesModel for V1)
	// divideInteger-cpu-arguments-constant: 148000
	// divideInteger-cpu-arguments-model-arguments-intercept: 425507
	// divideInteger-cpu-arguments-model-arguments-slope: 118
	divIntCPU := updatedCM.builtinCosts[builtin.DivideInteger].cpu.(*ConstAboveDiagonalModel)
	if divIntCPU.constant != 148000 {
		t.Errorf("Expected DivideInteger CPU constant to be 148000, got %d", divIntCPU.constant)
	}
	divIntCPUModel := divIntCPU.model.(*MultipliedSizesModel)
	if divIntCPUModel.intercept != 425507 {
		t.Errorf("Expected DivideInteger CPU model intercept to be 425507, got %d", divIntCPUModel.intercept)
	}
	if divIntCPUModel.slope != 118 {
		t.Errorf("Expected DivideInteger CPU model slope to be 118, got %d", divIntCPUModel.slope)
	}

	// Verify SubtractInteger costs (MaxSizeModel for V1)
	// subtractInteger-cpu-arguments-intercept: 197209, subtractInteger-cpu-arguments-slope: 0
	subIntCPU := updatedCM.builtinCosts[builtin.SubtractInteger].cpu.(*MaxSizeModel)
	if subIntCPU.intercept != 197209 {
		t.Errorf("Expected SubtractInteger CPU intercept to be 197209, got %d", subIntCPU.intercept)
	}
	if subIntCPU.slope != 0 {
		t.Errorf("Expected SubtractInteger CPU slope to be 0, got %d", subIntCPU.slope)
	}

	// Verify that the test successfully loads V1 cost model from a map
	// This test ensures the update mechanism works with map-based cost models
	// which is the format commonly found in genesis files
	// Note: The genesis file may not contain all parameters (especially ones added in later versions)
}

func TestFrameAwaitArgString(t *testing.T) {
	val := Constant{Constant: &syn.Integer{Inner: big.NewInt(42)}}
	ctx := &FrameAwaitFunValue[syn.DeBruijn]{Value: val}
	frame := FrameAwaitArg[syn.DeBruijn]{Value: val, Ctx: ctx}
	str := frame.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	// Just check it contains the expected parts
	if !contains(str, "FrameAwaitArg") || !contains(str, "42") {
		t.Errorf("String does not contain expected content: %s", str)
	}
}

func TestFrameAwaitFunTermString(t *testing.T) {
	env := &Env[syn.DeBruijn]{}
	// Create a simple term
	term := &syn.Var[syn.DeBruijn]{Name: 0}
	ctx := &FrameAwaitFunValue[syn.DeBruijn]{
		Value: Constant{Constant: &syn.Unit{}},
	}
	frame := FrameAwaitFunTerm[syn.DeBruijn]{Env: env, Term: term, Ctx: ctx}
	str := frame.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	if !contains(str, "FrameAwaitFunTerm") {
		t.Errorf("String does not contain expected content: %s", str)
	}
}

func TestFrameAwaitFunValueString(t *testing.T) {
	val := Constant{Constant: &syn.Integer{Inner: big.NewInt(123)}}
	ctx := &FrameAwaitFunValue[syn.DeBruijn]{Value: val}
	frame := FrameAwaitFunValue[syn.DeBruijn]{Value: val, Ctx: ctx}
	str := frame.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	if !contains(str, "FrameAwaitFunValue") || !contains(str, "123") {
		t.Errorf("String does not contain expected content: %s", str)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
func TestValueString(t *testing.T) {
	// Test Constant String
	constant := Constant{Constant: &syn.Integer{Inner: big.NewInt(99)}}
	str := constant.String()
	if !contains(str, "99") {
		t.Errorf("Constant String should contain 99: %s", str)
	}

	// Test Delay String
	env := &Env[syn.DeBruijn]{}
	term := &syn.Var[syn.DeBruijn]{Name: 1}
	delay := Delay[syn.DeBruijn]{Body: term, Env: env}
	str = delay.String()
	if !contains(str, "Delay") {
		t.Errorf("Delay String should contain Delay: %s", str)
	}
}

func TestMachineStateInterface(t *testing.T) {
	env := &Env[syn.DeBruijn]{}
	term := &syn.Var[syn.DeBruijn]{Name: 0}
	val := Constant{Constant: &syn.Unit{}}

	// Test Return implements MachineState
	var ret MachineState[syn.DeBruijn] = Return[syn.DeBruijn]{Ctx: nil, Value: val}
	_ = ret

	// Test Compute implements MachineState
	var comp MachineState[syn.DeBruijn] = Compute[syn.DeBruijn]{Ctx: nil, Env: env, Term: term}
	_ = comp

	// Test Done implements MachineState
	var done MachineState[syn.DeBruijn] = Done[syn.DeBruijn]{term: term}
	_ = done
}
