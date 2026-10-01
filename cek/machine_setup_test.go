package cek

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func TestNewMachineSharesCostCachesPerCostModel(t *testing.T) {
	evalContext, err := NewEvalContext(
		LanguageVersionV2,
		ProtoVersion{Major: 8},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	m1 := NewMachine[syn.DeBruijn](LanguageVersionV2, 0, evalContext)
	m2 := NewMachine[syn.DeBruijn](LanguageVersionV2, 0, evalContext)
	if m1.costCaches == nil || m1.costCaches != m2.costCaches {
		t.Fatal("machines from one EvalContext do not share cost caches")
	}
	if m1.costCaches.src != evalContext.CostModel.builtinCosts {
		t.Fatal("shared cost caches were not built from the context's costs")
	}

	// A clone may be rewritten in place, so it must not carry a cache.
	cloned := evalContext.CostModel.Clone()
	if cloned.caches != nil {
		t.Fatal("Clone returned a prebuilt cost cache")
	}
	m3 := NewMachine[syn.DeBruijn](
		LanguageVersionV2,
		0,
		&EvalContext{CostModel: cloned, ProtoMajor: 8},
	)
	if m3.costCaches == m1.costCaches || m3.costCaches.src != cloned.builtinCosts {
		t.Fatal("machine from a cloned cost model reused another model's cache")
	}
}

func TestNewAvailableBuiltinsMemoMatchesBuild(t *testing.T) {
	versions := []lang.LanguageVersion{
		lang.LanguageVersionV1,
		lang.LanguageVersionV2,
		lang.LanguageVersionV3,
		lang.LanguageVersionV4,
	}
	for _, version := range versions {
		for protoMajor := uint(0); protoMajor <= 13; protoMajor++ {
			want := buildAvailableBuiltins(version, protoMajor)
			first := newAvailableBuiltins(version, protoMajor)
			second := newAvailableBuiltins(version, protoMajor)
			if *first != *want {
				t.Fatalf("availability for %v at proto %d differs from build", version, protoMajor)
			}
			if first != second {
				t.Fatalf("availability for %v at proto %d is rebuilt per call", version, protoMajor)
			}
		}
	}
}

func randomCostData(r *rand.Rand, depth int) data.PlutusData {
	leaf := depth <= 0 || r.Intn(3) == 0
	if leaf {
		if r.Intn(2) == 0 {
			n := new(big.Int).Lsh(big.NewInt(int64(r.Intn(1000))), uint(r.Intn(200)))
			return data.NewInteger(n)
		}
		return data.NewByteString(make([]byte, r.Intn(100)))
	}
	children := make([]data.PlutusData, r.Intn(5))
	for i := range children {
		children[i] = randomCostData(r, depth-1)
	}
	switch r.Intn(3) {
	case 0:
		return data.NewConstr(uint64(r.Intn(10)), children...)
	case 1:
		return data.NewList(children...)
	default:
		pairs := make([][2]data.PlutusData, len(children))
		for i := range children {
			pairs[i] = [2]data.PlutusData{randomCostData(r, depth-1), children[i]}
		}
		return data.NewMap(pairs)
	}
}

// equalsDataMinExMem stops early and walks in a different order from
// dataExMem; whatever the shapes, it must still cost the exact smaller size.
func TestEqualsDataMinExMemIsMinOfFullSizes(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := range 2000 {
		x := randomCostData(r, r.Intn(6))
		y := randomCostData(r, r.Intn(6))
		if i%10 == 0 {
			y = data.NewByteString(nil)
		}
		want := min(dataExMem(x)(), dataExMem(y)())
		if got := equalsDataMinExMem(x, y); got != want {
			t.Fatalf("case %d: equalsDataMinExMem = %d, want %d", i, got, want)
		}
		if got := equalsDataMinExMem(y, x); got != want {
			t.Fatalf("case %d (swapped): equalsDataMinExMem = %d, want %d", i, got, want)
		}
	}
}
